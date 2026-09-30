package services

import (
	"context"
	"errors"

	"github.com/kelvins-io/12306cn/backend/internal/models"
)

func (s *Service) ListQuotas() ([]models.SegmentQuota, error) {
	var list []models.SegmentQuota
	err := s.DB.Order("train_id, seat_type, seg_index").Find(&list).Error
	return list, err
}

func (s *Service) UpsertQuota(q *models.SegmentQuota) error {
	if q.TrainID == 0 || q.SeatType == "" || q.MaxSold < 0 {
		return errors.New("参数无效")
	}
	var exist models.SegmentQuota
	err := s.DB.Where("train_id = ? AND seat_type = ? AND seg_index = ?", q.TrainID, q.SeatType, q.SegIndex).First(&exist).Error
	if err != nil {
		return s.DB.Create(q).Error
	}
	if err := s.DB.Model(&exist).Updates(map[string]interface{}{"max_sold": q.MaxSold}).Error; err != nil {
		return err
	}
	exist.MaxSold = q.MaxSold
	*q = exist
	return nil
}

func (s *Service) DeleteQuota(id uint) error {
	return s.DB.Delete(&models.SegmentQuota{}, id).Error
}

func (s *Service) ListSalePolicies() ([]models.SalePolicy, error) {
	var list []models.SalePolicy
	err := s.DB.Order("id").Find(&list).Error
	return list, err
}

func (s *Service) UpsertSalePolicy(p *models.SalePolicy) error {
	if p.Name == "" || p.AdvanceDays < 0 {
		return errors.New("参数无效")
	}
	if p.ID == 0 {
		return s.DB.Create(p).Error
	}
	return s.DB.Model(p).Updates(map[string]interface{}{
		"name": p.Name, "advance_days": p.AdvanceDays,
		"open_hour": p.OpenHour, "open_minute": p.OpenMinute, "enabled": p.Enabled,
	}).Error
}

func (s *Service) ListRiskBlocks() ([]models.RiskBlock, error) {
	var list []models.RiskBlock
	err := s.DB.Order("id desc").Find(&list).Error
	return list, err
}

func (s *Service) UpsertRiskBlock(b *models.RiskBlock) error {
	if b.Kind == "" || b.Value == "" {
		return errors.New("参数无效")
	}
	var exist models.RiskBlock
	err := s.DB.Where("value = ?", b.Value).First(&exist).Error
	if err != nil {
		return s.DB.Create(b).Error
	}
	if err := s.DB.Model(&exist).Updates(map[string]interface{}{
		"kind": b.Kind, "reason": b.Reason, "expire_at": b.ExpireAt,
	}).Error; err != nil {
		return err
	}
	exist.Kind = b.Kind
	exist.Reason = b.Reason
	exist.ExpireAt = b.ExpireAt
	*b = exist
	return nil
}

func (s *Service) DeleteRiskBlock(id uint) error {
	return s.DB.Delete(&models.RiskBlock{}, id).Error
}

func (s *Service) ListTrains() ([]models.Train, error) {
	var list []models.Train
	err := s.DB.Order("id").Find(&list).Error
	return list, err
}

func (s *Service) ListSeats(trainID uint) ([]models.Seat, error) {
	if s.LocalInv != nil {
		return s.LocalInv.ListSeats(trainID)
	}
	var list []models.Seat
	q := s.DB.Model(&models.Seat{})
	if trainID > 0 {
		q = q.Where("train_id = ?", trainID)
	}
	err := q.Order("train_id, carriage_no, seat_no").Find(&list).Error
	return list, err
}

func (s *Service) SetSeatBlocked(seatID uint, blocked bool, reason string) (*models.Seat, error) {
	if s.LocalInv != nil {
		return s.LocalInv.SetSeatBlocked(seatID, blocked, reason)
	}
	return nil, errors.New("库存服务不可用")
}

func (s *Service) RebuildProjection(trainID uint, date string) error {
	if s.LocalInv == nil {
		return errors.New("库存服务不可用")
	}
	return s.LocalInv.RebuildProjection(context.Background(), trainID, date)
}

func (s *Service) ListDayBlocks(trainID uint, date string) ([]models.SeatDayBlock, error) {
	if s.LocalInv != nil {
		return s.LocalInv.ListDayBlocks(trainID, date)
	}
	return nil, errors.New("库存服务不可用")
}

func (s *Service) UpsertDayBlock(trainID, seatID uint, date, reason string) (*models.SeatDayBlock, error) {
	if s.LocalInv != nil {
		return s.LocalInv.UpsertDayBlock(trainID, seatID, date, reason)
	}
	return nil, errors.New("库存服务不可用")
}

func (s *Service) DeleteDayBlock(id uint) error {
	if s.LocalInv != nil {
		return s.LocalInv.DeleteDayBlock(id)
	}
	return errors.New("库存服务不可用")
}

func (s *Service) ListSaleWaves() ([]models.SaleWave, error) {
	var list []models.SaleWave
	err := s.DB.Order("sort_order, id").Find(&list).Error
	return list, err
}

func (s *Service) UpsertSaleWave(w *models.SaleWave) error {
	if w.Name == "" || w.ReleasePct <= 0 {
		return errors.New("参数无效")
	}
	if w.ReleasePct > 100 {
		w.ReleasePct = 100
	}
	if w.ID == 0 {
		return s.DB.Create(w).Error
	}
	return s.DB.Model(w).Updates(map[string]interface{}{
		"name": w.Name, "advance_days": w.AdvanceDays,
		"open_hour": w.OpenHour, "open_minute": w.OpenMinute,
		"seat_type": w.SeatType, "train_id": w.TrainID, "seg_index": w.SegIndex,
		"release_pct": w.ReleasePct, "enabled": w.Enabled, "sort_order": w.SortOrder,
	}).Error
}

func (s *Service) DeleteSaleWave(id uint) error {
	return s.DB.Delete(&models.SaleWave{}, id).Error
}
