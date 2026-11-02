package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"livepoll/internal/auth"
	"livepoll/internal/models"
	"livepoll/internal/store"
	"livepoll/internal/validate"
)

type signupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) signup(c *gin.Context) {
	var req signupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadJSON(c, err)
		return
	}
	in, errs := validate.SignupInput(req.Name, req.Email, req.Password)
	if errs.Any() {
		failValidation(c, errs)
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		failInternal(c, err)
		return
	}
	user := &models.User{Name: in.Name, Email: in.Email, PasswordHash: hash}
	if err := s.store.CreateUser(c.Request.Context(), user); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			failValidation(c, validate.Errors{"email": "An account with this email already exists"})
			return
		}
		failInternal(c, err)
		return
	}
	s.startSession(c, user, http.StatusCreated)
}

func (s *Server) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadJSON(c, err)
		return
	}
	email, errs := validate.LoginInput(req.Email, req.Password)
	if errs.Any() {
		failValidation(c, errs)
		return
	}
	user, err := s.store.UserByEmail(c.Request.Context(), email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		failInternal(c, err)
		return
	}
	hash := ""
	if err == nil {
		hash = user.PasswordHash
	}
	// Same message and same bcrypt cost for "no such user" and "wrong password".
	if !auth.CheckPassword(hash, req.Password) {
		fail(c, http.StatusUnauthorized, "invalid_credentials", "Incorrect email or password")
		return
	}
	s.startSession(c, user, http.StatusOK)
}

func (s *Server) logout(c *gin.Context) {
	s.setCookie(c, sessionCookie, "", -1)
	c.Status(http.StatusNoContent)
}

func (s *Server) me(c *gin.Context) {
	id, ok := userID(c)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"user": nil})
		return
	}
	user, err := s.store.UserByID(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		s.setCookie(c, sessionCookie, "", -1)
		c.JSON(http.StatusOK, gin.H{"user": nil})
		return
	}
	if err != nil {
		failInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": toUserJSON(user)})
}

func (s *Server) startSession(c *gin.Context, user *models.User, status int) {
	token, err := s.tokens.Issue(user.ID.Hex())
	if err != nil {
		failInternal(c, err)
		return
	}
	s.setCookie(c, sessionCookie, token, s.tokens.TTL())
	c.JSON(status, gin.H{"user": toUserJSON(user)})
}
