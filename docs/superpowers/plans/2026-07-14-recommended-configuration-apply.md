# 应用推荐配置实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让当前账号在明确确认后，一键应用作者账号 `gid:1184655784` 的农场自动化与仓库自动出售推荐快照。

**Architecture:** 后端将已导出的推荐设置固定为一个不可变快照，并把它按自动化配置白名单合并到当前账号设置。存储层用单个 SQLite 事务同时保存自动农场和仓库自动出售设置；前端通过新的 Wails 方法请求应用、刷新返回状态并显示确认和结果状态。

**Tech Stack:** Go 1.25、SQLite（modernc）、Wails v2、React 18、TypeScript、Vitest。

---

## 文件结构

- 创建 `recommended_automation_config.go`：作者 `gid:1184655784` 的无运行时缓存推荐快照，以及按默认自动化键合并的帮助函数。
- 修改 `app.go`：暴露 `ApplyRecommendedFarmAutomationConfig`，复用账号隔离、规范化和调度器重配置。
- 修改 `internal/storage/automation.go`：增加将农场和仓库设置放进同一 SQLite 事务的保存方法。
- 修改 `app_test.go`：验证白名单覆盖、账号隔离、仓库同步与失败原子性。
- 修改 `frontend/wailsjs/go/main/App.js`、`App.d.ts`：暴露新的 Wails 调用。
- 修改 `frontend/src/AuthorizedApp.tsx`、`frontend/src/views/FarmWorkspaceView.tsx`：将调用从根状态传到自动化视图。
- 修改 `frontend/src/views/AutomationView.tsx`、`.test.tsx`、`frontend/src/style.css`：实现确认弹窗、提交状态、成功和失败反馈。

### Task 1: 原子保存和推荐快照

**Files:**
- Create: `recommended_automation_config.go`
- Modify: `internal/storage/automation.go`
- Test: `app_test.go`

- [x] **Step 1: 写入失败测试**

在 `app_test.go` 中创建当前账号包含非推荐字段（例如 `autoFarmFriendWhitelist`）的状态，调用将新增的 `ApplyRecommendedFarmAutomationConfig`，断言：推荐任务、`autoFarmPlantPrimaryMode=backpack_first`、非好友功能组启用，以及自动出售 `enabled=true`/`interval=60`/分类 `fruit,mutation` 生效；好友白名单、好友任务和每日运行标记保持原值。使用现有写入钩子或故障存储使事务失败，断言农场和仓库读取值均保持调用前值。

- [x] **Step 2: 运行失败测试**

Run: `go test ./ -run 'TestApplyRecommendedFarmAutomationConfig' -count=1`

Expected: FAIL，因为推荐接口和事务保存方法尚不存在。

- [x] **Step 3: 实现事务和快照**

在存储层将自动农场序列化和仓库字段映射分别提取为内部 `map[string]string` 构建函数；新增：

```go
func (s *Store) SaveAutoFarmAndWarehouseSettingsForAccount(
    ctx context.Context,
    accountKey string,
    automation AutoFarmSettings,
    warehouse WarehouseAutoSellSettings,
) error
```

该方法开启一次事务，写入两组 settings 键并在成功时提交。`recommended_automation_config.go` 固化已导出的 19 个任务、调度启用/350ms、支持的 `automation.DefaultConfig()` 键，以及仓库 `{Enabled:true, IntervalMinute:60, Categories:[]string{"fruit", "mutation"}}`。不保存 `autoFarmFriendStealCropOptions`、`*DailyDoneDate`、旧等待字段或其他运行时缓存。

- [x] **Step 4: 运行测试确认通过**

Run: `go test ./ -run 'TestApplyRecommendedFarmAutomationConfig' -count=1`

Expected: PASS。

### Task 2: Wails 应用接口

**Files:**
- Modify: `app.go`
- Modify: `app_test.go`
- Modify: `frontend/wailsjs/go/main/App.js`
- Modify: `frontend/wailsjs/go/main/App.d.ts`

- [x] **Step 1: 扩展失败测试**

添加账号 A/B 测试：在 A 应用推荐配置后切换 B，B 仍保持自己原配置；活动调度器读回推荐任务和仓库自动出售值。测试未授权调用返回授权错误。

- [x] **Step 2: 运行失败测试**

Run: `go test ./ -run 'TestApplyRecommendedFarmAutomationConfig(AccountIsolation|RequiresAuthorization)' -count=1`

Expected: FAIL，因为接口尚不存在。

- [x] **Step 3: 实现接口和绑定**

新增：

```go
func (a *App) ApplyRecommendedFarmAutomationConfig() (automation.State, error)
```

该接口授权检查后锁定现有自动化写锁，读取当前账号现有设置，使用推荐键覆盖农场配置、调度开关和任务值，保留所有非推荐字段和每日状态；将推荐仓库配置注入调度状态，调用新的事务保存，成功后才 `scheduler.Configure(settings)` 并返回 `automation.StateFromSettings(settings)`。Wails 包装新增无参数 `ApplyRecommendedFarmAutomationConfig(): Promise<automation.State>`。

- [x] **Step 4: 运行测试确认通过**

Run: `go test ./ -run 'TestApplyRecommendedFarmAutomationConfig' -count=1`

Expected: PASS。

### Task 3: 确认弹窗和前端状态同步

**Files:**
- Modify: `frontend/src/AuthorizedApp.tsx`
- Modify: `frontend/src/views/FarmWorkspaceView.tsx`
- Modify: `frontend/src/views/AutomationView.tsx`
- Modify: `frontend/src/views/AutomationView.test.tsx`
- Modify: `frontend/src/style.css`

- [x] **Step 1: 写入前端失败测试**

在 `AutomationView.test.tsx` 用 `react-test-renderer` 验证点击“应用推荐配置”出现 `aria-label="应用推荐配置确认"`，范围文本包含“农场自动化”“仓库自动出售”“不会覆盖”；关闭和取消不调用回调；确认调用回调时按钮禁用并显示“应用中”；成功时关闭弹窗、更新状态并显示指定成功提示；拒绝 Promise 时弹窗仍存在并显示错误。

- [x] **Step 2: 运行失败测试**

Run: `npm test -- --run frontend/src/views/AutomationView.test.tsx`

Expected: FAIL，因为推荐应用回调和确认弹窗尚不存在。

- [x] **Step 3: 实现前端流程**

在 `AuthorizedApp.tsx` 调用新 Wails 方法并用账号 scope token 更新 `automationState`；作为 `onApplyRecommendedConfig` 依次传入 `FarmWorkspaceView` 和 `AutomationView`。自动化视图增加 `recommendationOpen`、`recommendationApplying`、`recommendationError` 状态；入口打开弹窗，确认后调用回调。弹窗复用现有 `dialog-backdrop`，包含关闭、取消、确认按钮，使用 `X`、`Check` 图标和现有浅色设置弹窗的布局规则。成功消息严格为：

```
推荐配置已应用，请检查各项配置是否生效，并可根据自身需求进行调节。
```

- [x] **Step 4: 运行前端测试确认通过**

Run: `npm test -- --run frontend/src/views/AutomationView.test.tsx`

Expected: PASS。

### Task 4: 完整验证

**Files:**
- Modify: `app.go`, `recommended_automation_config.go`, `internal/storage/automation.go`, `app_test.go`, `frontend/src/AuthorizedApp.tsx`, `frontend/src/views/FarmWorkspaceView.tsx`, `frontend/src/views/AutomationView.tsx`, `frontend/src/views/AutomationView.test.tsx`, `frontend/src/style.css`, `frontend/wailsjs/go/main/App.js`, `frontend/wailsjs/go/main/App.d.ts`

- [x] **Step 1: 格式化**

Run: `gofmt -w app.go recommended_automation_config.go internal/storage/automation.go app_test.go`

- [x] **Step 2: 执行后端完整回归**

Run: `go test ./...`

Expected: PASS。

- [x] **Step 3: 执行前端完整回归与构建**

Run: `npm test -- --run && npm run build`

Working directory: `frontend`

Expected: PASS，TypeScript 和 Vite 均无错误。

- [x] **Step 4: 检查变更边界**

Run: `git diff --check && git status --short`

Expected: 无空白错误；仅包含推荐配置实现及用户已有的 `frontend/wailsjs/go/models.ts` 改动。
