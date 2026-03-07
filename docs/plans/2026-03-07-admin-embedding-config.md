# Admin Embedding Config Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a dedicated admin configuration area for embedding model `base_url`, `api_key`, and `model` so embeddings can use an independent small model instead of sharing chat provider credentials.

**Architecture:** Keep the existing provider pool for chat intact, but add an explicit embedding configuration layer in system settings. The backend embedding route should prefer dedicated embedding credentials first, then fall back to `embedding_provider_id` / `embedding_model`, and finally to the default chat provider if no embedding-specific config exists. The admin UI should expose this as a separate panel with clear wording that it only affects vectorization / memory retrieval, not chat generation.

**Tech Stack:** Go backend, Vue 3 admin frontend, existing admin REST API, Go tests.

---

### Task 1: Add failing backend routing tests for dedicated embedding config

**Files:**
- Modify: `Nukara_Backend/internal/agentx/provider/router_test.go`
- Modify: `Nukara_Backend/internal/agentx/llm/openai_compat_embeddings_test.go` (create if missing)

**Step 1: Write the failing route precedence test**
- Add a test proving `ResolveEmbeddingRoute()` prefers dedicated embedding `base_url` / `api_key` / `model` settings over shared provider credentials.
- Keep a second assertion that fallback to `embedding_provider_id` still works when dedicated settings are absent.

**Step 2: Run test to verify it fails**
Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/provider -run TestResolveEmbeddingRoutePrefersDedicatedEmbeddingConfig -count=1`
Expected: FAIL because router currently only uses provider-based settings.

**Step 3: Write the failing embedder request test**
- Add a test proving the embedder posts to `<embedding_base_url>/embeddings` with the dedicated embedding token.
- Assert it does not silently reuse chat provider credentials when embedding-specific settings are present.

**Step 4: Run test to verify it fails**
Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/llm -run TestOpenAICompatEmbedderUsesDedicatedEmbeddingConfig -count=1`
Expected: FAIL before implementation.

### Task 2: Implement backend embedding config settings

**Files:**
- Modify: `Nukara_Backend/internal/agentx/provider/router.go`
- Modify: `Nukara_Backend/internal/store/store.go`
- Modify: `Nukara_Backend/internal/store/postgres_store.go`
- Modify: `Nukara_Backend/internal/store/agentx_data.go`
- Modify: `Nukara_Backend/internal/store/postgres_agentx_data.go`
- Modify: `Nukara_Backend/internal/admin/user_provider_handler.go`
- Modify: `Nukara_Backend/internal/admin/provider_handler.go` (or add a small dedicated admin handler file)

**Step 1: Add dedicated system setting keys**
- Support these keys in system settings defaults and reads:
  - `embedding_base_url`
  - `embedding_api_key`
  - `embedding_model`
  - keep `embedding_provider_id` as optional fallback

**Step 2: Extend embedding route resolution**
- Update embedding route construction so it prefers dedicated embedding config first.
- If dedicated `base_url` exists, use it and its token/model directly.
- If not, fall back to provider-based embedding selection.

**Step 3: Expose embedding config through admin API**
- Extend the existing admin settings payload (currently returned by `/api/admin/users/provider-settings`) or add a dedicated endpoint for embedding config.
- Include read and write support for:
  - `embedding_base_url`
  - `embedding_api_key`
  - `embedding_model`
  - `embedding_provider_id`
- Return masked/safe values if needed, but preserve editability expectations.

**Step 4: Run focused backend tests**
Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/provider ./internal/agentx/llm -count=1`
Expected: PASS.

### Task 3: Add failing admin frontend regression coverage

**Files:**
- Modify: `Nukara_Admin_Web/src/App.vue`
- Modify: `Nukara_Admin_Web/src/api/admin.js`
- Add test only if an adjacent stable test setup already exists; otherwise rely on targeted manual/browser verification

**Step 1: Identify the smallest stable verification path**
- If the admin app already has a test harness, add a failing test for the embedding config form.
- If not, document a focused manual verification path in the plan and keep the implementation minimal.

**Step 2: Define failing behavior to cover**
- Embedding settings panel is missing.
- Admin cannot save independent embedding `base_url` / `api_key` / `model`.
- Saved values are not reflected after refresh.

### Task 4: Implement admin embedding config UI

**Files:**
- Modify: `Nukara_Admin_Web/src/App.vue`
- Modify: `Nukara_Admin_Web/src/api/admin.js`

**Step 1: Add embedding config state + API calls**
- Add frontend state for embedding base URL, API key, model, and optional fallback provider.
- Parse these fields from the admin settings payload returned by the backend.

**Step 2: Add a dedicated Embedding Config panel**
- Place it near the existing default provider summary.
- Label it clearly as “仅用于 embedding / 记忆检索，不影响聊天回复模型”.
- Fields:
  - `Embedding Base URL`
  - `Embedding API Key`
  - `Embedding Model`
  - optional `Fallback Provider`
- Add a save action with status/error feedback.

**Step 3: Keep UX safe and explicit**
- Do not silently overwrite chat default provider settings.
- Keep chat provider switching behavior unchanged.
- Make it visually obvious this is a separate config path.

**Step 4: Verify in browser / targeted checks**
- Load admin page.
- Save embedding settings.
- Refresh and confirm values round-trip.
- Confirm chat provider panel still works unchanged.

### Task 5: Run verification before completion

**Files:**
- No new files expected beyond tests/docs above

**Step 1: Backend verification**
Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/provider ./internal/agentx/llm ./internal/admin -count=1`
Expected: PASS.

**Step 2: Frontend/admin verification**
Run the project-appropriate admin verification command if one exists; otherwise perform a manual browser verification against the local admin page and record the exact steps/results.
Expected: Embedding config panel saves and reloads correctly.

**Step 3: Review diff**
- Confirm scope is limited to dedicated embedding config support.
- Confirm chat provider routing remains unchanged unless embedding-specific settings are present.
