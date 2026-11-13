package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/frngrit/HireAssist/services/hiring/internal/domain"
	"github.com/frngrit/HireAssist/services/hiring/internal/usecase"
)

// ---- test doubles for the ports the use case depends on ----

type fakeRepo struct {
	saved   map[string]domain.JobOpening
	order   []string
	saveErr error
	events  []domain.PipelineEvent
}

func newFakeRepo() *fakeRepo { return &fakeRepo{saved: map[string]domain.JobOpening{}} }

func (r *fakeRepo) Save(_ context.Context, o *domain.JobOpening, events ...domain.PipelineEvent) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.events = append(r.events, events...)
	if _, exists := r.saved[o.ID]; !exists {
		r.order = append(r.order, o.ID)
	}
	r.saved[o.ID] = *o
	return nil
}

func (r *fakeRepo) FindByID(_ context.Context, id string) (*domain.JobOpening, error) {
	o, ok := r.saved[id]
	if !ok {
		return nil, usecase.ErrNotFound
	}
	return &o, nil
}

func (r *fakeRepo) List(_ context.Context) ([]domain.JobOpening, error) {
	out := make([]domain.JobOpening, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.saved[id])
	}
	return out, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type sequentialIDs struct{ n int }

func (g *sequentialIDs) NewID() string {
	g.n++
	return fmt.Sprintf("jo-%d", g.n)
}

var now = time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)

func newService(repo *fakeRepo) *usecase.JobOpeningService {
	return usecase.NewJobOpeningService(repo, fixedClock{now}, &sequentialIDs{})
}

func validDraft() domain.JobOpeningDraft {
	return domain.JobOpeningDraft{
		Title:       "Backend Engineer",
		Description: "Backend engineer, 3+ years, Go or Python, must have shipped with Kafka.",
		Criteria: []domain.Criterion{
			{Name: "3+ years of Go or Python", Classification: domain.MustHave, Weight: 5},
			{Name: "Shipped with Kafka or RabbitMQ", Classification: domain.MustHave, Weight: 4},
		},
		ExpectedTimeToFill: 45 * 24 * time.Hour,
	}
}

// ---- createJobOpening() ----

// UC-1 step 6: the confirmed opening is stored as open, under a new id, dated today (FR-1.7).
func TestCreateJobOpening_StoresOpenOpening(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)

	created, err := svc.CreateJobOpening(context.Background(), validDraft())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID != "jo-1" {
		t.Errorf("ID = %q, want the generated %q", created.ID, "jo-1")
	}
	if !created.OpenedAt.Equal(now) {
		t.Errorf("OpenedAt = %v, want the clock's %v", created.OpenedAt, now)
	}
	stored, ok := repo.saved["jo-1"]
	if !ok {
		t.Fatal("opening was not saved")
	}
	if stored.Status != domain.StatusOpen {
		t.Errorf("stored Status = %q, want %q", stored.Status, domain.StatusOpen)
	}
}

// An opening that breaks a business rule is never stored.
func TestCreateJobOpening_RejectsInvalidDraftWithoutSaving(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	draft := validDraft()
	draft.Criteria = nil

	_, err := svc.CreateJobOpening(context.Background(), draft)

	if !errors.Is(err, domain.ErrInvalidJobOpening) {
		t.Fatalf("err = %v, want ErrInvalidJobOpening", err)
	}
	if len(repo.saved) != 0 {
		t.Errorf("saved %d openings, want 0", len(repo.saved))
	}
}

// A storage failure is reported, not swallowed: the Recruiter must not be told it was saved.
func TestCreateJobOpening_ReportsStorageFailure(t *testing.T) {
	repo := newFakeRepo()
	repo.saveErr = errors.New("database unavailable")
	svc := newService(repo)

	_, err := svc.CreateJobOpening(context.Background(), validDraft())

	if !errors.Is(err, repo.saveErr) {
		t.Fatalf("err = %v, want the storage error", err)
	}
}

// ---- getJobOpening() and listing ----

func createOne(t *testing.T, svc *usecase.JobOpeningService, title string) *domain.JobOpening {
	t.Helper()
	draft := validDraft()
	draft.Title = title
	o, err := svc.CreateJobOpening(context.Background(), draft)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	return o
}

// The Recruiter opens a job opening and sees its criteria and status.
func TestGetJobOpening_ReturnsStoredOpening(t *testing.T) {
	svc := newService(newFakeRepo())
	created := createOne(t, svc, "Backend Engineer")

	got, err := svc.GetJobOpening(context.Background(), created.ID)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != created.ID || got.Title != "Backend Engineer" || len(got.Criteria) != 2 {
		t.Errorf("got %+v, want the created opening", got)
	}
}

func TestGetJobOpening_UnknownIDIsNotFound(t *testing.T) {
	svc := newService(newFakeRepo())

	_, err := svc.GetJobOpening(context.Background(), "jo-404")

	if !errors.Is(err, usecase.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// The Recruiter sees every job opening, in the order they were created.
func TestListJobOpenings_ReturnsAllInCreationOrder(t *testing.T) {
	svc := newService(newFakeRepo())
	createOne(t, svc, "Backend Engineer")
	createOne(t, svc, "QA Engineer")

	list, err := svc.ListJobOpenings(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 || list[0].Title != "Backend Engineer" || list[1].Title != "QA Engineer" {
		t.Errorf("list = %+v, want both openings in creation order", list)
	}
}

// ---- reviseScreeningCriteria(), pauseOpening(), resumeOpening(), closeOpening() ----

// The Recruiter corrects the criteria after creation; the change is stored.
func TestReviseScreeningCriteria_PersistsNewCriteria(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	created := createOne(t, svc, "Backend Engineer")
	revised := []domain.Criterion{{Name: "English for daily standups", Classification: domain.NiceToHave, Weight: 2}}

	got, err := svc.ReviseScreeningCriteria(context.Background(), created.ID, revised)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Criteria) != 1 || repo.saved[created.ID].Criteria[0] != revised[0] {
		t.Errorf("stored criteria = %+v, want %+v", repo.saved[created.ID].Criteria, revised)
	}
}

// FR-1.8: pause, resume and close are stored, with the outcome recorded on close.
func TestLifecycle_PersistsEachTransition(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	ctx := context.Background()
	id := createOne(t, svc, "Backend Engineer").ID

	if _, err := svc.PauseOpening(ctx, id); err != nil {
		t.Fatalf("PauseOpening: %v", err)
	}
	if s := repo.saved[id].Status; s != domain.StatusPaused {
		t.Fatalf("after pause, stored Status = %q", s)
	}
	if _, err := svc.ResumeOpening(ctx, id); err != nil {
		t.Fatalf("ResumeOpening: %v", err)
	}
	if s := repo.saved[id].Status; s != domain.StatusOpen {
		t.Fatalf("after resume, stored Status = %q", s)
	}
	if _, err := svc.CloseOpening(ctx, id, domain.OutcomeFilled); err != nil {
		t.Fatalf("CloseOpening: %v", err)
	}
	if o := repo.saved[id]; o.Status != domain.StatusClosed || o.Outcome != domain.OutcomeFilled {
		t.Fatalf("after close, stored = %q/%q, want closed/filled", o.Status, o.Outcome)
	}
}

// Every update reports an unknown id as not found.
func TestUpdates_UnknownIDIsNotFound(t *testing.T) {
	svc := newService(newFakeRepo())
	ctx := context.Background()
	ops := map[string]func() error{
		"revise": func() error { _, err := svc.ReviseScreeningCriteria(ctx, "jo-404", validDraft().Criteria); return err },
		"pause":  func() error { _, err := svc.PauseOpening(ctx, "jo-404"); return err },
		"resume": func() error { _, err := svc.ResumeOpening(ctx, "jo-404"); return err },
		"close":  func() error { _, err := svc.CloseOpening(ctx, "jo-404", domain.OutcomeFilled); return err },
	}
	for name, op := range ops {
		if err := op(); !errors.Is(err, usecase.ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
}

// FR-1.9: a change the rules reject is reported, and the stored opening is left as it was.
func TestUpdates_RejectedChangeIsNotSaved(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo)
	ctx := context.Background()
	id := createOne(t, svc, "Backend Engineer").ID
	if _, err := svc.CloseOpening(ctx, id, domain.OutcomeCancelled); err != nil {
		t.Fatalf("setup: %v", err)
	}
	before := repo.saved[id]

	_, err := svc.ReviseScreeningCriteria(ctx, id, validDraft().Criteria)

	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
	if after := repo.saved[id]; after.Status != before.Status || len(after.Criteria) != len(before.Criteria) {
		t.Errorf("stored opening changed: before %+v, after %+v", before, after)
	}
}
