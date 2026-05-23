package handlers

import "net/http"

// ResultHandler handles result and leaderboard routes.
type ResultHandler struct{}

func NewResultHandler() *ResultHandler {
	return &ResultHandler{}
}

func (h *ResultHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /results", h.List)
	mux.HandleFunc("GET /results/{id}", h.Get)
	mux.HandleFunc("GET /leaderboard", h.Leaderboard)
	mux.HandleFunc("GET /profile/stats", h.Stats)
	mux.HandleFunc("GET /profile/streaks", h.Streaks)
}

func (h *ResultHandler) List(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

func (h *ResultHandler) Get(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

func (h *ResultHandler) Leaderboard(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

func (h *ResultHandler) Stats(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

func (h *ResultHandler) Streaks(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}
