package model

import (
	"time"
	"gorm.io/gorm"
)

// TeslaToken 表示 Tesla API token 的数据模型
type TeslaToken struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	UserID       string         `json:"user_id" gorm:"uniqueIndex;not null;size:100"`
	AccessToken  string         `json:"access_token" gorm:"type:text;not null"`
	RefreshToken string         `json:"refresh_token" gorm:"type:text;not null"`
	TokenType    string         `json:"token_type" gorm:"default:Bearer;size:20"`
	ExpiresAt    time.Time      `json:"expires_at" gorm:"not null"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 指定表名
func (TeslaToken) TableName() string {
	return "tesla_tokens"
}

// IsExpired 检查 token 是否过期
func (t *TeslaToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsExpiringSoon 检查 token 是否即将过期（5分钟内）
func (t *TeslaToken) IsExpiringSoon() bool {
	return time.Now().Add(5 * time.Minute).After(t.ExpiresAt)
}

// TokenUsageLog 表示 token 使用日志
type TokenUsageLog struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	UserID       string    `json:"user_id" gorm:"not null;size:100"`
	Action       string    `json:"action" gorm:"not null;size:50"` // validate, refresh, create, delete
	IPAddress    string    `json:"ip_address" gorm:"size:45"`
	UserAgent    string    `json:"user_agent" gorm:"type:text"`
	Success      bool      `json:"success" gorm:"default:true"`
	ErrorMessage string    `json:"error_message" gorm:"type:text"`
	CreatedAt    time.Time `json:"created_at"`
}

// TableName 指定表名
func (TokenUsageLog) TableName() string {
	return "token_usage_logs"
}

// SaveTokenRequest 保存 token 的请求结构
type SaveTokenRequest struct {
	UserID       string `json:"user_id" binding:"required"`
	AccessToken  string `json:"access_token" binding:"required"`
	RefreshToken string `json:"refresh_token" binding:"required"`
	ExpiresAt    string `json:"expires_at" binding:"required"` // RFC3339 格式
}

// TokenValidationResponse token 验证响应
type TokenValidationResponse struct {
	Valid     bool      `json:"valid"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	Message   string    `json:"message,omitempty"`
}

// TokenResponse token 响应结构
type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// TeslaAPIResponse Tesla API 响应结构
type TeslaAPIResponse struct {
	Response interface{} `json:"response"`
	Error    string      `json:"error,omitempty"`
}

// TeslaUserInfo Tesla 用户信息
type TeslaUserInfo struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
}

// TeslaTokenRefreshRequest Tesla token 刷新请求
type TeslaTokenRefreshRequest struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
}

// TeslaTokenRefreshResponse Tesla token 刷新响应
type TeslaTokenRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}