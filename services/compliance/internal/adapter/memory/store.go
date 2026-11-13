// Package memory implements every storage port in memory. The tests use it; the service itself
// refuses to start without PostgreSQL, because an audit log that vanishes on restart is not one.
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/frngrit/HireAssist/services/compliance/internal/domain"
	"github.com/frngrit/HireAssist/services/compliance/internal/usecase"
)

type entryKey struct {
	opening string
	stage   domain.Stage
	resume  string
}

type entry struct {
	candidateID string
	at          time.Time
}

// Store is safe for concurrent use.
type Store struct {
	mu         sync.Mutex
	policy     *domain.RetentionPolicy
	openings   map[string]domain.Opening
	entries    map[entryKey]entry
	holds      map[string]domain.Hold
	warnedFor  map[string]time.Time
	keepUntil  map[string]time.Time
	erasures   map[string]domain.Erasure
	rejections []domain.AccessRejection
}

// New returns an empty store.
func New() *Store {
	return &Store{
		openings: map[string]domain.Opening{}, entries: map[entryKey]entry{}, holds: map[string]domain.Hold{},
		warnedFor: map[string]time.Time{}, keepUntil: map[string]time.Time{}, erasures: map[string]domain.Erasure{},
	}
}

var (
	_ usecase.PolicyStore    = (*Store)(nil)
	_ usecase.PipelineStore  = (*Store)(nil)
	_ usecase.RetentionStore = (*Store)(nil)
	_ usecase.ErasureStore   = (*Store)(nil)
	_ usecase.AccessLog      = (*Store)(nil)
)

func (s *Store) GetPolicy(context.Context) (*domain.RetentionPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.policy == nil {
		return nil, nil
	}
	p := *s.policy
	return &p, nil
}

func (s *Store) SavePolicy(_ context.Context, p domain.RetentionPolicy) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policy = &p
	return nil
}

func (s *Store) GetOpening(_ context.Context, id string) (*domain.Opening, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.openings[id]
	if !ok {
		return nil, nil
	}
	o.Counts = s.counts(id)
	return &o, nil
}

func (s *Store) SaveOpening(_ context.Context, o *domain.Opening) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, existed := s.openings[o.ID]
	if existed && prev.Status == domain.StatusClosed {
		return nil
	}
	c := *o
	c.Counts = nil
	c.FlaggedAt = prev.FlaggedAt
	if c.Status == domain.StatusClosed {
		c.FlaggedAt = nil
	}
	s.openings[o.ID] = c
	return nil
}

func (s *Store) SetFlag(_ context.Context, id string, flaggedAt *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o, ok := s.openings[id]; ok && o.Status != domain.StatusClosed {
		o.FlaggedAt = flaggedAt
		s.openings[id] = o
	}
	return nil
}

func (s *Store) AddEntry(_ context.Context, openingID string, stage domain.Stage, resumeID, candidateID string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := entryKey{openingID, stage, resumeID}
	if _, ok := s.entries[k]; !ok {
		s.entries[k] = entry{candidateID, at}
	}
	return nil
}

func (s *Store) counts(openingID string) map[domain.Stage]int {
	c := map[domain.Stage]int{domain.StageApplied: 0, domain.StageScreened: 0, domain.StageShortlisted: 0}
	for k := range s.entries {
		if k.opening == openingID {
			c[k.stage]++
		}
	}
	return c
}

func (s *Store) ListOpenings(context.Context) ([]domain.Opening, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []domain.Opening{}
	for _, o := range s.openings {
		if o.Status != domain.StatusClosed {
			o.Counts = s.counts(o.ID)
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *Store) LastActivity(_ context.Context, candidateID string) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var last time.Time
	for _, e := range s.entries {
		if e.candidateID == candidateID && e.at.After(last) {
			last = e.at
		}
	}
	return last, nil
}

func (s *Store) SaveHold(_ context.Context, h domain.Hold) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.holds[h.CandidateID] = h
	return nil
}

func (s *Store) GetHold(_ context.Context, candidateID string) (*domain.Hold, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h, ok := s.holds[candidateID]
	if !ok {
		return nil, nil
	}
	return &h, nil
}

func (s *Store) ListHolds(context.Context) ([]domain.Hold, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []domain.Hold{}
	for _, h := range s.holds {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExpiresAt.Before(out[j].ExpiresAt) })
	return out, nil
}

func (s *Store) DeleteHold(_ context.Context, candidateID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.holds, candidateID)
	return nil
}

func (s *Store) Marks(_ context.Context, candidateID string) (time.Time, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.warnedFor[candidateID], s.keepUntil[candidateID], nil
}

func (s *Store) SetWarned(_ context.Context, candidateID string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.warnedFor[candidateID] = expiresAt
	return nil
}

func (s *Store) SetKeepUntil(_ context.Context, candidateID string, until time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keepUntil[candidateID] = until
	return nil
}

func copyErasure(e domain.Erasure) domain.Erasure {
	done := make(map[domain.Holder]bool, len(e.Done))
	for k, v := range e.Done {
		done[k] = v
	}
	e.Done = done
	return e
}

func (s *Store) SaveErasure(_ context.Context, e *domain.Erasure) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.erasures[e.Ref] = copyErasure(*e)
	return nil
}

func (s *Store) GetErasure(_ context.Context, ref string) (*domain.Erasure, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.erasures[ref]
	if !ok {
		return nil, usecase.ErrNotFound
	}
	e = copyErasure(e)
	return &e, nil
}

func (s *Store) PendingErasureFor(_ context.Context, candidateID string) (*domain.Erasure, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.erasures {
		if e.Status == domain.ErasurePending && e.CandidateID == candidateID {
			e = copyErasure(e)
			return &e, nil
		}
	}
	return nil, nil
}

func (s *Store) ListErasures(_ context.Context, status domain.ErasureStatus) ([]domain.Erasure, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []domain.Erasure{}
	for _, e := range s.erasures {
		if status == "" || e.Status == status {
			out = append(out, copyErasure(e))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RequestedAt.After(out[j].RequestedAt) })
	return out, nil
}

func (s *Store) ForgetCandidate(_ context.Context, candidateID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, e := range s.entries {
		if e.candidateID == candidateID {
			e.candidateID = ""
			s.entries[k] = e
		}
	}
	delete(s.holds, candidateID)
	delete(s.warnedFor, candidateID)
	delete(s.keepUntil, candidateID)
	return nil
}

func (s *Store) RecordRejection(_ context.Context, r domain.AccessRejection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rejections = append(s.rejections, r)
	return nil
}

func (s *Store) ListRejections(_ context.Context, limit int) ([]domain.AccessRejection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []domain.AccessRejection{}
	for i := len(s.rejections) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.rejections[i])
	}
	return out, nil
}
