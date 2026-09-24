# Land Detail Four Column Cards Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show runtime land details as compact four-column cards with the key fields used by the reference farm-tauri `/api/lands` payload.

**Architecture:** Keep the existing Wails API surface and enrich `farm.LandDetailsItem` normalization from `gameCtl.getFarmStatus`. Render those fields in `AssetsLandView` with a dense card layout and responsive CSS.

**Tech Stack:** Go land normalization, React 18, TypeScript, CSS, Vitest, Go tests.

---

### Task 1: Backend Land Field Normalization

**Files:**
- Modify: `internal/farm/gameconfig.go`
- Test: `app_test.go`

- [ ] Add a failing runtime land test that expects land type, display plant name, maturity text, season, occupancy, and work flags to survive `FarmLandDetails`.
- [ ] Extend `LandDetailsItem` with optional JSON fields mirroring the reference `/api/lands` card fields.
- [ ] Normalize those fields from runtime grid keys without adding image proxy or mutation inspection behavior.
- [ ] Run `go test ./...`.

### Task 2: Frontend Four-Column Card

**Files:**
- Modify: `frontend/src/views/AssetsLandView.tsx`
- Modify: `frontend/src/style.css`
- Test: `frontend/src/views/AssetsLandView.test.tsx`

- [ ] Add a failing render test for the new land card details and four-column grid class.
- [ ] Extend the local `LandDetailsItemLike` type.
- [ ] Replace the simple land tile body with compact rows, status pill, and tags.
- [ ] Change `.land-grid` to four equal columns and tighten `.land-tile` styling.
- [ ] Run frontend tests and build.

### Task 3: Verification

- [ ] Run focused Go and frontend tests.
- [ ] Run full `go test ./...`.
- [ ] Run `npm test` and `npm run build` from `frontend`.
- [ ] Report changed files and verification output.
