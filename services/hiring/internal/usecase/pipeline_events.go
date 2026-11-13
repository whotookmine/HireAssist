package usecase

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/frngrit/HireAssist/services/hiring/internal/domain"
)

// PipelineOutbox holds the pipeline events stored beside the changes they report, until they
// are published.
type PipelineOutbox interface {
	// Unpublished returns, oldest first, at most limit events not yet published.
	Unpublished(ctx context.Context, limit int) ([]domain.PipelineEvent, error)
	MarkPublished(ctx context.Context, eventID string, at time.Time) error
}

// PipelinePublisher hands one event to the broker, returning once the broker has taken it.
type PipelinePublisher interface {
	PublishPipelineEvent(ctx context.Context, e domain.PipelineEvent) error
}

// PipelineRelay publishes pipeline events to the Compliance & Insights Service (ADR-015).
type PipelineRelay struct {
	Outbox    PipelineOutbox
	Publisher PipelinePublisher
	Clock     Clock

	mu sync.Mutex // one pass at a time, so a pass never sends what another is sending
}

// PublishPending sends every waiting event, oldest first, and returns how many it sent. An
// event is marked published only after the broker confirms it, so a crash in between sends it
// again; Compliance counts each entry once per resume and stage, so a repeat changes nothing.
// It stops at the first failure, keeping the order.
func (r *PipelineRelay) PublishPending(ctx context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	events, err := r.Outbox.Unpublished(ctx, publishLimit)
	if err != nil {
		return 0, fmt.Errorf("finding unpublished pipeline events: %w", err)
	}
	for i, e := range events {
		if err := r.Publisher.PublishPipelineEvent(ctx, e); err != nil {
			return i, fmt.Errorf("publishing pipeline event %s: %w", e.ID, err)
		}
		if err := r.Outbox.MarkPublished(ctx, e.ID, r.Clock.Now()); err != nil {
			return i, fmt.Errorf("marking pipeline event %s published: %w", e.ID, err)
		}
	}
	return len(events), nil
}
