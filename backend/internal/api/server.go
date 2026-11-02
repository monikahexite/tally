// Package api is the HTTP layer: routing, middleware and thin handlers that
// translate between JSON and the service layer.
package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/redis/go-redis/v9"

	"livepoll/internal/auth"
	"livepoll/internal/config"
	"livepoll/internal/live"
	"livepoll/internal/service"
	"livepoll/internal/store"
)

type Server struct {
	cfg      *config.Config
	store    *store.Store
	rdb      *redis.Client
	polls    *service.Polls
	tokens   *auth.Tokens
	hub      *live.Hub
	presence *live.Presence
	limiter  *live.Limiter
	shutdown <-chan struct{} // closed when the process is stopping; ends SSE streams
}

type Deps struct {
	Config   *config.Config
	Store    *store.Store
	Redis    *redis.Client
	Polls    *service.Polls
	Tokens   *auth.Tokens
	Hub      *live.Hub
	Presence *live.Presence
	Limiter  *live.Limiter
	Shutdown <-chan struct{}
}

func NewServer(d Deps) *Server {
	return &Server{
		cfg: d.Config, store: d.Store, rdb: d.Redis, polls: d.Polls, tokens: d.Tokens,
		hub: d.Hub, presence: d.Presence, limiter: d.Limiter, shutdown: d.Shutdown,
	}
}

func (s *Server) Router() (*gin.Engine, error) {
	if s.cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	binding.EnableDecoderDisallowUnknownFields = true

	r := gin.New()
	if err := r.SetTrustedProxies(s.cfg.TrustedProxies); err != nil {
		return nil, err
	}
	r.Use(gin.Recovery(), requestLogger(), securityHeaders(), limitBody(32<<10))

	api := r.Group("/api")
	api.GET("/healthz", s.health)

	authGroup := api.Group("/auth")
	authGroup.POST("/signup", s.rateLimit("signup", 5, time.Hour), s.signup)
	authGroup.POST("/login", s.rateLimit("login", 10, 5*time.Minute), s.login)
	authGroup.POST("/logout", s.logout)
	authGroup.GET("/me", s.optionalAuth, s.me)

	polls := api.Group("/polls", s.optionalAuth)
	polls.POST("", s.requireAuth, s.rateLimit("create", 30, time.Hour), s.createPoll)
	polls.GET("/mine", s.requireAuth, s.myPolls)
	polls.GET("/:slug", s.voterIdentity, s.getPoll)
	polls.POST("/:slug/vote", s.voterIdentity, s.rateLimit("vote", 300, time.Minute), s.vote)
	polls.PATCH("/:slug", s.requireAuth, s.updatePoll)
	polls.DELETE("/:slug", s.requireAuth, s.deletePoll)
	polls.GET("/:slug/stream", s.rateLimit("stream", 600, time.Minute), s.stream)

	var spa gin.HandlerFunc
	if s.cfg.StaticDir != "" {
		spa = spaHandler(s.cfg.StaticDir)
		slog.Info("serving frontend", "dir", s.cfg.StaticDir)
	}
	r.NoRoute(func(c *gin.Context) {
		if spa == nil || strings.HasPrefix(c.Request.URL.Path, "/api/") {
			fail(c, http.StatusNotFound, "not_found", "No such endpoint")
			return
		}
		spa(c)
	})
	return r, nil
}

func (s *Server) health(c *gin.Context) {
	ctx := c.Request.Context()
	status, code := gin.H{"status": "ok", "liveConnections": s.hub.LocalSubscribers()}, http.StatusOK
	if err := s.store.Ping(ctx); err != nil {
		status["status"], status["mongo"], code = "degraded", "down", http.StatusServiceUnavailable
	}
	if err := s.rdb.Ping(ctx).Err(); err != nil {
		status["status"], status["redis"], code = "degraded", "down", http.StatusServiceUnavailable
	}
	c.JSON(code, status)
}
