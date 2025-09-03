package services

import (
    "bytes"
    "encoding/json"
    "errors"
    "fmt"
    "net/http"
    "time"
    
    "gorm.io/gorm"
    
    "teslamate-token-service/crypto"
    "teslamate-token-service/models"
)

const (
    TeslaAuthURL = "https://auth.tesla.com/oauth2/v3/token"
    TeslaAPIURL  = "https://owner-api.teslamotors.com/api/1/vehicles"
)

type TokenService struct {
    db         *gorm.DB
    encryptor  *crypto.TokenEncryptor
    httpClient *http.Client
}

func NewTokenService(db *gorm.DB, encryptor *crypto.TokenEncryptor) *TokenService {
    return &TokenService{
        db:        db,
        encryptor: encryptor,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

// SaveToken 保存或更新 Token
func (s *TokenService) SaveToken(req *models.TokenRequest) (*models.TokenResponse, error) {
    // 加密 tokens
    encryptedAccessToken, err := s.encryptor.Encrypt(req.AccessToken)
    if err != nil {
        return nil, fmt.Errorf("failed to encrypt access token: %w", err)
    }
    
    encryptedRefreshToken, err := s.encryptor.Encrypt(req.RefreshToken)
    if err != nil {
        return nil, fmt.Errorf("failed to encrypt refresh token: %w", err)
    }
    
    // 计算过期时间
    expiresAt := time.Now().Add(time.Duration(req.ExpiresIn) * time.Second)
    
    // 查找现有 token
    var token models.Token
    result := s.db.Where("account_id = ?", req.AccountID).First(&token)
    
    if result.Error == nil {
        // 更新现有 token
        token.AccessToken = encryptedAccessToken
        token.RefreshToken = encryptedRefreshToken
        token.ExpiresAt = expiresAt
        token.TokenType = req.TokenType
        token.Scope = req.Scope
        
        if err := s.db.Save(&token).Error; err != nil {
            return nil, fmt.Errorf("failed to update token: %w", err)
        }
    } else if errors.Is(result.Error, gorm.ErrRecordNotFound) {
        // 创建新 token
        token = models.Token{
            AccountID:    req.AccountID,
            AccessToken:  encryptedAccessToken,
            RefreshToken: encryptedRefreshToken,
            ExpiresAt:    expiresAt,
            TokenType:    req.TokenType,
            Scope:        req.Scope,
        }
        
        if err := s.db.Create(&token).Error; err != nil {
            return nil, fmt.Errorf("failed to create token: %w", err)
        }
    } else {
        return nil, fmt.Errorf("database error: %w", result.Error)
    }
    
    // 记录审计日志
    s.logAudit(req.AccountID, "created", true, "")
    
    return &models.TokenResponse{
        AccountID: token.AccountID,
        ExpiresAt: token.ExpiresAt,
        TokenType: token.TokenType,
    }, nil
}

// GetToken 获取 Token
func (s *TokenService) GetToken(accountID string) (*models.TokenResponse, error) {
    var token models.Token
    if err := s.db.Where("account_id = ?", accountID).First(&token).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, fmt.Errorf("token not found for account: %s", accountID)
        }
        return nil, fmt.Errorf("database error: %w", err)
    }
    
    // 解密 tokens
    accessToken, err := s.encryptor.Decrypt(token.AccessToken)
    if err != nil {
        return nil, fmt.Errorf("failed to decrypt access token: %w", err)
    }
    
    refreshToken, err := s.encryptor.Decrypt(token.RefreshToken)
    if err != nil {
        return nil, fmt.Errorf("failed to decrypt refresh token: %w", err)
    }
    
    return &models.TokenResponse{
        AccountID:    token.AccountID,
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresAt:    token.ExpiresAt,
        TokenType:    token.TokenType,
    }, nil
}

// ValidateToken 验证 Token 有效性
func (s *TokenService) ValidateToken(req *models.ValidateRequest) (*models.ValidateResponse, error) {
    var token models.Token
    if err := s.db.Where("account_id = ?", req.AccountID).First(&token).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return &models.ValidateResponse{Valid: false}, nil
        }
        return nil, fmt.Errorf("database error: %w", err)
    }
    
    // 解密存储的 access token
    storedAccessToken, err := s.encryptor.Decrypt(token.AccessToken)
    if err != nil {
        return nil, fmt.Errorf("failed to decrypt access token: %w", err)
    }
    
    // 验证 token 是否匹配
    if storedAccessToken != req.AccessToken {
        s.logAudit(req.AccountID, "validated", false, "token mismatch")
        return &models.ValidateResponse{Valid: false}, nil
    }
    
    // 检查是否过期
    if token.IsExpired() {
        s.logAudit(req.AccountID, "validated", false, "token expired")
        return &models.ValidateResponse{Valid: false}, nil
    }
    
    // 可选：调用 Tesla API 验证 token 真实有效性
    if err := s.validateWithTeslaAPI(req.AccessToken); err != nil {
        s.logAudit(req.AccountID, "validated", false, err.Error())
        return &models.ValidateResponse{Valid: false}, nil
    }
    
    s.logAudit(req.AccountID, "validated", true, "")
    
    return &models.ValidateResponse{
        Valid:            true,
        ExpiresAt:        token.ExpiresAt,
        RemainingSeconds: token.GetRemainingSeconds(),
    }, nil
}

// RefreshToken 刷新 Token
func (s *TokenService) RefreshToken(accountID string) (*models.TokenResponse, error) {
    // 获取当前 token
    tokenResp, err := s.GetToken(accountID)
    if err != nil {
        return nil, err
    }
    
    // 调用 Tesla API 刷新 token
    teslaResp, err := s.refreshWithTeslaAPI(tokenResp.RefreshToken)
    if err != nil {
        s.logAudit(accountID, "refreshed", false, err.Error())
        return nil, fmt.Errorf("failed to refresh token: %w", err)
    }
    
    // 保存新的 token
    saveReq := &models.TokenRequest{
        AccountID:    accountID,
        AccessToken:  teslaResp.AccessToken,
        RefreshToken: teslaResp.RefreshToken,
        ExpiresIn:    teslaResp.ExpiresIn,
        TokenType:    teslaResp.TokenType,
        Scope:        teslaResp.Scope,
    }
    
    result, err := s.SaveToken(saveReq)
    if err != nil {
        return nil, err
    }
    
    s.logAudit(accountID, "refreshed", true, "")
    
    // 返回完整的 token 信息
    return s.GetToken(accountID)
}

// validateWithTeslaAPI 使用 Tesla API 验证 token
func (s *TokenService) validateWithTeslaAPI(accessToken string) error {
    req, err := http.NewRequest("GET", TeslaAPIURL, nil)
    if err != nil {
        return err
    }
    
    req.Header.Set("Authorization", "Bearer "+accessToken)
    
    resp, err := s.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == http.StatusUnauthorized {
        return errors.New("unauthorized: invalid token")
    }
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("tesla API returned status: %d", resp.StatusCode)
    }
    
    return nil
}

// refreshWithTeslaAPI 使用 Tesla API 刷新 token
func (s *TokenService) refreshWithTeslaAPI(refreshToken string) (*models.TeslaTokenResponse, error) {
    requestBody := map[string]string{
        "grant_type":    "refresh_token",
        "refresh_token": refreshToken,
        "client_id":     "ownerapi",
        "scope":         "openid email offline_access",
    }
    
    jsonBody, err := json.Marshal(requestBody)
    if err != nil {
        return nil, err
    }
    
    req, err := http.NewRequest("POST", TeslaAuthURL, bytes.NewBuffer(jsonBody))
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := s.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("token refresh failed with status: %d", resp.StatusCode)
    }
    
    var tokenResp models.TeslaTokenResponse
    if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
        return nil, err
    }
    
    return &tokenResp, nil
}

// logAudit 记录审计日志
func (s *TokenService) logAudit(accountID, action string, success bool, errorMessage string) {
    log := models.TokenAuditLog{
        AccountID:    accountID,
        Action:       action,
        Success:      success,
        ErrorMessage: errorMessage,
        CreatedAt:    time.Now(),
    }
    
    // 异步记录，避免影响主流程
    go func() {
        if err := s.db.Create(&log).Error; err != nil {
            // 记录日志失败，可以使用其他日志系统记录
            fmt.Printf("Failed to create audit log: %v\n", err)
        }
    }()
}