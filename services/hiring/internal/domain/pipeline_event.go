package domain

import "time"

// PipelineEventType names one change the Compliance & Insights Service counts for the pipeline
// dashboard (UC-4). The values are the contract in services/compliance/MESSAGES.md (ADR-015).
type PipelineEventType string

const (
	EventOpeningOpened        PipelineEventType = "opening.opened"
	EventOpeningPaused        PipelineEventType = "opening.paused"
	EventOpeningResumed       PipelineEventType = "opening.resumed"
	EventOpeningClosed        PipelineEventType = "opening.closed"
	EventResumeSubmitted      PipelineEventType = "resume.submitted"
	EventResumeScreened       PipelineEventType = "resume.screened"
	EventCandidateShortlisted PipelineEventType = "candidate.shortlisted"
)

// PipelineEvent is one change to report. It is stored in the same transaction as the change
// it reports, and published from there, so it is neither lost nor sent for a change that did not
// happen. ID is given by the store.
type PipelineEvent struct {
	ID           string
	Type         PipelineEventType
	OccurredAt   time.Time
	JobOpeningID string
	// Title and ExpectedTimeToFillDays travel only on opening.opened.
	Title                  string
	ExpectedTimeToFillDays int
	ResumeID               string
	CandidateID            string
}

// OpeningEvent reports a change to a job opening's lifecycle.
func OpeningEvent(t PipelineEventType, o *JobOpening, at time.Time) PipelineEvent {
	e := PipelineEvent{Type: t, OccurredAt: at, JobOpeningID: o.ID}
	if t == EventOpeningOpened {
		const day = 24 * time.Hour
		e.Title = o.Title
		// Rounded up: the consumer requires at least one day, and a part day is still a day to wait.
		e.ExpectedTimeToFillDays = int((o.ExpectedTimeToFill + day - 1) / day)
	}
	return e
}

// ResumeEvent reports a resume reaching a stage of a job opening's pipeline.
func ResumeEvent(t PipelineEventType, openingID string, e *ScreeningEntry, at time.Time) PipelineEvent {
	ev := PipelineEvent{Type: t, OccurredAt: at, JobOpeningID: openingID, ResumeID: e.ResumeID}
	if e.Result != nil {
		ev.CandidateID = e.Result.CandidateID
	}
	return ev
}
