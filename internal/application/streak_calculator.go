package application

import (
	"context"
	"time"

	"github.com/philiaspace/shikenphi/internal/domain"
)

func UpdateStreakAfterSubmit(ctx context.Context, statsRepo domain.UserStatsRepository, result *domain.Result) error {
	today := time.Now().UTC().Truncate(24 * time.Hour)

	isPerfect := result.Percentage == 100

	if err := statsRepo.IncrementStreak(ctx, result.UserID, today, result.Score, isPerfect); err != nil {
		return err
	}

	stats, err := statsRepo.FindByUserID(ctx, result.UserID)
	if err != nil {
		return nil
	}

	currentStreak := calculateCurrentStreak(today, stats)
	stats.CurrentStreak = currentStreak
	if currentStreak > stats.LongestStreak {
		stats.LongestStreak = currentStreak
	}
	stats.LastExamDate = time.Now().UTC()
	stats.UpdatedAt = time.Now()

	return statsRepo.Save(ctx, stats)
}

func calculateCurrentStreak(today time.Time, stats *domain.UserStats) int {
	if stats.CurrentStreak == 0 {
		return 1
	}

	// Use LastExamDate if available, otherwise fall back to UpdatedAt
	lastExam := stats.LastExamDate
	if lastExam.IsZero() {
		lastExam = stats.UpdatedAt
	}
	if lastExam.IsZero() {
		return 1
	}

	lastExamDay := lastExam.UTC().Truncate(24 * time.Hour)
	daysSinceLastExam := int(today.Sub(lastExamDay).Hours() / 24)

	// If last exam was today or yesterday, streak continues
	if daysSinceLastExam <= 1 {
		return stats.CurrentStreak + 1
	}

	// Streak broken — start new
	return 1
}
