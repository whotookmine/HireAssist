// Package postgres implements every storage port of the Compliance & Insights Service in its
// own PostgreSQL database.
package postgres

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frngrit/HireAssist/services/compliance/internal/domain"
	"github.com/frngrit/HireAssist/services/compliance/internal/usecase"
)

//go:embed schema.sql
var schema string

// sweepLockKey is the advisory lock that keeps two replicas from sweeping at once (ADR-017).
const sweepLockKey = 0x48495245 // "HIRE"

// Store is safe for concurrent use.
type Store struct{ pool *pgxpool.Pool }

var (
	_ usecase.PolicyStore    = (*Store)(nil)
	_ usecase.PipelineStore  = (*Store)(nil)
	_ usecase.RetentionStore = (*Store)(nil)
	_ usecase.ErasureStore   = (*Store)(nil)
	_ usecase.AccessLog      = (*Store)(nil)
)

// Open connects and applies the schema.
func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connecting to PostgreSQL: %w", err)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("applying schema: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Ping reports whether the database answers, for the readiness check.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// WithSweepLock runs fn only if no other replica holds the sweep lock, and reports whether it
// ran. The lock is a transaction-level advisory lock, so it is released however fn ends.
func (s *Store) WithSweepLock(ctx context.Context, fn func(context.Context) error) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a no-op after commit
	var got bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, sweepLockKey).Scan(&got); err != nil || !got {
		return false, err
	}
	if err := fn(ctx); err != nil {
		return true, err
	}
	return true, tx.Commit(ctx)
}

// ---- retention policy ----

func (s *Store) GetPolicy(ctx context.Context) (*domain.RetentionPolicy, error) {
	var p domain.RetentionPolicy
	var anchor, action string
	err := s.pool.QueryRow(ctx, `SELECT period_days, anchor, action, warning_window_days, updated_at FROM retention_policy`).
		Scan(&p.PeriodDays, &anchor, &action, &p.WarningWindowDays, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	p.Anchor, p.Action = domain.Anchor(anchor), domain.ExpiryAction(action)
	return &p, err
}

func (s *Store) SavePolicy(ctx context.Context, p domain.RetentionPolicy) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO retention_policy (id, period_days, anchor, action, warning_window_days, updated_at)
		VALUES (true, $1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET period_days = EXCLUDED.period_days, anchor = EXCLUDED.anchor,
			action = EXCLUDED.action, warning_window_days = EXCLUDED.warning_window_days, updated_at = EXCLUDED.updated_at`,
		p.PeriodDays, string(p.Anchor), string(p.Action), p.WarningWindowDays, p.UpdatedAt)
	return err
}

// ---- pipeline projection ----

const openingColumns = `o.id, o.title, o.status, o.opened_at, o.expected_time_to_fill_days, o.recruiter, o.flagged_at,
	count(*) FILTER (WHERE e.stage = 'applied'), count(*) FILTER (WHERE e.stage = 'screened'),
	count(*) FILTER (WHERE e.stage = 'shortlisted')`

func scanOpening(row pgx.Row) (*domain.Opening, error) {
	var o domain.Opening
	var status string
	var openedAt *time.Time
	var applied, screened, shortlisted int
	if err := row.Scan(&o.ID, &o.Title, &status, &openedAt, &o.ExpectedTimeToFillDays, &o.Recruiter, &o.FlaggedAt,
		&applied, &screened, &shortlisted); err != nil {
		return nil, err
	}
	o.Status = domain.OpeningStatus(status)
	if openedAt != nil {
		o.OpenedAt = *openedAt
	}
	o.Counts = map[domain.Stage]int{domain.StageApplied: applied, domain.StageScreened: screened, domain.StageShortlisted: shortlisted}
	return &o, nil
}

func (s *Store) GetOpening(ctx context.Context, id string) (*domain.Opening, error) {
	o, err := scanOpening(s.pool.QueryRow(ctx, `SELECT `+openingColumns+`
		FROM openings o LEFT JOIN pipeline_entries e ON e.opening_id = o.id WHERE o.id = $1 GROUP BY o.id`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return o, err
}

func (s *Store) SaveOpening(ctx context.Context, o *domain.Opening) error {
	var openedAt *time.Time
	if !o.OpenedAt.IsZero() {
		openedAt = &o.OpenedAt
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO openings (id, title, status, opened_at, expected_time_to_fill_days, recruiter, flagged_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET title = EXCLUDED.title, status = EXCLUDED.status, opened_at = EXCLUDED.opened_at,
			expected_time_to_fill_days = EXCLUDED.expected_time_to_fill_days, recruiter = EXCLUDED.recruiter,
			flagged_at = CASE WHEN EXCLUDED.status = 'closed' THEN NULL ELSE openings.flagged_at END
		WHERE openings.status <> 'closed'`,
		o.ID, o.Title, string(o.Status), openedAt, o.ExpectedTimeToFillDays, o.Recruiter, nil)
	return err
}

func (s *Store) SetFlag(ctx context.Context, id string, flaggedAt *time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE openings SET flagged_at = $2 WHERE id = $1 AND status <> 'closed'`, id, flaggedAt)
	return err
}

func (s *Store) AddEntry(ctx context.Context, openingID string, stage domain.Stage, resumeID, candidateID string, at time.Time) error {
	var cand *string
	if candidateID != "" {
		cand = &candidateID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO pipeline_entries (opening_id, stage, resume_id, candidate_id, occurred_at) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (opening_id, stage, resume_id) DO NOTHING`, openingID, string(stage), resumeID, cand, at)
	return err
}

func (s *Store) ListOpenings(ctx context.Context) ([]domain.Opening, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+openingColumns+`
		FROM openings o LEFT JOIN pipeline_entries e ON e.opening_id = o.id
		WHERE o.status <> 'closed' GROUP BY o.id ORDER BY o.opened_at NULLS LAST, o.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Opening{}
	for rows.Next() {
		o, err := scanOpening(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}

func (s *Store) LastActivity(ctx context.Context, candidateID string) (time.Time, error) {
	var last *time.Time
	err := s.pool.QueryRow(ctx, `SELECT max(occurred_at) FROM pipeline_entries WHERE candidate_id = $1`, candidateID).Scan(&last)
	if err != nil || last == nil {
		return time.Time{}, err
	}
	return *last, nil
}

// ---- retention holds and marks ----

func (s *Store) SaveHold(ctx context.Context, h domain.Hold) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO retention_holds (candidate_id, job_opening_id, expires_at, held_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (candidate_id) DO UPDATE SET job_opening_id = EXCLUDED.job_opening_id, expires_at = EXCLUDED.expires_at`,
		h.CandidateID, h.JobOpeningID, h.ExpiresAt, h.HeldAt)
	return err
}

func (s *Store) GetHold(ctx context.Context, candidateID string) (*domain.Hold, error) {
	var h domain.Hold
	err := s.pool.QueryRow(ctx, `SELECT candidate_id, job_opening_id, expires_at, held_at FROM retention_holds WHERE candidate_id = $1`,
		candidateID).Scan(&h.CandidateID, &h.JobOpeningID, &h.ExpiresAt, &h.HeldAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &h, err
}

func (s *Store) ListHolds(ctx context.Context) ([]domain.Hold, error) {
	rows, err := s.pool.Query(ctx, `SELECT candidate_id, job_opening_id, expires_at, held_at FROM retention_holds ORDER BY expires_at`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Hold, error) {
		var h domain.Hold
		return h, r.Scan(&h.CandidateID, &h.JobOpeningID, &h.ExpiresAt, &h.HeldAt)
	})
}

func (s *Store) DeleteHold(ctx context.Context, candidateID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM retention_holds WHERE candidate_id = $1`, candidateID)
	return err
}

func (s *Store) Marks(ctx context.Context, candidateID string) (time.Time, time.Time, error) {
	var warned, keep *time.Time
	err := s.pool.QueryRow(ctx, `SELECT warned_for, keep_until FROM retention_marks WHERE candidate_id = $1`, candidateID).Scan(&warned, &keep)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, time.Time{}, nil
	}
	var w, k time.Time
	if warned != nil {
		w = *warned
	}
	if keep != nil {
		k = *keep
	}
	return w, k, err
}

func (s *Store) SetWarned(ctx context.Context, candidateID string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO retention_marks (candidate_id, warned_for) VALUES ($1, $2)
		ON CONFLICT (candidate_id) DO UPDATE SET warned_for = EXCLUDED.warned_for`, candidateID, expiresAt)
	return err
}

func (s *Store) SetKeepUntil(ctx context.Context, candidateID string, until time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO retention_marks (candidate_id, keep_until) VALUES ($1, $2)
		ON CONFLICT (candidate_id) DO UPDATE SET keep_until = EXCLUDED.keep_until`, candidateID, until)
	return err
}

// ---- erasures ----

const erasureColumns = `ref, candidate_id, reason, action, policy_days, requested_at, completed_at, status, done, attempts, last_error, last_tried_at`

func scanErasure(row pgx.Row) (domain.Erasure, error) {
	var e domain.Erasure
	var candidate *string
	var reason, action, status string
	var done []string
	err := row.Scan(&e.Ref, &candidate, &reason, &action, &e.PolicyDays, &e.RequestedAt, &e.CompletedAt, &status, &done,
		&e.Attempts, &e.LastError, &e.LastTriedAt)
	if candidate != nil {
		e.CandidateID = *candidate
	}
	e.Reason, e.Action, e.Status = domain.ErasureReason(reason), domain.ExpiryAction(action), domain.ErasureStatus(status)
	e.Done = map[domain.Holder]bool{}
	for _, h := range done {
		e.Done[domain.Holder(h)] = true
	}
	return e, err
}

func (s *Store) SaveErasure(ctx context.Context, e *domain.Erasure) error {
	var candidate *string
	if e.CandidateID != "" {
		candidate = &e.CandidateID
	}
	done := []string{}
	for _, h := range domain.Holders {
		if e.Done[h] {
			done = append(done, string(h))
		}
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO erasures (`+erasureColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (ref) DO UPDATE SET candidate_id = EXCLUDED.candidate_id, completed_at = EXCLUDED.completed_at,
			status = EXCLUDED.status, done = EXCLUDED.done, attempts = EXCLUDED.attempts,
			last_error = EXCLUDED.last_error, last_tried_at = EXCLUDED.last_tried_at`,
		e.Ref, candidate, string(e.Reason), string(e.Action), e.PolicyDays, e.RequestedAt, e.CompletedAt, string(e.Status),
		done, e.Attempts, e.LastError, e.LastTriedAt)
	return err
}

func (s *Store) GetErasure(ctx context.Context, ref string) (*domain.Erasure, error) {
	e, err := scanErasure(s.pool.QueryRow(ctx, `SELECT `+erasureColumns+` FROM erasures WHERE ref = $1`, ref))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, usecase.ErrNotFound
	}
	return &e, err
}

func (s *Store) PendingErasureFor(ctx context.Context, candidateID string) (*domain.Erasure, error) {
	e, err := scanErasure(s.pool.QueryRow(ctx, `SELECT `+erasureColumns+` FROM erasures WHERE candidate_id = $1 AND status = 'pending'`, candidateID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &e, err
}

func (s *Store) ListErasures(ctx context.Context, status domain.ErasureStatus) ([]domain.Erasure, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+erasureColumns+` FROM erasures WHERE $1 = '' OR status = $1 ORDER BY requested_at DESC`, string(status))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Erasure, error) { return scanErasure(r) })
}

// ForgetCandidate clears the candidate id everywhere this service keeps it, in one transaction.
func (s *Store) ForgetCandidate(ctx context.Context, candidateID string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		for _, q := range []string{
			`UPDATE pipeline_entries SET candidate_id = NULL WHERE candidate_id = $1`,
			`DELETE FROM retention_holds WHERE candidate_id = $1`,
			`DELETE FROM retention_marks WHERE candidate_id = $1`,
		} {
			if _, err := tx.Exec(ctx, q, candidateID); err != nil {
				return err
			}
		}
		return nil
	})
}

// ---- access audit log ----

func (s *Store) RecordRejection(ctx context.Context, r domain.AccessRejection) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO access_rejections (id, member_id, role, required_role, method, path, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, r.ID, r.MemberID, r.Role, r.RequiredRole, r.Method, r.Path, r.OccurredAt)
	return err
}

func (s *Store) ListRejections(ctx context.Context, limit int) ([]domain.AccessRejection, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, member_id, role, required_role, method, path, occurred_at
		FROM access_rejections ORDER BY occurred_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.AccessRejection, error) {
		var a domain.AccessRejection
		return a, r.Scan(&a.ID, &a.MemberID, &a.Role, &a.RequiredRole, &a.Method, &a.Path, &a.OccurredAt)
	})
}
