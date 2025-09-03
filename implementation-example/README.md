# TeslaMate Token Service 实现示例

这是一个基于 Gin 框架的 TeslaMate Token 管理服务实现示例。

## 项目结构

```
implementation-example/
├── main.go                 # 主程序入口
├── config/                 # 配置管理
├── controllers/            # API 控制器
├── crypto/                 # 加密工具
├── database/              # 数据库连接和迁移
├── middleware/            # 中间件
├── models/                # 数据模型
├── routes/                # 路由定义
├── services/              # 业务逻辑
├── .env.example           # 环境变量示例
├── Dockerfile             # Docker 构建文件
├── docker-compose.yml     # Docker Compose 配置
└── README.md              # 项目说明
```

## 环境配置

创建 `.env` 文件：

```env
# 数据库配置
DATABASE_URL=postgresql://teslamate:secret@localhost:5432/teslamate?sslmode=disable

# 加密密钥（32字节）
ENCRYPTION_KEY=your-32-byte-encryption-key-here

# API 密钥（逗号分隔）
API_KEYS=api-key-1,api-key-2

# Gin 模式
GIN_MODE=debug

# 服务端口
PORT=8080
```

## 快速开始

1. 安装依赖：
```bash
go mod init teslamate-token-service
go mod tidy
```

2. 运行数据库迁移：
```bash
go run main.go migrate
```

3. 启动服务：
```bash
go run main.go
```

## Docker 部署

```bash
# 构建镜像
docker build -t teslamate-token-service .

# 使用 docker-compose 启动
docker-compose up -d
```

## API 使用示例

### 1. 保存 Token
```bash
curl -X POST http://localhost:8080/api/v1/tokens \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "user@example.com",
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 28800,
    "token_type": "Bearer"
  }'
```

### 2. 获取 Token
```bash
curl -X GET http://localhost:8080/api/v1/tokens/user@example.com \
  -H "X-API-Key: your-api-key"
```

### 3. 验证 Token
```bash
curl -X POST http://localhost:8080/api/v1/tokens/validate \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "user@example.com",
    "access_token": "eyJhbGciOiJIUzI1NiIs..."
  }'
```

### 4. 刷新 Token
```bash
curl -X POST http://localhost:8080/api/v1/tokens/refresh \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "user@example.com"
  }'
```

## 注意事项

1. **加密密钥**：必须是 32 字节的字符串，用于 AES-256 加密
2. **API 密钥**：用于保护 API 端点，建议使用强密码
3. **Token 安全**：所有 Token 在数据库中都是加密存储的
4. **审计日志**：所有 Token 操作都会记录在 `token_audit_logs` 表中

## 开发指南

### 添加新的 API 端点

1. 在 `controllers/` 目录创建新的控制器
2. 在 `routes/` 目录更新路由定义
3. 必要时在 `services/` 目录添加业务逻辑

### 数据库迁移

修改模型后，运行：
```bash
go run main.go migrate
```

### 测试

```bash
go test ./...
```

## 许可证

MIT License