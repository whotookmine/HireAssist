package postgres_test

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frngrit/HireAssist/services/hiring/internal/adapter/postgres"
	"github.com/frngrit/HireAssist/services/hiring/internal/domain"
	"github.com/frngrit/HireAssist/services/hiring/internal/usecase"
)

// Compile-time check: the adapter satisfies the use case's port.
var _ usecase.JobOpeningRepository = (*postgres.JobOpeningRepository)(nil)

// newRepo connects to the database named by HIRING_TEST_DATABASE_URL, applies the schema and
// empties the tables. Without the variable the test is skipped, so `go test ./...` needs no
// database; `docker compose up -d postgres` provides one.
func newRepo(t *testing.T) *postgres.JobOpeningRepository {
	t.Helper()
	url := os.Getenv("HIRING_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("HIRING_TEST_DATABASE_URL not set; skipping PostgreSQL integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE job_openings, pipeline_outbox CASCADE"); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	return postgres.NewJobOpeningRepository(pool)
}

func opening(id, title string) *domain.JobOpening {
	return &domain.JobOpening{
		ID:          id,
		Title:       title,
		Description: "Backend engineer, 3+ years of Go.",
		Criteria: []domain.Criterion{
			{Name: "Go", Classification: domain.MustHave, Weight: 3},
			{Name: "Kafka", Classification: domain.NiceToHave, Weight: 1},
		},
		ExpectedTimeToFill: 45 * 24 * time.Hour,
		Status:             domain.StatusOpen,
		OpenedAt:           time.Date(2026, 9, 26, 8, 30, 15, 123456000, time.UTC),
	}
}

// Every field survives the round trip, including criteria order and the UTC opening time.
func TestRepository_SaveThenFindRoundTripsEveryField(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	want := opening("jo-1", "Backend Engineer")
	if err := repo.Save(ctx, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.FindByID(ctx, "jo-1")

	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip:\n got  %+v\n want %+v", got, want)
	}
}

func TestRepository_UnknownIDIsNotFound(t *testing.T) {
	_, err := newRepo(t).FindByID(context.Background(), "jo-404")

	if !errors.Is(err, usecase.ErrNotFound) {
		t.Fatalf("err = %v, want usecase.ErrNotFound", err)
	}
}

// Saving an existing id replaces it: status, outcome and the whole criteria list.
func TestRepository_SaveReplacesAnExistingOpening(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	_ = repo.Save(ctx, opening("jo-1", "Backend Engineer"))
	changed := opening("jo-1", "Senior Backend Engineer")
	changed.Criteria = []domain.Criterion{{Name: "Rust", Classification: domain.MustHave, Weight: 5}}
	changed.Status = domain.StatusClosed
	changed.Outcome = domain.OutcomeFilled
	if err := repo.Save(ctx, changed); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, _ := repo.FindByID(ctx, "jo-1")

	if !reflect.DeepEqual(got, changed) {
		t.Errorf("after update:\n got  %+v\n want %+v", got, changed)
	}
}

// Updating an opening does not move it in the list.
func TestRepository_ListKeepsCreationOrderAcrossUpdates(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	_ = repo.Save(ctx, opening("jo-1", "Backend Engineer"))
	_ = repo.Save(ctx, opening("jo-2", "QA Engineer"))
	_ = repo.Save(ctx, opening("jo-1", "Senior Backend Engineer"))

	list, err := repo.List(ctx)

	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 || list[0].Title != "Senior Backend Engineer" || list[1].Title != "QA Engineer" {
		t.Fatalf("list = %+v", list)
	}
	if !reflect.DeepEqual(list[1].Criteria, opening("", "").Criteria) {
		t.Errorf("listed criteria = %+v", list[1].Criteria)
	}
}

func TestRepository_EmptyListIsEmptyNotNil(t *testing.T) {
	list, err := newRepo(t).List(context.Background())

	if err != nil || list == nil || len(list) != 0 {
		t.Errorf("List = %#v, %v; want empty, non-nil", list, err)
	}
}

// Migrating twice is harmless, because the service migrates on every start.
func TestMigrate_IsIdempotent(t *testing.T) {
	newRepo(t)
	newRepo(t)
}
