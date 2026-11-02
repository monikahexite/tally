package live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return mr, rdb
}

var opts = []string{"a", "b", "c"}

func emptyLoader(context.Context, string) (map[string]int64, []string, error) {
	return map[string]int64{}, nil, nil
}

func TestVoteDedupeUnderConcurrency(t *testing.T) {
	_, rdb := newRedis(t)
	c := NewCounter(rdb, time.Hour, emptyLoader)
	ctx := context.Background()

	var wg sync.WaitGroup
	var accepted, rejected atomic.Int64
	// 50 distinct voters, each firing the same ballot 4 times concurrently.
	for v := 0; v < 50; v++ {
		for r := 0; r < 4; r++ {
			wg.Add(1)
			go func(v int) {
				defer wg.Done()
				_, err := c.RecordVote(ctx, "p1", opts, fmt.Sprintf("voter-%d", v), opts[v%3])
				switch {
				case err == nil:
					accepted.Add(1)
				case errors.Is(err, ErrAlreadyVoted):
					rejected.Add(1)
				default:
					t.Error(err)
				}
			}(v)
		}
	}
	wg.Wait()

	if accepted.Load() != 50 || rejected.Load() != 150 {
		t.Fatalf("accepted=%d rejected=%d, want 50/150", accepted.Load(), rejected.Load())
	}
	snap, err := c.Read(ctx, "p1", opts)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Total() != 50 || snap.Counts["a"] != 17 || snap.Counts["b"] != 17 || snap.Counts["c"] != 16 {
		t.Fatalf("unexpected counts %+v", snap.Counts)
	}
}

func TestSeqIsMonotonic(t *testing.T) {
	_, rdb := newRedis(t)
	c := NewCounter(rdb, time.Hour, emptyLoader)
	ctx := context.Background()
	s1, _ := c.RecordVote(ctx, "p1", opts, "v1", "a")
	s2, _ := c.RecordVote(ctx, "p1", opts, "v2", "b")
	s3, _ := c.Revert(ctx, "p1", opts, "v2", "b")
	if !(s1.Seq < s2.Seq && s2.Seq < s3.Seq) {
		t.Fatalf("seq not increasing: %d %d %d", s1.Seq, s2.Seq, s3.Seq)
	}
	if s3.Counts["b"] != 0 || s3.Counts["a"] != 1 {
		t.Fatalf("revert did not undo vote: %+v", s3.Counts)
	}
	// After a revert the same voter may vote again.
	if _, err := c.RecordVote(ctx, "p1", opts, "v2", "c"); err != nil {
		t.Fatalf("revote after revert failed: %v", err)
	}
}

func TestHydratesFromLoaderOnceAndKeepsDedupe(t *testing.T) {
	mr, rdb := newRedis(t)
	var loads atomic.Int64
	loader := func(context.Context, string) (map[string]int64, []string, error) {
		loads.Add(1)
		time.Sleep(20 * time.Millisecond) // widen the race window
		return map[string]int64{"a": 2, "b": 1}, []string{"x", "y", "z"}, nil
	}
	c := NewCounter(rdb, time.Hour, loader)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Read(ctx, "p1", opts); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if loads.Load() != 1 {
		t.Fatalf("loader ran %d times, want 1", loads.Load())
	}

	if _, err := c.RecordVote(ctx, "p1", opts, "x", "c"); !errors.Is(err, ErrAlreadyVoted) {
		t.Fatalf("voter from MongoDB was allowed to vote again: %v", err)
	}
	snap, _ := c.RecordVote(ctx, "p1", opts, "new", "c")
	if snap.Counts["a"] != 2 || snap.Counts["b"] != 1 || snap.Counts["c"] != 1 {
		t.Fatalf("unexpected counts after hydrate: %+v", snap.Counts)
	}

	// Simulate the poll's Redis state expiring: it must come back from MongoDB.
	mr.FastForward(2 * time.Hour)
	if _, err := c.Read(ctx, "p1", opts); err != nil {
		t.Fatal(err)
	}
	if loads.Load() != 2 {
		t.Fatalf("expected re-hydration after expiry, loads=%d", loads.Load())
	}
}

func TestUnknownOptionsAreIgnoredInSnapshot(t *testing.T) {
	_, rdb := newRedis(t)
	loader := func(context.Context, string) (map[string]int64, []string, error) {
		return map[string]int64{"a": 1, "gone": 5}, []string{"v"}, nil
	}
	c := NewCounter(rdb, time.Hour, loader)
	snap, err := c.Read(context.Background(), "p1", opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snap.Counts["gone"]; ok || snap.Counts["a"] != 1 || len(snap.Counts) != 3 {
		t.Fatalf("unexpected snapshot %+v", snap.Counts)
	}
}

func TestHubFanOut(t *testing.T) {
	_, rdb := newRedis(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Two hubs simulate two API instances sharing one Redis.
	h1, h2 := NewHub(rdb), NewHub(rdb)
	go h1.Run(ctx)
	go h2.Run(ctx)

	s1 := h1.Subscribe("p1")
	s2 := h2.Subscribe("p1")
	other := h2.Subscribe("p2")
	time.Sleep(100 * time.Millisecond) // let PSUBSCRIBE register

	if err := h1.Publish(ctx, "p1", map[string]any{"type": "counts", "seq": 7}); err != nil {
		t.Fatal(err)
	}
	for i, s := range []*Subscriber{s1, s2} {
		select {
		case msg := <-s.Messages():
			var ev map[string]any
			_ = json.Unmarshal(msg, &ev)
			if ev["seq"] != float64(7) {
				t.Fatalf("subscriber %d got %s", i, msg)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("subscriber %d received nothing", i)
		}
	}
	select {
	case msg := <-other.Messages():
		t.Fatalf("event leaked to another poll: %s", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSlowSubscriberIsDropped(t *testing.T) {
	_, rdb := newRedis(t)
	h := NewHub(rdb)
	s := h.Subscribe("p1")
	for i := 0; i < 100; i++ {
		h.dispatch("p1", []byte("x"))
	}
	select {
	case <-s.Done():
	default:
		t.Fatal("slow subscriber was not dropped")
	}
}

func TestPresenceAndRateLimit(t *testing.T) {
	_, rdb := newRedis(t)
	ctx := context.Background()
	p := NewPresence(rdb, time.Minute)
	p.Touch(ctx, "p1", "c1")
	n, _, _ := p.Touch(ctx, "p1", "c2")
	if n != 2 {
		t.Fatalf("viewers = %d, want 2", n)
	}
	if n, _ = p.Leave(ctx, "p1", "c1"); n != 1 {
		t.Fatalf("viewers after leave = %d, want 1", n)
	}

	l := NewLimiter(rdb)
	for i := 0; i < 3; i++ {
		if ok, _, _ := l.Allow(ctx, "vote", "1.2.3.4", 3, time.Minute); !ok {
			t.Fatalf("hit %d rejected", i)
		}
	}
	if ok, retry, _ := l.Allow(ctx, "vote", "1.2.3.4", 3, time.Minute); ok || retry <= 0 {
		t.Fatal("4th hit should be rejected with retry-after")
	}
	if ok, _, _ := l.Allow(ctx, "vote", "5.6.7.8", 3, time.Minute); !ok {
		t.Fatal("other IP should not be limited")
	}
}
