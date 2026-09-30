package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kelvins-io/12306cn/backend/internal/config"
	"github.com/kelvins-io/12306cn/backend/internal/middleware"
	"github.com/kelvins-io/12306cn/backend/internal/pkg/response"
	"github.com/kelvins-io/12306cn/backend/internal/services/adaptive"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.Load()
	port := getenv("GATEWAY_PORT", "8090")
	backendURL := getenv("BACKEND_URL", "http://backend:8080")
	queryURL := getenv("QUERY_URL", "http://query:8081")

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB})
	ctrl := adaptive.New(rdb, int64(cfg.BookRateMin), int64(cfg.BookRateMax), int64(cfg.BookRatePerSec))
	ctrl.SyncFromRedis(context.Background())

	backend, err := url.Parse(backendURL)
	if err != nil {
		log.Fatal(err)
	}
	query, err := url.Parse(queryURL)
	if err != nil {
		log.Fatal(err)
	}

	r := gin.Default()
	r.Use(middleware.CORS(cfg.CORSOrigins))

	// Single catch-all under /api to avoid Gin wildcard conflicts with fixed routes.
	r.Any("/api/*filepath", func(c *gin.Context) {
		path := c.Param("filepath") // e.g. /tickets, /health
		if path == "/health" {
			response.OK(c, gin.H{"status": "up", "service": "gateway", "adaptive": ctrl.Snapshot(c.Request.Context())})
			return
		}
		if path == "/gateway/stats" {
			response.OK(c, ctrl.Snapshot(c.Request.Context()))
			return
		}

		start := time.Now()
		if c.Request.Method == http.MethodPost && strings.HasPrefix(path, "/orders") &&
			!strings.Contains(path, "/pay") && !strings.Contains(path, "/cancel") &&
			!strings.Contains(path, "/refund") && !strings.Contains(path, "/reschedule") {
			rate := ctrl.CurrentRate()
			if !adaptive.RateLimitAllow(c.Request.Context(), rdb, "book", rate) {
				response.Fail(c, http.StatusTooManyRequests, 429, "网关自适应限流中，请稍后重试")
				return
			}
		}

		target := backend
		observe := true
		if strings.HasPrefix(path, "/tickets") || strings.HasPrefix(path, "/stations") {
			target = query
			observe = false
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("gateway proxy error: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, `{"code":502,"message":"upstream unavailable"}`)
		}
		if observe {
			proxy.ModifyResponse = func(resp *http.Response) error {
				ctrl.ObserveLatency(context.Background(), time.Since(start).Milliseconds())
				return nil
			}
		}
		c.Request.URL.Path = "/api" + path
		proxy.ServeHTTP(c.Writer, c.Request)
	})

	log.Printf("12306cn gateway listening on :%s (backend=%s query=%s)", port, backendURL, queryURL)
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
