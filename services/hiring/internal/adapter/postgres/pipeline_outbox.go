package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frngrit/HireAssist/services/hiring/internal/domain"
)

// insertEvents adds pipeline events to the outbox inside the transaction of the change they
// report (ADR-015).
func insertEvents(ctx context.Context, tx pgx.Tx, events []domain.PipelineEvent) error {
	for _, e := range events {
		if _, err := tx.Exec(ctx, `
			INSERT INTO pipeline_outbox (type, occurred_at, job_opening_id, title, expected_time_to_fill_days, resume_id, candidate_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			string(e.Type), e.OccurredAt, e.JobOpeningID, nonEmpty(e.Title), nonZeroInt(e.ExpectedTimeToFillDays),
			nonEmpty(e.ResumeID), nonEmpty(e.CandidateID),
		); err != nil {
			return fmt.Errorf("recording pipeline event %s: %w", e.Type, err)
		}
	}
	return nil
}

// PipelineOutbox reads the outbox the repositories write to.
type PipelineOutbox struct {
	pool *pgxpool.Pool
}

// NewPipelineOutbox returns the outbox over an open connection pool.
func NewPipelineOutbox(pool *pgxpool.Pool) *PipelineOutbox {
	return &PipelineOutbox{pool: pool}
}

// Unpublished returns, oldest first, at most limit events not yet published.
func (o *PipelineOutbox) Unpublished(ctx context.Context, limit int) ([]domain.PipelineEvent, error) {
	rows, err := o.pool.Query(ctx, `
		SELECT event_id, type, occurred_at, job_opening_id, coalesce(title, ''), coalesce(expected_time_to_fill_days, 0),
		       coalesce(resume_id, ''), coalesce(candidate_id, '')
		FROM pipeline_outbox WHERE published_at IS NULL ORDER BY seq LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("reading the pipeline outbox: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.PipelineEvent, error) {
		var e domain.PipelineEvent
		err := row.Scan(&e.ID, &e.Type, &e.OccurredAt, &e.JobOpeningID, &e.Title, &e.ExpectedTimeToFillDays, &e.ResumeID, &e.CandidateID)
		e.OccurredAt = e.OccurredAt.UTC()
		return e, err
	})
}

// MarkPublished records that the broker confirmed an event.
func (o *PipelineOutbox) MarkPublished(ctx context.Context, eventID string, at time.Time) error {
	if _, err := o.pool.Exec(ctx, `UPDATE pipeline_outbox SET published_at = $2 WHERE event_id = $1`, eventID, at); err != nil {
		return fmt.Errorf("marking pipeline event %s published: %w", eventID, err)
	}
	return nil
}

func nonZeroInt(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}
