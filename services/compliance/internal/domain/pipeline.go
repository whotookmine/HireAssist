package domain

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidEvent means a pipeline event can never be applied, however often it is delivered.
var ErrInvalidEvent = errors.New("invalid pipeline event")

// EventType names what happened in the hiring pipeline. Hiring publishes these; the contract is
// services/compliance/MESSAGES.md.
type EventType string

const (
	OpeningOpened        EventType = "opening.opened"
	OpeningPaused        EventType = "opening.paused"
	OpeningResumed       EventType = "opening.resumed"
	OpeningClosed        EventType = "opening.closed"
	ResumeSubmitted      EventType = "resume.submitted"      // the "applied" stage
	ResumeScreened       EventType = "resume.screened"       // the "screened" stage
	CandidateShortlisted EventType = "candidate.shortlisted" // the "shortlisted" stage
)

// Stage is a pipeline stage counted on the dashboard (FR-4.1).
type Stage string

const (
	StageApplied     Stage = "applied"
	StageScreened    Stage = "screened"
	StageShortlisted Stage = "shortlisted"
)

// PipelineEvent is one thing that happened to a job opening or to a candidate in it.
type PipelineEvent struct {
	ID           string
	Type         EventType
	OccurredAt   time.Time
	JobOpeningID string

	// opening.opened
	Title                  string
	ExpectedTimeToFillDays int
	Recruiter              string // the responsible Recruiter's email; optional

	// resume.* and candidate.*
	ResumeID    string
	CandidateID string // known from resume.screened on; links activity to the candidate
}

// Stage is the stage an entry event counts towards, and false for an opening event.
func (e PipelineEvent) Stage() (Stage, bool) {
	switch e.Type {
	case ResumeSubmitted:
		return StageApplied, true
	case ResumeScreened:
		return StageScreened, true
	case CandidateShortlisted:
		return StageShortlisted, true
	}
	return "", false
}

// Validate refuses an event that cannot be applied.
func (e PipelineEvent) Validate() error {
	if e.ID == "" || e.JobOpeningID == "" || e.OccurredAt.IsZero() {
		return fmt.Errorf("%w: event_id, job_opening_id and occurred_at are required", ErrInvalidEvent)
	}
	switch e.Type {
	case OpeningOpened:
		if e.ExpectedTimeToFillDays <= 0 {
			return fmt.Errorf("%w: opening.opened needs a positive expected_time_to_fill_days", ErrInvalidEvent)
		}
	case OpeningPaused, OpeningResumed, OpeningClosed:
	case ResumeSubmitted, ResumeScreened, CandidateShortlisted:
		// Entries are counted by resume, Hiring's id, so that a repeat is counted once and the
		// count holds no candidate id once the candidate is erased.
		if e.ResumeID == "" {
			return fmt.Errorf("%w: %s needs resume_id", ErrInvalidEvent, e.Type)
		}
	default:
		return fmt.Errorf("%w: unknown type %q", ErrInvalidEvent, e.Type)
	}
	return nil
}

// OpeningStatus mirrors the job opening's status in Hiring.
type OpeningStatus string

const (
	StatusOpen   OpeningStatus = "open"
	StatusPaused OpeningStatus = "paused"
	StatusClosed OpeningStatus = "closed"
)

// Opening is this service's own view of a job opening, built only from pipeline events (the
// dashboard is a projection, not a query of Hiring).
type Opening struct {
	ID                     string
	Title                  string
	Status                 OpeningStatus
	OpenedAt               time.Time
	ExpectedTimeToFillDays int // zero until opening.opened has arrived
	Recruiter              string
	FlaggedAt              *time.Time // set while the opening is flagged stale (FR-4.3)
	Counts                 map[Stage]int
}

// Apply changes the view for an opening event. A closed opening is final (FR-1.9), so an event
// that arrives after the close — a redelivery, most likely — changes nothing; closing clears
// the stale flag (FR-4.5).
func (o *Opening) Apply(e PipelineEvent) {
	if o.Status == StatusClosed {
		return
	}
	switch e.Type {
	case OpeningOpened:
		// Sent again when the details change; the opening date is the first one's.
		o.Title, o.ExpectedTimeToFillDays = e.Title, e.ExpectedTimeToFillDays
		if o.OpenedAt.IsZero() {
			o.OpenedAt = e.OccurredAt
		}
		if e.Recruiter != "" {
			o.Recruiter = e.Recruiter
		}
		if o.Status == "" {
			o.Status = StatusOpen
		}
	case OpeningPaused:
		o.Status = StatusPaused
	case OpeningResumed:
		o.Status = StatusOpen
	case OpeningClosed:
		o.Status, o.FlaggedAt = StatusClosed, nil
	}
}

// DaysOpen is how long the opening has been open, in whole days.
func (o Opening) DaysOpen(now time.Time) int {
	if o.OpenedAt.IsZero() {
		return 0
	}
	return int(now.Sub(o.OpenedAt) / Day)
}

// IsStale is the staleness rule: open for longer than its expected time-to-fill (FR-4.3). A
// paused opening is excluded while paused, so the alerts stay meaningful (UC-4 alternate flow
// 4a); a sweep therefore clears its flag, and flags it again if it is resumed still overdue. An
// opening whose opened event has not arrived is never stale.
func (o Opening) IsStale(now time.Time) bool {
	return o.Status == StatusOpen && o.ExpectedTimeToFillDays > 0 &&
		now.Sub(o.OpenedAt) > time.Duration(o.ExpectedTimeToFillDays)*Day
}
