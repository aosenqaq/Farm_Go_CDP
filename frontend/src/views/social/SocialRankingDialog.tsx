import { Ban, LoaderCircle, RefreshCcw, X } from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import {
  emptyRankingCache,
  invalidateRankingCache,
  mergeRankingPage,
  rankingCacheKey,
  selectRankingPage,
  shouldApplyRankingResponse,
  type RankingCache,
  type RankingDataVersions,
  type RankingDateRange,
  type RankingPage,
  type RankingPageRequest,
  type RankingPageRow,
  type RankingSummary,
  type RankingTab,
  type RankingViewMode,
} from '../../lib/socialRankingPages';

export type RankingPreferences = {
  stolenByMeViewMode: RankingViewMode;
  stolenFromMeViewMode: RankingViewMode;
};

export type SocialRankingActionResult = {
  ok: boolean;
  status: string;
  message: string;
};

export type SocialRankingDialogProps = {
  open: boolean;
  dataVersions: RankingDataVersions;
  rankingPreferences: RankingPreferences;
  onRankings: (request: RankingPageRequest) => Promise<RankingPage>;
  onRefreshVisitors: () => Promise<SocialRankingActionResult>;
  onSaveRankingPreferences: (
    preferences: RankingPreferences,
  ) => void | Promise<RankingPreferences | void>;
  onClose: () => void;
  onProtocolBlock: (target: { gid: string; displayName: string }) => void | Promise<void>;
};

type RankingLoading = 'idle' | 'first' | 'more';

type RankingError = {
  kind: 'first' | 'more';
  message: string;
};

const emptySummary: RankingSummary = {
  visitorCount: 0,
  stolenFromMeCount: 0,
  stolenByMeCount: 0,
  stolenByMeRecordCount: 0,
};

function normalizePreferences(preferences: RankingPreferences): RankingPreferences {
  return {
    stolenByMeViewMode: preferences.stolenByMeViewMode === 'ranking' ? 'ranking' : 'timeline',
    stolenFromMeViewMode: preferences.stolenFromMeViewMode === 'ranking' ? 'ranking' : 'timeline',
  };
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function selectedViewMode(tab: RankingTab, preferences: RankingPreferences): RankingViewMode {
  if (tab === 'stolenByMe') return preferences.stolenByMeViewMode;
  if (tab === 'stolenFromMe') return preferences.stolenFromMeViewMode;
  return 'timeline';
}

export function SocialRankingDialog({
  open,
  dataVersions,
  rankingPreferences,
  onRankings,
  onRefreshVisitors,
  onSaveRankingPreferences,
  onClose,
  onProtocolBlock,
}: SocialRankingDialogProps) {
  const [rankingCache, setRankingCache] = useState<RankingCache>(() => emptyRankingCache());
  const [rankingLoading, setRankingLoading] = useState<RankingLoading>('idle');
  const [rankingError, setRankingError] = useState<RankingError | null>(null);
  const [refreshStatus, setRefreshStatus] = useState('');
  const [refreshing, setRefreshing] = useState(false);
  const [tab, setTab] = useState<RankingTab>('stolenByMe');
  const [dateRange, setDateRange] = useState<RankingDateRange>('current');
  const [preferences, setPreferences] = useState<RankingPreferences>(() => normalizePreferences(rankingPreferences));
  const rankingGeneration = useRef(0);
  const rankingInFlight = useRef(new Set<string>());
  const previousDataVersions = useRef(dataVersions);
  const committedPreferences = useRef(normalizePreferences(rankingPreferences));
  const draftPreferences = useRef(normalizePreferences(rankingPreferences));
  const preferenceSavePending = useRef(false);
  const preferenceRequestID = useRef(0);

  useEffect(() => {
    const incoming = normalizePreferences(rankingPreferences);
    committedPreferences.current = incoming;
    if (preferenceSavePending.current) return;
    const changed = incoming.stolenByMeViewMode !== draftPreferences.current.stolenByMeViewMode
      || incoming.stolenFromMeViewMode !== draftPreferences.current.stolenFromMeViewMode;
    draftPreferences.current = incoming;
    if (!changed) return;
    rankingGeneration.current += 1;
    setRankingLoading('idle');
    setRankingError(null);
    setPreferences(incoming);
  }, [rankingPreferences.stolenByMeViewMode, rankingPreferences.stolenFromMeViewMode]);

  useEffect(() => () => {
    preferenceRequestID.current += 1;
  }, []);

  const viewMode = selectedViewMode(tab, preferences);
  const request = useMemo<RankingPageRequest>(() => ({
    tab,
    viewMode,
    dateRange,
    cursor: '',
    limit: 50,
  }), [dateRange, tab, viewMode]);
  const cacheKey = rankingCacheKey(request);
  const currentPage = selectRankingPage(rankingCache, request);

  const loadRankingPage = useCallback(async (
    pageRequest: RankingPageRequest,
    generation = rankingGeneration.current,
  ) => {
    const firstPage = pageRequest.cursor === '';
    const inFlightKey = `${generation}|${rankingCacheKey(pageRequest)}|${pageRequest.cursor}`;
    if (rankingInFlight.current.has(inFlightKey)) return;
    rankingInFlight.current.add(inFlightKey);
    if (shouldApplyRankingResponse({ requested: generation, current: rankingGeneration.current })) {
      setRankingError(null);
      setRankingLoading(firstPage ? 'first' : 'more');
    }

    try {
      const pageResult = await onRankings(pageRequest);
      if (!pageResult.ok) throw new Error(pageResult.message || '排行数据加载失败');
      if (!shouldApplyRankingResponse({ requested: generation, current: rankingGeneration.current })) return;
      setRankingCache((cache) => mergeRankingPage(cache, pageRequest, pageResult));
    } catch (error) {
      if (!shouldApplyRankingResponse({ requested: generation, current: rankingGeneration.current })) return;
      setRankingError({ kind: firstPage ? 'first' : 'more', message: errorMessage(error) });
    } finally {
      rankingInFlight.current.delete(inFlightKey);
      if (shouldApplyRankingResponse({ requested: generation, current: rankingGeneration.current })) {
        setRankingLoading('idle');
      }
    }
  }, [onRankings]);

  useEffect(() => {
    const previous = previousDataVersions.current;
    const changed: Array<keyof RankingDataVersions> = [];
    if (previous.stolenByMe !== dataVersions.stolenByMe) changed.push('stolenByMe');
    if (previous.visitors !== dataVersions.visitors) changed.push('visitors');
    previousDataVersions.current = dataVersions;
    if (changed.length === 0) return;

    const currentAffected = changed.some((version) => (
      version === 'stolenByMe'
        ? tab === 'stolenByMe'
        : tab === 'visitors' || tab === 'stolenFromMe'
    ));
    if (currentAffected) {
      rankingGeneration.current += 1;
      setRankingLoading('idle');
      setRankingError(null);
    }
    setRankingCache((cache) => changed.reduce(invalidateRankingCache, cache));
  }, [dataVersions.stolenByMe, dataVersions.visitors, tab]);

  useEffect(() => {
    if (!open || currentPage) return;
    void loadRankingPage(request);
  }, [cacheKey, currentPage, loadRankingPage, open, request]);

  function changeSelection(change: () => void) {
    rankingGeneration.current += 1;
    setRankingLoading('idle');
    setRankingError(null);
    setRefreshStatus('');
    change();
  }

  function changeTab(nextTab: RankingTab) {
    if (nextTab === tab) return;
    changeSelection(() => setTab(nextTab));
  }

  function changeDateRange(nextRange: RankingDateRange) {
    if (nextRange === dateRange) return;
    changeSelection(() => setDateRange(nextRange));
  }

  function changeViewMode(nextMode: RankingViewMode) {
    if (tab === 'visitors' || nextMode === viewMode) return;
    const preferenceKey = tab === 'stolenByMe' ? 'stolenByMeViewMode' : 'stolenFromMeViewMode';
    const requestID = ++preferenceRequestID.current;
    const nextPreferences = { ...draftPreferences.current, [preferenceKey]: nextMode };
    preferenceSavePending.current = true;
    draftPreferences.current = nextPreferences;
    changeSelection(() => setPreferences(nextPreferences));
    void persistRankingPreferences(nextPreferences, requestID);
  }

  async function persistRankingPreferences(nextPreferences: RankingPreferences, requestID: number) {
    try {
      const saved = await onSaveRankingPreferences(nextPreferences);
      if (requestID !== preferenceRequestID.current) return;
      const committed = normalizePreferences(saved ? { ...nextPreferences, ...saved } : nextPreferences);
      preferenceSavePending.current = false;
      committedPreferences.current = committed;
      draftPreferences.current = committed;
      setRefreshStatus('');
      const canonicalized = committed.stolenByMeViewMode !== nextPreferences.stolenByMeViewMode
        || committed.stolenFromMeViewMode !== nextPreferences.stolenFromMeViewMode;
      if (canonicalized) {
        rankingGeneration.current += 1;
        setRankingLoading('idle');
        setRankingError(null);
        setPreferences(committed);
      }
    } catch (error) {
      if (requestID !== preferenceRequestID.current) return;
      preferenceSavePending.current = false;
      draftPreferences.current = committedPreferences.current;
      rankingGeneration.current += 1;
      setRankingLoading('idle');
      setRankingError(null);
      setRefreshStatus(`保存排行榜偏好失败：${errorMessage(error)}`);
      setPreferences(committedPreferences.current);
    }
  }

  function retryFirstPage() {
    void loadRankingPage(request);
  }

  function loadMore() {
    if (!currentPage?.hasMore || !currentPage.nextCursor || rankingLoading === 'first') return;
    void loadRankingPage({ ...request, cursor: currentPage.nextCursor });
  }

  function handleScroll(event: { currentTarget: { scrollHeight: number; scrollTop: number; clientHeight: number } }) {
    const { scrollHeight, scrollTop, clientHeight } = event.currentTarget;
    if (scrollHeight - scrollTop - clientHeight <= 96) loadMore();
  }

  async function refreshVisitors() {
    if (refreshing) return;
    setRefreshing(true);
    setRefreshStatus('');
    try {
      const result = await onRefreshVisitors();
      if (!result.ok) throw new Error(result.message || '访客刷新失败');
      setRefreshStatus(result.message);
      if (tab === 'visitors' || tab === 'stolenFromMe') {
        rankingGeneration.current += 1;
        setRankingLoading('idle');
        setRankingError(null);
      }
      setRankingCache((cache) => invalidateRankingCache(cache, 'visitors'));
    } catch (error) {
      setRefreshStatus(`访客刷新失败：${errorMessage(error)}`);
    } finally {
      setRefreshing(false);
    }
  }

  if (!open) return null;

  const visibleSummary = currentPage?.summary ?? emptySummary;

  return (
    <div className="dialog-backdrop social-dialog-backdrop">
      <section className="social-dialog wide" role="dialog" aria-labelledby="social-ranking-dialog-title">
        <header>
          <h2 id="social-ranking-dialog-title">排行榜</h2>
          <button className="icon-button light" type="button" onClick={onClose} aria-label="关闭排行榜" title="关闭">
            <X size={16} />
          </button>
        </header>
        <div className="social-dialog-body social-ranking-dialog-body">
          <div className="social-ranking-shell">
            <div className="social-ranking-fixed">
              <div className="social-dialog-toolbar">
                <div className="social-tabs">
                  <RankingTabButton active={tab === 'stolenByMe'} onClick={() => changeTab('stolenByMe')}>偷取记录</RankingTabButton>
                  <RankingTabButton active={tab === 'stolenFromMe'} onClick={() => changeTab('stolenFromMe')}>被偷记录</RankingTabButton>
                  <RankingTabButton active={tab === 'visitors'} onClick={() => changeTab('visitors')}>访客记录</RankingTabButton>
                </div>
                <select
                  className="social-select"
                  aria-label="排行日期范围"
                  value={dateRange}
                  onChange={(event) => changeDateRange(event.target.value as RankingDateRange)}
                >
                  <option value="current">今日</option>
                  <option value="3d">近 3 天</option>
                  <option value="7d">近 7 天</option>
                  <option value="30d">近 30 天</option>
                  <option value="all">全部</option>
                </select>
                <button className="secondary-button social-action-button" type="button" disabled={refreshing} onClick={refreshVisitors}>
                  <RefreshCcw size={16} />
                  {refreshing ? '刷新中' : '刷新访客'}
                </button>
                {tab !== 'visitors' && (
                  <div className="social-tabs social-ranking-mode-tabs">
                    <RankingTabButton active={viewMode === 'timeline'} onClick={() => changeViewMode('timeline')}>列表模式</RankingTabButton>
                    <RankingTabButton active={viewMode === 'ranking'} onClick={() => changeViewMode('ranking')}>排行榜模式</RankingTabButton>
                  </div>
                )}
              </div>
              <div className="social-ranking-summary">
                <span>偷取 {visibleSummary.stolenByMeCount} 人</span>
                <span>记录 {visibleSummary.stolenByMeRecordCount} 条</span>
                <span>被偷 {visibleSummary.stolenFromMeCount} 人</span>
                <span>访客 {visibleSummary.visitorCount} 条</span>
              </div>
              {refreshStatus && <div className="social-inline-status" role="status">{refreshStatus}</div>}
            </div>
            <div className="social-ranking-panel">
              <div className="social-ranking-head">
                <span>时间</span>
                <span>好友</span>
                <span>详情</span>
              </div>
              <div className="social-mini-table social-ranking-list" data-ranking-scroll onScroll={handleScroll}>
                {currentPage?.rows.map((rankingRow) => (
                  <RankingRow key={rankingRow.key} row={rankingRow} onProtocolBlock={onProtocolBlock} />
                ))}
                {!currentPage && rankingLoading === 'first' && (
                  <div className="social-ranking-first-loading" role="status">
                    <LoaderCircle className="social-ranking-spinner" size={18} />
                    <span>加载中</span>
                  </div>
                )}
                {!currentPage && rankingError?.kind === 'first' && (
                  <div className="social-ranking-first-error">
                    <span>{rankingError.message}</span>
                    <button className="secondary-button social-action-button compact" type="button" onClick={retryFirstPage}>重试</button>
                  </div>
                )}
                {currentPage && currentPage.rows.length === 0 && rankingLoading !== 'more' && <div className="social-empty">暂无记录</div>}
                {currentPage && rankingLoading === 'more' && (
                  <div className="social-ranking-more-loading" role="status">
                    <LoaderCircle className="social-ranking-spinner" size={16} />
                    <span>加载更多</span>
                  </div>
                )}
                {currentPage && rankingError?.kind === 'more' && (
                  <div className="social-ranking-more-error" role="status">{rankingError.message}</div>
                )}
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}

function RankingTabButton({
  active,
  children,
  onClick,
}: {
  active: boolean;
  children: string;
  onClick: () => void;
}) {
  return (
    <button className={active ? 'social-tab active' : 'social-tab'} type="button" aria-pressed={active} onClick={onClick}>
      {children}
    </button>
  );
}

function RankingRow({
  row,
  onProtocolBlock,
}: {
  row: RankingPageRow;
  onProtocolBlock: (target: { gid: string; displayName: string }) => void | Promise<void>;
}) {
  return (
    <div className={row.actionTarget ? 'social-mini-row social-ranking-row action' : 'social-mini-row social-ranking-row'}>
      <time className="social-ranking-time">{formatRankingTime(row.timeMS) || '-'}</time>
      <strong className="social-ranking-name">{row.rank ? `第 ${row.rank} 名 · ${row.displayName}` : row.displayName}</strong>
      <span className="social-ranking-detail">{formatRankingDetail(row)}</span>
      {row.actionTarget && (
        <button
          className="secondary-button social-action-button compact"
          type="button"
          onClick={() => onProtocolBlock({ gid: row.actionTarget || '', displayName: row.displayName })}
        >
          <Ban size={14} />
          加入系统黑名单
        </button>
      )}
    </div>
  );
}

function formatRankingDetail(row: RankingPageRow): string {
  let primary = '';
  if (row.kind === 'visitorRecord') {
    primary = row.actionLabel || '访问';
  } else if (row.kind === 'stolenRanking') {
    primary = `${row.eventCount ?? 0} 次`;
    if ((row.stealCount ?? 0) > 0) primary += ` · 偷取 ${row.stealCount} 个`;
  } else {
    primary = `${row.stealCount ?? row.eventCount ?? 0} 次`;
  }
  const items = row.items.map((item) => `${item.name} x${item.count}`).join('、');
  return items ? `${primary} · ${items}` : primary;
}

function formatRankingTime(value: number): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const pad = (part: number) => String(part).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}
