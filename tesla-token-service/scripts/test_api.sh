#!/bin/bash

# Tesla Token Service API 测试脚本

BASE_URL="http://localhost:8080"
JWT_TOKEN="your_jwt_token_here"
USER_ID="test_user"

echo "=== Tesla Token Service API 测试 ==="

# 1. 健康检查
echo "1. 健康检查..."
curl -s "$BASE_URL/health" | jq .

echo -e "\n"

# 2. 保存 Token
echo "2. 保存 Token..."
curl -s -X POST "$BASE_URL/api/v1/tokens" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "'$USER_ID'",
    "access_token": "sample_access_token_123",
    "refresh_token": "sample_refresh_token_123",
    "expires_at": "'$(date -d '+1 hour' -Iseconds)'"
  }' | jq .

echo -e "\n"

# 3. 获取 Token
echo "3. 获取 Token..."
curl -s -X GET "$BASE_URL/api/v1/tokens/$USER_ID" \
  -H "Authorization: Bearer $JWT_TOKEN" | jq .

echo -e "\n"

# 4. 验证 Token
echo "4. 验证 Token..."
curl -s -X GET "$BASE_URL/api/v1/tokens/$USER_ID/validate" \
  -H "Authorization: Bearer $JWT_TOKEN" | jq .

echo -e "\n"

# 5. 刷新 Token
echo "5. 刷新 Token..."
curl -s -X POST "$BASE_URL/api/v1/tokens/$USER_ID/refresh" \
  -H "Authorization: Bearer $JWT_TOKEN" | jq .

echo -e "\n"

# 6. 删除 Token
echo "6. 删除 Token..."
curl -s -X DELETE "$BASE_URL/api/v1/tokens/$USER_ID" \
  -H "Authorization: Bearer $JWT_TOKEN" | jq .

echo -e "\n=== 测试完成 ==="