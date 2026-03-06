# AgentX / OpenAI / 本地部署修复 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复 `deploy/deploy-local.sh` 的部署源问题，为 provider 增加 `api_mode`，兼容 OpenAI `chat/completions` 与 `responses`，并补齐网页端错误可见性与关键回归测试。

**Architecture:** 通过在 provider 数据模型中引入 `api_mode`，把 runtime 与 admin provider 测试统一收敛到一个 OpenAI 兼容 client/factory。部署侧修复源码来源锁定与默认 provider 幂等自举，前端补充 websocket/runtime 错误可见性，避免“无回应”的黑盒体验。

**Tech Stack:** Go, PostgreSQL, Vue 3, Pinia, Bash, Nginx

---

### Task 1: 建立 worktree 基线

**Files:**
- Verify: `.worktrees/agentx-openai-deploy`
- Update: `.gitignore`

**Steps:**
- [ ] 确认 `.worktrees/` 已被 git ignore
- [ ] 记录 worktree 路径与当前 branch
- [ ] 检查 worktree 下 `git status` 是否仅包含预期文档变更

**Verify:**
- 运行 `git -C /Users/nidhogg/code/Nukara/.worktrees/agentx-openai-deploy status --short`

---

### Task 2: 补 provider 的 `api_mode` 数据模型

**Files:**
- Update: `Nukara_Backend/internal/store/provider.go`
- Update: `Nukara_Backend/internal/admin/provider_handler.go`
- Update: `Nukara_Backend/internal/admin/provider_chat_test_handler.go`
- Create: `Nukara_Backend/migrations/004_provider_api_mode.sql`

**Steps:**
- [ ] 先写后端单测，描述 provider `api_mode` 的读写与默认行为
- [ ] 运行单测，确认因字段缺失而失败
- [ ] 为 `Provider` 结构补 `api_mode`
- [ ] 为 provider 创建/更新/读取接口补 `api_mode`
- [ ] 编写 migration，为 `providers` 增加 `api_mode`
- [ ] 重新运行相关单测，确认通过

**Verify:**
- 运行相关 Go 单测（provider handler / store 相关）

---

### Task 3: 扩展 OpenAI 兼容 client 支持 `responses`

**Files:**
- Update: `Nukara_Backend/internal/agentx/llm/openai_compat.go`
- Create/Update: `Nukara_Backend/internal/agentx/llm/openai_compat_test.go`
- Update: `Nukara_Backend/internal/agentx/provider/types.go`（如需扩 route 字段）
- Update: `Nukara_Backend/internal/agentx/provider/router.go`

**Steps:**
- [ ] 先写 `chat_completions` 与 `responses` 的 JSON/SSE 解析测试
- [ ] 运行测试，确认 `responses` 用例先失败
- [ ] 在 client 中加入 `api_mode` 与 endpoint 选择逻辑
- [ ] 实现 `responses` JSON 返回文本抽取
- [ ] 实现 `responses` SSE/stream 文本抽取
- [ ] 实现 `auto` 模式的明确兜底与日志
- [ ] 重新运行 llm 测试，确认通过

**Verify:**
- 运行 `go test` 针对 `internal/agentx/llm`

---

### Task 4: 统一 runtime 与 admin provider 测试路径

**Files:**
- Update: `Nukara_Backend/internal/agentx/runtime.go`
- Update: `Nukara_Backend/internal/admin/provider_chat_test_handler.go`
- Update: `Nukara_Backend/internal/agentx/runtime_route_test.go`
- Update: `Nukara_Backend/internal/api/agentx_runtime_test.go`

**Steps:**
- [ ] 先为 runtime 写按 `api_mode` 选路测试
- [ ] 先为 admin chat-test 写模式透传测试
- [ ] 运行测试，确认先失败
- [ ] 统一 client factory 的接入点
- [ ] 为 runtime 增加 `client == nil` 的显式错误
- [ ] 保持 fallback 行为可观测，不再静默吞错
- [ ] 重新运行相关测试，确认通过

**Verify:**
- 运行 `go test` 针对 `internal/agentx` 与 `internal/api`

---

### Task 5: 修复默认 provider 自举幂等性

**Files:**
- Update: `deploy/lib/admin-bootstrap.sh`
- Update: `scripts/verify_default_provider_bootstrap.sh`

**Steps:**
- [ ] 先补自举脚本的幂等用例或 smoke 验证脚本
- [ ] 修改脚本：存在则更新并切换，不存在才创建
- [ ] 输出明确日志，包含 create/update/switch 结果
- [ ] 运行脚本级验证

**Verify:**
- 运行 `scripts/verify_default_provider_bootstrap.sh`

---

### Task 6: 修复 `deploy-local.sh` 的部署源锁定

**Files:**
- Update: `deploy/deploy-local.sh`
- Update: `docs/deployment-guide.md`
- Create/Update: `scripts/verify_force_clean.sh`

**Steps:**
- [ ] 先写/补脚本验证，覆盖“源码位于 `/opt/nukara` 下”的场景
- [ ] 运行验证，确认当前实现不能保证部署源稳定
- [ ] 在清理安装目录前锁定部署源
- [ ] 当源码根位于安装目录时，先复制到 `/tmp` 快照
- [ ] 移除本地部署脚本中的静默远程 clone fallback
- [ ] 打印实际部署源目录与 commit
- [ ] 重新运行脚本验证，确认通过

**Verify:**
- 运行相关脚本验证命令

---

### Task 7: 管理端支持选择 provider API 模式

**Files:**
- Update: `Nukara_Admin_Web/src/api/admin.js`
- Update: `Nukara_Admin_Web/src/App.vue`
- Update: `Nukara_Admin_Web/src/style.css`

**Steps:**
- [ ] 先补前端最小交互/数据处理测试（若项目已有可复用测试框架）
- [ ] 更新 provider payload，传递 `api_mode`
- [ ] 新建/编辑 UI 增加 API 模式选择
- [ ] provider 列表/卡片展示当前模式
- [ ] provider 快速测试带上当前模式语义
- [ ] 运行前端构建验证，确认通过

**Verify:**
- 运行管理端构建或相关测试

---

### Task 8: 网页端补错误可见性

**Files:**
- Update: `Nukara_Web/src/composables/useWebSocket.js`
- Update: `Nukara_Web/src/stores/chat.js`
- Update: `Nukara_Web/src/views/ChatView.vue`
- Update: `Nukara_Web/src/composables/useApi.js`

**Steps:**
- [ ] 先补与错误展示相关的前端测试（若已有相邻模式）
- [ ] 为 websocket 增加 `error` 事件分发
- [ ] 在 chat store 中增加 runtime/provider 错误状态
- [ ] 在聊天页显示明确错误提示
- [ ] 改善 API 非 JSON 错误处理
- [ ] 运行网页端构建验证，确认通过

**Verify:**
- 运行网页端构建或相关测试

---

### Task 9: 统一 provider schema 兼容层

**Files:**
- Update: `Nukara_Backend/migrations/003_create_provider_tables.sql`（仅在确认安全时最小修订）
- Create: `Nukara_Backend/migrations/005_provider_schema_compat.sql`
- Update: `Nukara_Backend/internal/store/postgres_store.go`

**Steps:**
- [ ] 先审查当前 migration 与内建 schema 差异
- [ ] 选择最小风险的兼容迁移方式
- [ ] 统一 provider 相关表的 TEXT 语义
- [ ] 确保老库升级不会破坏现有数据
- [ ] 运行 store / admin 相关测试

**Verify:**
- 运行后端相关测试

---

### Task 10: 全量回归与交付

**Files:**
- Review: `docs/plans/2026-03-06-agentx-openai-deploy-design.md`
- Review: `docs/plans/2026-03-06-agentx-openai-deploy-impl-plan.md`

**Steps:**
- [ ] 运行后端相关 Go 测试
- [ ] 运行管理端构建验证
- [ ] 运行网页端构建验证
- [ ] 运行部署/自举相关脚本验证
- [ ] 检查 `git diff --stat`
- [ ] 整理变更说明、验证结果、剩余风险

**Verify:**
- 仅在拿到新鲜命令输出后，才声明完成
