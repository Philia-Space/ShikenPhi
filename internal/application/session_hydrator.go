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
	Index          int                `json:"index"`
	ID             string             `json:"id"`
	Level          string             `json:"level"`
	Section        string             `json:"section"`
	Prompt         string             `json:"prompt"`
	Context        string             `json:"context,omitempty"`
	PassageID      string             `json:"passage_id,omitempty"`
	Passage        *HydratedPassage   `json:"passage,omitempty"`
	SourceGroupKey string             `json:"source_group_key,omitempty"`
	Options        []HydratedOption   `json:"options"`
	Assets         []HydratedAsset    `json:"assets,omitempty"`
	UserAnswer     string             `json:"user_answer,omitempty"`
}

// HydratedOption is a shuffled answer choice for the frontend.
type HydratedOption struct {
	ID    string `json:"id"`
	Value string `json:"value"`
	Label string `json:"label"`
}

// HydratedAsset is a question asset for the frontend.
type HydratedAsset struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// HydratedPassage is a passage for the frontend.
type HydratedPassage struct {
	Content string `json:"content"`
}

// HydrateSession fetches all questions for a session and applies option shuffling.
func (h *SessionHydrator) HydrateSession(ctx context.Context, session *domain.Session) ([]HydratedQuestion, error) {
	var hydrated []HydratedQuestion

	for i, qID := range session.QuestionIDs {
		question, options, assets, err := h.mondaiClient.GetQuestion(ctx, string(qID))
		if err != nil {
			return nil, fmt.Errorf("failed to fetch question %s: %w", qID, err)
		}

		order := session.OptionOrders[i]
		var shuffledOptions []HydratedOption

		if len(order) > 0 && len(order) <= len(options) {
			optionMap := make(map[string]mondaiphi.Option)
			for _, opt := range options {
				optionMap[opt.Value] = opt
			}
			for _, optValue := range order {
				if opt, ok := optionMap[fmt.Sprintf("%d", optValue)]; ok {
					shuffledOptions = append(shuffledOptions, HydratedOption{
						ID:    opt.ID,
						Value: opt.Value,
						Label: opt.Label,
					})
				}
			}
		}

		if len(shuffledOptions) == 0 {
			for _, opt := range options {
				shuffledOptions = append(shuffledOptions, HydratedOption{
					ID:    opt.ID,
					Value: opt.Value,
					Label: opt.Label,
				})
			}
		}

		var hydratedAssets []HydratedAsset
		for _, a := range assets {
			url := ""
			if a.ID != "" {
				resolved, err := h.mondaiClient.GetAssetURL(ctx, a.ID)
				if err == nil {
					url = resolved
				}
			}
			hydratedAssets = append(hydratedAssets, HydratedAsset{
				Type: a.Type,
				URL:  url,
			})
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
			Assets:         hydratedAssets,
		}

		if ans, ok := session.UserAnswers[i]; ok {
			hq.UserAnswer = ans
		}

		if question.PassageID != "" {
			passage, _, err := h.mondaiClient.GetPassage(ctx, question.PassageID)
			if err == nil {
				hq.Passage = &HydratedPassage{
					Content: passage.Content,
				}
			}
		}

		hydrated = append(hydrated, hq)
	}

	return hydrated, nil
}
