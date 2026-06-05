package application

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
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
	ExamID     string // if set, use exam-based question fetching (no shuffle, no limit)
}

// BuildSession creates a session by fetching questions from MondaiPhi.
func (b *SessionBuilder) BuildSession(ctx context.Context, cmd CreateSessionCommand) (*domain.Session, error) {
	var questionIDs []examd.QuestionID
	optionOrders := make(map[int][]int)
	var level examd.JLPTLevel

	if cmd.ExamID != "" {
		// ─── EXAM-BASED: ALL questions from a specific exam, no shuffle, no limit ───
		questions, err := b.mondaiClient.ListExamQuestions(ctx, cmd.ExamID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch exam questions: %w", err)
		}
		if len(questions) == 0 {
			return nil, fmt.Errorf("no questions found for exam %s", cmd.ExamID)
		}
		level = examd.JLPTLevel(questions[0].Level)

		// Sort by section order: vocab → reading → listening, then by section_order
		sectionPriority := map[string]int{"vocabulary": 0, "reading": 1, "listening": 2}
		sort.SliceStable(questions, func(i, j int) bool {
			pi := sectionPriority[questions[i].Section]
			pj := sectionPriority[questions[j].Section]
			if pi != pj {
				return pi < pj
			}
			return questions[i].SectionOrder < questions[j].SectionOrder
		})

		questionIDs = make([]examd.QuestionID, len(questions))
		for i, q := range questions {
			questionIDs[i] = examd.QuestionID(q.ID)
			optionOrders[i] = []int{1, 2, 3, 4} // sequential, no shuffle
		}
	} else {
		// ─── LEVEL-BASED: fetch by level + section with default counts ───
		sectionCounts := map[examd.Section]int{
			examd.Vocabulary: 30,
			examd.Reading:    25,
			examd.Listening:  20,
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

		// Section order for deterministic display: vocab → reading → listening
		sectionOrder := []examd.Section{examd.Vocabulary, examd.Reading, examd.Listening}

		var allQuestions []mondaiphi.Question
		for _, section := range sectionOrder {
			count, ok := sectionCounts[section]
			if !ok {
				continue
			}
			questions, err := b.mondaiClient.ListQuestions(ctx, cmd.Level, section, count+20)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch questions for %s: %w", section, err)
			}
			allQuestions = append(allQuestions, questions...)
		}

		// Sort by section then by section_order for archive chronology
		sort.SliceStable(allQuestions, func(i, j int) bool {
			posI := -1
			posJ := -1
			for p, s := range sectionOrder {
				if string(s) == allQuestions[i].Section {
					posI = p
				}
				if string(s) == allQuestions[j].Section {
					posJ = p
				}
			}
			if posI != posJ {
				return posI < posJ
			}
			return allQuestions[i].SectionOrder < allQuestions[j].SectionOrder
		})

		units := buildAtomicUnits(allQuestions)
		selectedQuestions := selectQuestionsFromUnitsInOrder(units, sectionCounts, sectionOrder)

		if len(selectedQuestions) == 0 {
			return nil, fmt.Errorf("no questions available for level %s", cmd.Level)
		}

		level = cmd.Level
		questionIDs = make([]examd.QuestionID, len(selectedQuestions))
		seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
		for i, q := range selectedQuestions {
			questionIDs[i] = examd.QuestionID(q.ID)
			order := []int{1, 2, 3, 4}
			seededRand.Shuffle(len(order), func(i, j int) {
				order[i], order[j] = order[j], order[i]
			})
			optionOrders[i] = order
		}
	}

	if len(questionIDs) == 0 {
		return nil, fmt.Errorf("no questions available")
	}

	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	session := &domain.Session{
		ID:           examd.SessionID("ssn_" + id.GenerateULID()),
		UserID:       cmd.UserID,
		Level:        level,
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

func selectQuestionsFromUnitsInOrder(units []atomicUnit, sectionCounts map[examd.Section]int, sectionOrder []examd.Section) []mondaiphi.Question {
	// Track how many we've selected per section
	selectedPerSection := make(map[examd.Section]int)
	var selected []mondaiphi.Question

	// For each section in order, greedily take whole units that fit within target
	for _, section := range sectionOrder {
		target := sectionCounts[section]
		for _, unit := range units {
			if unit.section != section {
				continue
			}
			current := selectedPerSection[section]
			unitSize := len(unit.questions)
			if current+unitSize <= target {
				selected = append(selected, unit.questions...)
				selectedPerSection[section] = current + unitSize
			}
		}
	}

	// If any section is under target, fill with remaining standalone questions
	for _, section := range sectionOrder {
		target := sectionCounts[section]
		current := selectedPerSection[section]
		if current >= target {
			continue
		}
		for _, unit := range units {
			if unit.section != section || len(unit.questions) != 1 {
				continue
			}
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

	return selected
}
