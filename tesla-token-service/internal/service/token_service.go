package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"tesla-token-service/internal/config"
	"tesla-token-service/internal/model"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type TokenService struct {
	db          *gorm.DB
	httpClient  *http.Client
	teslaConfig config.TeslaConfig
}

func NewTokenService(db *gorm.DB, teslaConfig config.TeslaConfig) *TokenService {
	return &TokenService{
		db: db,
		httpClient: &http.Client{
			Timeout: time.Duration(teslaConfig.Timeout) * time.Second,
		},
		teslaConfig: teslaConfig,
	}
}

// SaveToken 保存 Tesla token
func (s *TokenService) SaveToken(token *model.TeslaToken) error {
	// 使用 UPSERT 操作，如果用户已存在则更新
	result := s.db.Where("user_id = ?", token.UserID).Assign(token).FirstOrCreate(token)
	if result.Error != nil {
		logrus.Errorf("Failed to save token for user %s: %v", token.UserID, result.Error)
		return result.Error
	}

	s.logTokenUsage(token.UserID, "create", "", "", true, "")
	logrus.Infof("Token saved successfully for user: %s", token.UserID)
	return nil
}

// GetValidToken 获取有效的 token，如果过期则自动刷新
func (s *TokenService) GetValidToken(userID string) (*model.TeslaToken, error) {
	var token model.TeslaToken
	if err := s.db.Where("user_id = ?", userID).First(&token).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("token not found for user: %s", userID)
		}
		return nil, err
	}

	// 如果 token 即将过期或已过期，尝试刷新
	if token.IsExpiringSoon() || token.IsExpired() {
		logrus.Infof("Token for user %s is expiring soon, attempting refresh", userID)
		if err := s.RefreshTeslaToken(userID); err != nil {
			logrus.Errorf("Failed to refresh token for user %s: %v", userID, err)
			return nil, fmt.Errorf("token expired and refresh failed: %v", err)
		}
		
		// 重新获取刷新后的 token
		if err := s.db.Where("user_id = ?", userID).First(&token).Error; err != nil {
			return nil, err
		}
	}

	return &token, nil
}

// ValidateTeslaToken 验证 Tesla token 的有效性
func (s *TokenService) ValidateTeslaToken(userID string) (*model.TokenValidationResponse, error) {
	// 1. 从数据库获取 token
	var token model.TeslaToken
	if err := s.db.Where("user_id = ?", userID).First(&token).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			s.logTokenUsage(userID, "validate", "", "", false, "Token not found")
			return &model.TokenValidationResponse{
				Valid:   false,
				Message: "Token not found",
			}, nil
		}
		return nil, err
	}

	// 2. 检查本地过期时间
	if token.IsExpired() {
		logrus.Infof("Token for user %s has expired locally", userID)
		// 尝试刷新
		if err := s.RefreshTeslaToken(userID); err != nil {
			s.logTokenUsage(userID, "validate", "", "", false, "Token expired and refresh failed")
			return &model.TokenValidationResponse{
				Valid:   false,
				Message: "Token expired and refresh failed",
			}, nil
		}
		// 重新获取刷新后的 token
		if err := s.db.Where("user_id = ?", userID).First(&token).Error; err != nil {
			return nil, err
		}
	}

	// 3. 通过 Tesla API 验证 token 实际有效性
	valid, err := s.validateTokenWithTeslaAPI(token.AccessToken)
	if err != nil {
		s.logTokenUsage(userID, "validate", "", "", false, err.Error())
		return &model.TokenValidationResponse{
			Valid:   false,
			Message: fmt.Sprintf("Failed to validate with Tesla API: %v", err),
		}, nil
	}

	if !valid {
		logrus.Infof("Token for user %s is invalid on Tesla API, attempting refresh", userID)
		// Token 在 Tesla 端无效，尝试刷新
		if err := s.RefreshTeslaToken(userID); err != nil {
			s.logTokenUsage(userID, "validate", "", "", false, "Token invalid and refresh failed")
			return &model.TokenValidationResponse{
				Valid:   false,
				Message: "Token invalid and refresh failed",
			}, nil
		}
		
		// 重新验证刷新后的 token
		if err := s.db.Where("user_id = ?", userID).First(&token).Error; err == nil {
			if valid, _ := s.validateTokenWithTeslaAPI(token.AccessToken); valid {
				s.logTokenUsage(userID, "validate", "", "", true, "Token refreshed and valid")
				return &model.TokenValidationResponse{
					Valid:     true,
					ExpiresAt: token.ExpiresAt,
					UserID:    userID,
					Message:   "Token refreshed and valid",
				}, nil
			}
		}
		
		s.logTokenUsage(userID, "validate", "", "", false, "Token validation failed after refresh")
		return &model.TokenValidationResponse{
			Valid:   false,
			Message: "Token validation failed",
		}, nil
	}

	s.logTokenUsage(userID, "validate", "", "", true, "Token is valid")
	return &model.TokenValidationResponse{
		Valid:     true,
		ExpiresAt: token.ExpiresAt,
		UserID:    userID,
		Message:   "Token is valid",
	}, nil
}

// validateTokenWithTeslaAPI 通过 Tesla API 验证 token
func (s *TokenService) validateTokenWithTeslaAPI(accessToken string) (bool, error) {
	req, err := http.NewRequest("GET", s.teslaConfig.BaseURL+"/api/1/me", nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	logrus.Debugf("Tesla API validation response status: %d", resp.StatusCode)
	return resp.StatusCode == 200, nil
}

// RefreshTeslaToken 刷新 Tesla token
func (s *TokenService) RefreshTeslaToken(userID string) error {
	// 1. 获取当前 refresh token
	var token model.TeslaToken
	if err := s.db.Where("user_id = ?", userID).First(&token).Error; err != nil {
		return fmt.Errorf("token not found for user: %s", userID)
	}

	logrus.Infof("Attempting to refresh token for user: %s", userID)

	// 2. 构造刷新请求
	refreshData := model.TeslaTokenRefreshRequest{
		GrantType:    "refresh_token",
		RefreshToken: token.RefreshToken,
	}

	jsonData, err := json.Marshal(refreshData)
	if err != nil {
		return err
	}

	// 3. 调用 Tesla API 刷新 token
	req, err := http.NewRequest("POST", "https://auth.tesla.com/oauth2/v3/token", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logTokenUsage(userID, "refresh", "", "", false, err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		s.logTokenUsage(userID, "refresh", "", "", false, fmt.Sprintf("HTTP %d", resp.StatusCode))
		return fmt.Errorf("failed to refresh token, status: %d", resp.StatusCode)
	}

	// 4. 解析响应
	var tokenResp model.TeslaTokenRefreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		s.logTokenUsage(userID, "refresh", "", "", false, "Failed to decode response")
		return err
	}

	// 5. 更新数据库中的 token
	token.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		token.RefreshToken = tokenResp.RefreshToken
	}
	token.TokenType = tokenResp.TokenType
	token.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	token.UpdatedAt = time.Now()

	if err := s.db.Save(&token).Error; err != nil {
		s.logTokenUsage(userID, "refresh", "", "", false, "Failed to save refreshed token")
		return err
	}

	s.logTokenUsage(userID, "refresh", "", "", true, "Token refreshed successfully")
	logrus.Infof("Token refreshed successfully for user: %s", userID)
	return nil
}

// DeleteToken 删除用户的 token
func (s *TokenService) DeleteToken(userID string) error {
	result := s.db.Where("user_id = ?", userID).Delete(&model.TeslaToken{})
	if result.Error != nil {
		s.logTokenUsage(userID, "delete", "", "", false, result.Error.Error())
		return result.Error
	}

	if result.RowsAffected == 0 {
		s.logTokenUsage(userID, "delete", "", "", false, "Token not found")
		return fmt.Errorf("token not found for user: %s", userID)
	}

	s.logTokenUsage(userID, "delete", "", "", true, "Token deleted successfully")
	logrus.Infof("Token deleted successfully for user: %s", userID)
	return nil
}

// logTokenUsage 记录 token 使用日志
func (s *TokenService) logTokenUsage(userID, action, ipAddress, userAgent string, success bool, errorMessage string) {
	log := &model.TokenUsageLog{
		UserID:       userID,
		Action:       action,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		Success:      success,
		ErrorMessage: errorMessage,
	}

	if err := s.db.Create(log).Error; err != nil {
		logrus.Errorf("Failed to log token usage: %v", err)
	}
}

// GetTokenUsageLogs 获取 token 使用日志
func (s *TokenService) GetTokenUsageLogs(userID string, limit int) ([]model.TokenUsageLog, error) {
	var logs []model.TokenUsageLog
	query := s.db.Where("user_id = ?", userID).Order("created_at DESC")
	
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}

	return logs, nil
}

// GetAllTokens 获取所有用户的 token 信息（管理接口）
func (s *TokenService) GetAllTokens() ([]model.TeslaToken, error) {
	var tokens []model.TeslaToken
	if err := s.db.Find(&tokens).Error; err != nil {
		return nil, err
	}
	return tokens, nil
}

// CleanupExpiredTokens 清理过期的 token
func (s *TokenService) CleanupExpiredTokens() error {
	// 删除过期超过 7 天的 token
	expiredTime := time.Now().Add(-7 * 24 * time.Hour)
	result := s.db.Where("expires_at < ?", expiredTime).Delete(&model.TeslaToken{})
	
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		logrus.Infof("Cleaned up %d expired tokens", result.RowsAffected)
	}

	return nil
}