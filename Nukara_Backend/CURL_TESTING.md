# Nukara Backend — curl 测试指南

## 前置条件

启动后端服务（默认使用内存存储，无需数据库）：

```bash
cd Nukara_Backend
go build -o build/gateway ./cmd/gateway
./build/gateway
```

服务默认监听 `http://localhost:8080`。

---

## 1. 健康检查

```bash
curl http://localhost:8080/api/v1/gateway/health
```

预期返回：
```json
{"status":"ok","timestamp":"2026-02-26T12:00:00Z"}
```

## 2. 注册流程

### 2.1 发送验证码

```bash
curl -X POST http://localhost:8080/api/v1/auth/sms/send \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800000001","purpose":"register"}'
```

> 开发模式下验证码会打印在服务端日志中，格式：`[SMS] phone=... code=...`

### 2.2 注册

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800000001","sms_code":"从日志获取","nickname":"测试用户"}'
```

返回中包含 `access_token`，后续请求需要用到。

### 2.3 保存 Token

```bash
export TOKEN="返回的access_token值"
```

## 3. 登录（已注册用户）

```bash
# 先发验证码
curl -X POST http://localhost:8080/api/v1/auth/sms/send \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800000001","purpose":"login"}'

# 登录
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800000001","sms_code":"从日志获取"}'
```

## 4. Bot 管理

### 4.1 创建 Bot

```bash
curl -X POST http://localhost:8080/api/v1/bots \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "小夜",
    "summary": "温柔的陪伴型AI",
    "speaking_style": "说话温柔，喜欢用语气词",
    "background": "喜欢看星星的文艺少女",
    "traits": ["温柔","细心","有点害羞"],
    "gender": "female"
  }'
```

> 记下返回的 `id` 字段，后续用作 `BOT_ID`。

```bash
export BOT_ID="返回的bot id"
```

### 4.2 查看所有 Bot

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/bots
```

### 4.3 查看单个 Bot

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/bots/$BOT_ID
```

### 4.4 更新 Bot 人设

```bash
curl -X PATCH http://localhost:8080/api/v1/bots/$BOT_ID \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "speaking_style_adds": ["偶尔会用颜文字"],
    "trait_adds": ["爱吃甜食"]
  }'
```

## 5. 会话与消息

### 5.1 查看会话列表

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/conversations
```

> 创建 Bot 时会自动创建对应会话。记下 `id` 作为 `CONV_ID`。

```bash
export CONV_ID="返回的conversation id"
```

### 5.2 查看历史消息

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/conversations/$CONV_ID/messages?limit=20"
```

### 5.3 发送消息（HTTP 同步）

```bash
curl -X POST "http://localhost:8080/api/v1/conversations/$CONV_ID/send" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"content":{"type":"text","text":"你好呀，今天心情怎么样？"}}'
```

### 5.4 标记已读

```bash
curl -X POST "http://localhost:8080/api/v1/conversations/$CONV_ID/mark-read" \
  -H "Authorization: Bearer $TOKEN"
```

## 6. 测试聊天（Gateway Test 接口）

### 6.1 同步聊天测试

```bash
curl -X POST http://localhost:8080/api/v1/gateway/test/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"bot_id":"'$BOT_ID'","message":"最近在忙什么呢？","debug":true}'
```

### 6.2 流式聊天测试（SSE）

```bash
curl -N -X POST http://localhost:8080/api/v1/gateway/test/chat/stream \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"bot_id":"'$BOT_ID'","message":"给我讲个故事吧"}'
```

> `-N` 禁用缓冲，实时看到 SSE 事件流。

## 7. 主动消息

### 7.1 触发主动消息测试

```bash
curl -X POST http://localhost:8080/api/v1/gateway/test/proactive \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"bot_id":"'$BOT_ID'","conversation_id":"'$CONV_ID'","trigger_type":"manual"}'
```

### 7.2 查看主动消息日志

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/proactive/logs?limit=10"
```

## 8. 用户设置

### 8.1 注册设备推送 Token

```bash
curl -X POST http://localhost:8080/api/v1/users/device-token \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"device_token":"test-apns-token-xxx","platform":"ios"}'
```

### 8.2 通知设置

```bash
# 查看
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/users/notification-settings

# 更新
curl -X PUT http://localhost:8080/api/v1/users/notification-settings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"proactive_enabled":true,"dnd_start":"23:00","dnd_end":"08:00","frequency":"normal"}'
```

### 8.3 用户状态

```bash
# 查看
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/users/status

# 更新
curl -X PUT http://localhost:8080/api/v1/users/status \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"emoji":"😊","text":"心情不错"}'
```

## 9. 监控

```bash
curl http://localhost:8080/api/v1/gateway/metrics
```

## 10. WebSocket 聊天

WebSocket 端点：`ws://localhost:8080/ws/chat?token=$TOKEN`

可以用 `websocat` 测试：

```bash
# 安装 websocat: brew install websocat
websocat "ws://localhost:8080/ws/chat?token=$TOKEN"
```

连接后发送 JSON 消息：

```json
{"type":"message","conversation_id":"CONV_ID","content":{"type":"text","text":"你好"}}
```

服务端会推送以下事件类型：`ack`、`typing`、`stream_start`、`stream_chunk`、`stream_end`、`message`、`bot_status_update`、`proactive_message`。
