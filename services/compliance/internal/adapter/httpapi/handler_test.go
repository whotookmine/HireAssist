package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/frngrit/HireAssist/services/compliance/internal/adapter/memory"
	"github.com/frngrit/HireAssist/services/compliance/internal/domain"
	"github.com/frngrit/HireAssist/services/compliance/internal/usecase"
)

type clock struct{}

func (clock) Now() time.Time { return time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) }

type ids struct{}

func (ids) NewID(p string) string { return p + "-1" }

type eraser struct{ err error }

func (e eraser) EraseCandidate(context.Context, string) error { return e.err }

type sweeps struct{ ran bool }

func (s sweeps) Retention(context.Context) (usecase.RetentionReport, bool, error) {
	return usecase.RetentionReport{}, s.ran, nil
}
func (s sweeps) Staleness(context.Context) (usecase.StalenessReport, bool, error) {
	return usecase.StalenessReport{}, s.ran, nil
}

func newServer(profilesErr error, sweepRan bool) http.Handler {
	store := memory.New()
	erasures := &usecase.Erasures{Store: store, IDs: ids{}, Clock: clock{}, Holders: map[domain.Holder]usecase.HolderEraser{
		domain.HolderHiring: eraser{}, domain.HolderResumeProcessing: eraser{profilesErr}}}
	return NewHandler(Services{
		Pipeline:    &usecase.Pipeline{Store: store, Clock: clock{}},
		Retention:   &usecase.Retention{Policies: store, Marks: store, Pipeline: store, Erasures: erasures, Clock: clock{}},
		Erasures:    erasures,
		AccessAudit: &usecase.AccessAudit{Log: store, IDs: ids{}, Clock: clock{}},
		Sweeps:      sweeps{sweepRan},
		Ready:       func(context.Context) map[string]bool { return map[string]bool{"postgres": true} },
	})
}

func call(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestStatusCodes(t *testing.T) {
	h := newServer(nil, true)
	for _, tc := range []struct {
		method, path, body string
		want               int
		contains           string
	}{
		{"GET", "/compliance/retention-policy", "", 404, ""},
		{"PUT", "/compliance/retention-policy", `{"period_days":0,"anchor":"collected","expiry_action":"delete"}`, 400, "period"},
		{"PUT", "/compliance/retention-policy", `{"period_days":180,"anchor":"collected","expiry_action":"delete","colour":"red"}`, 400, "unknown field"},
		{"PUT", "/compliance/retention-policy", `{"period_days":180,"anchor":"collected","expiry_action":"anonymise","warning_window_days":14}`, 200, `"expiry_action":"anonymise"`},
		{"GET", "/compliance/retention-policy", "", 200, `"period_days":180`},
		{"POST", "/compliance/erasures", `{"candidate_id":""}`, 400, "candidate_id"},
		{"POST", "/compliance/erasures", `{"candidate_id":"cand-7"}`, 200, `"status":"erased"`},
		{"GET", "/compliance/erasures/era-1", "", 200, `"policy_period_days":180`},
		{"GET", "/compliance/erasures?status=bogus", "", 400, ""},
		{"POST", "/compliance/held-expiries/cand-x", `{"decision":"erase"}`, 404, ""},
		{"POST", "/internal/access-rejections", `{"member_id":"m-1","role":"recruiter","required_role":"admin","method":"PUT","path":"/members/m-2"}`, 201, `"id":"acc-1"`},
		{"GET", "/compliance/access-audit", "", 200, `"required_role":"admin"`},
		{"GET", "/compliance/dashboard", "", 200, `"openings":[]`},
		{"GET", "/readyz", "", 200, `"ready"`},
	} {
		rec := call(h, tc.method, tc.path, tc.body)
		if rec.Code != tc.want || !strings.Contains(rec.Body.String(), tc.contains) {
			t.Errorf("%s %s: %d %s, want %d containing %q", tc.method, tc.path, rec.Code, rec.Body, tc.want, tc.contains)
		}
	}
	if rec := call(h, "GET", "/compliance/erasures/era-1", ""); strings.Contains(rec.Body.String(), "cand-7") {
		t.Error("a finished erasure still names the candidate (FR-5.5)")
	}
}

func TestPendingErasureIsAccepted(t *testing.T) {
	h := newServer(errors.New("down"), true)
	rec := call(h, "POST", "/compliance/erasures", `{"candidate_id":"cand-7"}`)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"resume_processing":"pending"`) ||
		!strings.Contains(rec.Body.String(), `"candidate_id":"cand-7"`) {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
}

func TestSweepAlreadyRunning(t *testing.T) {
	if rec := call(newServer(nil, false), "POST", "/compliance/sweeps/retention", ""); rec.Code != http.StatusConflict {
		t.Errorf("got %d", rec.Code)
	}
}
