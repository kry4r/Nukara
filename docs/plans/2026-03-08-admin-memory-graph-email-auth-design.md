# Admin Memory Graph + Email Auth Design

**Date:** 2026-03-08

**Issue:** `#14 [feat] 邮箱注册与记忆可视化`

## Scope

一次性交付两项功能，并统一落在现有 admin 页面能力上：

1. 在 admin 页面提供真正的记忆图可视化，支持先选 `User`，再选其 `Robot`，并查看该机器人下的记忆图谱。
2. 将账号认证从 `phone + sms_code` 破坏式切换为 `email + email_code`，并在 admin 页面提供 SMTP 配置与测试能力。

## Goals

- 保留现有 `user.id` 以及其关联的 bot / conversation / memory / proactive 数据。
- 让 admin 可以明确验证“记忆是否已接入”，而不是只看原始表数据。
- 让 Web 与 iOS 客户端统一切到邮箱验证码登录/注册。
- 让 SMTP 配置可在 admin 内完成，并支持 QQ 邮箱授权码场景。

## Non-Goals

- 不引入通用多身份体系（如 `user_identities`）。
- 不保留手机号登录入口。
- 不在首版记忆图中把 `User`、`Robot` 作为图节点渲染到画布中；它们保留在左侧选择栏中。
- 不实现复杂图编辑能力；首版只读展示。

## Current State

- 后端认证接口仍使用 `/api/v1/auth/sms/send`、`/api/v1/auth/login`、`/api/v1/auth/register`，请求体依赖 `phone` 与 `sms_code`。
- `users` 表与内存/持久化 store 都以手机号为身份字段。
- Admin 页面目前只有 provider / embedding / user provider setting 等管理能力，没有记忆图和 SMTP 配置。
- Memory 数据已经进入 `memory_items`，topic 关联已经可写入 Neo4j，但没有 admin 可视化入口。
- `Nukara_Web` 与 `Nukara_App` 仍全部依赖手机号验证码流程。

## Chosen Approach

采用“保留用户主键与业务数据、切换登录凭证到邮箱”的方案：

- 数据层将 `users.phone` 改为 `users.email`，并把验证码存储从 `sms_codes` 改为 `email_codes`。
- 认证接口对外保持 `/api/v1/auth/login` 与 `/api/v1/auth/register` 路径不变，但请求体改为邮箱字段；验证码发送接口改为 `/api/v1/auth/email/send`。
- Admin 新增 SMTP 配置读写与测试接口；邮件发送能力从 system settings 驱动。
- Admin 新增记忆图查询接口；图的基础节点与边来自 `memory_items` + `topics`，若 Neo4j 可用则补充 topic 扩展边。

## Data Model

### User identity

- `users.phone` 重命名为 `users.email`
- store.User 结构同步改为：
  - `Email string \`json:"email"\``
  - 保留 `ID`、`Nickname`、`Avatar`、`CreatedAt`
- store 查询/创建接口改为：
  - `FindUserByEmail(email string)`
  - `CreateUser(email, nickname string)`

### Verification codes

- `sms_codes` 表重命名/替换为 `email_codes`
- Store 中的 `SMSCode` 改为 `EmailCode`
- Store 方法改为：
  - `SaveEmailCode(email, purpose, code string, ttl time.Duration)`
  - `ValidateEmailCode(email, purpose, code string) bool`

### SMTP settings

使用现有 `system_settings` 保存邮件配置，避免新开表：

- `email_auth_enabled`
- `smtp_host`
- `smtp_port`
- `smtp_username`
- `smtp_password`
- `smtp_from_email`
- `smtp_from_name`
- `email_code_ttl_seconds`

其中密码保留原值存储，admin 页面回显时可允许再次编辑，不做过度 masking，沿用现有 embedding/api_key 风格。

## Backend Architecture

### Auth flow

- `POST /api/v1/auth/email/send`
  - 请求：`{ email, purpose }`
  - 校验 SMTP 配置是否完整
  - 生成 6 位验证码
  - 保存验证码
  - 通过 SMTP 发送邮件
- `POST /api/v1/auth/register`
  - 请求：`{ email, email_code, nickname }`
  - 校验验证码后创建用户
  - 签发 token，token payload 仍只含 `user.id`
- `POST /api/v1/auth/login`
  - 请求：`{ email, email_code }`
  - 校验验证码后查找用户并签发 token

### Admin memory graph

新增 admin API：

- `GET /api/admin/users?q=<query>&limit=<n>&offset=<n>`
- `GET /api/admin/users/{userId}/bots`
- `GET /api/admin/users/{userId}/bots/{botId}/memory-graph`

`memory-graph` 响应：

```json
{
  "nodes": [
    {
      "id": "mem-1",
      "type": "memory",
      "label": "喜欢深夜散步",
      "content": "用户喜欢深夜散步并且下雨时会安静。",
      "importance": 88,
      "owner": "user",
      "topics": ["散步", "下雨", "夜晚"],
      "occurred_at": "2026-03-08T08:00:00Z"
    },
    {
      "id": "topic-夜晚",
      "type": "topic",
      "label": "夜晚"
    }
  ],
  "edges": [
    { "id": "mem-1-topic-夜晚", "source": "mem-1", "target": "topic-夜晚", "type": "memory_topic" }
  ],
  "summary": {
    "memory_count": 12,
    "topic_count": 8,
    "graph_source": "store+neo4j"
  }
}
```

退化策略：
- Neo4j 不可用时仍返回 `Memory -> Topic` 图，`graph_source=store`
- 没有记忆时返回空图，不报错

### SMTP service

新增一个小的邮件发送模块，例如 `Nukara_Backend/internal/mail/smtp.go`：

- 从 admin settings 读取 SMTP 参数
- 封装验证码邮件正文
- 提供 `SendVerificationCode(ctx, email, code, purpose)`
- 提供 `SendTestMail(ctx, to)`

## Admin Frontend Design

### Layout

在现有 admin 单页中新增两个板块：

1. `记忆图谱`
2. `邮箱认证 / SMTP`

### Memory graph interaction

左侧栏：
- 用户搜索框
- 用户卡片列表（显示邮箱、昵称）
- 当前选中用户下的 robot 卡片列表

右侧主区域：
- 图谱画布
- 选中节点详情面板

图谱表现：
- `Memory` 节点：矩形卡片，显示内容摘要
- `Topic` 节点：较小圆角节点
- 点击节点后右侧展示完整属性，尤其是 `content`

实现上优先使用轻量浏览器图组件；如果现有依赖没有图组件，则新增一个轻量库，以最小代价实现缩放、拖拽、点击选中。

### Email / SMTP panel

字段：
- SMTP Host
- SMTP Port
- SMTP Username
- SMTP Password / 授权码
- 发件邮箱
- 发件人名称
- 验证码 TTL（秒）
- 测试收件邮箱

操作：
- 保存 SMTP 配置
- 发送测试邮件
- 提示当前认证模式已切换为邮箱验证码

QQ 邮箱默认建议：
- host: `smtp.qq.com`
- port: `465`
- password: QQ 邮箱 SMTP 授权码

## Web + iOS Client Changes

### Web

- `Nukara_Web/src/stores/auth.js`：从 `requestSMS/login/register` 切换为 email 版请求
- `Nukara_Web/src/views/AuthView.vue`：输入框与文案从手机号改为邮箱
- 倒计时逻辑保留，改为“发送邮箱验证码”

### iOS

- `APIEndpoint.swift`：payload 从 `phone/sms_code` 改为 `email/email_code`
- `AuthRepositoryProtocol` / `RealAuthRepository` / `MockAuthRepository`：方法签名同步切换
- `SessionStore` / `AuthView`：UI 与调用链切换到邮箱

## Error Handling

- 邮箱格式非法：400
- SMTP 未配置：400 或 503，明确提示 admin 先配置 SMTP
- SMTP 发送失败：502，并返回摘要信息
- 邮箱已注册：400
- 邮箱不存在：404
- 验证码错误或过期：400
- 记忆图无数据：200 + 空图
- Neo4j 不可用：200 + 降级图

## Testing Strategy

### Backend

- 为 email auth 新增/改造 handler tests：
  - 发送验证码
  - 注册
  - 登录
  - 错误验证码
  - SMTP 未配置
- 为 admin 新增 tests：
  - email settings 读写
  - test mail endpoint
  - list users by email
  - list bots by user
  - memory graph payload build
  - neo4j fallback
- 为 SQL migration 增加验证脚本或 migration test，确保空库部署可成功迁移到 email schema

### Frontend

- Admin API 小测试扩展到新 helper
- 若 UI harness 不足，则至少执行精确手动验证：
  - 保存 SMTP
  - 发送测试邮件
  - 查看用户/robot
  - 打开记忆图并点击节点详情

### Clients

- Web：验证邮箱发送验证码、注册、登录
- iOS：更新 mock / unit tests 中的 auth payload 与调用签名

## Risks and Mitigations

- **风险：破坏式切换后无邮箱的旧用户无法登录**
  - 缓解：本次实现默认要求数据库里用户已有 email；若测试/开发库没有，可通过 SQL 或 admin 补录。
- **风险：图组件引入过重依赖**
  - 缓解：优先选轻量图渲染库，只做读展示。
- **风险：Neo4j 不稳定影响 admin 可用性**
  - 缓解：服务端构图以 `memory_items` 为真源，Neo4j 仅做增强。
- **风险：QQ 邮箱发送策略差异**
  - 缓解：提供测试邮件接口与明确配置提示，使用标准 SMTPS。

## Implementation Order

1. 数据层与 migration 切到 email
2. SMTP service 与 auth handler 切换
3. Admin email settings API
4. Admin memory graph API
5. Admin UI
6. Web auth UI / store
7. iOS auth flow
8. 全量验证
