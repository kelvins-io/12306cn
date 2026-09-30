package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Username     string         `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	Nickname     string         `gorm:"size:64" json:"nickname"`
	Phone        string         `gorm:"size:20" json:"phone"`
	Role         string         `gorm:"size:16;not null;default:user;index" json:"role"` // user/admin/station
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

type Passenger struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	UserID     uint           `gorm:"index;not null" json:"user_id"`
	Name       string         `gorm:"size:64;not null" json:"name"`
	IDType     string         `gorm:"size:20;not null;default:身份证" json:"id_type"`
	IDNumber   string         `gorm:"size:32;not null" json:"id_number"`
	PassengerType string      `gorm:"size:20;not null;default:成人" json:"passenger_type"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

type Station struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Code      string    `gorm:"size:16;uniqueIndex;not null" json:"code"`
	Name      string    `gorm:"size:64;not null;index" json:"name"`
	City      string    `gorm:"size:64;index" json:"city"`
	CreatedAt time.Time `json:"created_at"`
}

type Train struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TrainNo     string    `gorm:"size:16;uniqueIndex;not null" json:"train_no"`
	TrainType   string    `gorm:"size:16;not null" json:"train_type"` // G/D/K/T
	StartStation string   `gorm:"size:64;not null" json:"start_station"`
	EndStation  string    `gorm:"size:64;not null" json:"end_station"`
	CreatedAt   time.Time `json:"created_at"`
}

type TrainStop struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	TrainID       uint      `gorm:"uniqueIndex:idx_train_seq;not null" json:"train_id"`
	StationID     uint      `gorm:"index;not null" json:"station_id"`
	StationName   string    `gorm:"size:64;not null" json:"station_name"`
	Seq           int       `gorm:"uniqueIndex:idx_train_seq;not null" json:"seq"` // 0-based
	ArriveTime    string    `gorm:"size:8" json:"arrive_time"`                      // HH:MM
	DepartTime    string    `gorm:"size:8" json:"depart_time"`
	DistanceKM    int       `gorm:"default:0" json:"distance_km"`
	CreatedAt     time.Time `json:"created_at"`
}

// SeatType: 二等座/一等座/商务座/硬座/硬卧
type SeatType string

const (
	SeatSecond  SeatType = "二等座"
	SeatFirst   SeatType = "一等座"
	SeatBusiness SeatType = "商务座"
	SeatHard    SeatType = "硬座"
	SeatHardSleeper SeatType = "硬卧"
)

type Seat struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TrainID     uint      `gorm:"uniqueIndex:idx_train_seat;not null" json:"train_id"`
	SeatType    string    `gorm:"size:16;not null;index" json:"seat_type"`
	CarriageNo  string    `gorm:"size:8;not null" json:"carriage_no"`
	SeatNo      string    `gorm:"size:8;uniqueIndex:idx_train_seat;not null" json:"seat_no"`
	Position    string    `gorm:"size:8;not null;default:中间;index" json:"position"` // 靠窗/过道/中间
	Blocked     bool      `gorm:"not null;default:false;index" json:"blocked"`
	BlockReason string    `gorm:"size:128" json:"block_reason"`
	CreatedAt   time.Time `json:"created_at"`
}

// SeatInventory tracks occupancy bitmap per seat per travel date.
// Bit i means segment between stop i and i+1 is occupied.
type SeatInventory struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TrainID    uint      `gorm:"uniqueIndex:idx_inv;not null" json:"train_id"`
	TravelDate string    `gorm:"size:10;uniqueIndex:idx_inv;not null" json:"travel_date"` // YYYY-MM-DD
	SeatID     uint      `gorm:"uniqueIndex:idx_inv;not null" json:"seat_id"`
	SeatType   string    `gorm:"size:16;not null;index" json:"seat_type"`
	Occupancy  uint64    `gorm:"not null;default:0" json:"occupancy"`
	Version    int64     `gorm:"not null;default:0" json:"version"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type OrderStatus string

const (
	OrderPendingPay  OrderStatus = "pending_pay"
	OrderPaid        OrderStatus = "paid"
	OrderCancelled   OrderStatus = "cancelled"
	OrderRefunded    OrderStatus = "refunded"
	OrderExpired     OrderStatus = "expired"
	OrderRescheduled OrderStatus = "rescheduled"
)

type Order struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	OrderNo       string         `gorm:"size:32;uniqueIndex;not null" json:"order_no"`
	UserID        uint           `gorm:"index;not null" json:"user_id"`
	TrainID       uint           `gorm:"index;not null" json:"train_id"`
	TrainNo       string         `gorm:"size:16;not null" json:"train_no"`
	TravelDate    string         `gorm:"size:10;not null;index" json:"travel_date"`
	FromStation   string         `gorm:"size:64;not null" json:"from_station"`
	ToStation     string         `gorm:"size:64;not null" json:"to_station"`
	FromSeq       int            `gorm:"not null" json:"from_seq"`
	ToSeq         int            `gorm:"not null" json:"to_seq"`
	SeatType      string         `gorm:"size:16;not null" json:"seat_type"`
	Status        string         `gorm:"size:20;not null;index" json:"status"`
	TotalAmount   int            `gorm:"not null" json:"total_amount"` // fen
	ExpireAt      *time.Time     `json:"expire_at"`
	PaidAt        *time.Time     `json:"paid_at"`
	ParentOrderID   *uint          `gorm:"index" json:"parent_order_id,omitempty"` // 改签来源订单
	RescheduleCount int            `gorm:"not null;default:0" json:"reschedule_count"`
	PriceDiff       int            `gorm:"not null;default:0" json:"price_diff"`
	TransferGroupID string         `gorm:"size:36;index" json:"transfer_group_id,omitempty"`
	TransferLeg     int            `gorm:"not null;default:0" json:"transfer_leg"` // 0=直达 1/2=换乘程
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
	Tickets         []Ticket       `gorm:"foreignKey:OrderID" json:"tickets,omitempty"`
}

type Ticket struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	OrderID       uint       `gorm:"index;not null" json:"order_id"`
	TicketNo      string     `gorm:"size:32;uniqueIndex;not null" json:"ticket_no"`
	PassengerName string     `gorm:"size:64;not null" json:"passenger_name"`
	PassengerID   string     `gorm:"size:32;not null" json:"passenger_id"`
	PassengerType string     `gorm:"size:20;not null;default:成人" json:"passenger_type"`
	SeatID        uint       `gorm:"not null" json:"seat_id"`
	CarriageNo    string     `gorm:"size:8;not null" json:"carriage_no"`
	SeatNo        string     `gorm:"size:8;not null" json:"seat_no"`
	SeatType      string     `gorm:"size:16;not null" json:"seat_type"`
	Price         int        `gorm:"not null" json:"price"`
	Status        string     `gorm:"size:20;not null;default:held;index" json:"status"` // held/issued/verified/refunded
	VerifyCode    string     `gorm:"size:16;index" json:"verify_code"`
	VerifiedAt    *time.Time `json:"verified_at"`
	VerifiedBy    string     `gorm:"size:64" json:"verified_by"`
	CreatedAt     time.Time  `json:"created_at"`
}

// Payment records payment attempts against an order.
type Payment struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	PaymentNo     string     `gorm:"size:32;uniqueIndex;not null" json:"payment_no"`
	OrderID       uint       `gorm:"index;not null" json:"order_id"`
	UserID        uint       `gorm:"index;not null" json:"user_id"`
	Provider      string     `gorm:"size:32;not null" json:"provider"` // mock/wallet
	Amount        int        `gorm:"not null" json:"amount"`
	Status        string     `gorm:"size:20;not null;index" json:"status"` // pending/success/failed
	ProviderRef   string     `gorm:"size:64" json:"provider_ref"`
	FailReason    string     `gorm:"size:128" json:"fail_reason"`
	PaidAt        *time.Time `json:"paid_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type WaitlistStatus string

const (
	WaitlistPending   WaitlistStatus = "pending"
	WaitlistFulfilled WaitlistStatus = "fulfilled"
	WaitlistCancelled WaitlistStatus = "cancelled"
	WaitlistExpired   WaitlistStatus = "expired"
)

type Waitlist struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"index;not null" json:"user_id"`
	TrainID       uint      `gorm:"index;not null" json:"train_id"`
	TrainNo       string    `gorm:"size:16;not null" json:"train_no"`
	TravelDate    string    `gorm:"size:10;not null;index" json:"travel_date"`
	FromStation   string    `gorm:"size:64;not null" json:"from_station"`
	ToStation     string    `gorm:"size:64;not null" json:"to_station"`
	FromSeq       int       `gorm:"not null" json:"from_seq"`
	ToSeq         int       `gorm:"not null" json:"to_seq"`
	SeatType      string    `gorm:"size:16;not null" json:"seat_type"`
	PassengerName string    `gorm:"size:64;not null" json:"passenger_name"`
	PassengerID   string    `gorm:"size:32;not null" json:"passenger_id"`
	Status        string    `gorm:"size:20;not null;index" json:"status"`
	OrderID       *uint     `json:"order_id"`
	ExpireAt      time.Time `json:"expire_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SegmentQuota limits how many tickets of a seat type can be sold on one atomic segment.
// SegIndex i means the segment between stop i and i+1.
type SegmentQuota struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TrainID   uint      `gorm:"uniqueIndex:idx_quota;not null" json:"train_id"`
	SeatType  string    `gorm:"size:16;uniqueIndex:idx_quota;not null" json:"seat_type"`
	SegIndex  int       `gorm:"uniqueIndex:idx_quota;not null" json:"seg_index"`
	MaxSold   int       `gorm:"not null" json:"max_sold"`
	CreatedAt time.Time `json:"created_at"`
}

// SalePolicy controls when tickets for a travel date become available (分时/提前放票).
type SalePolicy struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:64;not null" json:"name"`
	AdvanceDays int       `gorm:"not null;default:15" json:"advance_days"` // 提前 N 天开售
	OpenHour    int       `gorm:"not null;default:8" json:"open_hour"`     // 当天开售整点
	OpenMinute  int       `gorm:"not null;default:0" json:"open_minute"`
	Enabled     bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

// SaleWave is a multi-wave release slice (按席别/区段阶梯放票).
// ReleasePct of matching waves that have opened are summed (capped at 100).
type SaleWave struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:64;not null" json:"name"`
	AdvanceDays int       `gorm:"not null;default:15" json:"advance_days"`
	OpenHour    int       `gorm:"not null;default:8" json:"open_hour"`
	OpenMinute  int       `gorm:"not null;default:0" json:"open_minute"`
	SeatType    string    `gorm:"size:16;index" json:"seat_type"` // 空=全部席别
	TrainID     uint      `gorm:"index;default:0" json:"train_id"` // 0=全部车次
	SegIndex    *int      `json:"seg_index"`                       // nil=全部区段；否则原子区段
	ReleasePct  int       `gorm:"not null;default:100" json:"release_pct"`
	Enabled     bool      `gorm:"not null;default:true;index" json:"enabled"`
	SortOrder   int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
}

// SeatDayBlock blocks a seat on a specific travel date (按日封锁).
type SeatDayBlock struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TrainID    uint      `gorm:"index;not null" json:"train_id"`
	SeatID     uint      `gorm:"uniqueIndex:idx_day_block;not null" json:"seat_id"`
	TravelDate string    `gorm:"size:10;uniqueIndex:idx_day_block;not null" json:"travel_date"`
	Reason     string    `gorm:"size:128" json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}

// RiskBlock stores blocked users/IPs.
type RiskBlock struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Kind      string     `gorm:"size:16;not null;index" json:"kind"` // user/ip
	Value     string     `gorm:"size:64;uniqueIndex;not null" json:"value"`
	Reason    string     `gorm:"size:128" json:"reason"`
	ExpireAt  *time.Time `json:"expire_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// RemainProjection is a CQRS read-model of remaining seats per OD × seat type.
type RemainProjection struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TrainID    uint      `gorm:"uniqueIndex:idx_proj;not null" json:"train_id"`
	TravelDate string    `gorm:"size:10;uniqueIndex:idx_proj;not null" json:"travel_date"`
	FromSeq    int       `gorm:"uniqueIndex:idx_proj;not null" json:"from_seq"`
	ToSeq      int       `gorm:"uniqueIndex:idx_proj;not null" json:"to_seq"`
	SeatType   string    `gorm:"size:16;uniqueIndex:idx_proj;not null" json:"seat_type"`
	Remaining  int       `gorm:"not null;default:0" json:"remaining"`
	UpdatedAt  time.Time `json:"updated_at"`
}
