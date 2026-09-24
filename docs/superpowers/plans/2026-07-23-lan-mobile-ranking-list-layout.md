# LAN Mobile Ranking List Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep every record row readable in the LAN mobile ranking sheet by preventing ranking rows from shrinking and keeping vertical overflow in the existing list container.

**Architecture:** This is a CSS-only behavior correction within the existing remote-phone media block. `SocialRankingDialog` already renders all tabs and both ranking modes into `.social-ranking-list`; a targeted CSS contract prevents those shared row elements from flex-shrinking while preserving the list's existing scroll and pagination handler.

**Tech Stack:** React 18, TypeScript, Vitest, CSS, Vite/Wails frontend build.

---

### Task 1: Add The Failing Mobile Row-Sizing Regression Test

**Files:**
- Modify: `frontend/src/views/social/SocialRankingDialog.test.tsx`

- [ ] **Step 1: Import the CSS file reader**

Add this import above the existing React test-renderer import:

```tsx
import { readFileSync } from 'node:fs';
```

- [ ] **Step 2: Add the failing source-style regression test**

Add this test inside the existing `describe('SocialRankingDialog', ...)` block:

```tsx
it('prevents every remote ranking record row from shrinking inside the shared scroll list', () => {
  const css = readFileSync(new URL('../../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
  const match = css.match(/\.app-shell-remote \.social-ranking-list > \.social-ranking-row\s*\{([\s\S]*?)\n  \}/);
  const declarations = match?.[1] ?? '';

  expect(declarations).toMatch(/flex:\s*0 0 auto;/);
});
```

This selector applies to the `.social-ranking-row` markup used for stolen-by-me,
stolen-from-me, and visitor pages, including both ranking modes.

- [ ] **Step 3: Run the focused test to verify RED**

Run:

```powershell
npm --prefix frontend test -- src/views/social/SocialRankingDialog.test.tsx
```

Expected: FAIL in `prevents every remote ranking record row from shrinking inside the shared scroll list` because the remote CSS has no `flex: 0 0 auto` declaration for these rows.

### Task 2: Make Shared Ranking Rows Inflexible On LAN Phones

**Files:**
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Add the narrow-scope CSS correction**

In the existing `.app-shell-remote` `@media (max-width: 760px)` ranking section,
immediately after `.app-shell-remote .social-ranking-list`, add:

```css
  .app-shell-remote .social-ranking-list > .social-ranking-row {
    flex: 0 0 auto;
  }
```

Do not change `SocialRankingDialog.tsx`, ranking page requests, cursor loading,
tab handlers, or any desktop rule. The existing list already owns `overflow: auto`
and receives the `onScroll` pagination callback.

- [ ] **Step 2: Run the focused test to verify GREEN**

Run:

```powershell
npm --prefix frontend test -- src/views/social/SocialRankingDialog.test.tsx
```

Expected: PASS, including existing caching, pagination, tab, refresh, and
system-blacklist tests.

### Task 3: Verify The Frontend Bundle And Mobile Contract

**Files:**
- Verify: `frontend/src/style.css`
- Verify: `frontend/src/views/social/SocialRankingDialog.test.tsx`
- Verify generated: `frontend/dist/index.html`
- Verify generated: `frontend/dist/assets/*`

- [ ] **Step 1: Run the full frontend test suite**

Run:

```powershell
npm --prefix frontend test
```

Expected: all Vitest suites pass with no newly introduced failures.

- [ ] **Step 2: Build the Wails frontend bundle**

Run:

```powershell
npm --prefix frontend run build
```

Expected: `tsc && vite build` completes successfully and updates
`frontend/dist/index.html` plus its hashed `assets/` references. This is the
same command Wails invokes through `wails.json` `frontend:build`.

- [ ] **Step 3: Check the generated asset references**

Run:

```powershell
Get-Content frontend/dist/index.html
Get-ChildItem frontend/dist/assets -File | Sort-Object LastWriteTime -Descending | Select-Object -First 5 Name, LastWriteTime, Length
```

Expected: `index.html` references a generated hashed JavaScript asset and the
newest asset timestamp is from the build in Step 2.

- [ ] **Step 4: Inspect the LAN WebUI at phone width**

At 360px and 412px widths, open `好友社交 > 排行榜`, then check each of:

```text
偷取记录：列表模式和排行榜模式
被偷记录：列表模式和排行榜模式
访客记录
```

Expected: controls and summaries remain above the list; each record occupies a
separate row; scrolling happens only in the record list; and reaching the
bottom still requests the next cursor page.

- [ ] **Step 5: Commit the implementation**

```powershell
git add frontend/src/style.css frontend/src/views/social/SocialRankingDialog.test.tsx
git commit -m "fix: stabilize LAN ranking rows on mobile"
```

## Self-Review

- Spec coverage: Task 1 detects the exact shared-row regression; Task 2 applies
  the narrow remote-mobile correction; Task 3 verifies unit behavior, the full
  frontend suite, the Wails build artifact, and all five ranking views.
- Placeholder scan: no placeholder work or unspecified file paths remain.
- Scope check: the plan does not alter ranking data, pagination, sorting,
  preferences, desktop styles, or any unrelated social UI.
