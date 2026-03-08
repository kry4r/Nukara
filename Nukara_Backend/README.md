# Nukara Backend (Day1 Microservices Skeleton)

This folder contains the backend implementation aligned with:
- `Nukara_Doc/11-基于Dify的后端集成方案.md`
- `Nukara_Doc/02-后端详细设计.md`
- `Nukara_Doc/05-主动消息系统.md`
- `Nukara_Doc/06-网关与测试方案.md`
- `Nukara_Doc/09-iOS真实后端接口契约.md`

## Services
- `cmd/gateway`: API gateway + test API + current end-to-end routes
- `cmd/account`: auth routes only
- `cmd/bot`: bot routes only
- `cmd/conversation`: conversation + chat test routes
- `cmd/proactive`: proactive + APNs-related routes

## Key API coverage
- Auth: `/api/v1/auth/email/send`, `/api/v1/auth/login`, `/api/v1/auth/register`
- Bot: `/api/v1/bots`, `/api/v1/bots/{id}`, `/api/v1/bots/{id}/persona`
  - Runtime portrait: `GET /api/v1/bots/{id}/profile`
  - Cached impression: `POST /api/v1/bots/{id}/impression`
  - Manual iterate fallback: `POST /api/v1/bots/{id}/iterate`
  - Persona change review: `POST /api/v1/bots/{id}/persona-changes/{changeID}/accept|reject`
- Conversation: `/api/v1/conversations`, `/api/v1/conversations/{id}/messages`, `/mark-read`, `/read`
- Proactive/APNs surfaces:
  - `POST /api/v1/users/device-token`
  - `GET|PUT /api/v1/users/notification-settings`
  - `GET /api/v1/proactive/logs`
  - `POST /api/v1/gateway/test/proactive`
- Admin surfaces:
  - `GET /api/admin/users/{userID}/bots/{botID}/memory-graph` now includes runtime state, recent impressions, recent changes and pending persona changes
  - `GET|PUT /api/admin/settings/post-turn-model` configures the cheaper dedicated post-turn model route
- Test APIs:
  - `POST /api/v1/gateway/test/chat`
  - `POST /api/v1/gateway/test/chat/stream` (SSE)
  - `GET /api/v1/gateway/health`
  - `GET /api/v1/gateway/metrics`

## Notes
- Store layer supports two modes:
  - in-memory default (no extra env)
  - PostgreSQL + Redis mode when `NUKARA_POSTGRES_DSN` is set
- `GET /ws/chat` is implemented with native WebSocket upgrade and supports:
  - inbound `message` from client
  - outbound `ack`, `typing`, `stream_start/chunk/end`, `message`, `bot_status_update`, `proactive_message`
- WS Hub supports Redis Pub/Sub broadcast (`NUKARA_REDIS_ADDR`) for multi-instance message fanout.
- Dify integration supports blocking + streaming workflow invocation (with fallback to local stub).
- Dify integration supports blocking + streaming workflow invocation. If Dify is unavailable, it falls back to Astron MaaS (OpenAI-compatible API).
- APNs client supports production p8 token auth; missing credentials automatically fallback to stub.
- Bot profile payload now carries runtime portrait data (`runtime_state`, `recent_impressions`, `key_memories`, `recent_changes`, `pending_persona_changes`) so the web detail page and admin memory graph can render the same post-turn state.

## Environment variables
- `NUKARA_POSTGRES_DSN`: enable persistent Postgres store.
- `NUKARA_REDIS_ADDR`: enable Redis metrics + WS pub/sub.
- `NUKARA_DIFY_BASE_URL`: Dify API base URL.
- `NUKARA_DIFY_API_KEY`: Dify workflow key (chat).
- `NUKARA_DIFY_PROACTIVE_API_KEY`: optional proactive workflow key.
- `NUKARA_ASTRON_BASE_URL`: Astron MaaS OpenAI-compatible base URL.
- `NUKARA_ASTRON_API_KEY`: Astron MaaS API key.
- `NUKARA_ASTRON_CHAT_MODEL`: chat model id (e.g. `xopglm47blth2` / `xopglm5` / `xopkimik25` / `xminimaxm2`).
- `NUKARA_ASTRON_PROACTIVE_MODEL`: proactive message generation model id.
- `NUKARA_ASTRON_EMBEDDING_MODEL`: embedding model id for embedding-related subtasks.
- `NUKARA_EMBEDDING_MODEL`: OpenAI-compatible embedding model used by the temporal memory graph embedding path.
- Temporal memory now uses PostgreSQL as the single persistence source; `pgvector` is optional and enabled opportunistically when the Postgres instance has the extension installed.
- `NUKARA_APNS_TOPIC`: APNs topic / bundle id.
- `NUKARA_APNS_KEY_ID`: APNs key id.
- `NUKARA_APNS_TEAM_ID`: Apple team id.
- `NUKARA_APNS_P8_PATH`: path to `.p8` key file.
- `NUKARA_APNS_P8_BASE64`: base64 encoded `.p8` key (alternative to path).
- `NUKARA_APNS_SANDBOX`: `true` for sandbox APNs endpoint.

## Astron MaaS model mapping (provider reference)
- `xopglm5` => iflyglm
- `xopglm47blth2` => glm4.7
- `xopkimik25` => kimi-2.5
- `xminimaxm2` => minmax

## Scripts
- `./scripts/dev_up.sh`: start dependencies and run gateway locally.
- `./scripts/deploy_gateway.sh`: compose-based deploy for gateway + deps.
- `./scripts/docker_local_up.sh [image] [container] [port]`: start persistent local docker gateway (restart policy enabled).
- `./scripts/docker_local_status.sh [container]`: show running status.
- `./scripts/docker_local_logs.sh [container] [tail]`: show recent logs.
- `./scripts/docker_local_down.sh [container]`: stop/remove local gateway container.
- `./scripts/docker_local_smoke.sh`: offline-friendly docker build/run/smoke (no external base image pull).
- `./scripts/smoke_backend.sh [base_url] [email]`: backend API smoke test（需先配置 SMTP）。
- `configs/ios-backend-integration.md`: iOS real-backend integration guide.

## Docker deployment notes
- Standard path (requires pulling images): `./scripts/deploy_gateway.sh`
- Offline-friendly path (local binary + `scratch` runtime image): `./scripts/docker_local_smoke.sh`
- Persistent local deployment:
  - up: `./scripts/docker_local_up.sh`
  - status: `./scripts/docker_local_status.sh`
  - logs: `./scripts/docker_local_logs.sh`
  - down: `./scripts/docker_local_down.sh`
