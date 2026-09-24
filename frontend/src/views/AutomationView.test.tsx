import { readFileSync } from 'node:fs';
import { useState } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import TestRenderer, { act } from 'react-test-renderer';
import { afterEach, describe, expect, it, vi } from 'vitest';

import * as AutomationViewModule from './AutomationView';
import {
  AUTOMATION_TOAST_AUTO_DISMISS_MS,
  AutomationView,
  isSchedulerTaskIntervalDisabled,
  mergeAutomationRuntimeState,
  reorderBackpackSeedPriority,
  resetBackpackSeedPriority,
  syncAutomationSchedulerEnabledState,
  syncAutomationSchedulerIntervals,
  type FarmAutomationState,
} from './AutomationView';
import automationViewSource from './AutomationView.tsx?raw';

const styleSource = readFileSync(new URL('../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');

const state: FarmAutomationState = {
  running: false,
  summary: { enabledTasks: 8, totalTasks: 18, todayHarvest: 0 },
  featureGroups: [
    { id: 'own_base', label: '基础任务', summary: '一键务农、自动收获、除草、浇水、杀虫。', enabled: true, settingKeys: [] },
    { id: 'planting', label: '自动种植', summary: '主/副策略、背包种子选择、随机延迟。', enabled: true, settingKeys: [] },
    { id: 'fertilizer', label: '自动施肥', summary: '自动施肥、催熟联动、自动购买。', enabled: false, settingKeys: [] },
    { id: 'friends', label: '好友互动', summary: '偷菜、帮忙、捣乱、冷却与静默时间。', enabled: false, settingKeys: [] },
    { id: 'rewards', label: '自动领取奖励', summary: '任务奖励、礼包、月卡、邮件、抽奖。', enabled: true, settingKeys: [] },
    { id: 'mystery_shop', label: '神秘商店自动购买', summary: '货币类型、折扣阈值和手动读取购买。', enabled: false, settingKeys: [] },
  ],
  scheduler: {
    enabled: true,
    minGapMs: 350,
    tasks: [
      { id: 'own_base', label: '一键务农', priority: 100, intervalSec: 60, enabled: true },
      { id: 'friend_steal', label: '好友偷菜', priority: 70, intervalSec: 90, enabled: false },
    ],
  },
  config: {
    autoFarmBasicTasksEnabled: true,
    autoFarmOneClickEnabled: true,
    autoFarmOwnCollectEnabled: true,
    autoFarmOwnCollectOnlyWhenOwnFarm: true,
    autoFarmOwnEraseGrassEnabled: true,
    autoFarmOwnWaterEnabled: true,
    autoFarmOwnKillBugEnabled: true,
    autoFarmLandUpgradeEnabled: false,
    autoFarmOwnBaseIntervalSec: 30,
    autoFarmOwnCollectIntervalSec: 30,
    autoFarmLandUpgradeIntervalSec: 43200,
    autoFarmPlantBackpackSeedOptions: [
      { seedId: 21032, name: '琉璃宝荷', level: 200, count: 9 },
      { seedId: 20133, name: '凤仙花', level: 21, count: 5 },
      { seedId: 20176, name: '金银花', level: 21, count: 2 },
    ],
    autoFarmPlantBackpackSeedPriority: [21032, 20133],
    autoFarmPlantBackpackSeedDisabled: [20176],
    autoFarmFriendBlacklistCooldownMin: 10,
    autoFarmFriendStealPlantListMode: 'blacklist',
    autoFarmFriendStealPlantBlacklistStrategy: 1,
    autoFarmFriendStealCropOptions: [
      { plantId: 1020002, seedId: 20002, name: '白萝卜', level: 1, imageUrl: 'data:image/png;base64,AA==' },
      { plantId: 1020003, seedId: 20003, name: '胡萝卜', level: 2, imageUrl: 'data:image/png;base64,AA==' },
    ],
  },
};

describe('AutomationView', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('keeps async action content stable and disables its spinner for reduced motion', () => {
    expect(styleSource).toContain('.async-action-button > svg');
    expect(styleSource).toMatch(
      /\.async-action-button\[aria-busy="true"\]:disabled\s*{\s*opacity: 1;/,
    );
    expect(styleSource).toMatch(
      /@media \(prefers-reduced-motion: reduce\)\s*{\s*\.async-action-button\[aria-busy="true"\] \.spin/,
    );
  });

  it('uses a content-sized bottom drawer for run modes in the remote mobile layout', () => {
    const mobileDrawerBlocks = [...styleSource.matchAll(/\.app-shell-remote \.automation-run-mode-dialog\s*{([\s\S]*?)\n  }/g)];
    const mobileDrawer = mobileDrawerBlocks[mobileDrawerBlocks.length - 1]?.[1] || '';

    expect(mobileDrawer).toContain('height: auto;');
    expect(mobileDrawer).toContain('max-height: calc(100dvh - 8px);');
    expect(mobileDrawer).not.toContain('height: min(70dvh, 560px);');
    expect(styleSource).toMatch(
      /\.app-shell-remote \.automation-run-mode-dialog::before\s*{[\s\S]*content: '';[\s\S]*display: block;/,
    );
    expect(styleSource).toMatch(
      /\.app-shell-remote \.automation-run-mode-dialog > footer\s*{[\s\S]*display: grid;[\s\S]*grid-template-columns: repeat\(2, minmax\(0, 1fr\)\);/,
    );
  });

  it('excludes the run-mode drawer from the remote mobile full-screen dialog rule', () => {
    const fullScreenDialogRule = styleSource.match(
      /\.app-shell-remote \.dialog-backdrop > :is\(section, aside\)[^{]+\{[\s\S]*?max-height: 100dvh;/,
    )?.[0] || '';

    expect(fullScreenDialogRule).toContain(':not(.automation-run-mode-dialog)');
  });

  it('requires a second confirmation before switching the run mode', async () => {
	vi.stubGlobal('window', { clearTimeout, setTimeout });
	const setRunMode = vi.fn(async (runMode: 'safe' | 'god') => ({
	  ...state,
	  runMode,
	}));
	const renderer = TestRenderer.create(
	  <AutomationView state={{ ...state, runMode: 'safe' }} onRunTask={() => undefined} onSetRunMode={setRunMode} />,
	);

	await act(async () => {
	  renderer.root.findByProps({ 'aria-label': '修改运行模式' }).props.onClick();
	});

	const dialog = renderer.root.findByProps({ 'aria-label': '运行模式' });
	expect(dialog.findAllByType('p').map((node) => node.children.join('')).join('')).toContain('慢一点，一件做完再做下一件，动作之间停一停。');
	expect(dialog.findAllByType('p').map((node) => node.children.join('')).join('')).toContain('能一起做的尽量同时做，完成快、操作紧凑。');
	expect(renderer.root.findByProps({ 'aria-label': '选择安全模式' }).props.disabled).toBe(true);

    await act(async () => {
      renderer.root.findByProps({ 'aria-label': '选择仙人模式' }).props.onClick();
    });

    await act(async () => {
      renderer.root.findByProps({ 'aria-label': '继续确认运行模式' }).props.onClick();
    });

	const confirmation = renderer.root.findByProps({ 'aria-label': '确认运行模式' });
	expect(confirmation.findAllByType('p').map((node) => node.children.join('')).join('')).toContain('守护服务不受影响');

	await act(async () => {
	  await renderer.root.findByProps({ 'aria-label': '确认切换运行模式' }).props.onClick();
	});

	expect(setRunMode).toHaveBeenCalledWith('god');
	expect(renderer.root.findAllByProps({ 'aria-label': '确认运行模式' })).toHaveLength(0);
	expect(
	  renderer.root.findAllByProps({ role: 'status' }).some((node) =>
		 node.findAllByType('span').some((span) => span.children.join('').includes('已切换到仙人模式')),
	  ),
	).toBe(true);
	});

  it('shows explicit loading feedback while automation is starting and blocks duplicate starts', async () => {
    vi.stubGlobal('window', { clearTimeout, setTimeout });
    let finishToggle!: () => void;
    const pendingToggle = new Promise<void>((resolve) => { finishToggle = resolve; });
    const onToggleAutomation = vi.fn(() => pendingToggle);
    const renderer = TestRenderer.create(
      <AutomationView state={state} onRunTask={() => undefined} onToggleAutomation={onToggleAutomation} />,
    );
    let request!: Promise<void>;

    await act(async () => {
      const startButton = renderer.root.findAllByProps({ 'aria-label': '启动自动化' })
        .find((node) => String(node.props.className).includes('automation-header-toggle'));
      expect(startButton).toBeDefined();
      if (!startButton) return;
      request = startButton.props.onClick();
      await Promise.resolve();
    });

    const busyButton = renderer.root.findAllByProps({ 'aria-label': '正在启动自动化' })
      .find((node) => String(node.props.className).includes('automation-header-toggle'));
    expect(busyButton).toBeDefined();
    if (!busyButton) return;
    expect(busyButton.props.disabled).toBe(true);
    expect(busyButton.props['aria-busy']).toBe(true);
    expect(busyButton.findAllByProps({ className: 'spin' })).toHaveLength(1);
    expect(busyButton.findAllByType('span').map((node) => node.children.join(''))).toContain('启动中');

    await act(async () => {
      void busyButton.props.onClick();
      await Promise.resolve();
    });
    expect(onToggleAutomation).toHaveBeenCalledTimes(1);
    expect(onToggleAutomation).toHaveBeenCalledWith(true);

    await act(async () => {
      finishToggle();
      await request;
    });
  });

  it('keeps the existing simple processing feedback while automation is stopping', async () => {
    vi.stubGlobal('window', { clearTimeout, setTimeout });
    let finishToggle!: () => void;
    const pendingToggle = new Promise<void>((resolve) => { finishToggle = resolve; });
    const renderer = TestRenderer.create(
      <AutomationView
        state={{ ...state, running: true }}
        onRunTask={() => undefined}
        onToggleAutomation={() => pendingToggle}
      />,
    );
    let request!: Promise<void>;

    await act(async () => {
      const stopButton = renderer.root.findAllByProps({ 'aria-label': '停止自动化' })
        .find((node) => String(node.props.className).includes('automation-header-toggle'));
      expect(stopButton).toBeDefined();
      if (!stopButton) return;
      request = stopButton.props.onClick();
      await Promise.resolve();
    });

    const busyButton = renderer.root.findAllByProps({ 'aria-label': '正在停止自动化' })
      .find((node) => String(node.props.className).includes('automation-header-toggle'));
    expect(busyButton).toBeDefined();
    if (!busyButton) return;
    expect(busyButton.findAllByProps({ className: 'spin' })).toHaveLength(0);
    expect(busyButton.findAllByType('span').map((node) => node.children.join(''))).toContain('处理中');

    await act(async () => {
      finishToggle();
      await request;
    });
  });

  it('keeps detailed task preferences when a feature group is turned off', async () => {
    const savedStates: FarmAutomationState[] = [];
    const renderer = TestRenderer.create(
      <AutomationView
        state={state}
        onRunTask={() => undefined}
        onSaveState={(next) => {
          savedStates.push(next);
          throw new Error('stop after capturing save payload');
        }}
      />,
    );

    await act(async () => {
      await renderer.root.findByProps({ 'aria-label': '关闭基础任务' }).props.onClick();
    });

    expect(savedStates[0].featureGroups.find((group) => group.id === 'own_base')?.enabled).toBe(false);
    expect(savedStates[0].config.autoFarmOneClickEnabled).toBe(true);
    expect(savedStates[0].config.autoFarmOwnCollectEnabled).toBe(true);
    expect(savedStates[0].config.autoFarmLandUpgradeEnabled).toBe(false);
    expect(savedStates[0].scheduler.tasks.find((task) => task.id === 'own_base')?.enabled).toBe(true);
  });

  it('syncs the fertilizer feature card switch into scheduler state', async () => {
    const savedStates: FarmAutomationState[] = [];
    const fertilizerState = {
      ...state,
      config: {
        ...state.config,
        'autoFarmFeatureGroupEnabled.fertilizer': false,
        autoFarmFertilizerEnabled: false,
      },
      scheduler: {
        ...state.scheduler,
        tasks: [
          ...state.scheduler.tasks,
          { id: 'own_fertilizer', label: '自动施肥', priority: 85, intervalSec: 30, enabled: false },
        ],
      },
    };
    const renderer = TestRenderer.create(
      <AutomationView
        state={fertilizerState}
        onRunTask={() => undefined}
        onSaveState={(next) => {
          savedStates.push(next);
          throw new Error('stop after capturing save payload');
        }}
      />,
    );

    await act(async () => {
      await renderer.root.findByProps({ 'aria-label': '开启自动施肥' }).props.onClick();
    });

    expect(savedStates).toHaveLength(1);
    expect(savedStates[0].config['autoFarmFeatureGroupEnabled.fertilizer']).toBe(true);
    expect(savedStates[0].config.autoFarmFertilizerEnabled).toBe(true);
    expect(savedStates[0].scheduler.tasks.find((task) => task.id === 'own_fertilizer')?.enabled).toBe(true);
  });

  it('syncs the mystery shop feature card switch into scheduler state', async () => {
    const savedStates: FarmAutomationState[] = [];
    const mysteryShopState = {
      ...state,
      config: {
        ...state.config,
        'autoFarmFeatureGroupEnabled.mystery_shop': false,
        autoFarmMysteryShopAutoBuyEnabled: false,
      },
      scheduler: {
        ...state.scheduler,
        tasks: [
          ...state.scheduler.tasks,
          { id: 'mystery_shop_auto_buy', label: '神秘商店自动购买', priority: 91, intervalSec: 43200, enabled: false },
        ],
      },
    };
    const renderer = TestRenderer.create(
      <AutomationView
        state={mysteryShopState}
        onRunTask={() => undefined}
        onSaveState={(next) => {
          savedStates.push(next);
          throw new Error('stop after capturing save payload');
        }}
      />,
    );

    await act(async () => {
      await renderer.root.findByProps({ 'aria-label': '开启神秘商店自动购买' }).props.onClick();
    });

    expect(savedStates).toHaveLength(1);
    expect(savedStates[0].config['autoFarmFeatureGroupEnabled.mystery_shop']).toBe(true);
    expect(savedStates[0].config.autoFarmMysteryShopAutoBuyEnabled).toBe(true);
    expect(savedStates[0].scheduler.tasks.find((task) => task.id === 'mystery_shop_auto_buy')?.enabled).toBe(true);
  });

  it('syncs the scheduler fertilizer switch back into the feature group', async () => {
    const savedStates: FarmAutomationState[] = [];
    const fertilizerState = {
      ...state,
      featureGroups: state.featureGroups.map((group) => (group.id === 'fertilizer' ? { ...group, enabled: true } : group)),
      config: {
        ...state.config,
        'autoFarmFeatureGroupEnabled.fertilizer': true,
        autoFarmFertilizerEnabled: true,
      },
      scheduler: {
        ...state.scheduler,
        tasks: [
          ...state.scheduler.tasks,
          { id: 'own_fertilizer', label: '自动施肥', priority: 85, intervalSec: 30, enabled: true },
        ],
      },
    };
    const renderer = TestRenderer.create(
      <AutomationView
        state={fertilizerState}
        onRunTask={() => undefined}
        onSaveState={(next) => {
          savedStates.push(next);
          throw new Error('stop after capturing save payload');
        }}
        initialSchedulerOpen
      />,
    );
    const fertilizerRow = renderer.root
      .findAllByProps({ className: 'automation-task-row' })
      .find((row) => row.findAllByType('strong').some((label) => label.children.join('') === '自动施肥'));
    const fertilizerToggle = fertilizerRow?.findByProps({ className: 'automation-task-check' });

    await act(async () => {
      fertilizerToggle?.findByType('input').props.onChange({ target: { checked: false } });
      await renderer.root.findByProps({ className: 'automation-save-button' }).props.onClick();
    });

    expect(savedStates).toHaveLength(1);
    expect(savedStates[0].featureGroups.find((group) => group.id === 'fertilizer')?.enabled).toBe(false);
    expect(savedStates[0].config['autoFarmFeatureGroupEnabled.fertilizer']).toBe(false);
    expect(savedStates[0].config.autoFarmFertilizerEnabled).toBe(false);
  });

  it('maps warehouse auto sell into scheduler configuration', () => {
    expect(automationViewSource).toContain("auto_warehouse_sell: 'autoWarehouseSellEnabled'");
    expect(automationViewSource).toContain("auto_warehouse_sell: 'autoWarehouseSellIntervalSec'");
  });

  it('syncs scheduler task enabled state from detailed feature config', () => {
    const synced = syncAutomationSchedulerEnabledState({
      ...state,
      config: {
        ...state.config,
        autoFarmFriendEnabled: false,
      },
      scheduler: {
        ...state.scheduler,
        tasks: state.scheduler.tasks.map((task) => (task.id === 'friend_steal' ? { ...task, enabled: true } : task)),
      },
    });

    expect(synced.config.autoFarmFriendEnabled).toBe(false);
    expect(synced.scheduler.tasks.find((task) => task.id === 'friend_steal')?.enabled).toBe(false);
  });

  it('wraps the runtime summary separately from the stretchable feature grid', () => {
    const html = renderToStaticMarkup(<AutomationView state={state} onRunTask={() => undefined} />);

    expect(html).toContain('automation-runtime-panel');
    expect(html.indexOf('automation-runtime-panel')).toBeLessThan(html.indexOf('automation-feature-grid'));
  });

  it('renders approved feature groups and scheduler entry', () => {
    const html = renderToStaticMarkup(<AutomationView state={state} onRunTask={() => undefined} />);

    expect(html).toContain('农场自动化');
    expect(html).toContain('基础任务');
    expect(html).toContain('自动种植');
    expect(html).toContain('自动施肥');
    expect(html).toContain('好友互动');
    expect(html).toContain('自动领取奖励');
    expect(html).toContain('神秘商店自动购买');
    expect(html).toContain('调度中心');
    expect(html).not.toContain('任务全局运行设置');
    expect(html).not.toContain('今日统计');
    expect(html).not.toContain('自家基础任务');
    expect(html).not.toContain('自动种植策略');
    expect(html).not.toContain('肥料与催熟');
    expect(html).not.toContain('好友自动化');
    expect(html).not.toContain('奖励与活动');
    expect(html).not.toContain('运行节奏与附属联动');
  });

  it('groups mobile feature rows by domain and retains unknown features', () => {
    type MobileFeatureSection = { id: string; groups: Array<{ id: string }> };
    const groupFeatureRows = (AutomationViewModule as {
      groupAutomationFeatureGroupsForMobile?: (groups: FarmAutomationState['featureGroups']) => MobileFeatureSection[];
    }).groupAutomationFeatureGroupsForMobile;

    expect(groupFeatureRows).toBeTypeOf('function');
    if (!groupFeatureRows) return;

    const sections = groupFeatureRows([
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

  it('renders mobile section semantics around the existing feature rows', () => {
    const html = renderToStaticMarkup(<AutomationView state={state} onRunTask={() => undefined} />);

    expect(html).toContain('automation-feature-section');
    expect(html).toContain('自己的农场');
    expect(html).toContain('好友互动');
    expect(html).toContain('奖励与商店');
    expect(html.indexOf('基础任务')).toBeLessThan(html.indexOf('好友互动'));
    expect(html.indexOf('好友互动')).toBeLessThan(html.indexOf('自动领取奖励'));
  });

  it('derives summary counts and running task labels from real scheduler state', () => {
    const html = renderToStaticMarkup(
      <AutomationView
        state={{
          ...state,
          running: true,
          summary: { enabledTasks: 99, totalTasks: 99, todayHarvest: 0 },
          scheduler: {
            ...state.scheduler,
            runningTaskId: 'own_base',
            tasks: [
              { id: 'own_base', label: '一键务农', priority: 100, intervalSec: 60, enabled: true },
              { id: 'friend_steal', label: '好友偷菜', priority: 70, intervalSec: 90, enabled: false },
            ],
          },
        }}
        onRunTask={() => undefined}
      />,
    );

    expect(html).toContain('运行中');
    expect(html).toContain('停止');
    expect(html).toContain('1 / 2');
    expect(html.indexOf('本轮')).toBeLessThan(html.indexOf('下次执行'));
    expect(html).toContain('<span>本轮</span><strong>一键务农</strong>');
  });

  it('renders scheduler task rows in the dialog surface', () => {
    const html = renderToStaticMarkup(<AutomationView state={state} onRunTask={() => undefined} initialSchedulerOpen />);

    expect(html).toContain('一键务农');
    expect(html).toContain('好友偷菜');
    expect(html).toContain('100');
    expect(html).toContain('90s');
  });

  it('renders labelled mobile scheduler controls without changing task inputs', () => {
    const html = renderToStaticMarkup(
      <AutomationView state={state} onRunTask={() => undefined} onSaveState={() => undefined} initialSchedulerOpen />,
    );

    expect(html).toContain('aria-label="关闭调度中心"');
    expect(html).toContain('automation-task-number-field');
    expect(html).toContain('优先级');
    expect(html).toContain('间隔(秒)');
    expect(html).toContain('automation-task-last-time');
    expect(html).toContain('automation-task-next-time');
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

  it('defines bounded remote mobile automation sheets and a grouped task list', () => {
    expect(styleSource).toContain('.app-shell-remote .automation-feature-grid');
    expect(styleSource).toContain('.app-shell-remote .automation-feature-section-title');
    expect(styleSource).toContain('.app-shell-remote .automation-mobile-run-control');
    expect(styleSource).toContain('height: min(88dvh, 760px)');
    expect(styleSource).toContain('max-height: calc(100dvh - 8px);');
    expect(styleSource).toContain('.app-shell-remote .automation-scheduler-footer');
    expect(styleSource).toContain('overflow-y: auto');
  });

  it('hides the mobile run control outside the remote mobile breakpoint', () => {
    expect(styleSource).toContain(`.automation-summary-strip .automation-mobile-run-control {
  display: none;
}`);
  });

  it('disables scheduler interval only for rewards using specified time', () => {
    const config = {
      ...state.config,
      autoFarmMailRewardScheduleMode: 'daily_time',
    };

    expect(isSchedulerTaskIntervalDisabled(config, 'mail_reward')).toBe(true);
    expect(isSchedulerTaskIntervalDisabled({ ...config, autoFarmMailRewardScheduleMode: 'interval' }, 'mail_reward')).toBe(false);
    expect(isSchedulerTaskIntervalDisabled(config, 'own_base')).toBe(false);

    const html = renderToStaticMarkup(
      <AutomationView
        state={{
          ...state,
          config,
          scheduler: {
            ...state.scheduler,
            tasks: [
              ...state.scheduler.tasks,
              {
                id: 'mail_reward',
                label: '自动领取邮件奖励',
                priority: 93,
                intervalSec: 43200,
                enabled: true,
              },
            ],
          },
        }}
        onRunTask={() => undefined}
        initialSchedulerOpen
      />,
    );

    const mailIntervalInput = html.match(/<input[^>]*name="interval-mail_reward"[^>]*>/)?.[0] || '';
    const ownIntervalInput = html.match(/<input[^>]*name="interval-own_base"[^>]*>/)?.[0] || '';
    expect(mailIntervalInput).toContain('disabled=""');
    expect(mailIntervalInput).toContain('value="43200"');
    expect(ownIntervalInput).not.toContain('disabled=""');
  });

  it('renders daily completion badges and scheduler timestamp columns', () => {
    const html = renderToStaticMarkup(
      <AutomationView
        state={
          {
            ...state,
            scheduler: {
              ...state.scheduler,
              tasks: [
                {
                  id: 'svip_daily_gift',
                  label: 'SVIP每日礼包',
                  priority: 97,
                  intervalSec: 43200,
                  enabled: true,
                  dailyDoneToday: true,
                  lastFinishedAt: '2026-07-09T08:15:00+08:00',
                  nextRunAt: '2026-07-10T00:00:00+08:00',
                },
              ],
            },
          } as any
        }
        onRunTask={() => undefined}
        initialSchedulerOpen
      />,
    );

    expect(html).toContain('今日已完成');
    expect(html).toContain('上次执行');
    expect(html).toContain('预计下次');
    expect(html).toContain('07-09 08:15');
    expect(html).toContain('07-10 00:00');
  });

  it('renders a reward specified-time next run from scheduler state', () => {
    const html = renderToStaticMarkup(
      <AutomationView
        state={
          {
            ...state,
            scheduler: {
              ...state.scheduler,
              tasks: [
                {
                  id: 'mail_reward',
                  label: '自动领取邮件奖励',
                  priority: 93,
                  intervalSec: 43200,
                  enabled: true,
                  nextRunAt: '2026-07-11T00:04:00',
                },
              ],
            },
          } as FarmAutomationState
        }
        onRunTask={() => undefined}
        initialSchedulerOpen
      />,
    );

    expect(html).toContain('自动领取邮件奖励');
    expect(html).toContain('预计下次');
    expect(html).toContain('07-11 00:04');
  });

  it('merges refreshed runtime task state into an open scheduler draft without losing edits', () => {
    const draft = {
      ...state,
      config: {
        ...state.config,
        autoFarmOneClickEnabled: false,
      },
      scheduler: {
        ...state.scheduler,
        tasks: state.scheduler.tasks.map((task) => (task.id === 'own_base' ? { ...task, enabled: false, priority: 123 } : task)),
      },
    };
    const refreshed = {
      ...state,
      summary: { enabledTasks: 2, totalTasks: 99, todayHarvest: 7 },
      scheduler: {
        ...state.scheduler,
        tasks: state.scheduler.tasks.map((task) =>
          task.id === 'own_base'
            ? { ...task, enabled: true, dailyDoneToday: true, nextRunAt: '2026-07-10T00:00:00+08:00', lastFinishedAt: '2026-07-09T08:15:00+08:00' }
            : { ...task, enabled: true },
        ),
      },
      config: {
        ...state.config,
        autoFarmOneClickEnabled: true,
        autoFarmSvipDailyGiftDailyDoneDate: '2026-07-09',
      },
    };

    const merged = mergeAutomationRuntimeState(draft, refreshed);
    const task = merged.scheduler.tasks.find((item) => item.id === 'own_base');

    expect(task?.enabled).toBe(false);
    expect(task?.priority).toBe(123);
    expect(task?.dailyDoneToday).toBe(true);
    expect(task?.nextRunAt).toBe('2026-07-10T00:00:00+08:00');
    expect(merged.config.autoFarmOneClickEnabled).toBe(false);
    expect(merged.config.autoFarmSvipDailyGiftDailyDoneDate).toBe('2026-07-09');
    expect(merged.summary).toEqual({ enabledTasks: 0, totalTasks: 2, todayHarvest: 7 });
  });

  it('renders editable scheduler settings and save action', () => {
    const html = renderToStaticMarkup(<AutomationView state={state} onRunTask={() => undefined} onSaveState={() => undefined} initialSchedulerOpen />);

    expect(html).toContain('调度启用');
    expect(html).toContain('保存调度');
    expect(html).toContain('name="scheduler-min-gap"');
    expect(html).toContain('name="priority-own_base"');
    expect(html).toContain('name="interval-own_base"');
  });

  it('syncs scheduler task intervals from detailed interval config', () => {
    const synced = syncAutomationSchedulerIntervals({
      ...state,
      config: {
        ...state.config,
        autoFarmFriendStealIntervalSec: 5,
      },
      scheduler: {
        ...state.scheduler,
        tasks: state.scheduler.tasks.map((task) => (task.id === 'friend_steal' ? { ...task, intervalSec: 90 } : task)),
      },
    });

    const task = synced.scheduler.tasks.find((item) => item.id === 'friend_steal');

    expect(task?.intervalSec).toBe(5);
    expect(synced.config.autoFarmFriendStealIntervalSec).toBe(5);
  });

  it('syncs detailed interval config from scheduler task intervals', () => {
    const synced = syncAutomationSchedulerIntervals(
      {
        ...state,
        config: {
          ...state.config,
          autoFarmFriendStealIntervalSec: 5,
        },
        scheduler: {
          ...state.scheduler,
          tasks: state.scheduler.tasks.map((task) => (task.id === 'friend_steal' ? { ...task, intervalSec: 7 } : task)),
        },
      },
      { source: 'scheduler' },
    );

    expect(synced.config.autoFarmFriendStealIntervalSec).toBe(7);
  });

  it('prefers backend-provided scheduler config keys over compatibility maps', () => {
    const canonicalState = {
      ...state,
      config: {
        ...state.config,
        autoFarmOneClickEnabled: true,
        autoFarmOwnBaseIntervalSec: 60,
        'canonical.ownBase.enabled': false,
        'canonical.ownBase.interval': 123,
      },
      scheduler: {
        ...state.scheduler,
        tasks: [
          {
            ...state.scheduler.tasks[0],
            enabledConfigKey: 'canonical.ownBase.enabled',
            intervalConfigKey: 'canonical.ownBase.interval',
          },
        ],
      },
    } as FarmAutomationState;

    const synced = syncAutomationSchedulerEnabledState(syncAutomationSchedulerIntervals(canonicalState));

    expect(synced.scheduler.tasks[0].enabled).toBe(false);
    expect(synced.scheduler.tasks[0].intervalSec).toBe(123);
  });

  it('edits and saves backend-provided scheduler config keys from detailed settings', async () => {
    const savedStates: FarmAutomationState[] = [];
    const renderer = TestRenderer.create(
      <AutomationView
        state={{
          ...state,
          config: {
            ...state.config,
            autoFarmOneClickEnabled: true,
            autoFarmOwnBaseIntervalSec: 60,
            'canonical.ownBase.enabled': true,
            'canonical.ownBase.interval': 60,
          },
          scheduler: {
            ...state.scheduler,
            tasks: [
              {
                ...state.scheduler.tasks[0],
                enabledConfigKey: 'canonical.ownBase.enabled',
                intervalConfigKey: 'canonical.ownBase.interval',
              },
            ],
          },
        }}
        onRunTask={() => undefined}
        onSaveState={(next) => {
          savedStates.push(next);
          throw new Error('stop after capturing save payload');
        }}
        initialSettingsGroupId="own_base"
      />,
    );
    const enabledInput = renderer.root.findByProps({ name: 'config-canonical.ownBase.enabled' });
    const intervalInput = renderer.root.findByProps({ name: 'config-canonical.ownBase.interval' });
    const saveButton = renderer.root.findByProps({ className: 'automation-save-button' });

    await act(async () => {
      enabledInput.props.onChange({ currentTarget: { checked: false } });
      intervalInput.props.onChange({ currentTarget: { value: '123' } });
      await saveButton.props.onClick();
    });

    expect(savedStates).toHaveLength(1);
    expect(savedStates[0].config['canonical.ownBase.enabled']).toBe(false);
    expect(savedStates[0].config['canonical.ownBase.interval']).toBe(123);
    expect(savedStates[0].scheduler.tasks[0].enabled).toBe(false);
    expect(savedStates[0].scheduler.tasks[0].intervalSec).toBe(123);
  });

  it('saves the latest interval edit before React renders again', async () => {
    const savedStates: FarmAutomationState[] = [];
    const renderer = TestRenderer.create(
      <AutomationView
        state={{
          ...state,
          scheduler: {
            ...state.scheduler,
            tasks: [
              ...state.scheduler.tasks,
              { id: 'own_collect', label: '自动收获', priority: 91, intervalSec: 30, enabled: true },
            ],
          },
        }}
        onRunTask={() => undefined}
        onSaveState={(next) => {
          savedStates.push(next);
          throw new Error('stop after capturing save payload');
        }}
        initialSettingsGroupId="own_base"
      />,
    );
    const intervalInput = renderer.root.findByProps({ name: 'config-autoFarmOwnCollectIntervalSec' });
    const saveButton = renderer.root.findByProps({ className: 'automation-save-button' });

    await act(async () => {
      intervalInput.props.onChange({ currentTarget: { value: '120' } });
      await saveButton.props.onClick();
    });

    expect(savedStates).toHaveLength(1);
    expect(savedStates[0].config.autoFarmOwnCollectIntervalSec).toBe(120);
    expect(savedStates[0].scheduler.tasks.find((task) => task.id === 'own_collect')?.intervalSec).toBe(120);
  });

  it('shows a right-bottom save success toast after scheduler and settings saves', () => {
    expect(automationViewSource).toContain('setSaveToastVisible(true)');
    expect(automationViewSource).toContain('<SaveSuccessToast visible={saveToastVisible} />');
  });

  it('renders the latest manual run result as an auto-dismissing toast without claiming success', () => {
    const html = renderToStaticMarkup(
      <AutomationView
        state={state}
        onRunTask={() => undefined}
        lastActionResult={{
          ok: false,
          status: 'not_migrated',
          taskId: 'friend_steal',
          message: '该自动化任务的真实运行时脚本尚未迁移。',
        }}
      />,
    );

    expect(html).toContain('social-toast');
    expect(html).toContain('automation-action-toast');
    expect(html).toContain('role="status"');
    expect(html).toContain('该自动化任务的真实运行时脚本尚未迁移。');
    expect(html).not.toContain('automation-action-result');
    expect(html).not.toContain('执行成功');
    expect(AUTOMATION_TOAST_AUTO_DISMISS_MS).toBe(3500);
  });

  it('does not replay a manual run result after the automation page remounts', () => {
    vi.stubGlobal('window', { clearTimeout, setTimeout });

    function AutomationPageBoundary() {
      const [mounted, setMounted] = useState(true);
      const [actionResult, setActionResult] = useState({
        ok: false,
        status: 'not_migrated',
        taskId: 'friend_steal',
        message: '该自动化任务的真实运行时脚本尚未迁移。',
      });

      return (
        <>
          <button type="button" aria-label="离开自动化" onClick={() => setMounted(false)}>离开</button>
          <button type="button" aria-label="返回自动化" onClick={() => setMounted(true)}>返回</button>
          {mounted && (
            <AutomationView
              state={state}
              lastActionResult={actionResult}
              onActionResultConsumed={() => setActionResult(null)}
              onRunTask={() => undefined}
            />
          )}
        </>
      );
    }

    const renderer = TestRenderer.create(<AutomationPageBoundary />);
    expect(JSON.stringify(renderer.toJSON())).toContain('该自动化任务的真实运行时脚本尚未迁移。');

    act(() => renderer.root.findByProps({ 'aria-label': '离开自动化' }).props.onClick());
    act(() => renderer.root.findByProps({ 'aria-label': '返回自动化' }).props.onClick());

    expect(JSON.stringify(renderer.toJSON())).not.toContain('该自动化任务的真实运行时脚本尚未迁移。');
  });

  it('renders runtime not ready copy for mapped automation tasks', () => {
    const html = renderToStaticMarkup(
      <AutomationView
        state={state}
        onRunTask={() => undefined}
        lastActionResult={{
          ok: false,
          status: 'runtime_not_ready',
          taskId: 'own_base',
          message: '游戏运行时尚未连接，无法执行自家基础任务。',
        }}
      />,
    );

    expect(html).toContain('social-toast');
    expect(html).toContain('automation-action-toast');
    expect(html).toContain('运行时未就绪');
    expect(html).toContain('游戏运行时尚未连接');
    expect(html).not.toContain('automation-action-result');
    expect(html).not.toContain('执行成功');
  });

  it('renders editable own base detailed settings instead of the placeholder', () => {
    const html = renderToStaticMarkup(
      <AutomationView state={state} onRunTask={() => undefined} onSaveState={() => undefined} initialSettingsGroupId="own_base" />,
    );

    expect(html).not.toContain('基础任务总开关');
    expect(html).not.toContain('name="config-autoFarmBasicTasksEnabled"');
    expect(html.match(/automation-settings-pair/g) || []).toHaveLength(3);
    expect(html).toContain('一键务农');
    expect(html).toContain('自动收获');
    expect(html).toContain('土地自动升级');
    expect(html).not.toContain('只在自家农场收获');
    expect(html).not.toContain('自动除草');
    expect(html).not.toContain('自动浇水');
    expect(html).not.toContain('自动杀虫');
    expect(html).not.toContain('name="config-autoFarmOwnCollectOnlyWhenOwnFarm"');
    expect(html).not.toContain('name="config-autoFarmOwnEraseGrassEnabled"');
    expect(html).not.toContain('name="config-autoFarmOwnWaterEnabled"');
    expect(html).not.toContain('name="config-autoFarmOwnKillBugEnabled"');
    expect(html).toContain('name="config-autoFarmOneClickEnabled"');
    expect(html).toContain('name="config-autoFarmOwnCollectEnabled"');
    expect(html).toContain('name="config-autoFarmLandUpgradeEnabled"');
    expect(html).toContain('name="config-autoFarmOwnBaseIntervalSec"');
    expect(html).toContain('name="config-autoFarmOwnCollectIntervalSec"');
    expect(html).toContain('name="config-autoFarmLandUpgradeIntervalSec"');
    expect(html).toContain('保存设置');
    expect(html).not.toContain('详细脚本设置会按功能切片迁移');
  });

  it('disables detailed controls while a feature group is off without hiding their values', () => {
    const html = renderToStaticMarkup(
      <AutomationView
        state={{
          ...state,
          featureGroups: state.featureGroups.map((group) => (group.id === 'own_base' ? { ...group, enabled: false } : group)),
        }}
        onRunTask={() => undefined}
        onSaveState={() => undefined}
        initialSettingsGroupId="own_base"
      />,
    );

    expect(html).toContain('<fieldset disabled=""');
    expect(html).toContain('name="config-autoFarmOneClickEnabled"');
    expect(html).toContain('checked=""');
    expect(html).toContain('保存设置');
  });

  it('renders editable planting strategy detailed settings', () => {
    const html = renderToStaticMarkup(
      <AutomationView state={state} onRunTask={() => undefined} onSaveState={() => undefined} initialSettingsGroupId="planting" />,
    );

    expect(html).toContain('主种植策略');
    expect(html).toContain('背包优先');
    expect(html).toContain('开启四格种植');
    expect(html).toContain('背包种子选择器');
    expect(html).toContain('刷新背包');
    expect(html).toContain('重置');
    expect(html).toContain('automation-random-delay-card');
    expect(html).toContain('琉璃宝荷');
    expect(html).toContain('Lv.200');
    expect(html).toContain('9');
    expect(html).toContain('禁用');
    expect(html).toContain('启用');
    expect(html).toContain('操作');
    expect(html).not.toContain('排序');
    expect(html).not.toContain('上移');
    expect(html).not.toContain('下移');
    expect(html).toContain('随机延迟最小(ms)');
    expect(html).not.toContain('指定种子ID');
    expect(html).not.toContain('指定种子优先');
    expect(html).toContain('最高等级作物');
    expect(html).not.toContain('最高等级限制');
    expect(html).not.toContain('#21032');
    expect(html).not.toContain('背包种子优先级');
    expect(html).not.toContain('背包种子禁用列表');
    expect(html).toContain('name="config-autoFarmPlantPrimaryMode"');
    expect(html).toContain('name="config-autoFarmPlantRandomizedDelayMinMs"');
    expect(html.indexOf('强制使用背包优先级')).toBeLessThan(html.indexOf('随机土地顺序'));
    expect(html.indexOf('随机土地顺序')).toBeLessThan(html.indexOf('启用随机种植延迟'));
    expect(html.indexOf('启用随机种植延迟')).toBeLessThan(html.indexOf('name="config-autoFarmPlantRandomizedDelayMinMs"'));
    expect(html.indexOf('name="config-autoFarmPlantRandomizedDelayMinMs"')).toBeLessThan(html.indexOf('name="config-autoFarmPlantRandomizedDelayMaxMs"'));
    const plantDelayMinInput = html.match(/<input[^>]*name="config-autoFarmPlantRandomizedDelayMinMs"[^>]*>/)?.[0] || '';
    const plantDelayMaxInput = html.match(/<input[^>]*name="config-autoFarmPlantRandomizedDelayMaxMs"[^>]*>/)?.[0] || '';
    expect(plantDelayMinInput).toContain('max="10000"');
    expect(plantDelayMaxInput).toContain('max="10000"');
  });

  it('disables unavailable four-grid backpack seeds in the priority selector', () => {
    const html = renderToStaticMarkup(
      <AutomationView
        state={{
          ...state,
          config: {
            ...state.config,
            autoFarmFourGridPlantEnabled: false,
            autoFarmPlantBackpackSeedOptions: [
              {
                seedId: 20416,
                name: '哈哈南瓜',
                level: 200,
                count: 8,
                plantable: false,
                plantableReason: 'multi_tile_seed_not_supported',
                plantableMessage: '哈哈南瓜为四格作物，当前背包种植策略不支持',
                plantSize: 2,
              },
            ],
            autoFarmPlantBackpackSeedPriority: [20416],
            autoFarmPlantBackpackSeedDisabled: [],
          },
        }}
        onRunTask={() => undefined}
        onSaveState={() => undefined}
        initialSettingsGroupId="planting"
      />,
    );

    expect(html).toContain('automation-backpack-row disabled');
    expect(html).toContain('name="config-autoFarmPlantBackpackSeedPriority-20416"');
    expect(html).toContain('disabled=""');
    expect(html).toContain('四格作物');
  });

  it('guards backpack seed auto-refresh to the active backpack planting panel once', () => {
    expect(automationViewSource).toContain('usesBackpackPlantStrategy(draftState.config)');
    expect(automationViewSource).toContain('autoRefreshedBackpackSettingsKey');
    expect(automationViewSource).toContain('!schedulerOpen && !settingsGroup');
  });

  it('reorders backpack planting priority with every visible seed as a drop target', () => {
    expect(reorderBackpackSeedPriority([21032], [21032, 20133, 20176], 20176, 20133)).toEqual([21032, 20176, 20133]);
    expect(reorderBackpackSeedPriority([21032, 20133], [21032, 20133, 20176], 20133, 20176)).toEqual([21032, 20176, 20133]);
    expect(reorderBackpackSeedPriority([21032], [21032, 20133, 20176], 21032, 21032)).toEqual([21032]);
  });

  it('resets backpack planting priority to the latest refreshed seed option order', () => {
    expect(
      resetBackpackSeedPriority([
        { seedId: 21032 },
        { itemId: 20133 },
        { id: 20176 },
        { seed_id: 20133 },
      ]),
    ).toEqual([21032, 20133, 20176]);
  });

  it('does not reset unavailable backpack seeds into the planting priority', () => {
    expect(
      resetBackpackSeedPriority([
        { seedId: 20416, plantable: false },
        { seedId: 20133, plantable: true },
      ]),
    ).toEqual([20133]);
  });

  it('renders editable fertilizer and rush detailed settings', () => {
    const html = renderToStaticMarkup(
      <AutomationView state={state} onRunTask={() => undefined} onSaveState={() => undefined} initialSettingsGroupId="fertilizer" />,
    );

    expect(html).not.toContain('自动施肥总开关');
    expect(html).not.toContain('name="config-autoFarmFertilizerEnabled"');
    expect(html).toContain('种植施肥策略');
    expect(html).toContain('智能无机肥');
    expect(html).toContain('智能有机肥');
    expect(html).toContain('催熟策略');
    expect(html).toContain('催熟联动收获');
    expect(html).toContain('多季节作物补肥');
    expect(html).toContain('自动填充肥料');
    expect(html).toContain('填充肥料间隔(秒)');
    expect(html).toContain('紫金土地');
    expect(html).toContain('延迟施肥');
    expect(html).toContain('name="config-autoFarmPlantFertilizerMode"');
    expect(html).toContain('name="config-autoFarmFertilizerIntervalSec"');
    expect(html).toContain('name="config-autoFarmFertilizerRushThresholdSec"');
    expect(html).toContain('name="config-autoFarmFertilizerDelayedSubmitEnabled"');
    expect(html).not.toContain('name="config-autoFarmFertilizerDelayedSubmitIntervalMs"');
    expect(html).not.toContain('种植后施肥模式');
    expect(html).not.toContain('催熟模式');
    expect(html).not.toContain('多季作物施肥');
    expect(html).not.toContain('肥料不足自动购买填充');
    expect(html).not.toContain('自动购买类型');
    expect(html).not.toContain('自动购买最大数量');
    expect(html).not.toContain('name="config-autoFarmFertilizerAutoBuyEnabled"');
    expect(html).not.toContain('name="config-autoFarmFertilizerAutoBuyType"');
    expect(html).not.toContain('name="config-autoFarmFertilizerAutoBuyMaxCount"');
    expect(html).not.toContain('批量施肥超时(ms)');
    expect(html).not.toContain('单块施肥超时(ms)');
    expect(html).not.toContain('每轮最多土地数');
    expect(html).toContain('name="config-autoFarmFertilizerFillEnabled"');
    expect(html).toContain('name="config-autoFarmFertilizerFillIntervalSec"');
    expect(html).not.toContain('name="config-autoFarmFertilizeBatchCallTimeoutMs"');
    expect(html).not.toContain('name="config-autoFarmFertilizeSingleCallTimeoutMs"');
    expect(html).not.toContain('name="config-autoFarmFertilizerMaxLandsPerRun"');
  });

  it('renders editable delayed fertilizer scopes when enabled', () => {
    const html = renderToStaticMarkup(
      <AutomationView
        state={{
          ...state,
          config: {
            ...state.config,
            autoFarmFertilizerDelayedSubmitEnabled: true,
            autoFarmFertilizerDelayedSubmitScopes: ['planting', 'rush', 'manual'],
            autoFarmFertilizerDelayedSubmitIntervalMs: 500,
          },
        }}
        onRunTask={() => undefined}
        onSaveState={() => undefined}
        initialSettingsGroupId="fertilizer"
      />,
    );

    expect(html).toContain('延迟施肥');
    expect(html).toContain('作用范围');
    expect(html).toContain('种植策略');
    expect(html).toContain('催熟策略');
    expect(html).toContain('手动执行');
    expect(html).toContain('地块间隔(毫秒)');
    expect(html).toContain('name="config-autoFarmFertilizerDelayedSubmitEnabled"');
    expect(html).toContain('name="config-autoFarmFertilizerDelayedSubmitIntervalMs"');
    expect(html).toContain('name="config-autoFarmFertilizerDelayedSubmitScopes-planting"');
    expect(html).toContain('name="config-autoFarmFertilizerDelayedSubmitScopes-rush"');
    expect(html).toContain('name="config-autoFarmFertilizerDelayedSubmitScopes-manual"');
  });

  it('renders editable rewards and events detailed settings', () => {
    const html = renderToStaticMarkup(
      <AutomationView state={state} onRunTask={() => undefined} onSaveState={() => undefined} initialSettingsGroupId="rewards" />,
    );

    expect(html).toContain('自动领取任务奖励');
    expect(html).toContain('SVIP每日礼包');
    expect(html).toContain('月卡奖励');
    expect(html).toContain('商城每日肥料');
    expect(html).toContain('分享奖励');
    expect(html).toContain('邮件奖励');
	    expect(html).toContain('千星游记奖励领取');
	    expect(html).toContain('name="config-autoFarmQianXingTravelRewardEnabled"');
	    expect(html).toContain('name="config-autoFarmQianXingTravelRewardIntervalMin"');
	    expect(html).toContain('name="config-autoFarmQianXingTravelRewardIntervalSec"');
	    expect(html).toContain('value="120"');
	    expect(html.indexOf('邮件奖励')).toBeLessThan(html.indexOf('千星游记奖励领取'));
	    expect(html).toContain('自动点亮星宿');
	    expect(html).toContain('name="config-autoFarmXingSuAutoLightUpEnabled"');
	    expect(html).toContain('name="config-autoFarmXingSuAutoLightUpIntervalMin"');
	    expect(html).toContain('name="config-autoFarmXingSuAutoLightUpIntervalSec"');
	    expect(html.indexOf('千星游记奖励领取')).toBeLessThan(html.indexOf('自动点亮星宿'));
    expect(html).not.toContain('荷风游记奖励');
    expect(html).not.toContain('荷风游记抽奖');
    expect(html).not.toContain('允许付费抽奖');
    expect(html).toContain('执行模式');
    expect(html).toContain('间隔执行');
    expect(html).toContain('指定时间执行');
    expect(html).toContain('间隔(分钟)');
    expect(html).not.toContain('automation-he-feng-draw-card');
    expect(html).not.toContain('限时种子抽奖');
    expect(html).not.toContain('任务奖励兜底间隔');
    expect(html).not.toContain('name="config-autoRewardClaimIntervalHour"');
    expect(html).not.toContain('name="config-autoFarmHeFengTravelRewardEnabled"');
    expect(html).not.toContain('name="config-autoFarmLimitedSeedDrawEnabled"');
    expect(html).not.toContain('name="config-autoFarmLimitedSeedDrawPaidEnabled"');
    expect(html).not.toContain('name="config-autoFarmLimitedSeedDrawScheduleMode"');
    expect(html).not.toContain('name="config-autoFarmLimitedSeedDrawIntervalMin"');
    expect(html).not.toContain('name="config-autoFarmLimitedSeedDrawIntervalSec"');
  });

  it('renders reward specified-time scheduling when selected', () => {
    const scheduledState: FarmAutomationState = {
      ...state,
      config: {
        ...state.config,
        autoFarmSvipDailyGiftScheduleMode: 'daily_time',
        autoFarmSvipDailyGiftScheduleTime: '09:30',
      },
    };
    const html = renderToStaticMarkup(
      <AutomationView state={scheduledState} onRunTask={() => undefined} onSaveState={() => undefined} initialSettingsGroupId="rewards" />,
    );

    expect(html).toContain('指定时间执行');
    expect(html).toContain('name="config-autoFarmSvipDailyGiftScheduleTime"');
    expect(html).toContain('type="time"');
    expect(html).toContain('09:30');
  });

  it('renders editable mystery shop auto buy detailed settings', () => {
    const html = renderToStaticMarkup(
      <AutomationView state={state} onRunTask={() => undefined} onSaveState={() => undefined} initialSettingsGroupId="mystery_shop" />,
    );

    expect(html).not.toContain('启用神秘商店自动购买');
    expect(html).not.toContain('name="config-autoFarmMysteryShopAutoBuyEnabled"');
    expect(html).not.toContain('目标种子ID');
    expect(html).not.toContain('name="config-autoFarmMysteryShopTargetSeedIds"');
    expect(html).toContain('读取当前神秘商店商品和价格');
    expect(html).toContain('按预设自动购买');
    expect(automationViewSource).toContain("onRunTask('mystery_shop_read')");
    expect(automationViewSource).toContain("onRunTask('mystery_shop_auto_buy')");
    expect(html).toContain('允许货币');
    expect(html).toContain('金币');
    expect(html).toContain('点券');
    expect(html).toContain('折扣阈值');
    expect(html).toContain('全部折扣');
    expect(html).toContain('name="config-autoFarmMysteryShopAutoBuyIntervalSec"');
  });

  it('renders mystery shop purchase records and the gold bean currency option', () => {
    const html = renderToStaticMarkup(
      <AutomationView
        state={state}
        onRunTask={() => undefined}
        initialSettingsGroupId="mystery_shop"
        mysteryShopPurchaseRecords={[
          {
            id: 'shop-1',
            occurredAt: '2026-07-26T19:42:00Z',
            itemName: '高级化肥',
            count: 2,
            unitPrice: 80,
            currencyId: 1005,
            currencyName: '金豆豆',
            discount: 50,
          },
        ]}
      />,
    );

    expect(html).toContain('金豆豆');
    expect(html).toContain('购买记录');
    expect(html).toContain('高级化肥 x2');
    expect(html).toContain('80 金豆豆');
    expect(html).toContain('5折');
  });

  it('renders an empty state when the current account has no mystery shop purchases', () => {
    const html = renderToStaticMarkup(
      <AutomationView state={state} onRunTask={() => undefined} initialSettingsGroupId="mystery_shop" mysteryShopPurchaseRecords={[]} />,
    );

    expect(html).toContain('暂无成功购买记录');
  });

  it('renders the latest mystery shop manual result inside the settings panel', () => {
    const html = renderToStaticMarkup(
      <AutomationView
        state={state}
        onRunTask={() => undefined}
        onSaveState={() => undefined}
        initialSettingsGroupId="mystery_shop"
        lastActionResult={{
          ok: true,
          status: 'ok',
          taskId: 'mystery_shop_read',
          message: '已读取当前神秘商店商品 0 个。',
        }}
      />,
    );

    expect(html).toContain('automation-mystery-result');
    expect(html).toContain('当前协议返回 0 个商品');
    expect(html).toContain('当前没有读取到神秘商店商品');
  });

  it('renders editable friend automation detailed settings', () => {
    const html = renderToStaticMarkup(
      <AutomationView state={state} onRunTask={() => undefined} onSaveState={() => undefined} initialSettingsGroupId="friends" />,
    );

    expect(html).toContain('好友偷菜');
    expect(html).toContain('好友帮忙');
    expect(html).toContain('好友捣乱');
    expect(html).toContain('automation-friend-action-row');
    expect(html).toContain('帮忙每日上限');
    expect(html).toContain('捣乱次数按游戏回包识别');
    expect(html).toContain('每批帮助好友数量');
    expect(html).not.toContain('访问冷却(分钟)');
    expect(html).not.toContain('name="config-autoFarmFriendVisitCooldownMin"');
    expect(html).toContain('黑名单跳过冷却(分钟)');
    expect(html).toContain('启用静默时间');
    expect(html).toContain('指定时间休眠');
    expect(html).toContain('指定时间工作');
    expect(html).toContain('作用范围');
    expect(html).toContain('name="config-autoFarmFriendQuietHoursScopes-steal"');
    expect(html).toContain('name="config-autoFarmFriendQuietHoursScopes-help"');
    expect(html).toContain('name="config-autoFarmFriendQuietHoursScopes-mischief"');
    expect(html).not.toContain('指定时间内不进行偷菜/捣乱/帮助');
    expect(html).not.toContain('偷菜好友上限');
    expect(html).not.toContain('捣乱每日上限');
    expect(html).not.toContain('name="config-autoFarmFriendMischiefDailyLimit"');
    expect(html).not.toContain('name="config-autoFarmFriendStealMaxFriends"');
    expect(html).not.toContain('name="config-autoFarmFriendMischiefMaxFriends"');
    expect(html).not.toContain('白名单范围');
    expect(html).not.toContain('黑名单列表');
    expect(html).not.toContain('屏蔽低级匿名偷菜者');
    expect(html).not.toContain('捣乱放草地块');
    expect(html).not.toContain('捣乱放虫地块');
    expect(html).not.toContain('name="config-autoFarmFriendMischiefGrassLandIds"');
    expect(html).not.toContain('name="config-autoFarmFriendMischiefBugLandIds"');
    expect(html).toContain('运行模式');
    expect(html).toContain('黑名单策略');
    expect(html).toContain('跳过黑名单农场');
    expect(html).toContain('跳过黑名单作物');
    expect(html).toContain('黑名单作物选择列表');
    expect(html).toContain('展开黑名单作物列表');
    expect(html).not.toContain('严苛模式');
    expect(html).not.toContain('name="config-autoFarmFriendStealPlantBlacklistStrictModeEnabled"');
    expect(html).not.toContain('偷菜作物名单模式');
    expect(html).not.toContain('偷菜作物黑名单<textarea');
    expect(html).not.toContain('偷菜作物白名单<textarea');
    expect(html).toContain('name="config-autoFarmFriendStealIntervalSec"');
    expect(html).toContain('随机偷取延迟');
    expect(html).toContain('name="config-autoFarmFriendStealRandomDelayEnabled"');
    expect(html).toContain('name="config-autoFarmFriendStealRandomDelayMinMs"');
    expect(html).toContain('name="config-autoFarmFriendStealRandomDelayMaxMs"');
    const randomDelayMinInput = html.match(/<input[^>]*name="config-autoFarmFriendStealRandomDelayMinMs"[^>]*>/)?.[0] || '';
    const randomDelayMaxInput = html.match(/<input[^>]*name="config-autoFarmFriendStealRandomDelayMaxMs"[^>]*>/)?.[0] || '';
    expect(randomDelayMinInput).toContain('disabled=""');
    expect(randomDelayMaxInput).toContain('disabled=""');
    expect(styleSource).toMatch(
      /\.automation-friend-quiet-primary\s*\{[^}]*grid-template-columns:\s*minmax\([^;]+repeat\(2,/s,
    );
    expect(styleSource).toMatch(/\.automation-friend-quiet-times\s*\{[^}]*grid-template-columns:\s*repeat\(2,/s);
    expect(styleSource).toMatch(
      /@media \(max-width: 760px\)[\s\S]*\.automation-friend-quiet-primary,[\s\S]*\.automation-friend-quiet-times\s*\{[^}]*grid-template-columns:\s*1fr/s,
    );
  });

  it('refreshes persisted steal crop images when the friends settings open remotely', async () => {
    const onRefreshStealCropOptions = vi.fn().mockResolvedValue({
      ok: true,
      list: [
        { plantId: 1020002, seedId: 20002, name: '白萝卜', level: 1, imageUrl: '/farm-assets/fresh-crop-image' },
      ],
    });

    await act(async () => {
      TestRenderer.create(
        <AutomationView
          remote
          state={state}
          onRunTask={() => undefined}
          onRefreshStealCropOptions={onRefreshStealCropOptions}
          initialSettingsGroupId="friends"
        />,
      );
      await Promise.resolve();
    });

    expect(onRefreshStealCropOptions).toHaveBeenCalledTimes(1);
  });

  it('edits and saves friend steal random delay settings', async () => {
    vi.stubGlobal('window', { clearTimeout, setTimeout });
    const onSaveState = vi.fn(async (next: FarmAutomationState) => next);
    const renderer = TestRenderer.create(
      <AutomationView state={state} onRunTask={() => undefined} onSaveState={onSaveState} initialSettingsGroupId="friends" />,
    );
    const enabledInputs = renderer.root.findAllByProps({ name: 'config-autoFarmFriendStealRandomDelayEnabled' });
    const minInputs = renderer.root.findAllByProps({ name: 'config-autoFarmFriendStealRandomDelayMinMs' });
    const maxInputs = renderer.root.findAllByProps({ name: 'config-autoFarmFriendStealRandomDelayMaxMs' });

    expect(enabledInputs).toHaveLength(1);
    expect(minInputs).toHaveLength(1);
    expect(maxInputs).toHaveLength(1);

    await act(async () => {
      enabledInputs[0].props.onChange({ currentTarget: { checked: true } });
      minInputs[0].props.onChange({ currentTarget: { value: '1200' } });
      maxInputs[0].props.onChange({ currentTarget: { value: '3600' } });
      await renderer.root.findByProps({ className: 'automation-save-button' }).props.onClick();
    });

    expect(onSaveState).toHaveBeenCalledTimes(1);
    expect(onSaveState.mock.calls[0][0].config.autoFarmFriendStealRandomDelayEnabled).toBe(true);
    expect(onSaveState.mock.calls[0][0].config.autoFarmFriendStealRandomDelayMinMs).toBe(1200);
    expect(onSaveState.mock.calls[0][0].config.autoFarmFriendStealRandomDelayMaxMs).toBe(3600);
  });

  it('edits and saves planting random delay settings', async () => {
    vi.stubGlobal('window', { clearTimeout, setTimeout });
    const onSaveState = vi.fn(async (next: FarmAutomationState) => next);
    const renderer = TestRenderer.create(
      <AutomationView state={state} onRunTask={() => undefined} onSaveState={onSaveState} initialSettingsGroupId="planting" />,
    );
    const enabled = renderer.root.findByProps({ name: 'config-autoFarmPlantRandomizedEnabled' });
    const min = renderer.root.findByProps({ name: 'config-autoFarmPlantRandomizedDelayMinMs' });
    const max = renderer.root.findByProps({ name: 'config-autoFarmPlantRandomizedDelayMaxMs' });

    await act(async () => {
      enabled.props.onChange({ currentTarget: { checked: true } });
      min.props.onChange({ currentTarget: { value: '1200' } });
      max.props.onChange({ currentTarget: { value: '3600' } });
      await renderer.root.findByProps({ className: 'automation-save-button' }).props.onClick();
    });

    expect(onSaveState).toHaveBeenCalledTimes(1);
    expect(onSaveState.mock.calls[0][0].config.autoFarmPlantRandomizedEnabled).toBe(true);
    expect(onSaveState.mock.calls[0][0].config.autoFarmPlantRandomizedDelayMinMs).toBe(1200);
    expect(onSaveState.mock.calls[0][0].config.autoFarmPlantRandomizedDelayMaxMs).toBe(3600);
  });

  it.each([
    ['5000', '1000', '最大种植随机延迟不能小于最小种植随机延迟'],
    ['-1', '1000', '种植随机延迟必须是 0 到 10000 的整数'],
    ['100', '10001', '种植随机延迟必须是 0 到 10000 的整数'],
  ])('rejects invalid planting random delay range %s-%s', async (minValue, maxValue, message) => {
    vi.stubGlobal('window', { clearTimeout, setTimeout });
    const onSaveState = vi.fn();
    const renderer = TestRenderer.create(
      <AutomationView state={state} onRunTask={() => undefined} onSaveState={onSaveState} initialSettingsGroupId="planting" />,
    );

    await act(async () => {
      renderer.root.findByProps({ name: 'config-autoFarmPlantRandomizedDelayMinMs' }).props.onChange({ currentTarget: { value: minValue } });
      renderer.root.findByProps({ name: 'config-autoFarmPlantRandomizedDelayMaxMs' }).props.onChange({ currentTarget: { value: maxValue } });
      await renderer.root.findByProps({ className: 'automation-save-button' }).props.onClick();
    });

    expect(onSaveState).not.toHaveBeenCalled();
    expect(renderer.root.findAllByType('span').map((node) => node.children.join('')).join('')).toContain(message);
  });

  it('rejects an invalid friend steal random delay range before saving', async () => {
    const onSaveState = vi.fn();
    const renderer = TestRenderer.create(
      <AutomationView state={state} onRunTask={() => undefined} onSaveState={onSaveState} initialSettingsGroupId="friends" />,
    );
    const minInputs = renderer.root.findAllByProps({ name: 'config-autoFarmFriendStealRandomDelayMinMs' });
    const maxInputs = renderer.root.findAllByProps({ name: 'config-autoFarmFriendStealRandomDelayMaxMs' });

    expect(minInputs).toHaveLength(1);
    expect(maxInputs).toHaveLength(1);

    await act(async () => {
      minInputs[0].props.onChange({ currentTarget: { value: '5000' } });
      maxInputs[0].props.onChange({ currentTarget: { value: '1000' } });
      await renderer.root.findByProps({ className: 'automation-save-button' }).props.onClick();
    });

    expect(onSaveState).not.toHaveBeenCalled();
    expect(renderer.root.findAllByType('span').map((node) => node.children.join('')).join('')).toContain('最大随机偷取延迟不能小于最小随机偷取延迟');
  });

  it('edits and saves friend quiet-hours mode, scopes, and times', async () => {
    const savedStates: FarmAutomationState[] = [];
    const renderer = TestRenderer.create(
      <AutomationView
        state={state}
        onRunTask={() => undefined}
        onSaveState={(next) => {
          savedStates.push(next);
          throw new Error('stop after capturing save payload');
        }}
        initialSettingsGroupId="friends"
      />,
    );

    expect(renderer.root.findByProps({ name: 'config-autoFarmFriendQuietHoursScopes-steal' }).props.checked).toBe(true);
    expect(renderer.root.findByProps({ name: 'config-autoFarmFriendQuietHoursScopes-help' }).props.checked).toBe(true);
    expect(renderer.root.findByProps({ name: 'config-autoFarmFriendQuietHoursScopes-mischief' }).props.checked).toBe(false);

    await act(async () => {
      renderer.root.findByProps({ 'aria-label': '指定时间工作' }).props.onClick();
    });
    await act(async () => {
      renderer.root.findByProps({ name: 'config-autoFarmFriendQuietHoursScopes-help' }).props.onChange({ currentTarget: { checked: false } });
    });
    await act(async () => {
      renderer.root.findByProps({ name: 'config-autoFarmFriendQuietHoursScopes-mischief' }).props.onChange({ currentTarget: { checked: true } });
    });
    await act(async () => {
      renderer.root.findByProps({ name: 'config-autoFarmFriendQuietHoursStart' }).props.onChange({ currentTarget: { value: '09:00' } });
      renderer.root.findByProps({ name: 'config-autoFarmFriendQuietHoursEnd' }).props.onChange({ currentTarget: { value: '17:30' } });
    });
    await act(async () => {
      await renderer.root.findByProps({ className: 'automation-save-button' }).props.onClick();
    });

    expect(savedStates).toHaveLength(1);
    expect(savedStates[0].config.autoFarmFriendQuietHoursMode).toBe('work');
    expect(savedStates[0].config.autoFarmFriendQuietHoursScopes).toEqual(['steal', 'mischief']);
    expect(savedStates[0].config.autoFarmFriendQuietHoursStart).toBe('09:00');
    expect(savedStates[0].config.autoFarmFriendQuietHoursEnd).toBe('17:30');
  });

  it('disables guard-only friend help until guard dog friends have been loaded', () => {
    const html = renderToStaticMarkup(
      <AutomationView state={state} dogGuardFriendCount={0} onRunTask={() => undefined} initialSettingsGroupId="friends" />,
    );
    const guardOnlyInput = html.match(/<input[^>]*name="config-autoFarmFriendHelpGuardDogOnly"[^>]*>/)?.[0] || '';

    expect(html).toContain('仅帮助护主犬好友');
    expect(html).toContain('请先读取护主犬好友');
    expect(guardOnlyInput).toContain('disabled=""');
  });

  it('enables guard-only friend help when guard dog friends are available', () => {
    const html = renderToStaticMarkup(
      <AutomationView state={state} dogGuardFriendCount={2} onRunTask={() => undefined} initialSettingsGroupId="friends" />,
    );
    const guardOnlyInput = html.match(/<input[^>]*name="config-autoFarmFriendHelpGuardDogOnly"[^>]*>/)?.[0] || '';

    expect(guardOnlyInput).not.toContain('disabled=""');
    expect(html).not.toContain('请先读取护主犬好友');
  });

  it('clears a guard-only draft when the available guard dog friend count becomes zero', async () => {
    const savedStates: FarmAutomationState[] = [];
    const guardOnlyState = {
      ...state,
      config: { ...state.config, autoFarmFriendHelpGuardDogOnly: true },
    };
    const renderer = TestRenderer.create(
      <AutomationView
        state={guardOnlyState}
        dogGuardFriendCount={2}
        onRunTask={() => undefined}
        onSaveState={(next) => {
          savedStates.push(next);
          throw new Error('stop after capturing save payload');
        }}
        initialSettingsGroupId="friends"
      />,
    );
    expect(renderer.root.findByProps({ name: 'config-autoFarmFriendHelpGuardDogOnly' }).props.checked).toBe(true);

    await act(async () => {
      renderer.update(
        <AutomationView
          state={guardOnlyState}
          dogGuardFriendCount={0}
          onRunTask={() => undefined}
          onSaveState={(next) => {
            savedStates.push(next);
            throw new Error('stop after capturing save payload');
          }}
          initialSettingsGroupId="friends"
        />,
      );
    });
    expect(renderer.root.findByProps({ name: 'config-autoFarmFriendHelpGuardDogOnly' }).props.checked).toBe(false);

    await act(async () => {
      await renderer.root.findByProps({ className: 'automation-save-button' }).props.onClick();
    });
    expect(savedStates[0].config.autoFarmFriendHelpGuardDogOnly).toBe(false);
  });

  it('sanitizes a later hidden guard-only value while the available count stays zero', async () => {
    const savedStates: FarmAutomationState[] = [];
    const captureSave = (next: FarmAutomationState) => {
      savedStates.push(next);
      throw new Error('stop after capturing save payload');
    };
    const renderer = TestRenderer.create(
      <AutomationView
        state={{ ...state, config: { ...state.config, autoFarmFriendHelpGuardDogOnly: false } }}
        dogGuardFriendCount={0}
        onRunTask={() => undefined}
        onSaveState={captureSave}
      />,
    );

    await act(async () => {
      renderer.update(
        <AutomationView
          state={{ ...state, config: { ...state.config, autoFarmFriendHelpGuardDogOnly: true } }}
          dogGuardFriendCount={0}
          onRunTask={() => undefined}
          onSaveState={captureSave}
        />,
      );
    });
    await act(async () => {
      renderer.root.findByProps({ 'aria-label': '好友互动设置' }).props.onClick();
    });

    const guardOnlyInput = renderer.root.findByProps({ name: 'config-autoFarmFriendHelpGuardDogOnly' });
    expect(guardOnlyInput.props.disabled).toBe(true);
    expect(guardOnlyInput.props.checked).toBe(false);

    await act(async () => {
      await renderer.root.findByProps({ className: 'automation-save-button' }).props.onClick();
    });
    expect(savedStates[0].config.autoFarmFriendHelpGuardDogOnly).toBe(false);
  });

  it('groups steal cooldown and crop list settings under the steal switch', () => {
    const html = renderToStaticMarkup(
      <AutomationView state={state} onRunTask={() => undefined} onSaveState={() => undefined} initialSettingsGroupId="friends" />,
    );
    const stealStart = html.indexOf('<div class="automation-friend-action-row steal">');
    const helpStart = html.indexOf('<div class="automation-friend-action-row help">');
    const stealSection = stealStart >= 0 && helpStart > stealStart ? html.slice(stealStart, helpStart) : '';

    expect(stealSection).toContain('好友偷菜');
    expect(stealSection).toContain('偷菜间隔(秒)');
    expect(stealSection).toContain('黑名单跳过冷却(分钟)');
    expect(stealSection).toContain('运行模式');
    expect(stealSection).toContain('黑名单策略');
    expect(stealSection).toContain('启用作物偷取名单');
    expect(stealSection).toContain('黑名单作物选择列表');
    expect(stealSection).not.toContain('严苛模式');
    expect(stealSection).not.toContain('好友帮忙');
    expect(stealSection.indexOf('启用作物偷取名单')).toBeLessThan(stealSection.indexOf('运行模式'));
    expect(stealSection.indexOf('启用作物偷取名单')).toBeLessThan(stealSection.indexOf('黑名单策略'));
  });

  it('does not expose legacy scene wait settings inside scheduler center', () => {
    const html = renderToStaticMarkup(
      <AutomationView state={state} onRunTask={() => undefined} onSaveState={() => undefined} initialSchedulerOpen />,
    );

    expect(html).not.toContain('进入农场等待(ms)');
    expect(html).not.toContain('动作等待(ms)');
    expect(html).not.toContain('进场等待');
    expect(html).not.toContain('动作等待');
    expect(html).not.toContain('name="config-autoFarmEnterWaitMs"');
    expect(html).not.toContain('name="config-autoFarmActionWaitMs"');
    expect(html).not.toContain('自动启动农场');
    expect(html).not.toContain('启用后端调度');
    expect(html).not.toContain('调度任务缓冲(ms)');
    expect(html).not.toContain('RPC超时(ms)');
    expect(html).not.toContain('仓库自动刷新');
    expect(html).not.toContain('刷新仅在自动出售时触发');
    expect(html).not.toContain('自动出售仓库');
    expect(html).not.toContain('出售分类');
    expect(html).not.toContain('变异作物');
    expect(html).not.toContain('name="config-autoFarmRpcTimeoutMs"');
  });
});
