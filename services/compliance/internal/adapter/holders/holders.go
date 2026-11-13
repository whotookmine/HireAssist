// Package holders is the REST client for the services that hold candidate data: the candidate
// register and profile erasure in Resume Processing, and erasure in Hiring (ADR-016).
package holders

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/frngrit/HireAssist/services/compliance/internal/usecase"
)

const timeout = 10 * time.Second

var client = &http.Client{Timeout: timeout}

// ResumeProcessing calls the Resume Processing Service.
type ResumeProcessing struct{ BaseURL string }

var (
	_ usecase.CandidateRegister = ResumeProcessing{}
	_ usecase.HolderEraser      = ResumeProcessing{}
	_ usecase.HolderEraser      = Hiring{}
)

// ListCandidates is listCandidateProfiles(): GET /candidates. The profile's created_at is the
// date its personal data was collected.
func (r ResumeProcessing) ListCandidates(ctx context.Context) ([]usecase.RegisteredCandidate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(r.BaseURL, "/")+"/candidates", nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, unexpected(res)
	}
	var body struct {
		Profiles []struct {
			ID           string    `json:"id"`
			JobOpeningID *string   `json:"job_opening_id"`
			CreatedAt    time.Time `json:"created_at"`
		} `json:"profiles"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("reading the candidate list: %w", err)
	}
	out := make([]usecase.RegisteredCandidate, 0, len(body.Profiles))
	for _, p := range body.Profiles {
		c := usecase.RegisteredCandidate{ID: p.ID, CollectedAt: p.CreatedAt}
		if p.JobOpeningID != nil {
			c.JobOpeningID = *p.JobOpeningID
		}
		out = append(out, c)
	}
	return out, nil
}

// EraseCandidate is eraseCandidateProfile(): DELETE /candidates/{id}. A 404 means Resume
// Processing holds nothing about the candidate, which is what erasure wants.
func (r ResumeProcessing) EraseCandidate(ctx context.Context, candidateID string) error {
	return erase(ctx, strings.TrimRight(r.BaseURL, "/")+"/candidates/"+url.PathEscape(candidateID),
		http.StatusOK, http.StatusNoContent, http.StatusNotFound)
}

// Hiring calls the Hiring Service.
type Hiring struct{ BaseURL string }

// EraseCandidate is eraseHiringData(): DELETE /candidates/{id}, answered 204 once Hiring holds
// nothing about the candidate — screening results, decisions, interview guides and the resume
// file. Only 204 counts: a 404 here could be a Hiring that does not serve the route yet, and
// mistaking that for success would report data as erased that is still held.
func (h Hiring) EraseCandidate(ctx context.Context, candidateID string) error {
	return erase(ctx, strings.TrimRight(h.BaseURL, "/")+"/candidates/"+url.PathEscape(candidateID), http.StatusNoContent)
}

func erase(ctx context.Context, target string, done ...int) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, target, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	for _, s := range done {
		if res.StatusCode == s {
			return nil
		}
	}
	return unexpected(res)
}

func unexpected(res *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(res.Body, 200))
	return fmt.Errorf("%s %s answered %d: %s", res.Request.Method, res.Request.URL.Path, res.StatusCode, strings.TrimSpace(string(b)))
}
