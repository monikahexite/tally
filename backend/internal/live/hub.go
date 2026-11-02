package live

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const eventsPrefix = keyPrefix + "events:"

// Hub fans Redis pub/sub messages out to the SSE connections held by this
// process. Every API instance runs one Hub with a single PSUBSCRIBE, so a vote
// handled by instance A reaches viewers connected to instance B.
type Hub struct {
	rdb  *redis.Client
	mu   sync.RWMutex
	subs map[string]map[*Subscriber]struct{}
}

func NewHub(rdb *redis.Client) *Hub {
	return &Hub{rdb: rdb, subs: make(map[string]map[*Subscriber]struct{})}
}

// Subscriber is one connected browser tab.
type Subscriber struct {
	pollID string
	ch     chan []byte
	done   chan struct{}
	once   sync.Once
}

func (s *Subscriber) Messages() <-chan []byte { return s.ch }

// Done is closed when the hub drops the subscriber (it fell too far behind).
// The client's EventSource reconnects and receives a fresh snapshot.
func (s *Subscriber) Done() <-chan struct{} { return s.done }

func (s *Subscriber) close() { s.once.Do(func() { close(s.done) }) }

func (s *Subscriber) deliver(msg []byte) {
	select {
	case s.ch <- msg:
	default:
		s.close() // slow consumer: never block the hub for everyone else
	}
}

func (h *Hub) Subscribe(pollID string) *Subscriber {
	s := &Subscriber{pollID: pollID, ch: make(chan []byte, 64), done: make(chan struct{})}
	h.mu.Lock()
	if h.subs[pollID] == nil {
		h.subs[pollID] = make(map[*Subscriber]struct{})
	}
	h.subs[pollID][s] = struct{}{}
	h.mu.Unlock()
	return s
}

func (h *Hub) Unsubscribe(s *Subscriber) {
	h.mu.Lock()
	if set := h.subs[s.pollID]; set != nil {
		delete(set, s)
		if len(set) == 0 {
			delete(h.subs, s.pollID)
		}
	}
	h.mu.Unlock()
	s.close()
}

// Publish sends an event to every viewer of the poll on every instance.
func (h *Hub) Publish(ctx context.Context, pollID string, event any) error {
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return h.rdb.Publish(ctx, eventsPrefix+pollID, b).Err()
}

// Run listens to Redis until ctx is cancelled, reconnecting on failure.
func (h *Hub) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if err := h.listen(ctx); err != nil && ctx.Err() == nil {
			slog.Error("pubsub listener stopped, retrying", "err", err)
			select {
			case <-ctx.Done():
			case <-time.After(2 * time.Second):
			}
		}
	}
}

func (h *Hub) listen(ctx context.Context) error {
	ps := h.rdb.PSubscribe(ctx, eventsPrefix+"*")
	defer ps.Close()
	if _, err := ps.Receive(ctx); err != nil {
		return err
	}
	slog.Info("subscribed to poll events")
	ch := ps.Channel(redis.WithChannelSize(1024))
	for {
		select {
		case <-ctx.Done():
			return nil
		case m, ok := <-ch:
			if !ok {
				return nil
			}
			h.dispatch(strings.TrimPrefix(m.Channel, eventsPrefix), []byte(m.Payload))
		}
	}
}

func (h *Hub) dispatch(pollID string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs[pollID] {
		s.deliver(msg)
	}
}

// LocalSubscribers is exposed for health/metrics.
func (h *Hub) LocalSubscribers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for _, set := range h.subs {
		n += len(set)
	}
	return n
}
