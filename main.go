package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/philiaspace/shikenphi/config"
	"github.com/philiaspace/shikenphi/handlers"
	"github.com/philiaspace/shikenphi/internal/domain"
	memory "github.com/philiaspace/shikenphi/repositories/memory"
	mongoRepos "github.com/philiaspace/shikenphi/repositories/mongo"
	"github.com/philiaspace/phi-core/observability"
	"github.com/philiaspace/phi-middleware"
	"go.mongodb.org/mongo-driver/mongo"
)

func main() {
	logger := observability.NewLogger(os.Getenv("LOG_LEVEL"))
	cfg := config.Load()

	// Use signal-aware context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Try to connect to MongoDB, but allow in-memory fallback
	var mongoClient *mongo.Client
	mongoClient, mongoErr := mongoRepos.Connect(ctx, cfg.MongoURL)
	if mongoErr != nil {
		logger.Info(ctx, "MongoDB not available, using in-memory repositories", "err", mongoErr)
		mongoClient = nil
	} else {
		logger.Info(ctx, "MongoDB connected")
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			mongoClient.Disconnect(ctx)
		}()
	}

	var sessionRepo domain.SessionRepository
	var resultRepo domain.ResultRepository
	var statsRepo domain.UserStatsRepository
	var leaderboardRepo domain.LeaderboardRepository
	var achievementRepo domain.AchievementRepository

	if mongoClient != nil {
		logger.Info(ctx, "Using MongoDB repositories")
		sessionRepo = mongoRepos.NewSessionRepository(mongoClient, cfg.MongoDB)
		resultRepo = mongoRepos.NewResultRepository(mongoClient, cfg.MongoDB)
		statsRepo = mongoRepos.NewUserStatsRepository(mongoClient, cfg.MongoDB)
		leaderboardRepo = mongoRepos.NewLeaderboardRepository(mongoClient, cfg.MongoDB)
		achievementRepo = mongoRepos.NewAchievementRepository(mongoClient, cfg.MongoDB)
	} else {
		logger.Info(ctx, "Using in-memory repositories")
		sessionRepo = memory.NewSessionRepository()
		resultRepo = memory.NewInMemoryResultRepository()
		statsRepo = memory.NewInMemoryUserStatsRepository()
		leaderboardRepo = memory.NewInMemoryLeaderboardRepository()
		achievementRepo = memory.NewAchievementRepository()
	}

	sessionHandler := handlers.NewSessionHandler(sessionRepo, resultRepo, statsRepo, leaderboardRepo, achievementRepo, cfg.MondaiPhiURL, cfg.MondaiPhiServiceSecret)
	resultHandler := handlers.NewResultHandler(resultRepo, statsRepo, leaderboardRepo, achievementRepo, cfg.MondaiPhiURL, cfg.MondaiPhiServiceSecret)

	go startLeaderboardRefresh(ctx, logger, leaderboardRepo, cfg.LeaderboardRefreshInterval)

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
			SkipPaths:      []string{"/health"},
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
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info(ctx, "shutting down ShikenPhi")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error(ctx, "shutdown error", "err", err)
	}
}

func startLeaderboardRefresh(ctx context.Context, logger observability.Logger, repo domain.LeaderboardRepository, intervalStr string) {
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		interval = 5 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	periods := []string{"alltime", "weekly", "monthly"}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, period := range periods {
				if err := repo.Refresh(ctx, period); err != nil {
					logger.Error(ctx, "leaderboard refresh failed", "period", period, "err", err)
				}
			}
		}
	}
}
