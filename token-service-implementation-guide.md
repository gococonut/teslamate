# TeslaMate Token 服务实现方案

## 一、项目概述

开发一个简单的 Gin 服务，用于保存和验证 TeslaMate 使用的 Tesla API Token。

## 二、核心功能

### 2.1 功能需求
1. **保存 Token**：接收并存储 access_token 和 refresh_token
2. **验证 Token**：检查 token 是否有效（是否存在、是否过期）
3. **获取 Token**：TeslaMate 需要时可以获取存储的 token

### 2.2 不需要实现的功能
- ❌ 自动刷新 token（TeslaMate 会自行处理）
- ❌ 与 Tesla API 的直接交互
- ❌ 复杂的 token 管理逻辑

## 三、技术架构

### 3.1 技术栈
- **框架**：Gin (Go Web Framework)
- **数据库**：PostgreSQL（与 TeslaMate 共享）
- **加密**：AES-256 加密存储 token

### 3.2 数据库设计

```sql
-- 简单的 token 存储表
CREATE TABLE tesla_tokens (
    id SERIAL PRIMARY KEY,
    access_token TEXT NOT NULL,      -- 加密存储
    refresh_token TEXT NOT NULL,     -- 加密存储
    expires_at TIMESTAMP,            -- token 过期时间
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## 四、API 接口设计

### 4.1 保存 Token
**接口**：`POST /api/v1/token`

**请求头**：
```
X-API-Key: {api_key}
Content-Type: application/json
```

**请求体**：
```json
{
  "access_token": "eyJhbGciOiJI...",
  "refresh_token": "eyJhbGciOiJI...",
  "expires_in": 28800  // 可选，秒数
}
```

**响应**：
```json
{
  "success": true,
  "message": "Token saved successfully"
}
```

### 4.2 验证 Token 有效性
**接口**：`GET /api/v1/token/validate`

**请求头**：
```
X-API-Key: {api_key}
```

**响应**：
```json
{
  "valid": true,
  "expires_at": "2024-01-20T08:00:00Z"
}
```

### 4.3 获取 Token（供 TeslaMate 使用）
**接口**：`GET /api/v1/token`

**请求头**：
```
X-API-Key: {api_key}
```

**响应**：
```json
{
  "access_token": "eyJhbGciOiJI...",
  "refresh_token": "eyJhbGciOiJI...",
  "expires_at": "2024-01-20T08:00:00Z"
}
```

## 五、实现要点

### 5.1 Token 加密
```go
// 使用环境变量中的密钥进行 AES 加密
// ENCRYPTION_KEY 必须是 32 字节
```

### 5.2 API 认证
```go
// 简单的 API Key 验证中间件
// API_KEYS 从环境变量读取，支持多个 key
```

### 5.3 错误处理
- Token 不存在：返回 404
- Token 已过期：返回 valid: false
- API Key 无效：返回 401

## 六、环境配置

```env
# 数据库连接
DATABASE_URL=postgresql://teslamate:password@localhost/teslamate

# 加密密钥（32字节）
ENCRYPTION_KEY=your-32-byte-encryption-key-here

# API 密钥
API_KEYS=key1,key2

# 服务端口
PORT=8080
```

## 七、部署建议

### 7.1 Docker 部署
```dockerfile
FROM golang:1.21-alpine AS builder
# ... 构建步骤

FROM alpine:latest
# ... 运行配置
```

### 7.2 与 TeslaMate 集成
1. 使用同一个 PostgreSQL 实例
2. 在同一个 Docker 网络中
3. TeslaMate 通过内部网络调用 Token 服务

## 八、安全考虑

1. **Token 加密存储**：所有 token 使用 AES-256 加密
2. **API 访问控制**：使用 API Key 保护所有端点
3. **网络隔离**：建议只在内网访问，不暴露到公网
4. **HTTPS**：如果需要公网访问，必须使用 HTTPS

## 九、项目结构建议

```
token-service/
├── main.go              # 主入口
├── config/             # 配置管理
├── handlers/           # API 处理器
├── middleware/         # 中间件（API Key 验证）
├── models/            # 数据模型
├── services/          # 业务逻辑（加密、数据库操作）
├── .env.example       # 环境变量示例
├── Dockerfile         # Docker 构建文件
└── docker-compose.yml # Docker Compose 配置
```

## 十、注意事项

1. **保持简单**：这是一个纯粹的 token 存储服务，不要添加过多功能
2. **与 TeslaMate 解耦**：不要依赖 TeslaMate 的内部实现
3. **日志记录**：记录关键操作，便于排查问题
4. **性能**：这是一个低频访问的服务，不需要过度优化

## 十一、测试建议

1. 单元测试：加密/解密功能
2. 集成测试：API 端点测试
3. 与 TeslaMate 的集成测试

---

这个方案保持了最小化和简单性，完全满足 TeslaMate 的需求，同时易于开发和维护。