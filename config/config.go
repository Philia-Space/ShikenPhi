package config

import "os"

// Config holds ShikenPhi environment configuration.
type Config struct {
	ServerPort                string
	Environment               string
	MongoURL                  string
	MongoDB                   string
	AuthJWKSURL               string
	MondaiPhiURL              string
	LeaderboardRefreshInterval string
}

// Load reads configuration from environment variables with sensible defaults for local dev.
func Load() *Config {
	return &Config{
		ServerPort:                 getEnv("SHIKENPHI_PORT", "8088"),
		Environment:                getEnv("SHIKENPHI_ENVIRONMENT", "development"),
		MongoURL:                   getEnv("SHIKENPHI_MONGO_URL", "mongodb://localhost:27018/shikenphi"),
		MongoDB:                    getEnv("SHIKENPHI_MONGO_DB", "shikenphi"),
		AuthJWKSURL:                getEnv("SHIKENPHI_AUTH_JWKS_URL", "http://localhost:8080/.well-known/jwks.json"),
		MondaiPhiURL:               getEnv("SHIKENPHI_MONDAIPHI_URL", "http://localhost:8087"),
		LeaderboardRefreshInterval: getEnv("SHIKENPHI_LEADERBOARD_REFRESH_INTERVAL", "5m"),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
