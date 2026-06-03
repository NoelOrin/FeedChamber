#!/bin/bash

# 测试 OrinForum API 对接
# 使用 curl 直接请求后端，绕过 dashboard 的 mock

BASE_URL="http://localhost:8080/api/v1"
TOKEN=""

echo "==================================="
echo "测试 OrinForum API 对接"
echo "==================================="
echo ""

# 1. 健康检查
echo "1. 健康检查"
curl -s -X POST "${BASE_URL}/health" | jq '.'
echo ""

# 2. 注册测试用户
echo "2. 注册测试用户"
REGISTER_RESPONSE=$(curl -s -X POST "${BASE_URL}/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"testadmin","email":"test@example.com","password":"Test123456"}')
echo "$REGISTER_RESPONSE" | jq '.'
echo ""

# 3. 登录
echo "3. 登录"
LOGIN_RESPONSE=$(curl -s -X POST "${BASE_URL}/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"testadmin","password":"Test123456"}')
echo "$LOGIN_RESPONSE" | jq '.'
TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.access_token')
echo "Token: $TOKEN"
echo ""

# 4. 获取公开设置
echo "4. 获取公开设置"
curl -s -X POST "${BASE_URL}/settings/public" | jq '.'
echo ""

# 5. 获取分类列表
echo "5. 获取分类列表"
curl -s -X POST "${BASE_URL}/categories/list" | jq '.'
echo ""

# 6. 获取标签列表
echo "6. 获取标签列表"
curl -s -X POST "${BASE_URL}/tags/list" | jq '.'
echo ""

echo "==================================="
echo "测试完成"
echo "==================================="
