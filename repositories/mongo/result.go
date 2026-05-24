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

type ResultRepository struct {
	client *mongo.Client
	dbName string
}

func NewResultRepository(client *mongo.Client, dbName string) *ResultRepository {
	return &ResultRepository{client: client, dbName: dbName}
}

func (r *ResultRepository) coll() *mongo.Collection {
	return collection(r.client, r.dbName, "results")
}

type resultDoc struct {
	ID               string                    `bson:"_id"`
	SessionID        string                    `bson:"session_id"`
	UserID           string                    `bson:"user_id"`
	Level            string                    `bson:"level"`
	Score            int                       `bson:"score"`
	TotalQuestions   int                       `bson:"total_questions"`
	Percentage       int                       `bson:"percentage"`
	SectionBreakdown map[string]int            `bson:"section_breakdown"`
	TimeSpentSeconds int                       `bson:"time_spent_seconds"`
	CompletedAt      time.Time                 `bson:"completed_at"`
	QuestionReviews  []reviewDoc               `bson:"question_reviews"`
}

type reviewDoc struct {
	QuestionID    string `bson:"question_id"`
	Section       string `bson:"section"`
	UserAnswer    string `bson:"user_answer"`
	CorrectAnswer string `bson:"correct_answer"`
	IsCorrect     bool   `bson:"is_correct"`
}

func toResultDoc(r *domain.Result) *resultDoc {
	d := &resultDoc{
		ID:               string(r.ID),
		SessionID:        string(r.SessionID),
		UserID:           r.UserID,
		Level:            string(r.Level),
		Score:            r.Score,
		TotalQuestions:   r.TotalQuestions,
		Percentage:       r.Percentage,
		SectionBreakdown: make(map[string]int),
		TimeSpentSeconds: r.TimeSpentSeconds,
		CompletedAt:      r.CompletedAt,
	}
	for k, v := range r.SectionBreakdown {
		d.SectionBreakdown[string(k)] = v
	}
	for _, qr := range r.QuestionReviews {
		d.QuestionReviews = append(d.QuestionReviews, reviewDoc{
			QuestionID:    qr.QuestionID,
			Section:       qr.Section,
			UserAnswer:    qr.UserAnswer,
			CorrectAnswer: qr.CorrectAnswer,
			IsCorrect:     qr.IsCorrect,
		})
	}
	return d
}

func (d *resultDoc) toDomain() *domain.Result {
	r := &domain.Result{
		ID:               examd.ResultID(d.ID),
		SessionID:        examd.SessionID(d.SessionID),
		UserID:           d.UserID,
		Level:            examd.JLPTLevel(d.Level),
		Score:            d.Score,
		TotalQuestions:   d.TotalQuestions,
		Percentage:       d.Percentage,
		SectionBreakdown: make(map[examd.Section]int),
		TimeSpentSeconds: d.TimeSpentSeconds,
		CompletedAt:      d.CompletedAt,
	}
	for k, v := range d.SectionBreakdown {
		r.SectionBreakdown[examd.Section(k)] = v
	}
	for _, rd := range d.QuestionReviews {
		r.QuestionReviews = append(r.QuestionReviews, domain.ResultQuestionReview{
			QuestionID:    rd.QuestionID,
			Section:       rd.Section,
			UserAnswer:    rd.UserAnswer,
			CorrectAnswer: rd.CorrectAnswer,
			IsCorrect:     rd.IsCorrect,
		})
	}
	return r
}

func (r *ResultRepository) FindByID(ctx context.Context, id string) (*domain.Result, error) {
	var doc resultDoc
	err := r.coll().FindOne(ctx, bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		return nil, fmt.Errorf("result not found: %s", id)
	}
	return doc.toDomain(), nil
}

func (r *ResultRepository) Save(ctx context.Context, result *domain.Result) error {
	doc := toResultDoc(result)
	opts := options.Replace().SetUpsert(true)
	_, err := r.coll().ReplaceOne(ctx, bson.M{"_id": doc.ID}, doc, opts)
	return err
}

func (r *ResultRepository) Delete(ctx context.Context, id string) error {
	_, err := r.coll().DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *ResultRepository) FindByUserID(ctx context.Context, userID string, level examd.JLPTLevel, limit, offset int) ([]domain.Result, error) {
	filter := bson.M{"user_id": userID}
	if level != "" {
		filter["level"] = string(level)
	}

	opts := options.Find().SetSort(bson.M{"completed_at": -1})
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

	var docs []resultDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	results := make([]domain.Result, len(docs))
	for i, d := range docs {
		results[i] = *d.toDomain()
	}
	return results, nil
}

func (r *ResultRepository) FindBySessionID(ctx context.Context, id examd.SessionID) (*domain.Result, error) {
	var doc resultDoc
	err := r.coll().FindOne(ctx, bson.M{"session_id": string(id)}).Decode(&doc)
	if err != nil {
		return nil, fmt.Errorf("result not found for session: %s", id)
	}
	return doc.toDomain(), nil
}
