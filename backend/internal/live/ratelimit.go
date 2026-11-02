package live

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter is a fixed-window rate limiter shared by every API instance.
type Limiter struct{ rdb *redis.Client }

func NewLimiter(rdb *redis.Client) *Limiter { return &Limiter{rdb: rdb} }

// Allow counts one hit for key in the current window. retryAfter is how long
// until the window resets when the hit is rejected.
func (l *Limiter) Allow(ctx context.Context, bucket, key string, limit int64, window time.Duration) (ok bool, retryAfter time.Duration, err error) {
	now := time.Now()
	windowStart := now.Truncate(window)
	k := keyPrefix + "rl:" + bucket + ":" + key + ":" + windowStart.Format("20060102150405")

	var incr *redis.IntCmd
	_, err = l.rdb.Pipelined(ctx, func(p redis.Pipeliner) error {
		incr = p.Incr(ctx, k)
		p.Expire(ctx, k, window)
		return nil
	})
	if err != nil {
		return false, 0, err
	}
	if incr.Val() > limit {
		return false, windowStart.Add(window).Sub(now), nil
	}
	return true, 0, nil
}
