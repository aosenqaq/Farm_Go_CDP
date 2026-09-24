
import { useState } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import TestRenderer, { act, type ReactTestInstance } from 'react-test-renderer';
// @ts-expect-error The frontend tsconfig intentionally omits Node types; this test reads source text only.
import { readFileSync } from 'node:fs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { RankingPageRequest } from '../lib/socialRankingPages';

import {
  SOCIAL_TOAST_AUTO_DISMISS_MS,
  SocialView,
  buildRuleExportFileRequest,
  buildRulesWithImportedList,
  formatRuleExportList,
  parseRuleImportText,
  sortFriendsByLevel,
  type DogGuardState,
  type SocialActionRequest,
  type SocialActionResult,
  type SocialState,
} from './SocialView';

const state: SocialState = {
  ok: true,
  status: 'ok',
  message: '好友列表已更新。',
  summary: {
    totalFriends: 2,
    stealableFriends: 1,
    helpableFriends: 1,
    mischiefFriends: 0,
    blacklisted: 1,
    whitelisted: 1,
    maskedBlocked: 1,
    protected: 0,
    dogGuardCount: 1,
  },
  rules: {
    whitelistEnabled: true,
    whitelistScopes: ['steal'],
    whitelist: ['10001'],
    blacklistEnabled: true,
    blacklistScopes: ['mischief'],
    blacklist: ['10002'],
    maskedBlacklist: true,
    maskedMaxLevel: 1,
  },
  friends: [
    {
      gid: 10001,
      displayName: 'A',
      level: 12,
      workCounts: { collect: 2 },
      stealable: true,
      helpable: false,
      mischiefable: false,
      whitelisted: true,
      blacklisted: false,
      maskedBlocked: false,
      protected: false,
      protocolBlocked: false,
      hasGuardDog: false,
    },
    {
      gid: 10002,
      displayName: 'B',
      level: 1,
      workCounts: { help: 1 },
      stealable: false,
      helpable: true,
      mischiefable: false,
      whitelisted: false,
      blacklisted: true,
      maskedBlocked: true,
      protected: false,
      protocolBlocked: false,
      hasGuardDog: true,
    },
  ],
};

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((onResolve, onReject) => {
    resolve = onResolve;
    reject = onReject;
  });
  return { promise, resolve, reject };
}

function textFromChildren(children: unknown): string {
  if (Array.isArray(children)) return children.map(textFromChildren).join('');
  return typeof children === 'string' || typeof children === 'number' ? String(children) : '';
}

function textFromNode(node: ReactTestInstance): string {
  return node.children.map((child) => (typeof child === 'string' || typeof child === 'number' ? String(child) : textFromNode(child))).join('');
}

function findButton(root: ReactTestInstance, label: string): ReactTestInstance {
  const button = root.findAllByType('button').find((candidate) => textFromNode(candidate).includes(label));
  if (!button) throw new Error(`missing button ${label}`);
  return button;
}

function clickRect() {
  return {
    currentTarget: {
      getBoundingClientRect: () => ({ top: 20, right: 300, bottom: 52, left: 220, width: 80, height: 32 }),
    },
  };
}

describe('SocialView', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('shows 查看好友QQ only for the QQ runtime', () => {
    const qqRenderer = TestRenderer.create(
      <SocialView state={state} isQQRuntime onRefresh={() => undefined} onAction={() => undefined} />,
    );
    act(() => findButton(qqRenderer.root, '操作').props.onClick(clickRect()));
    expect(findButton(qqRenderer.root, '查看好友QQ')).toBeTruthy();

    const nonQQRenderer = TestRenderer.create(
      <SocialView state={state} isQQRuntime={false} onRefresh={() => undefined} onAction={() => undefined} />,
    );
    act(() => findButton(nonQQRenderer.root, '操作').props.onClick(clickRect()));
    expect(JSON.stringify(nonQQRenderer.toJSON())).not.toContain('查看好友QQ');
  });

  it('submits 查看好友QQ once and keeps it disabled while pending', async () => {
    const pending = deferred<SocialActionResult>();
    const requests: SocialActionRequest[] = [];
    const renderer = TestRenderer.create(
      <SocialView
        state={state}
        isQQRuntime
        onRefresh={() => undefined}
        onAction={(request) => {
          requests.push(request);
          return pending.promise;
        }}
      />,
    );
    act(() => findButton(renderer.root, '操作').props.onClick(clickRect()));

    await act(async () => {
      void findButton(renderer.root, '查看好友QQ').props.onClick();
      await Promise.resolve();
    });

    expect(requests).toEqual([{ action: 'view_qq', target: '10001' }]);
    expect(findButton(renderer.root, '打开QQ中').props.disabled).toBe(true);

    await act(async () => {
      pending.resolve({ ok: true, status: 'ok', message: '已请求打开 QQ 原生好友对话框。' });
      await pending.promise;
    });
  });

  it('keeps default friend order and stably sorts numeric levels with unknown levels last', () => {
    const friends = [
      { ...state.friends[0], gid: 1, displayName: '等级 12', level: 12 },
      { ...state.friends[0], gid: 2, displayName: '未知等级', level: undefined },
      { ...state.friends[0], gid: 3, displayName: '等级 3-A', level: 3 },
      { ...state.friends[0], gid: 4, displayName: '等级 3-B', level: 3 },
    ];

    expect(sortFriendsByLevel(friends, 'default').map((friend) => friend.gid)).toEqual([1, 2, 3, 4]);
    expect(sortFriendsByLevel(friends, 'level_asc').map((friend) => friend.gid)).toEqual([3, 4, 1, 2]);
    expect(sortFriendsByLevel(friends, 'level_desc').map((friend) => friend.gid)).toEqual([1, 3, 4, 2]);
  });

  it('sorts the friend table from the compact level selector after filtering', () => {
    const sortedState: SocialState = {
      ...state,
      summary: { ...state.summary, totalFriends: 3 },
      friends: [
        { ...state.friends[0], gid: 1, displayName: '等级 12', level: 12, stealable: true },
        { ...state.friends[0], gid: 2, displayName: '等级 3', level: 3, stealable: true },
        { ...state.friends[1], gid: 3, displayName: '非可偷', level: 99, stealable: false },
      ],
    };
    const renderer = TestRenderer.create(<SocialView state={sortedState} onRefresh={() => undefined} onAction={() => undefined} />);
    const select = renderer.root.findByProps({ 'aria-label': '好友排序' });

    expect(JSON.stringify(renderer.toJSON())).toContain('等级升序');
    expect(JSON.stringify(renderer.toJSON())).toContain('等级倒序');
    act(() => select.props.onChange({ target: { value: 'level_asc' } }));
    act(() => findButton(renderer.root, '可偷').props.onClick());

    const names = renderer.root
      .findAllByProps({ className: 'social-friend-name' })
      .map((row) => row.findByType('strong').children.join(''));
    expect(names).toEqual(['等级 3', '等级 12']);
  });

  it('renders friend list as the primary content', () => {
    const html = renderToStaticMarkup(<SocialView state={state} onRefresh={() => undefined} onAction={() => undefined} />);

    expect(html).toContain('好友社交');
    expect(html).toContain('好友列表');
    expect(html).toContain('A');
    expect(html).toContain('B');
    expect(html).toContain('好友功能');
    expect(html).toContain('排行榜');
    expect(html).toContain('导入导出');
  });

  it('renders ranking dialog tabs when opened', () => {
    const html = renderToStaticMarkup(
      <SocialView state={state} onRefresh={() => undefined} onAction={() => undefined} initialRankingOpen />,
    );

    expect(html).toContain('偷取记录');
    expect(html).toContain('被偷记录');
    expect(html).toContain('访客记录');
    expect(html).not.toContain('原始数据');
  });

  it('opens the paged ranking dialog with one local query and no implicit visitor refresh', async () => {
    const onRankings = vi.fn(async (request: RankingPageRequest) => ({
      ok: true as const,
      status: 'ok' as const,
      message: '',
      tab: request.tab,
      viewMode: request.viewMode,
      dateRange: request.dateRange,
      summary: { visitorCount: 0, stolenFromMeCount: 0, stolenByMeCount: 0, stolenByMeRecordCount: 0 },
      rows: [],
      hasMore: false,
    }));
    const onRefreshVisitors = vi.fn(async () => ({ ok: true, status: 'ok', message: '' }));
    const renderer = TestRenderer.create(
      <SocialView
        state={state}
        rankingPreferences={{ stolenByMeViewMode: 'timeline', stolenFromMeViewMode: 'timeline' }}
        rankingDataVersions={{ stolenByMe: 0, visitors: 0 }}
        onRefresh={() => undefined}
        onAction={() => undefined}
        onRankings={onRankings}
        onRefreshVisitors={onRefreshVisitors}
      />,
    );

    expect(onRankings).not.toHaveBeenCalled();
    await act(async () => findButton(renderer.root, '排行榜').props.onClick());

    expect(onRankings).toHaveBeenCalledTimes(1);
    expect(onRankings).toHaveBeenCalledWith({
      tab: 'stolenByMe', viewMode: 'timeline', dateRange: 'current', cursor: '', limit: 50,
    });
    expect(onRefreshVisitors).not.toHaveBeenCalled();
  });

  it('renders action result toast and defines an auto-dismiss timeout', () => {

    const html = renderToStaticMarkup(
      <SocialView
        state={state}
        onRefresh={() => undefined}
        onAction={() => undefined}
        lastActionResult={{ ok: true, status: 'ok', message: '已提交好友 10001 的帮忙请求。' }}
      />,
    );

    expect(html).toContain('已提交好友 10001 的帮忙请求。');
    expect(SOCIAL_TOAST_AUTO_DISMISS_MS).toBe(3500);
  });

  it('groups each friend into operation and configuration menus', () => {
    const requests: SocialActionRequest[] = [];
    const renderer = TestRenderer.create(
      <SocialView state={state} onRefresh={() => undefined} onAction={(request) => { requests.push(request); }} />,
    );

    expect(findButton(renderer.root, '操作')).toBeDefined();
    expect(findButton(renderer.root, '配置')).toBeDefined();

    act(() => findButton(renderer.root, '操作').props.onClick(clickRect()));
    expect(JSON.stringify(renderer.toJSON())).toContain('查看');
    expect(JSON.stringify(renderer.toJSON())).toContain('偷菜');
    expect(JSON.stringify(renderer.toJSON())).toContain('捣乱');
    expect(JSON.stringify(renderer.toJSON())).toContain('帮助');
    expect(renderer.root.findByProps({ 'aria-label': '关闭 A 的操作菜单' })).toBeDefined();
    act(() => findButton(renderer.root, '查看').props.onClick());
    expect(requests).toEqual([{ action: 'enter', target: '10001' }]);
  });

  it('keeps mobile friend action labels above their visual backdrop', () => {
    const renderer = TestRenderer.create(
      <SocialView state={state} onRefresh={() => undefined} onAction={() => undefined} />,
    );

    for (const label of ['操作', '配置']) {
      const trigger = findButton(renderer.root, label);
      const foregroundLabel = trigger.findByProps({ className: 'social-command-label' });
      expect(textFromNode(foregroundLabel)).toBe(label);
    }
  });

  it('uses local-list configuration labels without removing the other local list', () => {
    const requests: SocialActionRequest[] = [];
    const renderer = TestRenderer.create(
      <SocialView state={state} onRefresh={() => undefined} onAction={(request) => { requests.push(request); }} />,
    );

    act(() => findButton(renderer.root, '配置').props.onClick(clickRect()));
    const content = JSON.stringify(renderer.toJSON());
    expect(content).toContain('加入本地黑名单');
    expect(content).toContain('移出本地白名单');
    expect(content).toContain('加入系统黑名单');
    act(() => findButton(renderer.root, '加入本地黑名单').props.onClick());
    expect(requests).toEqual([{ action: 'blacklist_toggle', target: '10001' }]);
  });

  it('confirms before submitting a system blacklist request', async () => {
    const requests: SocialActionRequest[] = [];
    const renderer = TestRenderer.create(
      <SocialView
        state={state}
        onRefresh={() => undefined}
        onAction={(request) => {
          requests.push(request);
          return { ok: true, status: 'ok', message: '已加入系统黑名单' };
        }}
      />,
    );

    act(() => findButton(renderer.root, '配置').props.onClick(clickRect()));
    act(() => findButton(renderer.root, '加入系统黑名单').props.onClick());
    expect(requests).toEqual([]);
    expect(JSON.stringify(renderer.toJSON())).toContain('确认加入系统黑名单');
    await act(async () => findButton(renderer.root, '确认加入').props.onClick());
    expect(requests).toEqual([{ action: 'block_friend', target: '10001' }]);
  });

  it('gives every social child sheet a named close control and optional fixed footer', () => {
    const source = readFileSync(new URL('./SocialView.tsx', import.meta.url), 'utf8');

    expect(source).toContain('footer?: ReactNode');
    expect(source).toContain('social-dialog-footer');
    expect(source).toContain('aria-label={`关闭${title}`}');
    expect(source).toContain('title="本地名单设置"');
    expect(source).toContain('title="系统黑名单"');
  });

  it('defines compact remote social rows and bounded child sheets', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(css).toContain('.app-shell-remote .social-header .header-actions');
    expect(css).toContain('.app-shell-remote .social-summary-strip');
    expect(css).toContain('.app-shell-remote .social-friend-row');
    expect(css).toContain('.app-shell-remote .social-command-trigger');
    expect(css).toContain('.app-shell-remote .social-command-menu');
    expect(css).toContain('.app-shell-remote .social-dialog');
    expect(css).toContain('.app-shell-remote .social-dialog-footer');
    expect(css).toContain('height: min(84dvh, 720px)');
    expect(css).toContain('min-height: 42px');
    expect(css).not.toContain('.social-icon-action');
    expect(readFileSync(new URL('./SocialView.tsx', import.meta.url), 'utf8')).not.toContain('social-icon-action');
  });

  it('keeps the desktop action column visible and gives the title its own toolbar row at tablet widths', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(css).not.toMatch(/\.social-friend-row > :nth-child\(4\)\s*\{\s*display: none;/);
    expect(css).toMatch(/@media \(max-width: 1120px\)\s*\{[\s\S]*?\.social-table-toolbar\s*\{\s*display: grid;/);
    expect(css).toContain('grid-template-columns: minmax(0, 1fr);');
  });

  it('keeps social sheets out of the generic full-screen phone dialog rule', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(css).toContain(':not(.social-dialog)');
  });

  it('groups friend filters into a phone-scrollable strip', () => {
    const source = readFileSync(new URL('./SocialView.tsx', import.meta.url), 'utf8');
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(source).toContain('social-filter-chips');
    expect(css).toContain('.app-shell-remote .social-filter-chips');
  });

  it('renders rule labels from saved rules even when row flags are stale', () => {
    const staleState: SocialState = {
      ...state,
      summary: { ...state.summary, totalFriends: 1, blacklisted: 0, whitelisted: 0, maskedBlocked: 0 },
      rules: { ...state.rules, blacklist: ['10001'], whitelist: ['A'] },
      friends: [{
        ...state.friends[0],
        blacklisted: false,
        whitelisted: false,
        maskedBlocked: false,
      }],
    };

    const html = renderToStaticMarkup(<SocialView state={staleState} onRefresh={() => undefined} onAction={() => undefined} />);

    expect(html).toContain('本地黑名单');
    expect(html).toContain('本地白名单');
    expect(html).not.toContain('普通</span>');
  });

  it('renders black and white rule labels in the status column', () => {
    const html = renderToStaticMarkup(<SocialView state={state} onRefresh={() => undefined} onAction={() => undefined} />);
    const rowAStatus = html.match(/<strong>A<\/strong>[\s\S]*?<div class="social-pill-line">([\s\S]*?)<\/div>/)?.[1] || '';
    const rowBStatus = html.match(/<strong>B<\/strong>[\s\S]*?<div class="social-pill-line">([\s\S]*?)<\/div>/)?.[1] || '';

    expect(rowAStatus).toContain('本地白名单');
    expect(rowBStatus).toContain('本地黑名单');
  });

  it('renders queried god-rank friends from the social action result', () => {
    const html = renderToStaticMarkup(
      <SocialView
        state={state}
        onRefresh={() => undefined}
        onAction={() => undefined}
        lastActionResult={{
          ok: true,
          status: 'ok',
          message: '封神榜已读取。',
          data: {
            action: 'get_god_rank_list',
            count: 2,
            list: [
              { gid: 10001, displayName: '封禁 A', displayRank: 1, level: 9, stealableCount: 3, isBanned: true },
              { gid: 90909, displayName: '普通 B', displayRank: 2, level: 8, stealableCount: 1, isBanned: false },
            ],
          },
        }}
      />,
    );

    expect(html).toContain('封神榜');
    expect(html).toContain('查询到的封禁好友');
    expect(html).toContain('封禁 A');
    expect(html).toContain('10001');
    expect(html).toContain('第 1');
    expect(html).toContain('1 人');
    expect(html).not.toContain('普通 B');
    expect(html).not.toContain('90909');
  });

  it('does not reopen god rank after the social page remounts', () => {
    function SocialPageBoundary() {
      const [mounted, setMounted] = useState(true);
      const [actionResult, setActionResult] = useState<SocialActionResult | null>({
        ok: true,
        status: 'ok',
        message: '封神榜已读取。',
        data: {
          action: 'get_god_rank_list',
          list: [{ gid: 10001, displayName: '封禁 A', isBanned: true }],
        },
      });

      return (
        <>
          <button type="button" aria-label="离开好友社交" onClick={() => setMounted(false)}>离开</button>
          <button type="button" aria-label="返回好友社交" onClick={() => setMounted(true)}>返回</button>
          {mounted && (
            <SocialView
              state={state}
              lastActionResult={actionResult}
              onActionResultConsumed={() => setActionResult(null)}
              onRefresh={() => undefined}
              onAction={() => undefined}
            />
          )}
        </>
      );
    }

    const renderer = TestRenderer.create(<SocialPageBoundary />);
    expect(JSON.stringify(renderer.toJSON())).toContain('封神榜');

    act(() => renderer.root.findByProps({ 'aria-label': '离开好友社交' }).props.onClick());
    act(() => renderer.root.findByProps({ 'aria-label': '返回好友社交' }).props.onClick());

    expect(JSON.stringify(renderer.toJSON())).not.toContain('封神榜');
  });

  it('renders friend feature actions as real pages instead of clear-only shortcuts', () => {
    const html = renderToStaticMarkup(
      <SocialView state={state} onRefresh={() => undefined} onAction={() => undefined} initialFeatureOpen />,
    );

    expect(html).toContain('清理无效本地黑名单');
    expect(html).toContain('批量移除本地黑名单');
    expect(html).toContain('批量移除本地白名单');
    expect(html).toContain('本地名单设置');
    expect(html).toContain('blacklist_remove_batch');
    expect(html).toContain('whitelist_remove_batch');
    expect(html).not.toContain('blacklist_clear');
    expect(html).not.toContain('whitelist_clear');
  });

  it('renders editable black and white rule settings', () => {
    const html = renderToStaticMarkup(
      <SocialView state={state} onRefresh={() => undefined} onAction={() => undefined} initialRulesOpen />,
    );

    expect(html).toContain('启用本地白名单');
    expect(html).toContain('启用本地黑名单');
    expect(html).toContain('低等级屏蔽');
    expect(html).toContain('保存设置');
    expect(html).toContain('偷菜');
    expect(html).toContain('帮忙');
    expect(html).toContain('捣乱');
  });

  it('renders blacklist and whitelist import/export actions like the reference panel', () => {
    const html = renderToStaticMarkup(
      <SocialView state={state} onRefresh={() => undefined} onAction={() => undefined} initialImportExportOpen />,
    );

    expect(html).toContain('名单导入导出');
    expect(html).toContain('导入本地黑名单');
    expect(html).toContain('导入本地白名单');
    expect(html).toContain('导出本地黑名单');
    expect(html).toContain('导出本地白名单');
    expect(html).toContain('导出全部');
  });

  it('parses plain gid lists and exported social JSON for targeted rule import', () => {
    expect(parseRuleImportText('10001\n10002，10001', 'blacklist')).toEqual(['10001', '10002']);
    expect(parseRuleImportText(JSON.stringify({ rules: { blacklist: ['10003'], whitelist: ['10004'] } }), 'blacklist')).toEqual(['10003']);
    expect(parseRuleImportText(JSON.stringify({ rules: { blacklist: ['10003'], whitelist: ['10004'] } }), 'whitelist')).toEqual(['10004']);
  });

  it('builds imported blacklist rules without replacing whitelist settings', () => {
    const next = buildRulesWithImportedList(state.rules, 'blacklist', '20001\n20002');

    expect(next.blacklist).toEqual(['20001', '20002']);
    expect(next.whitelist).toEqual(state.rules.whitelist);
    expect(next.whitelistEnabled).toBe(true);
  });

  it('formats rule exports as newline separated gids', () => {
    expect(formatRuleExportList(['10002', '10001', '10002'])).toBe('10002\n10001');
  });

  it('builds save-file requests for blacklist export', () => {
    expect(buildRuleExportFileRequest('blacklist', ['10002', '10001', '10002'])).toEqual({
      defaultName: 'farm-blacklist.txt',
      content: '10002\n10001',
      filters: [{ name: 'Text Files', extensions: ['txt'] }],
    });
  });

  it('keeps selected whitelist scopes active and disables matching blacklist scopes', () => {
    const whitelistFirstState: SocialState = {
      ...state,
      rules: {
        ...state.rules,
        whitelistEnabled: true,
        whitelistScopes: ['help'],
        blacklistEnabled: true,
        blacklistScopes: ['mischief'],
      },
    };

    const html = renderToStaticMarkup(
      <SocialView state={whitelistFirstState} onRefresh={() => undefined} onAction={() => undefined} initialRulesOpen />,
    );
    const whitelistHelp = html.match(/<label[^>]*data-scope="whitelist-help"[\s\S]*?<\/label>/)?.[0] || '';
    const blacklistHelp = html.match(/<label[^>]*data-scope="blacklist-help"[\s\S]*?<\/label>/)?.[0] || '';

    expect(html).toContain('本地白名单已启用该范围，本地黑名单不可选');
    expect(whitelistHelp).toContain('checked=""');
    expect(whitelistHelp).not.toContain('disabled=""');
    expect(blacklistHelp).toContain('disabled=""');
  });

  it('keeps selected blacklist scopes active and disables matching whitelist scopes', () => {
    const conflictState: SocialState = {
      ...state,
      rules: {
        ...state.rules,
        whitelistScopes: ['mischief'],
        blacklistEnabled: true,
        blacklistScopes: ['steal', 'help'],
      },
    };

    const html = renderToStaticMarkup(
      <SocialView state={conflictState} onRefresh={() => undefined} onAction={() => undefined} initialRulesOpen />,
    );

    expect(html).toContain('本地黑名单已启用该范围，本地白名单不可选');
    expect(html).toContain('data-scope="whitelist-steal"');
    expect(html).toContain('data-scope="blacklist-steal"');
    const whitelistSteal = html.match(/<label[^>]*data-scope="whitelist-steal"[\s\S]*?<\/label>/)?.[0] || '';
    const blacklistSteal = html.match(/<label[^>]*data-scope="blacklist-steal"[\s\S]*?<\/label>/)?.[0] || '';
    expect(whitelistSteal).toContain('disabled=""');
    expect(blacklistSteal).toContain('checked=""');
    expect(blacklistSteal).not.toContain('disabled=""');
  });

  it('renders the simplified dog guard scan options and cached results', () => {
    const html = renderToStaticMarkup(
      <SocialView
        state={state}
        onRefresh={() => undefined}
        onAction={() => undefined}
        dogGuardState={{
          running: false,
          stopRequested: false,
          total: 2,
          scanned: 1,
          hasGuardDogCount: 1,
          results: [{ gid: 10002, name: 'B', scanned: true, hasGuardDog: true, dogId: 90021, dogName: '护主犬' }],
        }}
        initialDogOpen
      />,
    );

    expect(html).toContain('扫描间隔');
    expect(html).toContain('value="300"');
    expect(html).not.toContain('扫描数量');
    expect(html).not.toContain('进入好友等待');
    expect(html).not.toContain('读取前等待');
    expect(html).toContain('跳过已扫描');
    expect(html).toContain('排除已发现护主犬');
    expect(html).toContain('有护主犬');
  });

  it('starts dog guard scanning with only the supported scan options', async () => {
    const requests: Record<string, unknown>[] = [];
    const renderer = TestRenderer.create(
      <SocialView
        state={state}
        onRefresh={() => undefined}
        onAction={() => undefined}
        onDogGuardAction={(request) => {
          requests.push(request);
        }}
        initialDogOpen
      />,
    );

    await act(async () => {
      await findButton(renderer.root, '开始扫描').props.onClick();
    });

    expect(requests).toEqual([{
      action: 'start',
      refresh: true,
      scanIntervalMs: 300,
      skipScanned: true,
      excludeGuardDog: false,
    }]);
  });

  it('shows one completion toast for a newly finished natural scan and refreshes social state', async () => {
    vi.useFakeTimers();
    const refreshes: boolean[] = [];
    const idle: DogGuardState = {
      running: false,
      stopRequested: false,
      total: 2,
      scanned: 2,
      hasGuardDogCount: 1,
      results: [],
      finishedAt: '2026-07-10T08:00:00Z',
    };
    const renderer = TestRenderer.create(
      <SocialView state={state} dogGuardState={idle} onRefresh={(refresh = false) => { refreshes.push(refresh); }} onAction={() => undefined} />,
    );
    expect(renderer.root.findAllByProps({ role: 'status' })).toHaveLength(0);

    await act(async () => {
      renderer.update(
        <SocialView
          state={state}
          dogGuardState={{ ...idle, running: true, scanned: 0, hasGuardDogCount: 0, startedAt: '2026-07-11T08:00:00Z', finishedAt: undefined }}
          onRefresh={(refresh = false) => { refreshes.push(refresh); }}
          onAction={() => undefined}
        />,
      );
    });
    const finished = {
      ...idle,
      hasGuardDogCount: 2,
      results: [
        { gid: 10002, name: 'B', scanned: true, hasGuardDog: true },
        { gid: 99999, name: '已删除好友', scanned: true, hasGuardDog: true },
      ],
      finishedAt: '2026-07-11T08:05:00Z',
    };
    await act(async () => {
      renderer.update(
        <SocialView state={state} dogGuardState={finished} onRefresh={(refresh = false) => { refreshes.push(refresh); }} onAction={() => undefined} />,
      );
    });

    expect(renderer.root.findAllByProps({ role: 'status' })).toHaveLength(1);
    const successMessage = renderer.root.findByProps({ role: 'status' }).findByType('span').props.children;
    expect(successMessage).toBe('已扫描到 1 名护主犬好友，已自动标记');
    expect(successMessage).not.toContain('已扫描到 2 名');
    expect(refreshes).toEqual([false]);

    act(() => {
      vi.advanceTimersByTime(SOCIAL_TOAST_AUTO_DISMISS_MS);
    });
    expect(renderer.root.findAllByProps({ role: 'status' })).toHaveLength(0);

    await act(async () => {
      renderer.update(
        <SocialView state={state} dogGuardState={{ ...finished }} onRefresh={(refresh = false) => { refreshes.push(refresh); }} onAction={() => undefined} />,
      );
    });
    expect(renderer.root.findAllByProps({ role: 'status' })).toHaveLength(0);
    expect(refreshes).toEqual([false]);
  });

  it('resets the toast timer when scan completion replaces an older action result', async () => {
    vi.useFakeTimers();
    const actionResult: SocialActionResult = { ok: true, status: 'ok', message: '旧操作已完成' };
    const running: DogGuardState = {
      running: true,
      stopRequested: false,
      total: 2,
      scanned: 1,
      hasGuardDogCount: 0,
      results: [],
    };
    const renderer = TestRenderer.create(
      <SocialView state={state} dogGuardState={running} lastActionResult={actionResult} onRefresh={() => undefined} onAction={() => undefined} />,
    );
    act(() => {
      vi.advanceTimersByTime(3000);
    });

    await act(async () => {
      renderer.update(
        <SocialView
          state={state}
          dogGuardState={{
            ...running,
            running: false,
            scanned: 2,
            hasGuardDogCount: 1,
            results: [{ gid: 10002, name: 'B', scanned: true, hasGuardDog: true }],
            finishedAt: '2026-07-11T09:00:00Z',
          }}
          lastActionResult={actionResult}
          onRefresh={() => undefined}
          onAction={() => undefined}
        />,
      );
    });
    expect(renderer.root.findByProps({ role: 'status' }).findByType('span').props.children).toBe('已扫描到 1 名护主犬好友，已自动标记');

    act(() => {
      vi.advanceTimersByTime(500);
    });
    expect(renderer.root.findAllByProps({ role: 'status' })).toHaveLength(1);
    act(() => {
      vi.advanceTimersByTime(SOCIAL_TOAST_AUTO_DISMISS_MS - 500);
    });
    expect(renderer.root.findAllByProps({ role: 'status' })).toHaveLength(0);
  });

  it('refreshes after a stopped scan without showing a success toast', async () => {
    vi.useFakeTimers();
    const refreshes: boolean[] = [];
    const renderer = TestRenderer.create(
      <SocialView
        state={state}
        dogGuardState={{ running: true, stopRequested: false, total: 2, scanned: 1, hasGuardDogCount: 0, results: [] }}
        onRefresh={(refresh = false) => { refreshes.push(refresh); }}
        onAction={() => undefined}
      />,
    );

    await act(async () => {
      renderer.update(
        <SocialView
          state={state}
          dogGuardState={{ running: false, stopRequested: true, total: 2, scanned: 1, hasGuardDogCount: 0, results: [], finishedAt: '2026-07-11T08:05:00Z' }}
          onRefresh={(refresh = false) => { refreshes.push(refresh); }}
          onAction={() => undefined}
        />,
      );
    });

    expect(renderer.root.findAllByProps({ role: 'status' })).toHaveLength(0);
    expect(refreshes).toEqual([false]);
    act(() => {
      vi.advanceTimersByTime(SOCIAL_TOAST_AUTO_DISMISS_MS);
    });
    expect(renderer.root.findAllByProps({ role: 'status' })).toHaveLength(0);
  });

  it('shows and deduplicates an auto-dismissing error toast after a failed scan', async () => {
    vi.useFakeTimers();
    const refreshes: boolean[] = [];
    const renderer = TestRenderer.create(
      <SocialView
        state={state}
        dogGuardState={{ running: true, stopRequested: false, total: 2, scanned: 1, hasGuardDogCount: 0, results: [] }}
        onRefresh={(refresh = false) => { refreshes.push(refresh); }}
        onAction={() => undefined}
      />,
    );

    await act(async () => {
      renderer.update(
        <SocialView
          state={state}
          dogGuardState={{ running: false, stopRequested: false, total: 2, scanned: 1, hasGuardDogCount: 0, results: [], error: '读取失败', finishedAt: '2026-07-11T08:05:00Z' }}
          onRefresh={(refresh = false) => { refreshes.push(refresh); }}
          onAction={() => undefined}
        />,
      );
    });

    const errorToast = renderer.root.findByProps({ role: 'status' });
    expect(errorToast.props.className).toBe('social-toast');
    expect(errorToast.findByType('span').props.children).toBe('读取失败');
    expect(errorToast.findByType('span').props.children).not.toContain('已自动标记');
    expect(refreshes).toEqual([false]);
    act(() => {
      vi.advanceTimersByTime(SOCIAL_TOAST_AUTO_DISMISS_MS);
    });
    expect(renderer.root.findAllByProps({ role: 'status' })).toHaveLength(0);

    await act(async () => {
      renderer.update(
        <SocialView
          state={state}
          dogGuardState={{ running: false, stopRequested: false, total: 2, scanned: 1, hasGuardDogCount: 0, results: [], error: '读取失败', finishedAt: '2026-07-11T08:05:00Z' }}
          onRefresh={(refresh = false) => { refreshes.push(refresh); }}
          onAction={() => undefined}
        />,
      );
    });
    expect(renderer.root.findAllByProps({ role: 'status' })).toHaveLength(0);
    expect(refreshes).toEqual([false]);
  });

  it('submits one protocol unblock and refreshes the runtime list', async () => {
    const requests: Record<string, unknown>[] = [];
    const refresh = vi.fn();
    const renderer = TestRenderer.create(
      <SocialView
        state={state}
        protocolBlockList={state.friends}
        onRefresh={() => undefined}
        onAction={(request) => {
          requests.push(request);
          return { ok: true, status: 'ok', message: '已解除' };
        }}
        onProtocolBlockList={refresh}
        initialProtocolOpen
      />,
    );

    await act(async () => {
      await findButton(renderer.root, '解除系统拉黑').props.onClick();
    });

    expect(requests).toEqual([{ action: 'unblock_friend', target: '10001' }]);
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it('selects protocol friends and submits a batch unblock payload', async () => {
    const requests: Record<string, unknown>[] = [];
    const refresh = vi.fn();
    const renderer = TestRenderer.create(
      <SocialView
        state={state}
        protocolBlockList={state.friends}
        onRefresh={() => undefined}
        onAction={(request) => {
          requests.push(request);
          return { ok: true, status: 'ok', message: '批量解除完成' };
        }}
        onProtocolBlockList={refresh}
        initialProtocolOpen
      />,
    );

    act(() => findButton(renderer.root, '全选当前列表').props.onClick());
    expect(renderer.root.findAllByType('input').filter((input) => input.props.type === 'checkbox' && input.props.checked)).toHaveLength(2);

    await act(async () => {
      await findButton(renderer.root, '解除选中').props.onClick();
    });

    expect(requests).toEqual([{ action: 'unblock_friend_batch', targets: ['10001', '10002'] }]);
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(findButton(renderer.root, '解除选中').props.disabled).toBe(true);
  });

  it('toggles and clears selected protocol friends without submitting', () => {
    const renderer = TestRenderer.create(
      <SocialView
        state={state}
        protocolBlockList={state.friends}
        onRefresh={() => undefined}
        onAction={() => undefined}
        initialProtocolOpen
      />,
    );
    const checkboxes = renderer.root.findAllByType('input').filter((input) => input.props.type === 'checkbox');

    act(() => checkboxes[0].props.onChange());
    expect(renderer.root.findAllByType('input').filter((input) => input.props.type === 'checkbox' && input.props.checked)).toHaveLength(1);
    act(() => findButton(renderer.root, '清空选择').props.onClick());

    expect(renderer.root.findAllByType('input').filter((input) => input.props.type === 'checkbox' && input.props.checked)).toHaveLength(0);
    expect(findButton(renderer.root, '解除选中').props.disabled).toBe(true);
  });

  it('keeps protocol unblock controls disabled while a request is pending and refreshes after failure', async () => {
    const pending = deferred<SocialActionResult>();
    const refresh = vi.fn();
    const renderer = TestRenderer.create(
      <SocialView
        state={state}
        protocolBlockList={state.friends}
        onRefresh={() => undefined}
        onAction={() => pending.promise}
        onProtocolBlockList={refresh}
        initialProtocolOpen
      />,
    );

    let request!: Promise<void>;
    await act(async () => {
      request = findButton(renderer.root, '解除系统拉黑').props.onClick();
      await Promise.resolve();
    });
    expect(findButton(renderer.root, '解除中').props.disabled).toBe(true);
    expect(findButton(renderer.root, '解除选中').props.disabled).toBe(true);

    await act(async () => {
      pending.resolve({ ok: false, status: 'failed', message: '解除失败' });
      await request;
    });
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(findButton(renderer.root, '解除系统拉黑').props.disabled).toBe(false);
  });
});
