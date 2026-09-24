# Mobile Auto-Sell Settings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the sparse remote-phone auto-sell dialog with the approved compact grouped form while preserving all settings behavior.

**Architecture:** Keep `WarehouseAutoSellDialog` as the owner of the existing `draft` state and save callback. Add semantic form-group class names in its markup, then scope compact grid, switch, input-unit, and category-layout CSS to `.app-shell-remote` at the existing 760px breakpoint.

**Tech Stack:** React, TypeScript, Vitest, CSS Grid, existing Vite build.

---

## File Structure

- Modify `frontend/src/views/AssetsLandView.tsx`: group the current enable checkbox and interval input, without changing their state handlers.
- Modify `frontend/src/style.css`: add remote-phone-only grouped form and checkbox-switch appearance rules.
- Modify `frontend/src/views/AssetsLandView.test.tsx`: assert the mobile form structure and scoped CSS contract.

### Task 1: Lock In The Grouped Mobile Form Contract

**Files:**
- Modify: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Write the failing structure and CSS-contract test**

Add this test beside `keeps warehouse auto sell settings in source with fixed category options`:

```tsx
it('uses a compact grouped auto-sell form on remote phones', () => {
  const source = readFileSync(new URL('./AssetsLandView.tsx', import.meta.url), 'utf8');
  const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

  expect(source).toContain('warehouse-settings-group warehouse-settings-execution');
  expect(source).toContain('warehouse-settings-toggle');
  expect(source).toContain('warehouse-settings-input-unit');
  expect(css).toContain('.app-shell-remote .warehouse-settings-execution');
  expect(css).toContain('.app-shell-remote .warehouse-settings-toggle');
  expect(css).toContain('.app-shell-remote .warehouse-settings-input-unit');
  expect(css).toContain('.app-shell-remote .warehouse-settings-options');
});
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `cd frontend && npm test -- AssetsLandView.test.tsx`

Expected: FAIL because the current dialog has no grouped execution, toggle, or input-unit class names.

### Task 2: Implement The Compact Auto-Sell Sheet

**Files:**
- Modify: `frontend/src/views/AssetsLandView.tsx:1796-1832`
- Modify: `frontend/src/style.css` in the final `@media (max-width: 760px)` remote block
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] **Step 1: Add the grouped execution markup**

Replace the standalone enable and interval controls with this markup, retaining the same `draft` updates:

```tsx
<div className="warehouse-settings-group warehouse-settings-execution">
  <strong>执行方式</strong>
  <label className="warehouse-settings-check warehouse-settings-toggle">
    <span>启用自动出售</span>
    <input
      type="checkbox"
      checked={draft.enabled}
      onChange={(event) => setDraft((current) => ({ ...current, enabled: event.target.checked }))}
    />
  </label>
  <label className="warehouse-settings-field">
    <span>出售间隔</span>
    <span className="warehouse-settings-input-unit">
      <input
        min={1}
        type="number"
        value={draft.intervalMinute}
        onChange={(event) => setDraft((current) => ({ ...current, intervalMinute: Number(event.target.value) }))}
      />
      <em>分钟</em>
    </span>
  </label>
</div>
```

Keep the existing category field under `warehouse-settings-group` and leave the error and footer untouched.

- [ ] **Step 2: Add the remote-phone CSS rules**

Append these rules inside the existing remote 760px media block:

```css
.app-shell-remote .warehouse-settings-body {
  align-content: start;
  gap: 10px;
}

.app-shell-remote .warehouse-settings-group {
  gap: 0;
  overflow: hidden;
  border: 1px solid #e2d7bf;
  border-radius: 8px;
  background: #fffdf7;
}

.app-shell-remote .warehouse-settings-group > strong {
  padding: 10px 11px 7px;
  color: #776e5b;
  font-size: 11px;
}

.app-shell-remote .warehouse-settings-toggle,
.app-shell-remote .warehouse-settings-field {
  min-height: 48px;
  margin: 0;
  padding: 0 11px;
  border-top: 1px solid #eee5d4;
}

.app-shell-remote .warehouse-settings-toggle {
  display: flex;
  justify-content: space-between;
  width: 100%;
}

.app-shell-remote .warehouse-settings-toggle input {
  appearance: none;
  width: 38px;
  height: 22px;
  margin: 0;
  border-radius: 999px;
  background: #b9beb4;
}

.app-shell-remote .warehouse-settings-toggle input:checked {
  background: #2f8f3b;
}

.app-shell-remote .warehouse-settings-input-unit {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}

.app-shell-remote .warehouse-settings-field input {
  width: 88px;
}

.app-shell-remote .warehouse-settings-input-unit em {
  color: #776e5b;
  font-size: 12px;
  font-style: normal;
}

.app-shell-remote .warehouse-settings-options {
  gap: 0;
  padding: 0 11px 9px;
}

.app-shell-remote .warehouse-settings-options .warehouse-settings-check {
  min-height: 40px;
  font-size: 12px;
}
```

- [ ] **Step 3: Run the focused test and verify GREEN**

Run: `cd frontend && npm test -- AssetsLandView.test.tsx`

Expected: PASS; existing tests confirm category keys, minute values, and Wails settings calls remain unchanged.

- [ ] **Step 4: Run full frontend verification and build**

Run: `cd frontend && npm test`

Expected: all frontend tests pass.

Run: `cd frontend && npm run build`

Expected: TypeScript and Vite production build complete with `dist/index.html` and hashed assets.

- [ ] **Step 5: Commit the implementation**

```bash
git add frontend/src/style.css frontend/src/views/AssetsLandView.tsx frontend/src/views/AssetsLandView.test.tsx
git commit -m "fix: compact mobile auto sell settings"
```

## Self-Review

- Spec coverage: the grouped execution controls, numeric minute input, two-column categories, fixed footer, and untouched behavior are covered by Tasks 1-2.
- Completeness scan: every test, markup class, style rule, command, and commit target is explicit.
- Type consistency: all new class names match between `AssetsLandView.tsx`, `style.css`, and `AssetsLandView.test.tsx`.
