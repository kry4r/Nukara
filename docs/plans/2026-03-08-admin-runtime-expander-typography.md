# Admin Runtime Expander Typography Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将后台左侧的 `Runtime Default` 改为真正的 expander，并缩小下方配置卡片的字体层级，让信息结构更自然。

**Architecture:** 保留现有 provider 数据和展开状态逻辑，把左栏 provider 列表收拢进 `Runtime Default` 卡片内部，新增独立的列表显隐状态。通过一个轻量源代码回归脚本约束模板结构和字体尺寸，避免回归到“列表在上、摘要在下”的布局。

**Tech Stack:** Vue 3 SFC、Vite、Node.js 轻量测试脚本

---

### Task 1: 锁定 Runtime Default 结构

**Files:**
- Modify: `Nukara_Admin_Web/src/App.vue`
- Modify: `Nukara_Admin_Web/src/style.css`
- Test: `Nukara_Admin_Web/tests/runtime-default-panel.spec.mjs`

**Step 1: Write the failing test**
- 断言 `App.vue` 存在 `providerListOpen` 状态
- 断言 `Runtime Default` 下方存在 `runtime-default-expander`
- 断言 `provider-card-list` 位于该 expander 内

**Step 2: Run test to verify it fails**

Run: `node Nukara_Admin_Web/tests/runtime-default-panel.spec.mjs`
Expected: FAIL because current source still keeps provider list above summary

**Step 3: Write minimal implementation**
- 将 provider 列表移动到 `Runtime Default` 卡片内部
- 点击 `Runtime Default` 按钮时控制列表展开/收起
- 展开时默认聚焦当前 runtime default provider

**Step 4: Run test to verify it passes**

Run: `node Nukara_Admin_Web/tests/runtime-default-panel.spec.mjs`
Expected: PASS

### Task 2: 收紧配置卡片字体层级

**Files:**
- Modify: `Nukara_Admin_Web/src/App.vue`
- Modify: `Nukara_Admin_Web/src/style.css`
- Modify: `Nukara_Admin_Web/src/components/EmailAuthPanel.vue`
- Modify: `Nukara_Admin_Web/src/components/PostTurnModelPanel.vue`
- Modify: `Nukara_Admin_Web/src/components/SummaryModelPanel.vue`
- Test: `Nukara_Admin_Web/tests/runtime-default-panel.spec.mjs`

**Step 1: Extend failing test**
- 断言配置卡片标题/eyebrow/描述字体使用更小的目标值

**Step 2: Run test to verify it fails**

Run: `node Nukara_Admin_Web/tests/runtime-default-panel.spec.mjs`
Expected: FAIL because current font-size remains too large

**Step 3: Write minimal implementation**
- 缩小 `h2`、`panel-eyebrow`、`panel-desc` 等层级
- 保持输入框和按钮可读性

**Step 4: Run test to verify it passes**

Run: `node Nukara_Admin_Web/tests/runtime-default-panel.spec.mjs`
Expected: PASS

### Task 3: 验证并发布

**Files:**
- Verify: `Nukara_Admin_Web/tests/runtime-default-panel.spec.mjs`
- Verify: `Nukara_Admin_Web/tests/provider-panel-state.spec.mjs`
- Verify: `Nukara_Admin_Web/tests/memory-graph-layout.spec.mjs`
- Verify: `Nukara_Admin_Web/tests/provider-api.spec.mjs`
- Verify: `Nukara_Admin_Web/package.json`

**Step 1: Run targeted tests**

Run: `node Nukara_Admin_Web/tests/runtime-default-panel.spec.mjs && node Nukara_Admin_Web/tests/provider-panel-state.spec.mjs && node Nukara_Admin_Web/tests/memory-graph-layout.spec.mjs && node Nukara_Admin_Web/tests/provider-api.spec.mjs`
Expected: PASS

**Step 2: Run build**

Run: `cd Nukara_Admin_Web && npm run build`
Expected: exit 0

**Step 3: Commit and push**

Run:
```bash
git add docs/plans/2026-03-08-admin-runtime-expander-typography.md \
  Nukara_Admin_Web/src/App.vue \
  Nukara_Admin_Web/src/style.css \
  Nukara_Admin_Web/src/components/EmailAuthPanel.vue \
  Nukara_Admin_Web/src/components/PostTurnModelPanel.vue \
  Nukara_Admin_Web/src/components/SummaryModelPanel.vue \
  Nukara_Admin_Web/tests/runtime-default-panel.spec.mjs

git commit -m "fix: make runtime default a real expander"
git push origin main
```
