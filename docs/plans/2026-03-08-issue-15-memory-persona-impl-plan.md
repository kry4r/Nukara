# Issue #15 Memory & Living-State Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement issue #15 by adding layered memory, chat-only promise follow-through, believable current living-state continuity, a dedicated cheaper post-turn model configuration, a runtime portrait bot detail page, and Admin inspection/configuration support built on top of the new `main` admin memory tooling.

**Architecture:** Keep `memory_items` as the main extracted-memory store and extend its `kind/status` semantics instead of replacing the memory stack. Add dedicated persistence for runtime state and persona-change events, route post-turn extraction/summarization through a separately configurable provider/model, then expose the resulting data through the existing bot profile flow and the `main` branch’s admin memory graph/config surfaces.

**Tech Stack:** Go backend, PostgreSQL, existing in-memory store, existing `agentx/subtasks` runner, Vue 3 web frontend, Vue 3 admin frontend, Go tests, Node admin API test script, Playwright web tests.

---

## Preflight

Before touching feature code, sync the implementation worktree to the latest `main`.

Run:

```bash
git fetch origin
git checkout brainstorm-issue-15-memory-persona
git merge --ff-only origin/main
```

Expected:

- branch fast-forwards from `58346b5` to at least `bb0b6bc`
- `docs/plans/2026-03-08-admin-memory-graph-email-auth-design.md` and `docs/plans/2026-03-08-admin-memory-graph-email-auth-impl-plan.md` become available in the worktree
- the implementation branch now includes `MemoryGraphPanel.vue` and the admin memory graph handlers from `main`

Then capture a clean baseline:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api ./internal/admin ./internal/agentx/... ./internal/store -count=1
cd ../Nukara_Admin_Web && node tests/provider-api.spec.mjs
cd ../Nukara_Web && npx playwright test tests/persona-proactive-refactor.spec.ts --project=chromium
```

Expected:

- existing focused suites pass, or any pre-existing failure is recorded before feature work starts

### Task 1: Add failing persistence coverage for runtime state and persona change events

**Files:**
- Modify: `Nukara_Backend/internal/store/interface.go`
- Modify: `Nukara_Backend/internal/store/agentx_data.go`
- Modify: `Nukara_Backend/internal/store/postgres_agentx_data.go`
- Create: `Nukara_Backend/internal/store/runtime_profile_test.go`
- Modify: `Nukara_Backend/deploy/sql/001_init.sql`
- Create: `Nukara_Backend/migrations/008_runtime_profile_state.sql`

**Step 1: Write the failing store tests**

Create `Nukara_Backend/internal/store/runtime_profile_test.go` covering at least:

- `UpsertBotRuntimeState(userID, botID, input)` stores and updates current living state
- `GetBotRuntimeState(userID, botID)` returns the last saved runtime state
- `CreatePersonaChangeEvent(...)` persists a pending change proposal
- `ListPersonaChangeEvents(userID, botID, status, limit)` returns newest-first change history
- `UpdatePersonaChangeEventStatus(changeID, status, reviewerNote)` transitions `pending -> accepted/rejected`

Use assertions like:

```go
state, ok := s.GetBotRuntimeState("u1", "b1")
if !ok || state.ActivityText != "刚下晚班，在回去路上" {
    t.Fatalf("unexpected runtime state: %+v ok=%v", state, ok)
}
```

**Step 2: Run the focused test to verify it fails**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/store -run 'TestRuntimeProfile|TestPersonaChange' -count=1
```

Expected: FAIL because the new store types/methods and schema do not exist yet.

**Step 3: Add the schema surfaces**

Design the new tables in `001_init.sql` and `008_runtime_profile_state.sql`:

- `bot_runtime_states`
- `persona_change_events`

Keep the schema minimal and keyed by `user_id + bot_id` or `change_id` as appropriate.

**Step 4: Implement in-memory and Postgres persistence**

Add exact structs and methods in `agentx_data.go`, `postgres_agentx_data.go`, and `interface.go` for runtime-state and change-event CRUD.

**Step 5: Re-run the focused store tests**

Run the same `go test` command from Step 2.
Expected: PASS.

**Step 6: Commit**

```bash
git add Nukara_Backend/internal/store/interface.go \
  Nukara_Backend/internal/store/agentx_data.go \
  Nukara_Backend/internal/store/postgres_agentx_data.go \
  Nukara_Backend/internal/store/runtime_profile_test.go \
  Nukara_Backend/deploy/sql/001_init.sql \
  Nukara_Backend/migrations/008_runtime_profile_state.sql
git commit -m "feat(store): add runtime profile state persistence"
```

### Task 2: Add failing subtask tests for memory classification, risk routing, and current-state updates

**Files:**
- Modify: `Nukara_Backend/internal/agentx/subtasks/memory_extract.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/runner.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/runner_test.go`
- Create: `Nukara_Backend/internal/agentx/subtasks/memory_extract_test.go`
- Create: `Nukara_Backend/internal/agentx/subtasks/runtime_state_test.go`
- Create: `Nukara_Backend/internal/agentx/persona/risk.go`
- Create: `Nukara_Backend/internal/agentx/persona/risk_test.go`

**Step 1: Write failing classification tests**

Cover these rules explicitly:

- `user_fact` only stores explicit user-stated future-relevant facts
- small talk and vague one-off remarks are dropped
- `promise` keeps due/fulfilled/expired status semantics
- low-risk self facts can auto-write
- core persona mutations are routed to `pending confirmation`

Add a risk test like:

```go
result := ClassifyPersonaChange(persona.Patch{IdentityAdds: []string{"其实我是医生"}})
if result.Risk != RiskHigh || result.Route != RoutePendingConfirm {
    t.Fatalf("unexpected routing: %+v", result)
}
```

**Step 2: Write failing current-state tests**

Create `runtime_state_test.go` covering:

- existing plausible state is retained across adjacent turns
- new explicit event can replace stale state
- time-of-day input can shift state from “上班中” to “下班回家” without random jumps

**Step 3: Run the focused tests to verify failure**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/subtasks ./internal/agentx/persona -run 'TestMemoryExtract|TestRuntimeState|TestRisk' -count=1
```

Expected: FAIL because the richer routing/state helpers do not exist yet.

**Step 4: Implement the minimal classification and risk helpers**

Add helpers that:

- normalize incoming extracted `kind/status`
- filter low-value candidates
- distinguish low-risk memory updates from high-risk stable persona changes
- derive or retain a compact current living-state record

Keep the first version deterministic where possible.

**Step 5: Re-run the focused tests**

Run the same `go test` command from Step 3.
Expected: PASS.

**Step 6: Commit**

```bash
git add Nukara_Backend/internal/agentx/subtasks/memory_extract.go \
  Nukara_Backend/internal/agentx/subtasks/runner.go \
  Nukara_Backend/internal/agentx/subtasks/runner_test.go \
  Nukara_Backend/internal/agentx/subtasks/memory_extract_test.go \
  Nukara_Backend/internal/agentx/subtasks/runtime_state_test.go \
  Nukara_Backend/internal/agentx/persona/risk.go \
  Nukara_Backend/internal/agentx/persona/risk_test.go
git commit -m "feat(agentx): classify runtime memories and state changes"
```

### Task 3: Add failing backend tests for dedicated post-turn model configuration and fallback routing

**Files:**
- Modify: `Nukara_Backend/internal/bootstrap/bootstrap.go`
- Modify: `Nukara_Backend/internal/store/turns_test.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/runner_test.go`
- Create: `Nukara_Backend/internal/admin/post_turn_config_handler.go`
- Create: `Nukara_Backend/internal/admin/post_turn_config_handler_test.go`
- Modify: `Nukara_Backend/internal/admin/server.go`

**Step 1: Write failing config handler tests**

Add `post_turn_config_handler_test.go` to cover:

- `GET /api/admin/settings/post-turn-model` returns provider/model values from system settings
- `PUT /api/admin/settings/post-turn-model` validates provider existence and persists settings
- missing config falls back cleanly rather than breaking chat or subtasks

**Step 2: Write failing routing tests**

Extend `runner_test.go` and/or `turns_test.go` to prove:

- a configured `post_turn_provider_id` + `post_turn_model` is selected for memory/impression/iteration subtasks
- when unset, the runner falls back to the normal routed provider/model

**Step 3: Run the focused tests to verify failure**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/admin ./internal/agentx/subtasks ./internal/store -run 'TestPostTurn|TestRunnerUsesPostTurnRoute' -count=1
```

Expected: FAIL because the admin route and routing/fallback code do not exist yet.

**Step 4: Implement config storage and routing**

Use existing system-setting patterns similar to embedding config. Persist keys such as:

- `post_turn_provider_id`
- `post_turn_model`

Add bootstrap plumbing so the subtask runner can request the dedicated route first and fall back to the main route when absent.

**Step 5: Re-run focused tests**

Run the same `go test` command from Step 3.
Expected: PASS.

**Step 6: Commit**

```bash
git add Nukara_Backend/internal/bootstrap/bootstrap.go \
  Nukara_Backend/internal/store/turns_test.go \
  Nukara_Backend/internal/agentx/subtasks/runner_test.go \
  Nukara_Backend/internal/admin/post_turn_config_handler.go \
  Nukara_Backend/internal/admin/post_turn_config_handler_test.go \
  Nukara_Backend/internal/admin/server.go
git commit -m "feat(admin): add post-turn model configuration"
```

### Task 4: Integrate the post-turn pipeline with runtime state and persona-change logging

**Files:**
- Modify: `Nukara_Backend/internal/agentx/subtasks/runner.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/persona_iterate.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/compact_update.go`
- Create: `Nukara_Backend/internal/agentx/subtasks/runtime_profile.go`
- Create: `Nukara_Backend/internal/agentx/subtasks/runtime_profile_test.go`
- Modify: `Nukara_Backend/internal/api/chat_flow.go`

**Step 1: Write failing integration tests**

Create `runtime_profile_test.go` or extend `runner_test.go` so one run proves:

- extracted candidates create `memory_items`
- explicit promises are persisted with correct status
- runtime-state record updates from the turn
- persona iteration no longer mutates stable persona fields directly when the change is high-risk
- instead, it creates a pending persona-change event

**Step 2: Run the focused tests to verify failure**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/subtasks -run 'TestRunnerWritesRuntimeProfile|TestRunnerCreatesPendingPersonaChange' -count=1
```

Expected: FAIL because the runner still applies persona patches directly every third turn and does not maintain runtime-state/change logs.

**Step 3: Implement the minimal integration**

Refactor `runner.go` so it:

- keeps memory extraction
- keeps compact updates
- writes runtime-state updates after extraction/classification
- converts persona-iteration output into either:
  - low-risk recent-change items, or
  - pending core persona confirmations
- does not directly mutate stable persona fields for high-risk outputs

**Step 4: Wire the runner through chat flow**

Ensure `chat_flow.go` continues to fire the async post-turn work, but now produces runtime-profile side effects instead of only direct memory + iterate behavior.

**Step 5: Re-run focused tests**

Run the same `go test` command from Step 2.
Expected: PASS.

**Step 6: Commit**

```bash
git add Nukara_Backend/internal/agentx/subtasks/runner.go \
  Nukara_Backend/internal/agentx/subtasks/persona_iterate.go \
  Nukara_Backend/internal/agentx/subtasks/compact_update.go \
  Nukara_Backend/internal/agentx/subtasks/runtime_profile.go \
  Nukara_Backend/internal/agentx/subtasks/runtime_profile_test.go \
  Nukara_Backend/internal/api/chat_flow.go
git commit -m "feat(chat): persist runtime profile after each turn"
```

### Task 5: Add failing API tests for runtime prompt assembly, profile payloads, and persona-change actions

**Files:**
- Modify: `Nukara_Backend/internal/api/runtime_context.go`
- Create: `Nukara_Backend/internal/api/runtime_context_test.go`
- Modify: `Nukara_Backend/internal/api/bot_profile.go`
- Modify: `Nukara_Backend/internal/api/bot_profile_test.go`
- Modify: `Nukara_Backend/internal/api/server.go`
- Create: `Nukara_Backend/internal/api/persona_change_actions_test.go`

**Step 1: Write failing runtime-context tests**

Add `runtime_context_test.go` to assert prompt assembly includes:

- stable persona summary
- current living-state block
- due/relevant promises block
- only a compact set of relevant memories

**Step 2: Write failing profile/action tests**

Extend `bot_profile_test.go` and add `persona_change_actions_test.go` to cover:

- `/api/v1/bots/:id/profile` returns `runtime_state`, `recent_impressions`, `key_memories`, `recent_changes`, and `pending_persona_changes`
- `/api/v1/bots/:id/impression` returns stored impression data instead of forcing the main UX to regenerate it
- `/api/v1/bots/:id/persona-changes/:changeID/accept` applies a pending change
- `/api/v1/bots/:id/persona-changes/:changeID/reject` marks it rejected without mutating persona fields

**Step 3: Run the focused tests to verify failure**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api -run 'TestBotProfile|TestRuntimeContext|TestPersonaChange' -count=1
```

Expected: FAIL because the runtime profile data and accept/reject routes are not exposed yet.

**Step 4: Implement the API changes**

Update `runtime_context.go`, `bot_profile.go`, and `server.go` so:

- runtime-state + key promise/memory summaries are injected into the system prompt
- `profile` becomes the main runtime portrait payload
- `impression` becomes read-mostly / cached-data oriented
- accept/reject routes update the change log and apply persona patch only on acceptance

**Step 5: Re-run focused API tests**

Run the same `go test` command from Step 3.
Expected: PASS.

**Step 6: Commit**

```bash
git add Nukara_Backend/internal/api/runtime_context.go \
  Nukara_Backend/internal/api/runtime_context_test.go \
  Nukara_Backend/internal/api/bot_profile.go \
  Nukara_Backend/internal/api/bot_profile_test.go \
  Nukara_Backend/internal/api/server.go \
  Nukara_Backend/internal/api/persona_change_actions_test.go
git commit -m "feat(api): expose runtime portrait and persona change actions"
```

### Task 6: Extend admin backend memory inspection and post-turn config APIs

**Files:**
- Modify: `Nukara_Backend/internal/admin/memory_graph_handler.go`
- Modify: `Nukara_Backend/internal/admin/memory_graph_handler_test.go`
- Modify: `Nukara_Backend/internal/admin/server.go`
- Modify: `Nukara_Backend/internal/admin/user_provider_handler.go`
- Modify: `Nukara_Backend/internal/admin/post_turn_config_handler.go`
- Modify: `Nukara_Backend/internal/admin/post_turn_config_handler_test.go`

**Step 1: Write failing admin-memory tests**

Extend `memory_graph_handler_test.go` so the admin response can surface:

- `kind` and `status` filters
- current runtime state payload
- recent impression summary
- recent change events
- pending persona changes

**Step 2: Run focused admin tests to verify failure**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/admin -run 'TestMemoryGraph|TestPostTurnConfig' -count=1
```

Expected: FAIL before the richer admin payloads exist.

**Step 3: Implement the admin backend extensions**

Keep the existing memory graph behavior intact, but extend it with read models for runtime-state/change/config inspection. Do not build a parallel admin memory endpoint tree.

**Step 4: Re-run focused admin tests**

Run the same `go test` command from Step 2.
Expected: PASS.

**Step 5: Commit**

```bash
git add Nukara_Backend/internal/admin/memory_graph_handler.go \
  Nukara_Backend/internal/admin/memory_graph_handler_test.go \
  Nukara_Backend/internal/admin/server.go \
  Nukara_Backend/internal/admin/user_provider_handler.go \
  Nukara_Backend/internal/admin/post_turn_config_handler.go \
  Nukara_Backend/internal/admin/post_turn_config_handler_test.go
git commit -m "feat(admin): inspect runtime profile and post-turn config"
```

### Task 7: Add failing admin frontend coverage and implement the new inspection/config panels

**Files:**
- Modify: `Nukara_Admin_Web/src/api/admin.js`
- Modify: `Nukara_Admin_Web/src/App.vue`
- Modify: `Nukara_Admin_Web/src/components/MemoryGraphPanel.vue`
- Create: `Nukara_Admin_Web/src/components/PostTurnModelPanel.vue`
- Modify: `Nukara_Admin_Web/src/style.css`
- Modify: `Nukara_Admin_Web/tests/provider-api.spec.mjs`

**Step 1: Write failing frontend helper tests**

Extend `provider-api.spec.mjs` to cover:

- fetching and saving post-turn model config
- requesting admin memory data with `kind/status` filters
- reading runtime-state / recent-change / pending-change sections from the admin response

**Step 2: Run the test to verify failure**

Run:

```bash
cd Nukara_Admin_Web && node tests/provider-api.spec.mjs
```

Expected: FAIL because the helper methods and payload normalization do not exist yet.

**Step 3: Implement the admin UI**

Build:

- a `PostTurnModelPanel.vue` for provider/model selection and save state
- richer memory inspection inside `MemoryGraphPanel.vue`
- wiring in `App.vue` and `admin.js`

Keep the Admin audience in mind: this is an inspection/config surface, not the end-user product view.

**Step 4: Re-run the admin frontend test**

Run the same `node tests/provider-api.spec.mjs` command.
Expected: PASS.

**Step 5: Commit**

```bash
git add Nukara_Admin_Web/src/api/admin.js \
  Nukara_Admin_Web/src/App.vue \
  Nukara_Admin_Web/src/components/MemoryGraphPanel.vue \
  Nukara_Admin_Web/src/components/PostTurnModelPanel.vue \
  Nukara_Admin_Web/src/style.css \
  Nukara_Admin_Web/tests/provider-api.spec.mjs
git commit -m "feat(admin-web): manage post-turn model and inspect runtime profile"
```

### Task 8: Add failing web detail-page tests and implement the runtime portrait UI

**Files:**
- Modify: `Nukara_Web/src/views/BotDetailView.vue`
- Create: `Nukara_Web/tests/bot-detail-runtime-portrait.spec.ts`
- Modify: `Nukara_Web/tests/persona-proactive-refactor.spec.ts`

**Step 1: Write the failing web test**

Create `bot-detail-runtime-portrait.spec.ts` that covers:

- current living-state card renders
- key memories section renders grouped items
- recent changes and pending persona changes render separately
- the main detail UX no longer depends on clicking “刷新印象” or “自我迭代” to see core state

Use selectors tied to stable `data-testid` attributes, e.g.:

```ts
await expect(page.getByTestId('bot-detail-runtime-state')).toContainText('刚下晚班')
await expect(page.getByTestId('bot-detail-pending-changes')).toBeVisible()
```

**Step 2: Run the focused Playwright test to verify failure**

Run:

```bash
cd Nukara_Web && npx playwright test tests/bot-detail-runtime-portrait.spec.ts --project=chromium
```

Expected: FAIL because the new runtime portrait layout does not exist yet.

**Step 3: Implement the detail page redesign**

Refactor `BotDetailView.vue` to:

- keep stable persona fields
- add current living-state card
- add grouped key memories
- render recent impression as stored summary content
- render recent changes and pending persona changes separately
- demote/remove `行为指令` from the primary layout

**Step 4: Re-run the focused Playwright test**

Run the same `npx playwright test` command from Step 2.
Expected: PASS.

**Step 5: Commit**

```bash
git add Nukara_Web/src/views/BotDetailView.vue \
  Nukara_Web/tests/bot-detail-runtime-portrait.spec.ts \
  Nukara_Web/tests/persona-proactive-refactor.spec.ts
git commit -m "feat(web): turn bot detail into runtime portrait page"
```

### Task 9: Update docs and run end-to-end focused verification

**Files:**
- Modify: `Nukara_Backend/README.md`
- Modify: `docs/deployment-guide.md`
- Modify: `Nukara_Backend/CURL_TESTING.md`
- Modify: `docs/plans/2026-03-08-issue-15-memory-persona-design.md`

**Step 1: Document new config and endpoint behavior**

Update docs for:

- dedicated post-turn provider/model configuration
- runtime portrait profile response
- persona change accept/reject endpoints
- admin runtime-memory inspection expectations

**Step 2: Run focused verification**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/store ./internal/agentx/... ./internal/api ./internal/admin -count=1
cd ../Nukara_Admin_Web && node tests/provider-api.spec.mjs
cd ../Nukara_Web && npx playwright test tests/bot-detail-runtime-portrait.spec.ts tests/persona-proactive-refactor.spec.ts --project=chromium
```

Expected:

- all targeted backend suites PASS
- admin API helper test PASS
- runtime portrait and persona regression Playwright tests PASS

**Step 3: Commit**

```bash
git add Nukara_Backend/README.md \
  docs/deployment-guide.md \
  Nukara_Backend/CURL_TESTING.md \
  docs/plans/2026-03-08-issue-15-memory-persona-design.md
git commit -m "docs: document runtime memory and living-state flow"
```
