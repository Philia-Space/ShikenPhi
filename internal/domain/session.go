package domain

import (
	"time"

	examd "github.com/philiaspace/phi-exam-domain/domain"
	"github.com/philiaspace/phi-core/domain"
)

// SessionStatus represents the lifecycle state of an exam session.
type SessionStatus string

const (
	Active    SessionStatus = "active"
	Completed SessionStatus = "completed"
	Expired   SessionStatus = "expired"
	Abandoned SessionStatus = "abandoned"
)

// Session is the aggregate root for an in-progress or completed exam.
type Session struct {
	domain.AggregateRoot
	ID               examd.SessionID
	UserID           string
	Level            examd.JLPTLevel
	TemplateID       string
	QuestionIDs      []examd.QuestionID
	OptionOrders     map[int][]int   // question index → shuffled option values
	UserAnswers      map[int]string  // question index → selected option value
	Status           SessionStatus
	StartedAt        time.Time
	CompletedAt      *time.Time
	ExpiresAt        time.Time
	Score            *int
	TimeSpentSeconds *int
}

// Result is the immutable record of a completed exam.
type Result struct {
	ID               examd.ResultID          `json:"id"`
	SessionID        examd.SessionID         `json:"session_id"`
	UserID           string                  `json:"user_id"`
	Level            examd.JLPTLevel         `json:"level"`
	Score            int                     `json:"score"`
	TotalQuestions   int                     `json:"total_questions"`
	Percentage       int                     `json:"percentage"`
	SectionBreakdown map[examd.Section]int   `json:"section_breakdown"`
	TimeSpentSeconds int                     `json:"time_spent_seconds"`
	CompletedAt      time.Time               `json:"completed_at"`
	QuestionReviews  []ResultQuestionReview  `json:"question_reviews"`
}

// ResultQuestionReview holds per-question scoring for post-exam review.
type ResultQuestionReview struct {
	QuestionID    string `json:"question_id"`
	Section       string `json:"section"`
	UserAnswer    string `json:"user_answer"`
	CorrectAnswer string `json:"correct_answer"`
	IsCorrect     bool   `json:"is_correct"`
}

// UserStats holds aggregated exam statistics for a user.
type UserStats struct {
	UserID                 string         `json:"user_id"`
	TotalExams             int            `json:"total_exams"`
	TotalQuestionsAnswered int            `json:"total_questions_answered"`
	TotalCorrect           int            `json:"total_correct"`
	AvgScore               float64        `json:"avg_score"`
	BestScore              int            `json:"best_score"`
	BestLevel              examd.JLPTLevel `json:"best_level"`
	TotalXP               int            `json:"total_xp"`
	CurrentRank            string         `json:"current_rank"`
	CurrentStreak          int            `json:"current_streak"`
	LongestStreak          int            `json:"longest_streak"`
	UpdatedAt              time.Time      `json:"updated_at"`
}

type UserStreak struct {
	UserID     string    `json:"user_id"`
	Date       time.Time `json:"date"`
	ExamCount  int       `json:"exam_count"`
	TotalScore int       `json:"total_score"`
	IsPerfect  bool      `json:"is_perfect"`
}

type LeaderboardEntry struct {
	UserID        string         `json:"user_id"`
	DisplayName   string         `json:"display_name"`
	TotalScore    int            `json:"total_score"`
	ExamCount     int            `json:"exam_count"`
	AvgPercentage int            `json:"avg_percentage"`
	Period        string         `json:"period"`
	Level         examd.JLPTLevel `json:"level"`
	UpdatedAt     time.Time      `json:"updated_at"`
}
