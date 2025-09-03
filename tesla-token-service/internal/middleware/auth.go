package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
)

// JWTAuthMiddleware JWT 认证中间件
func JWTAuthMiddleware(secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Authorization header required",
			})
			c.Abort()
			return
		}

		// 移除 "Bearer " 前缀
		if strings.HasPrefix(tokenString, "Bearer ") {
			tokenString = strings.TrimPrefix(tokenString, "Bearer ")
		}

		// 解析和验证 JWT token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// 验证签名方法
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secretKey), nil
		})

		if err != nil {
			logrus.Debugf("JWT parse error: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Invalid token",
			})
			c.Abort()
			return
		}

		if !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Token is not valid",
			})
			c.Abort()
			return
		}

		// 提取 claims
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			// 检查 token 是否过期
			if exp, ok := claims["exp"].(float64); ok {
				if int64(exp) < jwt.TimeFunc().Unix() {
					c.JSON(http.StatusUnauthorized, gin.H{
						"success": false,
						"error":   "Token has expired",
					})
					c.Abort()
					return
				}
			}

			// 将用户信息存储到上下文中
			if sub, ok := claims["sub"].(string); ok {
				c.Set("user_id", sub)
			}
		}

		c.Next()
	}
}

// CORSMiddleware CORS 中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// LoggingMiddleware 请求日志中间件
func LoggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		logrus.WithFields(logrus.Fields{
			"client_ip":   param.ClientIP,
			"method":      param.Method,
			"path":        param.Path,
			"status_code": param.StatusCode,
			"latency":     param.Latency,
			"user_agent":  param.Request.UserAgent(),
		}).Info("Request processed")
		return ""
	})
}

// RateLimitMiddleware 简单的速率限制中间件（可选）
func RateLimitMiddleware() gin.HandlerFunc {
	// 这里可以集成 redis 或内存的速率限制
	return func(c *gin.Context) {
		// 简单实现，实际项目中建议使用专业的限流库
		c.Next()
	}
}