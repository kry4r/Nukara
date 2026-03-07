# Chat Unread, Proactive Delivery, and Admin Scroll Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix unread badge clearing, preserve proactive message delivery outside the chat page, prevent proactive messages from being inserted into the wrong active thread, and restore access to the Admin provider save button on desktop web.

**Architecture:** Keep message persistence and unread counts in the backend conversation store, but move websocket session ownership on the frontend from the route-local chat page to an app-level lifecycle so the user keeps receiving message events after leaving a conversation. The chat UI should only render message events that belong to the currently open conversation, while the conversation list should still react to all incoming events and unread count changes. The Admin layout should remain desktop-first and remove the clipping/scroll constraint that currently hides the lower part of the provider form.

**Tech Stack:** Go backend, Vue 3 + Pinia frontend, existing websocket chat channel, Vite builds, Go tests.

---

### Task 1: Add failing regression tests for unread clearing and proactive routing boundaries

**Files:**
- Modify: `Nukara_Backend/internal/api/ws_chat_test.go`
- Modify: `Nukara_Web/src/stores/chat.js` (behavior target only; no tests unless existing stable harness appears)
- Modify: `Nukara_Backend/internal/store/postgres_store.go`

**Step 1: Write failing backend regression for read state after entering a conversation**
- Add a test proving that after the client marks a conversation as read, the next conversation listing returns `unread_count = 0` for that conversation.
- Add a second test proving that a new proactive/bot message increments unread again only when the conversation is not currently marked read by the active client flow.

**Step 2: Run focused backend tests to verify failure if needed**
Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api -run 'TestWS|TestConversation' -count=1`
Expected: expose current unread behavior mismatch or guard future regression.

**Step 3: Add failing boundary test for proactive event separation**
- Add a backend/frontend-facing regression proving proactive messages preserve their `conversation_id` and are not treated as current-thread messages unless the open conversation matches.
- Use existing websocket tests where possible to pin the event shape and conversation routing assumptions.

**Step 4: Run the focused websocket tests**
Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api -run TestWSChat -count=1`
Expected: PASS or targeted failure revealing the current leak boundary.

### Task 2: Fix unread clearing and keep message delivery alive outside ChatView

**Files:**
- Modify: `Nukara_Web/src/stores/chat.js`
- Modify: `Nukara_Web/src/stores/conversations.js`
- Modify: `Nukara_Web/src/views/ChatView.vue`
- Modify: `Nukara_Web/src/App.vue` or add a dedicated top-level websocket coordinator/composable
- Modify: `Nukara_Web/src/composables/useWebSocket.js`

**Step 1: Move websocket ownership to app-level lifecycle**
- Create or reuse a single user-level websocket connection that survives route changes while the user stays logged in.
- Do not disconnect the websocket just because the user leaves `/chat/:convId`.

**Step 2: Mark read immediately on chat entry and sync local list state**
- When entering a conversation, call the existing `mark-read` API immediately after loading messages.
- Update the local conversation list store so the unread badge disappears without waiting for a later refresh.

**Step 3: Keep list updates global while message rendering stays scoped**
- Conversation list store should react to incoming bot/proactive events for any conversation.
- Chat message list should only append/render payloads whose `conversation_id` matches the currently open conversation.

**Step 4: Preserve realtime updates when user is outside the chat page**
- While the websocket remains connected, proactive messages and unread count changes should still update the conversation list/home page.
- Do not require the user to open the conversation once before the first proactive message becomes visible.

**Step 5: Run targeted frontend and backend checks**
Run backend: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./internal/api ./internal/store -count=1`
Run web build: `cd Nukara_Web && npm run build`
Expected: PASS.

### Task 3: Prevent proactive messages from being inserted into the wrong active thread

**Files:**
- Modify: `Nukara_Web/src/stores/chat.js`
- Modify: `Nukara_Web/src/stores/conversations.js`
- Modify: `Nukara_Backend/internal/api/chat_flow.go` only if event metadata needs tightening

**Step 1: Tighten current-thread filtering**
- Ensure `handleMessage`, `handleProactiveMessage`, streaming handlers, and reply-group completion logic ignore events for non-active conversations when mutating the visible chat thread.
- Still allow those events to update conversation previews/unread counts in the conversation list store.

**Step 2: Avoid accidental mark-read on unrelated reply groups**
- Make sure any `mark-read` call triggered from stream/multi-reply completion only applies when the completed conversation matches the active conversation.

**Step 3: Verify with manual/browser reproduction**
- Open one conversation.
- Trigger or simulate a proactive message for a different conversation.
- Confirm the current thread does not get a stray inserted bot message, but the conversation list preview/unread state updates.

### Task 4: Restore access to the Admin provider save button on desktop web

**Files:**
- Modify: `Nukara_Admin_Web/src/style.css`
- Modify: `Nukara_Admin_Web/src/App.vue` only if layout structure needs a minimal wrapper adjustment

**Step 1: Identify the clipping container**
- Remove or relax the `overflow: hidden` / fixed-height combination in the left provider panel that prevents scrolling to the bottom of the create/edit form.
- Keep Admin optimized for desktop web only; do not add mobile-specific admin layout work.

**Step 2: Ensure desktop scrolling reaches all provider actions**
- The provider list may remain scrollable, but the overall left column/card must allow access to the lower action area and save buttons.

**Step 3: Verify in browser**
- Open Admin on desktop width.
- Expand/create a provider entry.
- Confirm the save button is reachable and clickable without hidden overflow.

### Task 5: Full verification and review

**Files:**
- No new files beyond the implementation and tests above

**Step 1: Backend full verification**
Run: `cd Nukara_Backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./...`
Expected: PASS.

**Step 2: Frontend verification**
Run: `cd Nukara_Web && npm run build`
Run: `cd Nukara_Admin_Web && npm run build`
Expected: PASS.

**Step 3: Browser verification**
- Verify unread badge clears on chat open.
- Verify first proactive message appears in the conversation list even if the user never opened the thread.
- Verify leaving the chat page does not break incoming message delivery to the conversation list.
- Verify proactive messages from other conversations do not appear inside the current thread.
- Verify Admin provider form can scroll to the save button on desktop.

**Step 4: Independent review**
- Review the diff for route-lifecycle websocket regressions, unread count correctness, and Admin layout clipping.
- Confirm scope is limited to the requested bug fixes.
