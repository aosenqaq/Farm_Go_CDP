# Remote Mobile Social Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the remote LAN WebUI overview, friend feature sheets, and rankings readable and scrollable on 360px-412px phones without changing desktop or social-runtime behavior.

**Architecture:** Keep all behavior in existing React components. Add remote-only phone rules beneath `.app-shell-remote` and make the rankings list the sole scroll owner inside the existing social dialog. Use existing CSS-contract tests to prevent future cascade regressions; no Wails or domain APIs change.

**Tech Stack:** React, TypeScript, Vitest, CSS Grid, existing Vite build.

---

## File Structure

- Modify `frontend/src/style.css`: add final, scoped remote-phone rules for the overview's page height/event rows and the social Sheet/ranking scroll boundary.
- Modify `frontend/src/views/OverviewView.test.tsx`: assert the overview remains above the fixed remote navigation and event text cannot overlap.
- Modify `frontend/src/views/social/SocialRankingDialog.test.tsx`: assert mobile ranking controls, Sheet containment, row order, and list-only scrolling.

### Task 1: Lock In Remote Overview Constraints

**Files:**
- Modify: `frontend/src/views/OverviewView.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write the failing overview CSS-contract assertions**

In the existing `uses responsive semantic workbench layout rules` test, add declarations for the remote main content, view stack, event time and event copy. Assert that the final 760px rules include the following properties:

```tsx
const narrowMainContent = ruleDeclarations(narrowStyles, '.app-shell-remote .main-content');
const narrowViewStack = ruleDeclarations(narrowStyles, '.app-shell-remote .view-stack.fill');
const narrowEventTime = ruleDeclarations(narrowStyles, '.app-shell-remote .workbench-event time');

expect(narrowMainContent).toMatch(/min-height:\s*0;/);
expect(narrowMainContent).toMatch(/overflow-y:\s*auto;/);
expect(narrowViewStack).toMatch(/min-height:\s*0;/);
expect(narrowEventTime).toMatch(/min-width:\s*0;/);
expect(narrowEventTitle).toMatch(/overflow:\s*hidden;/);
expect(narrowEventResult).toMatch(/overflow-wrap:\s*anywhere;/);
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `cd frontend && npm test -- OverviewView.test.tsx`

Expected: the new declaration assertions fail because the remote content scroll boundary and event-copy safety declarations are absent from the last remote-phone rule block.

- [ ] **Step 3: Add the minimal final remote-phone overview rules**

Append these scoped declarations in the last `@media (max-width: 760px)` block in `frontend/src/style.css` so they win over older breakpoint rules without changing desktop layout:

```css
.app-shell-remote .main-content {
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.app-shell-remote .view-stack.fill {
  min-height: 0;
}

.app-shell-remote .workbench-event time,
.app-shell-remote .workbench-event strong,
.app-shell-remote .workbench-event span {
  min-width: 0;
}

.app-shell-remote .workbench-event strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-shell-remote .workbench-event span {
  overflow-wrap: anywhere;
}
```

- [ ] **Step 4: Run the focused test and verify GREEN**

Run: `cd frontend && npm test -- OverviewView.test.tsx`

Expected: PASS, including the existing checks for the 84px fixed-nav reservation, two-column metrics, 44px event-time column, and clamped event text.

- [ ] **Step 5: Commit the overview slice**

```bash
git add frontend/src/style.css frontend/src/views/OverviewView.test.tsx
git commit -m "fix: stabilize remote overview phone layout"
```

### Task 2: Contain Every Mobile Social Sheet

**Files:**
- Modify: `frontend/src/views/social/SocialRankingDialog.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write the failing mobile-Sheet CSS-contract test**

Add Node file-reading imports and the same `blockAt`, `ruleDeclarations`, and `mediaDeclarations` helpers used by `OverviewView.test.tsx`. Add this test to `SocialRankingDialog.test.tsx`:

```tsx
it('contains remote social dialogs in a phone sheet with one scrolling body', () => {
  const css = readFileSync(new URL('../../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
  const narrowStyles = mediaDeclarations(css, 760);
  const backdrop = ruleDeclarations(narrowStyles, '.app-shell-remote .social-dialog-backdrop');
  const sheet = ruleDeclarations(narrowStyles, '.app-shell-remote .social-dialog-backdrop > section.social-dialog');
  const body = ruleDeclarations(narrowStyles, '.app-shell-remote .social-dialog-body');

  expect(backdrop).toMatch(/position:\s*fixed;/);
  expect(backdrop).toMatch(/inset:\s*0;/);
  expect(sheet).toMatch(/height:\s*min\(84dvh,\s*720px\);/);
  expect(sheet).toMatch(/overflow:\s*hidden;/);
  expect(body).toMatch(/min-height:\s*0;/);
  expect(body).toMatch(/overflow:\s*auto;/);
});
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `cd frontend && npm test -- SocialRankingDialog.test.tsx`

Expected: the backdrop assertions fail because the final remote-social block relies on generic dialog positioning instead of declaring its viewport containment explicitly.

- [ ] **Step 3: Add explicit remote Sheet containment and action sizing**

Append the following final remote-phone rules in `frontend/src/style.css`:

```css
.app-shell-remote .social-dialog-backdrop {
  position: fixed;
  inset: 0;
  min-height: 100dvh;
  overflow: hidden;
}

.app-shell-remote .social-dialog-body {
  overscroll-behavior: contain;
}

.app-shell-remote .social-feature-grid {
  min-height: 0;
}

.app-shell-remote .social-feature-button {
  min-width: 0;
  min-height: 52px;
  padding: 0 12px;
  text-align: left;
}
```

- [ ] **Step 4: Run the focused test and verify GREEN**

Run: `cd frontend && npm test -- SocialRankingDialog.test.tsx`

Expected: PASS; existing ranking pagination, error, cache, and action tests continue to pass because markup and callbacks are unchanged.

- [ ] **Step 5: Commit the shared Sheet slice**

```bash
git add frontend/src/style.css frontend/src/views/social/SocialRankingDialog.test.tsx
git commit -m "fix: contain remote social phone sheets"
```

### Task 3: Make Ranking Rows And Scrolling Phone-Native

**Files:**
- Modify: `frontend/src/views/social/SocialRankingDialog.test.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write the failing ranking scroll-and-order assertions**

Extend the mobile-Sheet test with the following declarations and expectations:

```tsx
const shell = ruleDeclarations(narrowStyles, '.app-shell-remote .social-ranking-shell');
const panel = ruleDeclarations(narrowStyles, '.app-shell-remote .social-ranking-panel');
const list = ruleDeclarations(narrowStyles, '.app-shell-remote .social-ranking-list');
const detail = ruleDeclarations(narrowStyles, '.app-shell-remote .social-ranking-list .social-ranking-detail');
const time = ruleDeclarations(narrowStyles, '.app-shell-remote .social-ranking-list .social-ranking-time');
const name = ruleDeclarations(narrowStyles, '.app-shell-remote .social-ranking-list .social-ranking-name');

expect(shell).toMatch(/min-height:\s*0;/);
expect(panel).toMatch(/min-height:\s*0;/);
expect(list).toMatch(/overflow-y:\s*auto;/);
expect(list).toMatch(/overscroll-behavior:\s*contain;/);
expect(detail).toMatch(/order:\s*1;/);
expect(time).toMatch(/order:\s*2;/);
expect(name).toMatch(/order:\s*3;/);
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `cd frontend && npm test -- SocialRankingDialog.test.tsx`

Expected: the `overflow-y` and row-order assertions fail, exposing the desktop DOM order and an insufficiently explicit ranking-list scroll owner.

- [ ] **Step 3: Add final ranking containment and mobile row ordering**

Append these CSS rules to the same final remote-phone block:

```css
.app-shell-remote .social-ranking-dialog-body,
.app-shell-remote .social-ranking-shell,
.app-shell-remote .social-ranking-panel {
  min-height: 0;
}

.app-shell-remote .social-ranking-shell {
  height: 100%;
}

.app-shell-remote .social-ranking-list {
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.app-shell-remote .social-ranking-list .social-ranking-detail {
  order: 1;
}

.app-shell-remote .social-ranking-list .social-ranking-time {
  order: 2;
}

.app-shell-remote .social-ranking-list .social-ranking-name {
  order: 3;
}
```

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `cd frontend && npm test -- OverviewView.test.tsx SocialRankingDialog.test.tsx SocialView.test.tsx`

Expected: PASS. The ranking still requests cursor pages through its unchanged `data-ranking-scroll` list and the social feature menu still opens all existing dialogs.

- [ ] **Step 5: Build and inspect both phone target widths**

Run: `cd frontend && npm run build`

Expected: Vite production build completes successfully.

Start the LAN WebUI using the repository's existing development command. At 360px and 412px viewport widths inspect:

```text
1. 工作台: all six statistics cards remain above the bottom nav; recent-event time, task and message do not overlap.
2. 好友功能: every menu entry is visible after scrolling, with a reachable close button.
3. 排行榜: tabs remain reachable, records read detail-first, and only the record list scrolls without text overlap.
```

- [ ] **Step 6: Commit the ranking mobile slice**

```bash
git add frontend/src/style.css frontend/src/views/social/SocialRankingDialog.test.tsx
git commit -m "fix: make remote rankings scroll on phones"
```

## Self-Review

- Spec coverage: Task 1 covers the overview and fixed navigation; Task 2 covers every friend feature Sheet; Task 3 covers ranking controls, record hierarchy, and the single scrolling list.
- Completeness scan: every test, CSS declaration, command, and acceptance check is explicit.
- Type consistency: the plan changes CSS and existing CSS-contract tests only, so it introduces no runtime interfaces or data shapes.
