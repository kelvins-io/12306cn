package risk

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kelvins-io/12306cn/backend/internal/middleware"
	"github.com/kelvins-io/12306cn/backend/internal/models"
	"github.com/kelvins-io/12306cn/backend/internal/pkg/response"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Guard struct {
	DB             *gorm.DB
	RDB            *redis.Client
	OrderPerMinute int
	QueryPerMinute int
}

func New(db *gorm.DB, rdb *redis.Client, orderPM, queryPM int) *Guard {
	if orderPM <= 0 {
		orderPM = 8
	}
	if queryPM <= 0 {
		queryPM = 60
	}
	return &Guard{DB: db, RDB: rdb, OrderPerMinute: orderPM, QueryPerMinute: queryPM}
}

func (g *Guard) blocked(kind, value string) bool {
	var b models.RiskBlock
	err := g.DB.Where("kind = ? AND value = ?", kind, value).First(&b).Error
	if err != nil {
		return false
	}
	if b.ExpireAt != nil && time.Now().After(*b.ExpireAt) {
		return false
	}
	return true
}

func (g *Guard) hitLimit(ctx context.Context, key string, limit int) bool {
	if g.RDB == nil {
		return false
	}
	n, err := g.RDB.Incr(ctx, key).Result()
	if err != nil {
		return false
	}
	if n == 1 {
		_ = g.RDB.Expire(ctx, key, time.Minute).Err()
	}
	return int(n) > limit
}

func (g *Guard) OrderLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if g.blocked("ip", ip) {
			response.Forbidden(c, "IP 已被风控限制")
			c.Abort()
			return
		}
		uid := middleware.GetUserID(c)
		if uid > 0 && g.blocked("user", fmt.Sprintf("%d", uid)) {
			response.Forbidden(c, "账号已被风控限制")
			c.Abort()
			return
		}
		ctx := c.Request.Context()
		if uid > 0 && g.hitLimit(ctx, fmt.Sprintf("risk:order:u:%d", uid), g.OrderPerMinute) {
			response.Fail(c, 429, 429, "下单过于频繁，请稍后再试")
			c.Abort()
			return
		}
		if g.hitLimit(ctx, fmt.Sprintf("risk:order:ip:%s", ip), g.OrderPerMinute*2) {
			response.Fail(c, 429, 429, "当前网络下单过于频繁")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (g *Guard) QueryLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if g.blocked("ip", ip) {
			response.Forbidden(c, "IP 已被风控限制")
			c.Abort()
			return
		}
		if g.hitLimit(c.Request.Context(), fmt.Sprintf("risk:query:ip:%s", ip), g.QueryPerMinute) {
			response.Fail(c, 429, 429, "查询过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}
