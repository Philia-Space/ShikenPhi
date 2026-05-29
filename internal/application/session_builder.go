package application

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/philiaspace/shikenphi/internal/domain"
	"github.com/philiaspace/shikenphi/internal/mondaiphi"
	examd "github.com/philiaspace/phi-exam-domain/domain"
	"github.com/philiaspace/phi-utils/id"
)

// SessionBuilder orchestrates exam session creation.
type SessionBuilder struct {
	mondaiClient *mondaiphi.Client
}

// NewSessionBuilder creates a new session builder.
func NewSessionBuilder(mondaiURL string, serviceSecret ...string) *SessionBuilder {
	return &SessionBuilder{
		mondaiClient: mondaiphi.NewClient(mondaiURL, serviceSecret...),
	}
}

// CreateSessionCommand contains the data needed to start an exam.
type CreateSessionCommand struct {
	UserID     string
	Level      examd.JLPTLevel
	TemplateID string
}

// BuildSession creates a session by fetching questions from MondaiPhi.
func (b *SessionBuilder) BuildSession(ctx context.Context, cmd CreateSessionCommand) (*domain.Session, error) {
	// Fetch template to get section counts
	// For now, use a default template if not specified
	sectionCounts := map[examd.Section]int{
		examd.Grammar:   30,
		examd.Reading:   25,
		examd.Listening: 20,
	}

	if cmd.TemplateID != "" {
		templates, err := b.mondaiClient.ListTemplates(ctx, cmd.Level)
		if err == nil {
			for _, t := range templates {
				if t.ID == cmd.TemplateID {
					sectionCounts = make(map[examd.Section]int)
					for sectionStr, count := range t.SectionCounts {
						sectionCounts[examd.Section(sectionStr)] = count
					}
					break
				}
			}
		}
	}

	// Fetch questions for each section
	var allQuestions []mondaiphi.Question
	for section, count := range sectionCounts {
		questions, err := b.mondaiClient.ListQuestions(ctx, cmd.Level, section, count+20) // Fetch extra for selection
		if err != nil {
			return nil, fmt.Errorf("failed to fetch questions for %s: %w", section, err)
		}
		allQuestions = append(allQuestions, questions...)
	}

	// Build atomic units
	units := buildAtomicUnits(allQuestions)

	// Shuffle units
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	seededRand.Shuffle(len(units), func(i, j int) {
		units[i], units[j] = units[j], units[i]
	})

	// Select questions from units to match target counts
	selectedQuestions := selectQuestionsFromUnits(units, sectionCounts)

	if len(selectedQuestions) == 0 {
		return nil, fmt.Errorf("no questions available for level %s", cmd.Level)
	}

	// Flatten question IDs and generate option orders
	questionIDs := make([]examd.QuestionID, len(selectedQuestions))
	optionOrders := make(map[int][]int)

	for i, q := range selectedQuestions {
		questionIDs[i] = examd.QuestionID(q.ID)
		// Fisher-Yates shuffle for options [1,2,3,4]
		order := []int{1, 2, 3, 4}
		seededRand.Shuffle(len(order), func(i, j int) {
			order[i], order[j] = order[j], order[i]
		})
		optionOrders[i] = order
	}

	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	session := &domain.Session{
		ID:           examd.SessionID("ssn_" + id.GenerateULID()),
		UserID:       cmd.UserID,
		Level:        cmd.Level,
		TemplateID:   cmd.TemplateID,
		QuestionIDs:  questionIDs,
		OptionOrders: optionOrders,
		UserAnswers:  make(map[int]string),
		Status:       domain.Active,
		StartedAt:    now,
		ExpiresAt:    expiresAt,
	}

	return session, nil
}

// atomicUnit is a group of questions that must stay together (passage or source group).
type atomicUnit struct {
	questions []mondaiphi.Question
	section   examd.Section
}

func buildAtomicUnits(questions []mondaiphi.Question) []atomicUnit {
	// Group by passage_id or source_group_key
	passageGroups := make(map[string][]mondaiphi.Question)
	groupKeyGroups := make(map[string][]mondaiphi.Question)
	standalone := []mondaiphi.Question{}

	for _, q := range questions {
		if q.PassageID != "" {
			passageGroups[q.PassageID] = append(passageGroups[q.PassageID], q)
		} else if q.SourceGroupKey != "" {
			groupKeyGroups[q.SourceGroupKey] = append(groupKeyGroups[q.SourceGroupKey], q)
		} else {
			standalone = append(standalone, q)
		}
	}

	var units []atomicUnit

	// Add passage groups
	for _, qs := range passageGroups {
		if len(qs) > 0 {
			units = append(units, atomicUnit{
				questions: qs,
				section:   examd.Section(qs[0].Section),
			})
		}
	}

	// Add source_group_key groups
	for _, qs := range groupKeyGroups {
		if len(qs) > 0 {
			units = append(units, atomicUnit{
				questions: qs,
				section:   examd.Section(qs[0].Section),
			})
		}
	}

	// Add standalone questions
	for _, q := range standalone {
		units = append(units, atomicUnit{
			questions: []mondaiphi.Question{q},
			section:   examd.Section(q.Section),
		})
	}

	return units
}

func selectQuestionsFromUnits(units []atomicUnit, sectionCounts map[examd.Section]int) []mondaiphi.Question {
	// Track how many we've selected per section
	selectedPerSection := make(map[examd.Section]int)
	var selected []mondaiphi.Question

	// Greedy selection: add whole units while we haven't exceeded target
	for _, unit := range units {
		target := sectionCounts[unit.section]
		current := selectedPerSection[unit.section]
		unitSize := len(unit.questions)

		if current+unitSize <= target {
			selected = append(selected, unit.questions...)
			selectedPerSection[unit.section] = current + unitSize
		}
	}

	// If any section is under target, fill with remaining standalone questions
	for section, target := range sectionCounts {
		current := selectedPerSection[section]
		if current < target {
			// Find more questions of this section from standalone
			for _, unit := range units {
				if unit.section != section || len(unit.questions) != 1 {
					continue
				}
				// Check if already selected
				alreadySelected := false
				for _, sq := range selected {
					if sq.ID == unit.questions[0].ID {
						alreadySelected = true
						break
					}
				}
				if !alreadySelected && current < target {
					selected = append(selected, unit.questions[0])
					current++
				}
			}
		}
	}

	return selected
}
