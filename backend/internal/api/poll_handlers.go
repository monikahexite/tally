package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"livepoll/internal/models"
	"livepoll/internal/service"
	"livepoll/internal/validate"
)

type createPollRequest struct {
	Question        string   `json:"question"`
	Description     string   `json:"description"`
	Options         []string `json:"options"`
	DurationMinutes int      `json:"durationMinutes"`
}

type voteRequest struct {
	OptionID string `json:"optionId"`
}

type updatePollRequest struct {
	Status string `json:"status"`
}

func (s *Server) createPoll(c *gin.Context) {
	var req createPollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadJSON(c, err)
		return
	}
	in, errs := validate.PollInput(req.Question, req.Description, req.Options, req.DurationMinutes)
	if errs.Any() {
		failValidation(c, errs)
		return
	}
	uid, _ := userID(c)
	owner, err := s.store.UserByID(c.Request.Context(), uid)
	if err != nil {
		fail(c, http.StatusUnauthorized, "unauthorized", "Please log in again")
		return
	}
	poll, err := s.polls.Create(c.Request.Context(), owner, in)
	if err != nil {
		failInternal(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"poll": toPollJSON(poll)})
}

func (s *Server) myPolls(c *gin.Context) {
	uid, _ := userID(c)
	list, err := s.polls.ListMine(c.Request.Context(), uid)
	if err != nil {
		failInternal(c, err)
		return
	}
	type item struct {
		pollJSON
		TotalVotes int64 `json:"totalVotes"`
	}
	out := make([]item, len(list))
	for i := range list {
		out[i] = item{pollJSON: toPollJSON(&list[i].Poll), TotalVotes: list[i].Total}
	}
	c.JSON(http.StatusOK, gin.H{"polls": out})
}

func (s *Server) getPoll(c *gin.Context) {
	poll, ok := s.loadPoll(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	results, err := s.polls.Results(ctx, poll)
	if err != nil {
		failInternal(c, err)
		return
	}
	myVote, err := s.polls.VoteOf(ctx, poll, c.GetString(ctxVoterKey))
	if err != nil {
		failInternal(c, err)
		return
	}
	uid, _ := userID(c)
	c.JSON(http.StatusOK, gin.H{
		"poll":    toPollJSON(poll),
		"results": results,
		"myVote":  nullable(myVote),
		"isOwner": poll.IsOwnedBy(uid),
	})
}

func (s *Server) vote(c *gin.Context) {
	poll, ok := s.loadPoll(c)
	if !ok {
		return
	}
	var req voteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadJSON(c, err)
		return
	}
	if !validate.IsOptionID(req.OptionID) {
		failValidation(c, validate.Errors{"optionId": "Choose one of the options"})
		return
	}
	results, err := s.polls.Vote(c.Request.Context(), poll, c.GetString(ctxVoterKey), req.OptionID)
	switch {
	case errors.Is(err, service.ErrInvalidOption):
		failValidation(c, validate.Errors{"optionId": err.Error()})
	case errors.Is(err, service.ErrClosed):
		fail(c, http.StatusConflict, "poll_closed", "This poll is closed")
	case errors.Is(err, service.ErrAlreadyVoted):
		fail(c, http.StatusConflict, "already_voted", "You have already voted on this poll")
	case err != nil:
		failInternal(c, err)
	default:
		c.JSON(http.StatusCreated, gin.H{"results": results, "myVote": req.OptionID})
	}
}

func (s *Server) updatePoll(c *gin.Context) {
	poll, ok := s.loadPoll(c)
	if !ok {
		return
	}
	var req updatePollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		failBadJSON(c, err)
		return
	}
	status, valid := validate.Status(req.Status)
	if !valid {
		failValidation(c, validate.Errors{"status": `Status must be "open" or "closed"`})
		return
	}
	uid, _ := userID(c)
	updated, err := s.polls.SetStatus(c.Request.Context(), poll, uid, status)
	if s.handleOwnerErr(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"poll": toPollJSON(updated)})
}

func (s *Server) deletePoll(c *gin.Context) {
	poll, ok := s.loadPoll(c)
	if !ok {
		return
	}
	uid, _ := userID(c)
	if s.handleOwnerErr(c, s.polls.Delete(c.Request.Context(), poll, uid)) {
		return
	}
	c.Status(http.StatusNoContent)
}

// loadPoll validates the :slug param and fetches the poll, writing the error
// response itself when it cannot.
func (s *Server) loadPoll(c *gin.Context) (*models.Poll, bool) {
	slug := c.Param("slug")
	if !validate.IsSlug(slug) {
		fail(c, http.StatusNotFound, "not_found", "Poll not found")
		return nil, false
	}
	poll, err := s.polls.BySlug(c.Request.Context(), slug)
	if errors.Is(err, service.ErrNotFound) {
		fail(c, http.StatusNotFound, "not_found", "Poll not found")
		return nil, false
	}
	if err != nil {
		failInternal(c, err)
		return nil, false
	}
	return poll, true
}

func (s *Server) handleOwnerErr(c *gin.Context, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, service.ErrForbidden):
		fail(c, http.StatusForbidden, "forbidden", "Only the poll's creator can do that")
	default:
		failInternal(c, err)
	}
	return true
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
