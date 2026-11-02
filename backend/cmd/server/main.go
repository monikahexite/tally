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

	"github.com/redis/go-redis/v9"

	"livepoll/internal/api"
	"livepoll/internal/auth"
	"livepoll/internal/config"
	"livepoll/internal/live"
	"livepoll/internal/service"
	"livepoll/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.IsProduction() {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	db, err := store.Connect(startCtx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		return err
	}
	defer db.Close(context.Background())
	slog.Info("connected to MongoDB")

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return err
	}
	rdb := redis.NewClient(redisOpts)
	defer rdb.Close()
	if err := rdb.Ping(startCtx).Err(); err != nil {
		return err
	}
	slog.Info("connected to Redis")

	hub := live.NewHub(rdb)
	hubCtx, stopHub := context.WithCancel(context.Background())
	defer stopHub()
	go hub.Run(hubCtx)

	counter := live.NewCounter(rdb, 72*time.Hour, db.VoteState)
	shutdown := make(chan struct{})
	srv := api.NewServer(api.Deps{
		Config:   cfg,
		Store:    db,
		Redis:    rdb,
		Polls:    service.NewPolls(db, counter, hub),
		Tokens:   auth.NewTokens(cfg.JWTSecret, cfg.SessionTTL),
		Hub:      hub,
		Presence: live.NewPresence(rdb, 60*time.Second),
		Limiter:  live.NewLimiter(rdb),
		Shutdown: shutdown,
	})
	router, err := srv.Router()
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout: SSE responses are intentionally long-lived.
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "port", cfg.Port, "env", cfg.Env)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	close(shutdown) // end open SSE streams so Shutdown does not wait on them
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	return httpServer.Shutdown(shutdownCtx)
}
