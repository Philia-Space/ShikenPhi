package application

import (
	"context"
	"time"

	"github.com/philiaspace/shikenphi/internal/domain"
	examd "github.com/philiaspace/phi-exam-domain/domain"
)

func UpdateStatsAfterSubmit(ctx context.Context, statsRepo domain.UserStatsRepository, result *domain.Result) error {
	stats, err := statsRepo.FindByUserID(ctx, result.UserID)
	if err != nil {
		stats = &domain.UserStats{
			UserID:        result.UserID,
			CurrentRank:   "Beginner",
			BestLevel:     result.Level,
		}
	}

	stats.TotalExams++
	stats.TotalQuestionsAnswered += result.TotalQuestions
	stats.TotalCorrect += result.Score

	if stats.TotalExams > 0 {
		stats.AvgScore = float64(stats.TotalCorrect) / float64(stats.TotalQuestionsAnswered) * 100
	}

	if result.Percentage > stats.BestScore {
		stats.BestScore = result.Percentage
	}

	if examd.IsHarderLevel(result.Level, stats.BestLevel) {
		stats.BestLevel = result.Level
	}

	xpEarned := examd.CalculateXP(result.Score)
	stats.TotalXP += xpEarned
	stats.CurrentRank = examd.CalculateRank(stats.TotalXP)

	stats.UpdatedAt = time.Now()

	return statsRepo.Save(ctx, stats)
}
