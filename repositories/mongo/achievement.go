package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/philiaspace/shikenphi/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AchievementMongoRepository struct {
	client *mongo.Client
	dbName string
}

func NewAchievementRepository(client *mongo.Client, dbName string) *AchievementMongoRepository {
	return &AchievementMongoRepository{client: client, dbName: dbName}
}

func (r *AchievementMongoRepository) coll() *mongo.Collection {
	return collection(r.client, r.dbName, "achievements")
}

type achievementDoc struct {
	ID         string    `bson:"_id"`
	UserID     string    `bson:"user_id"`
	Achievement string   `bson:"achievement"`
	UnlockedAt  time.Time `bson:"unlocked_at"`
	SessionID  string    `bson:"session_id"`
}

func (r *AchievementMongoRepository) FindByUserID(ctx context.Context, userID string) ([]domain.UserAchievement, error) {
	cursor, err := r.coll().Find(ctx, bson.M{"user_id": userID}, options.Find().SetSort(bson.M{"unlocked_at": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []achievementDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	result := make([]domain.UserAchievement, len(docs))
	for i, d := range docs {
		result[i] = domain.UserAchievement{
			UserID:      d.UserID,
			Achievement: domain.AchievementCode(d.Achievement),
			UnlockedAt:  d.UnlockedAt,
			SessionID:   d.SessionID,
		}
	}
	return result, nil
}

func (r *AchievementMongoRepository) Save(ctx context.Context, achievement domain.UserAchievement) error {
	id := fmt.Sprintf("%s_%s", achievement.UserID, string(achievement.Achievement))
	doc := achievementDoc{
		ID:          id,
		UserID:      achievement.UserID,
		Achievement: string(achievement.Achievement),
		UnlockedAt:  achievement.UnlockedAt,
		SessionID:   achievement.SessionID,
	}
	opts := options.Replace().SetUpsert(true)
	_, err := r.coll().ReplaceOne(ctx, bson.M{"_id": id}, doc, opts)
	return err
}

func (r *AchievementMongoRepository) HasAchievement(ctx context.Context, userID string, code domain.AchievementCode) (bool, error) {
	id := fmt.Sprintf("%s_%s", userID, string(code))
	err := r.coll().FindOne(ctx, bson.M{"_id": id}).Err()
	if err == mongo.ErrNoDocuments {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
