package models

import (
    "time"
    "gorm.io/gorm"
)

// Token 表示存储的 Tesla API Token
type Token struct {
    ID                uint      `json:"id" gorm:"primaryKey"`
    AccountID         string    `json:"account_id" gorm:"uniqueIndex;not null"`
    AccessToken       string    `json:"access_token" gorm:"not null"`
    RefreshToken      string    `json:"refresh_token" gorm:"not null"`
    ExpiresAt         time.Time `json:"expires_at" gorm:"not null"`
    TokenType         string    `json:"token_type" gorm:"default:'Bearer'"`
    Scope             string    `json:"scope"`
    IsEncrypted       bool      `json:"is_encrypted" gorm:"default:true"`
    EncryptionVersion int       `json:"encryption_version" gorm:"default:1"`
    CreatedAt         time.Time `json:"created_at"`
    UpdatedAt         time.Time `json:"updated_at"`
}

// TokenAuditLog 记录 Token 操作的审计日志
type TokenAuditLog struct {
    ID           uint      `json:"id" gorm:"primaryKey"`
    AccountID    string    `json:"account_id" gorm:"not null"`
    Action       string    `json:"action" gorm:"not null"` // created, refreshed, validated, expired
    IPAddress    string    `json:"ip_address"`
    UserAgent    string    `json:"user_agent"`
    Success      bool      `json:"success" gorm:"default:true"`
    ErrorMessage string    `json:"error_message"`
    CreatedAt    time.Time `json:"created_at"`
}

// TokenRequest 用于创建或更新 Token 的请求
type TokenRequest struct {
    AccountID    string `json:"account_id" binding:"required"`
    AccessToken  string `json:"access_token" binding:"required"`
    RefreshToken string `json:"refresh_token" binding:"required"`
    ExpiresIn    int    `json:"expires_in" binding:"required"` // 秒
    TokenType    string `json:"token_type"`
    Scope        string `json:"scope"`
}

// TokenResponse 用于返回 Token 信息
type TokenResponse struct {
    AccountID    string    `json:"account_id"`
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    ExpiresAt    time.Time `json:"expires_at"`
    TokenType    string    `json:"token_type"`
}

// ValidateRequest 用于验证 Token 的请求
type ValidateRequest struct {
    AccountID   string `json:"account_id" binding:"required"`
    AccessToken string `json:"access_token" binding:"required"`
}

// ValidateResponse 用于返回验证结果
type ValidateResponse struct {
    Valid            bool      `json:"valid"`
    ExpiresAt        time.Time `json:"expires_at,omitempty"`
    RemainingSeconds int64     `json:"remaining_seconds,omitempty"`
}

// RefreshRequest 用于刷新 Token 的请求
type RefreshRequest struct {
    AccountID string `json:"account_id" binding:"required"`
}

// TeslaTokenResponse Tesla API 返回的 Token 响应
type TeslaTokenResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int    `json:"expires_in"`
    TokenType    string `json:"token_type"`
    Scope        string `json:"scope"`
}

// BeforeCreate 在创建前执行
func (t *Token) BeforeCreate(tx *gorm.DB) error {
    t.CreatedAt = time.Now()
    t.UpdatedAt = time.Now()
    return nil
}

// BeforeUpdate 在更新前执行
func (t *Token) BeforeUpdate(tx *gorm.DB) error {
    t.UpdatedAt = time.Now()
    return nil
}

// IsExpired 检查 Token 是否已过期
func (t *Token) IsExpired() bool {
    return time.Now().After(t.ExpiresAt)
}

// GetRemainingSeconds 获取 Token 剩余有效时间（秒）
func (t *Token) GetRemainingSeconds() int64 {
    if t.IsExpired() {
        return 0
    }
    return int64(time.Until(t.ExpiresAt).Seconds())
}