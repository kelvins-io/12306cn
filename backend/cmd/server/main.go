package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kelvins-io/12306cn/backend/internal/config"
	"github.com/kelvins-io/12306cn/backend/internal/handlers"
	"github.com/kelvins-io/12306cn/backend/internal/middleware"
	riskmw "github.com/kelvins-io/12306cn/backend/internal/middleware/risk"
	"github.com/kelvins-io/12306cn/backend/internal/models"
	"github.com/kelvins-io/12306cn/backend/internal/seed"
	"github.com/kelvins-io/12306cn/backend/internal/services"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	cfg := config.Load()

	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := db.AutoMigrate(
		&models.User{}, &models.Passenger{}, &models.Station{}, &models.Train{}, &models.TrainStop{},
		&models.Seat{}, &models.SeatInventory{}, &models.SegmentQuota{}, &models.SalePolicy{}, &models.SaleWave{},
		&models.SeatDayBlock{}, &models.RiskBlock{},
		&models.Order{}, &models.Ticket{}, &models.Waitlist{}, &models.Payment{}, &models.RemainProjection{},
	); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if err := seed.Run(db); err != nil {
		log.Fatalf("seed: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB})
	svc := services.New(db, rdb, cfg)
	h := handlers.New(svc)
	guard := riskmw.New(db, rdb, cfg.RiskOrderPerMin, cfg.RiskQueryPerMin)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.StartWaitlistWorker(ctx)

	go func() {
		t := time.NewTicker(30 * time.Second)
		for range t.C {
			svc.SweepExpiredOrders()
		}
	}()

	r := gin.Default()
	r.Use(middleware.CORS(cfg.CORSOrigins))

	api := r.Group("/api")
	{
		api.GET("/health", h.Health)
		api.GET("/captcha", h.IssueCaptcha)
		api.POST("/auth/register", h.Register)
		api.POST("/auth/login", h.Login)
		api.GET("/stations", h.ListStations)
		api.GET("/tickets", guard.QueryLimit(), h.QueryTickets)
		api.GET("/transfers", guard.QueryLimit(), h.QueryTransfers)

		auth := api.Group("")
		auth.Use(middleware.JWTAuth(cfg.JWTSecret))
		{
			auth.GET("/me", h.Me)
			auth.GET("/passengers", h.ListPassengers)
			auth.POST("/passengers", h.AddPassenger)
			auth.DELETE("/passengers/:id", h.DeletePassenger)

			auth.POST("/orders", guard.OrderLimit(), h.CreateOrder)
			auth.POST("/orders/transfer", guard.OrderLimit(), h.CreateTransferOrders)
			auth.GET("/orders", h.ListOrders)
			auth.GET("/orders/:id", h.GetOrder)
			auth.POST("/orders/:id/pay", h.PayOrder)
			auth.POST("/orders/:id/cancel", h.CancelOrder)
			auth.POST("/orders/:id/refund", h.RefundOrder)
			auth.POST("/orders/:id/reschedule", guard.OrderLimit(), h.RescheduleOrder)

			auth.POST("/waitlist", guard.OrderLimit(), h.CreateWaitlist)
			auth.GET("/waitlist", h.ListWaitlist)
			auth.POST("/waitlist/:id/cancel", h.CancelWaitlist)

			station := auth.Group("")
			station.Use(middleware.RequireRoles("admin", "station"))
			{
				station.GET("/verify/ticket", h.LookupTicket)
				station.POST("/verify/ticket", h.VerifyTicket)
			}

			admin := auth.Group("/admin")
			admin.Use(middleware.RequireRoles("admin"))
			{
				admin.GET("/trains", h.AdminListTrains)
				admin.GET("/seats", h.AdminListSeats)
				admin.POST("/seats/:id/block", h.AdminBlockSeat)
				admin.POST("/seats/:id/unblock", h.AdminUnblockSeat)
				admin.GET("/day-blocks", h.AdminListDayBlocks)
				admin.POST("/day-blocks", h.AdminUpsertDayBlock)
				admin.DELETE("/day-blocks/:id", h.AdminDeleteDayBlock)
				admin.POST("/projections/rebuild", h.AdminRebuildProjection)
				admin.GET("/quotas", h.AdminListQuotas)
				admin.POST("/quotas", h.AdminUpsertQuota)
				admin.DELETE("/quotas/:id", h.AdminDeleteQuota)
				admin.GET("/sale-policies", h.AdminListSalePolicies)
				admin.POST("/sale-policies", h.AdminUpsertSalePolicy)
				admin.GET("/sale-waves", h.AdminListSaleWaves)
				admin.POST("/sale-waves", h.AdminUpsertSaleWave)
				admin.DELETE("/sale-waves/:id", h.AdminDeleteSaleWave)
				admin.GET("/risk-blocks", h.AdminListRiskBlocks)
				admin.POST("/risk-blocks", h.AdminUpsertRiskBlock)
				admin.DELETE("/risk-blocks/:id", h.AdminDeleteRiskBlock)
			}
		}
	}

	log.Printf("12306cn api listening on :%s (inventory=%s pay=%s)", cfg.ServerPort, cfg.InventoryURL, cfg.PaymentProvider)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatal(err)
	}
}
