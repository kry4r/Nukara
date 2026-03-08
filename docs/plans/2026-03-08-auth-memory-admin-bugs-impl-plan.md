# Auth / Memory / Admin Bugs Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复邮箱验证码重复发送、AgentX 对话链路 compact / temporal memory ID 混淆、admin provider 页面紧凑化与测试入口恢复。

**Architecture:** 将 provider-facing conversation id 与本地 conversation id 解耦；把 temporal memory / compact 中承载业务字符串的列改为 `TEXT`；admin memory graph 改为读取 temporal memory graph；邮箱验证码发送增加后端冷却保护；admin provider 页面改为卡牌 + expander。

**Tech Stack:** Go, PostgreSQL, Vue 3, Pinia, Vite

---

### Task 1: Cover current failures with tests

**Files:**
- Modify: `Nukara_Backend/internal/api/email_auth_test.go`
- Modify: `Nukara_Backend/internal/api/runtime_context_test.go`
- Modify: `Nukara_Backend/internal/admin/memory_graph_handler_test.go`
- Create/Modify: targeted store tests for temporal schema compatibility if needed

**Steps:**
1. Add failing test for duplicate email send suppression.
2. Add failing test proving runtime context uses local conversation id for compact lookup.
3. Add failing test proving temporal node IDs / session summary IDs may be non-UUID strings.
4. Add failing test proving admin graph includes temporal memory nodes.

### Task 2: Fix backend ID semantics and schema

**Files:**
- Modify: `Nukara_Backend/internal/api/runtime_context.go`
- Modify: `Nukara_Backend/internal/api/runtime_adapter.go`
- Modify: `Nukara_Backend/internal/api/chat_flow.go`
- Modify: `Nukara_Backend/internal/api/emotion_tracker.go`
- Modify: `Nukara_Backend/internal/api/bot_profile.go`
- Modify: `Nukara_Backend/internal/api/self_cognition_summary.go` if needed
- Modify: `Nukara_Backend/internal/store/postgres_store.go`
- Modify: `Nukara_Backend/internal/store/postgres_temporal_memory_graph.go`
- Add migration(s) under `Nukara_Backend/migrations`

**Steps:**
1. Introduce explicit local conversation id usage in runtime context path.
2. Keep provider-facing conversation id only for provider transport.
3. Change compact / temporal memory string-ID columns to `TEXT`.
4. Add schema migration for existing DBs.
5. Run targeted tests.

### Task 3: Fix duplicate email sending

**Files:**
- Modify: `Nukara_Backend/internal/api/server.go`
- Modify: `Nukara_Backend/internal/store/interface.go` if needed
- Modify: `Nukara_Backend/internal/store/store.go` / postgres store if persistent cooldown is needed

**Steps:**
1. Implement short cooldown / idempotency check on `(email, purpose)`.
2. Return success message without second SMTP send when within cooldown.
3. Keep existing login/register validation unchanged.
4. Run auth tests.

### Task 4: Switch admin graph to temporal memory graph source

**Files:**
- Modify: `Nukara_Backend/internal/admin/memory_graph_handler.go`
- Modify: `Nukara_Backend/internal/admin/memory_graph_handler_test.go`

**Steps:**
1. Query temporal nodes / edges as primary graph source.
2. Preserve side panels for recent impressions / persona changes.
3. Ensure new bots with temporal nodes render non-empty graph.
4. Run admin handler tests.

### Task 5: Compact admin provider UI

**Files:**
- Modify: `Nukara_Admin_Web/src/App.vue`
- Modify: `Nukara_Admin_Web/src/style.css`
- Modify tests under `Nukara_Admin_Web/tests` if needed

**Steps:**
1. Refactor provider list into compact card + expander layout.
2. Restore visible provider edit/test actions inside expanded section.
3. Keep switching, chat test, and save flows intact.
4. Build admin frontend.

### Task 6: Verify

**Files:**
- No source changes unless fixing local regressions

**Steps:**
1. Run targeted Go tests for auth, runtime context, admin memory graph, temporal memory graph.
2. Run admin frontend build.
3. If needed, run targeted chat / full-stack smoke.
4. Summarize any remaining manual validation items.
