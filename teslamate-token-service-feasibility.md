# TeslaMate Token 服务可行性分析与实现方案

## 一、项目背景

TeslaMate 是一个开源的 Tesla 数据记录器，需要通过 Tesla API 的 Access Token 和 Refresh Token 来获取车辆数据。由于 Tesla API 的认证机制和 Token 管理的重要性，需要开发一个专门的服务来安全地存储和管理这些 Token。

## 二、可行性分析

### 2.1 技术可行性

**✅ 可行**

1. **Gin 框架适用性**
   - Gin 是高性能的 Go Web 框架，适合构建 RESTful API
   - 支持中间件机制，便于实现认证和授权
   - 有成熟的生态系统和社区支持

2. **Token 存储方案**
   - 可以使用 PostgreSQL（与 TeslaMate 保持一致）
   - 支持加密存储，保障 Token 安全性
   - 可实现 Token 的版本管理和历史记录

3. **Tesla API 兼容性**
   - Tesla 使用标准的 OAuth2 认证流程
   - Access Token 有效期约 8 小时
   - Refresh Token 有效期约 45 天
   - 可通过 Refresh Token 获取新的 Access Token

### 2.2 安全性评估

1. **Token 加密存储**
   - 使用 AES-256 加密算法
   - 参考 TeslaMate 1.27.0+ 版本的 ENCRYPTION_KEY 机制

2. **API 访问控制**
   - 实现 API Key 或 JWT 认证
   - 支持 IP 白名单
   - 请求频率限制

3. **传输安全**
   - 强制使用 HTTPS
   - 实现请求签名验证

### 2.3 与 TeslaMate 集成

1. **数据库兼容**
   - 可以共享 PostgreSQL 实例
   - 独立的 tokens 表，避免影响 TeslaMate 核心功能

2. **Token 更新机制**
   - 提供 Webhook 通知 TeslaMate Token 更新
   - 或由 TeslaMate 定期轮询检查 Token 状态

## 三、系统架构设计

### 3.1 整体架构

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Tesla API     │     │  Token Service  │     │   TeslaMate     │
│                 │◄────│   (Gin)         │────►│                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                               │
                               ▼
                        ┌─────────────────┐
                        │   PostgreSQL    │
                        │   Database      │
                        └─────────────────┘
```

### 3.2 数据模型设计

#### tokens 表结构
```sql
CREATE TABLE tokens (
    id SERIAL PRIMARY KEY,
    account_id VARCHAR(255) NOT NULL UNIQUE,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    token_type VARCHAR(50) DEFAULT 'Bearer',
    scope TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_encrypted BOOLEAN DEFAULT true,
    encryption_version INTEGER DEFAULT 1
);

CREATE INDEX idx_tokens_account_id ON tokens(account_id);
CREATE INDEX idx_tokens_expires_at ON tokens(expires_at);
```

#### token_audit_logs 表结构
```sql
CREATE TABLE token_audit_logs (
    id SERIAL PRIMARY KEY,
    account_id VARCHAR(255) NOT NULL,
    action VARCHAR(50) NOT NULL, -- 'created', 'refreshed', 'validated', 'expired'
    ip_address INET,
    user_agent TEXT,
    success BOOLEAN DEFAULT true,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 3.3 API 设计

#### 1. 保存/更新 Token
```
POST /api/v1/tokens
Headers:
  X-API-Key: {api_key}
  Content-Type: application/json

Request Body:
{
  "account_id": "tesla_account_email@example.com",
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 28800,  // 秒
  "token_type": "Bearer",
  "scope": "openid email offline_access"
}

Response:
{
  "success": true,
  "message": "Token saved successfully",
  "data": {
    "account_id": "tesla_account_email@example.com",
    "expires_at": "2024-01-20T08:00:00Z"
  }
}
```

#### 2. 获取 Token
```
GET /api/v1/tokens/{account_id}
Headers:
  X-API-Key: {api_key}

Response:
{
  "success": true,
  "data": {
    "account_id": "tesla_account_email@example.com",
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_at": "2024-01-20T08:00:00Z",
    "token_type": "Bearer"
  }
}
```

#### 3. 验证 Token 有效性
```
POST /api/v1/tokens/validate
Headers:
  X-API-Key: {api_key}
  Content-Type: application/json

Request Body:
{
  "account_id": "tesla_account_email@example.com",
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}

Response:
{
  "success": true,
  "data": {
    "valid": true,
    "expires_at": "2024-01-20T08:00:00Z",
    "remaining_seconds": 14400
  }
}
```

#### 4. 刷新 Token
```
POST /api/v1/tokens/refresh
Headers:
  X-API-Key: {api_key}
  Content-Type: application/json

Request Body:
{
  "account_id": "tesla_account_email@example.com"
}

Response:
{
  "success": true,
  "data": {
    "access_token": "new_access_token...",
    "expires_at": "2024-01-20T16:00:00Z"
  }
}
```

## 四、核心功能实现

### 4.1 Token 加密存储

```go
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "errors"
    "io"
)

type TokenEncryptor struct {
    key []byte
}

func NewTokenEncryptor(key string) (*TokenEncryptor, error) {
    if len(key) != 32 {
        return nil, errors.New("encryption key must be 32 bytes")
    }
    return &TokenEncryptor{key: []byte(key)}, nil
}

func (e *TokenEncryptor) Encrypt(plaintext string) (string, error) {
    block, err := aes.NewCipher(e.key)
    if err != nil {
        return "", err
    }

    plaintextBytes := []byte(plaintext)
    ciphertext := make([]byte, aes.BlockSize+len(plaintextBytes))
    iv := ciphertext[:aes.BlockSize]
    
    if _, err := io.ReadFull(rand.Reader, iv); err != nil {
        return "", err
    }

    stream := cipher.NewCFBEncrypter(block, iv)
    stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintextBytes)

    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (e *TokenEncryptor) Decrypt(ciphertext string) (string, error) {
    data, err := base64.StdEncoding.DecodeString(ciphertext)
    if err != nil {
        return "", err
    }

    block, err := aes.NewCipher(e.key)
    if err != nil {
        return "", err
    }

    if len(data) < aes.BlockSize {
        return "", errors.New("ciphertext too short")
    }

    iv := data[:aes.BlockSize]
    data = data[aes.BlockSize:]

    stream := cipher.NewCFBDecrypter(block, iv)
    stream.XORKeyStream(data, data)

    return string(data), nil
}
```

### 4.2 Token 验证中间件

```go
package middleware

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

func APIKeyAuth(validKeys []string) gin.HandlerFunc {
    return func(c *gin.Context) {
        apiKey := c.GetHeader("X-API-Key")
        
        if apiKey == "" {
            c.JSON(http.StatusUnauthorized, gin.H{
                "success": false,
                "error": "API key is required",
            })
            c.Abort()
            return
        }

        isValid := false
        for _, key := range validKeys {
            if apiKey == key {
                isValid = true
                break
            }
        }

        if !isValid {
            c.JSON(http.StatusUnauthorized, gin.H{
                "success": false,
                "error": "Invalid API key",
            })
            c.Abort()
            return
        }

        c.Next()
    }
}
```

### 4.3 Token 刷新逻辑

```go
package service

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type TeslaTokenService struct {
    httpClient *http.Client
    encryptor  *TokenEncryptor
}

func (s *TeslaTokenService) RefreshToken(refreshToken string) (*TokenResponse, error) {
    requestBody := map[string]string{
        "grant_type":    "refresh_token",
        "refresh_token": refreshToken,
        "client_id":     "ownerapi",
        "scope":         "openid email offline_access",
    }

    jsonBody, _ := json.Marshal(requestBody)
    
    req, err := http.NewRequest("POST", "https://auth.tesla.com/oauth2/v3/token", 
        bytes.NewBuffer(jsonBody))
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

    var tokenResp TokenResponse
    if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
        return nil, err
    }

    return &tokenResp, nil
}
```

## 五、部署方案

### 5.1 Docker 部署

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/main .
COPY --from=builder /app/config ./config

EXPOSE 8080
CMD ["./main"]
```

### 5.2 docker-compose.yml

```yaml
version: '3.8'

services:
  token-service:
    build: .
    container_name: teslamate-token-service
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgresql://teslamate:secret@postgres/teslamate
      - ENCRYPTION_KEY=${ENCRYPTION_KEY}
      - API_KEYS=${API_KEYS}
      - GIN_MODE=release
    depends_on:
      - postgres
    networks:
      - teslamate-network

  postgres:
    image: postgres:15
    container_name: teslamate-postgres
    restart: unless-stopped
    environment:
      - POSTGRES_USER=teslamate
      - POSTGRES_PASSWORD=secret
      - POSTGRES_DB=teslamate
    volumes:
      - postgres-data:/var/lib/postgresql/data
    networks:
      - teslamate-network

volumes:
  postgres-data:

networks:
  teslamate-network:
    external: true
```

## 六、安全建议

1. **环境变量管理**
   - 使用 `.env` 文件管理敏感配置
   - 生产环境使用密钥管理服务（如 AWS Secrets Manager）

2. **访问控制**
   - 实施 IP 白名单
   - 使用反向代理（如 Nginx）添加额外的安全层
   - 实现请求频率限制

3. **监控和日志**
   - 记录所有 Token 操作的审计日志
   - 设置 Token 即将过期的告警
   - 监控异常访问模式

4. **备份策略**
   - 定期备份加密的 Token 数据
   - 实现 Token 恢复机制

## 七、与 TeslaMate 集成建议

1. **共享数据库方案**
   - 在 TeslaMate 的 PostgreSQL 实例中创建独立的 schema
   - 避免直接修改 TeslaMate 的表结构

2. **Token 同步机制**
   - Token Service 提供 Webhook，在 Token 更新时通知 TeslaMate
   - 或 TeslaMate 通过 API 定期检查 Token 状态

3. **配置示例**
   ```yaml
   # TeslaMate 配置
   TOKEN_SERVICE_URL: http://token-service:8080
   TOKEN_SERVICE_API_KEY: your-api-key
   TOKEN_CHECK_INTERVAL: 300  # 5分钟检查一次
   ```

## 八、总结

基于以上分析，使用 Gin 框架开发 TeslaMate Token 管理服务是**完全可行的**。该方案具有以下优势：

1. ✅ **技术成熟**：Go + Gin 的组合性能优异，适合构建高并发 API 服务
2. ✅ **安全可靠**：支持 Token 加密存储和完善的访问控制
3. ✅ **易于集成**：提供标准的 RESTful API，便于与 TeslaMate 集成
4. ✅ **可扩展性**：预留了审计日志、Webhook 通知等扩展点

建议按照本方案进行开发，并在实施过程中根据实际需求进行适当调整。