package postgres_test

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frngrit/HireAssist/services/hiring/internal/adapter/postgres"
	"github.com/frngrit/HireAssist/services/hiring/internal/domain"
	"github.com/frngrit/HireAssist/services/hiring/internal/usecase"
)

var _ usecase.PipelineOutbox = (*postgres.PipelineOutbox)(nil)

// ADR-015: events are stored with the change, read back oldest first until published; a change
// that fails stores no event; erasing a candidate takes their id out of the outbox.
func TestPipelineOutbox(t *testing.T) {
	repo, o := newScreeningRepo(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("HIRING_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	outbox := postgres.NewPipelineOutbox(pool)

	b := newScreeningBatch(t, "sb-1", o, "rs-1", "rs-2")
	submittedEvents := []domain.PipelineEvent{
		domain.ResumeEvent(domain.EventResumeSubmitted, o.ID, &b.Entries[0], submitted),
		domain.ResumeEvent(domain.EventResumeSubmitted, o.ID, &b.Entries[1], submitted),
	}
	if err := repo.SaveBatch(ctx, b, submittedEvents...); err != nil {
		t.Fatal(err)
	}
	if err := b.RecordResult("rs-1", domain.EntryScreened, domain.ScreeningResult{CandidateID: "cand-1", Score: 70, MeetsMustHaves: true, CompletedAt: submitted}); err != nil {
		t.Fatal(err)
	}
	e := b.Entry("rs-1")
	if err := repo.SaveEntry(ctx, e, domain.ResumeEvent(domain.EventResumeScreened, o.ID, e, submitted)); err != nil {
		t.Fatal(err)
	}
	// A save that fails rolls its event back with it.
	ghost := &domain.ScreeningEntry{ResumeID: "rs-404", Status: domain.EntryPending}
	if err := repo.SaveEntry(ctx, ghost, domain.ResumeEvent(domain.EventCandidateShortlisted, o.ID, ghost, submitted)); err == nil {
		t.Fatal("saving an unknown resume succeeded")
	}

	events, err := outbox.Unpublished(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	var got []domain.PipelineEventType
	for _, ev := range events {
		got = append(got, ev.Type)
		if ev.ID == "" || ev.JobOpeningID != o.ID || !ev.OccurredAt.Equal(submitted) {
			t.Errorf("event read back as %+v", ev)
		}
	}
	want := []domain.PipelineEventType{domain.EventResumeSubmitted, domain.EventResumeSubmitted, domain.EventResumeScreened}
	if !slices.Equal(got, want) {
		t.Fatalf("outbox %v, want %v", got, want)
	}
	if events[2].CandidateID != "cand-1" || events[2].ResumeID != "rs-1" {
		t.Errorf("screened event %+v", events[2])
	}

	if err := outbox.MarkPublished(ctx, events[0].ID, submitted); err != nil {
		t.Fatal(err)
	}
	if left, _ := outbox.Unpublished(ctx, 10); len(left) != 2 || left[0].ID != events[1].ID {
		t.Errorf("after one is published, %d left", len(left))
	}

	keys, err := repo.ResumeKeysOfCandidate(ctx, "cand-1")
	if err != nil || !slices.Equal(keys, []string{e.ObjectKey}) {
		t.Fatalf("keys %v, err %v", keys, err)
	}
	if err := repo.EraseCandidate(ctx, "cand-1"); err != nil {
		t.Fatal(err)
	}
	if keys, _ := repo.ResumeKeysOfCandidate(ctx, "cand-1"); len(keys) != 0 {
		t.Errorf("the candidate's resume is still held")
	}
	var named int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pipeline_outbox WHERE candidate_id = 'cand-1'`).Scan(&named); err != nil || named != 0 {
		t.Errorf("%d outbox rows still name the candidate (err %v)", named, err)
	}
	if _, err := repo.FindBatchOfResume(ctx, "rs-2"); err != nil {
		t.Errorf("the other resume was erased too: %v", err)
	}
}
