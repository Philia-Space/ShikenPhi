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

type SessionRepository struct {
	client *mongo.Client
	dbName string
}

func NewSessionRepository(client *mongo.Client, dbName string) *SessionRepository {
	return &SessionRepository{client: client, dbName: dbName}
}

func (r *SessionRepository) coll() *mongo.Collection {
	return collection(r.client, r.dbName, "sessions")
}

type sessionDoc struct {
	ID               string              `bson:"_id"`
	UserID           string              `bson:"user_id"`
	Level            string              `bson:"level"`
	TemplateID       string              `bson:"template_id"`
	QuestionIDs      []string            `bson:"question_ids"`
	OptionOrders      map[int][]int       `bson:"option_orders"`
	UserAnswers      map[int]string      `bson:"user_answers"`
	Status           string              `bson:"status"`
	StartedAt        time.Time           `bson:"started_at"`
	CompletedAt      *time.Time          `bson:"completed_at"`
	ExpiresAt        time.Time           `bson:"expires_at"`
	Score            *int                `bson:"score"`
	TimeSpentSeconds *int                `bson:"time_spent_seconds"`
	Version          int                 `bson:"version"`
}

func toSessionDoc(s *domain.Session) *sessionDoc {
	return &sessionDoc{
		ID:               string(s.ID),
		UserID:           s.UserID,
		Level:            string(s.Level),
		TemplateID:       s.TemplateID,
		QuestionIDs:      toStringSlice(s.QuestionIDs),
		OptionOrders:      s.OptionOrders,
		UserAnswers:      s.UserAnswers,
		Status:           string(s.Status),
		StartedAt:        s.StartedAt,
		CompletedAt:      s.CompletedAt,
		ExpiresAt:        s.ExpiresAt,
		Score:            s.Score,
		TimeSpentSeconds: s.TimeSpentSeconds,
		Version:          s.Version,
	}
}

func (d *sessionDoc) toDomain() *domain.Session {
	s := &domain.Session{
		ID:               examd.SessionID(d.ID),
		UserID:           d.UserID,
		Level:            examd.JLPTLevel(d.Level),
		TemplateID:       d.TemplateID,
		QuestionIDs:      toQuestionIDSlice(d.QuestionIDs),
		OptionOrders:      d.OptionOrders,
		UserAnswers:      d.UserAnswers,
		Status:           domain.SessionStatus(d.Status),
		StartedAt:        d.StartedAt,
		CompletedAt:      d.CompletedAt,
		ExpiresAt:        d.ExpiresAt,
		Score:            d.Score,
		TimeSpentSeconds: d.TimeSpentSeconds,
	}
	s.Version = d.Version
	return s
}

func (r *SessionRepository) FindByID(ctx context.Context, id string) (*domain.Session, error) {
	var doc sessionDoc
	err := r.coll().FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return doc.toDomain(), nil
}

func (r *SessionRepository) Save(ctx context.Context, session *domain.Session) error {
	doc := toSessionDoc(session)
	opts := options.Replace().SetUpsert(true)
	_, err := r.coll().ReplaceOne(ctx, bson.M{"_id": doc.ID}, doc, opts)
	return err
}

func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	_, err := r.coll().DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *SessionRepository) FindByUserID(ctx context.Context, userID string, status domain.SessionStatus, limit, offset int) ([]domain.Session, error) {
	filter := bson.M{"user_id": userID}
	if status != "" {
		filter["status"] = string(status)
	}

	opts := options.Find().SetSort(bson.M{"started_at": -1})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}
	if offset > 0 {
		opts.SetSkip(int64(offset))
	}

	cursor, err := r.coll().Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []sessionDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	sessions := make([]domain.Session, len(docs))
	for i, d := range docs {
		sessions[i] = *d.toDomain()
	}
	return sessions, nil
}

func (r *SessionRepository) FindActiveByUserID(ctx context.Context, userID string) (*domain.Session, error) {
	var doc sessionDoc
	err := r.coll().FindOne(ctx, bson.M{
		"user_id": userID,
		"status":  string(domain.Active),
	}).Decode(&doc)
	if err != nil {
		return nil, fmt.Errorf("no active session for user: %s", userID)
	}
	return doc.toDomain(), nil
}

func (r *SessionRepository) UpdateAnswers(ctx context.Context, id examd.SessionID, answers map[int]string, version int) error {
	filter := bson.M{"_id": string(id), "version": version}
	update := bson.M{
		"$set":   bson.M{"user_answers": answers},
		"$inc":   bson.M{"version": 1},
	}
	res, err := r.coll().UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("optimistic concurrency conflict for session: %s", id)
	}
	return nil
}

func (r *SessionRepository) Complete(ctx context.Context, id examd.SessionID, score, timeSpent int, completedAt time.Time, version int) error {
	now := time.Now()
	filter := bson.M{"_id": string(id), "version": version}
	update := bson.M{
		"$set": bson.M{
			"status":             string(domain.Completed),
			"score":             score,
			"time_spent_seconds": timeSpent,
			"completed_at":      completedAt,
		},
		"$inc": bson.M{"version": 1},
	}
	res, err := r.coll().UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		_ = now
		return fmt.Errorf("optimistic concurrency conflict for session: %s", id)
	}
	return nil
}

func (r *SessionRepository) ExpireOldSessions(ctx context.Context, before time.Time) (int, error) {
	filter := bson.M{
		"status":     string(domain.Active),
		"expires_at": bson.M{"$lt": before},
	}
	update := bson.M{"$set": bson.M{"status": string(domain.Expired)}}
	res, err := r.coll().UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, err
	}
	return int(res.ModifiedCount), nil
}

func toStringSlice(ids []examd.QuestionID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

func toQuestionIDSlice(ids []string) []examd.QuestionID {
	out := make([]examd.QuestionID, len(ids))
	for i, id := range ids {
		out[i] = examd.QuestionID(id)
	}
	return out
}
