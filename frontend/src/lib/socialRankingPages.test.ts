import { describe, expect, it } from 'vitest';

import {
  emptyRankingCache,
  invalidateRankingCache,
  mergeRankingPage,
  rankingCacheKey,
  selectRankingPage,
  shouldApplyRankingResponse,
  type RankingDataVersions,
  type RankingPage,
  type RankingPageItem,
  type RankingPageRequest,
  type RankingRowKind,
  type RankingTab,
  type RankingViewMode,
} from './socialRankingPages';

type ItemIDIsRequired = {} extends Pick<RankingPageItem, 'itemId'> ? false : true;
const itemIDIsRequired: ItemIDIsRequired = true;

const request = (tab: RankingTab, viewMode: RankingViewMode): RankingPageRequest => ({
  tab,
  viewMode,
  dateRange: 'current',
  cursor: '',
  limit: 50,
});

const page = (keys: string[], nextCursor: string): RankingPage => ({
  ok: true,
  status: 'ok',
  message: '',
  tab: 'stolenByMe',
  viewMode: 'timeline',
  dateRange: 'current',
  summary: {
    visitorCount: 0,
    stolenFromMeCount: 0,
    stolenByMeCount: 0,
    stolenByMeRecordCount: keys.length,
  },
  rows: keys.map((key) => ({
    kind: 'stealRecord',
    key,
    timeMS: 1,
    displayName: key,
    items: [],
  })),
  nextCursor,
  hasMore: nextCursor !== '',
});

describe('social ranking page cache', () => {
  it('requires item ids and represents every backend row kind', () => {
    const item: RankingPageItem = { itemId: 0, name: 'Unknown', count: 1 };
    const kinds: RankingRowKind[] = ['stealRecord', 'stealRanking', 'stolenRanking', 'visitorRecord'];

    expect(itemIDIsRequired).toBe(true);
    expect(item).toEqual({ itemId: 0, name: 'Unknown', count: 1 });
    expect(kinds).toHaveLength(4);
  });

  it('keys entries by date range, tab, and view mode', () => {
    expect(rankingCacheKey(request('stolenByMe', 'timeline'))).toBe('current|stolenByMe|timeline');
    expect(rankingCacheKey({ ...request('stolenByMe', 'ranking'), dateRange: '30d' })).toBe(
      '30d|stolenByMe|ranking',
    );
  });

  it('returns undefined for a missing cache entry', () => {
    expect(selectRankingPage(emptyRankingCache(), request('visitors', 'timeline'))).toBeUndefined();
  });

  it('keeps each ranking view in an independent cache entry', () => {
    const current = emptyRankingCache();
    const first = mergeRankingPage(current, request('stolenByMe', 'timeline'), page(['a'], 'next'));
    const second = mergeRankingPage(first, request('visitors', 'timeline'), page(['v'], ''));

    expect(selectRankingPage(second, request('stolenByMe', 'timeline'))?.rows.map((row) => row.key)).toEqual(['a']);
    expect(selectRankingPage(second, request('visitors', 'timeline'))?.rows.map((row) => row.key)).toEqual(['v']);
  });

  it('replaces an entry when merging an empty-cursor page', () => {
    const first = mergeRankingPage(emptyRankingCache(), request('stolenByMe', 'timeline'), page(['old'], 'next'));
    const replaced = mergeRankingPage(first, request('stolenByMe', 'timeline'), page(['new'], ''));

    expect(selectRankingPage(replaced, request('stolenByMe', 'timeline'))?.rows.map((row) => row.key)).toEqual(['new']);
  });

  it('appends a cursor page while preserving the first occurrence of each row key', () => {
    const first = mergeRankingPage(
      emptyRankingCache(),
      request('stolenByMe', 'timeline'),
      page(['a', 'b'], 'c2'),
    );
    const nextPage = page(['b', 'c'], '');
    nextPage.rows[0] = { ...nextPage.rows[0], displayName: 'replacement b' };
    const next = mergeRankingPage(first, { ...request('stolenByMe', 'timeline'), cursor: 'c2' }, nextPage);

    expect(selectRankingPage(next, request('stolenByMe', 'timeline'))?.rows).toMatchObject([
      { key: 'a', displayName: 'a' },
      { key: 'b', displayName: 'b' },
      { key: 'c', displayName: 'c' },
    ]);
  });

  it('invalidates only entries affected by a data-version change', () => {
    const requests = {
      stolenTimeline: request('stolenByMe', 'timeline'),
      stolenRanking: request('stolenByMe', 'ranking'),
      stolenFromMe: request('stolenFromMe', 'timeline'),
      visitors: request('visitors', 'timeline'),
    };
    let cache = emptyRankingCache();
    for (const scopedRequest of Object.values(requests)) {
      cache = mergeRankingPage(cache, scopedRequest, page([scopedRequest.tab], ''));
    }

    const versions: RankingDataVersions = { stolenByMe: 1, visitors: 2 };
    expect(versions).toEqual({ stolenByMe: 1, visitors: 2 });

    const withoutSteals = invalidateRankingCache(cache, 'stolenByMe');
    expect(selectRankingPage(withoutSteals, requests.stolenTimeline)).toBeUndefined();
    expect(selectRankingPage(withoutSteals, requests.stolenRanking)).toBeUndefined();
    expect(selectRankingPage(withoutSteals, requests.stolenFromMe)).toBeDefined();
    expect(selectRankingPage(withoutSteals, requests.visitors)).toBeDefined();

    const withoutVisitors = invalidateRankingCache(cache, 'visitors');
    expect(selectRankingPage(withoutVisitors, requests.stolenTimeline)).toBeDefined();
    expect(selectRankingPage(withoutVisitors, requests.stolenRanking)).toBeDefined();
    expect(selectRankingPage(withoutVisitors, requests.stolenFromMe)).toBeUndefined();
    expect(selectRankingPage(withoutVisitors, requests.visitors)).toBeUndefined();
  });
});

describe('social ranking response generation', () => {
  it('accepts only an exact generation match', () => {
    expect(shouldApplyRankingResponse({ requested: 3, current: 4 })).toBe(false);
    expect(shouldApplyRankingResponse({ requested: 5, current: 4 })).toBe(false);
    expect(shouldApplyRankingResponse({ requested: 4, current: 4 })).toBe(true);
  });
});
