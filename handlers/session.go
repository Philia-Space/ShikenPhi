package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/philiaspace/shikenphi/internal/domain"
	examd "github.com/philiaspace/phi-exam-domain/domain"
	"github.com/philiaspace/phi-core/transport"
)

// SessionHandler handles exam session routes.
type SessionHandler struct {
	repo     domain.SessionRepository
	resultRepo domain.ResultRepository
}

func NewSessionHandler(repo domain.SessionRepository, resultRepo domain.ResultRepository) *SessionHandler {
	return &SessionHandler{repo: repo, resultRepo: resultRepo}
}

func (h *SessionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /sessions", h.Create)
	mux.HandleFunc("GET /sessions/{id}", h.Get)
	mux.HandleFunc("POST /sessions/{id}/answers", h.SaveAnswer)
	mux.HandleFunc("POST /sessions/{id}/submit", h.Submit)
}

// Create creates a new exam session.
func (h *SessionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Level      string `json:"level"`
		TemplateID string `json:"template_id"`
		UserID     string `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		transport.BadRequest(w, "invalid request body")
		return
	}

	if req.Level == "" {
		transport.BadRequest(w, "level is required")
		return
	}

	level := examd.JLPTLevel(req.Level)
	if !isValidLevel(level) {
		transport.BadRequest(w, "invalid level: must be N1, N2, N3, N4, or N5")
		return
	}

	// TODO: Call MondaiPhi to get questions
	// For now, create a placeholder session
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)
	
		session := &domain.Session{
		ID:           examd.SessionID("ssn_" + generateShortID()),
		UserID:       req.UserID,
		Level:        level,
		TemplateID:   req.TemplateID,
		QuestionIDs:  []examd.QuestionID{},
		OptionOrders: make(map[int][]int),
		UserAnswers:  make(map[int]string),
		Status:       domain.Active,
		StartedAt:    now,
		ExpiresAt:    expiresAt,
	}

	if err := h.repo.Save(r.Context(), session); err != nil {
		transport.InternalError(w, "failed to create session")
		return
	}

	transport.Created(w, map[string]interface{}{
		"session_id":  session.ID,
		"level":       session.Level,
		"status":      session.Status,
		"expires_at":  session.ExpiresAt,
		"question_count": len(session.QuestionIDs),
	})
}

// Get loads a session by ID.
func (h *SessionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		transport.BadRequest(w, "session id is required")
		return
	}

	session, err := h.repo.FindByID(r.Context(), id)
	if err != nil {
		transport.FromError(w, err)
		return
	}

	// Check if expired
	if session.Status == domain.Active && time.Now().After(session.ExpiresAt) {
		session.Status = domain.Expired
		h.repo.Save(r.Context(), session)
	}

	// TODO: Hydrate questions from MondaiPhi
	
	transport.OK(w, map[string]interface{}{
		"session": map[string]interface{}{
			"id":              session.ID,
			"level":           session.Level,
			"status":          session.Status,
			"started_at":      session.StartedAt,
			"expires_at":      session.ExpiresAt,
			"completed_at":    session.CompletedAt,
			"score":           session.Score,
			"time_spent":      session.TimeSpentSeconds,
			"question_count":  len(session.QuestionIDs),
			"answered_count":  len(session.UserAnswers),
		},
	})
}

// SaveAnswer saves a single answer to a session.
func (h *SessionHandler) SaveAnswer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		transport.BadRequest(w, "session id is required")
		return
	}

	var req struct {
		QuestionIndex  int    `json:"question_index"`
		SelectedOption string `json:"selected_option"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		transport.BadRequest(w, "invalid request body")
		return
	}

	session, err := h.repo.FindByID(r.Context(), id)
	if err != nil {
		transport.FromError(w, err)
		return
	}

	if session.Status != domain.Active {
		transport.BadRequest(w, "session is not active")
		return
	}

	if time.Now().After(session.ExpiresAt) {
		session.Status = domain.Expired
		h.repo.Save(r.Context(), session)
		transport.BadRequest(w, "session has expired")
		return
	}

	answers := map[int]string{
		req.QuestionIndex: req.SelectedOption,
	}

	if err := h.repo.UpdateAnswers(r.Context(), examd.SessionID(id), answers, session.Version); err != nil {
		transport.InternalError(w, "failed to save answer")
		return
	}

	transport.OK(w, map[string]interface{}{
		"message": "answer saved",
		"question_index": req.QuestionIndex,
		"selected_option": req.SelectedOption,
	})
}

// Submit finalizes and scores a session.
func (h *SessionHandler) Submit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		transport.BadRequest(w, "session id is required")
		return
	}

	session, err := h.repo.FindByID(r.Context(), id)
	if err != nil {
		transport.FromError(w, err)
		return
	}

	if session.Status != domain.Active {
		transport.BadRequest(w, "session is not active")
		return
	}

	// TODO: Fetch correct answers from MondaiPhi and calculate score
	// For now, use placeholder scoring
	correct := 0
	total := len(session.QuestionIDs)
	
	if total == 0 {
		total = 75 // placeholder
	}
	
	// Simple scoring: count answered questions as correct for demo
	correct = len(session.UserAnswers)
	percentage := examd.CalculatePercentage(correct, total)
	timeSpent := int(time.Since(session.StartedAt).Seconds())

	// Complete the session
	now := time.Now()
	if err := h.repo.Complete(r.Context(), examd.SessionID(id), correct, timeSpent, now, session.Version); err != nil {
		transport.InternalError(w, "failed to complete session")
		return
	}

	// Create result record
	result := &domain.Result{
		ID:               examd.ResultID("rst_" + generateShortID()),
		SessionID:        examd.SessionID(id),
		UserID:           session.UserID,
		Level:            session.Level,
		Score:            correct,
		TotalQuestions:   total,
		Percentage:       percentage,
		SectionBreakdown: map[examd.Section]int{},
		TimeSpentSeconds: timeSpent,
		CompletedAt:      now,
	}

	if err := h.resultRepo.Save(r.Context(), result); err != nil {
		transport.InternalError(w, "failed to save result")
		return
	}

	transport.OK(w, map[string]interface{}{
		"score":      correct,
		"total":      total,
		"percentage": percentage,
		"time_spent": timeSpent,
		"result_id":  result.ID,
	})
}

func isValidLevel(level examd.JLPTLevel) bool {
	for _, l := range examd.AllLevels() {
		if l == level {
			return true
		}
	}
	return false
}

func generateShortID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
