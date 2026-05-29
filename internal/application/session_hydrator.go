package application

import (
	"context"
	"fmt"
	"sync"

	"github.com/philiaspace/shikenphi/internal/domain"
	"github.com/philiaspace/shikenphi/internal/mondaiphi"
)

// SessionHydrator fetches question data for an existing session.
type SessionHydrator struct {
	mondaiClient *mondaiphi.Client
}

// NewSessionHydrator creates a hydrator.
func NewSessionHydrator(mondaiURL string, serviceSecret ...string) *SessionHydrator {
	return &SessionHydrator{
		mondaiClient: mondaiphi.NewClient(mondaiURL, serviceSecret...),
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
// Uses a worker pool (max 10 concurrent) to avoid overwhelming MondaiPhi.
func (h *SessionHydrator) HydrateSession(ctx context.Context, session *domain.Session) ([]HydratedQuestion, error) {
	hydrated := make([]HydratedQuestion, len(session.QuestionIDs))
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

			question, options, assets, err := h.mondaiClient.GetQuestion(ctx, questionID)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to fetch question %s: %w", questionID, err)
				}
				mu.Unlock()
				return
			}

			// Shuffle options
			order := session.OptionOrders[idx]
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

			// Fetch assets concurrently
			var hydratedAssets []HydratedAsset
			if len(assets) > 0 {
				assetResults := make([]HydratedAsset, len(assets))
				var assetWg sync.WaitGroup
				for ai, a := range assets {
					assetWg.Add(1)
					go func(aIdx int, asset mondaiphi.Asset) {
						defer assetWg.Done()
						url := ""
						if asset.ID != "" {
							resolved, err := h.mondaiClient.GetAssetURL(ctx, asset.ID)
							if err == nil {
								url = resolved
							}
						}
						assetResults[aIdx] = HydratedAsset{Type: asset.Type, URL: url}
					}(ai, a)
				}
				assetWg.Wait()
				hydratedAssets = assetResults
			}

			hq := HydratedQuestion{
				Index:          idx,
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

			if ans, ok := session.UserAnswers[idx]; ok {
				hq.UserAnswer = ans
			}

			// Fetch passage if needed
			if question.PassageID != "" {
				passage, _, err := h.mondaiClient.GetPassage(ctx, question.PassageID)
				if err == nil {
					hq.Passage = &HydratedPassage{Content: passage.Content}
				}
			}

			mu.Lock()
			hydrated[idx] = hq
			mu.Unlock()
		}(i, string(qID))
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return hydrated, nil
}
