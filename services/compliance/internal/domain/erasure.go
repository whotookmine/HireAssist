package domain

import (
	"errors"
	"time"
)

// ErrInvalidErasure means an erasure request was refused.
var ErrInvalidErasure = errors.New("invalid erasure request")

// Holder is a service that holds part of a candidate's personal data and must erase it (FR-5.3).
type Holder string

const (
	// HolderHiring holds the screening results, decisions, interview guides and the resume file.
	HolderHiring Holder = "hiring"
	// HolderResumeProcessing holds the candidate profile, the parsed text and the justifications.
	HolderResumeProcessing Holder = "resume_processing"
)

// Holders is every holder an erasure must reach, in the order they are asked.
var Holders = []Holder{HolderHiring, HolderResumeProcessing}

// ErasureReason is why an erasure was started.
type ErasureReason string

const (
	ReasonExpired      ErasureReason = "expired"       // the scheduled retention sweep (FR-5.3)
	ReasonRequested    ErasureReason = "requested"     // an Admin, for a named candidate (FR-5.7)
	ReasonHoldResolved ErasureReason = "hold_resolved" // a Recruiter decided a held expiry (FR-5.6)
)

// ErasureStatus is where an erasure stands. It is erased only once every holder has confirmed;
// until then it is pending, and reported as such (FR-5.8).
type ErasureStatus string

const (
	ErasurePending ErasureStatus = "pending"
	ErasureErased  ErasureStatus = "erased"
)

// Erasure is one candidate's erasure and, once finished, its audit entry (FR-5.5). It is
// identified by a random reference. The candidate id is kept only while the erasure is pending,
// because it is needed to retry; once every holder has confirmed it is removed, and the
// reference can no longer be resolved to a person.
type Erasure struct {
	Ref         string
	CandidateID string // empty once erased
	Reason      ErasureReason
	Action      ExpiryAction
	PolicyDays  int // the retention period in force, or zero for a request outside a policy
	RequestedAt time.Time
	CompletedAt *time.Time
	Status      ErasureStatus
	Done        map[Holder]bool
	Attempts    int
	LastError   string // which holder failed and why; never personal data
	LastTriedAt *time.Time
}

// NewErasure starts an erasure.
func NewErasure(ref, candidateID string, reason ErasureReason, policy *RetentionPolicy, now time.Time) (*Erasure, error) {
	if candidateID == "" {
		return nil, errors.Join(ErrInvalidErasure, errors.New("candidate_id is required"))
	}
	e := &Erasure{Ref: ref, CandidateID: candidateID, Reason: reason, Action: ActionDelete,
		RequestedAt: now, Status: ErasurePending, Done: map[Holder]bool{}}
	if policy != nil {
		e.Action, e.PolicyDays = policy.Action, policy.PeriodDays
	}
	return e, nil
}

// Outstanding is the holders that have not confirmed yet.
func (e *Erasure) Outstanding() []Holder {
	var out []Holder
	for _, h := range Holders {
		if !e.Done[h] {
			out = append(out, h)
		}
	}
	return out
}

// Record notes one attempt: which holders confirmed, and the failure if there was one. When the
// last holder confirms, the erasure is finished and forgets whom it was about.
func (e *Erasure) Record(confirmed []Holder, failure string, now time.Time) {
	for _, h := range confirmed {
		e.Done[h] = true
	}
	e.Attempts++
	e.LastTriedAt = &now
	e.LastError = failure
	if len(e.Outstanding()) == 0 {
		e.Status, e.CompletedAt, e.CandidateID, e.LastError = ErasureErased, &now, "", ""
	}
}

// Hold is an expiry the sweep did not carry out because the candidate is in an active hiring
// process; a Recruiter decides it (FR-5.6).
type Hold struct {
	CandidateID  string
	JobOpeningID string
	ExpiresAt    time.Time
	HeldAt       time.Time
}

// HoldDecision is the Recruiter's answer to a hold.
type HoldDecision string

const (
	DecisionErase HoldDecision = "erase" // erase now
	DecisionKeep  HoldDecision = "keep"  // keep for now; asked again after the warning window
)

// AccessRejection is one request the gateway refused for want of a role (FR-0.4, NFR-12). It
// names the member by id only.
type AccessRejection struct {
	ID           string
	MemberID     string
	Role         string
	RequiredRole string
	Method       string
	Path         string
	OccurredAt   time.Time
}
