package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kelvins-io/12306cn/backend/internal/config"
	"github.com/kelvins-io/12306cn/backend/internal/middleware"
	"github.com/kelvins-io/12306cn/backend/internal/pkg/response"
	"github.com/kelvins-io/12306cn/backend/internal/services/inventory"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	cfg := config.Load()
	port := getenv("INVENTORY_PORT", "8082")

	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(40)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB})
	svc := inventory.NewService(db, rdb)

	r := gin.Default()
	r.Use(middleware.CORS(cfg.CORSOrigins))
	api := r.Group("/internal/v1")
	{
		api.GET("/health", func(c *gin.Context) {
			response.OK(c, gin.H{"status": "up", "service": "inventory"})
		})
		api.POST("/ensure", func(c *gin.Context) {
			var req struct {
				TrainID    uint   `json:"train_id"`
				TravelDate string `json:"travel_date"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "参数错误")
				return
			}
			if err := svc.EnsureDaily(req.TrainID, req.TravelDate); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.OK(c, gin.H{"ok": true})
		})
		api.POST("/occupy", func(c *gin.Context) {
			var req inventory.OccupyRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "参数错误")
				return
			}
			picks, err := svc.OccupyAndPublish(c.Request.Context(), req)
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.OK(c, picks)
		})
		api.POST("/release", func(c *gin.Context) {
			var req inventory.ReleaseRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				response.BadRequest(c, "参数错误")
				return
			}
			if err := svc.ReleaseAndPublish(c.Request.Context(), req); err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.OK(c, gin.H{"ok": true})
		})
		api.GET("/remaining", func(c *gin.Context) {
			trainID, _ := strconv.ParseUint(c.Query("train_id"), 10, 64)
			fromSeq, _ := strconv.Atoi(c.Query("from_seq"))
			toSeq, _ := strconv.Atoi(c.Query("to_seq"))
			n, err := svc.RemainingByType(uint(trainID), c.Query("travel_date"), c.Query("seat_type"), fromSeq, toSeq)
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.OK(c, gin.H{"remaining": n})
		})
	}

	log.Printf("12306cn inventory service listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
