// Package app is the composition root: it builds the adapters from configuration, wires them
// into the use cases, and runs the scheduler (ADR-017).
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/frngrit/HireAssist/services/compliance/internal/adapter/email"
	"github.com/frngrit/HireAssist/services/compliance/internal/adapter/holders"
	"github.com/frngrit/HireAssist/services/compliance/internal/adapter/httpapi"
	"github.com/frngrit/HireAssist/services/compliance/internal/adapter/postgres"
	"github.com/frngrit/HireAssist/services/compliance/internal/adapter/rabbitmq"
	"github.com/frngrit/HireAssist/services/compliance/internal/adapter/system"
	"github.com/frngrit/HireAssist/services/compliance/internal/domain"
	"github.com/frngrit/HireAssist/services/compliance/internal/usecase"
)

// Config is the service's configuration, read from the environment by main.
type Config struct {
	DatabaseURL         string
	RabbitMQURL         string
	ResumeProcessingURL string
	HiringURL           string
	SMTP                email.SMTP
	FallbackRecipient   string
	SweepInterval       time.Duration
}

// Locker runs fn unless another replica is already sweeping.
type Locker interface {
	WithSweepLock(ctx context.Context, fn func(context.Context) error) (bool, error)
}

// Sweeper runs the two scheduled sweeps (evaluateStaleness() and evaluateRetention()) under
// the sweep lock.
type Sweeper struct {
	Lock     Locker
	Pipeline *usecase.Pipeline
	Policy   *usecase.Retention
}

func (s Sweeper) Staleness(ctx context.Context) (r usecase.StalenessReport, ran bool, err error) {
	ran, err = s.Lock.WithSweepLock(ctx, func(ctx context.Context) error {
		r, err = s.Pipeline.EvaluateStaleness(ctx)
		return err
	})
	return r, ran, err
}

func (s Sweeper) Retention(ctx context.Context) (r usecase.RetentionReport, ran bool, err error) {
	ran, err = s.Lock.WithSweepLock(ctx, func(ctx context.Context) error {
		r, err = s.Policy.EvaluateRetention(ctx)
		return err
	})
	return r, ran, err
}

// Schedule runs both sweeps once shortly after start and then every interval, until ctx ends
// (FR-4.2, FR-5.2). A sweep that fails is logged and tried again at the next tick.
func (s Sweeper) Schedule(ctx context.Context, interval time.Duration) {
	timer := time.NewTimer(10 * time.Second) // let the other services come up first
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if r, ran, err := s.Staleness(ctx); err != nil {
			slog.Error("staleness sweep failed", "err", err)
		} else if ran {
			slog.Info("staleness sweep done", "flagged", len(r.Flagged), "cleared", len(r.Cleared))
		}
		if r, ran, err := s.Retention(ctx); err != nil {
			slog.Error("retention sweep failed", "err", err)
		} else if ran {
			slog.Info("retention sweep done", "policy_set", r.PolicySet, "evaluated", r.Evaluated,
				"warned", len(r.Warned), "erasures", len(r.Erasures), "held", len(r.Held), "retried_finished", r.RetriedFinished)
		}
		timer.Reset(interval)
	}
}

// New builds the service. The database is required: the audit logs this service keeps must
// survive a restart. The broker, the holders and SMTP are not; without one, the part that needs
// it says so in the log and the rest works.
func New(ctx context.Context, cfg Config) (http.Handler, func(), error) {
	if cfg.DatabaseURL == "" {
		return nil, nil, errors.New("DATABASE_URL is required: the erasure and access audit logs must not live in memory")
	}
	store, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}

	var notifier usecase.Notifier = email.Log{}
	if cfg.SMTP.Addr != "" {
		notifier = cfg.SMTP
	}
	clock, ids := system.Clock{}, system.IDs{}

	holderErasers := map[domain.Holder]usecase.HolderEraser{}
	var register usecase.CandidateRegister = noRegister{}
	if cfg.ResumeProcessingURL != "" {
		rp := holders.ResumeProcessing{BaseURL: cfg.ResumeProcessingURL}
		holderErasers[domain.HolderResumeProcessing], register = rp, rp
	}
	if cfg.HiringURL != "" {
		holderErasers[domain.HolderHiring] = holders.Hiring{BaseURL: cfg.HiringURL}
	}

	pipeline := &usecase.Pipeline{Store: store, Notifier: notifier, Clock: clock, FallbackRecipient: cfg.FallbackRecipient}
	erasures := &usecase.Erasures{Store: store, Holders: holderErasers, IDs: ids, Clock: clock}
	retention := &usecase.Retention{Policies: store, Marks: store, Pipeline: store, Register: register,
		Erasures: erasures, Notifier: notifier, Clock: clock, FallbackRecipient: cfg.FallbackRecipient}
	sweeper := Sweeper{Lock: store, Pipeline: pipeline, Policy: retention}

	ctx, cancel := context.WithCancel(ctx)
	var consumer *rabbitmq.Consumer
	if cfg.RabbitMQURL != "" {
		consumer = rabbitmq.New(cfg.RabbitMQURL)
		go consumer.Run(ctx, pipeline)
	}
	go sweeper.Schedule(ctx, cfg.SweepInterval)

	handler := httpapi.NewHandler(httpapi.Services{
		Pipeline: pipeline, Retention: retention, Erasures: erasures,
		AccessAudit: &usecase.AccessAudit{Log: store, IDs: ids, Clock: clock},
		Sweeps:      sweeper,
		Ready: func(ctx context.Context) map[string]bool {
			checks := map[string]bool{"postgres": store.Ping(ctx) == nil}
			if consumer != nil {
				checks["rabbitmq"] = consumer.Ready()
			}
			return checks
		},
	})
	return handler, func() { cancel(); store.Close() }, nil
}

// noRegister stands in when Resume Processing is not configured: the retention sweep then
// finds no candidates, rather than failing.
type noRegister struct{}

func (noRegister) ListCandidates(context.Context) ([]usecase.RegisteredCandidate, error) {
	return nil, nil
}
