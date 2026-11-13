// Package httpapi is the REST adapter of the Compliance & Insights Service. It translates HTTP to
// use case calls and business errors to status codes; it holds no rules.
//
// Everything a member uses sits under /compliance, the one prefix the gateway routes here; the
// gateway checks the role each path needs (ADR-010). /internal/* is for other services and is
// never routed by the gateway.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/frngrit/HireAssist/services/compliance/internal/domain"
	"github.com/frngrit/HireAssist/services/compliance/internal/usecase"
)

// Sweeps runs a sweep now, under the same lock the scheduler takes; ran is false when another
// replica is sweeping.
type Sweeps interface {
	Retention(ctx context.Context) (r usecase.RetentionReport, ran bool, err error)
	Staleness(ctx context.Context) (r usecase.StalenessReport, ran bool, err error)
}

// Services are what the handler drives.
type Services struct {
	Pipeline    *usecase.Pipeline
	Retention   *usecase.Retention
	Erasures    *usecase.Erasures
	AccessAudit *usecase.AccessAudit
	Sweeps      Sweeps
	// Ready reports whether the dependencies the service cannot work without answer.
	Ready func(ctx context.Context) map[string]bool
}

const maxBody = 64 << 10

type handler struct{ Services }

// NewHandler routes every endpoint.
func NewHandler(s Services) http.Handler {
	h := handler{s}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		write(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", h.ready)

	mux.HandleFunc("GET /compliance/dashboard", h.dashboard)
	mux.HandleFunc("GET /compliance/retention-policy", h.getPolicy)
	mux.HandleFunc("PUT /compliance/retention-policy", h.setPolicy)
	mux.HandleFunc("GET /compliance/held-expiries", h.listHolds)
	mux.HandleFunc("POST /compliance/held-expiries/{candidateId}", h.resolveHold)
	mux.HandleFunc("POST /compliance/erasures", h.erase)
	mux.HandleFunc("GET /compliance/erasures", h.listErasures)
	mux.HandleFunc("GET /compliance/erasures/{ref}", h.getErasure)
	mux.HandleFunc("GET /compliance/access-audit", h.listRejections)
	mux.HandleFunc("POST /compliance/sweeps/retention", h.sweepRetention)
	mux.HandleFunc("POST /compliance/sweeps/staleness", h.sweepStaleness)

	mux.HandleFunc("POST /internal/access-rejections", h.recordRejection)
	return mux
}

func (h handler) ready(w http.ResponseWriter, r *http.Request) {
	checks := h.Ready(r.Context())
	status, code := "ready", http.StatusOK
	for _, ok := range checks {
		if !ok {
			status, code = "not ready", http.StatusServiceUnavailable
		}
	}
	write(w, code, map[string]any{"status": status, "checks": checks})
}

// ---- UC-4 ----

func (h handler) dashboard(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pipeline.GetPipelineDashboard(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		o := row.Opening
		out[i] = map[string]any{
			"job_opening_id": o.ID, "title": o.Title, "status": o.Status,
			"applied": o.Counts[domain.StageApplied], "screened": o.Counts[domain.StageScreened],
			"shortlisted": o.Counts[domain.StageShortlisted], "days_open": row.DaysOpen,
			"expected_time_to_fill_days": o.ExpectedTimeToFillDays, "flagged": row.Flagged, "flagged_at": o.FlaggedAt,
		}
	}
	write(w, http.StatusOK, map[string]any{"openings": out})
}

func (h handler) sweepStaleness(w http.ResponseWriter, r *http.Request) {
	report, ran, err := h.Sweeps.Staleness(r.Context())
	sweepAnswer(w, report, ran, err)
}

// ---- UC-5 ----

func policyOut(p *domain.RetentionPolicy) map[string]any {
	return map[string]any{"period_days": p.PeriodDays, "anchor": p.Anchor, "expiry_action": p.Action,
		"warning_window_days": p.WarningWindowDays, "updated_at": p.UpdatedAt}
}

func (h handler) getPolicy(w http.ResponseWriter, r *http.Request) {
	p, err := h.Retention.GetRetentionPolicy(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	write(w, http.StatusOK, policyOut(p))
}

func (h handler) setPolicy(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PeriodDays        int    `json:"period_days"`
		Anchor            string `json:"anchor"`
		ExpiryAction      string `json:"expiry_action"`
		WarningWindowDays int    `json:"warning_window_days"`
	}
	if !read(w, r, &in) {
		return
	}
	p, err := h.Retention.SetRetentionPolicy(r.Context(), in.PeriodDays, domain.Anchor(in.Anchor), domain.ExpiryAction(in.ExpiryAction), in.WarningWindowDays)
	if err != nil {
		fail(w, err)
		return
	}
	write(w, http.StatusOK, policyOut(p))
}

func (h handler) listHolds(w http.ResponseWriter, r *http.Request) {
	holds, err := h.Retention.ListHeldExpiries(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]map[string]any, len(holds))
	for i, hd := range holds {
		out[i] = map[string]any{"candidate_id": hd.CandidateID, "job_opening_id": hd.JobOpeningID, "expires_at": hd.ExpiresAt, "held_at": hd.HeldAt}
	}
	write(w, http.StatusOK, map[string]any{"held_expiries": out})
}

func (h handler) resolveHold(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Decision string `json:"decision"`
	}
	if !read(w, r, &in) {
		return
	}
	e, err := h.Retention.ResolveHeldExpiry(r.Context(), r.PathValue("candidateId"), domain.HoldDecision(in.Decision))
	if err != nil {
		fail(w, err)
		return
	}
	if e == nil {
		write(w, http.StatusOK, map[string]string{"decision": in.Decision})
		return
	}
	writeErasure(w, e)
}

func (h handler) erase(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CandidateID string `json:"candidate_id"`
	}
	if !read(w, r, &in) {
		return
	}
	policy, err := h.Retention.Policies.GetPolicy(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	e, err := h.Erasures.EraseCandidate(r.Context(), in.CandidateID, domain.ReasonRequested, policy)
	if err != nil {
		fail(w, err)
		return
	}
	writeErasure(w, e)
}

func erasureOut(e *domain.Erasure) map[string]any {
	holders := map[string]string{}
	for _, hd := range domain.Holders {
		holders[string(hd)] = map[bool]string{true: "erased", false: "pending"}[e.Done[hd]]
	}
	out := map[string]any{
		"ref": e.Ref, "status": e.Status, "reason": e.Reason, "expiry_action": e.Action, "policy_period_days": e.PolicyDays,
		"requested_at": e.RequestedAt, "completed_at": e.CompletedAt, "holders": holders, "attempts": e.Attempts,
		"last_error": e.LastError, "last_tried_at": e.LastTriedAt,
	}
	if e.CandidateID != "" { // only while pending (FR-5.5)
		out["candidate_id"] = e.CandidateID
	}
	return out
}

// writeErasure answers 200 for a finished erasure and 202 for one still pending (FR-5.8).
func writeErasure(w http.ResponseWriter, e *domain.Erasure) {
	code := http.StatusOK
	if e.Status == domain.ErasurePending {
		code = http.StatusAccepted
	}
	write(w, code, erasureOut(e))
}

func (h handler) listErasures(w http.ResponseWriter, r *http.Request) {
	status := domain.ErasureStatus(r.URL.Query().Get("status"))
	if status != "" && status != domain.ErasurePending && status != domain.ErasureErased {
		write(w, http.StatusBadRequest, map[string]string{"error": "status must be pending or erased"})
		return
	}
	list, err := h.Erasures.ListErasures(r.Context(), status)
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]map[string]any, len(list))
	for i := range list {
		out[i] = erasureOut(&list[i])
	}
	write(w, http.StatusOK, map[string]any{"erasures": out})
}

func (h handler) getErasure(w http.ResponseWriter, r *http.Request) {
	e, err := h.Erasures.GetErasure(r.Context(), r.PathValue("ref"))
	if err != nil {
		fail(w, err)
		return
	}
	write(w, http.StatusOK, erasureOut(e))
}

func (h handler) sweepRetention(w http.ResponseWriter, r *http.Request) {
	report, ran, err := h.Sweeps.Retention(r.Context())
	sweepAnswer(w, report, ran, err)
}

// ---- access audit (FR-0.4, NFR-12) ----

type rejectionJSON struct {
	ID           string    `json:"id,omitempty"`
	MemberID     string    `json:"member_id"`
	Role         string    `json:"role"`
	RequiredRole string    `json:"required_role"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	OccurredAt   time.Time `json:"occurred_at"`
}

func (h handler) recordRejection(w http.ResponseWriter, r *http.Request) {
	var in rejectionJSON
	if !read(w, r, &in) {
		return
	}
	rec, err := h.AccessAudit.RecordAuthorisationRejected(r.Context(), domain.AccessRejection{
		MemberID: in.MemberID, Role: in.Role, RequiredRole: in.RequiredRole, Method: in.Method, Path: in.Path, OccurredAt: in.OccurredAt})
	if err != nil {
		fail(w, err)
		return
	}
	write(w, http.StatusCreated, rejectionJSON(rec))
}

func (h handler) listRejections(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := h.AccessAudit.ListAccessRejections(r.Context(), limit)
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]rejectionJSON, len(list))
	for i, a := range list {
		out[i] = rejectionJSON(a)
	}
	write(w, http.StatusOK, map[string]any{"rejections": out})
}

// ---- plumbing ----

func sweepAnswer(w http.ResponseWriter, report any, ran bool, err error) {
	switch {
	case err != nil:
		fail(w, err)
	case !ran:
		write(w, http.StatusConflict, map[string]string{"error": "a sweep is already running"})
	default:
		write(w, http.StatusOK, report)
	}
}

func read(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		write(w, http.StatusBadRequest, map[string]string{"error": "malformed JSON body: " + err.Error()})
		return false
	}
	return true
}

func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidPolicy), errors.Is(err, domain.ErrInvalidErasure), errors.Is(err, domain.ErrInvalidEvent):
		write(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, usecase.ErrNotFound):
		write(w, http.StatusNotFound, map[string]string{"error": "not found"})
	case errors.Is(err, usecase.ErrNoPolicy):
		write(w, http.StatusConflict, map[string]string{"error": err.Error()})
	default:
		slog.Error("request failed", "err", err)
		write(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

func write(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
