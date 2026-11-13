package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/frngrit/HireAssist/services/compliance/internal/domain"
)

// Pipeline maintains the dashboard from pipeline events and flags stale openings (UC-4).
type Pipeline struct {
	Store    PipelineStore
	Notifier Notifier
	Clock    Clock
	// FallbackRecipient receives notices for an opening whose Recruiter is not known.
	FallbackRecipient string
}

// RecordPipelineEvent applies one event to the projection (recordPipelineEvent()). Applying the
// same event twice changes nothing, so a redelivered message is harmless.
func (s *Pipeline) RecordPipelineEvent(ctx context.Context, e domain.PipelineEvent) error {
	if err := e.Validate(); err != nil {
		return err
	}
	o, err := s.Store.GetOpening(ctx, e.JobOpeningID)
	if err != nil {
		return err
	}
	if o == nil {
		// An entry may name an opening whose opened event this service never saw — it was
		// opened before Compliance existed. Keep it as an open opening with no details yet.
		o = &domain.Opening{ID: e.JobOpeningID, Status: domain.StatusOpen}
	}
	if stage, ok := e.Stage(); ok {
		if err := s.Store.SaveOpening(ctx, o); err != nil {
			return err
		}
		return s.Store.AddEntry(ctx, e.JobOpeningID, stage, e.ResumeID, e.CandidateID, e.OccurredAt)
	}
	o.Apply(e)
	return s.Store.SaveOpening(ctx, o)
}

// DashboardRow is one open job opening on the pipeline dashboard (FR-4.1).
type DashboardRow struct {
	Opening  domain.Opening
	DaysOpen int
	Flagged  bool
}

// GetPipelineDashboard returns every opening that is not closed (getPipelineDashboard()).
func (s *Pipeline) GetPipelineDashboard(ctx context.Context) ([]DashboardRow, error) {
	openings, err := s.Store.ListOpenings(ctx)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	rows := make([]DashboardRow, len(openings))
	for i, o := range openings {
		rows[i] = DashboardRow{Opening: o, DaysOpen: o.DaysOpen(now), Flagged: o.FlaggedAt != nil}
	}
	return rows, nil
}

// StalenessReport is what one staleness sweep did.
type StalenessReport struct {
	Flagged []string `json:"flagged"`
	Cleared []string `json:"cleared"`
}

// EvaluateStaleness runs the staleness rule over every open opening (evaluateStaleness(),
// FR-4.2). A newly stale opening is flagged and its Recruiter told once (FR-4.3, FR-4.4); a
// flag whose condition no longer holds is cleared (FR-4.5).
func (s *Pipeline) EvaluateStaleness(ctx context.Context) (StalenessReport, error) {
	report := StalenessReport{Flagged: []string{}, Cleared: []string{}}
	openings, err := s.Store.ListOpenings(ctx)
	if err != nil {
		return report, err
	}
	now := s.Clock.Now()
	for i := range openings {
		o := &openings[i]
		stale := o.IsStale(now)
		switch {
		case stale && o.FlaggedAt == nil:
			o.FlaggedAt = &now
			if err := s.Store.SetFlag(ctx, o.ID, o.FlaggedAt); err != nil {
				return report, err
			}
			report.Flagged = append(report.Flagged, o.ID)
			s.notify(ctx, recipient(o.Recruiter, s.FallbackRecipient),
				fmt.Sprintf("Job opening %q is stale", titleOf(o)),
				fmt.Sprintf("The job opening %q (%s) has been open for %d days, longer than its expected time-to-fill of %d days.\n",
					titleOf(o), o.ID, o.DaysOpen(now), o.ExpectedTimeToFillDays))
		case !stale && o.FlaggedAt != nil:
			o.FlaggedAt = nil
			if err := s.Store.SetFlag(ctx, o.ID, nil); err != nil {
				return report, err
			}
			report.Cleared = append(report.Cleared, o.ID)
		}
	}
	return report, nil
}

// notify sends a notice and logs, rather than fails, when it cannot: the flag is the record,
// and a sweep that stopped on a mail server would leave the openings after it unflagged.
func (s *Pipeline) notify(ctx context.Context, to, subject, body string) {
	sendNotice(ctx, s.Notifier, to, subject, body)
}

func sendNotice(ctx context.Context, n Notifier, to, subject, body string) {
	if to == "" {
		slog.Warn("notice not sent: no recipient is known", "subject", subject)
		return
	}
	if err := n.Notify(ctx, to, subject, body); err != nil {
		slog.Error("notice not sent", "subject", subject, "err", err)
	}
}

func recipient(recruiter, fallback string) string {
	if recruiter != "" {
		return recruiter
	}
	return fallback
}

func titleOf(o *domain.Opening) string {
	if o.Title != "" {
		return o.Title
	}
	return o.ID
}

// AccessAudit keeps the access audit log (recordAuthorisationRejected()).
type AccessAudit struct {
	Log   AccessLog
	IDs   IDs
	Clock Clock
}

// RecordAuthorisationRejected records one request the gateway refused for want of a role.
func (s *AccessAudit) RecordAuthorisationRejected(ctx context.Context, r domain.AccessRejection) (domain.AccessRejection, error) {
	if r.Method == "" || r.Path == "" {
		return r, fmt.Errorf("%w: method and path are required", domain.ErrInvalidEvent)
	}
	r.ID = s.IDs.NewID("acc")
	if r.OccurredAt.IsZero() {
		r.OccurredAt = s.Clock.Now()
	}
	return r, s.Log.RecordRejection(ctx, r)
}

// ListAccessRejections returns the most recent entries, newest first.
func (s *AccessAudit) ListAccessRejections(ctx context.Context, limit int) ([]domain.AccessRejection, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.Log.ListRejections(ctx, limit)
}
