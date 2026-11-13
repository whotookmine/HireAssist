package usecase

import (
	"context"
	"errors"
	"fmt"
)

// ErrInvalidCandidate means the request named no candidate.
var ErrInvalidCandidate = errors.New("a candidate id is required")

// CandidateErasure is eraseHiringData(): the Hiring Service's half of erasing one candidate
// (UC-5, FR-5.3, ADR-016). The Compliance & Insights Service calls it and records the answer.
type CandidateErasure struct {
	Batches ScreeningRepository
	Files   ResumeStore
	Guides  InterviewGuideRepository
}

// EraseCandidate removes the candidate's resume files, screening results and decisions, and
// interview guides. It succeeds when nothing is held, so asking again is safe. Files go first:
// the rows are how the files are found, so deleting them first could leave a file nobody can
// find again. A failure part-way leaves the rest for the next attempt.
func (s *CandidateErasure) EraseCandidate(ctx context.Context, candidateID string) error {
	if candidateID == "" {
		return ErrInvalidCandidate
	}
	keys, err := s.Batches.ResumeKeysOfCandidate(ctx, candidateID)
	if err != nil {
		return fmt.Errorf("finding the candidate's resumes: %w", err)
	}
	for _, key := range keys {
		if err := s.Files.DeleteResume(ctx, key); err != nil {
			return fmt.Errorf("deleting resume file %s: %w", key, err)
		}
	}
	if err := s.Guides.DeleteByCandidate(ctx, candidateID); err != nil {
		return fmt.Errorf("deleting the candidate's interview guides: %w", err)
	}
	if err := s.Batches.EraseCandidate(ctx, candidateID); err != nil {
		return fmt.Errorf("deleting the candidate's screening data: %w", err)
	}
	return nil
}
