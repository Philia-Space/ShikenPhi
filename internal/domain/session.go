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
	ID               examd.ResultID
	SessionID        examd.SessionID
	UserID           string
	Level            examd.JLPTLevel
	Score            int
	TotalQuestions   int
	Percentage       int
	SectionBreakdown map[examd.Section]int
	TimeSpentSeconds int
	CompletedAt      time.Time
}

// UserStats holds aggregated exam statistics for a user.
type UserStats struct {
	UserID               string
	TotalExams           int
	TotalQuestionsAnswered int
	TotalCorrect         int
	AvgScore             float64
	BestScore            int
	BestLevel            examd.JLPTLevel
	TotalXP              int
	CurrentRank          string
	CurrentStreak        int
	LongestStreak        int
	UpdatedAt            time.Time
}

// UserStreak tracks daily exam activity.
type UserStreak struct {
	UserID    string
	Date      time.Time
	ExamCount int
	TotalScore int
	IsPerfect bool
}

// LeaderboardEntry is a materialized read model for rankings.
type LeaderboardEntry struct {
	UserID        string
	DisplayName   string
	TotalScore    int
	ExamCount     int
	AvgPercentage int
	Period        string // "alltime" | "weekly" | "monthly"
	Level         examd.JLPTLevel
	UpdatedAt     time.Time
}
