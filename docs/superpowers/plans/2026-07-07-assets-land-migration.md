# Assets And Land Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the first four `资产与土地` blocks from `E:\desktop\farm-tauri`: `作物分析`, `土地详情`, `仓库`, and `图鉴`.

**Architecture:** Keep Farm_Go native. Static data-backed features read explicit JSON resources from `resources/gameConfig`; runtime-dependent features return structured gated states until Farm_Go has real game runtime commands. The React `资产与土地` area becomes a compact tabbed workspace rather than four placeholder cards.

**Tech Stack:** Go 1.25, Wails v2 bindings, React 18, TypeScript, Vitest, lucide-react, JSON game config from the reference project.

---

## Scope Rules

- Copy only `Plant.json`, `ItemInfo.json`, and `RoleLevel.json` in this batch; defer `plant_images/` until image-backed atlas work.
- Do not fake land or warehouse runtime data.
- Do not merge `账户状态`, `守护服务`, `日志中心`, `消息推送`, or `系统设置`.
- Commit after each migration unit so rollback is simple.

## File Structure

- Create `resources/gameConfig/Plant.json`, `ItemInfo.json`, `RoleLevel.json`: explicit config resources copied from `E:\desktop\farm-tauri\core\gameConfig`.
- Create `internal/farm/gameconfig.go`: resource path resolution, config loading, crop analytics, atlas preview, runtime-gated land and warehouse states.
- Create `internal/farm/gameconfig_test.go`: TDD coverage for config loading, crop metrics, atlas preview, land gating, warehouse gating.
- Modify `app.go`: expose `FarmCropAnalytics`, `FarmAtlasPreview`, `FarmLandDetails`, and `FarmWarehouse`.
- Modify `app_test.go`: verify Wails-facing methods return structured data.
- Modify generated Wails files under `frontend/wailsjs/go/main/` and `frontend/wailsjs/go/models.ts` after `wails generate module`.
- Create `frontend/src/views/AssetsLandView.tsx`: tabbed `资产与土地` workspace.
- Create `frontend/src/views/AssetsLandView.test.tsx`: render coverage for the four tabs and runtime-gated states.
- Modify `frontend/src/views/FarmWorkspaceView.tsx`: delegate `area="assets"` to `AssetsLandView`.
- Modify `frontend/src/style.css`: dense, readable asset workspace styles.

---

## Tasks

### Task 1: Game Config Resources

- [ ] Copy `Plant.json`, `ItemInfo.json`, and `RoleLevel.json` from the old project into `resources/gameConfig/`.
- [ ] Add a focused test that expects the resource directory to contain those three files.
- [ ] Run the focused test to confirm it fails before the copy and passes after the copy.
- [ ] Commit: `feat: add farm game config resources`.

### Task 2: Crop Analytics

- [ ] Write failing Go tests for crop analytics: it loads config, returns shop-eligible crops, computes grow time, harvest exp, net profit, and hourly metrics.
- [ ] Implement the minimal Go service and App method.
- [ ] Run `go test ./internal/farm ./...`.
- [ ] Generate Wails bindings.
- [ ] Write failing frontend tests for the crop analytics tab.
- [ ] Implement the tab using generated bindings, with loading, error, empty, and table states.
- [ ] Run frontend tests and build.
- [ ] Commit: `feat: migrate crop analytics`.

### Task 3: Atlas Preview

- [ ] Write failing Go tests for static atlas preview sections from config.
- [ ] Implement the static preview App method; keep refresh/buy actions marked pending.
- [ ] Run `go test ./internal/farm ./...`.
- [ ] Generate Wails bindings.
- [ ] Write failing frontend tests for the atlas tab and disabled runtime controls.
- [ ] Implement the tab.
- [ ] Run frontend tests and build.
- [ ] Commit: `feat: migrate atlas preview`.

### Task 4: Land Details Runtime Gate

- [ ] Write failing Go tests for `not_migrated` land status and disabled runtime actions.
- [ ] Implement the App method returning structured status, no fake lands.
- [ ] Run `go test ./internal/farm ./...`.
- [ ] Generate Wails bindings.
- [ ] Write failing frontend tests for the land tab status.
- [ ] Implement the tab.
- [ ] Run frontend tests and build.
- [ ] Commit: `feat: add land details runtime gate`.

### Task 5: Warehouse Runtime Gate

- [ ] Write failing Go tests for `not_migrated` warehouse status and disabled refresh/sell actions.
- [ ] Implement the App method returning structured status, no fake items.
- [ ] Run `go test ./internal/farm ./...`.
- [ ] Generate Wails bindings.
- [ ] Write failing frontend tests for the warehouse tab status.
- [ ] Implement the tab.
- [ ] Run frontend tests and build.
- [ ] Commit: `feat: add warehouse runtime gate`.

### Task 6: Final Verification

- [ ] Run `go test ./...`.
- [ ] Run `cd frontend; npm test`.
- [ ] Run `cd frontend; npm run build`.
- [ ] Report exact verification results and any remaining runtime limitations.

---

## Self-Review

- The plan covers exactly the four requested blocks.
- Runtime-dependent data is explicitly gated and not fabricated.
- Every migration unit has its own commit.
- Static config-backed features can work immediately without live QQ/WeChat runtime.
