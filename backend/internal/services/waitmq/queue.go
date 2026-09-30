package waitmq

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

const QueueKey = "mq:waitlist:jobs"

type Job struct {
	TrainID     uint   `json:"train_id"`
	TravelDate  string `json:"travel_date"`
	FromStation string `json:"from_station"`
	ToStation   string `json:"to_station"`
	SeatType    string `json:"seat_type"`
}

type Queue struct {
	RDB *redis.Client
}

func New(rdb *redis.Client) *Queue {
	return &Queue{RDB: rdb}
}

func (q *Queue) Enqueue(ctx context.Context, job Job) error {
	if q == nil || q.RDB == nil {
		return nil
	}
	raw, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return q.RDB.LPush(ctx, QueueKey, raw).Err()
}

type Handler func(ctx context.Context, job Job) error

// StartWorker blocks popping jobs and calling handler.
func (q *Queue) StartWorker(ctx context.Context, handler Handler) {
	if q == nil || q.RDB == nil {
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			res, err := q.RDB.BRPop(ctx, 3*time.Second, QueueKey).Result()
			if err != nil {
				if err == redis.Nil || err == context.Canceled || err == context.DeadlineExceeded {
					continue
				}
				// redis nil timeout
				if err.Error() == "redis: nil" {
					continue
				}
				log.Printf("waitmq brpop: %v", err)
				time.Sleep(time.Second)
				continue
			}
			if len(res) < 2 {
				continue
			}
			var job Job
			if json.Unmarshal([]byte(res[1]), &job) != nil {
				continue
			}
			if err := handler(ctx, job); err != nil {
				log.Printf("waitmq handle: %v", err)
				// retry once later
				_ = q.Enqueue(context.Background(), job)
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()
}
