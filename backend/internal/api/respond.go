package api

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"livepoll/internal/models"
	"livepoll/internal/validate"
)

type errorBody struct {
	Error  string            `json:"error"`
	Code   string            `json:"code"`
	Fields map[string]string `json:"fields,omitempty"`
}

func fail(c *gin.Context, status int, code, msg string) {
	c.JSON(status, errorBody{Error: msg, Code: code})
}

func failValidation(c *gin.Context, errs validate.Errors) {
	c.JSON(http.StatusUnprocessableEntity, errorBody{Error: "Please fix the highlighted fields", Code: "validation", Fields: errs})
}

func failBadJSON(c *gin.Context, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		fail(c, http.StatusRequestEntityTooLarge, "too_large", "Request body is too large")
		return
	}
	fail(c, http.StatusBadRequest, "bad_request", "Malformed JSON body")
}

func failInternal(c *gin.Context, err error) {
	slog.Error("internal error", "path", c.Request.URL.Path, "err", err)
	fail(c, http.StatusInternalServerError, "internal", "Something went wrong, please try again")
}

type userJSON struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func toUserJSON(u *models.User) userJSON {
	return userJSON{ID: u.ID.Hex(), Name: u.Name, Email: u.Email}
}

type pollJSON struct {
	Slug        string          `json:"slug"`
	Question    string          `json:"question"`
	Description string          `json:"description"`
	Options     []models.Option `json:"options"`
	Status      string          `json:"status"`
	ClosesAt    *time.Time      `json:"closesAt"`
	CreatedAt   time.Time       `json:"createdAt"`
	OwnerName   string          `json:"ownerName"`
}

func toPollJSON(p *models.Poll) pollJSON {
	return pollJSON{
		Slug:        p.Slug,
		Question:    p.Question,
		Description: p.Description,
		Options:     p.Options,
		Status:      p.EffectiveStatus(time.Now()),
		ClosesAt:    p.ClosesAt,
		CreatedAt:   p.CreatedAt,
		OwnerName:   p.OwnerName,
	}
}
