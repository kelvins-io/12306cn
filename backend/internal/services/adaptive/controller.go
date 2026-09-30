package adaptive

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	latencyKey = "gw:backend_latency_ms"
	rateKey    = "gw:book_rate_per_sec"
)

// Controller adjusts booking rate based on observed backend latency.
type Controller struct {
	RDB      *redis.Client
	MinRate  int64
	MaxRate  int64
	current  atomic.Int64
}

func New(rdb *redis.Client, minRate, maxRate, initial int64) *Controller {
	if minRate <= 0 {
		minRate = 1
	}
	if maxRate < minRate {
		maxRate = minRate
	}
	if initial < minRate {
		initial = minRate
	}
	if initial > maxRate {
		initial = maxRate
	}
	c := &Controller{RDB: rdb, MinRate: minRate, MaxRate: maxRate}
	c.current.Store(initial)
	return c
}

func (c *Controller) ObserveLatency(ctx context.Context, ms int64) {
	if c == nil {
		return
	}
	cur := c.current.Load()
	switch {
	case ms > 800:
		cur = max64(c.MinRate, cur-2)
	case ms > 400:
		cur = max64(c.MinRate, cur-1)
	case ms < 100:
		cur = min64(c.MaxRate, cur+1)
	}
	c.current.Store(cur)
	if c.RDB != nil {
		_ = c.RDB.Set(ctx, latencyKey, ms, time.Minute).Err()
		_ = c.RDB.Set(ctx, rateKey, cur, time.Minute).Err()
	}
}

func (c *Controller) CurrentRate() int {
	if c == nil {
		return 5
	}
	return int(c.current.Load())
}

func (c *Controller) Snapshot(ctx context.Context) map[string]interface{} {
	lat := int64(0)
	if c.RDB != nil {
		if v, err := c.RDB.Get(ctx, latencyKey).Result(); err == nil {
			lat, _ = strconv.ParseInt(v, 10, 64)
		}
	}
	return map[string]interface{}{
		"book_rate_per_sec": c.CurrentRate(),
		"backend_latency_ms": lat,
		"min_rate": c.MinRate,
		"max_rate": c.MaxRate,
	}
}

func (c *Controller) SyncFromRedis(ctx context.Context) {
	if c == nil || c.RDB == nil {
		return
	}
	if v, err := c.RDB.Get(ctx, rateKey).Result(); err == nil {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= c.MinRate && n <= c.MaxRate {
			c.current.Store(n)
		}
	}
}

func RateLimitAllow(ctx context.Context, rdb *redis.Client, bucket string, perSec int) bool {
	if rdb == nil || perSec <= 0 {
		return true
	}
	key := fmt.Sprintf("gw:rl:%s:%d", bucket, time.Now().Unix())
	n, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return true
	}
	if n == 1 {
		_ = rdb.Expire(ctx, key, 2*time.Second).Err()
	}
	return int(n) <= perSec
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
