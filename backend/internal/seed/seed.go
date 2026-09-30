package seed

import (
	"fmt"
	"log"

	"github.com/kelvins-io/12306cn/backend/internal/models"
	"github.com/kelvins-io/12306cn/backend/internal/services/inventory"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func Run(db *gorm.DB) error {
	var n int64
	db.Model(&models.Station{}).Count(&n)
	if n > 0 {
		_ = backfillSeatPositions(db)
		return ensureMeta(db)
	}
	log.Println("seeding demo data...")

	stations := []models.Station{
		{Code: "BJP", Name: "北京", City: "北京"},
		{Code: "TJP", Name: "天津", City: "天津"},
		{Code: "Jinan", Name: "济南", City: "济南"},
		{Code: "NJH", Name: "南京", City: "南京"},
		{Code: "SHH", Name: "上海", City: "上海"},
		{Code: "GZQ", Name: "广州", City: "广州"},
		{Code: "SZQ", Name: "深圳", City: "深圳"},
		{Code: "WHN", Name: "武汉", City: "武汉"},
		{Code: "CDW", Name: "成都", City: "成都"},
		{Code: "XAY", Name: "西安", City: "西安"},
	}
	if err := db.Create(&stations).Error; err != nil {
		return err
	}

	type stopDef struct {
		Name, Arrive, Depart string
		KM                   int
	}
	trains := []struct {
		No, Type, Start, End string
		Stops                []stopDef
		Seats                map[string]int // seatType -> count
	}{
		{
			No: "G1", Type: "G", Start: "北京", End: "上海",
			Stops: []stopDef{
				{"北京", "", "07:00", 0},
				{"天津", "07:35", "07:37", 120},
				{"济南", "09:10", "09:12", 410},
				{"南京", "11:20", "11:23", 1020},
				{"上海", "13:00", "", 1318},
			},
			Seats: map[string]int{"二等座": 40, "一等座": 16, "商务座": 8},
		},
		{
			No: "G7", Type: "G", Start: "北京", End: "深圳",
			Stops: []stopDef{
				{"北京", "", "08:00", 0},
				{"武汉", "12:30", "12:35", 1220},
				{"广州", "16:10", "16:15", 2100},
				{"深圳", "17:20", "", 2300},
			},
			Seats: map[string]int{"二等座": 36, "一等座": 12, "商务座": 6},
		},
		{
			No: "D3125", Type: "D", Start: "上海", End: "深圳",
			Stops: []stopDef{
				{"上海", "", "09:30", 0},
				{"南京", "11:00", "11:05", 300},
				{"武汉", "15:00", "15:08", 900},
				{"广州", "19:40", "19:45", 1600},
				{"深圳", "20:50", "", 1750},
			},
			Seats: map[string]int{"二等座": 48, "一等座": 20},
		},
		{
			No: "K257", Type: "K", Start: "北京", End: "成都",
			Stops: []stopDef{
				{"北京", "", "18:00", 0},
				{"西安", "06:20", "06:40", 1200},
				{"成都", "14:30", "", 1800},
			},
			Seats: map[string]int{"硬座": 50, "硬卧": 30},
		},
	}

	stationID := map[string]uint{}
	for _, st := range stations {
		stationID[st.Name] = st.ID
	}

	for _, t := range trains {
		train := models.Train{TrainNo: t.No, TrainType: t.Type, StartStation: t.Start, EndStation: t.End}
		if err := db.Create(&train).Error; err != nil {
			return err
		}
		for i, sp := range t.Stops {
			stop := models.TrainStop{
				TrainID:     train.ID,
				StationID:   stationID[sp.Name],
				StationName: sp.Name,
				Seq:         i,
				ArriveTime:  sp.Arrive,
				DepartTime:  sp.Depart,
				DistanceKM:  sp.KM,
			}
			if stop.ArriveTime == "" {
				stop.ArriveTime = sp.Depart
			}
			if stop.DepartTime == "" {
				stop.DepartTime = sp.Arrive
			}
			if err := db.Create(&stop).Error; err != nil {
				return err
			}
		}
		seatIdx := 1
		for seatType, count := range t.Seats {
			for range count {
				carriage := fmt.Sprintf("%02d", (seatIdx-1)/20+1)
				row := (seatIdx-1)%20 + 1
				letter := byte('A' + ((seatIdx - 1) % 5))
				seat := models.Seat{
					TrainID:    train.ID,
					SeatType:   seatType,
					CarriageNo: carriage,
					SeatNo:     fmt.Sprintf("%s%02d%c", carriage, row, letter),
					Position:   inventory.PositionFromSeatLetter(letter),
				}
				if err := db.Create(&seat).Error; err != nil {
					return err
				}
				seatIdx++
			}
		}
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	demo := models.User{Username: "demo", PasswordHash: string(hash), Nickname: "演示用户", Phone: "13800000000", Role: "user"}
	if err := db.Create(&demo).Error; err != nil {
		return err
	}
	adminHash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	_ = db.Create(&models.User{Username: "admin", PasswordHash: string(adminHash), Nickname: "运营管理员", Role: "admin"}).Error
	stationHash, _ := bcrypt.GenerateFromPassword([]byte("station123"), bcrypt.DefaultCost)
	_ = db.Create(&models.User{Username: "station", PasswordHash: string(stationHash), Nickname: "车站核验", Role: "station"}).Error
	ps := []models.Passenger{
		{UserID: demo.ID, Name: "张三", IDType: "身份证", IDNumber: "110101199001011234", PassengerType: "成人"},
		{UserID: demo.ID, Name: "李四", IDType: "身份证", IDNumber: "110101199202021234", PassengerType: "成人"},
	}
	if err := db.Create(&ps).Error; err != nil {
		return err
	}
	return ensureMeta(db)
}

func ensureMeta(db *gorm.DB) error {
	if err := ensureQuotas(db); err != nil {
		return err
	}
	if err := ensureSalePolicy(db); err != nil {
		return err
	}
	if err := ensureSaleWaves(db); err != nil {
		return err
	}
	return ensureStaffUsers(db)
}

func ensureSaleWaves(db *gorm.DB) error {
	var cnt int64
	db.Model(&models.SaleWave{}).Count(&cnt)
	if cnt > 0 {
		return nil
	}
	log.Println("seeding sale waves (阶梯放票)...")
	seg0 := 0
	waves := []models.SaleWave{
		{Name: "首波-二等座50%", AdvanceDays: 15, OpenHour: 8, OpenMinute: 0, SeatType: "二等座", ReleasePct: 50, Enabled: true, SortOrder: 1},
		{Name: "二波-二等座+30%", AdvanceDays: 10, OpenHour: 8, OpenMinute: 0, SeatType: "二等座", ReleasePct: 30, Enabled: true, SortOrder: 2},
		{Name: "三波-二等座余量", AdvanceDays: 5, OpenHour: 8, OpenMinute: 0, SeatType: "二等座", ReleasePct: 20, Enabled: true, SortOrder: 3},
		{Name: "首波-一等商务全量", AdvanceDays: 15, OpenHour: 8, OpenMinute: 0, SeatType: "一等座", ReleasePct: 100, Enabled: true, SortOrder: 4},
		{Name: "首波-商务全量", AdvanceDays: 15, OpenHour: 8, OpenMinute: 0, SeatType: "商务座", ReleasePct: 100, Enabled: true, SortOrder: 5},
		{Name: "首波-短途区段40%", AdvanceDays: 15, OpenHour: 8, OpenMinute: 0, SeatType: "二等座", SegIndex: &seg0, TrainID: 1, ReleasePct: 40, Enabled: false, SortOrder: 6},
	}
	return db.Create(&waves).Error
}

func ensureStaffUsers(db *gorm.DB) error {
	ensure := func(username, pass, nickname, role string) {
		var n int64
		db.Model(&models.User{}).Where("username = ?", username).Count(&n)
		if n > 0 {
			_ = db.Model(&models.User{}).Where("username = ?", username).Update("role", role).Error
			return
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
		_ = db.Create(&models.User{Username: username, PasswordHash: string(hash), Nickname: nickname, Role: role}).Error
	}
	ensure("admin", "admin123", "运营管理员", "admin")
	ensure("station", "station123", "车站核验", "station")
	_ = db.Model(&models.User{}).Where("username = ?", "demo").Update("role", "user").Error
	return nil
}

func backfillSeatPositions(db *gorm.DB) error {
	var seats []models.Seat
	if err := db.Find(&seats).Error; err != nil {
		return err
	}
	for _, seat := range seats {
		if len(seat.SeatNo) == 0 {
			continue
		}
		letter := seat.SeatNo[len(seat.SeatNo)-1]
		pos := inventory.PositionFromSeatLetter(letter)
		if seat.Position != pos {
			_ = db.Model(&seat).Update("position", pos).Error
		}
	}
	return nil
}

func ensureSalePolicy(db *gorm.DB) error {
	var cnt int64
	db.Model(&models.SalePolicy{}).Count(&cnt)
	if cnt > 0 {
		return nil
	}
	log.Println("seeding sale policy (提前15天 08:00 开售)...")
	return db.Create(&models.SalePolicy{
		Name: "默认放票", AdvanceDays: 15, OpenHour: 8, OpenMinute: 0, Enabled: true,
	}).Error
}

// ensureQuotas: G1 二等座短途区段限额低于物理座位数，演示区段配额。
func ensureQuotas(db *gorm.DB) error {
	var cnt int64
	db.Model(&models.SegmentQuota{}).Count(&cnt)
	if cnt > 0 {
		return nil
	}
	var g1 models.Train
	if err := db.Where("train_no = ?", "G1").First(&g1).Error; err != nil {
		return nil
	}
	quotas := []models.SegmentQuota{
		{TrainID: g1.ID, SeatType: "二等座", SegIndex: 0, MaxSold: 20},
		{TrainID: g1.ID, SeatType: "二等座", SegIndex: 1, MaxSold: 30},
		{TrainID: g1.ID, SeatType: "二等座", SegIndex: 2, MaxSold: 35},
		{TrainID: g1.ID, SeatType: "二等座", SegIndex: 3, MaxSold: 40},
		{TrainID: g1.ID, SeatType: "一等座", SegIndex: 0, MaxSold: 10},
		{TrainID: g1.ID, SeatType: "商务座", SegIndex: 0, MaxSold: 4},
	}
	log.Println("seeding segment quotas for G1...")
	return db.Create(&quotas).Error
}
