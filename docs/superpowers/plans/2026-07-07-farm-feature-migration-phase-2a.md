# Farm Feature Migration Phase 2A Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish the grouped Farm_Go feature shell and migrate already-available system blocks into the new A-style navigation.

**Architecture:** Keep Farm_Go native: React views call Wails methods, and Go exposes a typed farm feature catalog that tracks migration status. This phase does not port runtime-heavy warehouse/friend/land execution yet; it creates their permanent navigation homes and migrates existing guard/log/settings/account-adjacent surfaces without fake game actions.

**Tech Stack:** Go 1.25, Wails v2, React 18, TypeScript, Vitest, lucide-react, SQLite-backed existing runtime events/settings.

---

## Scope Check

The full screenshot migration covers multiple independent subsystems. Phase 2A intentionally covers only:

- Grouped navigation shell.
- Feature catalog and migration status.
- Existing system blocks in their new homes: 守护服务, 日志中心, 系统设置, 账户状态 planned card from local runtime/license readiness.
- Stable landing pages for later blocks: 自动农场, 调度中心, 作物分析, 土地详情, 仓库, 好友, 排行榜, 图鉴, 消息推送.

Warehouse, land actions, friends, rankings, atlas, and automation execution each need later one-feature plans and commits.

## File Structure

- Create `internal/farm/features.go`: canonical grouped feature catalog and status constants.
- Create `internal/farm/features_test.go`: verifies screenshot blocks are covered and grouped.
- Modify `app.go`: expose `FarmFeatureCatalog()` as a Wails method.
- Modify `app_test.go`: verifies the root App returns the catalog.
- Modify `frontend/src/components/AppShell.tsx`: replace three-item sidebar with five grouped work areas.
- Modify `frontend/src/components/AppShell.test.tsx`: assert grouped navigation labels and no long screenshot-style sidebar.
- Create `frontend/src/views/FarmWorkspaceView.tsx`: grouped work area surface with compact feature tiles and embedded existing views where applicable.
- Create `frontend/src/views/FarmWorkspaceView.test.tsx`: render tests for migrated/system blocks and future planned cards.
- Modify `frontend/src/App.tsx`: route the five work areas and pass existing status/events/guard props into workspace view.
- Modify `frontend/src/style.css`: grouped shell and workspace styles.
- Update generated Wails bindings only if `wails generate module` or `wails dev` is available locally; otherwise keep TS calls behind existing bindings and report that bindings must be regenerated.

---

### Task 1: Backend Farm Feature Catalog

**Files:**
- Create: `internal/farm/features.go`
- Create: `internal/farm/features_test.go`
- Modify: `app.go`
- Modify: `app_test.go`

- [ ] **Step 1: Write the failing Go tests**

Create `internal/farm/features_test.go`:

```go
package farm

import "testing"

func TestCatalogCoversScreenshotBlocks(t *testing.T) {
	catalog := Catalog()
	want := []string{"auto_farm", "scheduler", "crop_analytics", "lands", "warehouse", "friends", "rankings", "guard", "atlas", "account", "logs", "message_push", "settings"}
	seen := map[string]bool{}
	for _, group := range catalog.Groups {
		for _, feature := range group.Features {
			seen[feature.ID] = true
		}
	}
	for _, id := range want {
		if !seen[id] {
			t.Fatalf("catalog missing %s", id)
		}
	}
}

func TestCatalogUsesGroupedNavigation(t *testing.T) {
	catalog := Catalog()
	got := make([]string, 0, len(catalog.Groups))
	for _, group := range catalog.Groups {
		got = append(got, group.ID)
	}
	want := []string{"workspace", "automation", "assets", "social", "system"}
	if len(got) != len(want) {
		t.Fatalf("group count = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("group[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
```

Append to `app_test.go`:

```go
func TestFarmFeatureCatalogIsExposed(t *testing.T) {
	app := NewApp()
	catalog := app.FarmFeatureCatalog()
	if len(catalog.Groups) != 5 {
		t.Fatalf("group count = %d, want 5", len(catalog.Groups))
	}
}
```

- [ ] **Step 2: Run the tests and verify they fail**

Run:

```powershell
go test ./internal/farm ./...
```

Expected: failure because `internal/farm` and `FarmFeatureCatalog` do not exist.

- [ ] **Step 3: Implement the catalog**

Create `internal/farm/features.go`:

```go
package farm

type FeatureStatus string

const (
	StatusMigrated FeatureStatus = "migrated"
	StatusPlanned  FeatureStatus = "planned"
)

type FeatureCatalog struct {
	Groups []FeatureGroup `json:"groups"`
}

type FeatureGroup struct {
	ID       string        `json:"id"`
	Label    string        `json:"label"`
	Summary  string        `json:"summary"`
	Features []FarmFeature `json:"features"`
}

type FarmFeature struct {
	ID        string        `json:"id"`
	Label     string        `json:"label"`
	Summary   string        `json:"summary"`
	Status    FeatureStatus `json:"status"`
	Reference string        `json:"reference"`
}

func Catalog() FeatureCatalog {
	return FeatureCatalog{Groups: []FeatureGroup{
		{
			ID: "workspace", Label: "工作台", Summary: "总览、快捷任务和最近运行状态",
			Features: []FarmFeature{
				{ID: "overview", Label: "总览", Summary: "Farm_Go 运行状态与最近事件", Status: StatusMigrated, Reference: "Farm_Go OverviewView"},
			},
		},
		{
			ID: "automation", Label: "农场自动化", Summary: "自动农场、调度、消息推送",
			Features: []FarmFeature{
				{ID: "auto_farm", Label: "自动农场", Summary: "待迁移 auto-farm-manager / executor", Status: StatusPlanned, Reference: "core/src/auto-farm-manager.js"},
				{ID: "scheduler", Label: "调度中心", Summary: "待迁移自动任务间隔和运行队列", Status: StatusPlanned, Reference: "core/src/auto-farm-manager.js"},
				{ID: "message_push", Label: "消息推送", Summary: "待迁移消息推送状态和测试发送", Status: StatusPlanned, Reference: "core/src/message-push-manager.js"},
			},
		},
		{
			ID: "assets", Label: "资产与土地", Summary: "作物、土地、仓库、图鉴",
			Features: []FarmFeature{
				{ID: "crop_analytics", Label: "作物分析", Summary: "待迁移种植收益和种子策略分析", Status: StatusPlanned, Reference: "core/src/plant-analytics.js"},
				{ID: "lands", Label: "土地详情", Summary: "待迁移土地状态与升级/抢收/铲除动作", Status: StatusPlanned, Reference: "core/src/auto-farm-executor.js"},
				{ID: "warehouse", Label: "仓库", Summary: "待迁移仓库刷新和出售", Status: StatusPlanned, Reference: "core/src/warehouse-sell-record-store.js"},
				{ID: "atlas", Label: "图鉴", Summary: "待迁移图鉴刷新和解锁购买", Status: StatusPlanned, Reference: "core/src/plant-analytics.js"},
			},
		},
		{
			ID: "social", Label: "好友社交", Summary: "好友、排行榜、访客记录",
			Features: []FarmFeature{
				{ID: "friends", Label: "好友", Summary: "待迁移好友操作、帮助、捣乱和黑白名单", Status: StatusPlanned, Reference: "core/src/friend-*.js"},
				{ID: "rankings", Label: "排行榜", Summary: "待迁移偷取排行和访客记录", Status: StatusPlanned, Reference: "core/src/friend-steal-ranking-store.js"},
			},
		},
		{
			ID: "system", Label: "账户与系统", Summary: "账户、守护、日志、设置",
			Features: []FarmFeature{
				{ID: "account", Label: "账户状态", Summary: "本地运行状态已接入；授权卡密待后续迁移", Status: StatusPlanned, Reference: "core/src/license-auth.js"},
				{ID: "guard", Label: "守护服务", Summary: "进程守护已迁入 Farm_Go", Status: StatusMigrated, Reference: "internal/runtime/guard"},
				{ID: "logs", Label: "日志中心", Summary: "运行事件日志已迁入 Farm_Go", Status: StatusMigrated, Reference: "internal/storage/events.go"},
				{ID: "settings", Label: "系统设置", Summary: "运行链路设置已迁入 Farm_Go", Status: StatusMigrated, Reference: "internal/storage/settings.go"},
			},
		},
	}}
}
```

Modify `app.go` imports to add:

```go
	"Farm_Go/internal/farm"
```

Add this method near other Wails methods:

```go
func (a *App) FarmFeatureCatalog() farm.FeatureCatalog {
	return farm.Catalog()
}
```

- [ ] **Step 4: Run the tests and verify they pass**

Run:

```powershell
go test ./...
```

Expected: all Go tests pass.

- [ ] **Step 5: Commit**

Run:

```powershell
git add internal/farm/features.go internal/farm/features_test.go app.go app_test.go
git commit -m "feat: add farm feature migration catalog"
```

---

### Task 2: Grouped Navigation Shell

**Files:**
- Modify: `frontend/src/components/AppShell.test.tsx`
- Modify: `frontend/src/components/AppShell.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write the failing frontend navigation test**

Replace the assertion body in `frontend/src/components/AppShell.test.tsx` with grouped navigation expectations:

```tsx
expect(html).toContain('工作台');
expect(html).toContain('农场自动化');
expect(html).toContain('资产与土地');
expect(html).toContain('好友社交');
expect(html).toContain('账户与系统');
expect(html).not.toContain('<span>自动农场</span><span>调度中心</span>');
expect(html).not.toContain('<span>仓库</span><span>好友</span>');
```

- [ ] **Step 2: Run the test and verify it fails**

Run:

```powershell
cd frontend
npm test -- AppShell.test.tsx
```

Expected: failure because sidebar still shows `总览`, `守护`, `系统设置`.

- [ ] **Step 3: Implement grouped tabs**

Change `Tab` in `AppShell.tsx` to:

```ts
export type Tab = 'workspace' | 'automation' | 'assets' | 'social' | 'system';
```

Use nav labels `工作台`, `农场自动化`, `资产与土地`, `好友社交`, `账户与系统` with lucide icons `LayoutDashboard`, `Bot`, `Warehouse`, `Users`, `ShieldCheck`.

Update `App.tsx` initial active tab to `workspace` and route the five tabs to `FarmWorkspaceView` in Task 3.

- [ ] **Step 4: Run the navigation test**

Run:

```powershell
cd frontend
npm test -- AppShell.test.tsx
```

Expected: AppShell test passes.

- [ ] **Step 5: Commit**

Run:

```powershell
git add frontend/src/components/AppShell.tsx frontend/src/components/AppShell.test.tsx frontend/src/App.tsx frontend/src/style.css
git commit -m "feat: group farm feature navigation"
```

---

### Task 3: Workspace View With Migrated System Blocks

**Files:**
- Create: `frontend/src/views/FarmWorkspaceView.tsx`
- Create: `frontend/src/views/FarmWorkspaceView.test.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/style.css`

- [ ] **Step 1: Write the failing workspace tests**

Create `frontend/src/views/FarmWorkspaceView.test.tsx`:

```tsx
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { FarmWorkspaceView } from './FarmWorkspaceView';

const status = { target: 'qq_ws', phase: 'idle', connected: false, ready: false };
const guardStatus = {
  phase: 'disabled',
  runtimeTarget: '',
  timeoutStreak: 0,
  threshold: 3,
  restartCountInWindow: 0,
  maxRestartsPerWindow: 4,
  recentRestartEvents: [],
};

describe('FarmWorkspaceView', () => {
  it('renders automation planned cards without claiming migration is complete', () => {
    const html = renderToStaticMarkup(
      <FarmWorkspaceView
        area="automation"
        status={status}
        guardStatus={guardStatus}
        events={[]}
        onRefresh={() => undefined}
        onLaunch={() => undefined}
        onRestart={() => undefined}
        onToggleGuard={() => undefined}
      />,
    );
    expect(html).toContain('自动农场');
    expect(html).toContain('调度中心');
    expect(html).toContain('待迁移');
  });

  it('embeds migrated system blocks in account and system area', () => {
    const html = renderToStaticMarkup(
      <FarmWorkspaceView
        area="system"
        status={status}
        guardStatus={guardStatus}
        events={[]}
        onRefresh={() => undefined}
        onLaunch={() => undefined}
        onRestart={() => undefined}
        onToggleGuard={() => undefined}
      />,
    );
    expect(html).toContain('守护服务');
    expect(html).toContain('日志中心');
    expect(html).toContain('系统设置');
  });
});
```

- [ ] **Step 2: Run the test and verify it fails**

Run:

```powershell
cd frontend
npm test -- FarmWorkspaceView.test.tsx
```

Expected: failure because `FarmWorkspaceView` does not exist.

- [ ] **Step 3: Implement the view**

Create `FarmWorkspaceView.tsx` with:

- Area title and subtitle.
- Feature tiles for the selected group.
- `待迁移` badges for planned blocks.
- Embedded `GuardView`, `LogsView`, and `SettingsView` in the system area using existing components.
- No runtime action success claims for planned features.

- [ ] **Step 4: Run frontend tests**

Run:

```powershell
cd frontend
npm test
```

Expected: all frontend tests pass.

- [ ] **Step 5: Commit**

Run:

```powershell
git add frontend/src/views/FarmWorkspaceView.tsx frontend/src/views/FarmWorkspaceView.test.tsx frontend/src/App.tsx frontend/src/style.css
git commit -m "feat: add farm workspace migration surface"
```

---

### Task 4: Full Verification For Phase 2A

**Files:**
- No source edits expected unless verification reveals a bug.

- [ ] **Step 1: Run Go tests**

Run:

```powershell
go test ./...
```

Expected: all Go tests pass.

- [ ] **Step 2: Run frontend tests**

Run:

```powershell
cd frontend
npm test
```

Expected: all frontend tests pass.

- [ ] **Step 3: Run frontend build**

Run:

```powershell
cd frontend
npm run build
```

Expected: TypeScript and Vite build complete successfully.

- [ ] **Step 4: Commit verification fixes only if needed**

If verification required edits:

```powershell
git add <changed-files>
git commit -m "fix: stabilize farm migration phase 2a"
```

If no edits were needed, do not create an empty commit.

---

## Self-Review

- Spec coverage: Phase 2A covers grouped navigation, migration status, and existing system blocks. Runtime-heavy business blocks are intentionally left as planned tiles with explicit future phase ownership.
- Planned-card scan: The plan avoids incomplete-marker language and fake-success language. Planned features are explicitly marked `待迁移`.
- Type consistency: `workspace | automation | assets | social | system` is used consistently for frontend area IDs; backend group IDs match.
