package query

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kelvins-io/12306cn/backend/internal/models"
	"github.com/kelvins-io/12306cn/backend/internal/services/events"
	"github.com/kelvins-io/12306cn/backend/internal/services/invclient"
	"github.com/kelvins-io/12306cn/backend/internal/services/pricing"
	"github.com/kelvins-io/12306cn/backend/internal/services/sale"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Service struct {
	DB  *gorm.DB
	RDB *redis.Client
	Inv *invclient.Client
	Bus *events.Bus
}

func New(db *gorm.DB, rdb *redis.Client, inv *invclient.Client) *Service {
	s := &Service{DB: db, RDB: rdb, Inv: inv, Bus: events.NewBus(rdb)}
	s.Bus.Subscribe(context.Background(), func(ev events.Event) {
		s.onInventoryEvent(ev)
	})
	return s
}

func (s *Service) onInventoryEvent(ev events.Event) {
	if s.RDB == nil {
		return
	}
	ctx := context.Background()
	// wipe all ticket caches for the travel date (pattern scan limited)
	iter := s.RDB.Scan(ctx, 0, fmt.Sprintf("tickets:*:*:%s", ev.TravelDate), 100).Iterator()
	for iter.Next(ctx) {
		_ = s.RDB.Del(ctx, iter.Val()).Err()
	}
	if ev.FromStation != "" && ev.ToStation != "" {
		_ = s.RDB.Del(ctx, fmt.Sprintf("tickets:%s:%s:%s", ev.FromStation, ev.ToStation, ev.TravelDate)).Err()
	}
	if ev.TravelDate == "" {
		iter2 := s.RDB.Scan(ctx, 0, "tickets:*", 200).Iterator()
		for iter2.Next(ctx) {
			_ = s.RDB.Del(ctx, iter2.Val()).Err()
		}
	}
}

func (s *Service) readProjection(trainID uint, date, seatType string, fromSeq, toSeq int) (int, bool) {
	var p models.RemainProjection
	err := s.DB.Where("train_id = ? AND travel_date = ? AND from_seq = ? AND to_seq = ? AND seat_type = ?",
		trainID, date, fromSeq, toSeq, seatType).First(&p).Error
	if err != nil {
		return 0, false
	}
	return p.Remaining, true
}

type TicketItem struct {
	TrainID       uint           `json:"train_id"`
	TrainNo       string         `json:"train_no"`
	TrainType     string         `json:"train_type"`
	FromStation   string         `json:"from_station"`
	ToStation     string         `json:"to_station"`
	DepartTime    string         `json:"depart_time"`
	ArriveTime    string         `json:"arrive_time"`
	DurationMin   int            `json:"duration_min"`
	FromSeq       int            `json:"from_seq"`
	ToSeq         int            `json:"to_seq"`
	SeatRemaining map[string]int `json:"seat_remaining"`
	SeatPrice     map[string]int `json:"seat_price"` // adult base
	SaleOpen      bool           `json:"sale_open"`
	SaleMessage   string         `json:"sale_message,omitempty"`
}

func (s *Service) ListStations(keyword string) ([]models.Station, error) {
	q := s.DB.Model(&models.Station{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("name ILIKE ? OR code ILIKE ? OR city ILIKE ?", like, like, like)
	}
	var list []models.Station
	err := q.Order("id").Limit(50).Find(&list).Error
	return list, err
}

func (s *Service) QueryTickets(ctx context.Context, from, to, date string) ([]TicketItem, error) {
	cacheKey := fmt.Sprintf("tickets:%s:%s:%s", from, to, date)
	if s.RDB != nil {
		if raw, err := s.RDB.Get(ctx, cacheKey).Bytes(); err == nil {
			var cached []TicketItem
			if json.Unmarshal(raw, &cached) == nil {
				return cached, nil
			}
		}
	}

	saleErr := sale.CheckOpen(s.DB, date, time.Now())
	saleOpen := saleErr == nil
	saleMsg := ""
	if saleErr != nil {
		saleMsg = saleErr.Error()
	}

	var fromStops []models.TrainStop
	if err := s.DB.Where("station_name = ?", from).Find(&fromStops).Error; err != nil {
		return nil, err
	}
	result := make([]TicketItem, 0)
	for _, fs := range fromStops {
		var ts models.TrainStop
		if err := s.DB.Where("train_id = ? AND station_name = ?", fs.TrainID, to).First(&ts).Error; err != nil {
			continue
		}
		if ts.Seq <= fs.Seq {
			continue
		}
		var train models.Train
		if err := s.DB.First(&train, fs.TrainID).Error; err != nil {
			continue
		}
		seatTypes := []string{}
		s.DB.Model(&models.Seat{}).Where("train_id = ?", train.ID).Distinct("seat_type").Pluck("seat_type", &seatTypes)

		remaining := map[string]int{}
		prices := map[string]int{}
		for _, st := range seatTypes {
			if !saleOpen {
				remaining[st] = 0
			} else if n, ok := s.readProjection(train.ID, date, st, fs.Seq, ts.Seq); ok {
				remaining[st] = n
			} else {
				n, err := s.Inv.Remaining(ctx, train.ID, date, st, fs.Seq, ts.Seq)
				if err != nil {
					continue
				}
				remaining[st] = n
			}
			prices[st] = pricing.BasePrice(st, train.TrainType, fs.Seq, ts.Seq)
		}

		result = append(result, TicketItem{
			TrainID: train.ID, TrainNo: train.TrainNo, TrainType: train.TrainType,
			FromStation: from, ToStation: to, DepartTime: fs.DepartTime, ArriveTime: ts.ArriveTime,
			DurationMin: DurationMinutes(fs.DepartTime, ts.ArriveTime),
			FromSeq: fs.Seq, ToSeq: ts.Seq,
			SeatRemaining: remaining, SeatPrice: prices,
			SaleOpen: saleOpen, SaleMessage: saleMsg,
		})
	}

	if s.RDB != nil && saleOpen {
		if b, err := json.Marshal(result); err == nil {
			_ = s.RDB.Set(ctx, cacheKey, b, 15*time.Second).Err()
		}
	}
	return result, nil
}

func DurationMinutes(depart, arrive string) int {
	parse := func(s string) int {
		var h, m int
		fmt.Sscanf(s, "%d:%d", &h, &m)
		return h*60 + m
	}
	d := parse(arrive) - parse(depart)
	if d < 0 {
		d += 24 * 60
	}
	return d
}
