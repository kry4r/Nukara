# Mobile Settings Tab Bar + Bot Detail Scroll Design

**Context**
- The mobile web app uses a single `#app-root` flex column layout with route content above `NavBar`.
- On `/settings`, the route content can expand without shrinking, which allows the page body to push the bottom tab bar out of view.
- On `/bots/:id`, the detail page visually looks like it should scroll, but the active scroll container does not consistently receive usable height on small screens.

**Approved Approach**
- Keep the bottom tab bar in normal document flow.
- Fix the problem at the layout/container level instead of switching to `position: fixed`.
- Ensure route-level pages that own internal scrolling explicitly support flex shrink and scroll delegation.

**Design**
- Add a stable app-content wrapper in `Nukara_Web/src/App.vue` so routed pages live inside a `flex: 1; min-height: 0;` region above the tab bar.
- Update `Nukara_Web/src/style.css` to make the wrapper the shared route viewport.
- Update `Nukara_Web/src/views/SettingsView.vue` so the page itself can shrink and its body remains the scroll container without pushing `NavBar` away.
- Update `Nukara_Web/src/views/BotDetailView.vue` so the detail page and `main.detail-main` both participate correctly in flex sizing and vertical scrolling.

**Why this approach**
- It is the smallest fix that preserves current visual structure.
- It avoids keyboard/safe-area regressions from converting the tab bar to fixed positioning.
- It centralizes the flex layout contract so other pages can benefit from the same shell behavior.

**Testing**
- Add a Playwright regression test that verifies `/settings` still shows the bottom nav on a mobile viewport.
- Add a Playwright regression test that verifies `/bots/:id` exposes a scrollable detail area on a mobile viewport.
