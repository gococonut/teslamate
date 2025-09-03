# Tesla Token Service

一个基于 Gin 框架的 Tesla API Token 管理服务，专为 TeslaMate 设计，用于安全存储和管理 Tesla API 的访问令牌和刷新令牌。

## 功能特性

- 🔐 安全存储 Tesla API Access Token 和 Refresh Token
- 🔄 自动 Token 刷新机制
- ✅ Token 有效性验证
- 📊 完整的使用日志记录
- 🐳 Docker 容器化部署
- 🛡️ JWT 认证保护
- 📈 健康检查和监控

## 快速开始

### 1. 使用 Docker Compose（推荐）

```bash
# 克隆项目
git clone <repository-url>
cd tesla-token-service

# 复制并配置环境变量
cp .env.example .env
# 编辑 .env 文件，设置数据库密码和 JWT 密钥

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f tesla-token-service
```

### 2. 本地开发

```bash
# 安装依赖
go mod tidy

# 设置环境变量
export DB_HOST=localhost
export DB_USER=tesla_user
export DB_PASSWORD=your_password
export DB_NAME=tesla_tokens
export JWT_SECRET=your_jwt_secret

# 运行服务
go run cmd/server/main.go
```

## API 接口文档

### 1. 保存 Token

```http
POST /api/v1/tokens
Content-Type: application/json

{
    "user_id": "tesla_user_123",
    "access_token": "eyJ0eXAiOiJKV1QiLCJhbGc...",
    "refresh_token": "eyJ0eXAiOiJKV1QiLCJhbGc...",
    "expires_at": "2024-01-15T10:30:00Z"
}
```

**响应:**
```json
{
    "success": true,
    "message": "Token saved successfully",
    "user_id": "tesla_user_123"
}
```

### 2. 获取 Token

```http
GET /api/v1/tokens/{user_id}
Authorization: Bearer {jwt_token}
```

**响应:**
```json
{
    "access_token": "eyJ0eXAiOiJKV1QiLCJhbGc...",
    "token_type": "Bearer",
    "expires_at": "2024-01-15T10:30:00Z"
}
```

### 3. 验证 Token

```http
GET /api/v1/tokens/{user_id}/validate
Authorization: Bearer {jwt_token}
```

**响应:**
```json
{
    "valid": true,
    "expires_at": "2024-01-15T10:30:00Z",
    "user_id": "tesla_user_123",
    "message": "Token is valid"
}
```

### 4. 刷新 Token

```http
POST /api/v1/tokens/{user_id}/refresh
Authorization: Bearer {jwt_token}
```

**响应:**
```json
{
    "success": true,
    "access_token": "new_access_token...",
    "refresh_token": "new_refresh_token...",
    "token_type": "Bearer",
    "expires_at": "2024-01-15T11:30:00Z"
}
```

### 5. 删除 Token

```http
DELETE /api/v1/tokens/{user_id}
Authorization: Bearer {jwt_token}
```

### 6. 健康检查

```http
GET /health
```

**响应:**
```json
{
    "status": "healthy",
    "service": "tesla-token-service",
    "timestamp": "2024-01-15T09:30:00Z",
    "version": "1.0.0"
}
```

## 与 TeslaMate 集成

### 1. 环境变量配置

在 TeslaMate 的 docker-compose.yml 中添加：

```yaml
services:
  teslamate:
    # ... 其他配置
    environment:
      - TESLA_TOKEN_SERVICE_URL=http://tesla-token-service:8080
      - TESLA_TOKEN_SERVICE_JWT=your_internal_jwt_token
      - TESLA_USER_ID=tesla_user_123
```

### 2. TeslaMate 代码集成示例

```elixir
# 在 TeslaMate 中添加 token 服务客户端
defmodule TeslaMate.Auth.TokenServiceClient do
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
      {:ok, %{status_code: status}} -> 
        {:error, {:http_error, status}}
      {:error, reason} -> 
        {:error, reason}
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

## 部署说明

### 1. 生产环境部署

```bash
# 1. 构建镜像
docker build -t tesla-token-service:latest .

# 2. 运行服务
docker run -d \
  --name tesla-token-service \
  -p 8080:8080 \
  -e DB_HOST=your_db_host \
  -e DB_USER=your_db_user \
  -e DB_PASSWORD=your_db_password \
  -e DB_NAME=tesla_tokens \
  -e JWT_SECRET=your_production_jwt_secret \
  tesla-token-service:latest
```

### 2. 数据库初始化

服务启动时会自动创建必要的数据库表。如果需要手动初始化：

```bash
# 连接到 MySQL 数据库
mysql -h localhost -u tesla_user -p tesla_tokens

# 执行初始化脚本
source scripts/init.sql;
```

### 3. 生成内部 JWT Token

```bash
# 使用提供的工具生成内部 JWT token
go run scripts/generate_jwt.go --secret="your_jwt_secret" --subject="teslamate" --duration="8760h"
```

## 安全注意事项

1. **更改默认密钥**: 生产环境中必须更改 `JWT_SECRET` 和数据库密码
2. **使用 HTTPS**: 生产环境中强制使用 HTTPS
3. **网络隔离**: 将服务部署在私有网络中，仅允许 TeslaMate 访问
4. **定期备份**: 定期备份 token 数据库
5. **监控日志**: 监控异常访问和失败尝试

## 监控和维护

### 健康检查
```bash
curl http://localhost:8080/health
```

### 查看日志
```bash
# Docker 环境
docker-compose logs -f tesla-token-service

# 本地环境
tail -f logs/tesla-token-service.log
```

### 数据库维护
```bash
# 清理过期 token（可设置定时任务）
curl -X POST http://localhost:8080/api/v1/admin/cleanup \
  -H "Authorization: Bearer your_admin_jwt"
```

## 故障排除

### 常见问题

1. **数据库连接失败**
   - 检查数据库服务是否运行
   - 验证连接参数是否正确
   - 确认网络连通性

2. **Token 刷新失败**
   - 检查 Tesla API 是否可访问
   - 验证 Refresh Token 是否有效
   - 查看详细错误日志

3. **JWT 认证失败**
   - 确认 JWT_SECRET 配置正确
   - 检查 JWT token 是否过期
   - 验证 Authorization 头格式

## 开发指南

### 项目结构

```
tesla-token-service/
├── cmd/server/          # 主程序入口
├── internal/
│   ├── config/         # 配置管理
│   ├── database/       # 数据库连接
│   ├── handler/        # HTTP 处理器
│   ├── middleware/     # 中间件
│   ├── model/          # 数据模型
│   └── service/        # 业务逻辑
├── scripts/            # 脚本文件
├── docker-compose.yml  # Docker 编排
├── Dockerfile         # Docker 镜像构建
├── go.mod             # Go 模块
└── README.md          # 项目文档
```

### 添加新功能

1. 在 `internal/model/` 中定义数据模型
2. 在 `internal/service/` 中实现业务逻辑
3. 在 `internal/handler/` 中添加 HTTP 处理器
4. 在 `cmd/server/main.go` 中注册路由

### 运行测试

```bash
go test ./...
```

## 许可证

MIT License