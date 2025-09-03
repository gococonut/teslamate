package handler

import (
	"net/http"
	"strconv"
	"time"

	"tesla-token-service/internal/model"
	"tesla-token-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type TokenHandler struct {
	tokenService *service.TokenService
}

func NewTokenHandler(tokenService *service.TokenService) *TokenHandler {
	return &TokenHandler{
		tokenService: tokenService,
	}
}

// SaveToken 保存 Tesla token
func (h *TokenHandler) SaveToken(c *gin.Context) {
	var req model.SaveTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 解析过期时间
	expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid expires_at format, expected RFC3339",
		})
		return
	}

	token := &model.TeslaToken{
		UserID:       req.UserID,
		AccessToken:  req.AccessToken,
		RefreshToken: req.RefreshToken,
		TokenType:    "Bearer",
		ExpiresAt:    expiresAt,
	}

	if err := h.tokenService.SaveToken(token); err != nil {
		logrus.Errorf("Failed to save token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to save token",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Token saved successfully",
		"user_id": req.UserID,
	})
}

// GetToken 获取用户的有效 token
func (h *TokenHandler) GetToken(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "user_id is required",
		})
		return
	}

	token, err := h.tokenService.GetValidToken(userID)
	if err != nil {
		logrus.Errorf("Failed to get token for user %s: %v", userID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Token not found or invalid",
		})
		return
	}

	response := model.TokenResponse{
		AccessToken: token.AccessToken,
		TokenType:   token.TokenType,
		ExpiresAt:   token.ExpiresAt,
	}

	c.JSON(http.StatusOK, response)
}

// ValidateToken 验证 token 有效性
func (h *TokenHandler) ValidateToken(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "user_id is required",
		})
		return
	}

	validation, err := h.tokenService.ValidateTeslaToken(userID)
	if err != nil {
		logrus.Errorf("Failed to validate token for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if validation.Valid {
		c.JSON(http.StatusOK, validation)
	} else {
		c.JSON(http.StatusUnauthorized, validation)
	}
}

// RefreshToken 刷新 token
func (h *TokenHandler) RefreshToken(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "user_id is required",
		})
		return
	}

	if err := h.tokenService.RefreshTeslaToken(userID); err != nil {
		logrus.Errorf("Failed to refresh token for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 获取刷新后的 token
	token, err := h.tokenService.GetValidToken(userID)
	if err != nil {
		logrus.Errorf("Failed to get refreshed token for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get refreshed token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"token_type":    token.TokenType,
		"expires_at":    token.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// DeleteToken 删除用户的 token
func (h *TokenHandler) DeleteToken(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "user_id is required",
		})
		return
	}

	if err := h.tokenService.DeleteToken(userID); err != nil {
		logrus.Errorf("Failed to delete token for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Token deleted successfully",
	})
}

// GetTokenLogs 获取 token 使用日志
func (h *TokenHandler) GetTokenLogs(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "user_id is required",
		})
		return
	}

	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 100
	}

	logs, err := h.tokenService.GetTokenUsageLogs(userID, limit)
	if err != nil {
		logrus.Errorf("Failed to get token logs for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to get token logs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"logs":    logs,
	})
}

// HealthCheck 健康检查接口
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "tesla-token-service",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	})
}