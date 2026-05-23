package domain

import (
	"context"
	"time"

	examd "github.com/philiaspace/phi-exam-domain/domain"
	"github.com/philiaspace/phi-core/domain"
)

// SessionRepository defines the contract for session persistence.
type SessionRepository interface {
	domain.Repository[Session]
	FindByUserID(ctx context.Context, userID string, status SessionStatus, limit, offset int) ([]Session, error)
	FindActiveByUserID(ctx context.Context, userID string) (*Session, error)
	UpdateAnswers(ctx context.Context, id examd.SessionID, answers map[int]string, version int) error
	Complete(ctx context.Context, id examd.SessionID, score, timeSpent int, completedAt time.Time, version int) error
	ExpireOldSessions(ctx context.Context, before time.Time) (int, error)
}

// ResultRepository defines the contract for result persistence.
type ResultRepository interface {
	domain.Repository[Result]
	FindByUserID(ctx context.Context, userID string, level examd.JLPTLevel, limit, offset int) ([]Result, error)
	FindBySessionID(ctx context.Context, id examd.SessionID) (*Result, error)
}

// UserStatsRepository defines the contract for user stats persistence.
type UserStatsRepository interface {
	FindByUserID(ctx context.Context, userID string) (*UserStats, error)
	Save(ctx context.Context, stats *UserStats) error
	IncrementStreak(ctx context.Context, userID string, date time.Time, score int, isPerfect bool) error
}

// LeaderboardRepository defines the contract for leaderboard queries.
type LeaderboardRepository interface {
	Get(ctx context.Context, period string, level examd.JLPTLevel, limit int) ([]LeaderboardEntry, error)
	Refresh(ctx context.Context, period string) error
}
