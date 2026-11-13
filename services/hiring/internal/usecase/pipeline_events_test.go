package usecase_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/frngrit/HireAssist/services/hiring/internal/adapter/memory"
	"github.com/frngrit/HireAssist/services/hiring/internal/domain"
	"github.com/frngrit/HireAssist/services/hiring/internal/usecase"
)

func types(events []domain.PipelineEvent) []domain.PipelineEventType {
	out := make([]domain.PipelineEventType, len(events))
	for i, e := range events {
		out[i] = e.Type
	}
	return out
}

// ADR-015: each lifecycle change is saved with its event; revising criteria reports nothing.
func TestJobOpeningChangesAreSavedWithTheirPipelineEvents(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	ctx := context.Background()
	draft := validDraft()
	draft.ExpectedTimeToFill = 30*24*time.Hour + time.Hour // a part day rounds up
	o, err := svc.CreateJobOpening(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReviseScreeningCriteria(ctx, o.ID, draft.Criteria); err != nil {
		t.Fatal(err)
	}
	for _, step := range []func(context.Context, string) (*domain.JobOpening, error){svc.PauseOpening, svc.ResumeOpening} {
		if _, err := step(ctx, o.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.CloseOpening(ctx, o.ID, domain.OutcomeFilled); err != nil {
		t.Fatal(err)
	}
	// A refused change reports nothing.
	if _, err := svc.PauseOpening(ctx, o.ID); err == nil {
		t.Fatal("pausing a closed opening succeeded")
	}

	want := []domain.PipelineEventType{domain.EventOpeningOpened, domain.EventOpeningPaused, domain.EventOpeningResumed, domain.EventOpeningClosed}
	if got := types(repo.events); !slices.Equal(got, want) {
		t.Fatalf("events %v, want %v", got, want)
	}
	opened := repo.events[0]
	if opened.JobOpeningID != o.ID || opened.Title != draft.Title || opened.ExpectedTimeToFillDays != 31 || !opened.OccurredAt.Equal(o.OpenedAt) {
		t.Errorf("opened event %+v", opened)
	}
	if repo.events[3].Title != "" || repo.events[3].ExpectedTimeToFillDays != 0 {
		t.Errorf("only opening.opened carries the title and time-to-fill: %+v", repo.events[3])
	}
}

// UC-4's stages: every accepted resume is applied; only a scored one is screened; a shortlist
// carries the candidate.
func TestScreeningIsSavedWithItsPipelineEvents(t *testing.T) {
	f := newScreeningFixture(t, nil)
	b := f.submit(t, pdf("ploy.pdf"), pdf("somchai.pdf"))
	f.record(t, b.ID, "rs-1", domain.EntryScreened, scored("cand-1", 70))
	f.record(t, b.ID, "rs-2", domain.EntryNeedsReview, domain.ScreeningResult{Reason: "scanned image"})
	if _, err := f.svc.OverrideScore(context.Background(), f.opening.ID, "rs-1", 80); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ShortlistCandidate(context.Background(), f.opening.ID, "rs-1"); err != nil {
		t.Fatal(err)
	}

	events := f.batches.PipelineEvents()
	want := []domain.PipelineEventType{domain.EventResumeSubmitted, domain.EventResumeSubmitted, domain.EventResumeScreened, domain.EventCandidateShortlisted}
	if got := types(events); !slices.Equal(got, want) {
		t.Fatalf("events %v, want %v", got, want)
	}
	for _, e := range events {
		if e.JobOpeningID != f.opening.ID || e.ResumeID == "" {
			t.Errorf("event %+v lacks its opening or resume", e)
		}
	}
	if events[0].CandidateID != "" || events[2].CandidateID != "cand-1" || events[3].CandidateID != "cand-1" {
		t.Errorf("candidate ids %q %q %q", events[0].CandidateID, events[2].CandidateID, events[3].CandidateID)
	}
}

type fakeOutbox struct {
	waiting   []domain.PipelineEvent
	published []string
}

func (o *fakeOutbox) Unpublished(_ context.Context, limit int) ([]domain.PipelineEvent, error) {
	out := []domain.PipelineEvent{}
	for _, e := range o.waiting {
		if !slices.Contains(o.published, e.ID) && len(out) < limit {
			out = append(out, e)
		}
	}
	return out, nil
}

func (o *fakeOutbox) MarkPublished(_ context.Context, id string, _ time.Time) error {
	o.published = append(o.published, id)
	return nil
}

type flakyPublisher struct {
	sent   []string
	failOn string
}

func (p *flakyPublisher) PublishPipelineEvent(_ context.Context, e domain.PipelineEvent) error {
	if e.ID == p.failOn {
		return errors.New("broker down")
	}
	p.sent = append(p.sent, e.ID)
	return nil
}

// An event is marked published only once the broker took it, and a failure stops the pass so
// the order is kept; the next pass carries on from there.
func TestPipelineRelayKeepsOrderAndRetries(t *testing.T) {
	outbox := &fakeOutbox{waiting: []domain.PipelineEvent{{ID: "pe-1"}, {ID: "pe-2"}, {ID: "pe-3"}}}
	pub := &flakyPublisher{failOn: "pe-2"}
	relay := &usecase.PipelineRelay{Outbox: outbox, Publisher: pub, Clock: fixedClock{now}}

	sent, err := relay.PublishPending(context.Background())
	if err == nil || sent != 1 || !slices.Equal(outbox.published, []string{"pe-1"}) {
		t.Fatalf("first pass: sent %d, err %v, published %v", sent, err, outbox.published)
	}
	pub.failOn = ""
	if sent, err := relay.PublishPending(context.Background()); err != nil || sent != 2 {
		t.Fatalf("second pass: sent %d, err %v", sent, err)
	}
	if !slices.Equal(pub.sent, []string{"pe-1", "pe-2", "pe-3"}) {
		t.Errorf("sent %v", pub.sent)
	}
}

// eraseHiringData() (ADR-016): files, screening rows and guides go, other candidates stay, the
// candidate id leaves the held events, and asking again succeeds.
func TestEraseCandidateRemovesEverythingHeldAndCanBeRepeated(t *testing.T) {
	f := newScreeningFixture(t, nil)
	b := f.submit(t, pdf("ploy.pdf"), pdf("somchai.pdf"))
	f.record(t, b.ID, "rs-1", domain.EntryScreened, scored("cand-1", 70))
	f.record(t, b.ID, "rs-2", domain.EntryScreened, scored("cand-2", 60))
	guides := memory.NewInterviewGuideRepository()
	ctx := context.Background()
	for _, c := range []string{"cand-1", "cand-2"} {
		if err := guides.Save(ctx, &domain.InterviewGuide{ID: "ig-" + c, JobOpeningID: f.opening.ID, CandidateID: c}); err != nil {
			t.Fatal(err)
		}
	}
	keep := b.Entry("rs-2").ObjectKey
	gone := b.Entry("rs-1").ObjectKey
	erasure := &usecase.CandidateErasure{Batches: f.batches, Files: f.files, Guides: guides}

	for range 2 {
		if err := erasure.EraseCandidate(ctx, "cand-1"); err != nil {
			t.Fatal(err)
		}
	}

	if _, ok := f.files.Resume(gone); ok {
		t.Error("the candidate's resume file is still stored")
	}
	if _, ok := f.files.Resume(keep); !ok {
		t.Error("another candidate's resume file was deleted")
	}
	if _, err := f.batches.FindBatchOfResume(ctx, "rs-1"); !errors.Is(err, usecase.ErrResumeNotFound) {
		t.Errorf("the candidate's screening entry is still held: %v", err)
	}
	if _, err := f.batches.FindBatchOfResume(ctx, "rs-2"); err != nil {
		t.Errorf("another candidate's entry is gone: %v", err)
	}
	if _, err := guides.FindByCandidate(ctx, f.opening.ID, "cand-1"); !errors.Is(err, usecase.ErrGuideNotFound) {
		t.Errorf("the candidate's guide is still held: %v", err)
	}
	if _, err := guides.FindByCandidate(ctx, f.opening.ID, "cand-2"); err != nil {
		t.Errorf("another candidate's guide is gone: %v", err)
	}
	for _, e := range f.batches.PipelineEvents() {
		if e.CandidateID == "cand-1" {
			t.Errorf("event %s still names the erased candidate", e.Type)
		}
	}
}

func TestEraseCandidateFailsWhileAFileCannotBeDeleted(t *testing.T) {
	f := newScreeningFixture(t, nil)
	b := f.submit(t, pdf("ploy.pdf"))
	f.record(t, b.ID, "rs-1", domain.EntryScreened, scored("cand-1", 70))
	erasure := &usecase.CandidateErasure{Batches: f.batches, Files: brokenStore{}, Guides: memory.NewInterviewGuideRepository()}

	if err := erasure.EraseCandidate(context.Background(), "cand-1"); err == nil {
		t.Fatal("erasure succeeded with the file still stored")
	}
	// The row is how the file is found, so it must survive for the retry.
	if _, err := f.batches.FindBatchOfResume(context.Background(), "rs-1"); err != nil {
		t.Errorf("the entry was deleted before its file: %v", err)
	}
}
