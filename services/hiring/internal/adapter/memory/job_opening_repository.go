// Package memory is an in-process adapter for the job opening repository port, used for
// local runs and tests until the PostgreSQL adapter exists.
package memory

import (
	"context"
	"slices"
	"sync"

	"github.com/frngrit/HireAssist/services/hiring/internal/domain"
	"github.com/frngrit/HireAssist/services/hiring/internal/usecase"
)

// JobOpeningRepository keeps job openings in memory. It is safe for concurrent use, and
// stores and returns copies so that a change reaches the store only through Save.
type JobOpeningRepository struct {
	mu     sync.RWMutex
	byID   map[string]domain.JobOpening
	order  []string
	events []domain.PipelineEvent
}

// NewJobOpeningRepository returns an empty repository.
func NewJobOpeningRepository() *JobOpeningRepository {
	return &JobOpeningRepository{byID: map[string]domain.JobOpening{}}
}

// Save inserts a new job opening or replaces an existing one with the same id.
func (r *JobOpeningRepository) Save(_ context.Context, o *domain.JobOpening, events ...domain.PipelineEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, events...)
	if _, exists := r.byID[o.ID]; !exists {
		r.order = append(r.order, o.ID)
	}
	r.byID[o.ID] = clone(*o)
	return nil
}

// PipelineEvents returns the events saved so far. In memory nothing publishes them; it is for
// tests to read.
func (r *JobOpeningRepository) PipelineEvents() []domain.PipelineEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Clone(r.events)
}

// FindByID returns a copy of the job opening, or usecase.ErrNotFound.
func (r *JobOpeningRepository) FindByID(_ context.Context, id string) (*domain.JobOpening, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.byID[id]
	if !ok {
		return nil, usecase.ErrNotFound
	}
	c := clone(o)
	return &c, nil
}

// List returns copies of every job opening in creation order.
func (r *JobOpeningRepository) List(_ context.Context) ([]domain.JobOpening, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]domain.JobOpening, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, clone(r.byID[id]))
	}
	return out, nil
}

// clone copies an opening, including its criteria slice.
func clone(o domain.JobOpening) domain.JobOpening {
	o.Criteria = append([]domain.Criterion(nil), o.Criteria...)
	return o
}
