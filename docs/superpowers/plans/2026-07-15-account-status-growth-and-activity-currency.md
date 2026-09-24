# Account Status Growth And Activity Currency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Present account level experience progress, display the warehouse-backed "荷露" activity currency, and move retained runtime statistics into an accessible dialog.

**Architecture:** Keep `FarmAccountStatus` as the source for profile, fertilizer, and daily statistics. `AccountStatusView` independently invokes the existing `FarmWarehouse` binding in parallel and extracts only the exact-name `荷露` item from its returned snapshot. Factor the retained statistics controls into a dialog component inside the existing view so their aggregation helpers and range state remain local.

**Tech Stack:** React 18, TypeScript, Vitest, lucide-react, Wails bindings, CSS.

---

### Task 1: Lock Down Account Status Rendering With Failing Tests

**Files:**
- Modify: `frontend/src/views/AccountStatusView.test.tsx`
- Test: `frontend/src/views/AccountStatusView.test.tsx`

- [ ] **Step 1: Add static rendering assertions for growth progress and a warehouse-backed activity currency**

Replace the existing profile fixture and assertions that expect no progress with a full data fixture, passing an `initialWarehouse` snapshot containing an item named `荷露`:

```tsx
initialProfile={{
  name: 'Dpo.L', level: 118, gold: 3780797635, bean: 246703210, coupon: 10203040, diamond: 120000000,
  levelProgress: { current: 108855, needed: 823800, remaining: 714945, percent: 13, nextLevel: 119 },
  todayStats: { dateKey: '2026-07-07', runs: 8, collect: 10, water: 2, steal: 3, help: 1, mischiefGrass: 2, mischiefBug: 1, sell: 4 },
  statsHistory: { todayKey: '2026-07-07', days: [] },
}}
initialWarehouse={{
  status: 'runtime',
  items: [{ id: 'lotus-dew', itemId: 95225, name: '荷露', count: 5225, category: 'tool', categoryLabel: '道具' }],
}}

expect(html).toContain('经验升级进度');
expect(html).toContain('108,855 / 823,800');
expect(html).toContain('下一等级 Lv. 119');
expect(html).toContain('还差 714,945');
expect(html).toContain('活动货币');
expect(html).toContain('荷露');
expect(html).toContain('5,225');
expect(html).not.toContain('运行统计</h2>');
```

- [ ] **Step 2: Add the two currency fallback rendering tests**

Add tests that render the view with an empty runtime warehouse and with a non-runtime warehouse status. Both must assert the fallback label, and the empty snapshot case must assert that the fake quantity is absent:

```tsx
expect(html).toContain('暂未获取');
expect(html).not.toContain('>0<');
```

- [ ] **Step 3: Add a dialog interaction test**

Import `act` and `create` from `react-test-renderer`, render the view with a complete profile, invoke the button identified by `aria-label="打开运行统计"`, and assert that the dialog and retained range tabs appear. Invoke the close button and assert the dialog is removed:

```tsx
const statisticsButton = root.root.findAllByType('button')
  .find((button) => button.props['aria-label'] === '打开运行统计');
act(() => statisticsButton?.props.onClick());
expect(root.root.findByProps({ role: 'dialog' }).props['aria-labelledby']).toBe('account-statistics-title');
expect(root.root.findAllByType('button').some((button) => button.children.includes('近三天'))).toBe(true);

act(() => root.root.findByProps({ 'aria-label': '关闭运行统计' }).props.onClick());
expect(() => root.root.findByProps({ role: 'dialog' })).toThrow();
```

- [ ] **Step 4: Run the focused test to verify RED**

Run: `pnpm --dir frontend test -- AccountStatusView.test.tsx`

Expected: FAIL because `initialWarehouse` is not accepted, the activity currency markup is absent, and no statistics dialog trigger exists.

### Task 2: Add Account View State And Data Readers

**Files:**
- Modify: `frontend/src/views/AccountStatusView.tsx`
- Test: `frontend/src/views/AccountStatusView.test.tsx`

- [ ] **Step 1: Add the minimal warehouse types and binding import**

Extend the binding import and define only the local fields used by this view:

```tsx
import { FarmAccountStatus, FarmWarehouse } from '../../wailsjs/go/main/App';

type WarehousePayloadLike = {
  status?: string;
  items?: Array<{ name?: string; count?: number; categoryLabel?: string }>;
};

type ActivityCurrencyState =
  | { state: 'loading' }
  | { state: 'available'; count: number; categoryLabel: string }
  | { state: 'unavailable' };
```

Add `initialWarehouse?: WarehousePayloadLike` to the props so render tests do not make Wails calls.

- [ ] **Step 2: Implement an exact-name activity currency resolver**

Add a pure helper next to existing format helpers. It must accept only a runtime snapshot and must never manufacture a zero value for missing items:

```tsx
function resolveLotusDew(payload?: WarehousePayloadLike): ActivityCurrencyState {
  if (payload?.status !== 'runtime') return { state: 'unavailable' };
  const item = payload.items?.find((candidate) => candidate.name === '荷露');
  const count = Number(item?.count);
  if (!item || !Number.isFinite(count) || count < 0) return { state: 'unavailable' };
  return { state: 'available', count, categoryLabel: item.categoryLabel || '道具' };
}
```

- [ ] **Step 3: Fetch warehouse and account data in parallel**

Add `activityCurrency` state initialized from `initialWarehouse`, then invoke `FarmWarehouse()` alongside the existing account request on initial load and in `refreshAccount`. Preserve the current runtime-readiness guard. Guard asynchronous callbacks with the same `active` flag used by the existing account effect, and resolve rejection to `{ state: 'unavailable' }`:

```tsx
function loadActivityCurrency(onResult: (state: ActivityCurrencyState) => void) {
  FarmWarehouse()
    .then((payload) => onResult(resolveLotusDew(payload as WarehousePayloadLike)))
    .catch(() => onResult({ state: 'unavailable' }));
}
```

Do not call a refresh or sell operation directly; `FarmWarehouse` is the existing snapshot API.

- [ ] **Step 4: Run the focused test to verify GREEN for data state**

Run: `pnpm --dir frontend test -- AccountStatusView.test.tsx`

Expected: currency fixture and unavailable-state tests still fail only because presentation markup is not yet added.

### Task 3: Render The Growth Main Card And Activity Currency Card

**Files:**
- Modify: `frontend/src/views/AccountStatusView.tsx`
- Modify: `frontend/src/style.css`
- Test: `frontend/src/views/AccountStatusView.test.tsx`

- [ ] **Step 1: Replace the level metric with semantic growth content**

Inside the account-assets section, render an `AccountGrowthCard` before the four metric cards. It must return a message-only fallback when `needed <= 0`, otherwise use clamped percentage and labelled accessible progress semantics:

```tsx
<div
  className="account-growth-progress"
  role="progressbar"
  aria-label="经验升级进度"
  aria-valuemin={0}
  aria-valuemax={progress.needed}
  aria-valuenow={progress.current}
>
  <span style={{ width: `${progress.percent}%` }} />
</div>
```

Use `formatNumber` for `current`, `needed`, and `remaining`; show `下一等级 Lv. {nextLevel}` and `还差 {remaining}` only for valid progress.

- [ ] **Step 2: Replace the inline statistics section with the activity currency card**

Remove the `account-stats-section` from the page layout. Add an `account-activity-currency-section` beside the existing fertilizer section. Its content must be:

```tsx
<h2>活动货币</h2>
<p>当前活动物品</p>
<div className="activity-currency-row">
  <span className="activity-currency-mark" aria-hidden="true">荷</span>
  <div><strong>荷露</strong><span>{categoryLabel} · 仓库实时数量</span></div>
  <output>{formatNumber(count)}</output>
</div>
```

For loading and unavailable states, retain the heading and show `暂未获取` without rendering an `output` quantity.

- [ ] **Step 3: Add scoped styles**

In the account-status stylesheet section, replace the old wide stats-area grid rule with a two-column lower grid. Add `.account-growth-card`, `.account-growth-progress`, `.account-activity-currency-section`, `.activity-currency-row`, and `.activity-currency-mark`. Reuse the existing warm white panel, #d6ff67 accent rail, green progress fill, and 8px-or-less radii. Add the lower grid to the existing narrow-width media query so both cards stack below 980px.

- [ ] **Step 4: Run the focused test to verify GREEN for primary page rendering**

Run: `pnpm --dir frontend test -- AccountStatusView.test.tsx`

Expected: PASS, including complete progress, unavailable warehouse, and retained non-statistics page behavior.

### Task 4: Move Runtime Statistics Into An Accessible Dialog

**Files:**
- Modify: `frontend/src/views/AccountStatusView.tsx`
- Modify: `frontend/src/style.css`
- Test: `frontend/src/views/AccountStatusView.test.tsx`

- [ ] **Step 1: Add dialog state and an icon-only trigger**

Add `statisticsOpen` state and a ref to the trigger. Place the following button beside the existing refresh button:

```tsx
<button
  ref={statisticsTriggerRef}
  className="secondary-action account-statistics-action"
  type="button"
  aria-label="打开运行统计"
  title="运行统计"
  onClick={() => setStatisticsOpen(true)}
>
  <BarChart3 size={16} />
</button>
```

- [ ] **Step 2: Extract and render `AccountStatisticsDialog`**

Move the existing range tabs and eight metric cards into an `AccountStatisticsDialog` component. Render it only when open with `role="dialog"`, `aria-modal="true"`, and `aria-labelledby="account-statistics-title"`. Attach an Escape key listener while open, close when the backdrop itself is clicked, and give the close icon button `aria-label="关闭运行统计"`. On close, restore focus to the trigger ref.

Keep `range`, `setRange`, and the `buildStatsWindow` result owned by `AccountStatusView`, so all range aggregation code remains unchanged.

- [ ] **Step 3: Style the modal without changing metric card design**

Add `.account-statistics-backdrop`, `.account-statistics-dialog`, `.account-statistics-dialog-header`, and `.account-statistics-close`. Use a fixed dimmed backdrop and a constrained dialog width. Keep `.account-stats-grid` and `.account-range-tabs` styling intact inside the dialog; add a single-column responsive dialog grid at the existing narrow breakpoint.

- [ ] **Step 4: Run the focused test to verify GREEN for dialog interaction**

Run: `pnpm --dir frontend test -- AccountStatusView.test.tsx`

Expected: PASS. The interaction test opens and closes the dialog, and static page rendering no longer includes the full statistics region.

### Task 5: Full Verification And Static Bundle Refresh

**Files:**
- Modify: generated `public/app/*` through the frontend build only

- [ ] **Step 1: Run the entire frontend test suite**

Run: `pnpm --dir frontend test`

Expected: PASS with zero failed tests.

- [ ] **Step 2: Build the frontend bundle**

Run: `pnpm --dir frontend run build`

Expected: successful TypeScript and Vite build.

- [ ] **Step 3: Verify the served bundle changed**

Run: `Get-Content public/app/index.html; Get-ChildItem public/app/assets | Sort-Object LastWriteTime -Descending | Select-Object -First 5 Name, LastWriteTime`

Expected: `public/app/index.html` references the newly generated asset filename, and the corresponding asset has the current build timestamp.

- [ ] **Step 4: Review the final diff**

Run: `git diff --check; git diff -- frontend/src/views/AccountStatusView.tsx frontend/src/views/AccountStatusView.test.tsx frontend/src/style.css public/app/index.html`

Expected: no whitespace errors and only the planned account-status UI, tests, stylesheet, and generated bundle changes.
