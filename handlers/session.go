package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/philiaspace/shikenphi/internal/application"
	"github.com/philiaspace/shikenphi/internal/domain"
	examd "github.com/philiaspace/phi-exam-domain/domain"
	"github.com/philiaspace/phi-core/transport"
)

// SessionHandler handles exam session routes.
type SessionHandler struct {
	repo           domain.SessionRepository
	resultRepo     domain.ResultRepository
	sessionBuilder *application.SessionBuilder
	hydrator       *application.SessionHydrator
	scorer         *application.SessionScorer
}

func NewSessionHandler(
	repo domain.SessionRepository,
	resultRepo domain.ResultRepository,
	mondaiURL string,
) *SessionHandler {
	return &SessionHandler{
		repo:           repo,
		resultRepo:     resultRepo,
		sessionBuilder: application.NewSessionBuilder(mondaiURL),
		hydrator:       application.NewSessionHydrator(mondaiURL),
		scorer:         application.NewSessionScorer(mondaiURL),
	}
}

func (h *SessionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /sessions", h.Create)
	mux.HandleFunc("GET /sessions/{id}", h.Get)
	mux.HandleFunc("POST /sessions/{id}/answers", h.SaveAnswer)
	mux.HandleFunc("POST /sessions/{id}/submit", h.Submit)
}

// Create creates a new exam session with real questions from MondaiPhi.
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

	ctx := r.Context()
	cmd := application.CreateSessionCommand{
		UserID:     req.UserID,
		Level:      level,
		TemplateID: req.TemplateID,
	}

	session, err := h.sessionBuilder.BuildSession(ctx, cmd)
	if err != nil {
		transport.InternalError(w, fmt.Sprintf("failed to build session: %v", err))
		return
	}

	if err := h.repo.Save(ctx, session); err != nil {
		transport.InternalError(w, "failed to save session")
		return
	}

	transport.Created(w, map[string]interface{}{
		"session_id":     session.ID,
		"level":          session.Level,
		"status":         session.Status,
		"expires_at":     session.ExpiresAt,
		"question_count": len(session.QuestionIDs),
	})
}

// Get loads a session by ID with hydrated questions.
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

	// Hydrate questions from MondaiPhi
	questions, err := h.hydrator.HydrateSession(r.Context(), session)
	if err != nil {
		// Log error but still return session
		questions = nil
	}

	response := map[string]interface{}{
		"session": map[string]interface{}{
			"id":             session.ID,
			"level":          session.Level,
			"status":         session.Status,
			"started_at":     session.StartedAt,
			"expires_at":     session.ExpiresAt,
			"completed_at":   session.CompletedAt,
			"score":          session.Score,
			"time_spent":     session.TimeSpentSeconds,
			"question_count": len(session.QuestionIDs),
			"answered_count": len(session.UserAnswers),
		},
	}

	if questions != nil {
		response["questions"] = questions
	}

	transport.OK(w, response)
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
		"message":         "answer saved",
		"question_index":  req.QuestionIndex,
		"selected_option": req.SelectedOption,
	})
}

// Submit finalizes and scores a session against correct answers from MondaiPhi.
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

	// Score against MondaiPhi correct answers
	scoreResult, err := h.scorer.Score(r.Context(), session)
	if err != nil {
		// Fallback to basic scoring if MondaiPhi is unavailable
		scoreResult = fallbackScore(session)
	}

	now := time.Now()
	timeSpent := int(time.Since(session.StartedAt).Seconds())

	// Complete the session
	if err := h.repo.Complete(r.Context(), examd.SessionID(id), scoreResult.CorrectCount, timeSpent, now, session.Version); err != nil {
		transport.InternalError(w, "failed to complete session")
		return
	}

	// Create result record
	result := &domain.Result{
		ID:               examd.ResultID("rst_" + generateShortID()),
		SessionID:        examd.SessionID(id),
		UserID:           session.UserID,
		Level:            session.Level,
		Score:            scoreResult.CorrectCount,
		TotalQuestions:   scoreResult.TotalQuestions,
		Percentage:       scoreResult.Percentage,
		SectionBreakdown: make(map[examd.Section]int),
		TimeSpentSeconds: timeSpent,
		CompletedAt:      now,
	}

	// Flatten section breakdown
	for section, stats := range scoreResult.SectionBreakdown {
		result.SectionBreakdown[section] = stats.Correct
	}

	if err := h.resultRepo.Save(r.Context(), result); err != nil {
		transport.InternalError(w, "failed to save result")
		return
	}

	transport.OK(w, map[string]interface{}{
		"score":              scoreResult.CorrectCount,
		"total":              scoreResult.TotalQuestions,
		"percentage":         scoreResult.Percentage,
		"time_spent":         timeSpent,
		"result_id":          result.ID,
		"section_breakdown":  scoreResult.SectionBreakdown,
		"question_results":   scoreResult.QuestionResults,
	})
}

func fallbackScore(session *domain.Session) *application.ScoreResult {
	total := len(session.QuestionIDs)
	if total == 0 {
		total = 75
	}
	correct := len(session.UserAnswers)
	return &application.ScoreResult{
		CorrectCount:   correct,
		TotalQuestions: total,
		Percentage:     int(examd.CalculatePercentage(correct, total)),
		SectionBreakdown: make(map[examd.Section]struct {
			Correct int
			Total   int
		}),
		QuestionResults: []application.QuestionResult{},
	}
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
