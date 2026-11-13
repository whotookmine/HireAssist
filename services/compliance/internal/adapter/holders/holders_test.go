package holders

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func serve(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestWhatCountsAsErased(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		eraser func(url string) interface {
			EraseCandidate(context.Context, string) error
		}
		status int
		ok     bool
	}{
		{"resume processing deleted", rp, http.StatusOK, true},
		{"resume processing never held it", rp, http.StatusNotFound, true},
		{"resume processing down", rp, http.StatusServiceUnavailable, false},
		{"hiring erased", hiring, http.StatusNoContent, true},
		// A Hiring without the route answers 404 too; that must never read as erased.
		{"hiring 404", hiring, http.StatusNotFound, false},
		{"hiring 200", hiring, http.StatusOK, false},
	} {
		srv := serve(tc.status, "")
		err := tc.eraser(srv.URL).EraseCandidate(ctx, "cand-1")
		srv.Close()
		if (err == nil) != tc.ok {
			t.Errorf("%s: err = %v", tc.name, err)
		}
	}
}

func rp(u string) interface {
	EraseCandidate(context.Context, string) error
} {
	return ResumeProcessing{BaseURL: u}
}

func hiring(u string) interface {
	EraseCandidate(context.Context, string) error
} {
	return Hiring{BaseURL: u}
}

func TestListCandidates(t *testing.T) {
	srv := serve(http.StatusOK, `{"profiles":[
		{"id":"cand-1","job_opening_id":"job-1","created_at":"2026-01-02T03:04:05.000Z","full_name":"ignored"},
		{"id":"cand-2","job_opening_id":null,"created_at":"2026-02-01T00:00:00.000Z"}]}`)
	defer srv.Close()
	got, err := ResumeProcessing{BaseURL: srv.URL}.ListCandidates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].JobOpeningID != "job-1" || got[1].JobOpeningID != "" || got[0].CollectedAt.Day() != 2 {
		t.Errorf("got %+v", got)
	}
}
