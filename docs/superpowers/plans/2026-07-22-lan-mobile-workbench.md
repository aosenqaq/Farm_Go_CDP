# LAN Mobile Workbench Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the remote mobile workbench prioritize health and recent events, with labelled direct actions and non-full-screen renewal and notice sheets.

**Architecture:** Keep `OverviewView` data derivation intact. Add responsive labels to its current buttons and apply layout changes only inside the existing `.app-shell-remote` mobile breakpoint. Dialog API state and component behavior remain unchanged; CSS only alters their remote mobile presentation.

**Tech Stack:** React 18, TypeScript, Vitest, CSS media queries, Lucide React.

---

### Task 1: Label Direct Mobile Actions

**Files:**
- Modify: `frontend/src/views/OverviewView.test.tsx`
- Modify: `frontend/src/views/OverviewView.tsx`

- [x] **Step 1: Write the failing markup test**

Add these assertions to `renders the renewal entry beside the announcement command`:

```tsx
expect(html).toContain('header-action-label');
expect(html).toContain('查看公告');
expect(html).toContain('公告');
expect(html).toContain('aria-label="卡密续费"');
expect(html).toContain('aria-label="查看程序公告"');
```

- [x] **Step 2: Verify the test fails because responsive labels are absent**

Run: `npm test -- --run src/views/OverviewView.test.tsx`

Expected: FAIL on `header-action-label`.

- [x] **Step 3: Add the smallest markup change**

Keep the current callbacks and Lucide icons. Render labels as:

```tsx
<CreditCard size={16} />
<span className="header-action-label">续费</span>

<Megaphone size={16} />
<span className="header-action-label header-action-label-desktop">查看公告</span>
<span className="header-action-label header-action-label-mobile">公告</span>
```

Set the two existing buttons to `aria-label="卡密续费"` and `aria-label="查看程序公告"`.

- [x] **Step 4: Verify the focused test passes**

Run: `npm test -- --run src/views/OverviewView.test.tsx`

Expected: PASS.

### Task 2: Make The Remote Mobile Workbench One Dense Page

**Files:**
- Modify: `frontend/src/views/OverviewView.test.tsx`
- Modify: `frontend/src/style.css`

- [x] **Step 1: Write failing responsive CSS assertions**

Under the existing 760px media query, assert:

```tsx
expect(ruleDeclarations(narrowStyles, '.app-shell-remote .workbench-route-facts')).toMatch(/display:\s*none;/);
expect(ruleDeclarations(narrowStyles, '.app-shell-remote .workbench-signals')).toMatch(/grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/);
expect(ruleDeclarations(narrowStyles, '.app-shell-remote .workbench-event-lines')).toMatch(/height:\s*clamp\(/);
expect(ruleDeclarations(narrowStyles, '.app-shell-remote .workbench-event-lines')).toMatch(/overflow-y:\s*auto;/);
expect(ruleDeclarations(narrowStyles, '.app-shell-remote .workbench-event')).toMatch(/grid-template-columns:\s*52px\s+minmax\(0,\s*1fr\)/);
```

- [x] **Step 2: Verify the layout test fails**

Run: `npm test -- --run src/views/OverviewView.test.tsx`

Expected: FAIL because the mobile route facts, scroller, and two-column event rules do not exist.

- [x] **Step 3: Add only remote-mobile CSS**

In the existing `@media (max-width: 760px)` block, add:

```css
.app-shell-remote .workbench-route-facts { display: none; }
.app-shell-remote .workbench-signals { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 7px; }
.app-shell-remote .workbench-event-lines { height: clamp(180px, 30dvh, 260px); overflow-y: auto; overscroll-behavior: contain; }
.app-shell-remote .workbench-event { grid-template-columns: 52px minmax(0, 1fr); }
.app-shell-remote .workbench-event time { grid-row: span 2; }
.app-shell-remote .workbench-event span { white-space: normal; }
.app-shell-remote .workbench-run-statistics-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
```

Compact the same scoped header enough for 360px, show `公告` only on mobile, and preserve desktop dimensions and route facts.

- [x] **Step 4: Verify the focused test passes**

Run: `npm test -- --run src/views/OverviewView.test.tsx`

Expected: PASS.

### Task 3: Make Renewal And Notice Dialogs Remote-Mobile Sheets

**Files:**
- Modify: `frontend/src/views/OverviewView.test.tsx`
- Modify: `frontend/src/style.css`

- [x] **Step 1: Write failing sheet CSS assertions**

Replace the assertion that all remote dialogs are `100dvh`, while retaining it for generic dialogs. Add:

```tsx
expect(ruleDeclarations(narrowStyles, '.app-shell-remote .license-renewal-dialog')).toMatch(/max-height:\s*56dvh;/);
expect(ruleDeclarations(narrowStyles, '.app-shell-remote .program-notice-dialog')).toMatch(/max-height:\s*72dvh;/);
expect(ruleDeclarations(narrowStyles, '.app-shell-remote .program-notice-dialog')).toMatch(/grid-template-rows:\s*auto\s+minmax\(0,\s*1fr\)\s+auto;/);
expect(ruleDeclarations(narrowStyles, '.app-shell-remote .program-notice-body')).toMatch(/overflow:\s*auto;/);
```

- [x] **Step 2: Verify the sheet test fails**

Run: `npm test -- --run src/views/OverviewView.test.tsx`

Expected: FAIL because the two dialogs still inherit the generic full-screen rule.

- [x] **Step 3: Add narrow sheet exceptions**

Keep generic remote dialogs and drawers full-screen. Add these explicit exceptions:

```css
.app-shell-remote .license-renewal-dialog,
.app-shell-remote .program-notice-dialog {
  align-self: end;
  width: 100%;
  min-height: 0;
  height: auto;
  border-radius: 12px 12px 0 0;
}
.app-shell-remote .license-renewal-dialog { max-height: 56dvh; overflow: auto; }
.app-shell-remote .program-notice-dialog { display: grid; grid-template-rows: auto minmax(0, 1fr) auto; max-height: 72dvh; overflow: hidden; }
.app-shell-remote .program-notice-body { min-height: 0; max-height: none; overflow: auto; }
```

- [x] **Step 4: Verify focused tests pass**

Run: `npm test -- --run src/views/OverviewView.test.tsx src/components/LicenseRenewalDialog.test.tsx src/components/ProgramNoticeDialog.test.tsx`

Expected: PASS.

### Task 4: Verify And Commit

**Files:**
- Modify: `docs/superpowers/plans/2026-07-22-lan-mobile-workbench.md`

- [x] **Step 1: Run all frontend tests**

Run: `npm test`

Expected: PASS with no test failures.

- [x] **Step 2: Run the production build**

Run: `npm run build`

Expected: TypeScript compilation and Vite build complete successfully.

- [ ] **Step 3: Inspect both breakpoints**

At 360px: no header overlap or horizontal overflow; health is two-by-two; events scroll inside their region; route facts are absent; statistics are two columns; renewal and notice are bottom sheets. At desktop width: route facts and centered dialogs remain present.

- [ ] **Step 4: Commit the implementation**

```bash
git add frontend/src/views/OverviewView.tsx frontend/src/views/OverviewView.test.tsx frontend/src/style.css docs/superpowers/plans/2026-07-22-lan-mobile-workbench.md
git commit -m "feat: optimize LAN mobile workbench"
```
