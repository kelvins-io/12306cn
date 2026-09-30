package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kelvins-io/12306cn/backend/internal/config"
	riskmw "github.com/kelvins-io/12306cn/backend/internal/middleware/risk"
	"github.com/kelvins-io/12306cn/backend/internal/middleware"
	"github.com/kelvins-io/12306cn/backend/internal/pkg/response"
	"github.com/kelvins-io/12306cn/backend/internal/services/inventory"
	"github.com/kelvins-io/12306cn/backend/internal/services/invclient"
	"github.com/kelvins-io/12306cn/backend/internal/services/query"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	cfg := config.Load()
	port := cfg.ServerPort
	if v := os.Getenv("QUERY_PORT"); v != "" {
		port = v
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(30)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB})
	localInv := inventory.NewService(db, rdb)
	inv := invclient.New(cfg.InventoryURL, localInv)
	qs := query.New(db, rdb, inv)
	guard := riskmw.New(db, rdb, cfg.RiskOrderPerMin, cfg.RiskQueryPerMin)

	r := gin.Default()
	r.Use(middleware.CORS(cfg.CORSOrigins))
	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			response.OK(c, gin.H{"status": "up", "service": "query"})
		})
		api.GET("/stations", func(c *gin.Context) {
			list, err := qs.ListStations(c.Query("q"))
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.OK(c, list)
		})
		api.GET("/tickets", guard.QueryLimit(), func(c *gin.Context) {
			from, to, date := c.Query("from"), c.Query("to"), c.Query("date")
			if from == "" || to == "" || date == "" {
				response.BadRequest(c, "from/to/date 必填")
				return
			}
			list, err := qs.QueryTickets(c.Request.Context(), from, to, date)
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.OK(c, list)
		})
	}

	log.Printf("12306cn query service listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
