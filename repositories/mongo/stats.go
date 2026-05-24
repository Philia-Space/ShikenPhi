package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/philiaspace/shikenphi/internal/domain"
	examd "github.com/philiaspace/phi-exam-domain/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UserStatsRepository struct {
	client *mongo.Client
	dbName string
}

func NewUserStatsRepository(client *mongo.Client, dbName string) *UserStatsRepository {
	return &UserStatsRepository{client: client, dbName: dbName}
}

func (r *UserStatsRepository) statsColl() *mongo.Collection {
	return collection(r.client, r.dbName, "user_stats")
}

func (r *UserStatsRepository) streaksColl() *mongo.Collection {
	return collection(r.client, r.dbName, "user_streaks")
}

type userStatsDoc struct {
	UserID                 string  `bson:"_id"`
	TotalExams             int     `bson:"total_exams"`
	TotalQuestionsAnswered int     `bson:"total_questions_answered"`
	TotalCorrect           int     `bson:"total_correct"`
	AvgScore               float64 `bson:"avg_score"`
	BestScore              int     `bson:"best_score"`
	BestLevel              string  `bson:"best_level"`
	TotalXP                int     `bson:"total_xp"`
	CurrentRank            string  `bson:"current_rank"`
	CurrentStreak          int     `bson:"current_streak"`
	LongestStreak          int     `bson:"longest_streak"`
	UpdatedAt              time.Time `bson:"updated_at"`
}

type userStreakDoc struct {
	ID        string    `bson:"_id"`
	UserID    string    `bson:"user_id"`
	Date      time.Time `bson:"date"`
	ExamCount int       `bson:"exam_count"`
	TotalScore int       `bson:"total_score"`
	IsPerfect bool      `bson:"is_perfect"`
}

func (r *UserStatsRepository) FindByUserID(ctx context.Context, userID string) (*domain.UserStats, error) {
	var doc userStatsDoc
	err := r.statsColl().FindOne(ctx, bson.M{"_id": userID}).Decode(&doc)
	if err != nil {
		return nil, fmt.Errorf("stats not found for user: %s", userID)
	}
	return &domain.UserStats{
		UserID:                 doc.UserID,
		TotalExams:             doc.TotalExams,
		TotalQuestionsAnswered: doc.TotalQuestionsAnswered,
		TotalCorrect:           doc.TotalCorrect,
		AvgScore:               doc.AvgScore,
		BestScore:              doc.BestScore,
		BestLevel:              examd.JLPTLevel(doc.BestLevel),
		TotalXP:                doc.TotalXP,
		CurrentRank:            doc.CurrentRank,
		CurrentStreak:          doc.CurrentStreak,
		LongestStreak:          doc.LongestStreak,
		UpdatedAt:              doc.UpdatedAt,
	}, nil
}

func (r *UserStatsRepository) Save(ctx context.Context, stats *domain.UserStats) error {
	doc := userStatsDoc{
		UserID:                 stats.UserID,
		TotalExams:             stats.TotalExams,
		TotalQuestionsAnswered: stats.TotalQuestionsAnswered,
		TotalCorrect:           stats.TotalCorrect,
		AvgScore:               stats.AvgScore,
		BestScore:              stats.BestScore,
		BestLevel:              string(stats.BestLevel),
		TotalXP:                stats.TotalXP,
		CurrentRank:            stats.CurrentRank,
		CurrentStreak:          stats.CurrentStreak,
		LongestStreak:          stats.LongestStreak,
		UpdatedAt:              stats.UpdatedAt,
	}
	opts := options.Replace().SetUpsert(true)
	_, err := r.statsColl().ReplaceOne(ctx, bson.M{"_id": doc.UserID}, doc, opts)
	return err
}

func (r *UserStatsRepository) IncrementStreak(ctx context.Context, userID string, date time.Time, score int, isPerfect bool) error {
	key := fmt.Sprintf("%s_%s", userID, date.Format("2006-01-02"))
	filter := bson.M{"_id": key}
	update := bson.M{
		"$setOnInsert": bson.M{
			"user_id": userID,
			"date":    date,
		},
		"$inc": bson.M{
			"exam_count":  1,
			"total_score": score,
		},
	}
	if isPerfect {
		update["$set"] = bson.M{"is_perfect": true}
	}
	opts := options.Update().SetUpsert(true)
	_, err := r.streaksColl().UpdateOne(ctx, filter, update, opts)
	return err
}

type LeaderboardRepository struct {
	client *mongo.Client
	dbName string
}

func NewLeaderboardRepository(client *mongo.Client, dbName string) *LeaderboardRepository {
	return &LeaderboardRepository{client: client, dbName: dbName}
}

func (r *LeaderboardRepository) coll() *mongo.Collection {
	return collection(r.client, r.dbName, "leaderboard")
}

func (r *LeaderboardRepository) Get(ctx context.Context, period string, level examd.JLPTLevel, limit int) ([]domain.LeaderboardEntry, error) {
	filter := bson.M{"period": period}
	if level != "" {
		filter["level"] = string(level)
	}

	opts := options.Find().SetSort(bson.M{"total_score": -1})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}

	cursor, err := r.coll().Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []leaderboardEntryDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	entries := make([]domain.LeaderboardEntry, len(docs))
	for i, d := range docs {
		entries[i] = domain.LeaderboardEntry{
			UserID:        d.UserID,
			DisplayName:   d.DisplayName,
			TotalScore:    d.TotalScore,
			ExamCount:     d.ExamCount,
			AvgPercentage: d.AvgPercentage,
			Period:        d.Period,
			Level:         examd.JLPTLevel(d.Level),
			UpdatedAt:     d.UpdatedAt,
		}
	}
	return entries, nil
}

func (r *LeaderboardRepository) Refresh(ctx context.Context, period string) error {
	pipeline := []bson.M{
		{"$match": bson.M{"period": period}},
		{"$sort": bson.M{"total_score": -1}},
	}

	cursor, err := r.coll().Aggregate(ctx, pipeline)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	return nil
}

type leaderboardEntryDoc struct {
	UserID        string    `bson:"user_id"`
	DisplayName   string    `bson:"display_name"`
	TotalScore    int       `bson:"total_score"`
	ExamCount     int       `bson:"exam_count"`
	AvgPercentage int       `bson:"avg_percentage"`
	Period        string    `bson:"period"`
	Level         string    `bson:"level"`
	UpdatedAt     time.Time `bson:"updated_at"`
}
