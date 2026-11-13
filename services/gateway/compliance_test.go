package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestComplianceRoutesNeedTheRightRole(t *testing.T) {
	s := newSigner(t, "2026-10")
	parsed, err := parsePublicKeys(s.config(t))
	if err != nil {
		t.Fatal(err)
	}
	compliance := newUpstream(t, `{"service":"compliance"}`)
	other := newUpstream(t, "{}")
	g := &gateway{
		routes:   routeTable(other.url(t), other.url(t), compliance.url(t)),
		verifier: &verifier{keys: parsed},
		limiter:  newRateLimiter(1000, 1000),
	}
	h := g.handler()
	recruiter := s.token(t, "m-r", roleRecruiter, time.Minute)
	admin := s.token(t, "m-a", roleAdmin, time.Minute)

	cases := []struct {
		method, path, token string
		want                int
	}{
		{"GET", "/compliance/dashboard", recruiter, http.StatusOK},
		{"GET", "/compliance/held-expiries", recruiter, http.StatusOK},
		{"POST", "/compliance/held-expiries/cand-1", recruiter, http.StatusOK},
		{"PUT", "/compliance/retention-policy", recruiter, http.StatusForbidden},
		{"POST", "/compliance/erasures", recruiter, http.StatusForbidden},
		{"GET", "/compliance/access-audit", recruiter, http.StatusForbidden},
		{"POST", "/compliance/sweeps/retention", recruiter, http.StatusForbidden},
		{"PUT", "/compliance/retention-policy", admin, http.StatusOK},
		{"GET", "/compliance/dashboard", "", http.StatusUnauthorized},
		// The audit log's write route is the gateway's alone: never routed, so never forged.
		{"POST", "/internal/access-rejections", admin, http.StatusNotFound},
	}
	for _, c := range cases {
		if got := do(t, h, c.method, c.path, c.token).Code; got != c.want {
			t.Errorf("%s %s: %d, want %d", c.method, c.path, got, c.want)
		}
	}
}

func TestARoleRejectionIsSentToTheAccessAuditLog(t *testing.T) {
	s := newSigner(t, "2026-10")
	parsed, err := parsePublicKeys(s.config(t))
	if err != nil {
		t.Fatal(err)
	}
	var (
		mu   sync.Mutex
		got  []map[string]any
		path string
	)
	audit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(raw, &body)
		mu.Lock()
		got, path = append(got, body), r.URL.Path
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(audit.Close)
	auditURL, _ := url.Parse(audit.URL)

	var done sync.WaitGroup
	other := newUpstream(t, "{}")
	g := &gateway{
		routes:   routeTable(other.url(t), other.url(t), auditURL),
		verifier: &verifier{keys: parsed},
		limiter:  newRateLimiter(1000, 1000),
		audit:    auditURL,
		reported: done.Done,
	}
	h := g.handler()

	done.Add(1)
	if code := do(t, h, "GET", "/members", s.token(t, "m-r", roleRecruiter, time.Minute)).Code; code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", code)
	}
	done.Wait()
	// A request with no token names nobody, so it is logged but not sent.
	do(t, h, "GET", "/members", "")

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 {
		t.Fatalf("%d reports, want 1", len(got))
	}
	if path != "/internal/access-rejections" {
		t.Errorf("posted to %q", path)
	}
	r := got[0]
	if r["member_id"] != "m-r" || r["role"] != roleRecruiter || r["required_role"] != roleAdmin ||
		r["method"] != "GET" || r["path"] != "/members" || r["occurred_at"] == nil {
		t.Errorf("report %v", r)
	}
}
