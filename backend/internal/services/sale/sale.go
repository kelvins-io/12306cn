package sale

import (
	"errors"
	"fmt"
	"time"

	"github.com/kelvins-io/12306cn/backend/internal/models"
	"gorm.io/gorm"
)

// CheckOpen returns nil if travelDate is on sale now (base policy).
func CheckOpen(db *gorm.DB, travelDate string, now time.Time) error {
	var p models.SalePolicy
	err := db.Where("enabled = ?", true).Order("id asc").First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	td, err := time.ParseInLocation("2006-01-02", travelDate, now.Location())
	if err != nil {
		return errors.New("出行日期格式错误")
	}
	openAt := td.AddDate(0, 0, -p.AdvanceDays).
		Add(time.Duration(p.OpenHour)*time.Hour + time.Duration(p.OpenMinute)*time.Minute)
	if now.Before(openAt) {
		return fmt.Errorf("该日期车票未开售，预计 %s 起售", openAt.Format("2006-01-02 15:04"))
	}
	if td.Before(now.Truncate(24 * time.Hour)) {
		return errors.New("不能购买已过去的日期")
	}
	return nil
}

func OpenAt(db *gorm.DB, travelDate string, loc *time.Location) (*time.Time, error) {
	var p models.SalePolicy
	if err := db.Where("enabled = ?", true).Order("id asc").First(&p).Error; err != nil {
		return nil, err
	}
	td, err := time.ParseInLocation("2006-01-02", travelDate, loc)
	if err != nil {
		return nil, err
	}
	openAt := td.AddDate(0, 0, -p.AdvanceDays).
		Add(time.Duration(p.OpenHour)*time.Hour + time.Duration(p.OpenMinute)*time.Minute)
	return &openAt, nil
}

// CumulativeReleasePct returns how much inventory (0-100) is released for the OD now.
// Matching waves (seat type / train / intersecting segment) that have opened contribute ReleasePct.
func CumulativeReleasePct(db *gorm.DB, trainID uint, seatType, travelDate string, fromSeq, toSeq int, now time.Time) (int, error) {
	var waves []models.SaleWave
	if err := db.Where("enabled = ?", true).Order("sort_order, id").Find(&waves).Error; err != nil {
		return 100, err
	}
	if len(waves) == 0 {
		return 100, nil // no waves = fully released after base open
	}
	td, err := time.ParseInLocation("2006-01-02", travelDate, now.Location())
	if err != nil {
		return 0, errors.New("出行日期格式错误")
	}
	sum := 0
	matched := 0
	for _, w := range waves {
		if w.TrainID > 0 && w.TrainID != trainID {
			continue
		}
		if w.SeatType != "" && w.SeatType != seatType {
			continue
		}
		if w.SegIndex != nil {
			si := *w.SegIndex
			if si < fromSeq || si >= toSeq {
				continue
			}
		}
		matched++
		openAt := td.AddDate(0, 0, -w.AdvanceDays).
			Add(time.Duration(w.OpenHour)*time.Hour + time.Duration(w.OpenMinute)*time.Minute)
		if now.Before(openAt) {
			continue
		}
		sum += w.ReleasePct
	}
	if matched == 0 {
		return 100, nil // 无匹配波次则视为全量放票
	}
	if sum <= 0 {
		return 0, nil
	}
	if sum > 100 {
		sum = 100
	}
	return sum, nil
}

// ApplyReleaseCap limits free seats by wave release: maxSellable = total * pct/100, left = maxSellable - sold.
func ApplyReleaseCap(physicalFree, totalSeats, sold, releasePct int) int {
	if releasePct >= 100 {
		return physicalFree
	}
	if releasePct <= 0 || totalSeats <= 0 {
		return 0
	}
	maxSellable := totalSeats * releasePct / 100
	left := maxSellable - sold
	if left < 0 {
		left = 0
	}
	if left < physicalFree {
		return left
	}
	return physicalFree
}
