# Mobile Bot Form Scroll Fix Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the mobile bot creation page scroll correctly inside the fixed phone shell and keep the `创建` action reachable on phone-sized screens.

**Architecture:** Refactor the bot form page into a fixed header, a single scrollable content region, and a sticky bottom action area. Verify the behavior with a Playwright mobile regression that fails before the CSS/layout fix and passes after it.

**Tech Stack:** Vue 3, scoped CSS, Playwright.

---

### Task 1: Add a failing mobile regression test

**Files:**
- Modify: `Nukara_Web/tests/pencil-core-pages.spec.ts`

**Step 1: Write the failing test**
- Add a mobile viewport test for `/bots/new`.
- Assert the form page is visible.
- Assert the scroll region overflows vertically.
- Assert the submit button stays within the viewport bottom.

**Step 2: Run test to verify it fails**
Run: `cd Nukara_Web && npx playwright test tests/pencil-core-pages.spec.ts -g "bot form page keeps create action reachable on mobile"`
Expected: FAIL before the layout fix.

**Step 3: Write minimal implementation**
- No production changes in this task.

**Step 4: Re-run test to confirm failure is the intended one**
Run the same command and confirm the failing assertion points at bot form layout/visibility.

### Task 2: Refactor the bot form mobile layout

**Files:**
- Modify: `Nukara_Web/src/views/BotFormView.vue`

**Step 1: Implement the layout split**
- Keep the existing header.
- Make the page root `min-height: 0` and overflow-safe.
- Move the submit button into a dedicated sticky footer action container.
- Keep error messaging above the action.

**Step 2: Update styles**
- Make the content section the only scrollable region.
- Add bottom padding so the last field does not hide behind the sticky action or nav.
- Ensure the sticky action has a background and border for legibility.

**Step 3: Run the targeted test**
Run: `cd Nukara_Web && npx playwright test tests/pencil-core-pages.spec.ts -g "bot form page keeps create action reachable on mobile"`
Expected: PASS.

### Task 3: Run adjacent regression coverage

**Files:**
- Modify: `Nukara_Web/tests/pencil-core-pages.spec.ts` (no additional test changes expected)

**Step 1: Run related UI tests**
Run: `cd Nukara_Web && npx playwright test tests/pencil-core-pages.spec.ts -g "bot form page|app renders inside a fixed 9:16 phone shell"`
Expected: PASS.

**Step 2: Review the diff**
- Confirm only the bot form mobile layout and regression test changed.
