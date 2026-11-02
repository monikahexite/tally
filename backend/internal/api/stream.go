package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"livepoll/internal/live"
	"livepoll/internal/service"
)

const heartbeatEvery = 20 * time.Second

type snapshotEvent struct {
	Type     string     `json:"type"` // "snapshot"
	Status   string     `json:"status"`
	ClosesAt *time.Time `json:"closesAt"`
	Viewers  int64      `json:"viewers"`
	live.Snapshot
}

// stream is a Server-Sent Events endpoint. Votes travel over normal POSTs;
// this connection is one-way (server -> browser), which is exactly what SSE
// is for: plain HTTP, automatic reconnects, no extra protocol.
func (s *Server) stream(c *gin.Context) {
	poll, ok := s.loadPoll(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	pollID := poll.ID.Hex()

	// Subscribe BEFORE reading the snapshot. Any vote that lands in between
	// is then both in the snapshot and in the buffer; the browser drops the
	// duplicate because its seq is not newer. Reversing the order would lose it.
	sub := s.hub.Subscribe(pollID)
	defer s.hub.Unsubscribe(sub)

	snap, err := s.polls.Results(ctx, poll)
	if err != nil {
		failInternal(c, err)
		return
	}

	connID := randomConnID()
	viewers, _, err := s.presence.Touch(ctx, pollID, connID)
	if err != nil {
		slog.Warn("presence join", "err", err)
	}
	s.publishViewers(ctx, pollID, viewers)
	defer func() {
		bg, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if n, err := s.presence.Leave(bg, pollID, connID); err == nil {
			s.publishViewers(bg, pollID, n)
		}
	}()

	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // stop reverse proxies from buffering
	c.Status(http.StatusOK)

	// retry: tells EventSource how long to wait before reconnecting.
	fmt.Fprint(c.Writer, "retry: 2000\n\n")
	writeEvent(c, snapshotEvent{
		Type: "snapshot", Status: poll.EffectiveStatus(time.Now()), ClosesAt: poll.ClosesAt,
		Viewers: viewers, Snapshot: snap,
	})

	ticker := time.NewTicker(heartbeatEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		case <-sub.Done():
			return // too slow; client reconnects and resyncs from a snapshot
		case msg := <-sub.Messages():
			fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
			c.Writer.Flush()
		case <-ticker.C:
			// Comment line keeps idle proxies from closing the connection.
			fmt.Fprint(c.Writer, ": ping\n\n")
			c.Writer.Flush()
			if n, pruned, err := s.presence.Touch(ctx, pollID, connID); err == nil && pruned > 0 {
				s.publishViewers(ctx, pollID, n)
			}
		}
	}
}

func (s *Server) publishViewers(ctx context.Context, pollID string, n int64) {
	if err := s.hub.Publish(ctx, pollID, service.ViewersEvent{Type: "viewers", Count: n}); err != nil {
		slog.Warn("publish viewers", "err", err)
	}
}

func writeEvent(c *gin.Context, ev any) {
	b, err := json.Marshal(ev)
	if err != nil {
		slog.Error("marshal event", "err", err)
		return
	}
	fmt.Fprintf(c.Writer, "data: %s\n\n", b)
	c.Writer.Flush()
}

func randomConnID() string { return randomHex(12) }
