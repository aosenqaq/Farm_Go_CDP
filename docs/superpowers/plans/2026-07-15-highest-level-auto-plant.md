# 最高等级自动种植策略 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the existing `highest_level` strategy to the automatic planting strategy dropdown.

**Architecture:** The Go runtime already accepts `highest_level` and selects the highest-level available crop. The frontend owns the dropdown's static option list, so this change adds one value-label pair there and changes its rendering contract test.

**Tech Stack:** React, TypeScript, Vitest, React DOM server rendering.

---

### Task 1: Expose the Highest-Level Strategy

**Files:**
- Modify: `frontend/src/views/AutomationView.test.tsx:619-654`
- Modify: `frontend/src/views/AutomationView.tsx:1294-1301`

- [x] **Step 1: Write the failing test**

In the `renders editable planting strategy detailed settings` test, replace the absence assertion with:

```ts
expect(html).toContain('最高等级作物');
```

- [x] **Step 2: Run test to verify it fails**

Run: `npm test -- --run src/views/AutomationView.test.tsx`

Expected: FAIL because the rendered dropdown has no `最高等级作物` option.

- [x] **Step 3: Write minimal implementation**

Add this tuple immediately after `['backpack_first', '背包优先']` in `plantModeOptions`:

```ts
['highest_level', '最高等级作物'],
```

- [x] **Step 4: Run test to verify it passes**

Run: `npm test -- --run src/views/AutomationView.test.tsx`

Expected: PASS with all tests in the file passing.

- [x] **Step 5: Run frontend build**

Run: `npm run build`

Expected: TypeScript compilation and Vite build exit with code 0.

- [x] **Step 6: Commit**

```bash
git add frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx docs/superpowers/plans/2026-07-15-highest-level-auto-plant.md
git commit -m "feat: expose highest-level planting strategy"
```
