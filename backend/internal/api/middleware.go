package api

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	sessionCookie = "lp_session"
	voterCookie   = "lp_voter"
	ctxUserID     = "userID"
	ctxVoterKey   = "voterKey"
)

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		)
	}
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Content-Security-Policy",
			"default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; "+
				"connect-src 'self'; font-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		c.Next()
	}
}

func limitBody(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		c.Next()
	}
}

// optionalAuth reads the session cookie if present; it never rejects.
func (s *Server) optionalAuth(c *gin.Context) {
	if raw, err := c.Cookie(sessionCookie); err == nil && raw != "" {
		if sub, err := s.tokens.Parse(raw); err == nil {
			if id, err := bson.ObjectIDFromHex(sub); err == nil {
				c.Set(ctxUserID, id)
			}
		}
	}
	c.Next()
}

func (s *Server) requireAuth(c *gin.Context) {
	if _, ok := userID(c); !ok {
		fail(c, http.StatusUnauthorized, "unauthorized", "Please log in to continue")
		c.Abort()
		return
	}
	c.Next()
}

func userID(c *gin.Context) (bson.ObjectID, bool) {
	v, ok := c.Get(ctxUserID)
	if !ok {
		return bson.ObjectID{}, false
	}
	id, ok := v.(bson.ObjectID)
	return id, ok
}

// voterIdentity decides who a ballot belongs to: the account when logged in,
// otherwise a random, server-issued, HttpOnly browser cookie.
func (s *Server) voterIdentity(c *gin.Context) {
	if id, ok := userID(c); ok {
		c.Set(ctxVoterKey, "u:"+id.Hex())
		c.Next()
		return
	}
	token, err := c.Cookie(voterCookie)
	if err != nil || !isHexToken(token) {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		token = hex.EncodeToString(b)
		s.setCookie(c, voterCookie, token, 365*24*time.Hour)
	}
	c.Set(ctxVoterKey, "a:"+token)
	c.Next()
}

func isHexToken(s string) bool {
	if len(s) != 32 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// rateLimit throttles per client IP using a counter shared through Redis, so
// limits hold across all instances. If Redis is unreachable it fails open
// (the request still goes through) rather than taking the site down.
func (s *Server) rateLimit(bucket string, limit int64, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ok, retry, err := s.limiter.Allow(c.Request.Context(), bucket, c.ClientIP(), limit, window)
		if err != nil {
			slog.Warn("rate limiter unavailable", "err", err)
		} else if !ok {
			c.Header("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
			fail(c, http.StatusTooManyRequests, "rate_limited", "Too many requests, please slow down")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (s *Server) setCookie(c *gin.Context, name, value string, maxAge time.Duration) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode, // blocks cross-site POST/PATCH/DELETE (CSRF)
	})
}
