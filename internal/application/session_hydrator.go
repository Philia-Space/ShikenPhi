package application

import (
	"context"
	"fmt"

	"github.com/philiaspace/shikenphi/internal/domain"
	"github.com/philiaspace/shikenphi/internal/mondaiphi"
)

// SessionHydrator fetches question data for an existing session.
type SessionHydrator struct {
	mondaiClient *mondaiphi.Client
}

// NewSessionHydrator creates a hydrator.
func NewSessionHydrator(mondaiURL string) *SessionHydrator {
	return &SessionHydrator{
		mondaiClient: mondaiphi.NewClient(mondaiURL),
	}
}

// HydratedQuestion is a question ready for the frontend.
type HydratedQuestion struct {
	Index          int               `json:"index"`
	ID             string            `json:"id"`
	Level          string            `json:"level"`
	Section        string            `json:"section"`
	Prompt         string            `json:"prompt"`
	Context        string            `json:"context,omitempty"`
	PassageID      string            `json:"passage_id,omitempty"`
	Passage        *mondaiphi.Passage `json:"passage,omitempty"`
	SourceGroupKey string            `json:"source_group_key,omitempty"`
	Options        []string          `json:"options"` // Shuffled display order
	UserAnswer     string            `json:"user_answer,omitempty"`
}

// HydrateSession fetches all questions for a session and applies option shuffling.
func (h *SessionHydrator) HydrateSession(ctx context.Context, session *domain.Session) ([]HydratedQuestion, error) {
	var hydrated []HydratedQuestion

	for i, qID := range session.QuestionIDs {
		question, options, err := h.mondaiClient.GetQuestion(ctx, string(qID))
		if err != nil {
			return nil, fmt.Errorf("failed to fetch question %s: %w", qID, err)
		}

		// Apply option order from session
		shuffledOptions := make([]string, len(options))
		order := session.OptionOrders[i]
		if len(order) != len(options) {
			// Fallback: use original order
			for j, opt := range options {
				shuffledOptions[j] = opt.Value
			}
		} else {
			for j, optValue := range order {
				for _, opt := range options {
					if opt.Value == fmt.Sprintf("%d", optValue) {
						shuffledOptions[j] = opt.Value
						break
					}
				}
			}
		}

		hq := HydratedQuestion{
			Index:          i,
			ID:             question.ID,
			Level:          question.Level,
			Section:        question.Section,
			Prompt:         question.Prompt,
			Context:        question.Context,
			PassageID:      question.PassageID,
			SourceGroupKey: question.SourceGroupKey,
			Options:        shuffledOptions,
		}

		if ans, ok := session.UserAnswers[i]; ok {
			hq.UserAnswer = ans
		}

		// Fetch passage if present
		if question.PassageID != "" {
			passage, _, err := h.mondaiClient.GetPassage(ctx, question.PassageID)
			if err == nil {
				hq.Passage = passage
			}
		}

		hydrated = append(hydrated, hq)
	}

	return hydrated, nil
}
