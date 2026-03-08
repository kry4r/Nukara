# Memory Persona System Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the legacy post-turn memory/persona flow with a clean pipeline that extracts stable facts, writes them into the temporal memory graph, updates persona automatically when justified, and removes pending confirmation from the product flow.

**Architecture:** Rebuild the orchestration path in `Nukara_Backend/internal/agentx/subtasks/runner.go` so graph facts become the source of truth and persona becomes a derived view. Reuse the existing temporal graph store and recall infrastructure, add candidate extraction + similarity + persona decision modules, and update backend/web surfaces to consume accepted audit events instead of pending review actions.

**Tech Stack:** Go, existing in-memory/postgres store layer, AgentX subtask runtime, OpenAI-compatible LLM + embeddings, Vue 3, Playwright.

---

### Task 1: Extend store models for candidate facts, entities, and graph metadata

**Files:**
- Modify: `Nukara_Backend/internal/store/agentx_data.go`
- Modify: `Nukara_Backend/internal/store/postgres_agentx_data.go`
- Modify: `Nukara_Backend/internal/store/temporal_memory_graph.go`
- Modify: `Nukara_Backend/internal/store/postgres_temporal_memory_graph.go`
- Modify: `Nukara_Backend/internal/store/interface.go`
- Test: `Nukara_Backend/internal/store/temporal_memory_graph_test.go`
- Test: `Nukara_Backend/internal/store/runtime_profile_test.go`

**Step 1: Write the failing store tests**

Cover these rules explicitly:

- `TemporalMemoryNode` persists `source_turn_id`, `source_kind`, `semantic_category`, `stability`, `merge_key`, and `evidence_count`
- `MemoryItem` persists `entities` and `relations`
- `PersonaChangeEvent` accepts `accepted`, `skipped`, and `failed` states

Suggested assertions:

```go
node, _ := st.CreateMemoryNode(store.TemporalMemoryNode{
    UserID: user.ID,
    BotID: bot.ID,
    SessionID: conv.ID,
    NodeType: "episode",
    Summary: "养了一只猫叫小蜜",
    SourceTurnID: "turn-1",
    SourceKind: "self_fact",
    SemanticCategory: "life_context",
    Stability: "stable",
    MergeKey: "cat:xiaomi",
    EvidenceCount: 1,
})
if node.SourceTurnID != "turn-1" || node.SemanticCategory != "life_context" {
    t.Fatalf("unexpected node metadata: %+v", node)
}
```

**Step 2: Run the focused store tests to verify failure**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/store -run 'TestTemporalMemoryGraphStore|TestRuntimeProfile' -count=1
```

Expected: FAIL because the new metadata fields and state handling do not exist yet.

**Step 3: Implement the minimal store changes**

Add the new entity/relation structs and metadata fields. Update both in-memory and postgres-backed store code so clone/copy/update paths preserve the new fields.

**Step 4: Re-run the focused store tests**

Run the same `go test` command from Step 2.

Expected: PASS.

**Step 5: Commit**

```bash
git add Nukara_Backend/internal/store/agentx_data.go \
  Nukara_Backend/internal/store/postgres_agentx_data.go \
  Nukara_Backend/internal/store/temporal_memory_graph.go \
  Nukara_Backend/internal/store/postgres_temporal_memory_graph.go \
  Nukara_Backend/internal/store/interface.go \
  Nukara_Backend/internal/store/temporal_memory_graph_test.go \
  Nukara_Backend/internal/store/runtime_profile_test.go
git commit -m "feat(store): extend memory graph metadata for persona redesign"
```

### Task 2: Rework memory extraction into candidate facts with stability and relation hints

**Files:**
- Modify: `Nukara_Backend/internal/agentx/subtasks/memory_extract.go`
- Modify: `Nukara_Backend/internal/api/subtasks_runtime.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/memory_extract_test.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/runtime_state_test.go`

**Step 1: Write the failing extraction tests**

Add cases for:

- `不是纯金融，更偏研究型` survives filtering and is tagged as stable `identity`
- `养了一只猫叫小蜜` produces entity/relation hints
- `最近住在爸妈这边` is tagged as temporary `life_context`
- low-value small talk is still filtered out

Suggested test skeleton:

```go
items, err := ParseMemoryItems(raw)
if err != nil {
    t.Fatal(err)
}
if items[0].SemanticCategory != "identity" || items[0].Stability != "stable" {
    t.Fatalf("unexpected classification: %+v", items[0])
}
```

**Step 2: Run the focused extraction tests to verify failure**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/subtasks -run 'TestMemoryExtract|TestRuntimeState' -count=1
```

Expected: FAIL because the richer candidate classification and entity extraction do not exist yet.

**Step 3: Implement the minimal extraction rewrite**

Update the subtask prompt and parsing logic so extracted items carry semantic category, stability, entities, and relations. Keep filtering intentionally narrow: only obvious low-value greetings and fillers should be dropped.

**Step 4: Re-run the focused extraction tests**

Run the same `go test` command from Step 2.

Expected: PASS.

**Step 5: Commit**

```bash
git add Nukara_Backend/internal/agentx/subtasks/memory_extract.go \
  Nukara_Backend/internal/api/subtasks_runtime.go \
  Nukara_Backend/internal/agentx/subtasks/memory_extract_test.go \
  Nukara_Backend/internal/agentx/subtasks/runtime_state_test.go
git commit -m "feat(agentx): extract candidate facts with stability and relation hints"
```

### Task 3: Add similarity-based graph ingestion and session summary fact materialization

**Files:**
- Create: `Nukara_Backend/internal/agentx/memorygraph/similarity.go`
- Modify: `Nukara_Backend/internal/agentx/memorygraph/ingest.go`
- Modify: `Nukara_Backend/internal/agentx/memorygraph/service.go`
- Modify: `Nukara_Backend/internal/agentx/memorygraph/service_test.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/memorygraph_ingest_test.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/compact_update.go`

**Step 1: Write the failing ingestion tests**

Cover:

- similar facts merge instead of duplicating
- low-similarity facts create a fresh node
- `compact_update` materializes one `session_summary` plus one node per fact
- `summarizes` edges connect the summary to the materialized fact nodes

Suggested test names:

```go
func TestService_IngestTurn_MergesHighlySimilarFacts(t *testing.T) {}
func TestService_IngestTurn_MaterializesSummaryFacts(t *testing.T) {}
```

**Step 2: Run the focused ingestion tests to verify failure**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/memorygraph ./internal/agentx/subtasks -run 'TestService_IngestTurn|TestRunner_MaterializesSessionSummaryNode' -count=1
```

Expected: FAIL because similarity merge/create behavior and summary fact materialization are not implemented yet.

**Step 3: Implement the minimal similarity + ingest changes**

Add a similarity calculator that first tries normalized exact match / lexical overlap and later accepts embeddings. Update ingestion so it can:

- merge into an existing node when similarity is high
- supersede old short-lived nodes when a stronger version arrives
- create summary fact nodes from `compact_update.facts[]`
- emit `summarizes`, `follows`, and relation edges

**Step 4: Re-run the focused ingestion tests**

Run the same `go test` command from Step 2.

Expected: PASS.

**Step 5: Commit**

```bash
git add Nukara_Backend/internal/agentx/memorygraph/similarity.go \
  Nukara_Backend/internal/agentx/memorygraph/ingest.go \
  Nukara_Backend/internal/agentx/memorygraph/service.go \
  Nukara_Backend/internal/agentx/memorygraph/service_test.go \
  Nukara_Backend/internal/agentx/subtasks/memorygraph_ingest_test.go \
  Nukara_Backend/internal/agentx/subtasks/compact_update.go
git commit -m "feat(memorygraph): merge similar facts and materialize session summary facts"
```

### Task 4: Replace legacy persona iteration with decision-based auto-apply and audit logging

**Files:**
- Create: `Nukara_Backend/internal/agentx/memorygraph/persona_decision.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/runner.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/runner_test.go`
- Modify: `Nukara_Backend/internal/api/bot_profile.go`
- Modify: `Nukara_Backend/internal/api/server.go`
- Modify: `Nukara_Backend/internal/api/bot_profile_test.go`
- Modify: `Nukara_Backend/internal/api/persona_change_actions_test.go`

**Step 1: Write the failing persona-decision tests**

Cover these rules explicitly:

- stable `self_fact` identity updates are auto-applied
- temporary life-context facts are written to graph but skipped for persona
- `user_fact` stays out of bot persona unless it is a taboo/boundary
- accepted/skipped/failed events are recorded without `pending` routing

Suggested assertion:

```go
result, err := runner.Run(ctx, in)
if err != nil {
    t.Fatal(err)
}
if !result.PersonaUpdated {
    t.Fatalf("expected persona to auto-update")
}
```

**Step 2: Run the focused persona tests to verify failure**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/subtasks ./internal/api -run 'TestRunner_|TestBotProfile|TestPersonaChangeActions' -count=1
```

Expected: FAIL because the runner still uses `turnCount%3`, `PersonaIterator`, and pending-confirm behavior.

**Step 3: Implement the minimal persona rewrite**

Remove the old trigger in `runner.go`, compute persona decisions from saved graph facts, and auto-apply patches through `ApplyBotPersonaPatch`. Keep `persona_change_events` as an audit trail only. Update bot profile responses so `recent_changes` shows accepted/skipped/failed events and `pending_persona_changes` is no longer required.

**Step 4: Re-run the focused persona tests**

Run the same `go test` command from Step 2.

Expected: PASS.

**Step 5: Commit**

```bash
git add Nukara_Backend/internal/agentx/memorygraph/persona_decision.go \
  Nukara_Backend/internal/agentx/subtasks/runner.go \
  Nukara_Backend/internal/agentx/subtasks/runner_test.go \
  Nukara_Backend/internal/api/bot_profile.go \
  Nukara_Backend/internal/api/server.go \
  Nukara_Backend/internal/api/bot_profile_test.go \
  Nukara_Backend/internal/api/persona_change_actions_test.go
git commit -m "feat(agentx): auto-apply persona decisions from stable memory facts"
```

### Task 5: Inject relationship-based runtime context and remove pending-confirm UI from the web app

**Files:**
- Modify: `Nukara_Backend/internal/api/runtime_context.go`
- Modify: `Nukara_Backend/internal/api/runtime_context_test.go`
- Modify: `Nukara_Backend/internal/api/runtime_temporal_memory_graph_test.go`
- Modify: `Nukara_Web/src/views/BotDetailView.vue`
- Modify: `Nukara_Web/tests/bot-detail-runtime-portrait.spec.ts`

**Step 1: Write the failing runtime and web tests**

Cover:

- runtime prompt includes relationship guidance like “叔叔阿姨 = 你的父母” and “小蜜 = 你养的猫”
- bot detail page renders recent auto-applied changes without pending actions
- web test no longer clicks accept/reject

Suggested prompt assertion:

```go
if !strings.Contains(prompt, "用户提到的“叔叔阿姨”是指你的父母") {
    t.Fatalf("missing relationship guidance: %s", prompt)
}
```

**Step 2: Run the focused backend and web tests to verify failure**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api -run 'TestRuntime' -count=1
cd ../Nukara_Web && npx playwright test tests/bot-detail-runtime-portrait.spec.ts
```

Expected: FAIL because runtime guidance and the new web presentation do not exist yet.

**Step 3: Implement the minimal runtime + web rewrite**

Use graph entities/relations to build a small relationship-context section in the system prompt. In the web detail page, remove the pending-confirm block entirely and present only runtime state, key memories, self cognition, and recent auto-applied persona changes.

**Step 4: Re-run the focused backend and web tests**

Run the same commands from Step 2.

Expected: PASS.

**Step 5: Commit**

```bash
git add Nukara_Backend/internal/api/runtime_context.go \
  Nukara_Backend/internal/api/runtime_context_test.go \
  Nukara_Backend/internal/api/runtime_temporal_memory_graph_test.go \
  Nukara_Web/src/views/BotDetailView.vue \
  Nukara_Web/tests/bot-detail-runtime-portrait.spec.ts
git commit -m "feat(runtime): inject relationship context and remove pending persona UI"
```

### Task 6: Run cross-module verification and update implementation docs if behavior shifted

**Files:**
- Verify: `docs/plans/2026-03-08-memory-persona-system-implementation-design.md`
- Verify: `docs/plans/2026-03-08-memory-persona-system-impl-plan.md`
- Verify: touched backend and web files from Tasks 1-5

**Step 1: Run the backend regression suite covering touched modules**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/store ./internal/agentx/memorygraph ./internal/agentx/subtasks ./internal/api -count=1
```

Expected: PASS.

**Step 2: Run the targeted web regression**

Run:

```bash
cd Nukara_Web && npx playwright test tests/bot-detail-runtime-portrait.spec.ts
```

Expected: PASS.

**Step 3: Inspect the final diff**

Run:

```bash
git diff -- docs/plans/2026-03-08-memory-persona-system-implementation-design.md \
  docs/plans/2026-03-08-memory-persona-system-impl-plan.md \
  Nukara_Backend/internal/store \
  Nukara_Backend/internal/agentx/subtasks \
  Nukara_Backend/internal/agentx/memorygraph \
  Nukara_Backend/internal/api \
  Nukara_Web/src/views/BotDetailView.vue \
  Nukara_Web/tests/bot-detail-runtime-portrait.spec.ts
```

Expected: only the intended redesign changes are present.

**Step 4: Commit**

```bash
git add docs/plans/2026-03-08-memory-persona-system-implementation-design.md \
  docs/plans/2026-03-08-memory-persona-system-impl-plan.md \
  Nukara_Backend/internal/store \
  Nukara_Backend/internal/agentx/subtasks \
  Nukara_Backend/internal/agentx/memorygraph \
  Nukara_Backend/internal/api \
  Nukara_Web/src/views/BotDetailView.vue \
  Nukara_Web/tests/bot-detail-runtime-portrait.spec.ts
git commit -m "feat: ship memory persona system redesign"
```
