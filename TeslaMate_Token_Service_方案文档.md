# TeslaMate Token 管理服务方案文档

## 项目概述

基于 Gin 框架开发一个独立的 token 管理服务，用于安全存储和管理 TeslaMate 所需的 Tesla API 访问令牌（Access Token）和刷新令牌（Refresh Token），并提供 API 接口供 TeslaMate 验证 token 有效性。

## 方案可行性分析

### ✅ 技术可行性

1. **Tesla API 认证机制**
   - Tesla 使用 OAuth2 认证流程
   - 提供 Access Token（短期有效，约1小时）和 Refresh Token（长期有效，约45天）
   - API 端点：`https://owner-api.teslamotors.com/api/1/`
   - 验证端点：`/api/1/me` 可用于验证 token 有效性

2. **TeslaMate 集成**
   - TeslaMate 是 Elixir/Phoenix 应用，支持外部 token 提供
   - 可通过环境变量或数据库配置 token 信息
   - 支持 token 过期后的自动刷新机制

3. **Gin 框架优势**
   - 高性能、轻量级的 Go Web 框架
   - 丰富的中间件支持
   - 完善的 JWT 生态系统
   - 易于部署和维护

### ✅ 架构可行性

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   TeslaMate     │    │  Gin Token      │    │   Tesla API     │
│                 │    │   Service       │    │                 │
│                 │◄──►│                 │◄──►│                 │
│ - 获取车辆数据   │    │ - Token 存储     │    │ - 车辆信息      │
│ - 验证 Token    │    │ - Token 验证     │    │ - 用户信息      │
│ - 刷新 Token    │    │ - Token 刷新     │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 详细实现方案

### 1. 数据库设计

```sql
-- Tesla Token 存储表
CREATE TABLE tesla_tokens (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(100) NOT NULL UNIQUE COMMENT '用户标识',
    access_token TEXT NOT NULL COMMENT '访问令牌',
    refresh_token TEXT NOT NULL COMMENT '刷新令牌',
    token_type VARCHAR(20) DEFAULT 'Bearer' COMMENT '令牌类型',
    expires_at TIMESTAMP NOT NULL COMMENT '访问令牌过期时间',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_user_id (user_id),
    INDEX idx_expires_at (expires_at)
);

-- Token 使用日志表（可选）
CREATE TABLE token_usage_logs (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL COMMENT 'validate|refresh|create',
    ip_address VARCHAR(45),
    user_agent TEXT,
    success BOOLEAN DEFAULT TRUE,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    INDEX idx_user_id (user_id),
    INDEX idx_created_at (created_at)
);
```

### 2. 核心数据结构

```go
// Token 模型
type TeslaToken struct {
    ID           uint      `json:"id" gorm:"primaryKey"`
    UserID       string    `json:"user_id" gorm:"uniqueIndex;not null"`
    AccessToken  string    `json:"access_token" gorm:"type:text;not null"`
    RefreshToken string    `json:"refresh_token" gorm:"type:text;not null"`
    TokenType    string    `json:"token_type" gorm:"default:Bearer"`
    ExpiresAt    time.Time `json:"expires_at" gorm:"not null"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

// Token 验证响应
type TokenValidationResponse struct {
    Valid     bool      `json:"valid"`
    ExpiresAt time.Time `json:"expires_at,omitempty"`
    UserID    string    `json:"user_id,omitempty"`
    Message   string    `json:"message,omitempty"`
}

// Tesla API 用户信息响应
type TeslaUserInfo struct {
    ID    int    `json:"id"`
    Email string `json:"email"`
}
```

### 3. 核心 API 接口设计

#### 3.1 保存 Token
```
POST /api/v1/tokens
Content-Type: application/json

{
    "user_id": "tesla_user_123",
    "access_token": "eyJ0eXAiOiJKV1QiLCJhbGc...",
    "refresh_token": "eyJ0eXAiOiJKV1QiLCJhbGc...",
    "expires_at": "2024-01-15T10:30:00Z"
}

Response:
{
    "success": true,
    "message": "Token saved successfully",
    "user_id": "tesla_user_123"
}
```

#### 3.2 验证 Token
```
GET /api/v1/tokens/{user_id}/validate
Authorization: Bearer {internal_jwt_token}

Response:
{
    "valid": true,
    "expires_at": "2024-01-15T10:30:00Z",
    "user_id": "tesla_user_123",
    "message": "Token is valid"
}
```

#### 3.3 刷新 Token
```
POST /api/v1/tokens/{user_id}/refresh
Authorization: Bearer {internal_jwt_token}

Response:
{
    "success": true,
    "access_token": "new_access_token...",
    "refresh_token": "new_refresh_token...",
    "expires_at": "2024-01-15T11:30:00Z"
}
```

#### 3.4 获取 Token（供 TeslaMate 使用）
```
GET /api/v1/tokens/{user_id}
Authorization: Bearer {internal_jwt_token}

Response:
{
    "access_token": "eyJ0eXAiOiJKV1QiLCJhbGc...",
    "token_type": "Bearer",
    "expires_at": "2024-01-15T10:30:00Z"
}
```

### 4. 核心功能实现

#### 4.1 Token 验证服务

```go
package service

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type TokenService struct {
    db     *gorm.DB
    client *http.Client
}

// 验证 Tesla Token 有效性
func (s *TokenService) ValidateTeslaToken(userID string) (*TokenValidationResponse, error) {
    // 1. 从数据库获取 token
    var token TeslaToken
    if err := s.db.Where("user_id = ?", userID).First(&token).Error; err != nil {
        return &TokenValidationResponse{
            Valid:   false,
            Message: "Token not found",
        }, nil
    }

    // 2. 检查本地过期时间
    if time.Now().After(token.ExpiresAt) {
        // 尝试使用 refresh token 刷新
        if err := s.RefreshTeslaToken(userID); err != nil {
            return &TokenValidationResponse{
                Valid:   false,
                Message: "Token expired and refresh failed",
            }, nil
        }
        // 重新获取刷新后的 token
        if err := s.db.Where("user_id = ?", userID).First(&token).Error; err != nil {
            return &TokenValidationResponse{
                Valid:   false,
                Message: "Failed to get refreshed token",
            }, nil
        }
    }

    // 3. 通过 Tesla API 验证 token 实际有效性
    if valid, err := s.validateTokenWithTeslaAPI(token.AccessToken); err != nil || !valid {
        // Token 在 Tesla 端无效，尝试刷新
        if err := s.RefreshTeslaToken(userID); err != nil {
            return &TokenValidationResponse{
                Valid:   false,
                Message: "Token invalid and refresh failed",
            }, nil
        }
        // 重新验证
        if err := s.db.Where("user_id = ?", userID).First(&token).Error; err == nil {
            if valid, _ := s.validateTokenWithTeslaAPI(token.AccessToken); valid {
                return &TokenValidationResponse{
                    Valid:     true,
                    ExpiresAt: token.ExpiresAt,
                    UserID:    userID,
                    Message:   "Token refreshed and valid",
                }, nil
            }
        }
        return &TokenValidationResponse{
            Valid:   false,
            Message: "Token validation failed",
        }, nil
    }

    return &TokenValidationResponse{
        Valid:     true,
        ExpiresAt: token.ExpiresAt,
        UserID:    userID,
        Message:   "Token is valid",
    }, nil
}

// 通过 Tesla API 验证 token
func (s *TokenService) validateTokenWithTeslaAPI(accessToken string) (bool, error) {
    req, err := http.NewRequest("GET", "https://owner-api.teslamotors.com/api/1/me", nil)
    if err != nil {
        return false, err
    }
    
    req.Header.Set("Authorization", "Bearer "+accessToken)
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := s.client.Do(req)
    if err != nil {
        return false, err
    }
    defer resp.Body.Close()
    
    return resp.StatusCode == 200, nil
}

// 刷新 Tesla Token
func (s *TokenService) RefreshTeslaToken(userID string) error {
    // 1. 获取当前 refresh token
    var token TeslaToken
    if err := s.db.Where("user_id = ?", userID).First(&token).Error; err != nil {
        return fmt.Errorf("token not found for user: %s", userID)
    }

    // 2. 调用 Tesla API 刷新 token
    refreshData := map[string]string{
        "grant_type":    "refresh_token",
        "refresh_token": token.RefreshToken,
    }
    
    jsonData, _ := json.Marshal(refreshData)
    req, err := http.NewRequest("POST", "https://auth.tesla.com/oauth2/v3/token", bytes.NewBuffer(jsonData))
    if err != nil {
        return err
    }
    
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := s.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return fmt.Errorf("failed to refresh token, status: %d", resp.StatusCode)
    }

    // 3. 解析新的 token 信息
    var tokenResp struct {
        AccessToken  string `json:"access_token"`
        RefreshToken string `json:"refresh_token"`
        ExpiresIn    int    `json:"expires_in"`
        TokenType    string `json:"token_type"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
        return err
    }

    // 4. 更新数据库中的 token
    token.AccessToken = tokenResp.AccessToken
    token.RefreshToken = tokenResp.RefreshToken
    token.TokenType = tokenResp.TokenType
    token.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
    token.UpdatedAt = time.Now()

    return s.db.Save(&token).Error
}
```

#### 4.2 Gin 路由和中间件

```go
package main

import (
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
    "gorm.io/gorm"
)

// 内部 JWT 中间件（保护服务端 API）
func JWTAuthMiddleware(secretKey string) gin.HandlerFunc {
    return func(c *gin.Context) {
        tokenString := c.GetHeader("Authorization")
        if tokenString == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
            c.Abort()
            return
        }

        // 移除 "Bearer " 前缀
        if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
            tokenString = tokenString[7:]
        }

        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            return []byte(secretKey), nil
        })

        if err != nil || !token.Valid {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
            c.Abort()
            return
        }

        c.Next()
    }
}

// 设置路由
func SetupRoutes(r *gin.Engine, tokenService *TokenService, jwtSecret string) {
    api := r.Group("/api/v1")
    
    // 公开接口 - 保存 token（初始化时使用）
    api.POST("/tokens", tokenService.SaveToken)
    
    // 受保护的接口
    protected := api.Group("/tokens")
    protected.Use(JWTAuthMiddleware(jwtSecret))
    {
        protected.GET("/:user_id", tokenService.GetToken)
        protected.GET("/:user_id/validate", tokenService.ValidateToken)
        protected.POST("/:user_id/refresh", tokenService.RefreshToken)
        protected.DELETE("/:user_id", tokenService.DeleteToken)
    }
    
    // 健康检查
    r.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{
            "status":    "healthy",
            "timestamp": time.Now().UTC(),
        })
    })
}
```

#### 4.3 Token 服务控制器

```go
package service

type TokenController struct {
    tokenService *TokenService
}

// 保存 Token
func (tc *TokenController) SaveToken(c *gin.Context) {
    var req struct {
        UserID       string `json:"user_id" binding:"required"`
        AccessToken  string `json:"access_token" binding:"required"`
        RefreshToken string `json:"refresh_token" binding:"required"`
        ExpiresAt    string `json:"expires_at" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expires_at format"})
        return
    }

    token := &TeslaToken{
        UserID:       req.UserID,
        AccessToken:  req.AccessToken,
        RefreshToken: req.RefreshToken,
        TokenType:    "Bearer",
        ExpiresAt:    expiresAt,
    }

    if err := tc.tokenService.SaveToken(token); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save token"})
        return
    }

    c.JSON(http.StatusCreated, gin.H{
        "success": true,
        "message": "Token saved successfully",
        "user_id": req.UserID,
    })
}

// 获取 Token
func (tc *TokenController) GetToken(c *gin.Context) {
    userID := c.Param("user_id")
    
    token, err := tc.tokenService.GetValidToken(userID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Token not found or invalid"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "access_token": token.AccessToken,
        "token_type":   token.TokenType,
        "expires_at":   token.ExpiresAt.UTC().Format(time.RFC3339),
    })
}

// 验证 Token
func (tc *TokenController) ValidateToken(c *gin.Context) {
    userID := c.Param("user_id")
    
    validation, err := tc.tokenService.ValidateTeslaToken(userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    if validation.Valid {
        c.JSON(http.StatusOK, validation)
    } else {
        c.JSON(http.StatusUnauthorized, validation)
    }
}

// 刷新 Token
func (tc *TokenController) RefreshToken(c *gin.Context) {
    userID := c.Param("user_id")
    
    if err := tc.tokenService.RefreshTeslaToken(userID); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "success": false,
            "error":   err.Error(),
        })
        return
    }

    // 获取刷新后的 token
    token, err := tc.tokenService.GetValidToken(userID)
    if err != nil {
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
        "expires_at":    token.ExpiresAt.UTC().Format(time.RFC3339),
    })
}
```

### 5. 配置管理

```go
package config

type Config struct {
    Server struct {
        Host string `yaml:"host" env:"SERVER_HOST" env-default:"0.0.0.0"`
        Port int    `yaml:"port" env:"SERVER_PORT" env-default:"8080"`
    } `yaml:"server"`
    
    Database struct {
        Host     string `yaml:"host" env:"DB_HOST" env-default:"localhost"`
        Port     int    `yaml:"port" env:"DB_PORT" env-default:"3306"`
        User     string `yaml:"user" env:"DB_USER" env-required:"true"`
        Password string `yaml:"password" env:"DB_PASSWORD" env-required:"true"`
        Name     string `yaml:"name" env:"DB_NAME" env-required:"true"`
    } `yaml:"database"`
    
    JWT struct {
        Secret string `yaml:"secret" env:"JWT_SECRET" env-required:"true"`
        TTL    int    `yaml:"ttl" env:"JWT_TTL" env-default:"86400"` // 24小时
    } `yaml:"jwt"`
    
    Tesla struct {
        BaseURL string `yaml:"base_url" env:"TESLA_BASE_URL" env-default:"https://owner-api.teslamotors.com"`
        Timeout int    `yaml:"timeout" env:"TESLA_TIMEOUT" env-default:"30"`
    } `yaml:"tesla"`
}
```

### 6. Docker 部署配置

#### 6.1 Dockerfile
```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o tesla-token-service ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

COPY --from=builder /app/tesla-token-service .
COPY --from=builder /app/config/config.yaml ./config/

EXPOSE 8080
CMD ["./tesla-token-service"]
```

#### 6.2 docker-compose.yml
```yaml
version: '3.8'

services:
  tesla-token-service:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DB_HOST=mysql
      - DB_USER=tesla_user
      - DB_PASSWORD=tesla_password
      - DB_NAME=tesla_tokens
      - JWT_SECRET=your_super_secret_key
    depends_on:
      - mysql
    restart: unless-stopped

  mysql:
    image: mysql:8.0
    environment:
      - MYSQL_ROOT_PASSWORD=root_password
      - MYSQL_DATABASE=tesla_tokens
      - MYSQL_USER=tesla_user
      - MYSQL_PASSWORD=tesla_password
    volumes:
      - mysql_data:/var/lib/mysql
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql
    restart: unless-stopped

volumes:
  mysql_data:
```

## 与 TeslaMate 集成方案

### 1. TeslaMate 配置修改

在 TeslaMate 的配置中添加 token 服务的端点：

```bash
# 环境变量配置
TESLA_TOKEN_SERVICE_URL=http://tesla-token-service:8080
TESLA_TOKEN_SERVICE_JWT=your_internal_jwt_token
TESLA_USER_ID=tesla_user_123
```

### 2. TeslaMate 集成代码示例

```elixir
# TeslaMate 中的 token 获取模块
defmodule TeslaMate.Auth.TokenService do
  @base_url Application.get_env(:teslamate, :token_service_url)
  @jwt_token Application.get_env(:teslamate, :token_service_jwt)
  @user_id Application.get_env(:teslamate, :tesla_user_id)

  def get_valid_token do
    url = "#{@base_url}/api/v1/tokens/#{@user_id}"
    headers = [
      {"Authorization", "Bearer #{@jwt_token}"},
      {"Content-Type", "application/json"}
    ]

    case HTTPoison.get(url, headers) do
      {:ok, %{status_code: 200, body: body}} ->
        case Jason.decode(body) do
          {:ok, %{"access_token" => token}} -> {:ok, token}
          _ -> {:error, :invalid_response}
        end
      _ -> {:error, :service_unavailable}
    end
  end

  def validate_token do
    url = "#{@base_url}/api/v1/tokens/#{@user_id}/validate"
    headers = [
      {"Authorization", "Bearer #{@jwt_token}"},
      {"Content-Type", "application/json"}
    ]

    case HTTPoison.get(url, headers) do
      {:ok, %{status_code: 200}} -> :ok
      _ -> :error
    end
  end
end
```

## 安全考虑

### 1. 数据安全
- **加密存储**: 使用 AES-256 对敏感 token 进行加密存储
- **访问控制**: 内部 JWT 认证保护所有 API 接口
- **网络安全**: 强制使用 HTTPS，禁用 HTTP

### 2. 运行时安全
- **Token 轮换**: 自动检测和刷新即将过期的 token
- **异常检测**: 监控异常访问模式和失败尝试
- **日志审计**: 记录所有 token 操作的详细日志

### 3. 部署安全
- **环境隔离**: 使用 Docker 容器化部署
- **密钥管理**: 使用环境变量管理敏感配置
- **网络隔离**: 限制服务间的网络访问

## 部署和维护

### 1. 部署步骤
1. 构建 Docker 镜像
2. 配置环境变量
3. 启动数据库服务
4. 启动 token 服务
5. 配置 TeslaMate 连接

### 2. 监控和维护
- **健康检查**: `/health` 端点监控服务状态
- **指标收集**: 集成 Prometheus 监控
- **日志管理**: 结构化日志输出
- **备份策略**: 定期备份 token 数据库

### 3. 故障恢复
- **自动重试**: Token 刷新失败时的重试机制
- **降级策略**: 服务不可用时的备用方案
- **数据恢复**: 从备份恢复 token 数据的流程

## 预期收益

1. **安全性提升**: 集中管理 Tesla token，减少泄露风险
2. **可维护性**: 独立服务便于维护和升级
3. **可扩展性**: 支持多用户和多车辆管理
4. **可靠性**: 自动 token 刷新和故障恢复
5. **监控能力**: 完整的访问日志和监控指标

## 总结

该方案在技术上完全可行，通过 Gin 框架构建的 token 管理服务可以有效地：

1. ✅ 安全存储 Tesla API 的 Access Token 和 Refresh Token
2. ✅ 提供 RESTful API 供 TeslaMate 验证 token 有效性
3. ✅ 自动处理 token 刷新逻辑
4. ✅ 支持多用户环境
5. ✅ 提供完整的安全和监控机制

建议优先实现核心功能（token 存储、验证、刷新），然后逐步添加监控、日志和高级安全特性。