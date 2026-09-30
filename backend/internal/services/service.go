package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kelvins-io/12306cn/backend/internal/config"
	"github.com/kelvins-io/12306cn/backend/internal/models"
	jwtutil "github.com/kelvins-io/12306cn/backend/internal/pkg/jwt"
	"github.com/kelvins-io/12306cn/backend/internal/services/captcha"
	"github.com/kelvins-io/12306cn/backend/internal/services/inventory"
	"github.com/kelvins-io/12306cn/backend/internal/services/invclient"
	"github.com/kelvins-io/12306cn/backend/internal/services/payment"
	"github.com/kelvins-io/12306cn/backend/internal/services/pricing"
	"github.com/kelvins-io/12306cn/backend/internal/services/query"
	"github.com/kelvins-io/12306cn/backend/internal/services/queueing"
	"github.com/kelvins-io/12306cn/backend/internal/services/reschedule"
	"github.com/kelvins-io/12306cn/backend/internal/services/sale"
	"github.com/kelvins-io/12306cn/backend/internal/services/transfer"
	"github.com/kelvins-io/12306cn/backend/internal/services/waitmq"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Service struct {
	DB       *gorm.DB
	RDB      *redis.Client
	Config   *config.Config
	Inv      *invclient.Client
	Query    *query.Service
	Queue    *queueing.BookQueue
	Pay      payment.Provider
	WaitMQ   *waitmq.Queue
	Captcha  *captcha.Service
	LocalInv *inventory.Service
}

func New(db *gorm.DB, rdb *redis.Client, cfg *config.Config) *Service {
	localInv := inventory.NewService(db, rdb)
	inv := invclient.New(cfg.InventoryURL, localInv)
	pay, err := payment.New(cfg.PaymentProvider)
	if err != nil {
		pay = payment.MockProvider{}
	}
	return &Service{
		DB:       db,
		RDB:      rdb,
		Config:   cfg,
		Inv:      inv,
		Query:    query.New(db, rdb, inv),
		Queue:    queueing.New(rdb, cfg.BookRatePerSec, time.Duration(cfg.BookQueueWaitSec)*time.Second),
		Pay:      pay,
		WaitMQ:   waitmq.New(rdb),
		Captcha:  captcha.New(rdb),
		LocalInv: localInv,
	}
}

func (s *Service) Register(username, password, nickname, phone string) (*models.User, string, error) {
	if len(username) < 3 || len(password) < 6 {
		return nil, "", errors.New("用户名至少3位，密码至少6位")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}
	user := &models.User{Username: username, PasswordHash: string(hash), Nickname: nickname, Phone: phone, Role: "user"}
	if user.Nickname == "" {
		user.Nickname = username
	}
	if err := s.DB.Create(user).Error; err != nil {
		return nil, "", errors.New("用户名已存在")
	}
	token, err := jwtutil.Generate(s.Config.JWTSecret, user.ID, user.Username, user.Role, s.Config.JWTExpire())
	return user, token, err
}

func (s *Service) Login(username, password string) (*models.User, string, error) {
	var user models.User
	if err := s.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, "", errors.New("用户名或密码错误")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", errors.New("用户名或密码错误")
	}
	if user.Role == "" {
		user.Role = "user"
	}
	token, err := jwtutil.Generate(s.Config.JWTSecret, user.ID, user.Username, user.Role, s.Config.JWTExpire())
	return &user, token, err
}

func (s *Service) ListStations(keyword string) ([]models.Station, error) {
	return s.Query.ListStations(keyword)
}

type TicketQueryItem = query.TicketItem

func (s *Service) QueryTickets(ctx context.Context, from, to, date string) ([]TicketQueryItem, error) {
	return s.Query.QueryTickets(ctx, from, to, date)
}

type CreateOrderReq struct {
	TrainID         uint                     `json:"train_id" binding:"required"`
	TravelDate      string                   `json:"travel_date" binding:"required"`
	FromStation     string                   `json:"from_station" binding:"required"`
	ToStation       string                   `json:"to_station" binding:"required"`
	SeatType        string                   `json:"seat_type" binding:"required"`
	PassengerIDs    []uint                   `json:"passenger_ids"`
	Passengers      []PassengerSnapshot      `json:"passengers"`
	Preference      inventory.SeatPreference `json:"preference"`
	CaptchaID       string                   `json:"captcha_id"`
	CaptchaCode     string                   `json:"captcha_code"`
	TransferGroupID string                   `json:"-"`
	TransferLeg     int                      `json:"-"`
}

type PassengerSnapshot struct {
	Name          string `json:"name"`
	IDNumber      string `json:"id_number"`
	PassengerType string `json:"passenger_type"`
}

type CreateOrderResult struct {
	*models.Order
	QueuePosition int64 `json:"queue_position"`
}

func (s *Service) resolvePassengers(userID uint, req CreateOrderReq) ([]PassengerSnapshot, error) {
	passengers := req.Passengers
	if len(req.PassengerIDs) > 0 {
		var list []models.Passenger
		if err := s.DB.Where("user_id = ? AND id IN ?", userID, req.PassengerIDs).Find(&list).Error; err != nil {
			return nil, err
		}
		if len(list) != len(req.PassengerIDs) {
			return nil, errors.New("乘车人无效")
		}
		passengers = passengers[:0]
		for _, p := range list {
			pt := p.PassengerType
			if pt == "" {
				pt = "成人"
			}
			passengers = append(passengers, PassengerSnapshot{Name: p.Name, IDNumber: p.IDNumber, PassengerType: pt})
		}
	}
	if len(passengers) == 0 {
		return nil, errors.New("请选择乘车人")
	}
	for i := range passengers {
		if passengers[i].PassengerType == "" {
			passengers[i].PassengerType = "成人"
		}
	}
	return passengers, nil
}

func (s *Service) CreateOrder(userID uint, req CreateOrderReq) (*CreateOrderResult, error) {
	if err := sale.CheckOpen(s.DB, req.TravelDate, time.Now()); err != nil {
		return nil, err
	}
	passengers, err := s.resolvePassengers(userID, req)
	if err != nil {
		return nil, err
	}
	reqID := uuid.NewString()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.Config.BookQueueWaitSec)*time.Second+5*time.Second)
	defer cancel()
	pos, err := s.Queue.WaitTurn(ctx, req.TrainID, req.TravelDate, reqID)
	if err != nil {
		return nil, err
	}
	order, err := s.createOrderLocked(userID, req, passengers, string(models.OrderPendingPay), true, nil, 0, 0)
	if err != nil {
		return nil, err
	}
	return &CreateOrderResult{Order: order, QueuePosition: pos}, nil
}

func (s *Service) createOrderLocked(userID uint, req CreateOrderReq, passengers []PassengerSnapshot, status string, setExpire bool, parentID *uint, rescheduleCount, priceDiff int) (*models.Order, error) {
	var fromStop, toStop models.TrainStop
	if err := s.DB.Where("train_id = ? AND station_name = ?", req.TrainID, req.FromStation).First(&fromStop).Error; err != nil {
		return nil, errors.New("出发站不在该车次")
	}
	if err := s.DB.Where("train_id = ? AND station_name = ?", req.TrainID, req.ToStation).First(&toStop).Error; err != nil {
		return nil, errors.New("到达站不在该车次")
	}
	if toStop.Seq <= fromStop.Seq {
		return nil, errors.New("到站须在出发站之后")
	}
	var train models.Train
	if err := s.DB.First(&train, req.TrainID).Error; err != nil {
		return nil, errors.New("车次不存在")
	}

	ctx := context.Background()
	picks, err := s.Inv.Occupy(ctx, inventory.OccupyRequest{
		TrainID: req.TrainID, TravelDate: req.TravelDate, SeatType: req.SeatType,
		FromSeq: fromStop.Seq, ToSeq: toStop.Seq, Count: len(passengers),
		Preference: req.Preference, FromStation: req.FromStation, ToStation: req.ToStation,
	})
	if err != nil {
		return nil, err
	}

	total := 0
	prices := make([]int, len(passengers))
	for i, p := range passengers {
		prices[i] = pricing.PriceFor(req.SeatType, train.TrainType, p.PassengerType, fromStop.Seq, toStop.Seq)
		total += prices[i]
	}

	order := &models.Order{
		OrderNo: genNo("O"), UserID: userID, TrainID: train.ID, TrainNo: train.TrainNo,
		TravelDate: req.TravelDate, FromStation: req.FromStation, ToStation: req.ToStation,
		FromSeq: fromStop.Seq, ToSeq: toStop.Seq, SeatType: req.SeatType,
		Status: status, TotalAmount: total, ParentOrderID: parentID,
		RescheduleCount: rescheduleCount, PriceDiff: priceDiff,
		TransferGroupID: req.TransferGroupID, TransferLeg: req.TransferLeg,
	}
	if setExpire {
		exp := time.Now().Add(s.Config.OrderHold())
		order.ExpireAt = &exp
	} else {
		now := time.Now()
		order.PaidAt = &now
	}

	ticketStatus := "held"
	if status == string(models.OrderPaid) {
		ticketStatus = "issued"
	}

	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		for i, pick := range picks {
			ticket := models.Ticket{
				OrderID: order.ID, TicketNo: genNo("T"),
				PassengerName: passengers[i].Name, PassengerID: passengers[i].IDNumber,
				PassengerType: passengers[i].PassengerType,
				SeatID: pick.SeatID, CarriageNo: pick.CarriageNo, SeatNo: pick.SeatNo,
				SeatType: pick.SeatType, Price: prices[i], Status: ticketStatus,
			}
			if err := tx.Create(&ticket).Error; err != nil {
				return err
			}
			order.Tickets = append(order.Tickets, ticket)
		}
		return nil
	})
	if err != nil {
		// compensate inventory
		for _, pick := range picks {
			_ = s.Inv.Release(ctx, inventory.ReleaseRequest{
				TrainID: req.TrainID, TravelDate: req.TravelDate, SeatID: pick.SeatID,
				FromSeq: fromStop.Seq, ToSeq: toStop.Seq,
				FromStation: req.FromStation, ToStation: req.ToStation,
			})
		}
		return nil, err
	}
	return order, nil
}

type RescheduleReq struct {
	TrainID     uint                     `json:"train_id" binding:"required"`
	TravelDate  string                   `json:"travel_date" binding:"required"`
	FromStation string                   `json:"from_station" binding:"required"`
	ToStation   string                   `json:"to_station" binding:"required"`
	SeatType    string                   `json:"seat_type" binding:"required"`
	Preference  inventory.SeatPreference `json:"preference"`
}

func (s *Service) RescheduleOrder(userID, orderID uint, req RescheduleReq) (*models.Order, error) {
	if err := sale.CheckOpen(s.DB, req.TravelDate, time.Now()); err != nil {
		return nil, err
	}
	var old models.Order
	if err := s.DB.Preload("Tickets").Where("id = ? AND user_id = ?", orderID, userID).First(&old).Error; err != nil {
		return nil, errors.New("订单不存在")
	}
	if old.Status != string(models.OrderPaid) {
		return nil, errors.New("仅已出票订单可改签")
	}
	if len(old.Tickets) == 0 {
		return nil, errors.New("订单无有效车票")
	}
	var oldFrom models.TrainStop
	if err := s.DB.Where("train_id = ? AND station_name = ?", old.TrainID, old.FromStation).First(&oldFrom).Error; err != nil {
		return nil, errors.New("原出发站无效")
	}
	cfg := reschedule.Config{MaxTimes: s.Config.RescheduleMax, HoursBefore: s.Config.RescheduleHours}
	if err := reschedule.CheckAllowed(s.DB, old, oldFrom.DepartTime, old.TravelDate, time.Now(), cfg); err != nil {
		return nil, err
	}

	passengers := make([]PassengerSnapshot, 0, len(old.Tickets))
	for _, t := range old.Tickets {
		pt := t.PassengerType
		if pt == "" {
			pt = "成人"
		}
		passengers = append(passengers, PassengerSnapshot{Name: t.PassengerName, IDNumber: t.PassengerID, PassengerType: pt})
	}

	// preview new total for fare diff
	var newFrom, newTo models.TrainStop
	if err := s.DB.Where("train_id = ? AND station_name = ?", req.TrainID, req.FromStation).First(&newFrom).Error; err != nil {
		return nil, errors.New("新出发站无效")
	}
	if err := s.DB.Where("train_id = ? AND station_name = ?", req.TrainID, req.ToStation).First(&newTo).Error; err != nil {
		return nil, errors.New("新到达站无效")
	}
	var newTrain models.Train
	if err := s.DB.First(&newTrain, req.TrainID).Error; err != nil {
		return nil, errors.New("车次不存在")
	}
	newTotal := 0
	for _, p := range passengers {
		newTotal += pricing.PriceFor(req.SeatType, newTrain.TrainType, p.PassengerType, newFrom.Seq, newTo.Seq)
	}
	diff := reschedule.DiffAmount(old.TotalAmount, newTotal) // >0 need pay more
	nextCount := reschedule.CountChain(s.DB, old) + 1

	reqID := uuid.NewString()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.Config.BookQueueWaitSec)*time.Second+5*time.Second)
	defer cancel()
	if _, err := s.Queue.WaitTurn(ctx, req.TrainID, req.TravelDate, reqID); err != nil {
		return nil, err
	}

	bg := context.Background()
	for _, t := range old.Tickets {
		if err := s.Inv.Release(bg, inventory.ReleaseRequest{
			TrainID: old.TrainID, TravelDate: old.TravelDate, SeatID: t.SeatID,
			FromSeq: old.FromSeq, ToSeq: old.ToSeq,
			FromStation: old.FromStation, ToStation: old.ToStation,
		}); err != nil {
			return nil, err
		}
		_ = s.DB.Model(&t).Update("status", "refunded").Error
	}
	_ = s.DB.Model(&old).Update("status", string(models.OrderRescheduled)).Error

	parentID := old.ID
	status := string(models.OrderPaid)
	setExpire := false
	chargeAmount := newTotal
	priceDiff := -diff // store as >0 refund to user when high->low
	if diff > 0 {
		// 低改高：新单待支付补差价（TotalAmount = 差价）
		status = string(models.OrderPendingPay)
		setExpire = true
		chargeAmount = diff
		priceDiff = -diff
	} else {
		// 高改低或平价：直接出票，PriceDiff 为正表示应退
		priceDiff = -diff
	}

	newOrder, err := s.createOrderLocked(userID, CreateOrderReq{
		TrainID: req.TrainID, TravelDate: req.TravelDate,
		FromStation: req.FromStation, ToStation: req.ToStation,
		SeatType: req.SeatType, Preference: req.Preference,
	}, passengers, status, setExpire, &parentID, nextCount, priceDiff)
	if err != nil {
		return nil, fmt.Errorf("原票已释放，新票占座失败: %w", err)
	}
	// override total for补差价 case: tickets keep full new prices but order.TotalAmount is charge
	if diff > 0 {
		_ = s.DB.Model(newOrder).Update("total_amount", chargeAmount).Error
		newOrder.TotalAmount = chargeAmount
	} else if diff < 0 {
		_ = s.DB.Model(newOrder).Update("total_amount", newTotal).Error
		newOrder.TotalAmount = newTotal
	}
	return newOrder, nil
}

func (s *Service) PayOrder(userID, orderID uint) (*models.Order, error) {
	var order models.Order
	if err := s.DB.Preload("Tickets").Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
		return nil, errors.New("订单不存在")
	}
	if order.Status != string(models.OrderPendingPay) {
		return nil, errors.New("订单状态不可支付")
	}
	if order.ExpireAt != nil && time.Now().After(*order.ExpireAt) {
		_ = s.expireOrder(&order)
		return nil, errors.New("订单已过期")
	}

	payRec := models.Payment{
		PaymentNo: genNo("P"), OrderID: order.ID, UserID: userID,
		Provider: s.Pay.Name(), Amount: order.TotalAmount, Status: "pending",
	}
	if err := s.DB.Create(&payRec).Error; err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := s.Pay.Charge(ctx, payment.ChargeRequest{
		PaymentNo: payRec.PaymentNo, OrderID: order.ID, UserID: userID, Amount: order.TotalAmount,
	})
	if err != nil {
		_ = s.DB.Model(&payRec).Updates(map[string]interface{}{"status": "failed", "fail_reason": err.Error()}).Error
		return nil, errors.New("支付通道异常")
	}
	if !result.Success {
		_ = s.DB.Model(&payRec).Updates(map[string]interface{}{"status": "failed", "fail_reason": result.FailReason}).Error
		return nil, errors.New(result.FailReason)
	}

	now := time.Now()
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&payRec).Updates(map[string]interface{}{
			"status": "success", "provider_ref": result.ProviderRef, "paid_at": now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status": string(models.OrderPaid), "paid_at": now,
		}).Error; err != nil {
			return err
		}
		for i := range order.Tickets {
			code := fmt.Sprintf("%06d", (int(order.Tickets[i].ID)*7919+int(userID)*13+i*97)%1000000)
			if code == "000000" {
				code = "100001"
			}
			if err := tx.Model(&order.Tickets[i]).Updates(map[string]interface{}{
				"status": "issued", "verify_code": code,
			}).Error; err != nil {
				return err
			}
			order.Tickets[i].Status = "issued"
			order.Tickets[i].VerifyCode = code
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	order.Status = string(models.OrderPaid)
	order.PaidAt = &now
	return &order, nil
}

func (s *Service) CancelOrder(userID, orderID uint) error {
	var order models.Order
	if err := s.DB.Preload("Tickets").Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
		return errors.New("订单不存在")
	}
	if order.Status != string(models.OrderPendingPay) {
		return errors.New("仅待支付订单可取消")
	}
	return s.releaseOrder(&order, string(models.OrderCancelled))
}

func (s *Service) RefundOrder(userID, orderID uint) error {
	var order models.Order
	if err := s.DB.Preload("Tickets").Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
		return errors.New("订单不存在")
	}
	if order.Status != string(models.OrderPaid) {
		return errors.New("仅已支付订单可退票")
	}
	if err := s.releaseOrder(&order, string(models.OrderRefunded)); err != nil {
		return err
	}
	_ = s.WaitMQ.Enqueue(context.Background(), waitmq.Job{
		TrainID: order.TrainID, TravelDate: order.TravelDate,
		FromStation: order.FromStation, ToStation: order.ToStation, SeatType: order.SeatType,
	})
	return nil
}

func (s *Service) StartWaitlistWorker(ctx context.Context) {
	s.WaitMQ.StartWorker(ctx, func(ctx context.Context, job waitmq.Job) error {
		s.tryFulfillWaitlist(job.TrainID, job.TravelDate, job.FromStation, job.ToStation, job.SeatType)
		return nil
	})
}

func (s *Service) VerifyTicket(operator string, ticketNo, verifyCode string) (*models.Ticket, error) {
	var t models.Ticket
	if err := s.DB.Where("ticket_no = ?", ticketNo).First(&t).Error; err != nil {
		return nil, errors.New("车票不存在")
	}
	if t.Status == "verified" {
		return nil, errors.New("车票已核验")
	}
	if t.Status != "issued" {
		return nil, errors.New("车票状态不可核验")
	}
	if t.VerifyCode == "" || t.VerifyCode != verifyCode {
		return nil, errors.New("取票码错误")
	}
	now := time.Now()
	if err := s.DB.Model(&t).Updates(map[string]interface{}{
		"status": "verified", "verified_at": now, "verified_by": operator,
	}).Error; err != nil {
		return nil, err
	}
	t.Status = "verified"
	t.VerifiedAt = &now
	t.VerifiedBy = operator
	return &t, nil
}

func (s *Service) LookupTicket(ticketNo string) (*models.Ticket, *models.Order, error) {
	var t models.Ticket
	if err := s.DB.Where("ticket_no = ?", ticketNo).First(&t).Error; err != nil {
		return nil, nil, errors.New("车票不存在")
	}
	var o models.Order
	_ = s.DB.First(&o, t.OrderID)
	return &t, &o, nil
}

func (s *Service) expireOrder(order *models.Order) error {
	if len(order.Tickets) == 0 {
		_ = s.DB.Preload("Tickets").First(order, order.ID)
	}
	return s.releaseOrder(order, string(models.OrderExpired))
}

func (s *Service) releaseOrder(order *models.Order, status string) error {
	ctx := context.Background()
	for _, t := range order.Tickets {
		if err := s.Inv.Release(ctx, inventory.ReleaseRequest{
			TrainID: order.TrainID, TravelDate: order.TravelDate, SeatID: t.SeatID,
			FromSeq: order.FromSeq, ToSeq: order.ToSeq,
			FromStation: order.FromStation, ToStation: order.ToStation,
		}); err != nil {
			return err
		}
		_ = s.DB.Model(&t).Update("status", "refunded").Error
	}
	return s.DB.Model(order).Update("status", status).Error
}

func (s *Service) ListOrders(userID uint) ([]models.Order, error) {
	var list []models.Order
	err := s.DB.Preload("Tickets").Where("user_id = ?", userID).Order("id desc").Find(&list).Error
	return list, err
}

func (s *Service) GetOrder(userID, orderID uint) (*models.Order, error) {
	var order models.Order
	if err := s.DB.Preload("Tickets").Where("id = ? AND user_id = ?", orderID, userID).First(&order).Error; err != nil {
		return nil, errors.New("订单不存在")
	}
	return &order, nil
}

func (s *Service) AddPassenger(userID uint, p *models.Passenger) error {
	p.UserID = userID
	return s.DB.Create(p).Error
}

func (s *Service) ListPassengers(userID uint) ([]models.Passenger, error) {
	var list []models.Passenger
	err := s.DB.Where("user_id = ?", userID).Order("id").Find(&list).Error
	return list, err
}

func (s *Service) DeletePassenger(userID, id uint) error {
	return s.DB.Where("user_id = ? AND id = ?", userID, id).Delete(&models.Passenger{}).Error
}

type CreateWaitlistReq struct {
	TrainID       uint   `json:"train_id" binding:"required"`
	TravelDate    string `json:"travel_date" binding:"required"`
	FromStation   string `json:"from_station" binding:"required"`
	ToStation     string `json:"to_station" binding:"required"`
	SeatType      string `json:"seat_type" binding:"required"`
	PassengerName string `json:"passenger_name" binding:"required"`
	PassengerID   string `json:"passenger_id" binding:"required"`
}

func (s *Service) CreateWaitlist(userID uint, req CreateWaitlistReq) (*models.Waitlist, error) {
	if err := sale.CheckOpen(s.DB, req.TravelDate, time.Now()); err != nil {
		return nil, err
	}
	var fromStop, toStop models.TrainStop
	if err := s.DB.Where("train_id = ? AND station_name = ?", req.TrainID, req.FromStation).First(&fromStop).Error; err != nil {
		return nil, errors.New("出发站无效")
	}
	if err := s.DB.Where("train_id = ? AND station_name = ?", req.TrainID, req.ToStation).First(&toStop).Error; err != nil {
		return nil, errors.New("到达站无效")
	}
	var train models.Train
	if err := s.DB.First(&train, req.TrainID).Error; err != nil {
		return nil, errors.New("车次不存在")
	}
	w := &models.Waitlist{
		UserID: userID, TrainID: train.ID, TrainNo: train.TrainNo, TravelDate: req.TravelDate,
		FromStation: req.FromStation, ToStation: req.ToStation, FromSeq: fromStop.Seq, ToSeq: toStop.Seq,
		SeatType: req.SeatType, PassengerName: req.PassengerName, PassengerID: req.PassengerID,
		Status: string(models.WaitlistPending), ExpireAt: time.Now().Add(48 * time.Hour),
	}
	if err := s.DB.Create(w).Error; err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Service) ListWaitlist(userID uint) ([]models.Waitlist, error) {
	var list []models.Waitlist
	err := s.DB.Where("user_id = ?", userID).Order("id desc").Find(&list).Error
	return list, err
}

func (s *Service) CancelWaitlist(userID, id uint) error {
	res := s.DB.Model(&models.Waitlist{}).
		Where("id = ? AND user_id = ? AND status = ?", id, userID, string(models.WaitlistPending)).
		Update("status", string(models.WaitlistCancelled))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("候补记录不存在或不可取消")
	}
	return nil
}

func (s *Service) tryFulfillWaitlist(trainID uint, date, from, to, seatType string) {
	var list []models.Waitlist
	s.DB.Where("train_id = ? AND travel_date = ? AND from_station = ? AND to_station = ? AND seat_type = ? AND status = ?",
		trainID, date, from, to, seatType, string(models.WaitlistPending)).
		Order("id asc").Limit(5).Find(&list)
	for _, w := range list {
		res, err := s.CreateOrder(w.UserID, CreateOrderReq{
			TrainID: w.TrainID, TravelDate: w.TravelDate, FromStation: w.FromStation, ToStation: w.ToStation,
			SeatType: w.SeatType, Passengers: []PassengerSnapshot{{Name: w.PassengerName, IDNumber: w.PassengerID, PassengerType: "成人"}},
		})
		if err != nil {
			continue
		}
		paid, err := s.PayOrder(w.UserID, res.ID)
		if err != nil {
			_ = s.CancelOrder(w.UserID, res.ID)
			continue
		}
		_ = s.DB.Model(&w).Updates(map[string]interface{}{"status": string(models.WaitlistFulfilled), "order_id": paid.ID}).Error
		break
	}
}

func (s *Service) SweepExpiredOrders() {
	var orders []models.Order
	now := time.Now()
	s.DB.Preload("Tickets").Where("status = ? AND expire_at < ?", string(models.OrderPendingPay), now).Find(&orders)
	for i := range orders {
		_ = s.expireOrder(&orders[i])
	}
}

func (s *Service) SearchTransfers(from, to, date string) ([]transfer.Plan, error) {
	return transfer.Search(s.DB, from, to, date, 20)
}

type CreateTransferReq struct {
	TravelDate   string `json:"travel_date" binding:"required"`
	FromStation  string `json:"from_station" binding:"required"`
	ToStation    string `json:"to_station" binding:"required"`
	HubStation   string `json:"hub_station" binding:"required"`
	Leg1TrainID  uint   `json:"leg1_train_id" binding:"required"`
	Leg1SeatType string `json:"leg1_seat_type" binding:"required"`
	Leg2TrainID  uint   `json:"leg2_train_id" binding:"required"`
	Leg2SeatType string `json:"leg2_seat_type" binding:"required"`
	PassengerIDs []uint `json:"passenger_ids" binding:"required"`
	CaptchaID    string `json:"captcha_id"`
	CaptchaCode  string `json:"captcha_code"`
}

type TransferOrderResult struct {
	GroupID string                 `json:"group_id"`
	Orders  []*CreateOrderResult   `json:"orders"`
	Total   int                    `json:"total_amount"`
}

func (s *Service) CreateTransferOrders(userID uint, req CreateTransferReq) (*TransferOrderResult, error) {
	if err := sale.CheckOpen(s.DB, req.TravelDate, time.Now()); err != nil {
		return nil, err
	}
	gid := uuid.NewString()
	leg1, err := s.CreateOrder(userID, CreateOrderReq{
		TrainID: req.Leg1TrainID, TravelDate: req.TravelDate,
		FromStation: req.FromStation, ToStation: req.HubStation,
		SeatType: req.Leg1SeatType, PassengerIDs: req.PassengerIDs,
		TransferGroupID: gid, TransferLeg: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("第一程占座失败: %w", err)
	}
	leg2, err := s.CreateOrder(userID, CreateOrderReq{
		TrainID: req.Leg2TrainID, TravelDate: req.TravelDate,
		FromStation: req.HubStation, ToStation: req.ToStation,
		SeatType: req.Leg2SeatType, PassengerIDs: req.PassengerIDs,
		TransferGroupID: gid, TransferLeg: 2,
	})
	if err != nil {
		_ = s.CancelOrder(userID, leg1.ID)
		return nil, fmt.Errorf("第二程占座失败（已取消第一程）: %w", err)
	}
	return &TransferOrderResult{
		GroupID: gid,
		Orders:  []*CreateOrderResult{leg1, leg2},
		Total:   leg1.TotalAmount + leg2.TotalAmount,
	}, nil
}

func genNo(prefix string) string {
	return fmt.Sprintf("%s%s", prefix, uuid.NewString()[:12])
}
