# Auth Reset And Streaming Fix Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make `reset data` fully invalidate old user sessions and improve chat streaming so protocol markers never leak and message chunks feel natural.

**Architecture:** Tighten authenticated request handling by verifying that every JWT-backed user still exists in persistent storage before allowing HTTP or WebSocket access. For streaming, normalize protocol markers out of live deltas and relax overly aggressive punctuation-based chunk splitting so realtime bot replies read like natural chat instead of comma-by-comma fragments.

**Tech Stack:** Go backend, Vue 3 + Pinia frontend, Playwright, Go tests.

---

### Task 1: Add failing auth invalidation tests

**Files:**
- Modify: `Nukara_Backend/internal/api/ws_chat_test.go`
- Modify: `Nukara_Backend/internal/api/server_test.go` or nearest auth/api test file

**Step 1: Write the failing HTTP auth test**
- Add a test proving a previously valid token becomes unauthorized when its user record no longer exists.
- Build the token through existing server helpers rather than hardcoding format.
- Assert an authenticated HTTP endpoint returns `401` and does not continue with the request.

**Step 2: Run test to verify it fails**
Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api -run TestAuthRejectsTokenForDeletedUser -count=1`
Expected: FAIL because token-only validation still accepts the deleted user.

**Step 3: Write the failing WebSocket auth test**
- Add a test proving `/ws/chat?token=...` rejects a token whose user no longer exists after reset.
- Assert upgrade does not succeed and the response is unauthorized.

**Step 4: Run test to verify it fails**
Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api -run TestWSChatRejectsDeletedUserToken -count=1`
Expected: FAIL because websocket auth still accepts token-only identity.

### Task 2: Implement persistent user existence checks

**Files:**
- Modify: `Nukara_Backend/internal/store/interface.go`
- Modify: `Nukara_Backend/internal/store/store.go`
- Modify: `Nukara_Backend/internal/store/postgres_store.go`
- Modify: `Nukara_Backend/internal/api/server.go`
- Modify: `Nukara_Web/src/composables/useApi.js`
- Modify: `Nukara_Web/src/composables/useWebSocket.js` (if websocket 401/logout handling needs tightening)

**Step 1: Add store-level user lookup by ID / existence API**
- Add a small explicit method for checking whether a user ID still exists in backing storage.
- Implement it in both in-memory and Postgres stores.

**Step 2: Enforce the check in auth flow**
- After token verification, reject requests when the user record no longer exists.
- Return a specific unauthorized error that the frontend can treat as expired/invalidated session.
- Reuse the same check for WebSocket auth because `/ws/chat` already uses `authUserID()`.

**Step 3: Tighten frontend logout behavior**
- On `401`, clear both saved token and saved user and redirect to `/auth`.
- If websocket connection reports unauthorized/closed-after-auth-failure, clear session consistently instead of leaving the UI in a half-logged-in state.

**Step 4: Run the focused auth tests**
Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api -run 'TestAuthRejectsTokenForDeletedUser|TestWSChatRejectsDeletedUserToken' -count=1`
Expected: PASS.

### Task 3: Add failing streaming regression tests

**Files:**
- Modify: `Nukara_Backend/internal/agentx/postprocess/split_test.go`
- Modify: `Nukara_Web/tests/...` or add a frontend unit/integration test if a stable place exists
- Optionally modify: `Nukara_Backend/internal/api/ws_chat_test.go`

**Step 1: Write the failing chunking test**
- Add a Go test proving plain commas / pauses do not split a short natural reply into many separate segments.
- Keep explicit `<<<MSG>>>` support intact.

**Step 2: Run test to verify it fails**
Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/postprocess -run TestSplitSegmentsDoesNotOverSplitNaturalReply -count=1`
Expected: FAIL because current `splitByPause()` splits on commas and similar punctuation.

**Step 3: Write the failing protocol-marker test**
- Add a test proving streaming-visible text strips `<<<MSG>>>` from live deltas before they reach the client-visible message buffer.
- If easiest, cover this at the postprocess/helper level with a focused sanitizer test.

**Step 4: Run test to verify it fails**
Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/postprocess -run TestStreamSanitizerStripsMessageBoundaryProtocol -count=1`
Expected: FAIL because the current sanitizer ignores `<<<MSG>>>`.

### Task 4: Implement natural streaming cleanup and chunking

**Files:**
- Modify: `Nukara_Backend/internal/agentx/postprocess/sanitize.go`
- Modify: `Nukara_Backend/internal/agentx/postprocess/split.go`
- Modify: `Nukara_Backend/internal/agentx/llm/client.go`
- Modify: `Nukara_Backend/internal/agentx/runtime.go`
- Modify: `Nukara_Web/src/stores/chat.js`

**Step 1: Strip protocol tokens from live deltas**
- Ensure stream sanitization removes or buffers `<<<MSG>>>` so the frontend never sees it rendered.
- Preserve explicit multi-message semantics in final persisted message splitting.

**Step 2: Relax overly aggressive chunking**
- Stop splitting natural replies on commas / pause punctuation for normal streaming.
- Keep sentence-ending punctuation and explicit `<<<MSG>>>` as the main split boundaries.
- Keep max-length fallback for very long single sentences.

**Step 3: Align frontend display cleanup**
- Make frontend stream text sanitization defensively strip any leaked `<<<MSG>>>` token.
- Keep existing hidden-thought cleanup intact.

**Step 4: Run focused streaming tests**
Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/postprocess -run 'TestSplitSegmentsDoesNotOverSplitNaturalReply|TestStreamSanitizerStripsMessageBoundaryProtocol' -count=1`
Expected: PASS.

### Task 5: Run adjacent verification coverage

**Files:**
- Modify only if test fixtures require it

**Step 1: Backend regression coverage**
Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/agentx/postprocess ./internal/agentx/llm -count=1`
Expected: PASS.

**Step 2: Frontend chat regression coverage**
Run: `cd Nukara_Web && npx playwright test tests/pencil-core-pages.spec.ts -g 'chat|bot form page|phone shell'`
Expected: PASS if existing chat shell tests remain stable.

**Step 3: Review diff**
- Confirm changes stay scoped to auth/session invalidation, stream cleanup, and related tests/docs.
