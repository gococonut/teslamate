# TeslaMate Token 服务实现方案 V2

## 一、核心需求

开发一个 Gin 服务，在保存 Token 前先验证其有效性，只有有效的 Token 才能保存。

## 二、Token 验证流程

### 2.1 验证步骤
1. **接收 Token**：接收 access_token 和 refresh_token
2. **验证有效性**：调用 Tesla API 验证 token 是否真实有效
3. **保存 Token**：只有验证通过的 token 才保存到数据库
4. **返回结果**：告知客户端 token 是否有效并已保存

### 2.2 Tesla API 验证方法

使用 Tesla Owner API 验证 token：
```
GET https://owner-api.teslamotors.com/api/1/vehicles
Headers:
  Authorization: Bearer {access_token}
```

- **200 OK**：Token 有效
- **401 Unauthorized**：Token 无效或过期
- **其他错误**：网络或服务器问题

## 三、API 设计

### 3.1 保存 Token（带验证）
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
  "refresh_token": "eyJhbGciOiJI..."
}
```

**处理流程**：
1. 验证 API Key
2. 调用 Tesla API 验证 access_token
3. 如果有效，加密并保存到数据库
4. 如果无效，直接返回错误

**成功响应**：
```json
{
  "success": true,
  "message": "Token validated and saved successfully",
  "data": {
    "valid": true,
    "expires_at": "2024-01-20T08:00:00Z"
  }
}
```

**失败响应**：
```json
{
  "success": false,
  "error": "Invalid token",
  "data": {
    "valid": false,
    "reason": "Tesla API returned 401 Unauthorized"
  }
}
```

### 3.2 验证已保存的 Token
**接口**：`GET /api/v1/token/validate`

**请求头**：
```
X-API-Key: {api_key}
```

**响应**：
```json
{
  "valid": true,
  "expires_at": "2024-01-20T08:00:00Z",
  "stored": true  // 表示 token 已存储在系统中
}
```

## 四、实现细节

### 4.1 Token 验证服务
```go
// 伪代码示例
type TeslaValidator struct {
    httpClient *http.Client
}

func (v *TeslaValidator) ValidateToken(accessToken string) (bool, error) {
    // 调用 Tesla API
    // GET https://owner-api.teslamotors.com/api/1/vehicles
    // 检查响应状态码
    // 200 = 有效, 401 = 无效
}
```

### 4.2 保存流程
```go
// 伪代码示例
func SaveTokenHandler(c *gin.Context) {
    // 1. 解析请求
    var req TokenRequest
    
    // 2. 验证 token
    valid, err := teslaValidator.ValidateToken(req.AccessToken)
    if !valid {
        // 返回错误，不保存
        return
    }
    
    // 3. 加密 token
    encryptedAccess := encrypt(req.AccessToken)
    encryptedRefresh := encrypt(req.RefreshToken)
    
    // 4. 保存到数据库
    saveToDatabase(encryptedAccess, encryptedRefresh)
    
    // 5. 返回成功
}
```

### 4.3 数据库使用

**重要**：TeslaMate 使用加密方式存储 token，我们的服务应该与 TeslaMate 的存储方式保持一致。

**方案一：直接使用 TeslaMate 的 token 存储**
- 查看 TeslaMate 的数据库表结构
- 复用 TeslaMate 的 token 存储逻辑
- 使用相同的 ENCRYPTION_KEY 进行加密/解密

**方案二：创建独立的 token 表**
```sql
-- 如果需要独立管理，创建新表
CREATE TABLE IF NOT EXISTS api_tokens (
    id SERIAL PRIMARY KEY,
    access_token TEXT NOT NULL,      -- 加密存储
    refresh_token TEXT NOT NULL,     -- 加密存储
    validated_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 审计日志表（可选）
CREATE TABLE IF NOT EXISTS token_validation_logs (
    id SERIAL PRIMARY KEY,
    access_token_hash VARCHAR(64),
    valid BOOLEAN NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**注意**：
- 必须使用与 TeslaMate 相同的 ENCRYPTION_KEY
- 确保加密方式与 TeslaMate 兼容
- 建议先调研 TeslaMate 的具体实现再决定

## 五、Tesla API 集成

### 5.1 请求示例
```http
GET https://owner-api.teslamotors.com/api/1/vehicles
Authorization: Bearer eyJhbGciOiJI...
User-Agent: TeslaMateTokenService/1.0
```

### 5.2 响应处理
- **成功响应**（200 OK）：
  ```json
  {
    "response": [
      {
        "id": 123456789,
        "vehicle_id": 987654321,
        "vin": "5YJ3E1EA1JF000000",
        "display_name": "My Tesla"
      }
    ],
    "count": 1
  }
  ```

- **失败响应**（401 Unauthorized）：
  ```json
  {
    "error": "unauthorized",
    "error_description": "Invalid bearer token"
  }
  ```

### 5.3 网络超时处理
- 设置合理的超时时间（建议 30 秒）
- 网络错误时返回明确的错误信息
- 不要因为网络问题就认为 token 无效

## 六、安全考虑

### 6.1 Token 处理
1. **不记录原始 Token**：日志中只记录 token 的 hash 值
2. **加密存储**：使用 AES-256 加密存储在数据库中
3. **传输安全**：强制使用 HTTPS（如果对外暴露）

### 6.2 请求限制
1. **频率限制**：限制每个 API Key 的请求频率
2. **验证失败限制**：连续多次验证失败后暂时封禁

## 七、错误处理

### 7.1 错误类型
1. **Token 无效**：Tesla API 返回 401
2. **网络错误**：无法连接 Tesla API
3. **服务器错误**：Tesla API 返回 5xx
4. **请求格式错误**：缺少必要字段

### 7.2 错误响应格式
```json
{
  "success": false,
  "error": "错误类型",
  "message": "详细错误信息",
  "code": "ERROR_CODE"
}
```

## 八、环境配置

```env
# 数据库 - 使用 TeslaMate 的数据库连接
DATABASE_URL=postgresql://teslamate:password@localhost/teslamate

# 加密
ENCRYPTION_KEY=32-byte-key-for-aes-256-encryption

# API 安全
API_KEYS=key1,key2

# Tesla API
TESLA_API_BASE_URL=https://owner-api.teslamotors.com
TESLA_API_TIMEOUT=30

# 服务配置
PORT=8080
GIN_MODE=release
```

## 九、监控和日志

### 9.1 需要记录的信息
1. **验证成功**：记录时间、token hash
2. **验证失败**：记录时间、失败原因
3. **API 调用**：记录请求来源、时间

### 9.2 监控指标
1. Token 验证成功率
2. Tesla API 响应时间
3. 系统错误率

## 十、与 TeslaMate 集成

1. **数据库共享**：
   - 使用 TeslaMate 的 PostgreSQL 实例
   - 复用 TeslaMate 的数据库连接配置
   - 查看并使用 TeslaMate 现有的 token 存储机制

2. **TeslaMate 调用流程**：
   - 用户获取 token（通过第三方工具）
   - 调用本服务保存 token（自动验证）
   - TeslaMate 从数据库读取已验证的 token

3. **失败处理**：
   - 如果 token 无效，用户需要重新获取
   - 本服务只存储有效的 token

---

**关键点**：
- ✅ 保存前必须验证 token 有效性
- ✅ 只有通过 Tesla API 验证的 token 才能保存
- ✅ 提供清晰的错误信息帮助用户排查问题
- ✅ 安全地存储和处理 token