package config

import (
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	Port          string
	DatabaseURL   string
	JWTSecret     string
	RefreshTTLHrs int
}

func Load() Config {
	port := envOr("PORT", "8082")
	db := os.Getenv("DATABASE_URL")
	if db == "" {
		db = "postgres://commerce:commerce@localhost:5435/commerceflow?sslmode=disable"
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret-change-me"
		slog.Warn("JWT_SECRET not set, using dev default")
	}
	ttl := 168
	if v := os.Getenv("REFRESH_TTL_H"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ttl = n
		}
	}
	return Config{Port: port, DatabaseURL: db, JWTSecret: secret, RefreshTTLHrs: ttl}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
