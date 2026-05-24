package application

import (
	"context"
	"fmt"

	"github.com/philiaspace/phi-gamification"
	"github.com/philiaspace/shikenphi/internal/domain"
	examd "github.com/philiaspace/phi-exam-domain/domain"
)

type AchievementService struct {
	achievementRepo domain.AchievementRepository
}

func NewAchievementService(achievementRepo domain.AchievementRepository) *AchievementService {
	return &AchievementService{
		achievementRepo: achievementRepo,
	}
}

func (s *AchievementService) EvaluateAfterSubmit(ctx context.Context, result *domain.Result, stats *domain.UserStats, sessionID string) ([]domain.UserAchievement, error) {
	existing, err := s.achievementRepo.FindByUserID(ctx, result.UserID)
	if err != nil {
		existing = nil
	}

	existingMap := make(map[domain.AchievementCode]bool)
	for _, a := range existing {
		existingMap[a.Achievement] = true
	}

	evalCtx := gamification.EvaluateContext{
		TotalExams:           stats.TotalExams,
		CurrentStreak:       stats.CurrentStreak,
		ResultPercentage:    result.Percentage,
		ResultLevel:         result.Level,
		ResultTimeSpent:     result.TimeSpentSeconds,
		ResultCompletedAt:   result.CompletedAt,
		ExistingAchievements: make(map[gamification.AchievementCode]bool),
	}

	for code := range existingMap {
		evalCtx.ExistingAchievements[gamification.AchievementCode(code)] = true
	}

	unlocked := gamification.Evaluate(evalCtx)

	var resultAchievements []domain.UserAchievement
	for _, ua := range unlocked {
		domainUA := domain.UserAchievement{
			UserID:      result.UserID,
			Achievement: domain.AchievementCode(ua.Achievement),
			UnlockedAt:  ua.UnlockedAt,
			SessionID:  string(sessionID),
		}

		if err := s.achievementRepo.Save(ctx, domainUA); err != nil {
			return nil, fmt.Errorf("failed to save achievement %s: %w", ua.Achievement, err)
		}

		resultAchievements = append(resultAchievements, domainUA)
	}

	if len(resultAchievements) > 0 {
		bonusXP := gamification.BonusXP(unlocked)
		if bonusXP > 0 && stats != nil {
			stats.TotalXP += bonusXP
			stats.CurrentRank = examd.CalculateRank(stats.TotalXP)
		}
	}

	return resultAchievements, nil
}
