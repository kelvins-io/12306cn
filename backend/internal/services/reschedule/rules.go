package reschedule

import (
	"errors"
	"fmt"
	"time"

	"github.com/kelvins-io/12306cn/backend/internal/models"
	"gorm.io/gorm"
)

type Config struct {
	MaxTimes     int // max reschedule hops in a chain
	HoursBefore  int // must be >= N hours before departure
}

func DefaultConfig() Config {
	return Config{MaxTimes: 2, HoursBefore: 2}
}

// CountChain returns how many times this ticket lineage has been rescheduled.
func CountChain(db *gorm.DB, order models.Order) int {
	n := order.RescheduleCount
	if n > 0 {
		return n
	}
	// fallback: walk parents
	cur := order
	hops := 0
	for cur.ParentOrderID != nil && hops < 20 {
		hops++
		var p models.Order
		if err := db.Select("id, parent_order_id, reschedule_count").First(&p, *cur.ParentOrderID).Error; err != nil {
			break
		}
		if p.RescheduleCount > 0 {
			return p.RescheduleCount + hops
		}
		cur = p
	}
	return hops
}

func CheckAllowed(db *gorm.DB, order models.Order, departHHMM, travelDate string, now time.Time, cfg Config) error {
	if cfg.MaxTimes <= 0 {
		cfg.MaxTimes = 2
	}
	if cfg.HoursBefore < 0 {
		cfg.HoursBefore = 2
	}
	used := CountChain(db, order)
	if used >= cfg.MaxTimes {
		return fmt.Errorf("已改签 %d 次，达到上限 %d 次", used, cfg.MaxTimes)
	}
	dep, err := parseDepart(travelDate, departHHMM)
	if err != nil {
		return err
	}
	deadline := dep.Add(-time.Duration(cfg.HoursBefore) * time.Hour)
	if now.After(deadline) {
		return fmt.Errorf("距发车不足 %d 小时，不可改签", cfg.HoursBefore)
	}
	return nil
}

func parseDepart(date, hhmm string) (time.Time, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if loc == nil {
		loc = time.Local
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", date+" "+hhmm, loc)
	if err != nil {
		return time.Time{}, errors.New("发车时间无效")
	}
	return t, nil
}

// DiffAmount returns newTotal - oldTotal (positive means passenger owes more).
func DiffAmount(oldTotal, newTotal int) int {
	return newTotal - oldTotal
}
