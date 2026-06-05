package application

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/philiaspace/shikenphi/internal/domain"
	"github.com/philiaspace/shikenphi/internal/mondaiphi"
	examd "github.com/philiaspace/phi-exam-domain/domain"
	"github.com/philiaspace/phi-utils/id"
)

// SessionBuilder orchestrates exam session creation.
//
// As of 2026-06-05 cleanup, only exam-based question selection is supported.
// Legacy "random 75" / level-based selection was removed; sessions are now
// always built from a specific archived JLPT exam (cmd.ExamID required).
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
//
// ExamID is required — sessions are always backed by a specific archived exam.
// Level and TemplateID from the legacy random-75 path have been removed.
type CreateSessionCommand struct {
	UserID string
	ExamID string
}

// BuildSession creates a session by fetching ALL questions for the given
// archived exam from MondaiPhi. Questions are sorted in original chronological
// order (vocabulary → reading → listening, then by section_order).
func (b *SessionBuilder) BuildSession(ctx context.Context, cmd CreateSessionCommand) (*domain.Session, error) {
	if cmd.ExamID == "" {
		return nil, fmt.Errorf("exam_id is required")
	}

	questions, err := b.mondaiClient.ListExamQuestions(ctx, cmd.ExamID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch exam questions: %w", err)
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("no questions found for exam %s", cmd.ExamID)
	}

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

	level := examd.JLPTLevel(questions[0].Level)
	questionIDs := make([]examd.QuestionID, len(questions))
	optionOrders := make(map[int][]int, len(questions))
	for i, q := range questions {
		questionIDs[i] = examd.QuestionID(q.ID)
		optionOrders[i] = []int{1, 2, 3, 4} // sequential, no shuffle
	}

	now := time.Now()
	session := &domain.Session{
		ID:           examd.SessionID("ssn_" + id.GenerateULID()),
		UserID:       cmd.UserID,
		Level:        level,
		QuestionIDs:  questionIDs,
		OptionOrders: optionOrders,
		UserAnswers:  make(map[int]string),
		Status:       domain.Active,
		StartedAt:    now,
		ExpiresAt:    now.Add(24 * time.Hour),
	}

	return session, nil
}
