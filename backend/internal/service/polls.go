// Package service holds the poll business rules and coordinates MongoDB
// (durable truth) with Redis (live state + fan-out).
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"livepoll/internal/live"
	"livepoll/internal/models"
	"livepoll/internal/store"
	"livepoll/internal/validate"
)

var (
	ErrNotFound      = errors.New("poll not found")
	ErrForbidden     = errors.New("you do not own this poll")
	ErrClosed        = errors.New("this poll is closed")
	ErrInvalidOption = errors.New("that option does not belong to this poll")
	ErrAlreadyVoted  = errors.New("you have already voted on this poll")
)

// Event payloads sent to browsers over SSE.
type CountsEvent struct {
	Type string `json:"type"` // "counts"
	live.Snapshot
}

type StatusEvent struct {
	Type     string     `json:"type"` // "status"
	Status   string     `json:"status"`
	ClosesAt *time.Time `json:"closesAt"`
}

type ViewersEvent struct {
	Type  string `json:"type"` // "viewers"
	Count int64  `json:"count"`
}

type DeletedEvent struct {
	Type string `json:"type"` // "deleted"
}

type Polls struct {
	store   *store.Store
	counter *live.Counter
	hub     *live.Hub
}

func NewPolls(s *store.Store, c *live.Counter, h *live.Hub) *Polls {
	return &Polls{store: s, counter: c, hub: h}
}

func (s *Polls) Create(ctx context.Context, owner *models.User, in validate.Poll) (*models.Poll, error) {
	p := &models.Poll{
		OwnerID:     owner.ID,
		OwnerName:   owner.Name,
		Question:    in.Question,
		Description: in.Description,
		Status:      models.StatusOpen,
	}
	for _, text := range in.Options {
		p.Options = append(p.Options, models.Option{Text: text})
	}
	if in.DurationMinutes > 0 {
		t := time.Now().UTC().Add(time.Duration(in.DurationMinutes) * time.Minute)
		p.ClosesAt = &t
	}
	if err := s.store.CreatePoll(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Polls) BySlug(ctx context.Context, slug string) (*models.Poll, error) {
	p, err := s.store.PollBySlug(ctx, slug)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrNotFound
	}
	return p, err
}

func (s *Polls) Results(ctx context.Context, p *models.Poll) (live.Snapshot, error) {
	return s.counter.Read(ctx, p.ID.Hex(), p.OptionIDs())
}

func (s *Polls) VoteOf(ctx context.Context, p *models.Poll, voterKey string) (string, error) {
	return s.store.VoteOf(ctx, p.ID, voterKey)
}

// Vote records a ballot.
//
//  1. Redis Lua script atomically dedupes the voter and increments the count.
//     This is the fast path every concurrent request races through.
//  2. The ballot is written to MongoDB, whose unique index is the final word.
//  3. If MongoDB fails, the Redis increment is reverted so the two never drift.
//  4. Only a durable vote is broadcast to viewers.
func (s *Polls) Vote(ctx context.Context, p *models.Poll, voterKey, optionID string) (live.Snapshot, error) {
	if !p.AcceptsVotes(time.Now()) {
		return live.Snapshot{}, ErrClosed
	}
	if !p.HasOption(optionID) {
		return live.Snapshot{}, ErrInvalidOption
	}
	pollID, optionIDs := p.ID.Hex(), p.OptionIDs()

	snap, err := s.counter.RecordVote(ctx, pollID, optionIDs, voterKey, optionID)
	if errors.Is(err, live.ErrAlreadyVoted) {
		return live.Snapshot{}, ErrAlreadyVoted
	}
	if err != nil {
		return live.Snapshot{}, fmt.Errorf("record vote: %w", err)
	}

	// Finish the durable write even if the client disconnects mid-request.
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err = s.store.InsertVote(dbCtx, &models.Vote{PollID: p.ID, OptionID: optionID, VoterKey: voterKey})
	if err != nil {
		reverted, rerr := s.counter.Revert(dbCtx, pollID, optionIDs, voterKey, optionID)
		if rerr != nil {
			slog.Error("revert after failed insert", "poll", p.Slug, "err", rerr)
		} else {
			s.publish(dbCtx, pollID, CountsEvent{Type: "counts", Snapshot: reverted})
		}
		if errors.Is(err, store.ErrDuplicate) {
			return live.Snapshot{}, ErrAlreadyVoted
		}
		return live.Snapshot{}, fmt.Errorf("persist vote: %w", err)
	}

	s.publish(dbCtx, pollID, CountsEvent{Type: "counts", Snapshot: snap})
	return snap, nil
}

func (s *Polls) SetStatus(ctx context.Context, p *models.Poll, user bson.ObjectID, status string) (*models.Poll, error) {
	if !p.IsOwnedBy(user) {
		return nil, ErrForbidden
	}
	updated, err := s.store.SetPollStatus(ctx, p.ID, status)
	if err != nil {
		return nil, err
	}
	s.publish(ctx, p.ID.Hex(), StatusEvent{Type: "status", Status: updated.EffectiveStatus(time.Now()), ClosesAt: updated.ClosesAt})
	return updated, nil
}

func (s *Polls) Delete(ctx context.Context, p *models.Poll, user bson.ObjectID) error {
	if !p.IsOwnedBy(user) {
		return ErrForbidden
	}
	if err := s.store.DeletePoll(ctx, p.ID); err != nil {
		return err
	}
	s.publish(ctx, p.ID.Hex(), DeletedEvent{Type: "deleted"})
	if err := s.counter.Drop(ctx, p.ID.Hex()); err != nil {
		slog.Warn("drop live state", "poll", p.Slug, "err", err)
	}
	return nil
}

type Summary struct {
	Poll  models.Poll
	Total int64
}

func (s *Polls) ListMine(ctx context.Context, owner bson.ObjectID) ([]Summary, error) {
	polls, err := s.store.PollsByOwner(ctx, owner, 100)
	if err != nil {
		return nil, err
	}
	ids := make([]bson.ObjectID, len(polls))
	for i, p := range polls {
		ids[i] = p.ID
	}
	totals, err := s.store.TotalsByPoll(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Summary, len(polls))
	for i, p := range polls {
		out[i] = Summary{Poll: p, Total: totals[p.ID]}
	}
	return out, nil
}

func (s *Polls) publish(ctx context.Context, pollID string, ev any) {
	if err := s.hub.Publish(ctx, pollID, ev); err != nil {
		slog.Warn("publish event", "poll", pollID, "err", err)
	}
}
