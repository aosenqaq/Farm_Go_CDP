import { renderToStaticMarkup } from 'react-dom/server';
import { act, create } from 'react-test-renderer';
// @ts-expect-error The frontend tsconfig intentionally omits Node types; this test reads component styles only.
import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

import { OverviewView, type RuntimeStatusDto } from './OverviewView';
import type { LicenseStatus } from '../lib/license';

const readyStatus: RuntimeStatusDto = {
  target: 'qq_ws',
  phase: 'ready',
  connected: true,
  ready: true,
  instanceId: 'farm-runtime-1',
  hostVersion: '1.0.0',
  lastSeenAt: '2026-07-06 23:19:20',
};

const authorizedLicense: LicenseStatus = {
  phase: 'authorized',
  authorized: true,
  generation: 1,
  heartbeatFailures: 0,
  expireTime: '2026-12-31 23:59:59',
  message: '',
  errorCode: '',
};

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

describe('OverviewView', () => {
  it('renders live run duration from startedAt instead of stale durationSeconds', () => {
    const startedAt = new Date(Date.now() - 65_000).toISOString();
    const html = renderToStaticMarkup(
      <OverviewView
        status={readyStatus}
        licenseStatus={authorizedLicense}
        events={[]}
        onRefresh={() => undefined}
        runStatistics={{
          startedAt,
          durationSeconds: 1,
          collect: 0,
          farm: 0,
          steal: 0,
          help: 0,
          mischief: 0,
          saleEstimate: 0,
          estimateReady: true,
        }}
      />,
    );

    expect(html).toContain('00:01:05');
    expect(html).not.toContain('00:00:01');
  });

  it('renders the six run statistics as dedicated flat icon rows', () => {
    const html = renderToStaticMarkup(
      <OverviewView
        status={readyStatus}
        licenseStatus={authorizedLicense}
        events={[]}
        onRefresh={() => undefined}
        runStatistics={{
          durationSeconds: 61,
          collect: 47,
          farm: 93,
          steal: 5,
          help: 8,
          mischief: 2,
          saleEstimate: 78_255,
          estimateReady: true,
        }}
      />,
    );

    expect(html.match(/class="workbench-statistic"/g)).toHaveLength(6);
    expect(html).toContain('lucide-sprout');
    expect(html).toContain('lucide-tractor');
    expect(html).toContain('lucide-shopping-basket');
    expect(html).toContain('lucide-hand-heart');
    expect(html).toContain('lucide-bomb');
    expect(html).toContain('lucide-coins');
    expect(html).toContain('>47</strong>');
    expect(html).toContain('title="78,255">7.83万</strong>');
    expect(html).not.toContain('workbench-run-statistics-grid"><article class="metric-card"');
  });

  it('uses responsive semantic workbench layout rules', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
    const expiryRule = ruleDeclarations(css, '.workbench-signal-expiry');
    const narrowStyles = mediaDeclarations(css, 760);
    const narrowAppShell = ruleDeclarations(narrowStyles, '.app-shell-remote {');
    const narrowMainContent = ruleDeclarations(narrowStyles, '.app-shell-remote .main-content');
    const narrowEvent = ruleDeclarations(narrowStyles, '.app-shell-remote .workbench-event {');
    const narrowSignals = ruleDeclarations(narrowStyles, '.app-shell-remote .workbench-signals');
    const narrowEventLines = ruleDeclarations(narrowStyles, '.app-shell-remote .workbench-event-lines');
    const narrowRouteFacts = ruleDeclarations(narrowStyles, '.app-shell-remote .workbench-route-facts');
    const narrowEventTitle = ruleDeclarations(narrowStyles, '.app-shell-remote .workbench-event strong');
    const narrowEventResult = ruleDeclarations(narrowStyles, '.app-shell-remote .workbench-event span');
    const narrowRenewalActions = ruleDeclarations(narrowStyles, '.app-shell-remote .license-renewal-actions');
    const narrowNoticeActions = ruleDeclarations(narrowStyles, '.app-shell-remote .program-notice-actions');

    expect(expiryRule).toMatch(/overflow-wrap:\s*anywhere;/);
    expect(expiryRule).toMatch(/white-space:\s*normal;/);
    expect(ruleDeclarations(css, '.workbench-signals')).toMatch(/flex-wrap:\s*wrap;/);
    expect(ruleDeclarations(css, '.workbench-conclusion-pending')).toMatch(/color:\s*#684a00;/);
    expect(ruleDeclarations(css, '.workbench-conclusion-ready')).toMatch(/color:\s*#338a35;/);
    expect(ruleDeclarations(narrowStyles, 'body:has(.app-shell-remote)')).toMatch(/min-width:\s*0;/);
    expect(narrowAppShell).toMatch(/grid-template-columns:\s*minmax\(0,\s*1fr\);/);
    expect(ruleDeclarations(narrowStyles, '.app-shell-remote .sidebar')).toMatch(/position:\s*fixed;/);
    expect(ruleDeclarations(narrowStyles, '.app-shell-remote .sidebar-nav')).toMatch(/grid-template-columns:\s*repeat\(5,\s*minmax\(68px,\s*1fr\)\);/);
    expect(ruleDeclarations(narrowStyles, '.app-shell-remote .main-surface')).toMatch(/padding-bottom:\s*84px;/);
    expect(narrowMainContent).toMatch(/overflow-y:\s*auto;/);
    expect(narrowMainContent).toMatch(/overscroll-behavior:\s*contain;/);
    expect(ruleDeclarations(narrowStyles, '.app-shell {')).toBe('');
    expect(ruleDeclarations(css, '.header-action-label-mobile')).toMatch(/display:\s*none;/);
    expect(ruleDeclarations(narrowStyles, '.app-shell-remote .header-action-label-desktop')).toMatch(/display:\s*none;/);
    expect(ruleDeclarations(narrowStyles, '.app-shell-remote .header-action-label-mobile')).toMatch(/display:\s*inline;/);
    expect(narrowSignals).toMatch(/grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/);
    expect(narrowRouteFacts).toMatch(/display:\s*none;/);
    expect(narrowEventLines).toMatch(/height:\s*clamp\(118px,\s*18dvh,\s*148px\);/);
    expect(narrowEventLines).toMatch(/overflow-y:\s*auto;/);
    expect(narrowEvent).toMatch(/grid-template-columns:\s*52px\s+minmax\(0,\s*1fr\);/);
    expect(narrowEvent).toMatch(/grid-template-rows:\s*auto\s+auto;/);
    expect(narrowEvent).toMatch(/min-height:\s*56px;/);
    expect(narrowEvent).toMatch(/padding:\s*10px\s+0;/);
    expect(ruleDeclarations(narrowStyles, '.app-shell-remote .workbench-event time')).toMatch(/grid-row:\s*span\s+2;/);
    expect(narrowEventTitle).toMatch(/font-size:\s*12px;/);
    expect(narrowEventResult).toMatch(/-webkit-line-clamp:\s*2;/);
    expect(narrowRenewalActions).toMatch(/display:\s*grid;/);
    expect(narrowRenewalActions).toMatch(/grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/);
    expect(narrowNoticeActions).toMatch(/display:\s*grid;/);
    expect(ruleDeclarations(narrowStyles, '.app-shell-remote .license-renewal-actions .primary-button')).toMatch(/grid-column:\s*2;/);
    expect(ruleDeclarations(narrowStyles, '.app-shell-remote .program-notice-actions .primary-button')).toMatch(/grid-column:\s*2;/);
  });

  it('keeps remote data tables scrollable and reserves full-screen for generic phone dialogs', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
    const narrowStyles = mediaDeclarations(css, 760);

    expect(ruleDeclarations(narrowStyles, '.app-shell-remote .asset-table-wrap')).toMatch(/overflow-x:\s*auto;/);
    expect(ruleDeclarations(narrowStyles, '.app-shell-remote .social-friend-table')).toMatch(/overflow-x:\s*auto;/);
    expect(ruleDeclarations(narrowStyles, '.app-shell-remote .dialog-backdrop > :is(section, aside):not(.license-renewal-dialog):not(.program-notice-dialog)')).toMatch(/height:\s*100dvh;/);
    expect(ruleDeclarations(narrowStyles, '.app-shell-remote .land-rush-drawer {')).toMatch(/border-radius:\s*0;/);
    expect(ruleDeclarations(narrowStyles, '.app-shell-remote .license-renewal-dialog')).toMatch(/align-self:\s*end;/);
    expect(ruleDeclarations(narrowStyles, '.app-shell-remote .license-renewal-dialog {')).toMatch(/max-height:\s*56dvh;/);
    expect(narrowStyles).toMatch(/\.app-shell-remote \.program-notice-dialog \{\s*display:\s*grid;\s*grid-template-rows:\s*auto\s+minmax\(0,\s*1fr\)\s+auto;\s*max-height:\s*72dvh;/);
    expect(ruleDeclarations(narrowStyles, '.app-shell-remote .program-notice-body')).toMatch(/overflow:\s*auto;/);
  });

  it('keeps flat statistics in four-row flow across desktop and remote mobile layouts', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
    const narrowStyles = mediaDeclarations(css, 760);
    const overviewLayout = ruleDeclarations(css, '.overview-view');
    const statisticGrid = ruleDeclarations(css, '.workbench-run-statistics-grid');
    const statistic = ruleDeclarations(css, '.workbench-statistic');
    const statisticValue = ruleDeclarations(css, '.workbench-statistic-value {');
    const narrowOverview = ruleDeclarations(narrowStyles, '.app-shell-remote .overview-view');
    const narrowGrid = ruleDeclarations(narrowStyles, '.app-shell-remote .workbench-run-statistics-grid');

    expect(overviewLayout).toMatch(/grid-template-rows:\s*auto\s+auto\s+minmax\(0,\s*1fr\)\s+auto;/);
    expect(statisticGrid).toMatch(/grid-template-columns:\s*repeat\(3,\s*minmax\(0,\s*1fr\)\);/);
    expect(statistic).toMatch(/grid-template-columns:\s*auto\s+minmax\(0,\s*1fr\)\s+auto;/);
    expect(statistic).toMatch(/box-shadow:\s*none;/);
    expect(statistic).toMatch(/border:\s*0;/);
    expect(statisticValue).toMatch(/text-align:\s*right;/);
    expect(narrowOverview).toMatch(/grid-template-rows:\s*repeat\(4,\s*auto\);/);
    expect(narrowOverview).toMatch(/height:\s*auto;/);
    expect(narrowGrid).toMatch(/grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/);
  });




  it('keeps the renewal dialog usable on narrow screens', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
    const mobileStyles = mediaDeclarations(css, 560);

    expect(ruleDeclarations(css, '.license-renewal-dialog')).toMatch(/width:\s*min\(460px,\s*calc\(100vw\s*-\s*40px\)\)/);
    expect(ruleDeclarations(mobileStyles, '.license-renewal-actions')).toMatch(/flex-direction:\s*column-reverse/);
  });

  it('renders one readiness hero with inline service signals', () => {
    const html = renderToStaticMarkup(
      <OverviewView
        status={readyStatus}
        guardStatus={{ phase: 'watching', restartCountInWindow: 0, maxRestartsPerWindow: 4, recentRestartEvents: [] }}
        licenseStatus={authorizedLicense}
        events={[]}
        onRefresh={() => undefined}
      />,
    );

    expect(html).toContain('QQ WS 已就绪');
    expect(html).toContain('可安全执行自动化');
    expect(html).toContain('workbench-conclusion-ready');
    expect(html).toContain('网关 正常');
    expect(html).toContain('监控中');
    expect(html).toContain('当前链路');
    expect(html).not.toContain('overview-status-card');
  });

  it('renders a pending conclusion when the runtime is not ready', () => {
    const html = renderToStaticMarkup(
      <OverviewView
        status={{ ...readyStatus, target: 'wechat_cdp', phase: 'listening', connected: false, ready: false }}
        events={[]}
        onRefresh={() => undefined}
      />,
    );

    expect(html).toContain('微信 CDP 监听中');
    expect(html).toContain('正在等待运行时完成连接');
    expect(html).toContain('workbench-conclusion-pending');
    expect(html).not.toContain('workbench-conclusion-ready');
  });

  it('renders the empty-event fallback source in Chinese', () => {
    const html = renderToStaticMarkup(
      <OverviewView
        status={readyStatus}
        events={[]}
        onRefresh={() => undefined}
      />,
    );

    expect(html).toContain('<strong>任务日志</strong>');
    expect(html).toContain('暂无任务运行结果');
  });

  it('uses only the dedicated TSDK feed for its signal and dialog', () => {
    const renderer = create(
      <OverviewView
        status={{ ...readyStatus, phase: 'listening', connected: false, ready: false }}
        events={[{
          id: 1,
          timestamp: '2026-08-02T10:00:00+08:00',
          level: 'error',
          source: 'qq_ws',
          type: 'qqhost.log',
          message: '[TSDK-BLOCK] ordinary-feed-failure',
          data: { status: 'init_err' },
        }]}
        tsdkEvents={[{
          id: 2,
          timestamp: '2026-08-02T10:00:01+08:00',
          level: 'info',
          source: 'qq_ws',
          type: 'qqhost.log',
          message: '[TSDK-BLOCK] dedicated-feed-ready',
        }]}
        onRefresh={() => undefined}
      />,
    );

    const tsdkButton = renderer.root.findByProps({ title: '点击查看 TSDK 拦截详情' });
    expect(tsdkButton.children.join('')).toContain('TSDK 已拦截');
    act(() => tsdkButton.props.onClick());
    const rendered = JSON.stringify(renderer.toJSON());
    expect(rendered).toContain('[TSDK-BLOCK] dedicated-feed-ready');
    expect(rendered).not.toContain('[TSDK-BLOCK] ordinary-feed-failure');
  });

  it('shows recent automation task results instead of system logs', () => {
    const html = renderToStaticMarkup(
      <OverviewView
        status={readyStatus}
        events={[
          {
            id: 1,
            timestamp: '2026-07-13T12:06:06+08:00',
            level: 'info',
            source: 'auto_farm',
            type: 'task.start',
            message: 'automation task started',
            data: { taskId: 'own_plant' },
          },
          {
            id: 2,
            timestamp: '2026-07-13T12:06:06+08:00',
            level: 'info',
            source: 'auto_farm',
            type: 'task.done',
            message: '没有检测到可种植空地，本轮自动种植跳过。',
            data: { taskId: 'own_plant', ok: true, status: 'ok' },
          },
          {
            id: 3,
            timestamp: '2026-07-13T12:06:05+08:00',
            level: 'info',
            source: 'auto_farm',
            type: 'task.done',
            message: '仓库中没有可出售的指定分类物品。',
            data: { taskId: 'auto_warehouse_sell', ok: true, status: 'ok' },
          },
          {
            id: 4,
            timestamp: '2026-07-13T12:06:04+08:00',
            level: 'info',
            source: 'settings',
            type: 'settings.save',
            message: '设置已保存',
          },
        ]}
        onRefresh={() => undefined}
      />,
    );

    expect(html).toContain('<strong>自动种植</strong>');
    expect(html).toContain('没有检测到可种植空地，本轮自动种植跳过。');
    expect(html).toContain('<strong>仓库自动出售</strong>');
    expect(html).toContain('仓库中没有可出售的指定分类物品。');
    expect(html).not.toContain('automation task started');
    expect(html).not.toContain('<strong>auto_farm</strong>');
    expect(html).not.toContain('设置已保存');
  });





  it('uses connected fallbacks for ready CDP detail fields', () => {
    const html = renderToStaticMarkup(
      <OverviewView
        status={{ target: 'wechat_cdp', phase: 'ready', connected: true, ready: true }}
        guardStatus={{ phase: 'watching', restartCountInWindow: 0, maxRestartsPerWindow: 4, recentRestartEvents: [] }}
        events={[]}
        onRefresh={() => undefined}
      />,
    );

    expect(html).toContain('微信 CDP');
    expect(html).toContain('当前上下文');
    expect(html).toContain('CDP 已接入');
    expect(html).not.toContain('未连接');
  });

  it('shows disabled process guard state in the overview signals', () => {
    const html = renderToStaticMarkup(
      <OverviewView
        status={readyStatus}
        guardStatus={{
          enabled: false,
          phase: 'disabled',
          restartCountInWindow: 0,
          maxRestartsPerWindow: 4,
          recentRestartEvents: [],
        }}
        events={[]}
        onRefresh={() => undefined}
      />,
    );

    expect(html).toContain('守护');
    expect(html).toContain('未启用');
  });
});
