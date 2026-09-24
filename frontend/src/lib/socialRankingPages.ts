export type RankingTab = 'stolenByMe' | 'stolenFromMe' | 'visitors';

export type RankingViewMode = 'timeline' | 'ranking';

export type RankingRowKind = 'stealRecord' | 'stealRanking' | 'stolenRanking' | 'visitorRecord';

export type RankingDateRange = 'current' | '3d' | '7d' | '30d' | 'all';

export type RankingStatus =
  | 'ok'
  | 'runtime_not_ready'
  | 'unsupported_target'
  | 'busy'
  | 'skipped'
  | 'failed';

export type RankingPageRequest = {
  tab: RankingTab;
  viewMode: RankingViewMode;
  dateRange: RankingDateRange;
  cursor: string;
  limit: number;
};

export type RankingPageItem = {
  itemId: number;
  name: string;
  count: number;
  landIds?: number[];
};

export type RankingPageRow = {
  kind: RankingRowKind;
  key: string;
  timeMS: number;
  displayName: string;
  rank?: number;
  eventCount?: number;
  stealCount?: number;
  items: RankingPageItem[];
  actionType?: number;
  actionLabel?: string;
  actionTarget?: string;
};

export type RankingSummary = {
  visitorCount: number;
  stolenFromMeCount: number;
  stolenByMeCount: number;
  stolenByMeRecordCount: number;
};

export type RankingPage = {
  ok: boolean;
  status: RankingStatus;
  message: string;
  tab: RankingTab;
  viewMode: RankingViewMode;
  dateRange: RankingDateRange;
  summary: RankingSummary;
  rows: RankingPageRow[];
  nextCursor?: string;
  hasMore: boolean;
};

export type RankingCacheEntry = RankingPage;

export type RankingCache = Record<string, RankingCacheEntry>;

export type RankingDataVersions = {
  stolenByMe: number;
  visitors: number;
};

export type RankingDataVersionKey = keyof RankingDataVersions;

export function emptyRankingCache(): RankingCache {
  return {};
}

export function rankingCacheKey(request: RankingPageRequest): string {
  return `${request.dateRange}|${request.tab}|${request.viewMode}`;
}

export function selectRankingPage(
  cache: RankingCache,
  request: RankingPageRequest,
): RankingCacheEntry | undefined {
  return cache[rankingCacheKey(request)];
}

export function mergeRankingPage(
  cache: RankingCache,
  request: RankingPageRequest,
  page: RankingPage,
): RankingCache {
  const key = rankingCacheKey(request);
  if (request.cursor === '' || !cache[key]) {
    return { ...cache, [key]: page };
  }

  const seen = new Set<string>();
  const rows = [...cache[key].rows, ...page.rows].filter((row) => {
    if (seen.has(row.key)) return false;
    seen.add(row.key);
    return true;
  });
  return { ...cache, [key]: { ...page, rows } };
}

export function invalidateRankingCache(cache: RankingCache, changed: RankingDataVersionKey): RankingCache {
  const affectedTabs: RankingTab[] = changed === 'stolenByMe' ? ['stolenByMe'] : ['visitors', 'stolenFromMe'];
  return Object.fromEntries(
    Object.entries(cache).filter(([key]) => !affectedTabs.includes(key.split('|')[1] as RankingTab)),
  );
}

export function shouldApplyRankingResponse(generation: { requested: number; current: number }): boolean {
  return generation.requested === generation.current;
}
