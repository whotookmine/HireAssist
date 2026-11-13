package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/frngrit/HireAssist/services/compliance/internal/domain"
)

// The integration test runs only against a real database:
//
//	COMPLIANCE_TEST_DATABASE_URL=postgres://… go test ./internal/adapter/postgres
//
// It starts from empty tables, so point it at a database of its own.
func open(t *testing.T) *Store {
	url := os.Getenv("COMPLIANCE_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("COMPLIANCE_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	s, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `TRUNCATE retention_policy, pipeline_entries, openings, retention_holds, retention_marks, erasures, access_rejections`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestProjectionAndErasureRoundTrip(t *testing.T) {
	s, ctx := open(t), context.Background()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	o := &domain.Opening{ID: "job-1", Title: "Backend", Status: domain.StatusOpen, OpenedAt: t0, ExpectedTimeToFillDays: 30, Recruiter: "r@acme.example"}
	if err := s.SaveOpening(ctx, o); err != nil {
		t.Fatal(err)
	}
	for _, e := range []struct {
		stage  domain.Stage
		resume string
		cand   string
	}{{domain.StageApplied, "res-1", ""}, {domain.StageApplied, "res-1", ""}, {domain.StageScreened, "res-1", "cand-1"}} {
		if err := s.AddEntry(ctx, "job-1", e.stage, e.resume, e.cand, t0.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	flag := t0.Add(31 * domain.Day)
	if err := s.SetFlag(ctx, "job-1", &flag); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveOpening(ctx, o); err != nil { // an event after the flag must not clear it
		t.Fatal(err)
	}
	got, err := s.GetOpening(ctx, "job-1")
	if err != nil || got.Counts[domain.StageApplied] != 1 || got.Counts[domain.StageScreened] != 1 || got.FlaggedAt == nil {
		t.Fatalf("opening %+v, %v", got, err)
	}
	o.Status = domain.StatusClosed
	if err := s.SaveOpening(ctx, o); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.ListOpenings(ctx); len(list) != 0 {
		t.Error("a closed opening is listed")
	}
	if got, _ := s.GetOpening(ctx, "job-1"); got.FlaggedAt != nil {
		t.Error("closing did not clear the flag")
	}

	p, _ := domain.NewRetentionPolicy(30, domain.AnchorCollected, domain.ActionAnonymise, 7, t0)
	if err := s.SavePolicy(ctx, p); err != nil {
		t.Fatal(err)
	}
	e, _ := domain.NewErasure("era-1", "cand-1", domain.ReasonExpired, &p, t0)
	e.Record([]domain.Holder{domain.HolderHiring}, "resume_processing: down", t0)
	if err := s.SaveErasure(ctx, e); err != nil {
		t.Fatal(err)
	}
	pending, err := s.PendingErasureFor(ctx, "cand-1")
	if err != nil || pending == nil || !pending.Done[domain.HolderHiring] || pending.Done[domain.HolderResumeProcessing] {
		t.Fatalf("pending %+v, %v", pending, err)
	}
	if err := s.ForgetCandidate(ctx, "cand-1"); err != nil {
		t.Fatal(err)
	}
	pending.Record([]domain.Holder{domain.HolderResumeProcessing}, "", t0.Add(domain.Day))
	if err := s.SaveErasure(ctx, pending); err != nil {
		t.Fatal(err)
	}
	if last, _ := s.LastActivity(ctx, "cand-1"); !last.IsZero() {
		t.Error("the candidate's activity survived erasure")
	}
	if got, _ := s.GetOpening(ctx, "job-1"); got.Counts[domain.StageScreened] != 1 {
		t.Error("the count did not survive erasure")
	}
	done, err := s.GetErasure(ctx, "era-1")
	if err != nil || done.Status != domain.ErasureErased || done.CandidateID != "" || done.Action != domain.ActionAnonymise {
		t.Fatalf("erased %+v, %v", done, err)
	}

	ran, err := s.WithSweepLock(ctx, func(ctx context.Context) error {
		inner, err := s.WithSweepLock(ctx, func(context.Context) error { return nil })
		if inner {
			t.Error("a second sweep took the lock")
		}
		return err
	})
	if !ran || err != nil {
		t.Fatalf("lock: %v %v", ran, err)
	}
}
