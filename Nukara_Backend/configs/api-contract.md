# API Contract Additions for iOS Real Integration

## Device token registration
- Method: `POST`
- Path: `/api/v1/users/device-token`
- Body:
```json
{
  "device_token": "apns-device-token",
  "platform": "ios"
}
```

## Notification settings
- Method: `GET | PUT`
- Path: `/api/v1/users/notification-settings`
- PUT body:
```json
{
  "proactive_enabled": true,
  "dnd_start": "22:00",
  "dnd_end": "08:00",
  "frequency": "normal"
}
```

## Manual proactive trigger
- Method: `POST`
- Path: `/api/v1/gateway/test/proactive`
- Body:
```json
{
  "bot_id": "bot_xxx",
  "conversation_id": "conv_xxx",
  "trigger_type": "manual"
}
```

## WebSocket chat
- Method: `GET`
- Path: `/ws/chat`
- Auth header: `Authorization: Bearer <access_token>`

Client event:
```json
{
  "type": "message",
  "conversation_id": "conv_xxx",
  "client_msg_id": "client_xxx",
  "content": {
    "type": "text",
    "text": "你好"
  }
}
```

Server events:
- `ack`
- `typing`
- `stream_start`
- `stream_chunk`
- `stream_end`
- `message`
- `bot_status_update`
- `proactive_message`
