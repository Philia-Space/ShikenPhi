package domain

import (
	"context"
	"time"
)

type AchievementCode string

type UserAchievement struct {
	UserID      string
	Achievement AchievementCode
	UnlockedAt  time.Time
	SessionID  string
}

type AchievementRepository interface {
	FindByUserID(ctx context.Context, userID string) ([]UserAchievement, error)
	Save(ctx context.Context, achievement UserAchievement) error
	HasAchievement(ctx context.Context, userID string, code AchievementCode) (bool, error)
}
