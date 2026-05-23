package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/philiaspace/shikenphi/config"
	"github.com/philiaspace/shikenphi/handlers"
	"github.com/philiaspace/shikenphi/repositories/memory"
	"github.com/philiaspace/shikenphi/repositories/mongo"
	"github.com/philiaspace/phi-core/observability"
	"github.com/philiaspace/phi-middleware"
)

func main() {
	logger := observability.NewLogger(os.Getenv("LOG_LEVEL"))
	cfg := config.Load()

	ctx := context.Background()

	// Try to connect to MongoDB, but allow in-memory fallback
	mongoClient, mongoErr := mongo.Connect(ctx, cfg.MongoURL)
	if mongoErr != nil {
		logger.Info(ctx, "MongoDB not available, using in-memory repositories", "err", mongoErr)
	} else {
		logger.Info(ctx, "MongoDB connected")
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			mongoClient.Disconnect(ctx)
		}()
	}

	// Initialize in-memory repositories (replace with MongoDB later)
	sessionRepo := memory.NewSessionRepository()
	resultRepo := memory.NewInMemoryResultRepository()
	statsRepo := memory.NewInMemoryUserStatsRepository()
	leaderboardRepo := memory.NewInMemoryLeaderboardRepository()

	// Initialize handlers
	sessionHandler := handlers.NewSessionHandler(sessionRepo, resultRepo)
	resultHandler := handlers.NewResultHandler(resultRepo, statsRepo, leaderboardRepo)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Register routes
	sessionHandler.RegisterRoutes(mux)
	resultHandler.RegisterRoutes(mux)

	// Apply middleware chain
	handler := middleware.Chain(mux,
		middleware.Recovery(logger),
		middleware.Logger(logger),
		middleware.CORS(),
		middleware.RateLimit(100),
		middleware.AuthJWKS(middleware.JWKSAuthConfig{
			IssuerURL:      cfg.AuthJWKSURL,
			JWKSEndpoint:   "/.well-known/jwks.json",
			ExpectedIssuer: cfg.AuthJWKSURL,
			Audience:       "philia-space",
			CacheTTL:       5 * time.Minute,
			SkipPaths:      []string{"/health", "/.well-known", "/sessions", "/leaderboard", "/results", "/profile"},
		}),
	)

	addr := ":" + cfg.ServerPort
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logger.Info(ctx, "ShikenPhi starting", "addr", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error(ctx, "server error", "err", err)
		os.Exit(1)
	}
}
