# Farm Feature Migration Design

## Goal

Migrate the farm feature blocks from `E:\desktop\farm-tauri` into Farm_Go's Wails/Go/React application with a simpler grouped navigation layout.

## Selected Approach

Use option A from the visual review: grouped navigation plus native Wails panels. Farm_Go should not recreate the long one-item-per-page sidebar from the reference project. It should expose five work areas:

- `工作台`: runtime summary, quick actions, recent events.
- `农场自动化`: automatic farm tasks, scheduling, planting, fertilizing, rewards, message push shortcuts.
- `资产与土地`: crop analytics, land details, warehouse, atlas.
- `好友社交`: friends, friend actions, rankings.
- `账户与系统`: account status, process guard, logs, message push, system settings.

## Scope

This design covers the screenshot blocks:

- 自动农场
- 调度中心
- 作物分析
- 土地详情
- 仓库
- 好友
- 排行榜
- 守护服务
- 图鉴
- 账户状态
- 日志中心
- 消息推送
- 系统设置

The migration must happen incrementally. Each independently migrated script or feature block gets its own local git commit so it can be reverted without rolling back unrelated work.

## Reference Sources

Use `farm-tauri功能脚本梳理.md` as the source map. For implementation details, use these reference project areas:

- `E:\desktop\farm-tauri\core\src\gateway.js` for API behavior.
- `E:\desktop\farm-tauri\core\src\auto-farm-manager.js` and `auto-farm-executor.js` for automation behavior.
- `E:\desktop\farm-tauri\core\src\plant-analytics.js` for crop analysis and atlas support.
- `E:\desktop\farm-tauri\core\src\warehouse-sell-record-store.js` plus executor warehouse functions for warehouse behavior.
- `E:\desktop\farm-tauri\core\src\friend-*.js`, `friend-steal-ranking-store.js`, and `visitor-record-store.js` for friend and ranking behavior.
- Existing Farm_Go guard, log, settings, runtime, diagnostics, QQ WS, and WMPF packages for already migrated platform functions.

## Architecture

Farm_Go stays Go-native. The React UI calls Wails methods. Go services own feature state, storage, and runtime calls. The old Node core is a reference source, not a runtime dependency for production features.

Use a narrow vertical-slice migration pattern:

1. Add typed backend service functions for one feature block.
2. Add Wails-facing methods on `App`.
3. Add focused frontend view components.
4. Add tests before implementation.
5. Commit that feature block.

When a feature depends on live QQ/WeChat/YYB runtime access, the first migration may expose a read-only or command-planning surface with structured "runtime not ready" errors. It must not fake successful game actions.

## UI Design

The left sidebar should contain the five work areas, not all screenshot entries. Inside each work area, show compact tabs or action sections for the screenshot features. The UI should stay dense, work-focused, and consistent with the existing Farm_Go palette and typography.

Expected grouping:

- `工作台`: 总览, 快捷任务, 最近日志.
- `农场自动化`: 自动农场, 调度中心, 消息推送 automation hooks.
- `资产与土地`: 作物分析, 土地详情, 仓库, 图鉴.
- `好友社交`: 好友, 排行榜.
- `账户与系统`: 账户状态, 守护服务, 日志中心, 消息推送 settings, 系统设置.

## Data Flow

Frontend views call generated Wails bindings. Wails bindings call root `App` methods. Root `App` delegates to focused Go services under `internal/farm` or existing service packages. Runtime-dependent actions go through `internal/runtime` diagnostics or a new farm runtime command facade.

Feature state should persist in SQLite only when Farm_Go owns the state. Reference JSON files from farm-tauri should not be copied as hidden runtime state unless they are converted into explicit migrations or imported user configuration.

## Error Handling

Every migrated action must return structured status:

- `ok`
- `runtime_not_ready`
- `unsupported_target`
- `not_migrated`
- `failed`

Runtime-dependent commands must report what is missing: no active runtime, runtime not ready, missing game context, or command failure. UI panels should show concise status text and keep controls disabled while prerequisites are missing.

## Testing

Each feature block needs:

- Go unit tests for service normalization, state transitions, storage, and runtime error handling.
- Frontend Vitest tests for rendering, navigation, disabled states, and action callbacks.
- Existing project verification: `go test ./...`, `cd frontend && npm test`, and `cd frontend && npm run build`.

Tests must be written before production changes for each migration unit.

## Commit Policy

Use one local commit per completed migration unit. Suggested format:

- `docs: plan farm feature migration`
- `feat: group farm feature navigation`
- `feat: migrate farm logs block`
- `feat: migrate account status block`
- `feat: migrate warehouse block`

Do not combine unrelated feature blocks in one commit.
