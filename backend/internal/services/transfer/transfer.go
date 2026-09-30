package transfer

import (
	"fmt"
	"sort"

	"github.com/kelvins-io/12306cn/backend/internal/models"
	"github.com/kelvins-io/12306cn/backend/internal/services/pricing"
	"gorm.io/gorm"
)

const MinTransferMinutes = 30

type Leg struct {
	TrainID     uint           `json:"train_id"`
	TrainNo     string         `json:"train_no"`
	TrainType   string         `json:"train_type"`
	FromStation string         `json:"from_station"`
	ToStation   string         `json:"to_station"`
	DepartTime  string         `json:"depart_time"`
	ArriveTime  string         `json:"arrive_time"`
	TravelDate  string         `json:"travel_date"`
	FromSeq     int            `json:"from_seq"`
	ToSeq       int            `json:"to_seq"`
	DurationMin int            `json:"duration_min"`
	SeatPrice   map[string]int `json:"seat_price"`
	SeatTypes   []string       `json:"seat_types"`
}

type Plan struct {
	HubStation     string `json:"hub_station"`
	WaitMinutes    int    `json:"wait_minutes"`
	TotalDuration  int    `json:"total_duration_min"`
	Leg1           Leg    `json:"leg1"`
	Leg2           Leg    `json:"leg2"`
}

func durationMin(depart, arrive string) int {
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

func minutesOf(hhmm string) int {
	var h, m int
	fmt.Sscanf(hhmm, "%d:%d", &h, &m)
	return h*60 + m
}

// Search finds same-day one-hub transfers from->to.
func Search(db *gorm.DB, from, to, date string, limit int) ([]Plan, error) {
	if from == to {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	var fromStops []models.TrainStop
	if err := db.Where("station_name = ?", from).Find(&fromStops).Error; err != nil {
		return nil, err
	}
	plans := make([]Plan, 0)
	seen := map[string]bool{}

	for _, fs := range fromStops {
		var hubs []models.TrainStop
		if err := db.Where("train_id = ? AND seq > ?", fs.TrainID, fs.Seq).Find(&hubs).Error; err != nil {
			continue
		}
		var train1 models.Train
		if err := db.First(&train1, fs.TrainID).Error; err != nil {
			continue
		}
		for _, hub := range hubs {
			if hub.StationName == to {
				continue // direct, skip
			}
			arrive1 := hub.ArriveTime
			if arrive1 == "" {
				arrive1 = hub.DepartTime
			}
			// second legs departing hub toward destination
			var hubDeparts []models.TrainStop
			if err := db.Where("station_name = ?", hub.StationName).Find(&hubDeparts).Error; err != nil {
				continue
			}
			for _, hd := range hubDeparts {
				if hd.TrainID == fs.TrainID {
					continue
				}
				var dest models.TrainStop
				if err := db.Where("train_id = ? AND station_name = ? AND seq > ?", hd.TrainID, to, hd.Seq).First(&dest).Error; err != nil {
					continue
				}
				depart2 := hd.DepartTime
				if depart2 == "" {
					depart2 = hd.ArriveTime
				}
				wait := minutesOf(depart2) - minutesOf(arrive1)
				if wait < MinTransferMinutes {
					continue
				}
				var train2 models.Train
				if err := db.First(&train2, hd.TrainID).Error; err != nil {
					continue
				}
				key := fmt.Sprintf("%d-%d-%s", train1.ID, train2.ID, hub.StationName)
				if seen[key] {
					continue
				}
				seen[key] = true

				leg1Types := seatTypes(db, train1.ID)
				leg2Types := seatTypes(db, train2.ID)
				leg1 := Leg{
					TrainID: train1.ID, TrainNo: train1.TrainNo, TrainType: train1.TrainType,
					FromStation: from, ToStation: hub.StationName,
					DepartTime: fs.DepartTime, ArriveTime: arrive1, TravelDate: date,
					FromSeq: fs.Seq, ToSeq: hub.Seq, DurationMin: durationMin(fs.DepartTime, arrive1),
					SeatTypes: leg1Types, SeatPrice: prices(train1.TrainType, fs.Seq, hub.Seq, leg1Types),
				}
				leg2 := Leg{
					TrainID: train2.ID, TrainNo: train2.TrainNo, TrainType: train2.TrainType,
					FromStation: hub.StationName, ToStation: to,
					DepartTime: depart2, ArriveTime: dest.ArriveTime, TravelDate: date,
					FromSeq: hd.Seq, ToSeq: dest.Seq, DurationMin: durationMin(depart2, dest.ArriveTime),
					SeatTypes: leg2Types, SeatPrice: prices(train2.TrainType, hd.Seq, dest.Seq, leg2Types),
				}
				total := leg1.DurationMin + wait + leg2.DurationMin
				plans = append(plans, Plan{
					HubStation: hub.StationName, WaitMinutes: wait, TotalDuration: total,
					Leg1: leg1, Leg2: leg2,
				})
			}
		}
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].TotalDuration != plans[j].TotalDuration {
			return plans[i].TotalDuration < plans[j].TotalDuration
		}
		return plans[i].WaitMinutes < plans[j].WaitMinutes
	})
	if len(plans) > limit {
		plans = plans[:limit]
	}
	return plans, nil
}

func seatTypes(db *gorm.DB, trainID uint) []string {
	var types []string
	db.Model(&models.Seat{}).Where("train_id = ?", trainID).Distinct("seat_type").Pluck("seat_type", &types)
	return types
}

func prices(trainType string, fromSeq, toSeq int, types []string) map[string]int {
	m := map[string]int{}
	for _, t := range types {
		m[t] = pricing.BasePrice(t, trainType, fromSeq, toSeq)
	}
	return m
}
