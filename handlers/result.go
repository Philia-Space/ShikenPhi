package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"sync"

	"github.com/philiaspace/shikenphi/internal/application"
	"github.com/philiaspace/shikenphi/internal/domain"
	"github.com/philiaspace/shikenphi/internal/mondaiphi"
	examd "github.com/philiaspace/phi-exam-domain/domain"
	"github.com/philiaspace/phi-core/transport"
	middleware "github.com/philiaspace/phi-middleware"
)

type ResultHandler struct {
	resultRepo      domain.ResultRepository
	statsRepo       domain.UserStatsRepository
	leaderboardRepo domain.LeaderboardRepository
	achievementRepo domain.AchievementRepository
	hydrator        *application.SessionHydrator
	mondaiClient    *mondaiphi.Client
}

func NewResultHandler(resultRepo domain.ResultRepository, statsRepo domain.UserStatsRepository, leaderboardRepo domain.LeaderboardRepository, achievementRepo domain.AchievementRepository, mondaiURL string, mondaiSecret ...string) *ResultHandler {
	secret := ""
	if len(mondaiSecret) > 0 {
		secret = mondaiSecret[0]
	}
	return &ResultHandler{
		resultRepo:      resultRepo,
		statsRepo:       statsRepo,
		leaderboardRepo: leaderboardRepo,
		achievementRepo: achievementRepo,
		hydrator:        application.NewSessionHydrator(mondaiURL, secret),
		mondaiClient:    mondaiphi.NewClient(mondaiURL, secret),
	}
}

func getUserID(r *http.Request) (string, error) {
	claims, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		return "", fmt.Errorf("unauthorized")
	}
	return claims.UserID, nil
}

func (h *ResultHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /results", h.List)
	mux.HandleFunc("GET /results/{id}", h.Get)
	mux.HandleFunc("GET /results/{id}/review", h.Review)
	mux.HandleFunc("GET /leaderboard", h.Leaderboard)
	mux.HandleFunc("GET /profile/stats", h.Stats)
	mux.HandleFunc("GET /profile/streaks", h.Streaks)
	mux.HandleFunc("GET /profile/achievements", h.Achievements)
}

// List returns the user's result history.
func (h *ResultHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"success":false,"error":{"code":"UNAUTHORIZED","message":"authentication required"}}`))
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

// Get returns a single result by result ID or session ID.
func (h *ResultHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"success":false,"error":{"code":"UNAUTHORIZED","message":"authentication required"}}`))
		return
	}

	id := r.PathValue("id")
	if id == "" {
		transport.BadRequest(w, "result id is required")
		return
	}

	var result *domain.Result

	if len(id) > 4 && id[:4] == "rst_" {
		result, err = h.resultRepo.FindByID(r.Context(), id)
	} else {
		result, err = h.resultRepo.FindBySessionID(r.Context(), examd.SessionID(id))
	}

	if err != nil {
		transport.FromError(w, err)
		return
	}

	// Ownership check
	if result.UserID != userID {
		transport.Forbidden(w, "access denied")
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
	userID, err := getUserID(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"success":false,"error":{"code":"UNAUTHORIZED","message":"authentication required"}}`))
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
	userID, err := getUserID(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"success":false,"error":{"code":"UNAUTHORIZED","message":"authentication required"}}`))
		return
	}

	stats, err := h.statsRepo.FindByUserID(r.Context(), userID)
	if err != nil {
		transport.OK(w, map[string]interface{}{
			"user_id":         userID,
			"current_streak":  0,
			"longest_streak":  0,
		})
		return
	}

	transport.OK(w, map[string]interface{}{
		"user_id":         userID,
		"current_streak":  stats.CurrentStreak,
		"longest_streak":  stats.LongestStreak,
	})
}

func (h *ResultHandler) Review(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"success":false,"error":{"code":"UNAUTHORIZED","message":"authentication required"}}`))
		return
	}

	id := r.PathValue("id")
	if id == "" {
		transport.BadRequest(w, "result id is required")
		return
	}

	var result *domain.Result

	if len(id) > 4 && id[:4] == "rst_" {
		result, err = h.resultRepo.FindByID(r.Context(), id)
	} else {
		result, err = h.resultRepo.FindBySessionID(r.Context(), examd.SessionID(id))
	}

	if err != nil {
		transport.FromError(w, err)
		return
	}

	// Ownership check
	if result.UserID != userID {
		transport.Forbidden(w, "access denied")
		return
	}

	type optionItem struct {
		Value  string `json:"value"`
		Label  string `json:"label"`
	}

	type enrichedReview struct {
		QuestionID    string       `json:"question_id"`
		Section       string       `json:"section"`
		Prompt        string       `json:"prompt"`
		UserAnswer    string       `json:"user_answer"`
		CorrectAnswer string       `json:"correct_answer"`
		IsCorrect     bool         `json:"is_correct"`
		Options       []optionItem `json:"options"`
	}

	var mu sync.Mutex
	reviews := make([]enrichedReview, len(result.QuestionReviews))
	var wg sync.WaitGroup

	for i, qr := range result.QuestionReviews {
		reviews[i] = enrichedReview{
			QuestionID:    qr.QuestionID,
			Section:       qr.Section,
			UserAnswer:    qr.UserAnswer,
			CorrectAnswer: qr.CorrectAnswer,
			IsCorrect:     qr.IsCorrect,
		}

		wg.Add(1)
		go func(idx int, qid string) {
			defer wg.Done()
			question, options, _, fetchErr := h.mondaiClient.GetQuestion(r.Context(), qid)
			if fetchErr != nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			reviews[idx].Prompt = question.Prompt
			for _, opt := range options {
				reviews[idx].Options = append(reviews[idx].Options, optionItem{
					Value: opt.Value,
					Label: opt.Label,
				})
			}
		}(i, qr.QuestionID)
	}
	wg.Wait()

	sectionOrder := map[string]int{string(examd.Grammar): 0, string(examd.Vocabulary): 1, string(examd.Reading): 2, string(examd.Listening): 3}
	sort.SliceStable(reviews, func(i, j int) bool {
		return sectionOrder[reviews[i].Section] < sectionOrder[reviews[j].Section]
	})

	transport.OK(w, map[string]interface{}{
		"result_id": result.ID,
		"level":     result.Level,
		"reviews":   reviews,
		"count":     len(reviews),
	})
}

func (h *ResultHandler) Achievements(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"success":false,"error":{"code":"UNAUTHORIZED","message":"authentication required"}}`))
		return
	}

	achievements, err := h.achievementRepo.FindByUserID(r.Context(), userID)
	if err != nil {
		transport.OK(w, map[string]interface{}{
			"user_id":      userID,
			"achievements": []interface{}{},
		})
		return
	}

	transport.OK(w, map[string]interface{}{
		"user_id":      userID,
		"achievements": achievements,
		"count":        len(achievements),
	})
}
