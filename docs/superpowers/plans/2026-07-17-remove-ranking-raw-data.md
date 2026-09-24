# Remove Ranking Raw Data Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the ranking dialog's raw data control and all code used only by that mode.

**Architecture:** Keep the existing ranking data and presentation pipeline unchanged. Remove only the UI state and conditional branch that bypass the normal `buildRankingRows` rendering path.

**Tech Stack:** React, TypeScript, Vitest, Vite

---

### Task 1: Remove the raw data mode

**Files:**
- Modify: `frontend/src/views/SocialView.test.tsx`
- Modify: `frontend/src/views/SocialView.tsx`
- Modify: `frontend/src/style.css`

- [x] **Step 1: Write the failing test**

Change the ranking dialog assertion to:

```tsx
expect(html).not.toContain('原始数据');
```

- [x] **Step 2: Run the focused test to verify it fails**

Run: `npm test -- SocialView.test.tsx -t "renders ranking dialog tabs when opened"`

Expected: FAIL because the current dialog still renders "原始数据".

- [x] **Step 3: Write the minimal implementation**

Delete `rawMode`, its toggle button, the `RankingPanel` property, and the `if (rawMode)` JSON branch. Restore the ranking mode controls condition to `rankingTab !== 'visitors'`, remove the obsolete interaction test, and delete the `.social-raw` CSS rule.

- [x] **Step 4: Run the focused test and full verification**

Run: `npm test -- SocialView.test.tsx`

Expected: all SocialView tests pass.

Run: `npm test`

Expected: all frontend tests pass.

Run: `npm run build`

Expected: TypeScript and Vite build exit successfully.
