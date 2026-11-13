// Package rabbitmq consumes the pipeline events Hiring publishes (ADR-015). The contract —
// queue, body, headers — is services/compliance/MESSAGES.md.
package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/frngrit/HireAssist/services/compliance/internal/domain"
)

const (
	// Queue is shared with Hiring: durable, no arguments, so either side may declare it.
	Queue = "pipeline.events"
	// DeadLetterQueue is this service's own: events it can never apply wait there for a person.
	DeadLetterQueue = "pipeline.events.dlq"

	correlationHeader = "x-correlation-id"
	prefetch          = 20
	maxBackoff        = 30 * time.Second
)

// Events is the use case consumed events are handed to.
type Events interface {
	RecordPipelineEvent(ctx context.Context, e domain.PipelineEvent) error
}

// Message is the body of one pipeline event, as MESSAGES.md defines it.
type Message struct {
	EventID                string    `json:"event_id"`
	Type                   string    `json:"type"`
	OccurredAt             time.Time `json:"occurred_at"`
	JobOpeningID           string    `json:"job_opening_id"`
	Title                  string    `json:"title,omitempty"`
	ExpectedTimeToFillDays int       `json:"expected_time_to_fill_days,omitempty"`
	Recruiter              string    `json:"recruiter,omitempty"`
	ResumeID               string    `json:"resume_id,omitempty"`
	CandidateID            string    `json:"candidate_id,omitempty"`
	CorrelationID          string    `json:"correlation_id,omitempty"`
}

// ToDomain converts the message.
func (m Message) ToDomain() domain.PipelineEvent {
	return domain.PipelineEvent{
		ID: m.EventID, Type: domain.EventType(m.Type), OccurredAt: m.OccurredAt, JobOpeningID: m.JobOpeningID,
		Title: m.Title, ExpectedTimeToFillDays: m.ExpectedTimeToFillDays, Recruiter: m.Recruiter,
		ResumeID: m.ResumeID, CandidateID: m.CandidateID,
	}
}

// Consumer keeps one connection to RabbitMQ and hands every event to the use case. It
// reconnects by itself.
type Consumer struct {
	url   string
	queue string
	dlq   string
	ready atomic.Bool
}

// New returns a consumer of the contract's queue. It is not connected yet; Run connects it.
func New(url string) *Consumer { return &Consumer{url: url, queue: Queue, dlq: DeadLetterQueue} }

// NewWithQueues returns a consumer of other queues, so a test's traffic never meets a service's.
func NewWithQueues(url, queue, dlq string) *Consumer {
	return &Consumer{url: url, queue: queue, dlq: dlq}
}

// Ready reports whether the consumer is connected, for the readiness check.
func (c *Consumer) Ready() bool { return c.ready.Load() }

// Run consumes until ctx ends, reconnecting with backoff when the connection drops.
func (c *Consumer) Run(ctx context.Context, events Events) {
	backoff := time.Second
	for {
		started := time.Now()
		err := c.session(ctx, events)
		c.ready.Store(false)
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) > time.Minute {
			backoff = time.Second
		}
		slog.Warn("RabbitMQ unavailable; reconnecting", "err", err, "in", backoff.String())
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(2*backoff, maxBackoff)
	}
}

func (c *Consumer) session(ctx context.Context, events Events) error {
	conn, err := amqp.DialConfig(c.url, amqp.Config{Heartbeat: 10 * time.Second, Properties: amqp.Table{"connection_name": "compliance"}})
	if err != nil {
		return err
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	for _, q := range []string{c.queue, c.dlq} {
		if _, err := ch.QueueDeclare(q, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declaring queue %s: %w", q, err)
		}
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return err
	}
	deliveries, err := ch.ConsumeWithContext(ctx, c.queue, "compliance", false, false, false, false, nil)
	if err != nil {
		return err
	}
	c.ready.Store(true)
	slog.Info("connected to RabbitMQ", "events", c.queue)
	closed := conn.NotifyClose(make(chan *amqp.Error, 1))
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-closed:
			if err == nil {
				return errors.New("connection closed")
			}
			return err
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("the broker stopped delivering events")
			}
			c.handle(ctx, ch, events, d)
		}
	}
}

// handle applies one event. It is acknowledged once applied; an event that can never be applied
// is moved to the dead-letter queue; any other failure puts it back to be delivered again.
func (c *Consumer) handle(ctx context.Context, ch *amqp.Channel, events Events, d amqp.Delivery) {
	var m Message
	if err := json.Unmarshal(d.Body, &m); err != nil {
		c.deadLetter(ctx, ch, d, fmt.Errorf("%w: %w", domain.ErrInvalidEvent, err))
		return
	}
	switch err := events.RecordPipelineEvent(ctx, m.ToDomain()); {
	case err == nil:
		slog.Info("pipeline event recorded", "event", m.EventID, "type", m.Type, "opening", m.JobOpeningID, "correlation_id", m.CorrelationID)
		_ = d.Ack(false)
	case errors.Is(err, domain.ErrInvalidEvent):
		c.deadLetter(ctx, ch, d, err)
	default:
		slog.Warn("pipeline event not recorded; it will be delivered again", "event", m.EventID, "err", err)
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
		_ = d.Nack(false, true)
	}
}

func (c *Consumer) deadLetter(ctx context.Context, ch *amqp.Channel, d amqp.Delivery, reason error) {
	slog.Error("pipeline event refused; moved to the dead-letter queue", "err", reason, "queue", c.dlq, "message_id", d.MessageId)
	if err := ch.PublishWithContext(ctx, "", c.dlq, false, false, amqp.Publishing{
		ContentType: d.ContentType, DeliveryMode: amqp.Persistent, MessageId: d.MessageId, Timestamp: time.Now().UTC(),
		Headers: amqp.Table{"x-refused-because": reason.Error(), correlationHeader: d.Headers[correlationHeader]},
		Body:    d.Body,
	}); err != nil {
		slog.Error("could not keep the refused event; it will be delivered again", "err", err)
		_ = d.Nack(false, true)
		return
	}
	_ = d.Ack(false)
}
