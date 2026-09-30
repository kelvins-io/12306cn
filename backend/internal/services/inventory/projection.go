package inventory

import (
	"context"
	"time"

	"github.com/kelvins-io/12306cn/backend/internal/models"
	"github.com/kelvins-io/12306cn/backend/internal/services/events"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RebuildProjection recomputes OD×seat_type remaining for a train/date into remain_projections.
func (s *Service) RebuildProjection(ctx context.Context, trainID uint, date string) error {
	_ = s.EnsureDaily(trainID, date)

	var stops []models.TrainStop
	if err := s.DB.Where("train_id = ?", trainID).Order("seq").Find(&stops).Error; err != nil {
		return err
	}
	if len(stops) < 2 {
		return nil
	}
	var seatTypes []string
	s.DB.Model(&models.Seat{}).Where("train_id = ? AND blocked = ?", trainID, false).Distinct("seat_type").Pluck("seat_type", &seatTypes)
	if len(seatTypes) == 0 {
		s.DB.Model(&models.Seat{}).Where("train_id = ?", trainID).Distinct("seat_type").Pluck("seat_type", &seatTypes)
	}

	now := time.Now()
	rows := make([]models.RemainProjection, 0)
	for _, st := range seatTypes {
		for i := 0; i < len(stops); i++ {
			for j := i + 1; j < len(stops); j++ {
				n, err := s.RemainingByType(trainID, date, st, stops[i].Seq, stops[j].Seq)
				if err != nil {
					return err
				}
				rows = append(rows, models.RemainProjection{
					TrainID: trainID, TravelDate: date,
					FromSeq: stops[i].Seq, ToSeq: stops[j].Seq,
					SeatType: st, Remaining: n, UpdatedAt: now,
				})
			}
		}
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("train_id = ? AND travel_date = ?", trainID, date).Delete(&models.RemainProjection{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "train_id"}, {Name: "travel_date"}, {Name: "from_seq"}, {Name: "to_seq"}, {Name: "seat_type"}},
			DoUpdates: clause.AssignmentColumns([]string{"remaining", "updated_at"}),
		}).CreateInBatches(rows, 100).Error; err != nil {
			return err
		}
		s.Bus.Publish(ctx, events.Event{
			Kind: events.KindProjected, TrainID: trainID, TravelDate: date,
		})
		return nil
	})
}

func (s *Service) GetProjection(trainID uint, date, seatType string, fromSeq, toSeq int) (int, bool) {
	var p models.RemainProjection
	err := s.DB.Where("train_id = ? AND travel_date = ? AND from_seq = ? AND to_seq = ? AND seat_type = ?",
		trainID, date, fromSeq, toSeq, seatType).First(&p).Error
	if err != nil {
		return 0, false
	}
	return p.Remaining, true
}

func (s *Service) rebuildAsync(trainID uint, date string) {
	go func() {
		_ = s.RebuildProjection(context.Background(), trainID, date)
	}()
}
