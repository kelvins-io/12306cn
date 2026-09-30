package events

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

const Channel = "ticket:inventory_events"

type Kind string

const (
	KindOccupied  Kind = "occupied"
	KindReleased  Kind = "released"
	KindEnsured   Kind = "ensured"
	KindProjected Kind = "projected"
	KindSeatBlock Kind = "seat_block"
)

type Event struct {
	Kind       Kind   `json:"kind"`
	TrainID    uint   `json:"train_id"`
	TravelDate string `json:"travel_date"`
	FromStation string `json:"from_station,omitempty"`
	ToStation   string `json:"to_station,omitempty"`
	SeatType    string `json:"seat_type,omitempty"`
}

type Bus struct {
	RDB *redis.Client
}

func NewBus(rdb *redis.Client) *Bus {
	return &Bus{RDB: rdb}
}

func (b *Bus) Publish(ctx context.Context, ev Event) {
	if b == nil || b.RDB == nil {
		return
	}
	raw, err := json.Marshal(ev)
	if err != nil {
		return
	}
	if err := b.RDB.Publish(ctx, Channel, raw).Err(); err != nil {
		log.Printf("publish event: %v", err)
	}
}

func (b *Bus) Subscribe(ctx context.Context, handler func(Event)) {
	if b == nil || b.RDB == nil {
		return
	}
	sub := b.RDB.Subscribe(ctx, Channel)
	ch := sub.Channel()
	go func() {
		for {
			select {
			case <-ctx.Done():
				_ = sub.Close()
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var ev Event
				if json.Unmarshal([]byte(msg.Payload), &ev) == nil {
					handler(ev)
				}
			}
		}
	}()
}
