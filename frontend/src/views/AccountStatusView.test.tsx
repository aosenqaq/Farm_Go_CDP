import { renderToStaticMarkup } from 'react-dom/server';
import { act, create } from 'react-test-renderer';
// @ts-expect-error The frontend tsconfig intentionally omits Node types; this test reads component styles only.
import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

import { AccountStatusView, buildStatsWindow, formatSaleEstimate } from './AccountStatusView';

function blockAt(css: string, start: number) {
  const openBrace = css.indexOf('{', start);
  if (openBrace < 0) return '';

  let depth = 1;
  for (let index = openBrace + 1; index < css.length; index += 1) {
    if (css[index] === '{') depth += 1;
    if (css[index] === '}') depth -= 1;
    if (depth === 0) return css.slice(openBrace + 1, index);
  }
  return '';
}

function ruleDeclarations(css: string, selector: string) {
  const selectorIndex = css.indexOf(selector);
  return selectorIndex < 0 ? '' : blockAt(css, selectorIndex);
}

function mediaDeclarations(css: string, maxWidth: number) {
  const mediaMatch = /@media\s*\(\s*max-width:\s*(\d+)px\s*\)/g;
  let match: RegExpExecArray | null;
  let declarations = '';

  while ((match = mediaMatch.exec(css))) {
    if (Number(match[1]) === maxWidth) declarations += blockAt(css, match.index);
  }
  return declarations;
}

function accountProfileView(avatarUrl?: string) {
  return (
    <AccountStatusView
      initialProfile={{
        gid: 123456789,
        name: 'Dpo.L',
        level: 100,
        avatarUrl,
        levelProgress: { current: 161475, needed: 401000, remaining: 239525, percent: 40, nextLevel: 101 },
      }}
      initialFertilizer={{}}
      initialWarehouse={{ status: 'runtime', items: [] }}
    />
  );
}

describe('AccountStatusView', () => {
  it('formats sale estimates with automatic Chinese units', () => {
    expect(formatSaleEstimate(999_999)).toBe('999,999');
    expect(formatSaleEstimate(1_000_000)).toBe('1百万');
    expect(formatSaleEstimate(6_780_000)).toBe('6.78百万');
    expect(formatSaleEstimate(10_000_000)).toBe('1千万');
    expect(formatSaleEstimate(67_084_933)).toBe('6.71千万');
    expect(formatSaleEstimate(100_000_000)).toBe('1亿');
    expect(formatSaleEstimate(125_000_000)).toBe('1.25亿');
  });

  it('uses compact flat statistics across desktop and mobile dialog widths', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
    const mobileStyles = mediaDeclarations(css, 640);
    const grid = ruleDeclarations(css, '.account-stats-grid');
    const statistic = ruleDeclarations(css, '.account-statistic');
    const value = ruleDeclarations(css, '.account-statistic-value');
    const mobileGrid = ruleDeclarations(mobileStyles, '.account-stats-grid');

    expect(grid).toMatch(/grid-template-columns:\s*repeat\(4,\s*minmax\(0,\s*1fr\)\);/);
    expect(statistic).toMatch(/grid-template-columns:\s*auto\s+minmax\(0,\s*1fr\)\s+auto;/);
    expect(statistic).toMatch(/min-height:\s*48px;/);
    expect(statistic).toMatch(/border:\s*0;/);
    expect(statistic).toMatch(/box-shadow:\s*none;/);
    expect(value).toMatch(/text-align:\s*right;/);
    expect(mobileGrid).toMatch(/grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/);
  });

  it('uses balanced account identity sizing on desktop and LAN mobile', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
    const mobileStyles = mediaDeclarations(css, 760);
    const identityBar = ruleDeclarations(css, '.account-identity-bar');
    const desktopAvatar = ruleDeclarations(css, '.account-status-identity-avatar');
    const identityCopy = ruleDeclarations(css, '.account-status-identity-copy');
    const identityName = ruleDeclarations(css, '.account-identity-name,');
    const identityNameWidth = ruleDeclarations(css, '.account-identity-name {');
    const mobileIdentityBar = ruleDeclarations(mobileStyles, '.app-shell-remote .account-identity-bar');
    const mobileAvatar = ruleDeclarations(mobileStyles, '.app-shell-remote .account-status-identity-avatar');
    const mobileMetrics = ruleDeclarations(mobileStyles, '.app-shell-remote .account-metric-grid');

    expect(identityBar).toMatch(/grid-template-columns:\s*52px\s+minmax\(0,\s*1fr\)\s+auto;/);
    expect(identityBar).toMatch(/box-shadow:\s*none;/);
    expect(desktopAvatar).toMatch(/width:\s*52px;/);
    expect(desktopAvatar).toMatch(/height:\s*52px;/);
    expect(identityCopy).toMatch(/justify-items:\s*start;/);
    expect(identityCopy).toMatch(/margin:\s*0;/);
    expect(identityCopy).toMatch(/text-align:\s*left;/);
    expect(identityCopy).toMatch(/width:\s*100%;/);
    expect(identityName).toMatch(/overflow:\s*hidden;/);
    expect(identityName).toMatch(/text-overflow:\s*ellipsis;/);
    expect(identityName).toMatch(/white-space:\s*nowrap;/);
    expect(identityNameWidth).toMatch(/width:\s*100%;/);
    expect(mobileIdentityBar).toMatch(/grid-template-columns:\s*44px\s+minmax\(0,\s*1fr\)\s+auto;/);
    expect(mobileAvatar).toMatch(/width:\s*44px;/);
    expect(mobileAvatar).toMatch(/height:\s*44px;/);
    expect(mobileMetrics).toMatch(/grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/);
  });

  it('aggregates sale estimates for the selected date range', () => {
    const profile = {
      todayStats: { dateKey: '2026-07-23', saleEstimate: 500, estimateReady: true },
      statsHistory: {
        todayKey: '2026-07-23',
        days: [
          { dateKey: '2026-07-21', saleEstimate: 100, estimateReady: true },
          { dateKey: '2026-07-22', saleEstimate: 300, estimateReady: true },
          { dateKey: '2026-07-23', saleEstimate: 500, estimateReady: true },
        ],
      },
    };

    expect(buildStatsWindow(profile, 'today').stats.saleEstimate).toBe(500);
    const threeDays = buildStatsWindow(profile, '3d');
    expect(threeDays.stats.saleEstimate).toBe(900);
    expect(threeDays.stats.estimateReady).toBe(true);

    profile.statsHistory.days[1].estimateReady = false;
    expect(buildStatsWindow(profile, '3d').stats.estimateReady).toBe(false);
  });

  it('waits for runtime readiness before loading account data', () => {
    const html = renderToStaticMarkup(
      <AccountStatusView
        status={{
          target: 'wechat_cdp',
          phase: 'handshaking',
          connected: true,
          ready: false,
        }}
      />,
    );

    expect(html).toContain('等待游戏运行时就绪');
    expect(html).not.toContain('Runtime.evaluate failed');
  });

  it('renders the runtime avatar and separated account identity', () => {
    const html = renderToStaticMarkup(accountProfileView('https://example.test/runtime-avatar.png'));

    expect(html).toContain('class="account-identity-bar"');
    expect(html).toContain('class="account-status-identity-avatar"');
    expect(html).not.toContain('class="account-identity-avatar"');
    expect(html).toContain('class="account-status-identity-copy"');
    expect(html).not.toContain('class="account-identity-copy"');
    expect(html).toContain('src="https://example.test/runtime-avatar.png"');
    expect(html).toContain('alt="Dpo.L"');
    expect(html).toContain('GID 123456789');
    expect(html).not.toContain('GID 123,456,789');
    expect(html).toContain('Lv. 100');
    expect(html).toContain('经验升级进度');
  });

  it('shows the user icon when the runtime profile has no avatar', () => {
    let tree: ReturnType<typeof create>;
    act(() => {
      tree = create(accountProfileView());
    });

    const avatar = tree!.root.findByProps({ className: 'account-status-identity-avatar' });
    expect(avatar.findAllByType('img')).toHaveLength(0);
    expect(avatar.findAll((node) => typeof node.props.className === 'string' && node.props.className.includes('lucide-user-round')).length).toBeGreaterThan(0);
  });

  it('retries with a refreshed avatar URL after the previous image failed', async () => {
    const runtimeGlobal = globalThis as { window?: unknown };
    const previousWindow = runtimeGlobal.window;
    let tree: ReturnType<typeof create>;
    try {
      runtimeGlobal.window = {
        go: {
          main: {
            App: {
              FarmAccountStatus: () => Promise.resolve({
                profile: {
                  gid: 123456789,
                  name: 'Dpo.L',
                  level: 100,
                  avatarUrl: 'https://example.test/refreshed-avatar.png',
                  levelProgress: { current: 161475, needed: 401000, remaining: 239525, percent: 40, nextLevel: 101 },
                },
                fertilizer: {},
              }),
              FarmWarehouse: () => Promise.resolve({ status: 'runtime', items: [] }),
            },
          },
        },
      };
      act(() => {
        tree = create(accountProfileView('https://example.test/first-avatar.png'));
      });

      const currentAvatar = () => tree!.root.findAllByType('img').find((node) => node.props.alt === 'Dpo.L')!;
      act(() => currentAvatar().props.onError());
      expect(currentAvatar().props.src).toBe('/logo.png');

      await act(async () => {
        tree!.root.findByProps({ className: 'secondary-action account-refresh-action' }).props.onClick();
        await Promise.resolve();
        await Promise.resolve();
      });
      expect(currentAvatar().props.src).toBe('https://example.test/refreshed-avatar.png');
    } finally {
      if (previousWindow === undefined) delete runtimeGlobal.window;
      else runtimeGlobal.window = previousWindow;
    }
  });

  it('renders runtime account profile, level progress, fertilizer container, and activity currency', () => {
    const html = renderToStaticMarkup(
      <AccountStatusView
        initialProfile={{
          name: 'Dpo.L',
          level: 118,
          gold: 3780797635,
          bean: 246703210,
          coupon: 10203040,
          diamond: 120000000,
          levelProgress: { current: 108855, needed: 823800, remaining: 714945, percent: 13, nextLevel: 119 },
          todayStats: {
            dateKey: '2026-07-07',
            updatedAt: '2026-07-07T10:00:00.000Z',
            runs: 8,
            collect: 10,
            water: 2,
            steal: 3,
            help: 1,
            mischiefGrass: 2,
            mischiefBug: 1,
            sell: 4,
          },
          statsHistory: { todayKey: '2026-07-07', days: [] },
        }}
        initialFertilizer={{
          normal: { available: false, remainingHours: 0 },
          organic: { available: true, remainingSec: 382320 },
        }}
        initialWarehouse={{
          status: 'runtime',
          items: [{ name: '星砂', count: 5225, categoryLabel: '道具' }],
        }}
      />,
    );

    expect(html).toContain('账户状态');
    expect(html).toContain('刷新账户');
    expect(html).toContain('Dpo.L');
    expect(html).toContain('经验升级进度');
    expect(html).toContain('108,855 / 823,800');
    expect(html).toContain('下一等级 Lv. 119');
    expect(html).toContain('还差 714,945');
    expect(html).toContain('37亿8079万');
    expect(html).toContain('金豆豆');
    expect(html).toContain('2亿4670万');
    expect(html).toContain('1020万');
    expect(html).toContain('1亿2000万');
    expect(html).toContain('肥料容器');
    expect(html).toContain('无机剩余');
    expect(html).toContain('当前不可用');
    expect(html).toContain('有机剩余');
    expect(html).toContain('106.2 小时');
    expect(html).toContain('活动货币');
    expect(html).toContain('星砂');
    expect(html).toContain('src="/items/starsand.png"');
    expect(html).toContain('alt="星砂"');
    expect(html).toContain('5,225');
    expect(html).not.toContain('荷露');
    expect(html).not.toContain('account-stats-section');
    expect(html).not.toContain('授权状态');
    expect(html).not.toContain('运行链路');
    expect(html).not.toContain('待迁移');
  });

  it('does not treat the previous activity currency as current', () => {
    const html = renderToStaticMarkup(
      <AccountStatusView
        initialProfile={{ name: 'Dpo.L', level: 118 }}
        initialFertilizer={{}}
        initialWarehouse={{
          status: 'runtime',
          items: [{ name: '荷露', count: 1, categoryLabel: '道具' }],
        }}
      />,
    );

    expect(html).toContain('活动货币');
    expect(html).toContain('暂未获取');
    expect(html).not.toContain('仓库实时数量');
  });

  it('opens and closes the retained statistics dialog', () => {
    let tree: ReturnType<typeof create>;
    act(() => {
      tree = create(
        <AccountStatusView
          initialProfile={{
            name: 'Dpo.L',
            level: 118,
            todayStats: { dateKey: '2026-07-07', runs: 8, collect: 10, water: 2, steal: 3, help: 1, mischiefGrass: 2, mischiefBug: 1, sell: 4 },
            statsHistory: { todayKey: '2026-07-07', days: [] },
          }}
          initialFertilizer={{}}
          initialWarehouse={{ status: 'runtime', items: [] }}
        />,
      );
    });

    const openButton = tree!.root.findAllByType('button').find((button) => button.props['aria-label'] === '打开运行统计');
    expect(openButton).toBeTruthy();
    act(() => openButton!.props.onClick());
    expect(tree!.root.findByProps({ role: 'dialog' }).props['aria-labelledby']).toBe('account-statistics-title');
    expect(tree!.root.findAllByType('button').some((button) => button.children.includes('近三天'))).toBe(true);

    act(() => tree!.root.findByProps({ 'aria-label': '关闭运行统计' }).props.onClick());
    expect(() => tree!.root.findByProps({ role: 'dialog' })).toThrow();
  });

  it('renders eight dedicated flat statistic rows inside the statistics dialog', () => {
    let tree: ReturnType<typeof create>;
    act(() => {
      tree = create(
        <AccountStatusView
          initialProfile={{
            name: 'Dpo.L',
            level: 118,
            todayStats: {
              dateKey: '2026-07-23',
              runs: 8,
              collect: 10,
              water: 2,
              steal: 3,
              help: 1,
              mischiefGrass: 2,
              mischiefBug: 1,
              sell: 4,
              saleEstimate: 67_084_933,
              estimateReady: true,
            },
            statsHistory: { todayKey: '2026-07-23', days: [] },
          }}
          initialFertilizer={{}}
          initialWarehouse={{ status: 'runtime', items: [] }}
        />,
      );
    });

    act(() => tree!.root.findByProps({ 'aria-label': '打开运行统计' }).props.onClick());

    expect(tree!.root.findAll((node) => node.props.className === 'account-statistic')).toHaveLength(8);
    const rendered = JSON.stringify(tree!.toJSON());
    for (const iconClass of ['lucide-activity', 'lucide-sprout', 'lucide-tractor', 'lucide-shopping-basket', 'lucide-hand-heart', 'lucide-bomb', 'lucide-coins', 'lucide-chart-no-axes-combined']) {
      expect(rendered).toContain(iconClass);
    }
    expect(rendered).toContain('预估收益');
    expect(rendered).toContain('6.71千万');
    expect(rendered).toContain('67,084,933');
    expect(rendered).not.toContain('覆盖天数');
    expect(tree!.root.findAll((node) => node.props.className === 'metric-card')).toHaveLength(6);
  });
});
