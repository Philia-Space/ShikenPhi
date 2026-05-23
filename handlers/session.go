package handlers

import "net/http"

// SessionHandler handles exam session routes.
type SessionHandler struct{}

func NewSessionHandler() *SessionHandler {
	return &SessionHandler{}
}

func (h *SessionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /sessions", h.Create)
	mux.HandleFunc("GET /sessions/{id}", h.Get)
	mux.HandleFunc("POST /sessions/{id}/answers", h.SaveAnswer)
	mux.HandleFunc("POST /sessions/{id}/submit", h.Submit)
}

func (h *SessionHandler) Create(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

func (h *SessionHandler) Get(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

func (h *SessionHandler) SaveAnswer(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}

func (h *SessionHandler) Submit(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte(`{"error":"not implemented"}`))
}
