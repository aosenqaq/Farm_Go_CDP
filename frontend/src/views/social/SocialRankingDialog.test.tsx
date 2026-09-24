import { readFileSync } from 'node:fs';
import TestRenderer, { act, type ReactTestInstance } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import type {
  RankingPage,
  RankingPageRequest,
  RankingPageRow,
  RankingSummary,
} from '../../lib/socialRankingPages';

import {
  SocialRankingDialog,
  type RankingPreferences,
  type SocialRankingDialogProps,
} from './SocialRankingDialog';

type Deferred<T> = {
  promise: Promise<T>;
  resolve: (value: T) => void;
  reject: (reason: unknown) => void;
};

function deferred<T>(): Deferred<T> {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((onResolve, onReject) => {
    resolve = onResolve;
    reject = onReject;
  });
  return { promise, resolve, reject };
}

const summary: RankingSummary = {
  visitorCount: 4,
  stolenFromMeCount: 3,
  stolenByMeCount: 2,
  stolenByMeRecordCount: 5,
};

const preferences: RankingPreferences = {
  stolenByMeViewMode: 'timeline',
  stolenFromMeViewMode: 'timeline',
};

function row(key: string, overrides: Partial<RankingPageRow> = {}): RankingPageRow {
  return {
    kind: 'stealRecord',
    key,
    timeMS: Date.UTC(2026, 6, 20, 8, 30),
    displayName: key,
    items: [],
    ...overrides,
  };
}

function page(
  request: RankingPageRequest,
  keys: string[],
  overrides: Partial<RankingPage> = {},
): RankingPage {
  return {
    ok: true,
    status: 'ok',
    message: '',
    tab: request.tab,
    viewMode: request.viewMode,
    dateRange: request.dateRange,
    summary,
    rows: keys.map((key) => row(key)),
    nextCursor: '',
    hasMore: false,
    ...overrides,
  };
}

function defaultProps(overrides: Partial<SocialRankingDialogProps> = {}): SocialRankingDialogProps {
  return {
    open: true,
    dataVersions: { stolenByMe: 0, visitors: 0 },
    rankingPreferences: preferences,
    onRankings: async (request) => page(request, ['first']),
    onRefreshVisitors: async () => ({ ok: true, status: 'ok', message: '访客记录已刷新。' }),
    onSaveRankingPreferences: async (next) => next,
    onClose: () => undefined,
    onProtocolBlock: () => undefined,
    ...overrides,
  };
}

async function renderDialog(overrides: Partial<SocialRankingDialogProps> = {}) {
  const props = defaultProps(overrides);
  let renderer!: TestRenderer.ReactTestRenderer;
  await act(async () => {
    renderer = TestRenderer.create(<SocialRankingDialog {...props} />);
  });
  return { props, renderer };
}

function text(node: ReactTestInstance): string {
  return node.children.map((child) => (typeof child === 'string' ? child : text(child))).join('');
}

function button(root: ReactTestInstance, label: string): ReactTestInstance {
  const match = root.findAllByType('button').find((candidate) => text(candidate).includes(label));
  if (!match) throw new Error(`missing button: ${label}`);
  return match;
}

function list(root: ReactTestInstance): ReactTestInstance {
  return root.findByProps({ 'data-ranking-scroll': true });
}

function rankingRows(root: ReactTestInstance): ReactTestInstance[] {
  return root.findAll((node) => String(node.props.className || '').split(' ').includes('social-ranking-row'));
}

function nearBottom(root: ReactTestInstance) {
  list(root).props.onScroll({
    currentTarget: { scrollHeight: 500, scrollTop: 305, clientHeight: 100 },
  });
}

describe('SocialRankingDialog', () => {
  it('loads the default first page only when opened and never refreshes visitors implicitly', async () => {
    const request = deferred<RankingPage>();
    const onRankings = vi.fn(() => request.promise);
    const onRefreshVisitors = vi.fn(async () => ({ ok: true, status: 'ok', message: '' }));
    const props = defaultProps({ open: false, onRankings, onRefreshVisitors });
    const renderer = TestRenderer.create(<SocialRankingDialog {...props} />);

    expect(renderer.toJSON()).toBeNull();
    expect(onRankings).not.toHaveBeenCalled();

    await act(async () => {
      renderer.update(<SocialRankingDialog {...props} open />);
    });
    expect(onRankings).toHaveBeenCalledTimes(1);
    expect(onRankings).toHaveBeenCalledWith({
      tab: 'stolenByMe',
      viewMode: 'timeline',
      dateRange: 'current',
      cursor: '',
      limit: 50,
    });
    expect(onRefreshVisitors).not.toHaveBeenCalled();
    expect(renderer.root.findByProps({ className: 'social-ranking-first-loading' })).toBeTruthy();

    await act(async () => request.resolve(page(onRankings.mock.calls[0][0], ['cached'])));
    expect(text(rankingRows(renderer.root)[0])).toContain('cached');
  });

  it('retains a cached page while closed and reopens without a duplicate request', async () => {
    const onRankings = vi.fn(async (request: RankingPageRequest) => page(request, ['cached']));
    const props = defaultProps({ onRankings });
    const { renderer } = await renderDialog({ onRankings });

    act(() => renderer.update(<SocialRankingDialog {...props} open={false} />));
    expect(renderer.toJSON()).toBeNull();
    act(() => renderer.update(<SocialRankingDialog {...props} open />));

    expect(onRankings).toHaveBeenCalledTimes(1);
    expect(text(rankingRows(renderer.root)[0])).toContain('cached');
  });

  it('requests one cursor page within 96px of the bottom and deduplicates it while in flight', async () => {
    const next = deferred<RankingPage>();
    const onRankings = vi.fn((request: RankingPageRequest) => (
      request.cursor
        ? next.promise
        : Promise.resolve(page(request, ['a'], { hasMore: true, nextCursor: 'cursor-2' }))
    ));
    const { renderer } = await renderDialog({ onRankings });

    act(() => {
      nearBottom(renderer.root);
      nearBottom(renderer.root);
    });

    expect(onRankings).toHaveBeenCalledTimes(2);
    expect(onRankings.mock.calls[1][0]).toMatchObject({ cursor: 'cursor-2', limit: 50 });
    expect(renderer.root.findByProps({ className: 'social-ranking-more-loading' })).toBeTruthy();

    await act(async () => next.resolve(page(onRankings.mock.calls[1][0], ['b'])));
    expect(rankingRows(renderer.root).map(text).join('|')).toContain('a');
    expect(rankingRows(renderer.root).map(text).join('|')).toContain('b');
  });

  it('does not page when the cached response has no more rows', async () => {
    const onRankings = vi.fn(async (request: RankingPageRequest) => page(request, ['done']));
    const { renderer } = await renderDialog({ onRankings });

    act(() => nearBottom(renderer.root));

    expect(onRankings).toHaveBeenCalledTimes(1);
  });

  it('offers all ranges and tabs and gives each changed date, tab, and mode a fresh cache key', async () => {
    const onRankings = vi.fn(async (request: RankingPageRequest) => page(request, [`${request.tab}-${request.viewMode}-${request.dateRange}`]));
    const onSaveRankingPreferences = vi.fn(async (next: RankingPreferences) => next);
    const { renderer } = await renderDialog({ onRankings, onSaveRankingPreferences });
    const range = renderer.root.findByProps({ 'aria-label': '排行日期范围' });

    expect(range.findAllByType('option').map((option) => option.props.value)).toEqual(['current', '3d', '7d', '30d', 'all']);
    expect(['偷取记录', '被偷记录', '访客记录'].every((label) => Boolean(button(renderer.root, label)))).toBe(true);

    await act(async () => range.props.onChange({ target: { value: '3d' } }));
    await act(async () => button(renderer.root, '被偷记录').props.onClick());
    await act(async () => button(renderer.root, '排行榜模式').props.onClick());

    expect(onRankings.mock.calls.slice(1).map(([request]) => request)).toEqual([
      { tab: 'stolenByMe', viewMode: 'timeline', dateRange: '3d', cursor: '', limit: 50 },
      { tab: 'stolenFromMe', viewMode: 'timeline', dateRange: '3d', cursor: '', limit: 50 },
      { tab: 'stolenFromMe', viewMode: 'ranking', dateRange: '3d', cursor: '', limit: 50 },
    ]);
    expect(onSaveRankingPreferences).toHaveBeenCalledWith({
      stolenByMeViewMode: 'timeline',
      stolenFromMeViewMode: 'ranking',
    });
  });

  it('applies canonical preferences returned by the latest save', async () => {
    const saved = deferred<RankingPreferences>();
    const onSaveRankingPreferences = vi.fn(() => saved.promise);
    const { renderer } = await renderDialog({ onSaveRankingPreferences });

    act(() => button(renderer.root, '排行榜模式').props.onClick());
    expect(button(renderer.root, '排行榜模式').props['aria-pressed']).toBe(true);

    await act(async () => saved.resolve(preferences));

    expect(button(renderer.root, '列表模式').props['aria-pressed']).toBe(true);
  });

  it('keeps the first ranking-mode response when its preference save finishes first', async () => {
    const saved = deferred<RankingPreferences>();
    const rankingPage = deferred<RankingPage>();
    const onRankings = vi.fn((request: RankingPageRequest) => (
      request.viewMode === 'ranking'
        ? rankingPage.promise
        : Promise.resolve(page(request, ['timeline']))
    ));
    const { renderer } = await renderDialog({
      onRankings,
      onSaveRankingPreferences: vi.fn(() => saved.promise),
    });

    act(() => button(renderer.root, '排行榜模式').props.onClick());
    expect(onRankings.mock.calls[1][0]).toMatchObject({ viewMode: 'ranking', cursor: '' });

    await act(async () => saved.resolve({ ...preferences, stolenByMeViewMode: 'ranking' }));
    await act(async () => rankingPage.resolve(page(onRankings.mock.calls[1][0], ['ranked'])));

    expect(rankingRows(renderer.root).map(text).join('|')).toContain('ranked');
  });

  it('rolls back the latest failed preference save without letting an older failure win', async () => {
    const first = deferred<RankingPreferences>();
    const second = deferred<RankingPreferences>();
    const onSaveRankingPreferences = vi.fn()
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise);
    const { renderer } = await renderDialog({ onSaveRankingPreferences });

    act(() => button(renderer.root, '排行榜模式').props.onClick());
    act(() => button(renderer.root, '列表模式').props.onClick());
    await act(async () => second.reject(new Error('latest save failed')));
    await act(async () => first.reject(new Error('stale save failed')));

    expect(button(renderer.root, '列表模式').props['aria-pressed']).toBe(true);
    expect(text(renderer.root.findByProps({ className: 'social-inline-status' }))).toContain('latest save failed');
    expect(text(renderer.root.findByProps({ className: 'social-inline-status' }))).not.toContain('stale save failed');
  });

  it('rejects an older deferred response after the selected view changes', async () => {
    const oldPage = deferred<RankingPage>();
    const newPage = deferred<RankingPage>();
    const onRankings = vi.fn((request: RankingPageRequest) => (
      request.tab === 'stolenByMe' ? oldPage.promise : newPage.promise
    ));
    const { renderer } = await renderDialog({ onRankings });

    await act(async () => button(renderer.root, '访客记录').props.onClick());
    await act(async () => newPage.resolve(page(onRankings.mock.calls[1][0], ['new-view'])));
    await act(async () => oldPage.resolve(page(onRankings.mock.calls[0][0], ['stale-view'])));

    const visible = rankingRows(renderer.root).map(text).join('|');
    expect(visible).toContain('new-view');
    expect(visible).not.toContain('stale-view');
  });

  it('shows a first-page retry that repeats the empty-cursor request', async () => {
    const onRankings = vi.fn()
      .mockRejectedValueOnce(new Error('首页失败'))
      .mockImplementationOnce(async (request: RankingPageRequest) => page(request, ['recovered']));
    const { renderer } = await renderDialog({ onRankings });

    expect(text(renderer.root.findByProps({ className: 'social-ranking-first-error' }))).toContain('首页失败');
    await act(async () => button(renderer.root, '重试').props.onClick());

    expect(onRankings).toHaveBeenCalledTimes(2);
    expect(onRankings.mock.calls[1][0].cursor).toBe('');
    expect(text(rankingRows(renderer.root)[0])).toContain('recovered');
  });

  it('keeps existing rows after a next-page failure and allows scroll retry', async () => {
    const retry = deferred<RankingPage>();
    const onRankings = vi.fn()
      .mockImplementationOnce(async (request: RankingPageRequest) => page(request, ['kept'], { hasMore: true, nextCursor: 'next' }))
      .mockRejectedValueOnce(new Error('下一页失败'))
      .mockImplementationOnce(() => retry.promise);
    const { renderer } = await renderDialog({ onRankings });

    await act(async () => nearBottom(renderer.root));
    expect(text(rankingRows(renderer.root)[0])).toContain('kept');
    expect(text(renderer.root.findByProps({ className: 'social-ranking-more-error' }))).toContain('下一页失败');

    act(() => nearBottom(renderer.root));
    expect(onRankings).toHaveBeenCalledTimes(3);
    await act(async () => retry.resolve(page(onRankings.mock.calls[2][0], ['after-retry'])));
    expect(rankingRows(renderer.root).map(text).join('|')).toContain('after-retry');
  });

  it('keeps rows and shows non-blocking status when explicit visitor refresh fails', async () => {
    const onRefreshVisitors = vi.fn(async () => ({ ok: false, status: 'failed', message: '运行时断开' }));
    const onRankings = vi.fn(async (request: RankingPageRequest) => page(request, ['kept']));
    const { renderer } = await renderDialog({ onRankings, onRefreshVisitors });

    await act(async () => button(renderer.root, '访客记录').props.onClick());
    await act(async () => button(renderer.root, '刷新访客').props.onClick());

    expect(text(rankingRows(renderer.root)[0])).toContain('kept');
    expect(text(renderer.root.findByProps({ className: 'social-inline-status' }))).toContain('运行时断开');
    expect(onRankings).toHaveBeenCalledTimes(2);
  });

  it('invalidates visitor caches after refresh success and reloads an affected current view', async () => {
    const onRankings = vi.fn(async (request: RankingPageRequest) => page(request, [`load-${onRankings.mock.calls.length}`]));
    const { renderer } = await renderDialog({ onRankings });
    await act(async () => button(renderer.root, '访客记录').props.onClick());

    await act(async () => button(renderer.root, '刷新访客').props.onClick());

    expect(onRankings).toHaveBeenCalledTimes(3);
    expect(onRankings.mock.calls[2][0]).toMatchObject({ tab: 'visitors', cursor: '' });
    expect(text(rankingRows(renderer.root)[0])).toContain('load-3');
  });

  it('invalidates only cache entries affected by each data version', async () => {
    const onRankings = vi.fn(async (request: RankingPageRequest) => page(request, [`${request.tab}-${onRankings.mock.calls.length}`]));
    const props = defaultProps({ onRankings });
    const { renderer } = await renderDialog({ onRankings });

    await act(async () => button(renderer.root, '访客记录').props.onClick());
    await act(async () => button(renderer.root, '偷取记录').props.onClick());
    expect(onRankings).toHaveBeenCalledTimes(2);

    await act(async () => renderer.update(
      <SocialRankingDialog {...props} dataVersions={{ stolenByMe: 1, visitors: 0 }} />,
    ));
    expect(onRankings).toHaveBeenCalledTimes(3);
    expect(onRankings.mock.calls[2][0].tab).toBe('stolenByMe');

    await act(async () => button(renderer.root, '访客记录').props.onClick());
    expect(onRankings).toHaveBeenCalledTimes(3);

    await act(async () => renderer.update(
      <SocialRankingDialog {...props} dataVersions={{ stolenByMe: 1, visitors: 1 }} />,
    ));
    expect(onRankings).toHaveBeenCalledTimes(4);
    expect(onRankings.mock.calls[3][0].tab).toBe('visitors');

    await act(async () => button(renderer.root, '偷取记录').props.onClick());
    expect(onRankings).toHaveBeenCalledTimes(4);
  });

  it('renders page summary and offers a system blacklist action with friend context', async () => {
    const onClose = vi.fn();
    const onProtocolBlock = vi.fn();
    const onRankings = vi.fn(async (request: RankingPageRequest) => page(request, [], {
      rows: [row('ranked', {
        kind: 'stolenRanking',
        displayName: '目标好友',
        rank: 7,
        eventCount: 9,
        stealCount: 12,
        items: [{ itemId: 1, name: '白萝卜', count: 4 }],
        actionTarget: '10002',
      })],
    }));
    const { renderer } = await renderDialog({
      rankingPreferences: { ...preferences, stolenByMeViewMode: 'ranking' },
      onRankings,
      onClose,
      onProtocolBlock,
    });

    const summaryText = text(renderer.root.findByProps({ className: 'social-ranking-summary' }));
    expect(summaryText).toContain('偷取 2 人');
    expect(summaryText).toContain('记录 5 条');
    expect(summaryText).toContain('被偷 3 人');
    expect(summaryText).toContain('访客 4 条');
    expect(text(rankingRows(renderer.root)[0])).toContain('第 7 名 · 目标好友');
    expect(text(rankingRows(renderer.root)[0])).toContain('9 次 · 偷取 12 个 · 白萝卜 x4');

    act(() => button(renderer.root, '加入系统黑名单').props.onClick());
    act(() => renderer.root.findByProps({ 'aria-label': '关闭排行榜' }).props.onClick());
    expect(onProtocolBlock).toHaveBeenCalledWith({ gid: '10002', displayName: '目标好友' });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('prevents every remote ranking record row from shrinking inside the shared scroll list', () => {
    const css = readFileSync(new URL('../../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
    const match = css.match(/\.app-shell-remote \.social-ranking-list > \.social-ranking-row\s*\{([\s\S]*?)\n  \}/);
    const declarations = match?.[1] ?? '';

    expect(declarations).toMatch(/flex:\s*0 0 auto;/);
  });
});
