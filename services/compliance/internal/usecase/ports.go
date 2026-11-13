// Package usecase holds the Compliance & Insights Service's operations and declares the ports
// they need. Adapters implement the ports; nothing here knows HTTP, SQL or AMQP.
package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/frngrit/HireAssist/services/compliance/internal/domain"
)

// ErrNotFound means the thing asked for does not exist.
var ErrNotFound = errors.New("not found")

// Clock tells the time; tests set it.
type Clock interface{ Now() time.Time }

// IDs makes the random references erasures and audit entries are known by.
type IDs interface{ NewID(prefix string) string }

// PolicyStore keeps the deployment's retention policy.
type PolicyStore interface {
	// GetPolicy returns nil, without error, when no policy has been set.
	GetPolicy(ctx context.Context) (*domain.RetentionPolicy, error)
	SavePolicy(ctx context.Context, p domain.RetentionPolicy) error
}

// PipelineStore keeps the projection of job openings and their entries.
type PipelineStore interface {
	// GetOpening returns nil, without error, for an opening no event has named.
	GetOpening(ctx context.Context, id string) (*domain.Opening, error)
	// SaveOpening stores what an event changed. It leaves the stale flag alone — only SetFlag and
	// a close change it — so an event and a sweep running at once cannot undo each other.
	SaveOpening(ctx context.Context, o *domain.Opening) error
	// SetFlag sets or clears the stale flag of an opening that is not closed.
	SetFlag(ctx context.Context, id string, flaggedAt *time.Time) error
	// AddEntry records that a resume reached a stage. Recording the same one twice is harmless.
	AddEntry(ctx context.Context, openingID string, stage domain.Stage, resumeID, candidateID string, at time.Time) error
	// ListOpenings returns every opening that is not closed, with its counts.
	ListOpenings(ctx context.Context) ([]domain.Opening, error)
	// LastActivity is the latest event naming the candidate, or zero.
	LastActivity(ctx context.Context, candidateID string) (time.Time, error)
}

// RetentionStore keeps what the retention sweep has decided about each candidate.
type RetentionStore interface {
	SaveHold(ctx context.Context, h domain.Hold) error
	GetHold(ctx context.Context, candidateID string) (*domain.Hold, error)
	ListHolds(ctx context.Context) ([]domain.Hold, error)
	DeleteHold(ctx context.Context, candidateID string) error
	// Marks says for which expiry date the Recruiter was last warned, and until when a held
	// expiry was decided "keep". Zero values when neither has happened.
	Marks(ctx context.Context, candidateID string) (warnedFor, keepUntil time.Time, err error)
	SetWarned(ctx context.Context, candidateID string, expiresAt time.Time) error
	SetKeepUntil(ctx context.Context, candidateID string, until time.Time) error
}

// ErasureStore keeps erasures, which are also the erasure audit log (FR-5.5).
type ErasureStore interface {
	SaveErasure(ctx context.Context, e *domain.Erasure) error
	GetErasure(ctx context.Context, ref string) (*domain.Erasure, error)
	// PendingErasureFor returns the candidate's unfinished erasure, or nil.
	PendingErasureFor(ctx context.Context, candidateID string) (*domain.Erasure, error)
	ListErasures(ctx context.Context, status domain.ErasureStatus) ([]domain.Erasure, error)
	// ForgetCandidate removes the candidate id from everything this service keeps — activity,
	// holds, marks — so that nothing here resolves to the person once they are erased. The
	// pipeline counts survive, which is what anonymisation keeps (FR-5.4).
	ForgetCandidate(ctx context.Context, candidateID string) error
}

// AccessLog keeps the access audit log (FR-0.4, NFR-12).
type AccessLog interface {
	RecordRejection(ctx context.Context, r domain.AccessRejection) error
	ListRejections(ctx context.Context, limit int) ([]domain.AccessRejection, error)
}

// RegisteredCandidate is one entry of the candidate register in Resume Processing
// (listCandidateProfiles()).
type RegisteredCandidate struct {
	ID           string
	JobOpeningID string
	CollectedAt  time.Time
}

// CandidateRegister lists every candidate the deployment holds.
type CandidateRegister interface {
	ListCandidates(ctx context.Context) ([]RegisteredCandidate, error)
}

// HolderEraser asks one holder to erase a candidate's data. It returns nil once the holder
// holds nothing about the candidate, including when it held nothing to begin with.
type HolderEraser interface {
	EraseCandidate(ctx context.Context, candidateID string) error
}

// Notifier sends one notice to one person (FR-2.12, FR-4.4, FR-5.2).
type Notifier interface {
	Notify(ctx context.Context, to, subject, body string) error
}
