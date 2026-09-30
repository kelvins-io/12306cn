package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	jwtutil "github.com/kelvins-io/12306cn/backend/internal/pkg/jwt"
	"github.com/kelvins-io/12306cn/backend/internal/pkg/response"
)

const CtxUserID = "user_id"
const CtxUsername = "username"
const CtxRole = "role"

func CORS(origins string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allow := origins
		if origins == "*" || origins == "" {
			if origin != "" {
				allow = origin
			} else {
				allow = "*"
			}
		}
		c.Header("Access-Control-Allow-Origin", allow)
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			response.Unauthorized(c, "缺少登录凭证")
			c.Abort()
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		claims, err := jwtutil.Parse(secret, token)
		if err != nil {
			response.Unauthorized(c, "登录已失效，请重新登录")
			c.Abort()
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxUsername, claims.Username)
		role := claims.Role
		if role == "" {
			role = "user"
		}
		c.Set(CtxRole, role)
		c.Next()
	}
}

func RequireRoles(roles ...string) gin.HandlerFunc {
	set := map[string]struct{}{}
	for _, r := range roles {
		set[r] = struct{}{}
	}
	return func(c *gin.Context) {
		role, _ := c.Get(CtxRole)
		rs, _ := role.(string)
		if _, ok := set[rs]; !ok {
			response.Forbidden(c, "无权限")
			c.Abort()
			return
		}
		c.Next()
	}
}

func GetUserID(c *gin.Context) uint {
	v, _ := c.Get(CtxUserID)
	id, _ := v.(uint)
	return id
}

func GetUsername(c *gin.Context) string {
	v, _ := c.Get(CtxUsername)
	s, _ := v.(string)
	return s
}

func GetRole(c *gin.Context) string {
	v, _ := c.Get(CtxRole)
	s, _ := v.(string)
	return s
}
