package domain

import (
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func mustPolicy(t *testing.T, days int, anchor Anchor, warn int) RetentionPolicy {
	t.Helper()
	p, err := NewRetentionPolicy(days, anchor, ActionDelete, warn, t0)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNewRetentionPolicyRefusesNonsense(t *testing.T) {
	for name, f := range map[string]func() error{
		"zero period":      func() error { _, err := NewRetentionPolicy(0, AnchorCollected, ActionDelete, 0, t0); return err },
		"unknown anchor":   func() error { _, err := NewRetentionPolicy(30, "birthday", ActionDelete, 0, t0); return err },
		"unknown action":   func() error { _, err := NewRetentionPolicy(30, AnchorCollected, "shred", 0, t0); return err },
		"window too long":  func() error { _, err := NewRetentionPolicy(30, AnchorCollected, ActionDelete, 30, t0); return err },
		"negative warning": func() error { _, err := NewRetentionPolicy(30, AnchorCollected, ActionDelete, -1, t0); return err },
	} {
		if err := f(); !errors.Is(err, ErrInvalidPolicy) {
			t.Errorf("%s: got %v, want ErrInvalidPolicy", name, err)
		}
	}
}

func TestEvaluate(t *testing.T) {
	p := mustPolicy(t, 30, AnchorCollected, 7)
	c := Candidate{ID: "cand-1", CollectedAt: t0}
	for _, tc := range []struct {
		at     time.Time
		active bool
		want   RetentionState
	}{
		{t0.Add(22 * Day), false, Retained},
		{t0.Add(23 * Day), false, Warning},
		{t0.Add(30 * Day), false, Expired},
		{t0.Add(30 * Day), true, Held},
		{t0.Add(23 * Day), true, Warning}, // an active process only holds an expiry
	} {
		c.InActiveProcess = tc.active
		if got := p.Evaluate(c, tc.at); got != tc.want {
			t.Errorf("day %v active=%v: got %s, want %s", tc.at.Sub(t0)/Day, tc.active, got, tc.want)
		}
	}
}

func TestLastActivityAnchorUsesTheLaterDate(t *testing.T) {
	p := mustPolicy(t, 30, AnchorLastActivity, 0)
	c := Candidate{CollectedAt: t0, LastActivityAt: t0.Add(10 * Day)}
	if got := p.ExpiresAt(c); !got.Equal(t0.Add(40 * Day)) {
		t.Errorf("expires %v, want day 40", got)
	}
	c.LastActivityAt = time.Time{} // no activity recorded: falls back to collection
	if got := p.ExpiresAt(c); !got.Equal(t0.Add(30 * Day)) {
		t.Errorf("expires %v, want day 30", got)
	}
}

func TestOpeningStalenessAndClose(t *testing.T) {
	o := &Opening{ID: "job-1"}
	o.Apply(PipelineEvent{Type: OpeningOpened, OccurredAt: t0, Title: "Backend", ExpectedTimeToFillDays: 30})
	if o.IsStale(t0.Add(30 * Day)) {
		t.Error("stale on day 30; the rule is longer than the time-to-fill")
	}
	if !o.IsStale(t0.Add(31 * Day)) {
		t.Error("not stale on day 31")
	}
	o.Apply(PipelineEvent{Type: OpeningOpened, OccurredAt: t0.Add(31 * Day), Title: "Backend", ExpectedTimeToFillDays: 60})
	if !o.OpenedAt.Equal(t0) || o.IsStale(t0.Add(31*Day)) {
		t.Error("a re-sent opened event must keep the opening date and take the new time-to-fill")
	}
	o.ExpectedTimeToFillDays = 30
	o.Apply(PipelineEvent{Type: OpeningPaused})
	if o.IsStale(t0.Add(31 * Day)) {
		t.Error("a paused opening is excluded from staleness (UC-4 4a)")
	}
	o.Apply(PipelineEvent{Type: OpeningResumed})
	flagged := t0
	o.FlaggedAt = &flagged
	o.Apply(PipelineEvent{Type: OpeningClosed})
	if o.FlaggedAt != nil || o.IsStale(t0.Add(99*Day)) {
		t.Error("closing must clear the flag and end staleness (FR-4.5)")
	}
	o.Apply(PipelineEvent{Type: OpeningResumed})
	if o.Status != StatusClosed {
		t.Error("a closed opening must stay closed (FR-1.9)")
	}
}

func TestEventValidation(t *testing.T) {
	ok := PipelineEvent{ID: "e1", Type: ResumeSubmitted, OccurredAt: t0, JobOpeningID: "job-1", ResumeID: "res-1"}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, e := range map[string]PipelineEvent{
		"unknown type":        {ID: "e", Type: "opening.exploded", OccurredAt: t0, JobOpeningID: "j"},
		"no resume":           {ID: "e", Type: ResumeScreened, OccurredAt: t0, JobOpeningID: "j"},
		"shortlist no resume": {ID: "e", Type: CandidateShortlisted, OccurredAt: t0, JobOpeningID: "j", CandidateID: "c"},
		"no time-to-fill":     {ID: "e", Type: OpeningOpened, OccurredAt: t0, JobOpeningID: "j"},
		"no id":               {Type: OpeningClosed, OccurredAt: t0, JobOpeningID: "j"},
	} {
		if err := e.Validate(); !errors.Is(err, ErrInvalidEvent) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestErasureForgetsTheCandidateOnlyWhenEveryHolderConfirms(t *testing.T) {
	p := mustPolicy(t, 30, AnchorCollected, 0)
	e, err := NewErasure("era-1", "cand-1", ReasonExpired, &p, t0)
	if err != nil {
		t.Fatal(err)
	}
	e.Record([]Holder{HolderHiring}, "resume_processing: 503", t0)
	if e.Status != ErasurePending || e.CandidateID != "cand-1" {
		t.Fatalf("partial erasure reported as %s", e.Status)
	}
	e.Record([]Holder{HolderResumeProcessing}, "", t0.Add(Day))
	if e.Status != ErasureErased || e.CandidateID != "" || e.CompletedAt == nil || e.Attempts != 2 {
		t.Fatalf("finished erasure: %+v", e)
	}
}
