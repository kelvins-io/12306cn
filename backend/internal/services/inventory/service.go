package inventory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/kelvins-io/12306cn/backend/internal/models"
	"github.com/kelvins-io/12306cn/backend/internal/services/events"
	"github.com/kelvins-io/12306cn/backend/internal/services/sale"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	DB  *gorm.DB
	RDB *redis.Client
	Bus *events.Bus
}

func NewService(db *gorm.DB, rdb *redis.Client) *Service {
	return &Service{DB: db, RDB: rdb, Bus: events.NewBus(rdb)}
}

type SeatPreference struct {
	Position   string `json:"position"`    // 靠窗/过道/中间/空=不限
	CarriageNo string `json:"carriage_no"` // 指定车厢，空=不限
}

type OccupyRequest struct {
	TrainID     uint           `json:"train_id"`
	TravelDate  string         `json:"travel_date"`
	SeatType    string         `json:"seat_type"`
	FromSeq     int            `json:"from_seq"`
	ToSeq       int            `json:"to_seq"`
	Count       int            `json:"count"`
	Preference  SeatPreference `json:"preference"`
	FromStation string         `json:"from_station"`
	ToStation   string         `json:"to_station"`
}

type SeatPickDTO struct {
	SeatID     uint   `json:"seat_id"`
	CarriageNo string `json:"carriage_no"`
	SeatNo     string `json:"seat_no"`
	SeatType   string `json:"seat_type"`
	Position   string `json:"position"`
}

func (s *Service) EnsureDaily(trainID uint, date string) error {
	var count int64
	s.DB.Model(&models.SeatInventory{}).Where("train_id = ? AND travel_date = ?", trainID, date).Count(&count)
	if count > 0 {
		return nil
	}
	var seats []models.Seat
	if err := s.DB.Where("train_id = ?", trainID).Find(&seats).Error; err != nil {
		return err
	}
	rows := make([]models.SeatInventory, 0, len(seats))
	for _, seat := range seats {
		rows = append(rows, models.SeatInventory{
			TrainID:    trainID,
			TravelDate: date,
			SeatID:     seat.ID,
			SeatType:   seat.SeatType,
			Occupancy:  0,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	if err := s.DB.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(rows, 100).Error; err != nil {
		return err
	}
	s.Bus.Publish(context.Background(), events.Event{
		Kind: events.KindEnsured, TrainID: trainID, TravelDate: date,
	})
	return nil
}

func (s *Service) RemainingByType(trainID uint, date, seatType string, fromSeq, toSeq int) (int, error) {
	need := SegmentMask(fromSeq, toSeq)
	blockedIDs := s.blockedSeatIDs(trainID, date, seatType)

	q := s.DB.Model(&models.SeatInventory{}).
		Where("train_id = ? AND travel_date = ? AND seat_type = ?", trainID, date, seatType)
	if len(blockedIDs) > 0 {
		q = q.Where("seat_id NOT IN ?", blockedIDs)
	}
	var occ []uint64
	if err := q.Pluck("occupancy", &occ).Error; err != nil {
		return 0, err
	}
	if len(occ) == 0 {
		if err := s.EnsureDaily(trainID, date); err != nil {
			return 0, err
		}
		q2 := s.DB.Model(&models.SeatInventory{}).
			Where("train_id = ? AND travel_date = ? AND seat_type = ?", trainID, date, seatType)
		if len(blockedIDs) > 0 {
			q2 = q2.Where("seat_id NOT IN ?", blockedIDs)
		}
		_ = q2.Pluck("occupancy", &occ).Error
	}
	avail := RemainingCount(occ, need)
	if qlim, err := s.quotaLimitedRemaining(trainID, seatType, fromSeq, toSeq, occ); err == nil && qlim < avail {
		avail = qlim
	}
	// 多波次放票：按累计放票比例限制可售量
	total := len(occ)
	sold := total - RemainingCount(occ, need)
	if pct, err := sale.CumulativeReleasePct(s.DB, trainID, seatType, date, fromSeq, toSeq, time.Now()); err == nil {
		avail = sale.ApplyReleaseCap(avail, total, sold, pct)
	}
	return avail, nil
}

func (s *Service) blockedSeatIDs(trainID uint, date, seatType string) []uint {
	var permanent []uint
	q := s.DB.Model(&models.Seat{}).Where("train_id = ? AND blocked = ?", trainID, true)
	if seatType != "" {
		q = q.Where("seat_type = ?", seatType)
	}
	_ = q.Pluck("id", &permanent).Error

	var dayIDs []uint
	dq := s.DB.Model(&models.SeatDayBlock{}).Where("train_id = ? AND travel_date = ?", trainID, date)
	_ = dq.Pluck("seat_id", &dayIDs).Error
	if seatType != "" && len(dayIDs) > 0 {
		var filtered []uint
		_ = s.DB.Model(&models.Seat{}).Where("id IN ? AND seat_type = ?", dayIDs, seatType).Pluck("id", &filtered).Error
		dayIDs = filtered
	}

	seen := map[uint]bool{}
	out := make([]uint, 0, len(permanent)+len(dayIDs))
	for _, id := range permanent {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, id := range dayIDs {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func (s *Service) quotaLimitedRemaining(trainID uint, seatType string, fromSeq, toSeq int, occ []uint64) (int, error) {
	var quotas []models.SegmentQuota
	if err := s.DB.Where("train_id = ? AND seat_type = ? AND seg_index >= ? AND seg_index < ?",
		trainID, seatType, fromSeq, toSeq).Find(&quotas).Error; err != nil {
		return 0, err
	}
	if len(quotas) == 0 {
		return len(occ), nil
	}
	minLeft := len(occ)
	for _, q := range quotas {
		sold := 0
		bit := uint64(1) << uint(q.SegIndex)
		for _, o := range occ {
			if o&bit != 0 {
				sold++
			}
		}
		left := q.MaxSold - sold
		if left < 0 {
			left = 0
		}
		if left < minLeft {
			minLeft = left
		}
	}
	return minLeft, nil
}

func (s *Service) CheckQuota(tx *gorm.DB, trainID uint, seatType string, fromSeq, toSeq, count int, occ []models.SeatInventory) error {
	q := s.DB
	if tx != nil {
		q = tx
	}
	var quotas []models.SegmentQuota
	if err := q.Where("train_id = ? AND seat_type = ? AND seg_index >= ? AND seg_index < ?",
		trainID, seatType, fromSeq, toSeq).Find(&quotas).Error; err != nil {
		return err
	}
	for _, quota := range quotas {
		sold := 0
		bit := uint64(1) << uint(quota.SegIndex)
		for _, inv := range occ {
			if inv.Occupancy&bit != 0 {
				sold++
			}
		}
		if sold+count > quota.MaxSold {
			return fmt.Errorf("区段配额不足（区间段 %d 上限 %d，已售 %d）", quota.SegIndex, quota.MaxSold, sold)
		}
	}
	return nil
}

type SeatPick struct {
	Inv  models.SeatInventory
	Seat models.Seat
}

func (s *Service) WithTrainLock(ctx context.Context, trainID uint, date string, fn func() error) error {
	if s.RDB == nil {
		return fn()
	}
	lockKey := fmt.Sprintf("lock:train:%d:%s", trainID, date)
	ok, err := s.RDB.SetNX(ctx, lockKey, "1", 12*time.Second).Result()
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("当前车次繁忙，请稍后重试")
	}
	defer s.RDB.Del(ctx, lockKey)
	return fn()
}

// OccupyLocked occupies seats; caller may already hold lock. Prefer matching seats first.
func (s *Service) OccupyLocked(tx *gorm.DB, req OccupyRequest) ([]SeatPick, error) {
	need := SegmentMask(req.FromSeq, req.ToSeq)
	var invs []models.SeatInventory
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("train_id = ? AND travel_date = ? AND seat_type = ?", req.TrainID, req.TravelDate, req.SeatType).
		Order("seat_id").Find(&invs).Error; err != nil {
		return nil, err
	}
	if err := s.CheckQuota(tx, req.TrainID, req.SeatType, req.FromSeq, req.ToSeq, req.Count, invs); err != nil {
		return nil, err
	}
	// 多波次放票容量检查
	needMask := need
	totalSeats := 0
	sold := 0
	dayBlocked := map[uint]bool{}
	for _, id := range s.blockedSeatIDs(req.TrainID, req.TravelDate, req.SeatType) {
		dayBlocked[id] = true
	}
	for _, inv := range invs {
		if dayBlocked[inv.SeatID] {
			continue
		}
		var seat models.Seat
		if tx.First(&seat, inv.SeatID).Error == nil && seat.Blocked {
			continue
		}
		totalSeats++
		if !Available(inv.Occupancy, needMask) {
			sold++
		}
	}
	if pct, err := sale.CumulativeReleasePct(tx, req.TrainID, req.SeatType, req.TravelDate, req.FromSeq, req.ToSeq, time.Now()); err == nil {
		maxSellable := totalSeats
		if pct < 100 {
			maxSellable = totalSeats * pct / 100
		}
		if sold+req.Count > maxSellable {
			return nil, fmt.Errorf("当前放票波次余票不足（已放 %d%%，可售 %d，已售 %d）", pct, maxSellable, sold)
		}
	}

	type cand struct {
		inv   models.SeatInventory
		seat  models.Seat
		score int
	}
	cands := make([]cand, 0)
	for _, inv := range invs {
		if !Available(inv.Occupancy, need) {
			continue
		}
		if dayBlocked[inv.SeatID] {
			continue
		}
		var seat models.Seat
		if err := tx.First(&seat, inv.SeatID).Error; err != nil {
			return nil, err
		}
		if seat.Blocked {
			continue
		}
		score := 0
		if req.Preference.CarriageNo != "" && seat.CarriageNo == req.Preference.CarriageNo {
			score += 10
		}
		if req.Preference.Position != "" && seat.Position == req.Preference.Position {
			score += 5
		}
		cands = append(cands, cand{inv: inv, seat: seat, score: score})
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		return cands[i].seat.ID < cands[j].seat.ID
	})

	chosen := make([]SeatPick, 0, req.Count)
	for _, c := range cands {
		newOcc := Occupy(c.inv.Occupancy, need)
		res := tx.Model(&models.SeatInventory{}).
			Where("id = ? AND version = ?", c.inv.ID, c.inv.Version).
			Updates(map[string]interface{}{
				"occupancy":  newOcc,
				"version":    c.inv.Version + 1,
				"updated_at": time.Now(),
			})
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected == 0 {
			return nil, errors.New("座位冲突，请重试")
		}
		c.inv.Occupancy = newOcc
		c.inv.Version++
		chosen = append(chosen, SeatPick{Inv: c.inv, Seat: c.seat})
		if len(chosen) == req.Count {
			break
		}
	}
	if len(chosen) < req.Count {
		if req.Preference.Position != "" || req.Preference.CarriageNo != "" {
			return nil, errors.New("符合选座偏好的余票不足，请调整偏好或减少人数")
		}
		return nil, errors.New("余票不足")
	}
	return chosen, nil
}

// OccupyAndPublish locks train, occupies, publishes event.
func (s *Service) OccupyAndPublish(ctx context.Context, req OccupyRequest) ([]SeatPickDTO, error) {
	if err := s.EnsureDaily(req.TrainID, req.TravelDate); err != nil {
		return nil, err
	}
	var picks []SeatPick
	err := s.WithTrainLock(ctx, req.TrainID, req.TravelDate, func() error {
		return s.DB.Transaction(func(tx *gorm.DB) error {
			var e error
			picks, e = s.OccupyLocked(tx, req)
			return e
		})
	})
	if err != nil {
		return nil, err
	}
	out := make([]SeatPickDTO, 0, len(picks))
	for _, p := range picks {
		out = append(out, SeatPickDTO{
			SeatID: p.Seat.ID, CarriageNo: p.Seat.CarriageNo, SeatNo: p.Seat.SeatNo,
			SeatType: p.Seat.SeatType, Position: p.Seat.Position,
		})
	}
	s.Bus.Publish(ctx, events.Event{
		Kind: events.KindOccupied, TrainID: req.TrainID, TravelDate: req.TravelDate,
		FromStation: req.FromStation, ToStation: req.ToStation, SeatType: req.SeatType,
	})
	s.rebuildAsync(req.TrainID, req.TravelDate)
	return out, nil
}

type ReleaseRequest struct {
	TrainID     uint   `json:"train_id"`
	TravelDate  string `json:"travel_date"`
	SeatID      uint   `json:"seat_id"`
	FromSeq     int    `json:"from_seq"`
	ToSeq       int    `json:"to_seq"`
	FromStation string `json:"from_station"`
	ToStation   string `json:"to_station"`
}

func (s *Service) ReleaseSeat(tx *gorm.DB, trainID uint, date string, seatID uint, fromSeq, toSeq int) error {
	need := SegmentMask(fromSeq, toSeq)
	var inv models.SeatInventory
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("train_id = ? AND travel_date = ? AND seat_id = ?", trainID, date, seatID).
		First(&inv).Error; err != nil {
		return err
	}
	newOcc := Release(inv.Occupancy, need)
	return tx.Model(&inv).Updates(map[string]interface{}{
		"occupancy":  newOcc,
		"version":    inv.Version + 1,
		"updated_at": time.Now(),
	}).Error
}

func (s *Service) ReleaseAndPublish(ctx context.Context, req ReleaseRequest) error {
	err := s.WithTrainLock(ctx, req.TrainID, req.TravelDate, func() error {
		return s.DB.Transaction(func(tx *gorm.DB) error {
			return s.ReleaseSeat(tx, req.TrainID, req.TravelDate, req.SeatID, req.FromSeq, req.ToSeq)
		})
	})
	if err != nil {
		return err
	}
	s.Bus.Publish(ctx, events.Event{
		Kind: events.KindReleased, TrainID: req.TrainID, TravelDate: req.TravelDate,
		FromStation: req.FromStation, ToStation: req.ToStation,
	})
	s.rebuildAsync(req.TrainID, req.TravelDate)
	return nil
}

func (s *Service) ListSeats(trainID uint) ([]models.Seat, error) {
	var list []models.Seat
	q := s.DB.Model(&models.Seat{})
	if trainID > 0 {
		q = q.Where("train_id = ?", trainID)
	}
	err := q.Order("train_id, carriage_no, seat_no").Find(&list).Error
	return list, err
}

func (s *Service) SetSeatBlocked(seatID uint, blocked bool, reason string) (*models.Seat, error) {
	var seat models.Seat
	if err := s.DB.First(&seat, seatID).Error; err != nil {
		return nil, errors.New("座位不存在")
	}
	updates := map[string]interface{}{"blocked": blocked}
	if blocked {
		updates["block_reason"] = reason
	} else {
		updates["block_reason"] = ""
	}
	if err := s.DB.Model(&seat).Updates(updates).Error; err != nil {
		return nil, err
	}
	seat.Blocked = blocked
	if blocked {
		seat.BlockReason = reason
	} else {
		seat.BlockReason = ""
	}
	s.Bus.Publish(context.Background(), events.Event{
		Kind: events.KindSeatBlock, TrainID: seat.TrainID,
	})
	var dates []string
	s.DB.Model(&models.SeatInventory{}).Where("train_id = ?", seat.TrainID).Distinct("travel_date").Limit(30).Pluck("travel_date", &dates)
	for _, d := range dates {
		s.rebuildAsync(seat.TrainID, d)
	}
	return &seat, nil
}

func (s *Service) ListDayBlocks(trainID uint, date string) ([]models.SeatDayBlock, error) {
	var list []models.SeatDayBlock
	q := s.DB.Model(&models.SeatDayBlock{})
	if trainID > 0 {
		q = q.Where("train_id = ?", trainID)
	}
	if date != "" {
		q = q.Where("travel_date = ?", date)
	}
	err := q.Order("id desc").Limit(500).Find(&list).Error
	return list, err
}

func (s *Service) UpsertDayBlock(trainID, seatID uint, date, reason string) (*models.SeatDayBlock, error) {
	if seatID == 0 || date == "" {
		return nil, errors.New("座位与日期必填")
	}
	var seat models.Seat
	if err := s.DB.First(&seat, seatID).Error; err != nil {
		return nil, errors.New("座位不存在")
	}
	if trainID == 0 {
		trainID = seat.TrainID
	}
	var exist models.SeatDayBlock
	err := s.DB.Where("seat_id = ? AND travel_date = ?", seatID, date).First(&exist).Error
	if err == nil {
		_ = s.DB.Model(&exist).Update("reason", reason).Error
		exist.Reason = reason
		s.rebuildAsync(trainID, date)
		return &exist, nil
	}
	b := &models.SeatDayBlock{TrainID: trainID, SeatID: seatID, TravelDate: date, Reason: reason}
	if err := s.DB.Create(b).Error; err != nil {
		return nil, err
	}
	s.Bus.Publish(context.Background(), events.Event{Kind: events.KindSeatBlock, TrainID: trainID, TravelDate: date})
	s.rebuildAsync(trainID, date)
	return b, nil
}

func (s *Service) DeleteDayBlock(id uint) error {
	var b models.SeatDayBlock
	if err := s.DB.First(&b, id).Error; err != nil {
		return errors.New("记录不存在")
	}
	if err := s.DB.Delete(&b).Error; err != nil {
		return err
	}
	s.rebuildAsync(b.TrainID, b.TravelDate)
	return nil
}

func (s *Service) InvalidateQueryCache(from, to, date string) {
	if s.RDB == nil {
		return
	}
	ctx := context.Background()
	_ = s.RDB.Del(ctx, fmt.Sprintf("tickets:%s:%s:%s", from, to, date)).Err()
}

// PositionFromSeatLetter maps A/F -> 靠窗, C/D -> 过道, else 中间 (demo rule).
func PositionFromSeatLetter(letter byte) string {
	switch letter {
	case 'A', 'F', 'a', 'f':
		return "靠窗"
	case 'C', 'D', 'c', 'd':
		return "过道"
	default:
		return "中间"
	}
}
