// Command identity runs the HireAssist Identity Service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// config holds everything the service reads from environment variables.
type config struct {
	port        string
	databaseURL string
}

func loadConfig() config {
	return config{
		port: envOr("PORT", "8081"),
		// No default: a connection string carries a password, so it never lives in code.
		// `make run` passes the one for the local Docker database.
		databaseURL: os.Getenv("DATABASE_URL"),
	}
}

// envOr returns the environment variable, or fallback when it is unset or empty.
func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func main() {
	// One JSON object per log line.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("identity service stopped", "error", err)
		os.Exit(1)
	}
}

// run starts the server and blocks until Ctrl+C or SIGTERM, then shuts down gracefully.
func run() error {
	cfg := loadConfig()
	if cfg.databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is not set; start the service with `make run`")
	}

	// Fail at startup, with a clear message, rather than on the first request.
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelConnect()
	pool, err := pgxpool.New(connectCtx, cfg.databaseURL)
	if err != nil {
		return fmt.Errorf("bad DATABASE_URL: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(connectCtx); err != nil {
		return fmt.Errorf("cannot reach PostgreSQL (is it running? try `make db-up`): %w", err)
	}
	slog.Info("connected to PostgreSQL")

	api := &API{store: &Store{db: pool}}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /readyz", api.handleReady)

	mux.HandleFunc("GET /members", api.listMembers)                     // listMembers()
	mux.HandleFunc("GET /members/{id}", api.getMember)                  // getMember()
	mux.HandleFunc("POST /members", api.createMember)                   // createMember()
	mux.HandleFunc("PATCH /members/{id}", api.changeMemberRole)         // changeMemberRole()
	mux.HandleFunc("PUT /members/{id}/password", api.setMemberPassword) // setMemberPassword()
	mux.HandleFunc("DELETE /members/{id}", api.removeMember)            // removeMember()

	server := &http.Server{
		Addr:              ":" + cfg.port,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("identity service listening", "port", cfg.port)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("start server: %w", err)
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down server: %w", err)
	}
	slog.Info("stopped")
	return nil
}

// handleHealth answers the liveness probe: the process is up and serving HTTP.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
