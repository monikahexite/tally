package live

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Presence tracks who is watching a poll right now, across all instances.
// Each connection is a member of a sorted set scored by its last heartbeat,
// so connections lost in a crash age out instead of being counted forever.
type Presence struct {
	rdb   *redis.Client
	stale time.Duration
}

func NewPresence(rdb *redis.Client, stale time.Duration) *Presence {
	return &Presence{rdb: rdb, stale: stale}
}

// Touch adds or refreshes a connection and returns the live viewer count and
// how many stale connections were pruned.
func (p *Presence) Touch(ctx context.Context, pollID, connID string) (count, pruned int64, err error) {
	key := pollKey(pollID, "viewers")
	now := time.Now()
	var rem *redis.IntCmd
	var card *redis.IntCmd
	_, err = p.rdb.TxPipelined(ctx, func(pp redis.Pipeliner) error {
		pp.ZAdd(ctx, key, redis.Z{Score: float64(now.UnixMilli()), Member: connID})
		rem = pp.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(now.Add(-p.stale).UnixMilli(), 10))
		card = pp.ZCard(ctx, key)
		pp.Expire(ctx, key, p.stale*2)
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return card.Val(), rem.Val(), nil
}

func (p *Presence) Leave(ctx context.Context, pollID, connID string) (int64, error) {
	key := pollKey(pollID, "viewers")
	var card *redis.IntCmd
	_, err := p.rdb.TxPipelined(ctx, func(pp redis.Pipeliner) error {
		pp.ZRem(ctx, key, connID)
		card = pp.ZCard(ctx, key)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return card.Val(), nil
}
