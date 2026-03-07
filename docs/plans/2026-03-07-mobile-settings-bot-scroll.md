# Mobile Settings Tab Bar + Bot Detail Scroll Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Keep the bottom tab bar visible on the mobile `/settings` page and restore vertical scrolling on the mobile bot detail page.

**Architecture:** Preserve the current app shell and bottom navigation structure, but make the routed content live inside an explicit flex child that can shrink correctly. Then tighten page-level flex/overflow contracts on the settings and bot detail views so internal scroll containers work on small screens.

**Tech Stack:** Vue 3, Vue Router, Vite, Playwright

---

### Task 1: Add failing mobile layout regression tests

**Files:**
- Modify: `Nukara_Web/tests/pencil-core-pages.spec.ts`

**Step 1: Write the failing test**
- Add one test that opens `/settings` on a mobile viewport and asserts the bottom navigation is visible in the viewport.
- Add one test that opens `/bots/:id` on a mobile viewport and asserts the detail page has a scrollable main area.

**Step 2: Run test to verify it fails**
Run: `cd Nukara_Web && npx playwright test tests/pencil-core-pages.spec.ts --config=playwright.config.ts`
Expected: FAIL on the new mobile assertions.

**Step 3: Write minimal implementation**
- No implementation in this task.

**Step 4: Run test to verify it still fails for the expected reason**
Run: `cd Nukara_Web && npx playwright test tests/pencil-core-pages.spec.ts --config=playwright.config.ts`
Expected: FAIL only on the new assertions.

### Task 2: Fix shared app-shell flex sizing

**Files:**
- Modify: `Nukara_Web/src/App.vue`
- Modify: `Nukara_Web/src/style.css`

**Step 1: Write the failing test**
- Covered by Task 1.

**Step 2: Run test to verify it fails**
- Covered by Task 1.

**Step 3: Write minimal implementation**
- Wrap `router-view` in a dedicated app-content container.
- Give the wrapper `flex: 1` and `min-height: 0` so route pages cannot push `NavBar` out of the visible shell.

**Step 4: Run test to verify partial progress**
Run: `cd Nukara_Web && npx playwright test tests/pencil-core-pages.spec.ts --config=playwright.config.ts`
Expected: At least the settings-page nav visibility regression is fixed, or the failure surface moves to the route-level layout.

### Task 3: Fix settings page shrink/scroll behavior

**Files:**
- Modify: `Nukara_Web/src/views/SettingsView.vue`

**Step 1: Write the failing test**
- Covered by Task 1.

**Step 2: Run test to verify it fails**
- Covered by Task 1.

**Step 3: Write minimal implementation**
- Ensure `.settings-page` can shrink inside the app shell with `min-height: 0`.
- Keep `.settings-body` as the internal scroll container.
- Avoid expanding the page beyond the shell height.

**Step 4: Run test to verify it passes**
Run: `cd Nukara_Web && npx playwright test tests/pencil-core-pages.spec.ts --config=playwright.config.ts`
Expected: The mobile `/settings` regression passes.

### Task 4: Fix bot detail scroll behavior

**Files:**
- Modify: `Nukara_Web/src/views/BotDetailView.vue`

**Step 1: Write the failing test**
- Covered by Task 1.

**Step 2: Run test to verify it fails**
- Covered by Task 1.

**Step 3: Write minimal implementation**
- Ensure `.detail-page` can shrink inside the shell with `min-height: 0`.
- Ensure `.detail-main` remains the vertical scroll container and receives usable height.

**Step 4: Run test to verify it passes**
Run: `cd Nukara_Web && npx playwright test tests/pencil-core-pages.spec.ts --config=playwright.config.ts`
Expected: The mobile `/bots/:id` scroll regression passes.

### Task 5: Final verification

**Files:**
- Verify only

**Step 1: Run targeted regression tests**
Run: `cd Nukara_Web && npx playwright test tests/pencil-core-pages.spec.ts --config=playwright.config.ts`
Expected: PASS

**Step 2: Run web build**
Run: `cd Nukara_Web && npm run build`
Expected: PASS

**Step 3: Commit**
```bash
git add docs/plans/2026-03-07-mobile-settings-bot-scroll-design.md docs/plans/2026-03-07-mobile-settings-bot-scroll.md Nukara_Web/src/App.vue Nukara_Web/src/style.css Nukara_Web/src/views/SettingsView.vue Nukara_Web/src/views/BotDetailView.vue Nukara_Web/tests/pencil-core-pages.spec.ts
git commit -m "Fix mobile settings nav and bot detail scroll"
```
