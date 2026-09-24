# Restore Fertilizer Fill Controls Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the unimplemented fertilizer auto-buy controls with the working fertilizer-fill controls in the automation settings page.

**Architecture:** The fertilizer settings section remains a single React view. The change only alters controls rendered by its fertilizer branch and the static-markup test that defines its visible contract; backend configuration keys remain untouched.

**Tech Stack:** React, TypeScript, Vitest, Vite.

---

### Task 1: Restore The Fertilizer Fill Controls

**Files:**
- Modify: `frontend/src/views/AutomationView.test.tsx:689`
- Modify: `frontend/src/views/AutomationView.tsx:1626`

- [x] **Step 1: Write the failing UI contract**

Replace the fertilizer-settings assertions with these expectations:

```tsx
expect(html).toContain('自动填充肥料');
expect(html).toContain('填充肥料间隔(秒)');
expect(html).toContain('name="config-autoFarmFertilizerFillEnabled"');
expect(html).toContain('name="config-autoFarmFertilizerFillIntervalSec"');
expect(html).not.toContain('肥料不足自动购买填充');
expect(html).not.toContain('name="config-autoFarmFertilizerAutoBuyEnabled"');
expect(html).not.toContain('name="config-autoFarmFertilizerAutoBuyType"');
expect(html).not.toContain('name="config-autoFarmFertilizerAutoBuyMaxCount"');
```

- [x] **Step 2: Verify the test fails before implementation**

Run: `npm test -- --run frontend/src/views/AutomationView.test.tsx`

Expected: FAIL because the rendered page does not yet include the fertilizer-fill controls and still includes the auto-buy controls.

- [x] **Step 3: Make the minimal UI change**

In the fertilizer settings group, replace the auto-buy checkbox and its type/count fields with:

```tsx
<label className="automation-settings-check">
  <input
    checked={configBool('autoFarmFertilizerFillEnabled', false)}
    name="config-autoFarmFertilizerFillEnabled"
    type="checkbox"
    onChange={(event) => updateConfig('autoFarmFertilizerFillEnabled', event.currentTarget.checked)}
  />
  <span>自动填充肥料</span>
</label>
<label className="automation-settings-field">
  填充肥料间隔(秒)
  <input
    min={5}
    name="config-autoFarmFertilizerFillIntervalSec"
    type="number"
    value={configNumber('autoFarmFertilizerFillIntervalSec', 43200)}
    onChange={(event) => updateConfig('autoFarmFertilizerFillIntervalSec', numberFromInput(event.currentTarget.value, 43200))}
  />
</label>
```

- [x] **Step 4: Verify the focused test passes**

Run: `npm test -- --run frontend/src/views/AutomationView.test.tsx`

Expected: PASS.

- [x] **Step 5: Verify the frontend distribution**

Run: `npm test; npx vite build`

Expected: all frontend tests pass and Vite emits the updated application bundle to `frontend/dist`.

`npm run build` was also run. Its `tsc` stage remains blocked by pre-existing test-only Node type configuration: without `@types/node`, `LogsView.test.tsx` cannot resolve `node:*`; with the types available, five intentional `@ts-expect-error` comments become invalid. The Vite production bundle completed successfully.
