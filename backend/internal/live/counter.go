// Package live contains everything Redis does: live vote counts, real-time
// fan-out over pub/sub, viewer presence and rate limiting.
package live

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Key layout (all per poll, all expire after `ttl` of inactivity):
//
//	lp:poll:{id}:ready   marker: Redis state is hydrated and authoritative
//	lp:poll:{id}:counts  HASH optionId -> votes
//	lp:poll:{id}:voters  SET  of voter keys (dedupe)
//	lp:poll:{id}:seq     monotonically increasing version of `counts`
//	lp:poll:{id}:lock    short lock while hydrating from MongoDB
const keyPrefix = "lp:"

func pollKey(pollID, suffix string) string { return keyPrefix + "poll:" + pollID + ":" + suffix }

var (
	ErrAlreadyVoted = errors.New("already voted")
	ErrNotReady     = errors.New("live state is being rebuilt, try again")
)

// Snapshot is a consistent view of a poll's counts. Seq lets clients discard
// out-of-order or already-applied updates.
type Snapshot struct {
	Seq    int64            `json:"seq"`
	Counts map[string]int64 `json:"counts"`
}

func (s Snapshot) Total() int64 {
	var t int64
	for _, n := range s.Counts {
		t += n
	}
	return t
}

// Loader rebuilds a poll's counts and voter set from the durable store.
type Loader func(ctx context.Context, pollID string) (counts map[string]int64, voters []string, err error)

type Counter struct {
	rdb    *redis.Client
	ttl    time.Duration
	loader Loader
}

func NewCounter(rdb *redis.Client, ttl time.Duration, loader Loader) *Counter {
	return &Counter{rdb: rdb, ttl: ttl, loader: loader}
}

// Every script returns {status, seq, field1, value1, ...}; status -1 = not hydrated.

// voteScript: dedupe + increment + bump version in one atomic step, so two
// concurrent ballots from the same voter can never both count.
var voteScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then return {-1} end
if redis.call('SADD', KEYS[2], ARGV[1]) == 0 then return {0} end
redis.call('HINCRBY', KEYS[3], ARGV[2], 1)
local seq = redis.call('INCR', KEYS[4])
for i = 1, 4 do redis.call('EXPIRE', KEYS[i], ARGV[3]) end
local out = {1, seq}
local flat = redis.call('HGETALL', KEYS[3])
for i = 1, #flat do out[#out + 1] = flat[i] end
return out
`)

// revertScript undoes a ballot whose durable write failed.
var revertScript = redis.NewScript(`
if redis.call('SREM', KEYS[2], ARGV[1]) == 1 then
  if redis.call('HINCRBY', KEYS[3], ARGV[2], -1) < 0 then redis.call('HSET', KEYS[3], ARGV[2], 0) end
end
local seq = redis.call('INCR', KEYS[4])
local out = {1, seq}
local flat = redis.call('HGETALL', KEYS[3])
for i = 1, #flat do out[#out + 1] = flat[i] end
return out
`)

// readScript reads counts and seq atomically (a pipelined HGETALL + GET could
// pair old counts with a newer seq) and refreshes the TTL of an active poll.
var readScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then return {-1} end
for i = 1, 4 do redis.call('EXPIRE', KEYS[i], ARGV[1]) end
local out = {1, tonumber(redis.call('GET', KEYS[4]) or '0')}
local flat = redis.call('HGETALL', KEYS[3])
for i = 1, #flat do out[#out + 1] = flat[i] end
return out
`)

var unlockScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]) end
return 0
`)

func (c *Counter) keys(pollID string) []string {
	return []string{
		pollKey(pollID, "ready"),
		pollKey(pollID, "voters"),
		pollKey(pollID, "counts"),
		pollKey(pollID, "seq"),
	}
}

func (c *Counter) ttlSeconds() int64 { return int64(c.ttl / time.Second) }

// Read returns the current snapshot, hydrating from MongoDB if needed.
func (c *Counter) Read(ctx context.Context, pollID string, optionIDs []string) (Snapshot, error) {
	return c.withHydration(ctx, pollID, optionIDs, func() (int64, Snapshot, error) {
		return c.run(ctx, readScript, pollID, optionIDs, c.ttlSeconds())
	})
}

// RecordVote counts a ballot. Returns ErrAlreadyVoted if voterKey already voted.
func (c *Counter) RecordVote(ctx context.Context, pollID string, optionIDs []string, voterKey, optionID string) (Snapshot, error) {
	return c.withHydration(ctx, pollID, optionIDs, func() (int64, Snapshot, error) {
		return c.run(ctx, voteScript, pollID, optionIDs, voterKey, optionID, c.ttlSeconds())
	})
}

// Revert removes a ballot previously added by RecordVote.
func (c *Counter) Revert(ctx context.Context, pollID string, optionIDs []string, voterKey, optionID string) (Snapshot, error) {
	_, snap, err := c.run(ctx, revertScript, pollID, optionIDs, voterKey, optionID)
	return snap, err
}

// Drop deletes all live state for a poll.
func (c *Counter) Drop(ctx context.Context, pollID string) error {
	return c.rdb.Del(ctx, append(c.keys(pollID), pollKey(pollID, "viewers"))...).Err()
}

// withHydration runs op; if Redis has no state for the poll it rebuilds it
// from MongoDB once and retries.
func (c *Counter) withHydration(ctx context.Context, pollID string, optionIDs []string, op func() (int64, Snapshot, error)) (Snapshot, error) {
	for attempt := 0; attempt < 2; attempt++ {
		status, snap, err := op()
		if err != nil {
			return Snapshot{}, err
		}
		switch status {
		case 1:
			return snap, nil
		case 0:
			return Snapshot{}, ErrAlreadyVoted
		}
		if err := c.hydrate(ctx, pollID); err != nil {
			return Snapshot{}, err
		}
	}
	return Snapshot{}, ErrNotReady
}

// hydrate copies a poll's vote state from MongoDB into Redis. A short lock
// makes sure only one request does the work; others wait for the ready flag.
// While the ready flag is absent the vote script refuses ballots, so no vote
// can slip in between the MongoDB read and the Redis write.
func (c *Counter) hydrate(ctx context.Context, pollID string) error {
	lockKey, readyKey := pollKey(pollID, "lock"), pollKey(pollID, "ready")
	token := randomToken()
	ok, err := c.rdb.SetNX(ctx, lockKey, token, 10*time.Second).Result()
	if err != nil {
		return err
	}
	if !ok {
		return c.waitReady(ctx, readyKey)
	}
	defer unlockScript.Run(context.WithoutCancel(ctx), c.rdb, []string{lockKey}, token)

	if n, err := c.rdb.Exists(ctx, readyKey).Result(); err != nil || n == 1 {
		return err // someone finished hydrating just before we got the lock
	}

	counts, voters, err := c.loader(ctx, pollID)
	if err != nil {
		return fmt.Errorf("load vote state: %w", err)
	}
	k := c.keys(pollID)
	_, err = c.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.Del(ctx, k[1], k[2])
		if len(counts) > 0 {
			p.HSet(ctx, k[2], toArgs(counts)...)
		}
		for i := 0; i < len(voters); i += 1000 {
			end := min(i+1000, len(voters))
			p.SAdd(ctx, k[1], toAny(voters[i:end])...)
		}
		// Seq starts from wall-clock millis so it is always ahead of any seq a
		// client saw before the state expired.
		p.SetNX(ctx, k[3], time.Now().UnixMilli(), 0)
		for _, key := range k[1:] {
			p.Expire(ctx, key, c.ttl)
		}
		p.Set(ctx, k[0], "1", c.ttl)
		return nil
	})
	return err
}

func (c *Counter) waitReady(ctx context.Context, readyKey string) error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if n, err := c.rdb.Exists(ctx, readyKey).Result(); err != nil || n == 1 {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return ErrNotReady
}

func (c *Counter) run(ctx context.Context, s *redis.Script, pollID string, optionIDs []string, args ...any) (int64, Snapshot, error) {
	raw, err := s.Run(ctx, c.rdb, c.keys(pollID), args...).Slice()
	if err != nil {
		return 0, Snapshot{}, err
	}
	return parseResult(raw, optionIDs)
}

func parseResult(raw []any, optionIDs []string) (int64, Snapshot, error) {
	if len(raw) == 0 {
		return 0, Snapshot{}, errors.New("empty script result")
	}
	status, _ := raw[0].(int64)
	if status != 1 {
		return status, Snapshot{}, nil
	}
	snap := Snapshot{Counts: make(map[string]int64, len(optionIDs))}
	snap.Seq, _ = raw[1].(int64)
	for _, id := range optionIDs {
		snap.Counts[id] = 0
	}
	for i := 2; i+1 < len(raw); i += 2 {
		field, _ := raw[i].(string)
		val, _ := raw[i+1].(string)
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return 0, Snapshot{}, fmt.Errorf("bad count for %q: %w", field, err)
		}
		if _, known := snap.Counts[field]; known {
			snap.Counts[field] = n
		}
	}
	return 1, snap, nil
}

func toArgs(m map[string]int64) []any {
	out := make([]any, 0, len(m)*2)
	for k, v := range m {
		out = append(out, k, v)
	}
	return out
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func randomToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
