// Package domain holds the Compliance & Insights Service's rules: the retention policy and how a
// candidate is judged against it (UC-5), and the pipeline view of a job opening with its
// staleness rule (UC-4). It imports nothing from the rest of the service.
package domain

import (
	"errors"
	"fmt"
	"time"
)

// Day is the unit retention periods, warning windows and time-to-fill are stated in.
const Day = 24 * time.Hour

// ErrInvalidPolicy means a retention policy was refused.
var ErrInvalidPolicy = errors.New("invalid retention policy")

// Anchor is the date a candidate's retention period is counted from (FR-5.1).
type Anchor string

const (
	AnchorCollected    Anchor = "collected"     // the date the candidate's data was collected
	AnchorLastActivity Anchor = "last_activity" // the date of the latest pipeline event about them
)

// ExpiryAction is what happens to a candidate's personal data when it expires (FR-5.1).
type ExpiryAction string

const (
	ActionDelete    ExpiryAction = "delete"
	ActionAnonymise ExpiryAction = "anonymise"
)

// RetentionPolicy is the deployment's one retention policy, set by an Admin (FR-5.1). There is
// no default: until one is set, nothing expires, because erasing personal data under a period
// nobody chose is worse than keeping it until someone does.
type RetentionPolicy struct {
	PeriodDays        int
	Anchor            Anchor
	Action            ExpiryAction
	WarningWindowDays int // how long before expiry the responsible Recruiter is warned (FR-5.2)
	UpdatedAt         time.Time
}

// NewRetentionPolicy validates a policy.
func NewRetentionPolicy(periodDays int, anchor Anchor, action ExpiryAction, warningDays int, now time.Time) (RetentionPolicy, error) {
	switch {
	case periodDays <= 0:
		return RetentionPolicy{}, fmt.Errorf("%w: the retention period must be at least one day", ErrInvalidPolicy)
	case anchor != AnchorCollected && anchor != AnchorLastActivity:
		return RetentionPolicy{}, fmt.Errorf("%w: anchor must be %q or %q", ErrInvalidPolicy, AnchorCollected, AnchorLastActivity)
	case action != ActionDelete && action != ActionAnonymise:
		return RetentionPolicy{}, fmt.Errorf("%w: expiry action must be %q or %q", ErrInvalidPolicy, ActionDelete, ActionAnonymise)
	case warningDays < 0 || warningDays >= periodDays:
		return RetentionPolicy{}, fmt.Errorf("%w: the warning window must be zero or more days and shorter than the period", ErrInvalidPolicy)
	}
	return RetentionPolicy{PeriodDays: periodDays, Anchor: anchor, Action: action, WarningWindowDays: warningDays, UpdatedAt: now}, nil
}

// Candidate is what retention needs to know about one candidate. CollectedAt comes from the
// candidate register in Resume Processing; LastActivityAt and InActiveProcess come from the
// pipeline events this service has received.
type Candidate struct {
	ID              string
	JobOpeningID    string
	CollectedAt     time.Time
	LastActivityAt  time.Time // zero when no pipeline event has named the candidate
	InActiveProcess bool      // linked to a job opening that is not closed (FR-5.6)
}

// RetentionState is where a candidate stands against the policy.
type RetentionState string

const (
	Retained RetentionState = "retained" // not yet in the warning window
	Warning  RetentionState = "warning"  // inside the warning window: the Recruiter is told (FR-5.2)
	Expired  RetentionState = "expired"  // past expiry: erase (FR-5.3)
	Held     RetentionState = "held"     // past expiry, but in an active hiring process (FR-5.6)
)

// AnchorDate is the date the candidate's period runs from.
func (p RetentionPolicy) AnchorDate(c Candidate) time.Time {
	if p.Anchor == AnchorLastActivity && c.LastActivityAt.After(c.CollectedAt) {
		return c.LastActivityAt
	}
	return c.CollectedAt
}

// ExpiresAt is the moment the candidate's data expires.
func (p RetentionPolicy) ExpiresAt(c Candidate) time.Time {
	return p.AnchorDate(c).Add(time.Duration(p.PeriodDays) * Day)
}

// Evaluate judges one candidate at a moment.
func (p RetentionPolicy) Evaluate(c Candidate, now time.Time) RetentionState {
	expires := p.ExpiresAt(c)
	switch {
	case !now.Before(expires) && c.InActiveProcess:
		return Held
	case !now.Before(expires):
		return Expired
	case !now.Before(expires.Add(-time.Duration(p.WarningWindowDays) * Day)):
		return Warning
	default:
		return Retained
	}
}
