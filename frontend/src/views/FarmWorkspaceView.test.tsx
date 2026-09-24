import { renderToStaticMarkup } from 'react-dom/server';
import TestRenderer, { act } from 'react-test-renderer';
import { describe, expect, it } from 'vitest';

import type { FarmAutomationState } from './AutomationView';
import { FarmWorkspaceView } from './FarmWorkspaceView';
import { SocialView } from './SocialView';

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

const noop = () => undefined;

const runStatistics = {
  durationSeconds: 1122,
  collect: 8,
  farm: 5,
  steal: 1,
  help: 2,
  mischief: 3,
  saleEstimate: 1280,
  estimateReady: true,
};

const customAutomationState: FarmAutomationState = {
  running: true,
  summary: { enabledTasks: 1, totalTasks: 1, todayHarvest: 7 },
  config: {},
  featureGroups: [
    {
      id: 'custom_probe',
      label: 'Wails状态探针',
      summary: '来自 App 注入的自动化状态。',
      enabled: true,
      settingKeys: [],
    },
    {
      id: 'friends',
      label: '好友互动',
      summary: '好友设置。',
      enabled: false,
      settingKeys: ['autoFarmFriendHelpEnabled'],
    },
  ],
  scheduler: {
    enabled: true,
    minGapMs: 250,
    runningTaskId: 'custom_task',
    tasks: [
      {
        id: 'custom_task',
        label: '外部调度任务',
        priority: 88,
        intervalSec: 45,
        enabled: true,
      },
    ],
  },
};

describe('FarmWorkspaceView', () => {
	it('passes the QQ runtime capability to the social view', () => {
		const qq = renderToStaticMarkup(
			<FarmWorkspaceView
				area="social"
				status={status}
				guardStatus={guardStatus}
				bindingStatus={null}
				events={[]}
				onRefresh={noop}
				onLaunch={noop}
				onRestart={noop}
				onToggleGuard={noop}
			/>,
		);
		const nonQQ = renderToStaticMarkup(
			<FarmWorkspaceView
				area="social"
				status={{ ...status, target: 'wechat_cdp' }}
				guardStatus={guardStatus}
				bindingStatus={null}
				events={[]}
				onRefresh={noop}
				onLaunch={noop}
				onRestart={noop}
				onToggleGuard={noop}
			/>,
		);

		expect(qq).toContain('data-qq-runtime="true"');
		expect(nonQQ).toContain('data-qq-runtime="false"');
	});

	it('renders the selected single-row run statistics on the workspace', () => {
		const html = renderToStaticMarkup(
			<FarmWorkspaceView
				area="workspace"
				status={status}
				guardStatus={guardStatus}
				bindingStatus={null}
				events={[]}
				runStatistics={runStatistics}
				onRefresh={noop}
				onLaunch={noop}
				onRestart={noop}
				onToggleGuard={noop}
			/>,
		);

		expect(html).toContain('本次运行统计');
		expect(html).toContain('00:18:42');
		expect(html).toContain('收获次数');
		expect(html).toContain('出售预估收益');
		expect(html).toContain('1,280');
	});

	it('forwards dedicated TSDK events to the overview', () => {
		const html = renderToStaticMarkup(
			<FarmWorkspaceView
				area="workspace"
				status={status}
				guardStatus={guardStatus}
				bindingStatus={null}
				events={[]}
				tsdkEvents={[{
					id: 1,
					timestamp: '2026-08-02T10:00:00+08:00',
					level: 'info',
					source: 'qq_ws',
					type: 'qqhost.log',
					message: '[TSDK-BLOCK] workspace-ready',
				}]}
				onRefresh={noop}
				onLaunch={noop}
				onRestart={noop}
				onToggleGuard={noop}
			/>,
		);

		expect(html).toContain('TSDK 已拦截');
	});

	it('uses compact Chinese units and preserves the full sale estimate in a tooltip', () => {
		const html = renderToStaticMarkup(
			<FarmWorkspaceView
				area="workspace"
				status={status}
				guardStatus={guardStatus}
				bindingStatus={null}
				events={[]}
				runStatistics={{ ...runStatistics, saleEstimate: 1_234_567 }}
				onRefresh={noop}
				onLaunch={noop}
				onRestart={noop}
				onToggleGuard={noop}
			/>,
		);

		expect(html).toContain('123.46万');
		expect(html).toContain('title="1,234,567"');
	});

	it('uses Yi for hundred-million sale estimates and trims trailing decimals', () => {
		const yiHtml = renderToStaticMarkup(
			<FarmWorkspaceView
				area="workspace"
				status={status}
				guardStatus={guardStatus}
				bindingStatus={null}
				events={[]}
				runStatistics={{ ...runStatistics, saleEstimate: 100_000_000 }}
				onRefresh={noop}
				onLaunch={noop}
				onRestart={noop}
				onToggleGuard={noop}
			/>,
		);
		const wanHtml = renderToStaticMarkup(
			<FarmWorkspaceView
				area="workspace"
				status={status}
				guardStatus={guardStatus}
				bindingStatus={null}
				events={[]}
				runStatistics={{ ...runStatistics, saleEstimate: 12_000 }}
				onRefresh={noop}
				onLaunch={noop}
				onRestart={noop}
				onToggleGuard={noop}
			/>,
		);

		expect(yiHtml).toContain('1亿');
		expect(wanHtml).toContain('1.2万');
	});

	it('renders the automation control surface feature groups', () => {
    const html = renderToStaticMarkup(
      <FarmWorkspaceView
        area="automation"
        status={status}
        guardStatus={guardStatus}
        bindingStatus={null}
        events={[]}
        onRefresh={noop}
        onLaunch={noop}
        onRestart={noop}
        onToggleGuard={noop}
      />,
    );

    expect(html).toContain('农场自动化');
    expect(html).toContain('基础任务');
    expect(html).toContain('自动种植');
    expect(html).toContain('自动施肥');
    expect(html).toContain('自动施肥、催熟联动、自动购买。');
    expect(html).toContain('好友互动');
    expect(html).toContain('调度中心');
    expect(html).not.toContain('任务全局运行设置');
    expect(html).not.toContain('自动施肥、填充肥料、催熟联动、自动购买。');
    expect(html).not.toContain('自动填充肥料');
    expect(html).not.toContain('自家基础任务');
    expect(html).not.toContain('自动种植策略');
    expect(html).not.toContain('肥料与催熟');
    expect(html).not.toContain('好友自动化');
    expect(html).not.toContain('消息推送');
    expect(html).not.toContain('已完成');
  });

  it('renders automation state passed from the app instead of the local fallback', () => {
    const html = renderToStaticMarkup(
      <FarmWorkspaceView
        area="automation"
        status={status}
        guardStatus={guardStatus}
        bindingStatus={null}
        events={[]}
        automationState={customAutomationState}
        onRunAutomationTask={noop}
        onRefresh={noop}
        onLaunch={noop}
        onRestart={noop}
        onToggleGuard={noop}
      />,
    );

    expect(html).toContain('Wails状态探针');
    expect(html).toContain('外部调度任务');
    expect(html).not.toContain('基础任务');
  });

  it('passes the social guard dog count into automation settings', async () => {
    const renderer = TestRenderer.create(
      <FarmWorkspaceView
        area="automation"
        status={status}
        guardStatus={guardStatus}
        bindingStatus={null}
        events={[]}
        automationState={customAutomationState}
        socialState={{
          ok: true,
          status: 'ok',
          message: '',
          summary: { totalFriends: 1, stealableFriends: 0, helpableFriends: 0, mischiefFriends: 0, blacklisted: 0, whitelisted: 0, maskedBlocked: 0, protected: 0, dogGuardCount: 3 },
          rules: { whitelistEnabled: false, whitelistScopes: [], whitelist: [], blacklistEnabled: false, blacklistScopes: [], blacklist: [], maskedBlacklist: false, maskedMaxLevel: 1 },
          friends: [],
        }}
        onRunAutomationTask={noop}
        onRefresh={noop}
        onLaunch={noop}
        onRestart={noop}
        onToggleGuard={noop}
      />,
    );

    await act(async () => {
      renderer.root.findByProps({ 'aria-label': '好友互动设置' }).props.onClick();
    });

    expect(renderer.root.findByProps({ name: 'config-autoFarmFriendHelpGuardDogOnly' }).props.disabled).toBe(false);
  });

  it('passes account-scoped mystery shop purchase records into the settings panel', async () => {
    const renderer = TestRenderer.create(
      <FarmWorkspaceView
        area="automation"
        status={status}
        guardStatus={guardStatus}
        bindingStatus={null}
        events={[]}
        mysteryShopPurchaseRecords={[
          {
            id: 'purchase-1',
            occurredAt: '2026-07-26T19:42:00Z',
            itemName: '高级化肥',
            count: 2,
            unitPrice: 80,
            currencyId: 1005,
            currencyName: '金豆豆',
            discount: 50,
          },
        ]}
        onRefresh={noop}
        onLaunch={noop}
        onRestart={noop}
        onToggleGuard={noop}
      />,
    );

    await act(async () => {
      renderer.root.findByProps({ 'aria-label': '神秘商店自动购买设置' }).props.onClick();
    });

    expect(renderer.root.findAllByType('strong').some((node) => node.children.join('') === '高级化肥 x2')).toBe(true);
  });

  it('does not embed account, guard, logs, or settings as a merged system area', () => {
    const html = renderToStaticMarkup(
      <FarmWorkspaceView
        area="social"
        status={status}
        guardStatus={guardStatus}
        bindingStatus={null}
        events={[]}
        onRefresh={noop}
        onLaunch={noop}
        onRestart={noop}
        onToggleGuard={noop}
      />,
    );

    expect(html).not.toContain('账户状态');
    expect(html).not.toContain('守护服务');
    expect(html).not.toContain('日志中心');
    expect(html).not.toContain('系统设置');
  });

  it('does not pass the desktop file-save action into the remote social workspace', () => {
    const saveTextFile = async () => ({ path: 'C:/desktop/farm.txt' });
    const renderer = TestRenderer.create(
      <FarmWorkspaceView
        area="social"
        remote
        status={status}
        guardStatus={guardStatus}
        bindingStatus={null}
        events={[]}
        onRefresh={noop}
        onLaunch={noop}
        onRestart={noop}
        onToggleGuard={noop}
        onSaveTextFile={saveTextFile}
      />,
    );

    expect(renderer.root.findByType(SocialView).props.onSaveTextFile).toBeUndefined();
    renderer.unmount();
  });

  it('passes paged ranking dependencies and visitor refresh independently to SocialView', () => {
    const onSocialRankings = async () => ({
      ok: true as const,
      status: 'ok' as const,
      message: '',
      tab: 'stolenByMe' as const,
      viewMode: 'timeline' as const,
      dateRange: 'current' as const,
      summary: { visitorCount: 0, stolenFromMeCount: 0, stolenByMeCount: 0, stolenByMeRecordCount: 0 },
      rows: [],
      hasMore: false,
    });
    const onRefreshSocialVisitors = async () => ({ ok: true, status: 'ok', message: '' });
    const rankingPreferences = { stolenByMeViewMode: 'ranking' as const, stolenFromMeViewMode: 'timeline' as const };
    const rankingDataVersions = { stolenByMe: 4, visitors: 7 };
    const renderer = TestRenderer.create(
      <FarmWorkspaceView
        area="social"
        status={status}
        guardStatus={guardStatus}
        events={[]}
        socialRankingPreferences={rankingPreferences}
        socialRankingDataVersions={rankingDataVersions}
        onSocialRankings={onSocialRankings}
        onRefreshSocialVisitors={onRefreshSocialVisitors}
        onRefresh={noop}
        onLaunch={noop}
        onRestart={noop}
        onToggleGuard={noop}
      />,
    );

    const social = renderer.root.findByType(SocialView);
    expect(social.props.rankingPreferences).toBe(rankingPreferences);
    expect(social.props.rankingDataVersions).toBe(rankingDataVersions);
    expect(social.props.onRankings).toBe(onSocialRankings);
    expect(social.props.onRefreshVisitors).toBe(onRefreshSocialVisitors);
  });
});
