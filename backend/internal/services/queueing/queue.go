package queueing

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// BookQueue implements per-train booking queue + token-bucket rate limit.
type BookQueue struct {
	RDB        *redis.Client
	RatePerSec int           // max bookings processed per second per train+date
	WaitTimeout time.Duration
}

func New(rdb *redis.Client, ratePerSec int, waitTimeout time.Duration) *BookQueue {
	if ratePerSec <= 0 {
		ratePerSec = 5
	}
	if waitTimeout <= 0 {
		waitTimeout = 30 * time.Second
	}
	return &BookQueue{RDB: rdb, RatePerSec: ratePerSec, WaitTimeout: waitTimeout}
}

// WaitTurn enqueues the request and blocks until it is the head and a rate token is acquired.
func (q *BookQueue) WaitTurn(ctx context.Context, trainID uint, date, reqID string) (position int64, err error) {
	if q == nil || q.RDB == nil {
		return 0, nil
	}
	queueKey := fmt.Sprintf("bookq:%d:%s", trainID, date)
	rateKey := fmt.Sprintf("bookrate:%d:%s", trainID, date)

	if err := q.RDB.RPush(ctx, queueKey, reqID).Err(); err != nil {
		return 0, err
	}
	defer q.RDB.LRem(ctx, queueKey, 1, reqID)

	deadline := time.Now().Add(q.WaitTimeout)
	for {
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("排队超时，请重试")
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		list, err := q.RDB.LRange(ctx, queueKey, 0, 50).Result()
		if err != nil {
			return 0, err
		}
		pos := int64(-1)
		for i, v := range list {
			if v == reqID {
				pos = int64(i)
				break
			}
		}
		if pos < 0 {
			return 0, fmt.Errorf("排队异常，请重试")
		}
		if pos > 0 {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		// head of queue: try rate token (simple sliding window counter)
		n, err := q.RDB.Incr(ctx, rateKey).Result()
		if err != nil {
			return 0, err
		}
		if n == 1 {
			_ = q.RDB.Expire(ctx, rateKey, time.Second).Err()
		}
		if int(n) > q.RatePerSec {
			time.Sleep(150 * time.Millisecond)
			continue
		}
		return pos, nil
	}
}
