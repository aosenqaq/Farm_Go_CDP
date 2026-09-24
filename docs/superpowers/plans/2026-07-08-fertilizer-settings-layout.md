# Fertilizer Settings Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update the automatic fertilizer detailed settings panel labels, grouping, and visible controls.

**Architecture:** Keep existing config keys and backend behavior unchanged. Adjust only the React settings markup, add a small local layout class if needed, and update the existing static render test.

**Tech Stack:** React, TypeScript, Vitest, Vite.

---

### Task 1: Fertilizer Settings Panel

**Files:**
- Modify: `frontend/src/views/AutomationView.test.tsx`
- Modify: `frontend/src/views/AutomationView.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write the failing test**

Update the fertilizer settings render test to assert the new labels are visible and removed frontend-only controls are absent.

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- AutomationView.test.tsx`
Expected: FAIL because the UI still renders the old labels and hidden controls.

- [ ] **Step 3: Write minimal implementation**

Rename labels, reorder the strategy/interval and rush/threshold pairs, group auto-buy type and max count in one row, and remove fill interval plus timeout inputs from the rendered JSX.

- [ ] **Step 4: Run test to verify it passes**

Run: `npm test -- AutomationView.test.tsx`
Expected: PASS.

- [ ] **Step 5: Build frontend**

Run: `npm run build`
Expected: TypeScript and Vite build complete with exit code 0.
