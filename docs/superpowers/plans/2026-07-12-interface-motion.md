# Interface Motion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add fast, restrained entry motion to navigation views, assets sub-tabs, dialogs, and the land-rush drawer without changing frontend behavior or dependencies.

**Architecture:** Preserve conditional rendering. Keyed wrappers remount the selected primary and assets views, which starts CSS keyframes. Existing dialog backdrops and surfaces receive shared CSS entry rules. This fast UI-only implementation deliberately leaves close callbacks immediate.

**Tech Stack:** React 18, TypeScript, existing global CSS, Vite.

---

## File Structure

- Modify: `frontend/src/AuthorizedApp.tsx` — wrap the selected top-level view in a keyed animated surface.
- Modify: `frontend/src/views/AssetsLandView.tsx` — wrap the selected assets sub-tab panel in a keyed animated surface.
- Modify: `frontend/src/style.css` — add motion keyframes, shared dialog/drawer selectors, and a reduced-motion override.

### Task 1: Animate Primary Navigation

**Files:**
- Modify: `frontend/src/AuthorizedApp.tsx:724-798`

- [ ] **Step 1: Put the current selected-view conditions inside a keyed wrapper**

Insert one wrapper directly inside `AppShell`, retaining every existing child and prop exactly as-is:

```tsx
<section className="app-view-enter" key={activeTab}>
  {isWorkspaceArea(activeTab) && <FarmWorkspaceView key={accountScopeRenderKey} /* existing props */ />}
  {activeTab === 'account' && <AccountStatusView status={status} />}
  {activeTab === 'guard' && <GuardView /* existing props */ />}
  {activeTab === 'logs' && <LogsView events={events} onRefresh={refreshStatus} />}
  {activeTab === 'message_push' && <MessagePushView key={accountScopeRenderKey} />}
  {activeTab === 'settings' && <SettingsView /* existing props */ />}
</section>
```

- [ ] **Step 2: Build once after the JSX edit**

Run: `npm run build` from `frontend`.

Expected: PASS, with no changed API calls, callbacks, or app-shell layout.

### Task 2: Animate Assets Sub-Tabs

**Files:**
- Modify: `frontend/src/views/AssetsLandView.tsx:590-635`

- [ ] **Step 1: Enclose the existing crop, land, warehouse, and atlas panels in a keyed wrapper**

Place the wrapper below the existing `asset-tabs` navigation and leave all panel JSX untouched:

```tsx
<div className="asset-tab-panel-enter" key={activeTab}>
  {activeTab === 'crops' && (/* existing crop panel */)}
  {activeTab === 'lands' && (/* existing lands panel */)}
  {activeTab === 'warehouse' && (/* existing warehouse panel */)}
  {activeTab === 'atlas' && (/* existing atlas panel */)}
</div>
```

Keep `LandRushDialog`, `WarehouseRecordsDialog`, and `WarehouseSettingsDialog` outside this wrapper because their state and stacking remain independent.

- [ ] **Step 2: Build once after the tab wrapper edit**

Run: `npm run build` from `frontend`.

Expected: PASS, with no JSX nesting or type errors.

### Task 3: Add Shared Motion CSS

**Files:**
- Modify: `frontend/src/style.css` before the responsive media-query section.

- [ ] **Step 1: Add the entry keyframes and selectors**

```css
@keyframes app-view-enter {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes asset-tab-panel-enter {
  from { opacity: 0; transform: translateX(12px); }
  to { opacity: 1; transform: translateX(0); }
}

@keyframes dialog-backdrop-enter {
  from { opacity: 0; }
  to { opacity: 1; }
}

@keyframes dialog-surface-enter {
  from { opacity: 0; transform: translateY(8px) scale(0.98); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}

@keyframes drawer-enter {
  from { opacity: 0; transform: translateX(16px); }
  to { opacity: 1; transform: translateX(0); }
}

.app-view-enter { min-width: 0; min-height: 0; animation: app-view-enter 180ms ease-out both; }
.asset-tab-panel-enter { min-width: 0; min-height: 0; animation: asset-tab-panel-enter 200ms ease-out both; }
.dialog-backdrop,
.land-rush-drawer-backdrop { animation: dialog-backdrop-enter 150ms ease-out both; }
.dialog-backdrop > :is(section, aside) { animation: dialog-surface-enter 180ms ease-out both; }
.land-rush-drawer { animation: drawer-enter 180ms ease-out both; }
```

- [ ] **Step 2: Add the accessibility override**

```css
@media (prefers-reduced-motion: reduce) {
  .app-view-enter,
  .asset-tab-panel-enter,
  .dialog-backdrop,
  .land-rush-drawer-backdrop,
  .dialog-backdrop > :is(section, aside),
  .land-rush-drawer {
    animation-duration: 1ms;
  }
}
```

- [ ] **Step 3: Run focused verification**

Run: `npm run build` from `frontend`.

Expected: PASS.

Run: `git diff --check` from the repository root.

Expected: no whitespace errors.

### Task 4: Fast Manual Acceptance

**Files:**
- Verify only.

- [ ] **Step 1: Start the local app and inspect the changed interactions**

Run: `wails dev`.

Expected: a main navigation change fades upward once; an Assets/Land tab change enters from the right; opening update, startup, account, social, automation, warehouse, or settings dialogs fades the overlay and lifts the surface; the land-rush drawer enters from the right.

- [ ] **Step 2: Check one reduced-motion path**

Enable operating-system reduced motion, switch one main tab, and open one dialog.

Expected: both operations stay immediate without visible translation.

- [ ] **Step 3: Commit the finished UI change**

```bash
git add frontend/src/AuthorizedApp.tsx frontend/src/views/AssetsLandView.tsx frontend/src/style.css
git commit -m "feat: add restrained interface transitions"
```

Expected: one focused frontend commit.

## Self-Review

- Coverage: primary views, assets sub-tabs, shared dialogs, drawer, reduced motion, build, and manual interaction checks are included.
- Scope: no exit-presence state machine, modal registry, animation dependency, or broad test suite is added, per the requested fast UI-only delivery.
- Consistency: the plan uses the existing `activeTab` state and existing dialog class names only.
