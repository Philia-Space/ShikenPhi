package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/philiaspace/shikenphi/internal/domain"
	examd "github.com/philiaspace/phi-exam-domain/domain"
)

// In-memory implementations for ResultRepository, UserStatsRepository, and LeaderboardRepository.

type InMemoryResultRepository struct {
	mu      sync.RWMutex
	results map[string]*domain.Result
}

func NewInMemoryResultRepository() *InMemoryResultRepository {
	return &InMemoryResultRepository{
		results: make(map[string]*domain.Result),
	}
}

func (r *InMemoryResultRepository) FindByID(ctx context.Context, id string) (*domain.Result, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	result, ok := r.results[id]
	if !ok {
		return nil, fmt.Errorf("result not found: %s", id)
	}
	return result, nil
}

func (r *InMemoryResultRepository) Save(ctx context.Context, result *domain.Result) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.results[string(result.ID)] = result
	return nil
}

func (r *InMemoryResultRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	delete(r.results, id)
	return nil
}

func (r *InMemoryResultRepository) FindByUserID(ctx context.Context, userID string, level examd.JLPTLevel, limit, offset int) ([]domain.Result, error) {
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

func (r *InMemoryResultRepository) FindBySessionID(ctx context.Context, id examd.SessionID) (*domain.Result, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	for _, res := range r.results {
		if res.SessionID == id {
			return res, nil
		}
	}
	return nil, fmt.Errorf("result not found for session: %s", id)
}

type InMemoryUserStatsRepository struct {
	mu     sync.RWMutex
	stats  map[string]*domain.UserStats
	streaks map[string]*domain.UserStreak
}

func NewInMemoryUserStatsRepository() *InMemoryUserStatsRepository {
	return &InMemoryUserStatsRepository{
		stats:   make(map[string]*domain.UserStats),
		streaks: make(map[string]*domain.UserStreak),
	}
}

func (r *InMemoryUserStatsRepository) FindByUserID(ctx context.Context, userID string) (*domain.UserStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	stats, ok := r.stats[userID]
	if !ok {
		return nil, fmt.Errorf("stats not found for user: %s", userID)
	}
	return stats, nil
}

func (r *InMemoryUserStatsRepository) Save(ctx context.Context, stats *domain.UserStats) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.stats[stats.UserID] = stats
	return nil
}

func (r *InMemoryUserStatsRepository) IncrementStreak(ctx context.Context, userID string, date time.Time, score int, isPerfect bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	key := fmt.Sprintf("%s_%s", userID, date.Format("2006-01-02"))
	streak, ok := r.streaks[key]
	if !ok {
		streak = &domain.UserStreak{
			UserID: userID,
			Date:   date,
		}
	}
	streak.ExamCount++
	streak.TotalScore += score
	if isPerfect {
		streak.IsPerfect = true
	}
	r.streaks[key] = streak
	return nil
}

type InMemoryLeaderboardRepository struct {
	mu      sync.RWMutex
	entries []domain.LeaderboardEntry
}

func NewInMemoryLeaderboardRepository() *InMemoryLeaderboardRepository {
	return &InMemoryLeaderboardRepository{
		entries: []domain.LeaderboardEntry{
			{
				UserID:        "user_1",
				DisplayName:   "Raychi",
				TotalScore:    2340,
				ExamCount:     34,
				AvgPercentage: 72,
				Period:        "alltime",
				Level:         examd.N3,
			},
			{
				UserID:        "user_2",
				DisplayName:   "Ayumu",
				TotalScore:    1850,
				ExamCount:     25,
				AvgPercentage: 65,
				Period:        "alltime",
				Level:         examd.N4,
			},
		},
	}
}

func (r *InMemoryLeaderboardRepository) Get(ctx context.Context, period string, level examd.JLPTLevel, limit int) ([]domain.LeaderboardEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var filtered []domain.LeaderboardEntry
	for _, entry := range r.entries {
		if entry.Period == period && (level == "" || entry.Level == level) {
			filtered = append(filtered, entry)
		}
	}
	
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

func (r *InMemoryLeaderboardRepository) Refresh(ctx context.Context, period string) error {
	// TODO: Implement background refresh
	return nil
}
