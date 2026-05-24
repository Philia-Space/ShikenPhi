package memory

import (
	"context"
	"sync"

	"github.com/philiaspace/shikenphi/internal/domain"
)

type AchievementRepository struct {
	mu           sync.RWMutex
	achievements map[string][]domain.UserAchievement
}

func NewAchievementRepository() *AchievementRepository {
	return &AchievementRepository{
		achievements: make(map[string][]domain.UserAchievement),
	}
}

func (r *AchievementRepository) FindByUserID(ctx context.Context, userID string) ([]domain.UserAchievement, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]domain.UserAchievement, len(r.achievements[userID]))
	copy(result, r.achievements[userID])
	return result, nil
}

func (r *AchievementRepository) Save(ctx context.Context, achievement domain.UserAchievement) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.achievements[achievement.UserID] = append(r.achievements[achievement.UserID], achievement)
	return nil
}

func (r *AchievementRepository) HasAchievement(ctx context.Context, userID string, code domain.AchievementCode) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, a := range r.achievements[userID] {
		if a.Achievement == code {
			return true, nil
		}
	}
	return false, nil
}
