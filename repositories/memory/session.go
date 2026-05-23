package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/philiaspace/shikenphi/internal/domain"
	examd "github.com/philiaspace/phi-exam-domain/domain"
)

// SessionRepository is an in-memory implementation for testing.
type SessionRepository struct {
	mu       sync.RWMutex
	sessions map[string]*domain.Session
	results  map[string]*domain.Result
}

// NewSessionRepository creates a new in-memory repository.
func NewSessionRepository() *SessionRepository {
	return &SessionRepository{
		sessions: make(map[string]*domain.Session),
		results:  make(map[string]*domain.Result),
	}
}

func (r *SessionRepository) FindByID(ctx context.Context, id string) (*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	session, ok := r.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return session, nil
}

func (r *SessionRepository) Save(ctx context.Context, session *domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.sessions[string(session.ID)] = session
	return nil
}

func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	delete(r.sessions, id)
	return nil
}

func (r *SessionRepository) FindByUserID(ctx context.Context, userID string, status domain.SessionStatus, limit, offset int) ([]domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var sessions []domain.Session
	for _, s := range r.sessions {
		if s.UserID == userID && (status == "" || s.Status == status) {
			sessions = append(sessions, *s)
		}
	}
	return sessions, nil
}

func (r *SessionRepository) FindActiveByUserID(ctx context.Context, userID string) (*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	for _, s := range r.sessions {
		if s.UserID == userID && s.Status == domain.Active {
			return s, nil
		}
	}
	return nil, fmt.Errorf("no active session found for user: %s", userID)
}

func (r *SessionRepository) UpdateAnswers(ctx context.Context, id examd.SessionID, answers map[int]string, version int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	session, ok := r.sessions[string(id)]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	
	if session.Version != version {
		return fmt.Errorf("concurrency conflict")
	}
	
	for k, v := range answers {
		session.UserAnswers[k] = v
	}
	session.Version++
	return nil
}

func (r *SessionRepository) Complete(ctx context.Context, id examd.SessionID, score, timeSpent int, completedAt time.Time, version int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	session, ok := r.sessions[string(id)]
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	
	if session.Version != version {
		return fmt.Errorf("concurrency conflict")
	}
	
	session.Status = domain.Completed
	session.Score = &score
	session.TimeSpentSeconds = &timeSpent
	session.CompletedAt = &completedAt
	session.Version++
	return nil
}

func (r *SessionRepository) ExpireOldSessions(ctx context.Context, before time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	count := 0
	for _, s := range r.sessions {
		if s.Status == domain.Active && s.ExpiresAt.Before(before) {
			s.Status = domain.Expired
			count++
		}
	}
	return count, nil
}

// ResultRepository methods
func (r *SessionRepository) SaveResult(ctx context.Context, result *domain.Result) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.results[string(result.ID)] = result
	return nil
}

func (r *SessionRepository) FindResultsByUserID(ctx context.Context, userID string, level examd.JLPTLevel, limit, offset int) ([]domain.Result, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var results []domain.Result
	for _, res := range r.results {
		if res.UserID == userID && (level == "" || res.Level == level) {
			results = append(results, *res)
		}
	}
	return results, nil
}

func (r *SessionRepository) FindResultBySessionID(ctx context.Context, id examd.SessionID) (*domain.Result, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	for _, res := range r.results {
		if res.SessionID == id {
			return res, nil
		}
	}
	return nil, fmt.Errorf("result not found for session: %s", id)
}
