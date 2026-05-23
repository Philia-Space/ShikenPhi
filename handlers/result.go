package handlers

import (
	"net/http"

	"github.com/philiaspace/shikenphi/internal/domain"
	examd "github.com/philiaspace/phi-exam-domain/domain"
	"github.com/philiaspace/phi-core/transport"
)

// ResultHandler handles result and leaderboard routes.
type ResultHandler struct {
	resultRepo       domain.ResultRepository
	statsRepo        domain.UserStatsRepository
	leaderboardRepo  domain.LeaderboardRepository
}

func NewResultHandler(resultRepo domain.ResultRepository, statsRepo domain.UserStatsRepository, leaderboardRepo domain.LeaderboardRepository) *ResultHandler {
	return &ResultHandler{
		resultRepo:      resultRepo,
		statsRepo:       statsRepo,
		leaderboardRepo: leaderboardRepo,
	}
}

func (h *ResultHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /results", h.List)
	mux.HandleFunc("GET /results/{id}", h.Get)
	mux.HandleFunc("GET /leaderboard", h.Leaderboard)
	mux.HandleFunc("GET /profile/stats", h.Stats)
	mux.HandleFunc("GET /profile/streaks", h.Streaks)
}

// List returns the user's result history.
func (h *ResultHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		transport.BadRequest(w, "user_id is required")
		return
	}

	results, err := h.resultRepo.FindByUserID(r.Context(), userID, "", 50, 0)
	if err != nil {
		transport.InternalError(w, "failed to fetch results")
		return
	}

	transport.OK(w, map[string]interface{}{
		"results": results,
		"count":   len(results),
	})
}

// Get returns a single result by ID.
func (h *ResultHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		transport.BadRequest(w, "result id is required")
		return
	}

	result, err := h.resultRepo.FindBySessionID(r.Context(), examd.SessionID(id))
	if err != nil {
		transport.FromError(w, err)
		return
	}

	transport.OK(w, result)
}

// Leaderboard returns global rankings.
func (h *ResultHandler) Leaderboard(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "alltime"
	}

	levelParam := r.URL.Query().Get("level")
	level := examd.JLPTLevel(levelParam)

	entries, err := h.leaderboardRepo.Get(r.Context(), period, level, 100)
	if err != nil {
		transport.InternalError(w, "failed to fetch leaderboard")
		return
	}

	transport.OK(w, map[string]interface{}{
		"period":    period,
		"level":     levelParam,
		"entries":   entries,
		"count":     len(entries),
	})
}

// Stats returns user statistics.
func (h *ResultHandler) Stats(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		transport.BadRequest(w, "user_id is required")
		return
	}

	stats, err := h.statsRepo.FindByUserID(r.Context(), userID)
	if err != nil {
		// If not found, return empty stats
		stats = &domain.UserStats{
			UserID:        userID,
			TotalExams:    0,
			AvgScore:      0,
			BestScore:     0,
			CurrentStreak: 0,
			LongestStreak: 0,
		}
	}

	transport.OK(w, stats)
}

// Streaks returns daily streak data.
func (h *ResultHandler) Streaks(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		transport.BadRequest(w, "user_id is required")
		return
	}

	// TODO: Implement streak retrieval
	transport.OK(w, map[string]interface{}{
		"user_id": userID,
		"streaks": []interface{}{},
		"message": "streaks not yet implemented",
	})
}
