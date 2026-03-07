# Mobile Bot Form Scroll Design

**Problem**

On mobile-sized viewports, the bot creation page places the primary submit action too low in the layout. The form content does not provide a reliable, obvious scroll experience inside the fixed 9:16 phone shell, so users can get stuck above the `创建` button.

**Recommended Approach**

Use a fixed header + single scroll region + sticky action bar layout inside the bot form page.

- Keep the page header static at the top.
- Make the form body the only vertical scroll container.
- Move the submit button into a bottom sticky action area inside the page.
- Preserve enough bottom padding so the last field is still readable above the sticky action and bottom navigation.

**Why this approach**

- It directly solves the mobile usability issue instead of only increasing spacing.
- It is resilient across different phone heights and dynamic browser UI changes.
- It matches common mobile form behavior: content scrolls, action stays reachable.

**Validation**

Add a Playwright mobile regression that verifies:

- the bot form page renders in a 390×844 viewport,
- the main form scroll region has overflow,
- the submit button remains visible inside the viewport.
