# TeslaMate Token 服务实现方案 V2

## 一、核心需求

开发一个 Gin 服务，在保存 Token 前先验证其有效性，只有有效的 Token 才能保存。

**关键约束**：
- 不创建新的数据库表，完全复用 TeslaMate 的表结构
- 必须在 TeslaMate 启动并创建好数据库表之后才能启动本服务
- 加密/解密方式必须与 TeslaMate 完全一致

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

**重要原则**：
- **不创建新表**，直接使用 TeslaMate 创建的表结构
- **服务启动顺序**：必须等 TeslaMate 完全启动并创建好表结构后，本服务才能启动
- **加密方式**：必须与 TeslaMate 的加密/解密方式完全一致

**实施步骤**：
1. **查看 TeslaMate 表结构**
   - 连接到 TeslaMate 的 PostgreSQL 数据库
   - 使用 `\dt` 查看所有表
   - 使用 `\d+ [表名]` 查看具体的 token 相关表结构
   - 找到 TeslaMate 存储 access_token 和 refresh_token 的具体表和字段

2. **复用 TeslaMate 的加密逻辑**
   - 使用相同的 `ENCRYPTION_KEY` 环境变量
   - 研究 TeslaMate 源码，确认其使用的加密算法（可能是 AES）
   - 实现相同的加密/解密函数

3. **数据库操作**
   - 只进行 SELECT/UPDATE 操作，不创建新表
   - 直接读写 TeslaMate 的 token 表

**注意事项**：
- 本服务必须在 docker-compose 中设置 `depends_on: teslamate`
- 可能需要添加健康检查，确保 TeslaMate 数据库表已创建
- 加密算法必须与 TeslaMate 完全一致，否则无法正确解密

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

### 10.1 启动顺序
```yaml
# docker-compose.yml 示例
services:
  token-service:
    depends_on:
      teslamate:
        condition: service_healthy
    environment:
      - ENCRYPTION_KEY=${ENCRYPTION_KEY}  # 必须与 TeslaMate 相同
```

### 10.2 数据库集成
1. **表结构发现**：
   - 服务启动后，先查询 TeslaMate 的表结构
   - 找到存储 token 的表和字段名
   - 动态适配 TeslaMate 的数据结构

2. **加密兼容性**：
   - 使用相同的 ENCRYPTION_KEY
   - 实现与 TeslaMate 相同的加密/解密算法
   - 测试确保能正确解密 TeslaMate 存储的 token

3. **操作限制**：
   - 只进行 SELECT/UPDATE 操作
   - 不修改表结构
   - 不删除已有数据

### 10.3 实现建议
1. **先研究 TeslaMate 源码**：
   - 查看 Elixir 代码中的加密实现
   - 确认具体的加密算法和参数
   - 在 Go 中实现相同的算法

2. **健康检查**：
   - 检查数据库连接
   - 验证 token 表是否存在
   - 测试加密/解密功能

3. **失败处理**：
   - 如果 token 无效，返回明确错误
   - 如果无法解密，提示检查 ENCRYPTION_KEY
   - 记录详细日志便于排查

---

**关键点总结**：
- ✅ 保存前必须验证 token 有效性
- ✅ 只有通过 Tesla API 验证的 token 才能保存
- ✅ 不创建新表，完全复用 TeslaMate 的表结构
- ✅ 使用与 TeslaMate 相同的加密方式（ENCRYPTION_KEY）
- ✅ 必须在 TeslaMate 启动后才能启动本服务

**开发前必做**：
1. 查看 TeslaMate 的数据库表结构
2. 研究 TeslaMate 的加密算法实现
3. 确保加密/解密与 TeslaMate 完全兼容