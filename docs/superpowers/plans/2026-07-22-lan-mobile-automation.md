# LAN Mobile Automation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (- [ ]) syntax for tracking.

**Goal:** Make the remote mobile automation page grouped and operable, with bounded scheduler and recommendation sheets whose exit and confirmation controls remain visible.

**Architecture:** Keep FarmAutomationState, scheduler update functions, and the recommendation API unchanged. AutomationView derives display-only feature sections and adds semantic wrappers that CSS renders as the current desktop grid/table or as the approved mobile list/sheet. Responsive behavior stays inside the existing .app-shell-remote max-width: 760px CSS block.

**Tech Stack:** React 18, TypeScript, Lucide React, Vitest, React Test Renderer, CSS media queries.

---

### Task 1: Derive And Render Stable Feature Sections

**Files:**
- Modify: frontend/src/views/AutomationView.test.tsx
- Modify: frontend/src/views/AutomationView.tsx

- [x] **Step 1: Write the failing section tests**

Import groupAutomationFeatureGroupsForMobile from ./AutomationView and add:

    it('groups mobile feature rows by domain and retains unknown features', () => {
      const sections = groupAutomationFeatureGroupsForMobile([
        ...state.featureGroups,
        { id: 'future_probe', label: '未来功能', summary: '用于兼容性测试。', enabled: false, settingKeys: [] },
      ]);

      expect(sections.map((section) => [section.id, section.groups.map((group) => group.id)])).toEqual([
        ['own_farm', ['own_base', 'planting', 'fertilizer']],
        ['friends', ['friends']],
        ['rewards_and_shop', ['rewards', 'mystery_shop']],
        ['other', ['future_probe']],
      ]);
    });

    it('renders section semantics around the existing feature rows', () => {
      const html = renderToStaticMarkup(<AutomationView state={state} onRunTask={() => undefined} />);
      expect(html).toContain('automation-feature-section');
      expect(html).toContain('自己的农场');
      expect(html).toContain('好友互动');
      expect(html).toContain('奖励与商店');
      expect(html.indexOf('基础任务')).toBeLessThan(html.indexOf('好友互动'));
      expect(html.indexOf('好友互动')).toBeLessThan(html.indexOf('自动领取奖励'));
    });

- [x] **Step 2: Verify that the new tests fail**

Run: npm test -- --run src/views/AutomationView.test.tsx

Expected: FAIL because groupAutomationFeatureGroupsForMobile and the section markup do not exist.

- [x] **Step 3: Add the display-only section helper and wrappers**

Add this named export after the feature-group types:

    const mobileFeatureSectionDefinitions = [
      { id: 'own_farm', label: '自己的农场', featureGroupIds: ['own_base', 'planting', 'fertilizer'] },
      { id: 'friends', label: '好友互动', featureGroupIds: ['friends'] },
      { id: 'rewards_and_shop', label: '奖励与商店', featureGroupIds: ['rewards', 'mystery_shop'] },
    ] as const;

    export function groupAutomationFeatureGroupsForMobile(featureGroups: FarmAutomationFeatureGroup[]) {
      const byId = new Map(featureGroups.map((group) => [group.id, group]));
      const assigned = new Set<string>();
      const sections = mobileFeatureSectionDefinitions
        .map(({ id, label, featureGroupIds }) => {
          const groups = featureGroupIds
            .map((featureGroupId) => byId.get(featureGroupId))
            .filter((group): group is FarmAutomationFeatureGroup => Boolean(group));
          groups.forEach((group) => assigned.add(group.id));
          return { id, label, groups };
        })
        .filter((section) => section.groups.length > 0);
      const otherGroups = featureGroups.filter((group) => !assigned.has(group.id));
      return otherGroups.length > 0 ? [...sections, { id: 'other', label: '其他功能', groups: otherGroups }] : sections;
    }

Replace the direct state.featureGroups.map call inside automation-feature-grid with a map over this helper. Move the current automation-feature-card article into the inner map unchanged:

    <section className="automation-feature-section" key={section.id}>
      <h2 className="automation-feature-section-title">{section.label}</h2>
      <div className="automation-feature-section-list">
        {section.groups.map((group) => (
          /* existing feature article and current callbacks */
        ))}
      </div>
    </section>

Do not change feature IDs, toggleFeatureGroup, onSaveState, or task normalization.

- [x] **Step 4: Verify the focused test passes**

Run: npm test -- --run src/views/AutomationView.test.tsx

Expected: PASS, including feature-toggle and scheduler-sync tests.

- [x] **Step 5: Commit this task**

    git add frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx
    git commit -m "feat: group mobile automation features"

### Task 2: Add Mobile-Readable Scheduler Semantics

**Files:**
- Modify: frontend/src/views/AutomationView.test.tsx
- Modify: frontend/src/views/AutomationView.tsx

- [x] **Step 1: Write failing scheduler structure and cancellation tests**

Add these tests after the current scheduler-rendering tests:

    it('renders labelled mobile scheduler controls without changing task inputs', () => {
      const html = renderToStaticMarkup(
        <AutomationView state={state} onRunTask={() => undefined} onSaveState={() => undefined} initialSchedulerOpen />,
      );

      expect(html).toContain('aria-label="关闭调度中心"');
      expect(html).toContain('automation-task-number-field');
      expect(html).toContain('优先级');
      expect(html).toContain('间隔(秒)');
      expect(html).toContain('automation-scheduler-footer');
      expect(html).toContain('aria-label="取消调度编辑"');
      expect(html).toContain('name="priority-own_base"');
      expect(html).toContain('name="interval-own_base"');
      expect(html).toContain('automation-mobile-run-control');
    });

    it('closes scheduler edits from the footer without saving', async () => {
      const onSaveState = vi.fn();
      const renderer = TestRenderer.create(
        <AutomationView state={state} onRunTask={() => undefined} onSaveState={onSaveState} initialSchedulerOpen />,
      );
      await act(async () => {
        renderer.root.findByProps({ 'aria-label': '取消调度编辑' }).props.onClick();
      });
      expect(renderer.root.findAllByProps({ 'aria-label': '调度中心' })).toHaveLength(0);
      expect(onSaveState).not.toHaveBeenCalled();
    });

Extend the current recommendation confirmation test to assert that close application configuration, automation-recommendation-body, and confirm application remain present.

- [x] **Step 2: Verify that the new tests fail**

Run: npm test -- --run src/views/AutomationView.test.tsx

Expected: FAIL on the missing scheduler close label, number-field class, and footer.

- [x] **Step 3: Preserve handlers while adding local labels and footer actions**

Extract the current start/stop button content into a small local renderer that accepts an extra class name, while retaining its current aria label, disabled state, icon, text, and toggleAutomation callback. Keep the existing header copy with class automation-header-toggle and add a second copy after automation-summary-strip:

    <div className="automation-mobile-run-control">
      {renderAutomationToggle('automation-runtime-toggle')}
    </div>

Update the two existing async-action tests to select the automation-header-toggle instance from findAllByProps, so the additional mobile-only button does not make the desktop-control assertion ambiguous.

Replace the scheduler text x close control with Lucide X. Keep its current setSchedulerOpen(false) callback and add aria-label="关闭调度中心".

Wrap the existing priority and interval inputs without changing their current name, value, min, disabled, or onChange props:

    <label className="automation-task-number-field automation-task-priority-field">
      <span>优先级</span>
      <input name={'priority-' + task.id} /* current props */ />
    </label>
    <label className="automation-task-number-field automation-task-interval-field">
      <span>间隔(秒)</span>
      <input name={'interval-' + task.id} /* current props */ />
    </label>

Wrap the existing enabled checkbox and run button in a div with class automation-task-actions. Add automation-task-last-time and automation-task-next-time classes to the existing timestamp spans. Add this footer after the task table; it reuses only existing state and handlers:

    <footer className="automation-scheduler-footer">
      <button aria-label="取消调度编辑" className="secondary-button" disabled={saving} type="button" onClick={() => setSchedulerOpen(false)}>
        取消
      </button>
      <button aria-label="保存调度" className="primary-button" disabled={saving || !onSaveState} type="button" onClick={saveScheduler}>
        <Save size={14} />
        {saving ? '保存中' : '保存调度'}
      </button>
    </footer>

Keep the inline desktop save button. Give the feature-settings close control aria-label={settingsGroup.label + '设置关闭'}. Do not change recommendation API state or its current close behavior.

- [x] **Step 4: Verify the focused test passes**

Run: npm test -- --run src/views/AutomationView.test.tsx

Expected: PASS; the existing test targeting automation-save-button still resolves the inline desktop action.

- [x] **Step 5: Commit this task**

    git add frontend/src/views/AutomationView.tsx frontend/src/views/AutomationView.test.tsx
    git commit -m "feat: make automation scheduler mobile-ready"

### Task 3: Add Scoped Remote-Mobile Layout Rules

**Files:**
- Modify: frontend/src/views/AutomationView.test.tsx
- Modify: frontend/src/style.css

- [x] **Step 1: Write failing responsive-CSS assertions**

Add this source test:

    it('defines bounded remote mobile automation sheets and a grouped task list', () => {
      expect(styleSource).toContain('.app-shell-remote .automation-feature-grid');
      expect(styleSource).toContain('.app-shell-remote .automation-feature-section-title');
      expect(styleSource).toContain('.app-shell-remote .automation-mobile-run-control');
      expect(styleSource).toContain('height: min(88dvh, 760px)');
      expect(styleSource).toContain('max-height: 70dvh');
      expect(styleSource).toContain('.app-shell-remote .automation-scheduler-footer');
      expect(styleSource).toContain('overflow-y: auto');
    });

- [x] **Step 2: Verify that the CSS test fails**

Run: npm test -- --run src/views/AutomationView.test.tsx

Expected: FAIL because the remote-specific feature-list and bounded-sheet rules do not exist.

- [x] **Step 3: Add desktop-safe defaults and remote-only overrides**

Near the current automation styles, add the desktop compatibility rules:

    .automation-feature-section,
    .automation-feature-section-list,
    .automation-task-number-field,
    .automation-task-actions { display: contents; }

    .automation-feature-section-title,
    .automation-task-number-field > span,
    .automation-scheduler-footer { display: none; }

Inside the existing remote max-width: 760px block, exclude automation-scheduler-dialog, automation-settings-dialog, and automation-recommendation-dialog from the generic 100dvh selector. Add:

    .app-shell-remote .automation-dialog-backdrop { align-items: end; padding: 0 8px; }

    .app-shell-remote .automation-scheduler-dialog,
    .app-shell-remote .automation-settings-dialog,
    .app-shell-remote .automation-recommendation-dialog {
      align-self: end;
      width: 100%;
      min-height: 0;
      border-radius: 14px 14px 0 0;
    }

    .app-shell-remote .automation-scheduler-dialog {
      height: min(88dvh, 760px);
      max-height: 88dvh;
      grid-template-rows: auto auto minmax(0, 1fr) auto;
    }

    .app-shell-remote .automation-settings-dialog {
      height: min(86dvh, 720px);
      max-height: 86dvh;
    }

    .app-shell-remote .automation-recommendation-dialog {
      height: min(70dvh, 560px);
      max-height: 70dvh;
    }

    .app-shell-remote .automation-task-table {
      min-height: 0;
      overflow-x: hidden;
      overflow-y: auto;
    }

    .app-shell-remote .automation-scheduler-footer {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 8px;
    }

At this breakpoint, make the feature grid one column and each feature section a vertical block with a displayed title. Give each task row named grid areas title actions, priority interval, and last next; hide the desktop row header; reveal number labels; stack automation-task-actions; and add timestamp labels with CSS before pseudo-elements. Hide only the inline desktop save action and message, then show the footer save action. Set automation-recommendation-body to min-height: 0 and overflow: auto.

Keep .automation-mobile-run-control hidden in desktop CSS. At the remote mobile breakpoint, hide .automation-header-toggle, display .automation-mobile-run-control, and use a two-column runtime summary so the new start/stop action aligns beside the state and count instead of consuming the header row.

- [x] **Step 4: Verify the focused test passes**

Run: npm test -- --run src/views/AutomationView.test.tsx

Expected: PASS, including scheduler interval and recommendation tests.

- [x] **Step 5: Commit this task**

    git add frontend/src/style.css frontend/src/views/AutomationView.test.tsx
    git commit -m "feat: adapt automation UI for LAN mobile"

### Task 4: Verify The Responsive Contract End To End

**Files:**
- Modify: docs/superpowers/plans/2026-07-22-lan-mobile-automation.md

- [x] **Step 1: Run all frontend tests**

Run: npm test

Expected: PASS with no Vitest failures.

- [x] **Step 2: Build the production frontend**

Run: npm run build

Expected: TypeScript compilation and the Vite production build complete without errors.

- [ ] **Step 3: Inspect both responsive layouts**

At 360px in the remote shell, verify the feature order is 自己的农场, 好友互动, 奖励与商店, then 其他功能 when present; task rows have no horizontal overflow; toggles and settings remain touchable; scheduler and recommendation header, scroll region, close, cancel, and save/apply controls remain usable. At desktop width, verify the original two-column cards, seven-column scheduler table, and inline save action remain present.

- [x] **Step 4: Record automated verification and commit**

Mark completed checkbox steps in this plan, then run:

    git add docs/superpowers/plans/2026-07-22-lan-mobile-automation.md
    git commit -m "docs: record mobile automation verification"
