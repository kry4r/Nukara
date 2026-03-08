# Temporal Memory Graph Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the current vector/topic runtime memory path with a single-database temporal memory graph that supports anthropomorphic recall, compact/session integration, low-token prompt cards, and admin explainability.

**Architecture:** The implementation introduces new Postgres-backed temporal graph tables and a new `internal/agentx/memorygraph` runtime package. Post-turn writes feed graph nodes and edges asynchronously, online chat recall uses a bounded seed-and-activation path to assemble prompt cards, and compact/session state is materialized into graph nodes for long-term recall. Rollout starts in shadow mode so the new graph can be verified before becoming the primary chat memory path.

**Tech Stack:** Go, PostgreSQL, `pgvector`, Vue 3, Playwright, existing admin/web test suites.

---

## Ground Rules

- Follow `@superpowers/test-driven-development` for every behavior change.
- Do not commit unless the user explicitly asks; use checkpoint diffs instead of auto-commits.
- Prefer small slices with runnable tests after each task.
- Keep the current mobile shell/chat split behavior untouched unless a task explicitly calls for a UI change.

### Task 1: Create the temporal graph schema and store API

**Files:**
- Create: `Nukara_Backend/migrations/009_temporal_memory_graph.sql`
- Create: `Nukara_Backend/internal/store/temporal_memory_graph.go`
- Create: `Nukara_Backend/internal/store/postgres_temporal_memory_graph.go`
- Create: `Nukara_Backend/internal/store/temporal_memory_graph_test.go`
- Modify: `Nukara_Backend/internal/store/interface.go`
- Modify: `Nukara_Backend/internal/store/store.go`
- Modify: `Nukara_Backend/internal/store/postgres_store.go`

**Step 1: Write the failing store tests**

Add tests for:

- creating nodes and edges
- listing nodes by bot/user
- updating `valid_to` and `status`
- storing and fetching `session_summary` nodes

Suggested test names:

```go
func TestTemporalMemoryGraphStore_CreateNodeAndEdge(t *testing.T) {}
func TestTemporalMemoryGraphStore_UpdateNodeValidity(t *testing.T) {}
func TestTemporalMemoryGraphStore_ListSessionSummaryNodes(t *testing.T) {}
```

**Step 2: Run the test to verify it fails**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/store -run 'TestTemporalMemoryGraphStore' -count=1
```

Expected: FAIL because the temporal graph store types and methods do not exist.

**Step 3: Write the minimal schema and store implementation**

Implement:

- `memory_nodes`
- `memory_edges`
- `memory_embeddings`
- `memory_cards`
- `activation_traces`

Expose store methods such as:

```go
type TemporalMemoryStore interface {
    CreateMemoryNode(node TemporalMemoryNode) (TemporalMemoryNode, error)
    UpdateMemoryNode(node TemporalMemoryNode) (TemporalMemoryNode, error)
    GetMemoryNode(id string) (TemporalMemoryNode, bool)
    ListMemoryNodes(userID, botID string, filter TemporalMemoryNodeFilter) []TemporalMemoryNode
    CreateMemoryEdge(edge TemporalMemoryEdge) (TemporalMemoryEdge, error)
    ListMemoryEdges(nodeIDs []string, filter TemporalMemoryEdgeFilter) []TemporalMemoryEdge
    UpsertMemoryCard(card MemoryCard) (MemoryCard, error)
    ListMemoryCards(userID, botID string, filter MemoryCardFilter) []MemoryCard
    SaveActivationTrace(trace ActivationTrace) (ActivationTrace, error)
}
```

**Step 4: Run the tests to verify they pass**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/store -run 'TestTemporalMemoryGraphStore' -count=1
```

Expected: PASS.

**Step 5: Checkpoint diff**

Review:

```bash
git diff -- Nukara_Backend/migrations/009_temporal_memory_graph.sql Nukara_Backend/internal/store/interface.go Nukara_Backend/internal/store/temporal_memory_graph.go Nukara_Backend/internal/store/postgres_temporal_memory_graph.go Nukara_Backend/internal/store/temporal_memory_graph_test.go
```

---

### Task 2: Introduce the new `memorygraph` domain package

**Files:**
- Create: `Nukara_Backend/internal/agentx/memorygraph/types.go`
- Create: `Nukara_Backend/internal/agentx/memorygraph/ingest.go`
- Create: `Nukara_Backend/internal/agentx/memorygraph/recall.go`
- Create: `Nukara_Backend/internal/agentx/memorygraph/cards.go`
- Create: `Nukara_Backend/internal/agentx/memorygraph/consolidate.go`
- Create: `Nukara_Backend/internal/agentx/memorygraph/recall_test.go`
- Create: `Nukara_Backend/internal/agentx/memorygraph/cards_test.go`
- Create: `Nukara_Backend/internal/agentx/memorygraph/consolidate_test.go`

**Step 1: Write the failing recall and card tests**

Cover:

- seed retrieval stays bounded
- graph activation prefers open promises over stale snapshots
- chain assembly returns ordered recall chains instead of flat nodes
- card assembly respects the configured prompt budget

Suggested test names:

```go
func TestRecall_BuildsBoundedActivationSet(t *testing.T) {}
func TestRecall_PrefersOpenLoopsUntilResolved(t *testing.T) {}
func TestCards_AssembleFixedBudgetPromptCards(t *testing.T) {}
func TestConsolidate_PromotesRepeatedEpisodesToHabit(t *testing.T) {}
```

**Step 2: Run the tests to verify they fail**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/memorygraph -run 'TestRecall|TestCards|TestConsolidate' -count=1
```

Expected: FAIL because the package does not exist yet.

**Step 3: Write the minimal package implementation**

Define core structs such as:

```go
type Cue struct { ... }
type Seed struct { ... }
type ActivationResult struct { ... }
type RecallChain struct { ... }
type PromptCard struct { ... }
```

Implement:

- cue parsing
- seed selection from vector + lexical + unresolved loops
- 1–2 hop graph activation
- chain assembly
- prompt-card assembly
- consolidation hooks for `habit`, `self_model`, and `session_summary`

**Step 4: Run the tests to verify they pass**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/memorygraph -run 'TestRecall|TestCards|TestConsolidate' -count=1
```

Expected: PASS.

**Step 5: Checkpoint diff**

Review:

```bash
git diff -- Nukara_Backend/internal/agentx/memorygraph
```

---

### Task 3: Wire post-turn extraction into graph ingestion

**Files:**
- Modify: `Nukara_Backend/internal/agentx/subtasks/runner.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/memory_extract.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/compact_update.go`
- Modify: `Nukara_Backend/internal/api/subtasks_runtime.go`
- Create: `Nukara_Backend/internal/agentx/subtasks/memorygraph_ingest_test.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/runner_test.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/memory_extract_test.go`

**Step 1: Write the failing ingestion tests**

Add tests for:

- episode/state/promise nodes created from one turn
- low-risk user facts become nodes
- high-risk user facts stay as evidence episodes only
- compact refresh creates or updates a `session_summary` node

Suggested test names:

```go
func TestRunner_WritesEpisodePromiseAndStateNodes(t *testing.T) {}
func TestRunner_DoesNotPromoteHighRiskUserFact(t *testing.T) {}
func TestRunner_MaterializesSessionSummaryNode(t *testing.T) {}
```

**Step 2: Run the tests to verify they fail**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/subtasks -run 'TestRunner_|TestMemoryExtract' -count=1
```

Expected: FAIL because the new ingestion path is not wired.

**Step 3: Implement minimal ingestion wiring**

- call the new `memorygraph` ingestion service from the post-turn runner
- keep existing behavior alive in shadow mode
- materialize `session_summary` from compact updates
- create initial edges from new nodes to session and prior state

**Step 4: Run the tests to verify they pass**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/subtasks -run 'TestRunner_|TestMemoryExtract' -count=1
```

Expected: PASS.

**Step 5: Checkpoint diff**

Review:

```bash
git diff -- Nukara_Backend/internal/agentx/subtasks/runner.go Nukara_Backend/internal/agentx/subtasks/memory_extract.go Nukara_Backend/internal/agentx/subtasks/compact_update.go Nukara_Backend/internal/agentx/subtasks
```

---

### Task 4: Switch runtime chat context to prompt cards from graph recall

**Files:**
- Modify: `Nukara_Backend/internal/api/runtime_context.go`
- Modify: `Nukara_Backend/internal/api/server.go`
- Modify: `Nukara_Backend/internal/api/chat_flow.go`
- Modify: `Nukara_Backend/internal/api/runtime_context_test.go`
- Modify: `Nukara_Backend/internal/api/runtime_semantic_memory_test.go`
- Create: `Nukara_Backend/internal/api/runtime_temporal_memory_graph_test.go`

**Step 1: Write the failing runtime tests**

Cover:

- main chat system prompt uses graph-backed prompt cards
- unresolved promises are present in the memory prompt section
- `session_summary` can bridge an older compacted thread into a current reply
- total memory text stays within a bounded character budget

Suggested test names:

```go
func TestBuildRuntimeContext_UsesTemporalMemoryCards(t *testing.T) {}
func TestBuildRuntimeContext_IncludesOpenLoopCard(t *testing.T) {}
func TestBuildRuntimeContext_UsesSessionSummaryBridge(t *testing.T) {}
func TestBuildRuntimeContext_BoundsPromptMemoryBudget(t *testing.T) {}
```

**Step 2: Run the tests to verify they fail**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api -run 'TestBuildRuntimeContext_|TestRuntimeTemporalMemoryGraph' -count=1
```

Expected: FAIL because runtime still uses the old recall path.

**Step 3: Implement the minimal runtime switch**

- replace `selectRuntimeMemories` with graph recall cards when the feature flag is enabled
- keep the old path as fallback during shadow mode
- render cards into stable prompt sections:
  - `当前自我认知`
  - `当前生活状态`
  - `与你相关的要点`
  - `还挂着的事`
  - `刚被想起的经历`

**Step 4: Run the tests to verify they pass**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api -run 'TestBuildRuntimeContext_|TestRuntimeTemporalMemoryGraph' -count=1
```

Expected: PASS.

**Step 5: Checkpoint diff**

Review:

```bash
git diff -- Nukara_Backend/internal/api/runtime_context.go Nukara_Backend/internal/api/runtime_context_test.go Nukara_Backend/internal/api/runtime_temporal_memory_graph_test.go
```

---

### Task 5: Add activation trace persistence and admin APIs

**Files:**
- Modify: `Nukara_Backend/internal/admin/server.go`
- Modify: `Nukara_Backend/internal/admin/memory_graph_handler.go`
- Create: `Nukara_Backend/internal/admin/memory_activation_trace_handler.go`
- Create: `Nukara_Backend/internal/admin/memory_activation_trace_handler_test.go`
- Modify: `Nukara_Backend/internal/admin/memory_graph_handler_test.go`

**Step 1: Write the failing admin handler tests**

Cover:

- graph API returns typed nodes and edges from the new temporal graph tables
- activation trace API returns cue, seeds, activated nodes, selected cards, and reply excerpt
- self-evolution view returns the `self_model` version chain

Suggested test names:

```go
func TestMemoryGraphHandler_UsesTemporalGraphTables(t *testing.T) {}
func TestMemoryActivationTraceHandler_ReturnsTracePayload(t *testing.T) {}
func TestMemoryGraphHandler_ReturnsSelfModelChain(t *testing.T) {}
```

**Step 2: Run the tests to verify they fail**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/admin -run 'TestMemoryGraphHandler|TestMemoryActivationTraceHandler' -count=1
```

Expected: FAIL because the handlers do not exist or still read old structures only.

**Step 3: Implement the minimal admin APIs**

- extend the memory graph handler to read `memory_nodes` and `memory_edges`
- add a trace handler for activation traces
- add `self_model` chain payload support

**Step 4: Run the tests to verify they pass**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/admin -run 'TestMemoryGraphHandler|TestMemoryActivationTraceHandler' -count=1
```

Expected: PASS.

**Step 5: Checkpoint diff**

Review:

```bash
git diff -- Nukara_Backend/internal/admin/server.go Nukara_Backend/internal/admin/memory_graph_handler.go Nukara_Backend/internal/admin/memory_activation_trace_handler.go Nukara_Backend/internal/admin
```

---

### Task 6: Update Admin Web graph and trace panels

**Files:**
- Modify: `Nukara_Admin_Web/src/api/admin.js`
- Modify: `Nukara_Admin_Web/src/components/MemoryGraphPanel.vue`
- Create: `Nukara_Admin_Web/src/components/MemoryActivationTracePanel.vue`
- Modify: `Nukara_Admin_Web/src/App.vue`
- Modify: `Nukara_Admin_Web/src/style.css`
- Modify: `Nukara_Admin_Web/src/utils/memory-graph-layout.js`
- Create: `Nukara_Admin_Web/tests/memory-activation-trace.spec.mjs`
- Modify: `Nukara_Admin_Web/tests/memory-graph-layout.spec.mjs`

**Step 1: Write the failing frontend tests**

Cover:

- graph panel renders typed node groups without text overflow
- activation trace panel renders cue → seeds → activation → cards
- self-model evolution chain is visible in the admin UI

Suggested test names:

```js
it('renders temporal graph node groups without overflow', () => {})
it('renders activation trace stages in order', () => {})
it('renders self model evolution chain metadata', () => {})
```

**Step 2: Run the tests to verify they fail**

Run:

```bash
cd Nukara_Admin_Web && npm test -- --runInBand 2>/dev/null || true
node --test tests/memory-graph-layout.spec.mjs tests/memory-activation-trace.spec.mjs
```

Expected: FAIL because the new trace panel and graph payload support do not exist.

**Step 3: Implement the minimal admin UI changes**

- support the new node and edge payloads
- add the activation trace panel
- keep node labels short and push full content into the detail pane
- preserve the existing left-column scroll fix

**Step 4: Run the tests to verify they pass**

Run:

```bash
cd Nukara_Admin_Web && node --test tests/memory-graph-layout.spec.mjs tests/memory-activation-trace.spec.mjs
npm run build
```

Expected: PASS.

**Step 5: Checkpoint diff**

Review:

```bash
git diff -- Nukara_Admin_Web/src/api/admin.js Nukara_Admin_Web/src/components/MemoryGraphPanel.vue Nukara_Admin_Web/src/components/MemoryActivationTracePanel.vue Nukara_Admin_Web/src/style.css Nukara_Admin_Web/tests
```

---

### Task 7: Add rollout flags, bootstrap wiring, and backfill path

**Files:**
- Modify: `Nukara_Backend/internal/bootstrap/bootstrap.go`
- Modify: `Nukara_Backend/internal/api/server.go`
- Create: `Nukara_Backend/internal/agentx/memorygraph/backfill.go`
- Create: `Nukara_Backend/internal/agentx/memorygraph/backfill_test.go`
- Create: `Nukara_Backend/cmd/memory_backfill/main.go`
- Modify: `Nukara_Backend/internal/bootstrap/bootstrap_test.go`

**Step 1: Write the failing bootstrap and backfill tests**

Cover:

- shadow mode wires the new services but keeps old recall readable
- graph mode makes runtime prefer the temporal graph path
- backfill migrates `memory_items`, runtime state, and compact data into new graph rows

Suggested test names:

```go
func TestBootstrap_WiresTemporalMemoryGraphShadowMode(t *testing.T) {}
func TestBootstrap_WiresTemporalMemoryGraphReadMode(t *testing.T) {}
func TestBackfill_MigratesLegacyMemoryAndCompacts(t *testing.T) {}
```

**Step 2: Run the tests to verify they fail**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/bootstrap ./internal/agentx/memorygraph -run 'TestBootstrap_|TestBackfill_' -count=1
```

Expected: FAIL because rollout flags and backfill do not exist.

**Step 3: Implement the minimal bootstrap and backfill wiring**

- add env/config flag such as `NUKARA_TEMPORAL_MEMORY_MODE=off|shadow|read`
- instantiate the new graph services in bootstrap
- expose a backfill command for old data

**Step 4: Run the tests to verify they pass**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/bootstrap ./internal/agentx/memorygraph -run 'TestBootstrap_|TestBackfill_' -count=1
```

Expected: PASS.

**Step 5: Checkpoint diff**

Review:

```bash
git diff -- Nukara_Backend/internal/bootstrap/bootstrap.go Nukara_Backend/internal/agentx/memorygraph/backfill.go Nukara_Backend/cmd/memory_backfill/main.go Nukara_Backend/internal/bootstrap/bootstrap_test.go
```

---

### Task 8: Run integration verification across backend and UI

**Files:**
- Modify as needed from prior tasks only
- Test: `Nukara_Backend/internal/api/runtime_temporal_memory_graph_test.go`
- Test: `Nukara_Backend/internal/admin/memory_activation_trace_handler_test.go`
- Test: `Nukara_Admin_Web/tests/memory-activation-trace.spec.mjs`

**Step 1: Add one end-to-end integration test if missing**

If the prior tasks do not already prove it, add a single integration test that shows:

- one chat turn writes graph nodes
- a later chat turn recalls via prompt cards
- admin trace endpoint exposes the same activation path

Suggested test name:

```go
func TestTemporalMemoryGraph_ChatRecallAndAdminTraceStayConsistent(t *testing.T) {}
```

**Step 2: Run focused integration tests**

Run:

```bash
cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api ./internal/admin ./internal/agentx/memorygraph -run 'TestTemporalMemoryGraph_|TestBuildRuntimeContext_|TestMemoryActivationTraceHandler' -count=1
cd ../Nukara_Admin_Web && node --test tests/memory-graph-layout.spec.mjs tests/memory-activation-trace.spec.mjs
```

Expected: PASS.

**Step 3: Run broader verification**

Run:

```bash
cd /Users/nidhogg/code/Nukara/.worktrees/issue-15-memory-persona/Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./...
cd /Users/nidhogg/code/Nukara/.worktrees/issue-15-memory-persona/Nukara_Admin_Web && npm run build
cd /Users/nidhogg/code/Nukara/.worktrees/issue-15-memory-persona/Nukara_Web && npm run build
```

Expected: PASS, or if unrelated existing failures remain, document them precisely and do not mask them.

**Step 4: Final checkpoint diff**

Review:

```bash
git status --short
git diff --stat
```

**Step 5: Optional commit only if the user asks**

If and only if the user explicitly requests a commit:

```bash
git add Nukara_Backend Nukara_Admin_Web docs/plans/2026-03-08-temporal-memory-graph-design.md docs/plans/2026-03-08-temporal-memory-graph-impl-plan.md
git commit -m "feat: add temporal memory graph runtime"
```
