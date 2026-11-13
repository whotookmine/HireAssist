package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/frngrit/HireAssist/services/compliance/internal/domain"
)

// Erasures orchestrates a candidate's erasure across every service that holds their data
// (ADR-016): it asks each holder in turn, records which confirmed, and keeps the erasure
// pending — retried by every sweep — until all have (FR-5.3, FR-5.8).
type Erasures struct {
	Store   ErasureStore
	Holders map[domain.Holder]HolderEraser
	IDs     IDs
	Clock   Clock
}

// EraseCandidate erases one candidate now (eraseCandidate()). Asking again for a candidate whose
// erasure is still pending retries that erasure rather than starting a second one.
func (s *Erasures) EraseCandidate(ctx context.Context, candidateID string, reason domain.ErasureReason, policy *domain.RetentionPolicy) (*domain.Erasure, error) {
	candidateID = strings.TrimSpace(candidateID)
	e, err := s.Store.PendingErasureFor(ctx, candidateID)
	if err != nil {
		return nil, err
	}
	if e == nil {
		if e, err = domain.NewErasure(s.IDs.NewID("era"), candidateID, reason, policy, s.Clock.Now()); err != nil {
			return nil, err
		}
		if err := s.Store.SaveErasure(ctx, e); err != nil {
			return nil, err
		}
	}
	return e, s.attempt(ctx, e)
}

// RetryPending retries every unfinished erasure, and returns how many finished.
func (s *Erasures) RetryPending(ctx context.Context) (finished int, err error) {
	pending, err := s.Store.ListErasures(ctx, domain.ErasurePending)
	if err != nil {
		return 0, err
	}
	for i := range pending {
		if err := s.attempt(ctx, &pending[i]); err != nil {
			return finished, err
		}
		if pending[i].Status == domain.ErasureErased {
			finished++
		}
	}
	return finished, nil
}

// attempt asks every holder that has not confirmed. A holder that fails does not stop the
// others; the erasure stays pending with the failure recorded, and the next sweep asks again.
// The candidate is forgotten here before the erasure is saved as finished, so a crash between
// the two leaves a pending erasure that finishes on retry, never a finished one that still
// names the person.
func (s *Erasures) attempt(ctx context.Context, e *domain.Erasure) error {
	var confirmed []domain.Holder
	var failures []string
	for _, h := range e.Outstanding() {
		eraser, ok := s.Holders[h]
		if !ok {
			failures = append(failures, fmt.Sprintf("%s: not configured", h))
			continue
		}
		if err := eraser.EraseCandidate(ctx, e.CandidateID); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", h, err))
			continue
		}
		confirmed = append(confirmed, h)
	}
	if len(failures) == 0 {
		if err := s.Store.ForgetCandidate(ctx, e.CandidateID); err != nil {
			return err
		}
	}
	e.Record(confirmed, strings.Join(failures, "; "), s.Clock.Now())
	if e.Status == domain.ErasurePending {
		slog.Warn("erasure pending; it will be retried", "erasure", e.Ref, "attempts", e.Attempts, "failure", e.LastError)
	} else {
		slog.Info("candidate erased", "erasure", e.Ref, "reason", e.Reason)
	}
	return s.Store.SaveErasure(ctx, e)
}

// GetErasure returns one erasure audit entry (getErasureAudit()).
func (s *Erasures) GetErasure(ctx context.Context, ref string) (*domain.Erasure, error) {
	return s.Store.GetErasure(ctx, ref)
}

// ListErasures returns the erasure audit log, optionally only one status.
func (s *Erasures) ListErasures(ctx context.Context, status domain.ErasureStatus) ([]domain.Erasure, error) {
	return s.Store.ListErasures(ctx, status)
}

// Retention keeps the retention policy and runs the retention sweep (UC-5).
type Retention struct {
	Policies PolicyStore
	Marks    RetentionStore
	Pipeline PipelineStore
	Register CandidateRegister
	Erasures *Erasures
	Notifier Notifier
	Clock    Clock
	// FallbackRecipient receives warnings about candidates whose Recruiter is not known.
	FallbackRecipient string
}

// SetRetentionPolicy replaces the deployment's policy (setRetentionPolicy(), FR-5.1).
func (s *Retention) SetRetentionPolicy(ctx context.Context, periodDays int, anchor domain.Anchor, action domain.ExpiryAction, warningDays int) (*domain.RetentionPolicy, error) {
	p, err := domain.NewRetentionPolicy(periodDays, anchor, action, warningDays, s.Clock.Now())
	if err != nil {
		return nil, err
	}
	return &p, s.Policies.SavePolicy(ctx, p)
}

// GetRetentionPolicy returns the policy in force, or ErrNotFound when none has been set.
func (s *Retention) GetRetentionPolicy(ctx context.Context) (*domain.RetentionPolicy, error) {
	p, err := s.Policies.GetPolicy(ctx)
	if err == nil && p == nil {
		err = ErrNotFound
	}
	return p, err
}

// RetentionReport is what one retention sweep did. Candidates are named by internal id only.
type RetentionReport struct {
	PolicySet       bool     `json:"policy_set"`
	Evaluated       int      `json:"evaluated"`
	Warned          []string `json:"warned"`
	Erasures        []string `json:"erasures"` // references of the erasures started
	Held            []string `json:"held"`
	RetriedFinished int      `json:"retried_erasures_finished"`
}

// EvaluateRetention runs the retention sweep (evaluateRetention(), FR-5.2, FR-5.3, FR-5.6):
// retry unfinished erasures, then judge every registered candidate against the policy. With no
// policy set it only retries.
func (s *Retention) EvaluateRetention(ctx context.Context) (RetentionReport, error) {
	report := RetentionReport{Warned: []string{}, Erasures: []string{}, Held: []string{}}
	finished, err := s.Erasures.RetryPending(ctx)
	report.RetriedFinished = finished
	if err != nil {
		return report, err
	}
	policy, err := s.Policies.GetPolicy(ctx)
	if err != nil || policy == nil {
		return report, err
	}
	report.PolicySet = true

	registered, err := s.Register.ListCandidates(ctx)
	if err != nil {
		return report, fmt.Errorf("listing candidates: %w", err)
	}
	now := s.Clock.Now()
	warnings := map[string][]string{} // recipient → lines
	for _, rc := range registered {
		c, recruiter, err := s.candidate(ctx, rc)
		if err != nil {
			return report, err
		}
		report.Evaluated++
		expires := policy.ExpiresAt(c)
		warnedFor, keepUntil, err := s.Marks.Marks(ctx, c.ID)
		if err != nil {
			return report, err
		}
		state := policy.Evaluate(c, now)
		if state != domain.Held {
			if err := s.Marks.DeleteHold(ctx, c.ID); err != nil {
				return report, err
			}
		}
		switch state {
		case domain.Warning:
			if !warnedFor.Equal(expires) {
				to := recipient(recruiter, s.FallbackRecipient)
				warnings[to] = append(warnings[to], fmt.Sprintf("- %s (job opening %s): expires %s", c.ID, orNone(c.JobOpeningID), expires.Format("2006-01-02")))
				if err := s.Marks.SetWarned(ctx, c.ID, expires); err != nil {
					return report, err
				}
				report.Warned = append(report.Warned, c.ID)
			}
		case domain.Expired:
			e, err := s.Erasures.EraseCandidate(ctx, c.ID, domain.ReasonExpired, policy)
			if err != nil {
				return report, err
			}
			report.Erasures = append(report.Erasures, e.Ref)
		case domain.Held:
			if now.Before(keepUntil) {
				continue // a Recruiter decided to keep it, and the decision still stands
			}
			if h, err := s.Marks.GetHold(ctx, c.ID); err != nil {
				return report, err
			} else if h == nil {
				if err := s.Marks.SaveHold(ctx, domain.Hold{CandidateID: c.ID, JobOpeningID: c.JobOpeningID, ExpiresAt: expires, HeldAt: now}); err != nil {
					return report, err
				}
			}
			report.Held = append(report.Held, c.ID)
		}
	}
	s.sendWarnings(ctx, warnings, policy)
	return report, nil
}

// candidate builds what retention knows about one registered candidate. A candidate linked to
// an opening this service has no record of is treated as in an active process: when the
// pipeline events are missing, the safe mistake is to ask a Recruiter, not to erase.
func (s *Retention) candidate(ctx context.Context, rc RegisteredCandidate) (domain.Candidate, string, error) {
	c := domain.Candidate{ID: rc.ID, JobOpeningID: rc.JobOpeningID, CollectedAt: rc.CollectedAt}
	last, err := s.Pipeline.LastActivity(ctx, rc.ID)
	if err != nil {
		return c, "", err
	}
	c.LastActivityAt = last
	if rc.JobOpeningID == "" {
		return c, "", nil
	}
	o, err := s.Pipeline.GetOpening(ctx, rc.JobOpeningID)
	if err != nil {
		return c, "", err
	}
	if o == nil {
		c.InActiveProcess = true
		return c, "", nil
	}
	c.InActiveProcess = o.Status != domain.StatusClosed
	return c, o.Recruiter, nil
}

func (s *Retention) sendWarnings(ctx context.Context, warnings map[string][]string, p *domain.RetentionPolicy) {
	recipients := make([]string, 0, len(warnings))
	for to := range warnings {
		recipients = append(recipients, to)
	}
	sort.Strings(recipients)
	for _, to := range recipients {
		lines := warnings[to]
		body := fmt.Sprintf("These candidates' personal data reaches the end of its %d-day retention period soon, and will then be %s:\n\n%s\n",
			p.PeriodDays, map[domain.ExpiryAction]string{domain.ActionDelete: "deleted", domain.ActionAnonymise: "anonymised"}[p.Action], strings.Join(lines, "\n"))
		sendNotice(ctx, s.Notifier, to, fmt.Sprintf("%d candidate(s) approaching retention expiry", len(lines)), body)
	}
}

// ListHeldExpiries returns the expiries waiting for a Recruiter's decision (listHeldExpiries()).
func (s *Retention) ListHeldExpiries(ctx context.Context) ([]domain.Hold, error) {
	return s.Marks.ListHolds(ctx)
}

// ErrNoPolicy means an operation needs a retention policy and none is set.
var ErrNoPolicy = errors.New("no retention policy is set")

// ResolveHeldExpiry applies a Recruiter's decision to a held expiry (resolveHeldExpiry()).
// "erase" erases now; "keep" keeps the data and asks again once the warning window has passed.
func (s *Retention) ResolveHeldExpiry(ctx context.Context, candidateID string, d domain.HoldDecision) (*domain.Erasure, error) {
	h, err := s.Marks.GetHold(ctx, candidateID)
	if err != nil {
		return nil, err
	}
	if h == nil {
		return nil, ErrNotFound
	}
	policy, err := s.Policies.GetPolicy(ctx)
	if err != nil {
		return nil, err
	}
	switch d {
	case domain.DecisionErase:
		return s.Erasures.EraseCandidate(ctx, candidateID, domain.ReasonHoldResolved, policy)
	case domain.DecisionKeep:
		if policy == nil {
			return nil, ErrNoPolicy
		}
		window := max(policy.WarningWindowDays, 1)
		if err := s.Marks.SetKeepUntil(ctx, candidateID, s.Clock.Now().Add(time.Duration(window)*domain.Day)); err != nil {
			return nil, err
		}
		return nil, s.Marks.DeleteHold(ctx, candidateID)
	}
	return nil, fmt.Errorf("%w: decision must be %q or %q", domain.ErrInvalidErasure, domain.DecisionErase, domain.DecisionKeep)
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
