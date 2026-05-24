package application

import (
	"context"
	"fmt"

	"github.com/philiaspace/shikenphi/internal/domain"
	"github.com/philiaspace/shikenphi/internal/mondaiphi"
	examd "github.com/philiaspace/phi-exam-domain/domain"
)

// SessionScorer calculates exam scores by comparing answers with MondaiPhi.
type SessionScorer struct {
	mondaiClient *mondaiphi.Client
}

// NewSessionScorer creates a scorer.
func NewSessionScorer(mondaiURL string) *SessionScorer {
	return &SessionScorer{
		mondaiClient: mondaiphi.NewClient(mondaiURL),
	}
}

// ScoreResult holds the scoring output.
type ScoreResult struct {
	CorrectCount     int
	TotalQuestions   int
	Percentage       int
	SectionBreakdown map[examd.Section]struct {
		Correct int
		Total   int
	}
	QuestionResults []QuestionResult
}

// QuestionResult holds per-question scoring.
type QuestionResult struct {
	Index        int
	QuestionID   string
	UserAnswer   string
	CorrectAnswer string
	IsCorrect    bool
	Section      examd.Section
}

// Score calculates the result for a completed session.
func (s *SessionScorer) Score(ctx context.Context, session *domain.Session) (*ScoreResult, error) {
	if len(session.QuestionIDs) == 0 {
		return nil, fmt.Errorf("session has no questions")
	}

	result := &ScoreResult{
		TotalQuestions: len(session.QuestionIDs),
		SectionBreakdown: make(map[examd.Section]struct {
			Correct int
			Total   int
		}),
		QuestionResults: make([]QuestionResult, 0, len(session.QuestionIDs)),
	}

	for i, qID := range session.QuestionIDs {
		question, options, _, err := s.mondaiClient.GetQuestionForScoring(ctx, string(qID))
		if err != nil {
			return nil, fmt.Errorf("failed to fetch question %s: %w", qID, err)
		}

		// Find correct answer by matching option value to question's AnswerValue
		var correctOption string
		for _, opt := range options {
			if opt.Value == question.AnswerValue {
				correctOption = opt.Value
				break
			}
		}

		userAnswer := session.UserAnswers[i]
		isCorrect := userAnswer != "" && userAnswer == correctOption

		if isCorrect {
			result.CorrectCount++
		}

		section := examd.Section(question.Section)
		secStats := result.SectionBreakdown[section]
		secStats.Total++
		if isCorrect {
			secStats.Correct++
		}
		result.SectionBreakdown[section] = secStats

		result.QuestionResults = append(result.QuestionResults, QuestionResult{
			Index:         i,
			QuestionID:    string(qID),
			UserAnswer:    userAnswer,
			CorrectAnswer: correctOption,
			IsCorrect:     isCorrect,
			Section:       section,
		})
	}

	result.Percentage = int(examd.CalculatePercentage(result.CorrectCount, result.TotalQuestions))
	return result, nil
}
