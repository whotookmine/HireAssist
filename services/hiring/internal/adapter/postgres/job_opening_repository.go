// Package postgres is the PostgreSQL adapter for the job opening repository port — the
// Hiring Service's system of record (ADR-003).
package postgres

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frngrit/HireAssist/services/hiring/internal/domain"
	"github.com/frngrit/HireAssist/services/hiring/internal/usecase"
)

//go:embed schema.sql
var schema string

// Migrate creates the service's tables if they do not exist. It is safe to run on every start.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("applying schema: %w", err)
	}
	return nil
}

// JobOpeningRepository stores job openings in PostgreSQL: one row per opening, one row per
// criterion. It is safe for concurrent use; concurrent saves of the same opening are last-write-wins.
type JobOpeningRepository struct {
	pool *pgxpool.Pool
}

// NewJobOpeningRepository returns a repository over an open connection pool.
func NewJobOpeningRepository(pool *pgxpool.Pool) *JobOpeningRepository {
	return &JobOpeningRepository{pool: pool}
}

// Save inserts a new job opening or replaces an existing one with the same id, criteria
// included, in one transaction.
func (r *JobOpeningRepository) Save(ctx context.Context, o *domain.JobOpening, events ...domain.PipelineEvent) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var outcome *string
		if o.Outcome != "" {
			s := string(o.Outcome)
			outcome = &s
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO job_openings (id, title, description, expected_time_to_fill, status, opened_at, outcome)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET
				title = EXCLUDED.title,
				description = EXCLUDED.description,
				expected_time_to_fill = EXCLUDED.expected_time_to_fill,
				status = EXCLUDED.status,
				opened_at = EXCLUDED.opened_at,
				outcome = EXCLUDED.outcome`,
			o.ID, o.Title, o.Description, interval(o.ExpectedTimeToFill), string(o.Status), o.OpenedAt, outcome,
		); err != nil {
			return fmt.Errorf("saving job opening %s: %w", o.ID, err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM job_opening_criteria WHERE job_opening_id = $1`, o.ID); err != nil {
			return fmt.Errorf("replacing criteria of %s: %w", o.ID, err)
		}
		rows := make([][]any, len(o.Criteria))
		for i, c := range o.Criteria {
			rows[i] = []any{o.ID, i, c.Name, string(c.Classification), c.Weight}
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"job_opening_criteria"},
			[]string{"job_opening_id", "position", "name", "classification", "weight"},
			pgx.CopyFromRows(rows),
		); err != nil {
			return fmt.Errorf("saving criteria of %s: %w", o.ID, err)
		}
		return insertEvents(ctx, tx, events)
	})
}

// FindByID returns the job opening, or usecase.ErrNotFound.
func (r *JobOpeningRepository) FindByID(ctx context.Context, id string) (*domain.JobOpening, error) {
	openings, err := r.query(ctx, `WHERE o.id = $1`, id)
	if err != nil {
		return nil, err
	}
	if len(openings) == 0 {
		return nil, usecase.ErrNotFound
	}
	return &openings[0], nil
}

// List returns every job opening in creation order.
func (r *JobOpeningRepository) List(ctx context.Context) ([]domain.JobOpening, error) {
	return r.query(ctx, ``)
}

// query loads openings matching where, each with its criteria in their saved order.
func (r *JobOpeningRepository) query(ctx context.Context, where string, args ...any) ([]domain.JobOpening, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT o.id, o.title, o.description, o.expected_time_to_fill, o.status, o.opened_at,
		       coalesce(o.outcome, ''), c.name, c.classification, c.weight
		FROM job_openings o
		LEFT JOIN job_opening_criteria c ON c.job_opening_id = o.id
		`+where+`
		ORDER BY o.seq, c.position`, args...)
	if err != nil {
		return nil, fmt.Errorf("loading job openings: %w", err)
	}
	defer rows.Close()

	openings := []domain.JobOpening{}
	for rows.Next() {
		var (
			o                    domain.JobOpening
			ttf                  pgtype.Interval
			name, classification *string
			weight               *int
		)
		if err := rows.Scan(&o.ID, &o.Title, &o.Description, &ttf, &o.Status, &o.OpenedAt,
			&o.Outcome, &name, &classification, &weight); err != nil {
			return nil, fmt.Errorf("reading job opening: %w", err)
		}
		if n := len(openings); n == 0 || openings[n-1].ID != o.ID {
			o.ExpectedTimeToFill = duration(ttf)
			o.OpenedAt = o.OpenedAt.UTC()
			openings = append(openings, o)
		}
		if name != nil {
			last := &openings[len(openings)-1]
			last.Criteria = append(last.Criteria, domain.Criterion{
				Name: *name, Classification: domain.Classification(*classification), Weight: *weight,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("loading job openings: %w", err)
	}
	return openings, nil
}

// interval stores a duration exactly, as microseconds with no day or month component.
func interval(d time.Duration) pgtype.Interval {
	return pgtype.Interval{Microseconds: d.Microseconds(), Valid: true}
}

// duration converts a stored interval back. Days count as 24 hours; this adapter never writes months.
func duration(i pgtype.Interval) time.Duration {
	return time.Duration(i.Microseconds)*time.Microsecond + time.Duration(i.Days)*24*time.Hour
}
