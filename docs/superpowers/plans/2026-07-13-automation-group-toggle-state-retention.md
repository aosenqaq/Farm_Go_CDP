# 自动化功能组总开关状态保留 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 关闭农场自动化功能组总开关时保留子功能配置并停止该组调度，重新开启后恢复原有子功能偏好。

**Architecture:** `autoFarmFeatureGroupEnabled.<groupId>` 存放父开关，子任务配置键和持久化任务 `enabled` 存放个人偏好。`StateFromSettings` 计算供调度器使用的有效状态，但不会将父开关造成的关闭回写到子功能偏好；详情页继续以禁用的 `fieldset` 保留既有控件状态和值。

**Tech Stack:** Go、React 18、TypeScript、Vitest、Wails。

---

### Task 1: 为状态保留写失败测试

**Files:**
- Modify: `frontend/src/views/AutomationView.test.tsx`
- Test: `frontend/src/views/AutomationView.test.tsx`

- [ ] **Step 1: 添加点击功能组总开关的载荷测试**

在 `describe('AutomationView')` 新增用例，使用 `TestRenderer.create` 和 `act`。传入默认 `state`，让 `onSaveState` 将 `next` 推入 `savedStates` 后抛出 `Error('stop after capturing save payload')`。点击 `renderer.root.findByProps({ 'aria-label': '关闭基础任务' })` 后断言：`own_base` 功能组为关闭，`autoFarmOneClickEnabled` 与 `autoFarmOwnCollectEnabled` 仍为 `true`，`autoFarmLandUpgradeEnabled` 仍为 `false`，`own_base` 调度任务仍为启用。

```tsx
expect(savedStates[0].featureGroups.find((group) => group.id === 'own_base')?.enabled).toBe(false);
expect(savedStates[0].config.autoFarmOneClickEnabled).toBe(true);
expect(savedStates[0].scheduler.tasks.find((task) => task.id === 'own_base')?.enabled).toBe(true);
```

- [ ] **Step 2: 运行新测试并确认失败原因正确**

Run: `npm test -- --run src/views/AutomationView.test.tsx`

Expected: 新用例失败，显示子功能配置或 `own_base.enabled` 被改为 `false`。

- [ ] **Step 3: 提交失败测试**

Run: `git add -- frontend/src/views/AutomationView.test.tsx; git commit -m "test: cover automation group state retention"`

### Task 2: 仅持久化功能组总开关

**Files:**
- Modify: `frontend/src/views/AutomationView.tsx:506-536`
- Test: `frontend/src/views/AutomationView.test.tsx`

- [ ] **Step 1: 缩减 `toggleFeatureGroup` 的状态转换**

删除 `taskIds`、`nextConfig` 和 `scheduler.tasks` 映射。保留规范化、`enforceGuardOnlyHelpAvailability`、保存和错误处理；`nextState` 只替换匹配 `groupId` 的功能组。

```tsx
const nextState = enforceGuardOnlyHelpAvailability(normalizeAutomationState({
  ...state,
  featureGroups: state.featureGroups.map((group) => (group.id === groupId ? { ...group, enabled } : group)),
}), dogGuardFriendCount);
```

- [ ] **Step 2: 重新运行状态保留测试**

Run: `npm test -- --run src/views/AutomationView.test.tsx`

Expected: 新用例和既有 `AutomationView` 测试均通过。

- [ ] **Step 3: 提交最小状态修复**

Run: `git add -- frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx; git commit -m "fix: retain automation subfeature preferences"`

### Task 3: 禁用关闭功能组的详情页操作

**Files:**
- Modify: `frontend/src/views/AutomationView.tsx:760-776`
- Modify: `frontend/src/style.css:3158-3164`
- Modify: `frontend/src/views/AutomationView.test.tsx`
- Test: `frontend/src/views/AutomationView.test.tsx`

- [ ] **Step 1: 添加关闭功能组详情页的失败测试**

静态渲染 `initialSettingsGroupId="own_base"`，将 `own_base.enabled` 改为 `false`。断言 HTML 包含 `<fieldset disabled=""`、`name="config-autoFarmOneClickEnabled"`、`checked=""` 和 `保存设置`，确保子项状态显示但控件不可编辑。

```tsx
expect(html).toContain('<fieldset disabled=""');
expect(html).toContain('name="config-autoFarmOneClickEnabled"');
expect(html).toContain('checked=""');
expect(html).toContain('保存设置');
```

- [ ] **Step 2: 运行测试并确认缺少禁用容器导致失败**

Run: `npm test -- --run src/views/AutomationView.test.tsx`

Expected: 新用例因找不到 `<fieldset disabled=""` 失败。

- [ ] **Step 3: 用原生禁用 fieldset 包裹详情设置内容**

在 `.automation-settings-body` 内将原有 `renderSettingsGroup(...)` 调用包裹为 `<fieldset disabled={!settingsGroup.enabled}>...</fieldset>`，保持所有原参数不变。同时在 `frontend/src/style.css` 新增样式，移除 fieldset 默认 `border`、`margin`、`min-width` 和 `padding`：

```css
.automation-settings-body > fieldset { border: 0; margin: 0; min-width: 0; padding: 0; }
```

- [ ] **Step 4: 运行前端视图用例**

Run: `npm test -- --run src/views/AutomationView.test.tsx`

Expected: 新增禁用用例和该文件所有既有用例通过。

- [ ] **Step 5: 提交界面禁用行为**

Run: `git add -- frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx frontend/src/style.css; git commit -m "fix: disable controls for inactive automation groups"`

### Task 4: 后端保留子功能偏好并计算有效任务状态

**Files:**
- Modify: `internal/farm/automation/catalog.go`
- Modify: `internal/farm/automation/catalog_test.go`
- Test: `internal/farm/automation/catalog_test.go`

- [ ] **Step 1: 写出父开关关闭后子功能偏好不变的失败回归测试**

在 `TestStateFromSettingsUsesDisabledIndependentFeatureGroupToForceTasksOff` 位置替换旧断言：构造 `friends` 父开关关闭、`autoFarmFriendEnabled=true` 的设置；断言返回状态中 `friend_steal` 有效关闭，但 `config["autoFarmFriendEnabled"]` 仍为 `true`。再将返回状态的 `friends` 功能组改为启用，执行 `StateFromSettings(SettingsFromState(state))`，断言 `friend_steal` 恢复启用且配置仍为 `true`。

- [ ] **Step 2: 运行单一测试并确认它因子功能键被回写为关闭而失败**

Run: `go test ./internal/farm/automation -run TestStateFromSettingsPreservesSubfeaturePreferenceWhenGroupDisabled -count=1`

Expected: FAIL，显示 `autoFarmFriendEnabled` 为 `false`。

- [ ] **Step 3: 分离功能组键与子功能键**

将 `featureGroupSwitchConfigKeys` 统一映射到 `autoFarmFeatureGroupEnabled.<groupId>`。保留旧键仅用于读取没有新键的历史设置；`SettingsFromState` 总是写入新父开关键，不再跳过共享键。

- [ ] **Step 4: 保留子功能配置而不回写父开关造成的有效关闭**

让 `schedulerTaskEnabledFromConfig` 返回该关闭是否由父功能组造成。`StateFromSettings` 仍把这种任务标为有效关闭供调度器跳过，但跳过 `config[taskKey] = schedulerTask.Enabled` 这次回写。对子功能显式关闭和正常任务状态维持原有回写。

- [ ] **Step 5: 运行自动化包测试**

Run: `go test ./internal/farm/automation -count=1`

Expected: PASS。

### Task 5: 完整验证

**Files:**
- Verify: `frontend/src/views/AutomationView.tsx`
- Verify: `frontend/src/views/AutomationView.test.tsx`

- [ ] **Step 1: 运行完整前端测试套件**

Run: `npm test -- --run`

Expected: Vitest 以退出码 0 完成，没有失败用例。

- [ ] **Step 2: 运行前端生产构建**

Run: `npm run build`

Expected: Vite、TypeScript 和打包以退出码 0 完成。

- [ ] **Step 3: 检查最终工作区**

Run: `git diff --check; git status --short`

Expected: 无空白错误；仅存在本任务受控变更或已存在的未跟踪日志目录。
