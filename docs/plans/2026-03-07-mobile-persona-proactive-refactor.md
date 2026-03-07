# Mobile Persona & Proactive Refactor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor the web and backend stack so the app always renders as a fixed 9:16 phone UI, bots use the new five-field persona model, reply splitting becomes human-like, proactive messaging uses explicit interval choices, DND is enforced correctly, and locale-aware timing is inferred from life context.

**Architecture:** Use an additive database migration and compatibility helpers so the new persona fields become the system of record without breaking legacy callers. Implement backend contract changes first with tests, then update frontend views/stores to the new API shape, and finish with Playwright regression coverage for the end-to-end flows.

**Tech Stack:** Go, PostgreSQL migrations, Vue 3 + Pinia + Vite, Playwright, existing backend unit tests.

---

### Task 1: Add the new persona schema and backfill helpers

**Files:**
- Create: `Nukara_Backend/migrations/006_add_bot_persona_v2.sql`
- Modify: `Nukara_Backend/internal/store/store.go`
- Modify: `Nukara_Backend/internal/store/postgres_store.go`
- Modify: `Nukara_Backend/internal/store/interface.go`
- Test: `Nukara_Backend/internal/store/store_test.go`

**Step 1: Write the failing tests**

Add tests that expect `store.Bot` to expose:

```go
type Bot struct {
    Identity              string   `json:"identity"`
    Personality           []string `json:"personality"`
    ExpressionStyle       string   `json:"expression_style"`
    LifeContext           string   `json:"life_context"`
    TaboosAndPreferences  string   `json:"taboos_and_preferences"`
}
```

Add a backfill test that seeds legacy fields and expects compatibility helpers to derive the new fields without overwriting non-empty v2 fields.

**Step 2: Run test to verify it fails**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/store -run 'Test(BotPersonaV2Fields|BotPersonaV2Backfill)' -count=1`
Expected: FAIL because the fields/helpers do not exist yet.

**Step 3: Write minimal implementation**

- Add the five new fields to `store.Bot`.
- Add migration `006_add_bot_persona_v2.sql` that creates the new columns and backfills them from legacy columns using idempotent SQL.
- Add helper functions such as:

```go
func DerivePersonaV2FromLegacy(bot Bot) Bot
func SyncLegacyPersonaFields(bot Bot) Bot
```

- Update Postgres scan/insert/update statements to read/write the new columns.

**Step 4: Run test to verify it passes**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/store -run 'Test(BotPersonaV2Fields|BotPersonaV2Backfill)' -count=1`
Expected: PASS.

**Step 5: Commit**

```bash
git add Nukara_Backend/migrations/006_add_bot_persona_v2.sql Nukara_Backend/internal/store/store.go Nukara_Backend/internal/store/postgres_store.go Nukara_Backend/internal/store/interface.go Nukara_Backend/internal/store/store_test.go
git commit -m "feat: add persona v2 storage fields"
```

### Task 2: Migrate bot CRUD and profile APIs to the new contract

**Files:**
- Modify: `Nukara_Backend/internal/api/server.go`
- Modify: `Nukara_Backend/internal/api/bot_profile.go`
- Test: `Nukara_Backend/internal/api/bot_profile_test.go`
- Test: `Nukara_Backend/internal/api/agentx_runtime_test.go`

**Step 1: Write the failing tests**

Add API tests that expect bot create/read/update/profile responses to use:

```json
{
  "name": "苏子衿",
  "identity": "你的恋人，也是会认真接住你情绪的人",
  "personality": ["细腻", "敏锐"],
  "expression_style": "口语化，短句，会接梗",
  "life_context": "现在住在东京，平时摄影、通勤、喝便利店咖啡",
  "taboos_and_preferences": "不喜欢被命令式对待，更喜欢被温柔回应"
}
```

Add a test that `GET /api/v1/bots/:id/profile` returns the new fields in `bot` and no longer depends on `summary/background/speaking_style` for the primary UI contract.

**Step 2: Run test to verify it fails**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api -run 'Test(BotProfileEndpoints|BotCRUDUsesPersonaV2)' -count=1`
Expected: FAIL because handlers still parse and emit legacy shapes.

**Step 3: Write minimal implementation**

- Update bot request structs in `server.go` to accept the new persona fields.
- Update response payloads so `identity/personality/expression_style/life_context/taboos_and_preferences` are first-class.
- Use compatibility helpers to keep legacy prompt paths functioning.
- Ensure bot lists expose an identity summary for the frontend card subtitle.

**Step 4: Run test to verify it passes**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api -run 'Test(BotProfileEndpoints|BotCRUDUsesPersonaV2)' -count=1`
Expected: PASS.

**Step 5: Commit**

```bash
git add Nukara_Backend/internal/api/server.go Nukara_Backend/internal/api/bot_profile.go Nukara_Backend/internal/api/bot_profile_test.go Nukara_Backend/internal/api/agentx_runtime_test.go
git commit -m "feat: migrate bot APIs to persona v2"
```

### Task 3: Refactor persona compiler and iteration patches to the five-field model

**Files:**
- Modify: `Nukara_Backend/internal/agentx/persona/compiler.go`
- Modify: `Nukara_Backend/internal/agentx/persona/compiler_test.go`
- Modify: `Nukara_Backend/internal/agentx/persona/changes.go`
- Modify: `Nukara_Backend/internal/api/bot_profile.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/runner.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/runner_test.go`

**Step 1: Write the failing tests**

Add compiler tests that expect prompt output sections like:

```text
【身份设定】...
【性格特征】...
【表达风格】...
【生活环境】...
【禁忌与偏好】...
```

Add iteration tests that expect patch fields:

```json
{"identity_adds":[],"personality_adds":[],"expression_style_adds":[],"life_context_adds":[],"taboos_and_preferences_adds":[]}
```

**Step 2: Run test to verify it fails**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/persona ./internal/agentx/subtasks ./internal/api -run 'Test(CompilePrompt|BotProfileEndpoints|Runner)' -count=1`
Expected: FAIL because the compiler and patch structs still use legacy groupings.

**Step 3: Write minimal implementation**

- Replace compiler assembly with the five approved fields.
- Replace iterate patch structs with new add-lists.
- Update `buildIteratePrompt()` to ask for the new patch shape.
- When applying iterate results, append into persona-v2 fields first and sync legacy fields second.

**Step 4: Run test to verify it passes**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/persona ./internal/agentx/subtasks ./internal/api -run 'Test(CompilePrompt|BotProfileEndpoints|Runner)' -count=1`
Expected: PASS.

**Step 5: Commit**

```bash
git add Nukara_Backend/internal/agentx/persona/compiler.go Nukara_Backend/internal/agentx/persona/compiler_test.go Nukara_Backend/internal/agentx/persona/changes.go Nukara_Backend/internal/api/bot_profile.go Nukara_Backend/internal/agentx/subtasks/runner.go Nukara_Backend/internal/agentx/subtasks/runner_test.go
git commit -m "feat: move persona compiler and iteration to v2 fields"
```

### Task 4: Add locale inference from `life_context` and inject local time context

**Files:**
- Create: `Nukara_Backend/internal/api/locale_context.go`
- Create: `Nukara_Backend/internal/api/locale_context_test.go`
- Modify: `Nukara_Backend/internal/api/runtime_context.go`
- Modify: `Nukara_Backend/internal/agent/agent.go`
- Modify: `Nukara_Backend/internal/api/scheduler.go`

**Step 1: Write the failing tests**

Add tests such as:

```go
func TestInferLocaleContextDefaultsToChina(t *testing.T) {}
func TestInferLocaleContextFromLifeContext(t *testing.T) {}
```

Expected cases:
- empty `life_context` -> `Asia/Shanghai`
- mentions `东京`/`日本` -> Asia/Tokyo
- mentions `纽约`/`美国` -> America/New_York

Also verify runtime context contains values like `local_time`, `local_timezone`, and `day_phase`.

**Step 2: Run test to verify it fails**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api -run 'TestInferLocaleContext' -count=1`
Expected: FAIL because no locale inference helper exists.

**Step 3: Write minimal implementation**

Implement a deterministic helper:

```go
type LocaleContext struct {
    Timezone string
    Region   string
    LocalTime string
    DayPhase string
}

func InferLocaleContext(lifeContext string, now time.Time) LocaleContext
```

Inject the derived values into normal chat and proactive system context builders.

**Step 4: Run test to verify it passes**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api -run 'TestInferLocaleContext' -count=1`
Expected: PASS.

**Step 5: Commit**

```bash
git add Nukara_Backend/internal/api/locale_context.go Nukara_Backend/internal/api/locale_context_test.go Nukara_Backend/internal/api/runtime_context.go Nukara_Backend/internal/agent/agent.go Nukara_Backend/internal/api/scheduler.go
git commit -m "feat: infer locale context from life context"
```

### Task 5: Replace fixed rune slicing with semantic WeChat-style splitting

**Files:**
- Modify: `Nukara_Backend/internal/agent/agent.go`
- Modify: `Nukara_Backend/internal/agentx/llm/client.go`
- Modify: `Nukara_Backend/internal/agentx/postprocess/split.go`
- Modify: `Nukara_Backend/internal/agentx/postprocess/split_test.go`
- Modify: `Nukara_Backend/internal/api/ws_chat_test.go`

**Step 1: Write the failing tests**

Add splitting tests that expect:

- `"我刚下班。好累，不过想到你又好一点。"` -> 1-2 natural chunks, not 8-rune slices
- explicit multi-part protocol output -> exact chunk boundaries preserved
- short replies like `"嗯，那你先去吃饭"` stay single-message

Also extend websocket/chat tests to verify streamed bot chunks are grouped semantically.

**Step 2: Run test to verify it fails**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/postprocess ./internal/api -run 'Test(Split|WSChat)' -count=1`
Expected: FAIL because the code still uses fixed rune-window slicing.

**Step 3: Write minimal implementation**

- Extend `chatStyleSkill` to instruct the model to decide one-vs-many WeChat-style messages.
- Add a parser/splitter that prefers:
  1. explicit protocol boundaries
  2. line breaks and punctuation
  3. pause words
  4. length fallback
- Replace `splitByRuneWindow(reply, 8)` with semantic splitting.

**Step 4: Run test to verify it passes**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/postprocess ./internal/api -run 'Test(Split|WSChat)' -count=1`
Expected: PASS.

**Step 5: Commit**

```bash
git add Nukara_Backend/internal/agent/agent.go Nukara_Backend/internal/agentx/llm/client.go Nukara_Backend/internal/agentx/postprocess/split.go Nukara_Backend/internal/agentx/postprocess/split_test.go Nukara_Backend/internal/api/ws_chat_test.go
git commit -m "feat: stream semantic wechat-style reply chunks"
```

### Task 6: Upgrade proactive settings to explicit intervals and verify DND blocking

**Files:**
- Modify: `Nukara_Backend/internal/store/store.go`
- Modify: `Nukara_Backend/internal/store/postgres_store.go`
- Modify: `Nukara_Backend/internal/api/server.go`
- Modify: `Nukara_Backend/internal/api/scheduler.go`
- Modify: `Nukara_Backend/internal/api/scheduler_test.go`
- Modify: `Nukara_Backend/configs/api-contract.md`

**Step 1: Write the failing tests**

Add tests that expect notification settings to store an explicit minutes value, for example:

```json
{
  "proactive_enabled": true,
  "proactive_interval_minutes": 10,
  "dnd_start": "23:00",
  "dnd_end": "08:00"
}
```

Add scheduler tests that verify:
- `10` allows a 10-minute cooldown
- DND blocks proactive sends even when cooldown is satisfied
- manual proactive endpoint returns a blocked reason like `dnd_active`

**Step 2: Run test to verify it fails**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api ./internal/store -run 'Test(Scheduler|NotificationSettings)' -count=1`
Expected: FAIL because settings still use `high/normal/low`.

**Step 3: Write minimal implementation**

- Replace enum frequency storage with `ProactiveIntervalMinutes int`.
- Add migration logic in the store layer that maps old enum values to minutes during reads if needed.
- Update scheduler cooldown calculation to use explicit minutes.
- Update manual proactive response reasons for blocked sends.

**Step 4: Run test to verify it passes**

Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api ./internal/store -run 'Test(Scheduler|NotificationSettings)' -count=1`
Expected: PASS.

**Step 5: Commit**

```bash
git add Nukara_Backend/internal/store/store.go Nukara_Backend/internal/store/postgres_store.go Nukara_Backend/internal/api/server.go Nukara_Backend/internal/api/scheduler.go Nukara_Backend/internal/api/scheduler_test.go Nukara_Backend/configs/api-contract.md
git commit -m "feat: use explicit proactive intervals and dnd blocking"
```

### Task 7: Rebuild the frontend around the phone shell and persona-v2 forms

**Files:**
- Modify: `Nukara_Web/src/App.vue`
- Modify: `Nukara_Web/src/style.css`
- Modify: `Nukara_Web/src/stores/bots.js`
- Modify: `Nukara_Web/src/stores/settings.js`
- Modify: `Nukara_Web/src/utils/constants.js`
- Modify: `Nukara_Web/src/views/BotFormView.vue`
- Modify: `Nukara_Web/src/views/BotDetailView.vue`
- Modify: `Nukara_Web/src/views/BotsView.vue`
- Modify: `Nukara_Web/src/views/SettingsView.vue`
- Test: `Nukara_Web/tests/pencil-core-pages.spec.ts`

**Step 1: Write the failing tests**

Extend Playwright coverage so it expects:
- a fixed-ratio phone shell element on desktop and mobile viewports
- bot form labels for `身份设定 / 性格特征 / 表达风格 / 生活环境 / 禁忌与偏好`
- bot detail page to render the new sections
- settings to show interval options from 10 minutes to 5 hours

**Step 2: Run test to verify it fails**

Run: `cd Nukara_Web && npx playwright test tests/pencil-core-pages.spec.ts`
Expected: FAIL because the app still uses legacy labels and a width-based responsive shell.

**Step 3: Write minimal implementation**

- Add a dedicated phone-shell wrapper in `App.vue` and CSS similar to:

```vue
<div id="stage">
  <div class="phone-shell">
    <router-view />
    <NavBar v-if="showNav()" />
  </div>
</div>
```

- Replace the bot form/detail/list field mapping with the five new persona fields.
- Replace frequency options with minute values plus labels (`10分钟` … `5小时`).

**Step 4: Run test to verify it passes**

Run: `cd Nukara_Web && npx playwright test tests/pencil-core-pages.spec.ts`
Expected: PASS.

**Step 5: Commit**

```bash
git add Nukara_Web/src/App.vue Nukara_Web/src/style.css Nukara_Web/src/stores/bots.js Nukara_Web/src/stores/settings.js Nukara_Web/src/utils/constants.js Nukara_Web/src/views/BotFormView.vue Nukara_Web/src/views/BotDetailView.vue Nukara_Web/src/views/BotsView.vue Nukara_Web/src/views/SettingsView.vue Nukara_Web/tests/pencil-core-pages.spec.ts
git commit -m "feat: rebuild web ui for phone shell and persona v2"
```

### Task 8: Add end-to-end regression coverage and run final verification

**Files:**
- Create: `Nukara_Web/tests/persona-proactive-refactor.spec.ts`
- Modify: `Nukara_Web/tests/pencil-core-pages.spec.ts`
- Modify: `docs/deployment-guide.md`

**Step 1: Write the failing test**

Create a Playwright scenario that covers:
- bot detail iteration action returns success instead of `bot not found`
- settings persist `proactive_interval_minutes`
- DND UI still saves correctly
- chat screen keeps grouped message chunks visible in the fixed 9:16 shell

**Step 2: Run test to verify it fails**

Run: `cd Nukara_Web && npx playwright test tests/persona-proactive-refactor.spec.ts`
Expected: FAIL until the UI and API contracts are fully updated.

**Step 3: Write minimal implementation/documentation**

- Patch any remaining frontend or backend mismatches uncovered by the E2E test.
- Update `docs/deployment-guide.md` with the new persona-v2 fields, proactive interval setting, and DND behavior.

**Step 4: Run final verification**

Run these commands:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/store ./internal/api ./internal/agent ./internal/agentx/... ./internal/analysis
cd Nukara_Web && npm run build
cd Nukara_Web && npx playwright test tests/pencil-core-pages.spec.ts tests/persona-proactive-refactor.spec.ts
```

Expected: all pass.

**Step 5: Commit**

```bash
git add Nukara_Web/tests/persona-proactive-refactor.spec.ts Nukara_Web/tests/pencil-core-pages.spec.ts docs/deployment-guide.md
git commit -m "test: add regression coverage for persona proactive refactor"
```
