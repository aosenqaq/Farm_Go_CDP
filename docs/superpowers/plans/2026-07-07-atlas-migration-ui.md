# Atlas Migration UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a working crop/mutation atlas page with runtime refresh and crop-only locked seed purchase.

**Architecture:** Keep the existing Wails boundary. Go normalizes runtime atlas data and purchase plans, while React renders category tabs and calls Wails methods for refresh, preview, and purchase.

**Tech Stack:** Go, Wails generated JS bindings, React, TypeScript, Vitest.

---

## File Structure

- Modify `internal/farm/gameconfig.go`: atlas item fields, runtime normalization, purchase plan types and builder.
- Modify `internal/farm/gameconfig_test.go`: failing tests for mutation section and crop-only purchase plan.
- Modify `app.go`: Wails methods for atlas buy preview and purchase.
- Modify `frontend/wailsjs/go/main/App.js` and `frontend/wailsjs/go/main/App.d.ts`: generated-style bindings for new methods.
- Modify `frontend/wailsjs/go/models.ts`: generated-style farm model type additions if needed.
- Modify `frontend/src/views/AssetsLandView.tsx`: category tabs, cards, crop-only purchase controls, preview/purchase state.
- Modify `frontend/src/views/AssetsLandView.test.tsx`: static-render tests for category behavior.
- Modify `frontend/src/style.css`: atlas UI styling consistent with the app.

## Tasks

### Task 1: Backend Atlas Data And Purchase Plan

- [x] Write Go tests in `internal/farm/gameconfig_test.go` for `BuildRuntimeAtlasPreview` returning both `crop` and `mutation` sections and for `BuildAtlasLockedCropPurchasePlan` ignoring mutation rows.
- [x] Run `go test ./internal/farm -run "Atlas|PurchasePlan"` and verify the new tests fail because purchase-plan APIs and fields are missing.
- [x] Implement atlas item fields and purchase-plan types in `internal/farm/gameconfig.go`.
- [x] Run `go test ./internal/farm -run "Atlas|PurchasePlan"` and verify the tests pass.

### Task 2: Wails Atlas Actions

- [x] Add Go methods in `app.go`: `FarmAtlasBuyLockedPreview(input map[string]any) farm.AtlasPurchasePreviewPayload` and `FarmAtlasBuyLockedCrops(input map[string]any) farm.AtlasPurchasePayload`.
- [x] Add generated-style bindings in `frontend/wailsjs/go/main/App.js` and `frontend/wailsjs/go/main/App.d.ts`.
- [x] Run `go test ./...` and fix compile errors.

### Task 3: Frontend Atlas Categories

- [x] Add React tests in `frontend/src/views/AssetsLandView.test.tsx` proving crop tab renders purchase controls and mutation tab content can render without those controls.
- [x] Run `npm test -- src/views/AssetsLandView.test.tsx` from `frontend` and verify the new tests fail against the current single-section UI.
- [x] Update `frontend/src/views/AssetsLandView.tsx` with atlas category state, card rendering, preview/purchase calls, and crop-only purchase actions.
- [x] Update `frontend/src/style.css` with compact atlas tabs, summary strip, image cards, and purchase preview rows.
- [x] Run `npm test -- src/views/AssetsLandView.test.tsx` from `frontend` and verify the tests pass.

### Task 4: Final Verification

- [x] Run `go test ./...`.
- [x] Run `npm test` from `frontend`.
- [x] Run `npm run build` from `frontend`.
- [x] Check the git diff for unrelated edits and report any pre-existing dirty files separately.
