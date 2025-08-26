# TeslaMate Token 保存实现指南

## 概述

本文档详细说明如何使用 Golang Gin 框架实现与 TeslaMate 完全兼容的 Token 保存功能。通过严格遵循 TeslaMate 的加密和存储逻辑，确保保存的 Token 能被 TeslaMate 正确识别和使用。

## TeslaMate Token 处理机制分析

### 1. 加密算法详解

TeslaMate 使用 Cloak 库实现 AES-256-GCM 加密，具体参数：

- **加密算法**: AES-256-GCM
- **密钥长度**: 256 位（32 字节）
- **IV 长度**: 12 字节（固定）
- **认证标签长度**: 16 字节（GCM 默认）
- **密钥派生**: SHA256(ENCRYPTION_KEY)

### 2. 加密数据格式

```
完整加密数据结构:
+------------------+---------------+-----------------------------------+
| Key Tag Header   | IV (12 bytes) | Ciphertext + Auth Tag (n+16 bytes)|
+------------------+---------------+-----------------------------------+

Key Tag Header 结构:
+---------------+-----------------+---------------------+
| Type (1 byte) | Length (1 byte) | Value ("AES.GCM.V1")|
+---------------+-----------------+---------------------+
```

具体说明：
- **Type**: 固定值 0x01
- **Length**: Key Tag Value 的长度（10 字节）
- **Value**: 固定字符串 "AES.GCM.V1"
- **完整 Key Tag**: [0x01, 0x0A, 0x41, 0x45, 0x53, 0x2E, 0x47, 0x43, 0x4D, 0x2E, 0x56, 0x31]

### 3. 数据库存储格式

- 表名: `tokens`
- 字段类型: PostgreSQL `bytea`
- 存储内容: 完整的加密数据（包含 Key Tag Header）

## Golang 实现

### 1. 项目结构

```
teslamate-token-service/
├── main.go
├── internal/
│   ├── crypto/
│   │   └── vault.go        # 加密实现
│   ├── models/
│   │   └── token.go        # 数据模型
│   └── services/
│       └── token_service.go # 业务逻辑
├── config/
│   └── config.go           # 配置管理
├── go.mod
├── go.sum
└── README.md
```

### 2. 核心加密实现

```go
// internal/crypto/vault.go
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "crypto/sha256"
    "errors"
)

const (
    // TeslaMate Cloak 配置常量
    KeyTagType   = 0x01
    KeyTagValue  = "AES.GCM.V1"
    IVLength     = 12
    TagLength    = 16
)

type Vault struct {
    key []byte
}

// NewVault 创建新的加密保管库
func NewVault(encryptionKey string) (*Vault, error) {
    if encryptionKey == "" {
        return nil, errors.New("encryption key cannot be empty")
    }
    
    // 严格遵循 TeslaMate: SHA256(ENCRYPTION_KEY)
    hash := sha256.Sum256([]byte(encryptionKey))
    
    return &Vault{
        key: hash[:],
    }, nil
}

// Encrypt 加密数据，严格遵循 TeslaMate/Cloak 格式
func (v *Vault) Encrypt(plaintext string) ([]byte, error) {
    // 1. 创建 AES-256 cipher
    block, err := aes.NewCipher(v.key)
    if err != nil {
        return nil, err
    }
    
    // 2. 创建 GCM mode
    aesGCM, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    // 3. 生成随机 IV (12 bytes)
    iv := make([]byte, IVLength)
    if _, err := rand.Read(iv); err != nil {
        return nil, err
    }
    
    // 4. 加密数据 (包含 16 字节的认证标签)
    ciphertext := aesGCM.Seal(nil, iv, []byte(plaintext), nil)
    
    // 5. 构建 Key Tag Header
    keyTag := v.buildKeyTag()
    
    // 6. 组装最终数据: KeyTag + IV + Ciphertext(含AuthTag)
    result := make([]byte, 0, len(keyTag)+IVLength+len(ciphertext))
    result = append(result, keyTag...)
    result = append(result, iv...)
    result = append(result, ciphertext...)
    
    return result, nil
}

// buildKeyTag 构建 TeslaMate 兼容的 Key Tag
func (v *Vault) buildKeyTag() []byte {
    value := []byte(KeyTagValue)
    keyTag := []byte{
        KeyTagType,        // Type: 0x01
        byte(len(value)),  // Length: 0x0A (10)
    }
    keyTag = append(keyTag, value...) // Value: "AES.GCM.V1"
    return keyTag
}

// 用于验证的解密函数（可选）
func (v *Vault) Decrypt(encrypted []byte) (string, error) {
    // 验证最小长度
    minLen := 2 + len(KeyTagValue) + IVLength + TagLength
    if len(encrypted) < minLen {
        return "", errors.New("invalid encrypted data length")
    }
    
    // 解析 Key Tag
    pos := 0
    if encrypted[pos] != KeyTagType {
        return "", errors.New("invalid key tag type")
    }
    pos++
    
    tagLen := int(encrypted[pos])
    pos++
    
    if tagLen != len(KeyTagValue) {
        return "", errors.New("invalid key tag length")
    }
    
    tag := string(encrypted[pos : pos+tagLen])
    if tag != KeyTagValue {
        return "", errors.New("invalid key tag value")
    }
    pos += tagLen
    
    // 提取 IV
    iv := encrypted[pos : pos+IVLength]
    pos += IVLength
    
    // 提取密文（包含认证标签）
    ciphertext := encrypted[pos:]
    
    // 解密
    block, err := aes.NewCipher(v.key)
    if err != nil {
        return "", err
    }
    
    aesGCM, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    
    plaintext, err := aesGCM.Open(nil, iv, ciphertext, nil)
    if err != nil {
        return "", err
    }
    
    return string(plaintext), nil
}
```

### 3. 数据库模型

```go
// internal/models/token.go
package models

import (
    "time"
)

type Token struct {
    ID        int       `db:"id"`
    Access    []byte    `db:"access"`
    Refresh   []byte    `db:"refresh"`
    CreatedAt time.Time `db:"created_at"`
    UpdatedAt time.Time `db:"updated_at"`
}
```

### 4. Token 服务实现

```go
// internal/services/token_service.go
package services

import (
    "database/sql"
    "fmt"
    "time"
    
    "your-module/internal/crypto"
    _ "github.com/lib/pq"
)

type TokenService struct {
    db    *sql.DB
    vault *crypto.Vault
}

func NewTokenService(db *sql.DB, encryptionKey string) (*TokenService, error) {
    vault, err := crypto.NewVault(encryptionKey)
    if err != nil {
        return nil, err
    }
    
    return &TokenService{
        db:    db,
        vault: vault,
    }, nil
}

// SaveTokens 保存加密的 tokens，严格遵循 TeslaMate 逻辑
func (s *TokenService) SaveTokens(accessToken, refreshToken string) error {
    // 1. 验证输入
    if accessToken == "" || refreshToken == "" {
        return fmt.Errorf("tokens cannot be empty")
    }
    
    // 2. 加密 tokens
    encryptedAccess, err := s.vault.Encrypt(accessToken)
    if err != nil {
        return fmt.Errorf("failed to encrypt access token: %w", err)
    }
    
    encryptedRefresh, err := s.vault.Encrypt(refreshToken)
    if err != nil {
        return fmt.Errorf("failed to encrypt refresh token: %w", err)
    }
    
    // 3. 开始数据库事务
    tx, err := s.db.Begin()
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback()
    
    // 4. 清空现有 tokens（TeslaMate 只允许一条记录）
    _, err = tx.Exec("DELETE FROM tokens")
    if err != nil {
        return fmt.Errorf("failed to delete existing tokens: %w", err)
    }
    
    // 5. 插入新 tokens
    query := `
        INSERT INTO tokens (access, refresh, created_at, updated_at)
        VALUES ($1, $2, $3, $4)
    `
    now := time.Now()
    _, err = tx.Exec(query, encryptedAccess, encryptedRefresh, now, now)
    if err != nil {
        return fmt.Errorf("failed to insert tokens: %w", err)
    }
    
    // 6. 提交事务
    if err = tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }
    
    return nil
}

// VerifyTokens 验证 tokens 是否正确加密（可选）
func (s *TokenService) VerifyTokens() error {
    var access, refresh []byte
    
    query := "SELECT access, refresh FROM tokens LIMIT 1"
    err := s.db.QueryRow(query).Scan(&access, &refresh)
    if err != nil {
        if err == sql.ErrNoRows {
            return fmt.Errorf("no tokens found")
        }
        return err
    }
    
    // 尝试解密验证
    _, err = s.vault.Decrypt(access)
    if err != nil {
        return fmt.Errorf("failed to decrypt access token: %w", err)
    }
    
    _, err = s.vault.Decrypt(refresh)
    if err != nil {
        return fmt.Errorf("failed to decrypt refresh token: %w", err)
    }
    
    return nil
}
```

### 5. Gin 服务实现

```go
// main.go
package main

import (
    "database/sql"
    "log"
    "net/http"
    "os"
    
    "github.com/gin-gonic/gin"
    _ "github.com/lib/pq"
    
    "your-module/internal/services"
)

type TokenRequest struct {
    AccessToken  string `json:"access_token" binding:"required"`
    RefreshToken string `json:"refresh_token" binding:"required"`
}

type Config struct {
    DatabaseURL   string
    EncryptionKey string
    Port          string
}

func loadConfig() (*Config, error) {
    cfg := &Config{
        DatabaseURL:   os.Getenv("DATABASE_URL"),
        EncryptionKey: os.Getenv("ENCRYPTION_KEY"),
        Port:          os.Getenv("PORT"),
    }
    
    if cfg.DatabaseURL == "" {
        cfg.DatabaseURL = "postgres://teslamate:secret@localhost/teslamate?sslmode=disable"
    }
    
    if cfg.EncryptionKey == "" {
        return nil, fmt.Errorf("ENCRYPTION_KEY is required")
    }
    
    if cfg.Port == "" {
        cfg.Port = "8080"
    }
    
    return cfg, nil
}

func main() {
    // 加载配置
    cfg, err := loadConfig()
    if err != nil {
        log.Fatal(err)
    }
    
    // 连接数据库
    db, err := sql.Open("postgres", cfg.DatabaseURL)
    if err != nil {
        log.Fatal("Failed to connect to database:", err)
    }
    defer db.Close()
    
    // 测试数据库连接
    if err := db.Ping(); err != nil {
        log.Fatal("Failed to ping database:", err)
    }
    
    // 创建 token 服务
    tokenService, err := services.NewTokenService(db, cfg.EncryptionKey)
    if err != nil {
        log.Fatal("Failed to create token service:", err)
    }
    
    // 设置 Gin
    if os.Getenv("GIN_MODE") == "" {
        gin.SetMode(gin.ReleaseMode)
    }
    
    r := gin.Default()
    
    // 健康检查
    r.GET("/health", func(c *gin.Context) {
        var count int
        err := db.QueryRow("SELECT COUNT(*) FROM tokens").Scan(&count)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "status": "error",
                "error":  err.Error(),
            })
            return
        }
        
        c.JSON(http.StatusOK, gin.H{
            "status":            "healthy",
            "tokens_configured": count > 0,
            "database":          "connected",
        })
    })
    
    // 保存 tokens
    r.POST("/api/tokens", func(c *gin.Context) {
        var req TokenRequest
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{
                "error": "Invalid request format",
                "details": err.Error(),
            })
            return
        }
        
        // 保存 tokens
        if err := tokenService.SaveTokens(req.AccessToken, req.RefreshToken); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": "Failed to save tokens",
                "details": err.Error(),
            })
            return
        }
        
        c.JSON(http.StatusOK, gin.H{
            "message": "Tokens saved successfully",
            "notice":  "TeslaMate will use these tokens on next startup",
        })
    })
    
    // 验证 tokens（可选）
    r.GET("/api/tokens/verify", func(c *gin.Context) {
        if err := tokenService.VerifyTokens(); err != nil {
            c.JSON(http.StatusOK, gin.H{
                "valid": false,
                "error": err.Error(),
            })
            return
        }
        
        c.JSON(http.StatusOK, gin.H{
            "valid":   true,
            "message": "Tokens are properly encrypted and stored",
        })
    })
    
    log.Printf("Starting server on port %s", cfg.Port)
    if err := r.Run(":" + cfg.Port); err != nil {
        log.Fatal("Failed to start server:", err)
    }
}
```

## 部署指南

### 1. 环境变量配置

```bash
# 必需的环境变量
ENCRYPTION_KEY=your-encryption-key-here  # 必须与 TeslaMate 使用的完全相同！
DATABASE_URL=postgres://teslamate:secret@localhost/teslamate?sslmode=disable

# 可选的环境变量
PORT=8080
GIN_MODE=release
```

### 2. Docker 部署

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o token-service .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/token-service .

EXPOSE 8080
CMD ["./token-service"]
```

### 3. Docker Compose 配置

```yaml
version: '3.8'

services:
  teslamate:
    image: teslamate/teslamate:latest
    restart: always
    environment:
      - DATABASE_USER=teslamate
      - DATABASE_PASS=secret
      - DATABASE_NAME=teslamate
      - DATABASE_HOST=database
      - ENCRYPTION_KEY=your-very-secret-encryption-key-minimum-32-chars
      - MQTT_HOST=mosquitto
    ports:
      - 4000:4000
    volumes:
      - ./import:/opt/app/import
    depends_on:
      - database
      - mosquitto

  token-service:
    build: .
    restart: always
    environment:
      - DATABASE_URL=postgres://teslamate:secret@database/teslamate?sslmode=disable
      - ENCRYPTION_KEY=your-very-secret-encryption-key-minimum-32-chars
      - GIN_MODE=release
    ports:
      - 8080:8080
    depends_on:
      - database

  database:
    image: postgres:14
    restart: always
    environment:
      - POSTGRES_USER=teslamate
      - POSTGRES_PASSWORD=secret
      - POSTGRES_DB=teslamate
    volumes:
      - teslamate-db:/var/lib/postgresql/data

  mosquitto:
    image: eclipse-mosquitto:2
    restart: always
    command: mosquitto -c /mosquitto-no-auth.conf
    volumes:
      - mosquitto-conf:/mosquitto/config
      - mosquitto-data:/mosquitto/data

volumes:
  teslamate-db:
  mosquitto-conf:
  mosquitto-data:
```

## 使用说明

### 1. 首次使用

```bash
# 1. 启动所有服务
docker-compose up -d

# 2. 等待服务就绪
docker-compose logs -f token-service

# 3. 保存 Tesla tokens
curl -X POST http://localhost:8080/api/tokens \
  -H "Content-Type: application/json" \
  -d '{
    "access_token": "your-tesla-access-token",
    "refresh_token": "your-tesla-refresh-token"
  }'

# 4. 验证 tokens（可选）
curl http://localhost:8080/api/tokens/verify

# 5. 重启 TeslaMate 使其加载新 tokens
docker-compose restart teslamate
```

### 2. 验证步骤

1. 查看 TeslaMate 日志：
   ```bash
   docker-compose logs -f teslamate | grep -E "(Refreshing|Starting logger)"
   ```

2. 检查数据库：
   ```sql
   -- 连接到数据库
   docker-compose exec database psql -U teslamate

   -- 检查 tokens
   SELECT encode(access, 'hex'), encode(refresh, 'hex') FROM tokens;
   
   -- 检查车辆
   SELECT * FROM cars;
   ```

3. 访问 TeslaMate Web UI：
   - http://localhost:4000
   - 应该直接看到车辆信息，无需登录

## 重要注意事项

### 1. 安全性

- **加密密钥管理**：
  - 使用强密码（建议至少 32 字符）
  - 妥善保管，丢失后无法恢复
  - 不要在代码中硬编码

- **网络安全**：
  - 生产环境使用 HTTPS
  - 添加认证机制（如 API Key）
  - 限制访问 IP

### 2. 兼容性

- **TeslaMate 版本**：本实现基于 TeslaMate v1.27+ 的加密格式
- **数据库**：仅支持 PostgreSQL
- **加密格式**：必须严格遵循 Cloak 库的格式

### 3. 故障排查

常见问题：

1. **"Could not decrypt API tokens!"**
   - 检查 ENCRYPTION_KEY 是否一致
   - 确认加密格式正确

2. **Token 刷新失败**
   - 验证 tokens 是否有效
   - 检查网络连接

3. **车辆未显示**
   - 等待 TeslaMate 重启后自动发现
   - 检查日志中的错误信息

## 测试建议

1. **单元测试**：测试加密/解密功能
2. **集成测试**：使用测试数据库验证完整流程
3. **端到端测试**：验证 TeslaMate 能正确使用保存的 tokens

## 参考资料

- TeslaMate 源码：`lib/teslamate/vault.ex`、`lib/teslamate/auth.ex`
- Cloak 库文档：https://github.com/danielberkompas/cloak
- Tesla API 文档：https://tesla-api.timdorr.com/