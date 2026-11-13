// Command compliance runs the Compliance & Insights Service.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/frngrit/HireAssist/services/compliance/internal/adapter/email"
	"github.com/frngrit/HireAssist/services/compliance/internal/app"
)

func main() {
	// Configuration comes from the environment (ADR-005 rule 4).
	addr := ":" + envOr("PORT", "8083")
	interval, err := time.ParseDuration(envOr("SWEEP_INTERVAL", "1h"))
	if err != nil || interval < time.Minute {
		slog.Error("SWEEP_INTERVAL must be a duration of at least 1m, such as 1h", "value", os.Getenv("SWEEP_INTERVAL"))
		os.Exit(1)
	}
	cfg := app.Config{
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		RabbitMQURL:         os.Getenv("RABBITMQ_URL"),
		ResumeProcessingURL: os.Getenv("RESUME_PROCESSING_URL"),
		HiringURL:           os.Getenv("HIRING_URL"),
		SMTP: email.SMTP{
			Addr:     os.Getenv("SMTP_ADDR"),
			From:     envOr("SMTP_FROM", "HireAssist <no-reply@hireassist.local>"),
			Username: os.Getenv("SMTP_USERNAME"),
			Password: os.Getenv("SMTP_PASSWORD"),
		},
		FallbackRecipient: os.Getenv("NOTIFY_FALLBACK_TO"),
		SweepInterval:     interval,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	handler, closeService, err := app.New(ctx, cfg)
	if err != nil {
		slog.Error("startup failed", "err", err)
		os.Exit(1)
	}
	defer closeService()
	warnAboutWhatIsMissing(cfg)

	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		slog.Info("compliance service listening", "addr", addr, "sweep_interval", interval.String())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "err", err)
	}
}

// warnAboutWhatIsMissing says, once at startup, what the service does without each optional
// dependency.
func warnAboutWhatIsMissing(cfg app.Config) {
	for _, d := range []struct {
		configured bool
		without    string
	}{
		{cfg.RabbitMQURL != "", "RABBITMQ_URL not set: no pipeline events arrive, so the dashboard stays empty"},
		{cfg.ResumeProcessingURL != "", "RESUME_PROCESSING_URL not set: the retention sweep finds no candidates, and no erasure can finish"},
		{cfg.HiringURL != "", "HIRING_URL not set: no erasure can finish"},
		{cfg.SMTP.Addr != "", "SMTP_ADDR not set: notices are logged, not sent"},
		{cfg.FallbackRecipient != "", "NOTIFY_FALLBACK_TO not set: notices about an opening with no known Recruiter are dropped"},
	} {
		if !d.configured {
			slog.Warn(d.without)
		}
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
