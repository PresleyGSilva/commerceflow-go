package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PresleyGSilva/commerceflow-go/internal/config"
	"github.com/PresleyGSilva/commerceflow-go/internal/db"
	httpx "github.com/PresleyGSilva/commerceflow-go/internal/http"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		slog.Error("database connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	mctx, mcancel := context.WithTimeout(context.Background(), 60*time.Second)
	if err := db.Migrate(mctx, pool, "migrations"); err != nil {
		slog.Error("migrations failed", "err", err)
		os.Exit(1)
	}
	mcancel()

	router := httpx.NewRouter(httpx.Deps{DB: pool, JWTSecret: cfg.JWTSecret, RefreshTTLH: cfg.RefreshTTLHrs})
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: router}

	go func() {
		slog.Info("listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	slog.Info("shutting down")
	sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer scancel()
	_ = srv.Shutdown(sctx)
}
