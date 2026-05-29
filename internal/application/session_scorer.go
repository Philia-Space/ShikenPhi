package application

import (
	"context"
	"fmt"
	"sync"

	"github.com/philiaspace/shikenphi/internal/domain"
	"github.com/philiaspace/shikenphi/internal/mondaiphi"
	examd "github.com/philiaspace/phi-exam-domain/domain"
)

// SessionScorer calculates exam scores by comparing answers with MondaiPhi.
type SessionScorer struct {
	mondaiClient *mondaiphi.Client
}

// NewSessionScorer creates a scorer.
func NewSessionScorer(mondaiURL string, serviceSecret ...string) *SessionScorer {
	return &SessionScorer{
		mondaiClient: mondaiphi.NewClient(mondaiURL, serviceSecret...),
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
// Uses a worker pool (max 10 concurrent) to avoid overwhelming MondaiPhi.
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

	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error

	sem := make(chan struct{}, 10) // max 10 concurrent requests

	for i, qID := range session.QuestionIDs {
		wg.Add(1)
		go func(idx int, questionID string) {
			sem <- struct{}{}        // acquire slot
			defer func() { <-sem }() // release slot
			defer wg.Done()

			question, options, _, err := s.mondaiClient.GetQuestionForScoring(ctx, questionID)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to fetch question %s: %w", questionID, err)
				}
				mu.Unlock()
				return
			}

			// Find correct answer by matching option value to question's AnswerValue
			var correctOption string
			for _, opt := range options {
				if opt.Value == question.AnswerValue {
					correctOption = opt.Value
					break
				}
			}

			userAnswer := session.UserAnswers[idx]
			isCorrect := userAnswer != "" && userAnswer == correctOption

			section := examd.Section(question.Section)

			mu.Lock()
			if isCorrect {
				result.CorrectCount++
			}
			secStats := result.SectionBreakdown[section]
			secStats.Total++
			if isCorrect {
				secStats.Correct++
			}
			result.SectionBreakdown[section] = secStats
			result.QuestionResults = append(result.QuestionResults, QuestionResult{
				Index:         idx,
				QuestionID:    questionID,
				UserAnswer:    userAnswer,
				CorrectAnswer: correctOption,
				IsCorrect:     isCorrect,
				Section:       section,
			})
			mu.Unlock()
		}(i, string(qID))
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	result.Percentage = int(examd.CalculatePercentage(result.CorrectCount, result.TotalQuestions))
	return result, nil
}
