package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServerPort       string
	DSN              string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	JWTSecret        string
	JWTExpireHours   int
	OrderHoldMinutes int
	CORSOrigins      string
	BookRatePerSec   int
	BookQueueWaitSec int
	InventoryURL     string
	RiskOrderPerMin  int
	RiskQueryPerMin  int
	PaymentProvider  string
	BookRateMin      int
	BookRateMax      int
	RescheduleMax    int
	RescheduleHours  int
}

func Load() *Config {
	return &Config{
		ServerPort:       getEnv("SERVER_PORT", "8080"),
		DSN:              getEnv("DATABASE_DSN", "host=localhost user=postgres password=postgres dbname=ticket port=5432 sslmode=disable TimeZone=Asia/Shanghai"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:          getEnvInt("REDIS_DB", 0),
		JWTSecret:        getEnv("JWT_SECRET", "12306cn-dev-secret-change-me"),
		JWTExpireHours:   getEnvInt("JWT_EXPIRE_HOURS", 72),
		OrderHoldMinutes: getEnvInt("ORDER_HOLD_MINUTES", 15),
		CORSOrigins:      getEnv("CORS_ORIGINS", "*"),
		BookRatePerSec:   getEnvInt("BOOK_RATE_PER_SEC", 5),
		BookQueueWaitSec: getEnvInt("BOOK_QUEUE_WAIT_SEC", 30),
		InventoryURL:     getEnv("INVENTORY_URL", ""),
		RiskOrderPerMin:  getEnvInt("RISK_ORDER_PER_MIN", 8),
		RiskQueryPerMin:  getEnvInt("RISK_QUERY_PER_MIN", 60),
		PaymentProvider:  getEnv("PAYMENT_PROVIDER", "mock"),
		BookRateMin:      getEnvInt("BOOK_RATE_MIN", 1),
		BookRateMax:      getEnvInt("BOOK_RATE_MAX", 20),
		RescheduleMax:    getEnvInt("RESCHEDULE_MAX_TIMES", 2),
		RescheduleHours:  getEnvInt("RESCHEDULE_HOURS_BEFORE", 2),
	}
}

func (c *Config) JWTExpire() time.Duration {
	return time.Duration(c.JWTExpireHours) * time.Hour
}

func (c *Config) OrderHold() time.Duration {
	return time.Duration(c.OrderHoldMinutes) * time.Minute
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
