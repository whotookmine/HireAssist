package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/frngrit/HireAssist/services/compliance/internal/adapter/memory"
	"github.com/frngrit/HireAssist/services/compliance/internal/domain"
	"github.com/frngrit/HireAssist/services/compliance/internal/usecase"
)

var t0 = time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

type ids struct{ n int }

func (i *ids) NewID(prefix string) string { i.n++; return fmt.Sprintf("%s-%d", prefix, i.n) }

type notice struct{ to, subject, body string }

type outbox struct{ sent []notice }

func (o *outbox) Notify(_ context.Context, to, subject, body string) error {
	o.sent = append(o.sent, notice{to, subject, body})
	return nil
}

type register struct{ candidates []usecase.RegisteredCandidate }

func (r *register) ListCandidates(context.Context) ([]usecase.RegisteredCandidate, error) {
	return r.candidates, nil
}

// holder erases by removing the candidate from the register, and can be told to fail.
type holder struct {
	reg    *register
	fail   error
	erased []string
}

func (h *holder) EraseCandidate(_ context.Context, id string) error {
	if h.fail != nil {
		return h.fail
	}
	h.erased = append(h.erased, id)
	if h.reg != nil {
		kept := h.reg.candidates[:0]
		for _, c := range h.reg.candidates {
			if c.ID != id {
				kept = append(kept, c)
			}
		}
		h.reg.candidates = kept
	}
	return nil
}

type fixture struct {
	ctx       context.Context
	clock     *clock
	store     *memory.Store
	mail      *outbox
	reg       *register
	hiring    *holder
	profiles  *holder
	pipeline  *usecase.Pipeline
	retention *usecase.Retention
}

func newFixture() *fixture {
	f := &fixture{ctx: context.Background(), clock: &clock{t0}, store: memory.New(), mail: &outbox{}, reg: &register{}}
	f.hiring, f.profiles = &holder{}, &holder{reg: f.reg}
	f.pipeline = &usecase.Pipeline{Store: f.store, Notifier: f.mail, Clock: f.clock, FallbackRecipient: "hr@acme.example"}
	erasures := &usecase.Erasures{Store: f.store, IDs: &ids{}, Clock: f.clock,
		Holders: map[domain.Holder]usecase.HolderEraser{domain.HolderHiring: f.hiring, domain.HolderResumeProcessing: f.profiles}}
	f.retention = &usecase.Retention{Policies: f.store, Marks: f.store, Pipeline: f.store, Register: f.reg,
		Erasures: erasures, Notifier: f.mail, Clock: f.clock, FallbackRecipient: "hr@acme.example"}
	return f
}

func (f *fixture) event(t *testing.T, e domain.PipelineEvent) {
	t.Helper()
	if e.OccurredAt.IsZero() {
		e.OccurredAt = f.clock.now
	}
	if err := f.pipeline.RecordPipelineEvent(f.ctx, e); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) open(t *testing.T, id string, ttfDays int) {
	f.event(t, domain.PipelineEvent{ID: "open-" + id, Type: domain.OpeningOpened, JobOpeningID: id,
		Title: "Backend Engineer", ExpectedTimeToFillDays: ttfDays, Recruiter: "somchai@acme.example"})
}

func (f *fixture) policy(t *testing.T, days, warn int) {
	t.Helper()
	if _, err := f.retention.SetRetentionPolicy(f.ctx, days, domain.AnchorCollected, domain.ActionDelete, warn); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) sweep(t *testing.T) usecase.RetentionReport {
	t.Helper()
	r, err := f.retention.EvaluateRetention(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestDashboardCountsEachResumeOnce(t *testing.T) {
	f := newFixture()
	f.open(t, "job-1", 30)
	for _, e := range []domain.PipelineEvent{
		{ID: "e1", Type: domain.ResumeSubmitted, JobOpeningID: "job-1", ResumeID: "res-1"},
		{ID: "e2", Type: domain.ResumeSubmitted, JobOpeningID: "job-1", ResumeID: "res-2"},
		{ID: "e2", Type: domain.ResumeSubmitted, JobOpeningID: "job-1", ResumeID: "res-2"}, // redelivered
		{ID: "e3", Type: domain.ResumeScreened, JobOpeningID: "job-1", ResumeID: "res-1", CandidateID: "cand-1"},
		{ID: "e4", Type: domain.CandidateShortlisted, JobOpeningID: "job-1", ResumeID: "res-1", CandidateID: "cand-1"},
	} {
		f.event(t, e)
	}
	f.clock.now = t0.Add(5 * domain.Day)
	rows, err := f.pipeline.GetPipelineDashboard(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows", len(rows))
	}
	got := rows[0]
	c := got.Opening.Counts
	if c[domain.StageApplied] != 2 || c[domain.StageScreened] != 1 || c[domain.StageShortlisted] != 1 || got.DaysOpen != 5 {
		t.Errorf("row %+v", got)
	}

	f.event(t, domain.PipelineEvent{ID: "e5", Type: domain.OpeningClosed, JobOpeningID: "job-1"})
	if rows, _ := f.pipeline.GetPipelineDashboard(f.ctx); len(rows) != 0 {
		t.Error("a closed opening is not on the dashboard")
	}
}

func TestStaleOpeningIsFlaggedAndNotifiedOnceThenClearedOnClose(t *testing.T) {
	f := newFixture()
	f.open(t, "job-1", 30)
	f.clock.now = t0.Add(31 * domain.Day)
	for range 2 {
		if _, err := f.pipeline.EvaluateStaleness(f.ctx); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.mail.sent) != 1 || f.mail.sent[0].to != "somchai@acme.example" {
		t.Fatalf("notices %+v, want one to the opening's Recruiter", f.mail.sent)
	}
	if rows, _ := f.pipeline.GetPipelineDashboard(f.ctx); !rows[0].Flagged {
		t.Error("not flagged on the dashboard")
	}
	f.event(t, domain.PipelineEvent{ID: "p", Type: domain.OpeningPaused, JobOpeningID: "job-1"})
	if r, _ := f.pipeline.EvaluateStaleness(f.ctx); len(r.Cleared) != 1 {
		t.Errorf("pausing should clear the flag at the next sweep: %+v", r)
	}
	f.event(t, domain.PipelineEvent{ID: "c", Type: domain.OpeningClosed, JobOpeningID: "job-1"})
	o, _ := f.store.GetOpening(f.ctx, "job-1")
	if o.FlaggedAt != nil {
		t.Error("closing must clear the flag (FR-4.5)")
	}
}

func TestNoPolicyErasesNothing(t *testing.T) {
	f := newFixture()
	f.reg.candidates = []usecase.RegisteredCandidate{{ID: "cand-1", CollectedAt: t0.AddDate(-5, 0, 0)}}
	r := f.sweep(t)
	if r.PolicySet || len(r.Erasures) != 0 || len(f.profiles.erased) != 0 {
		t.Errorf("report %+v", r)
	}
}

func TestExpiredCandidateIsErasedEverywhereAndForgotten(t *testing.T) {
	f := newFixture()
	f.policy(t, 30, 7)
	f.open(t, "job-1", 30)
	f.event(t, domain.PipelineEvent{ID: "s", Type: domain.ResumeScreened, JobOpeningID: "job-1", ResumeID: "res-1", CandidateID: "cand-1"})
	f.event(t, domain.PipelineEvent{ID: "c", Type: domain.OpeningClosed, JobOpeningID: "job-1"})
	f.reg.candidates = []usecase.RegisteredCandidate{{ID: "cand-1", JobOpeningID: "job-1", CollectedAt: t0}}

	f.clock.now = t0.Add(24 * domain.Day)
	if r := f.sweep(t); len(r.Warned) != 1 || len(f.mail.sent) != 1 || f.mail.sent[0].to != "somchai@acme.example" {
		t.Fatalf("warning: report %+v, notices %+v", r, f.mail.sent)
	}
	f.sweep(t)
	if len(f.mail.sent) != 1 {
		t.Error("warned twice about the same expiry")
	}

	f.clock.now = t0.Add(30 * domain.Day)
	r := f.sweep(t)
	if len(r.Erasures) != 1 || len(f.hiring.erased) != 1 || len(f.profiles.erased) != 1 {
		t.Fatalf("report %+v", r)
	}
	e, err := f.retention.Erasures.GetErasure(f.ctx, r.Erasures[0])
	if err != nil {
		t.Fatal(err)
	}
	if e.Status != domain.ErasureErased || e.CandidateID != "" || e.PolicyDays != 30 || e.Reason != domain.ReasonExpired {
		t.Errorf("audit entry %+v", e)
	}
	if last, _ := f.store.LastActivity(f.ctx, "cand-1"); !last.IsZero() {
		t.Error("the candidate id survives in the activity record")
	}
	o, _ := f.store.GetOpening(f.ctx, "job-1")
	if o.Counts[domain.StageScreened] != 1 {
		t.Error("the aggregate count must survive erasure (FR-5.4)")
	}
}

func TestFailedHolderLeavesTheErasurePendingUntilRetried(t *testing.T) {
	f := newFixture()
	f.profiles.fail = errors.New("503")
	e, err := f.retention.Erasures.EraseCandidate(f.ctx, "cand-9", domain.ReasonRequested, nil)
	if err != nil {
		t.Fatal(err)
	}
	if e.Status != domain.ErasurePending || !e.Done[domain.HolderHiring] || e.LastError == "" {
		t.Fatalf("erasure %+v", e)
	}
	again, _ := f.retention.Erasures.EraseCandidate(f.ctx, "cand-9", domain.ReasonRequested, nil)
	if again.Ref != e.Ref {
		t.Error("a second request started a second erasure")
	}
	if len(f.hiring.erased) != 1 {
		t.Error("a holder that confirmed was asked again")
	}
	f.profiles.fail = nil
	f.sweep(t) // retries even with no policy set
	e, _ = f.retention.Erasures.GetErasure(f.ctx, e.Ref)
	if e.Status != domain.ErasureErased || e.Attempts != 3 {
		t.Errorf("after retry %+v", e)
	}
}

func TestActiveOrUnknownProcessHoldsTheExpiry(t *testing.T) {
	f := newFixture()
	f.policy(t, 30, 7)
	f.open(t, "job-1", 60)
	f.reg.candidates = []usecase.RegisteredCandidate{
		{ID: "cand-1", JobOpeningID: "job-1", CollectedAt: t0},       // opening still open
		{ID: "cand-2", JobOpeningID: "job-unknown", CollectedAt: t0}, // no event ever named it
	}
	f.clock.now = t0.Add(31 * domain.Day)
	r := f.sweep(t)
	if len(r.Erasures) != 0 || len(r.Held) != 2 {
		t.Fatalf("report %+v", r)
	}
	if holds, _ := f.retention.ListHeldExpiries(f.ctx); len(holds) != 2 {
		t.Fatalf("holds %+v", holds)
	}

	if _, err := f.retention.ResolveHeldExpiry(f.ctx, "cand-1", domain.DecisionKeep); err != nil {
		t.Fatal(err)
	}
	if r := f.sweep(t); len(r.Held) != 1 {
		t.Errorf("a kept expiry was held again at once: %+v", r)
	}
	e, err := f.retention.ResolveHeldExpiry(f.ctx, "cand-2", domain.DecisionErase)
	if err != nil || e.Status != domain.ErasureErased || e.Reason != domain.ReasonHoldResolved {
		t.Fatalf("erase decision: %+v, %v", e, err)
	}
	if _, err := f.retention.ResolveHeldExpiry(f.ctx, "cand-2", domain.DecisionErase); !errors.Is(err, usecase.ErrNotFound) {
		t.Errorf("resolving a hold that is gone: %v", err)
	}

	f.clock.now = t0.Add(39 * domain.Day) // past the keep decision's window
	if r := f.sweep(t); len(r.Held) != 1 || r.Held[0] != "cand-1" {
		t.Errorf("the kept expiry should be asked again: %+v", r)
	}
}
