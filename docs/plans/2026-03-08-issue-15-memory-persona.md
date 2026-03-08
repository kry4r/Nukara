# Issue 15 Memory Persona Follow-up Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Finish the approved issue #15 persona-memory flow so low-risk lifestyle changes auto-land as summarized dynamic evolution and self-cognition, while high-risk core persona changes still require confirmation.

**Architecture:** Keep static persona fields stable and treat low-risk updates as dynamic events plus a summarized self-cognition refresh. Add a dedicated self-cognition summary model route with admin configuration, expose summarized recent evolution in the profile API, cap retained evolution history, and verify the end-to-end chat/admin flows with the real provider.

**Tech Stack:** Go backend, Vue web/admin frontends, Postgres stores, Playwright headed browser.

---

### Task 1: Lock expected backend behavior with tests

**Files:**
- Modify: `Nukara_Backend/internal/agentx/persona/risk_test.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/runner_test.go`
- Modify: `Nukara_Backend/internal/api/bot_profile_test.go`
- Modify: `Nukara_Backend/internal/admin/post_turn_config_handler_test.go`
- Modify: `Nukara_Backend/internal/agentx/runtime_test.go`

**Step 1:** Add failing tests for semantic risk classification and low-risk non-static application.
**Step 2:** Run targeted Go tests and verify the new assertions fail for the expected reason.
**Step 3:** Add failing tests for self-cognition summary route/config fallback and summarized recent change output.
**Step 4:** Run targeted Go tests again and verify red state stays focused.

### Task 2: Implement backend persona/memory flow

**Files:**
- Modify: `Nukara_Backend/internal/agentx/persona/risk.go`
- Modify: `Nukara_Backend/internal/agentx/subtasks/runner.go`
- Modify: `Nukara_Backend/internal/api/server.go`
- Modify: `Nukara_Backend/internal/api/subtasks_prompts.go`
- Modify: `Nukara_Backend/internal/api/bot_profile.go`
- Modify: `Nukara_Backend/internal/store/agentx_data.go`
- Modify: `Nukara_Backend/internal/store/postgres_agentx_data.go`
- Modify: `Nukara_Backend/internal/admin/post_turn_config_handler.go`
- Modify: `Nukara_Backend/internal/agentx/provider/router.go`
- Modify: `Nukara_Backend/internal/agentx/runtime.go`

**Step 1:** Implement semantic low/high risk classification with focused heuristics for habits/routines vs core identity.
**Step 2:** Refactor low-risk acceptance to create accepted evolution events and refresh self-cognition without mutating static persona fields.
**Step 3:** Add dedicated self-cognition summary routing/config fallback and prompt handling.
**Step 4:** Add summarized change text in API view and retention pruning to 20 events.
**Step 5:** Re-run targeted backend tests until green.

### Task 3: Update web and admin UI

**Files:**
- Modify: `Nukara_Web/src/views/BotDetailView.vue`
- Modify: `Nukara_Admin_Web/src/components/PostTurnModelPanel.vue`
- Create: `Nukara_Admin_Web/src/components/SummaryModelPanel.vue`
- Modify: `Nukara_Admin_Web/src/App.vue`
- Modify: `Nukara_Admin_Web/src/api/admin.js`

**Step 1:** Add a dedicated self-cognition card and use summarized recent-evolution text.
**Step 2:** Bound the recent-evolution list to a scrollable area sized for roughly five items.
**Step 3:** Add admin summary-model panel with provider/model sync API.
**Step 4:** Run focused frontend tests where available.

### Task 4: Verify live flows

**Files:**
- Verify runtime only; no required code changes.

**Step 1:** Run targeted Go tests for changed packages.
**Step 2:** Run headed Playwright checks against the current worktree services.
**Step 3:** Validate chat-triggered persona evolution, self-cognition display, admin config sync, and persisted impression/recent change behavior with the real provider.
