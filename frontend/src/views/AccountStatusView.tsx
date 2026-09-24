import {
  Activity,
  BarChart3,
  Bomb,
  ChartNoAxesCombined,
  CircleUserRound,
  Coins,
  HandHeart,
  RefreshCw,
  ShoppingBasket,
  Sprout,
  Tractor,
  UserRound,
  X,
  type LucideIcon,
} from 'lucide-react';
import { useEffect, useMemo, useRef, useState } from 'react';

import { FarmAccountStatus, FarmWarehouse } from '../../wailsjs/go/main/App';
import { FallbackImage } from '../components/FallbackImage';
import type { RuntimeStatusDto } from './OverviewView';

type AccountStatusViewProps = {
  status?: RuntimeStatusDto;
  initialProfile?: AccountProfileLike;
  initialFertilizer?: FertilizerContainerLike;
  initialWarehouse?: WarehousePayloadLike;
};

type AccountStatusPayloadLike = {
  status?: string;
  message?: string;
  profile?: AccountProfileLike;
  fertilizer?: FertilizerContainerLike;
  profileError?: string;
  fertilizerError?: string;
};

type AccountProfileLike = {
  gid?: number;
  name?: string;
  nick?: string;
  level?: number;
  plantLevel?: number;
  farmMaxLandLevel?: number;
  exp?: number;
  nextLevelExp?: number;
  gold?: number;
  money?: number;
  bean?: number;
  coupon?: number;
  diamond?: number;
  avatarUrl?: string;
  levelProgress?: LevelProgressLike;
  todayStats?: DayStatsLike;
  statsHistory?: StatsHistoryLike;
};

type LevelProgressLike = {
  current?: number;
  expInLevel?: number;
  needed?: number;
  nextLevelExp?: number;
  remaining?: number;
  percent?: number;
  nextLevel?: number;
};

type WarehousePayloadLike = {
  status?: string;
  items?: Array<{
    name?: string;
    count?: number;
    categoryLabel?: string;
  }>;
};

type ActivityCurrencyState =
  | { state: 'loading' }
  | { state: 'available'; count: number; categoryLabel: string }
  | { state: 'unavailable' };

type FertilizerContainerLike = {
  normal?: FertilizerSlotLike;
  organic?: FertilizerSlotLike;
};

type FertilizerSlotLike = {
  available?: boolean;
  remainingSec?: number;
  remainingHours?: number;
  remainingText?: string;
};

type StatsHistoryLike = {
  todayKey?: string;
  days?: DayStatsLike[];
};

type DayStatsLike = {
  dateKey?: string;
  updatedAt?: string;
  runs?: number;
  collect?: number;
  water?: number;
  steal?: number;
  help?: number;
  mischiefGrass?: number;
  mischiefBug?: number;
  sell?: number;
  saleEstimate?: number;
  estimateReady?: boolean;
};

type StatsRange = 'today' | '3d' | '7d' | '15d' | '30d';

const statsRanges: Array<{ value: StatsRange; label: string }> = [
  { value: 'today', label: '今天' },
  { value: '3d', label: '近三天' },
  { value: '7d', label: '近七天' },
  { value: '15d', label: '近十五天' },
  { value: '30d', label: '近三十天' },
];

const statKeys = ['runs', 'collect', 'water', 'steal', 'help', 'mischiefGrass', 'mischiefBug', 'sell', 'saleEstimate'] as const;

const runtimeWaitingText = '等待游戏运行时就绪...';

export function AccountStatusView({ status, initialProfile, initialFertilizer, initialWarehouse }: AccountStatusViewProps) {
  const waitingForRuntime = !initialProfile && !initialFertilizer && status ? !status.ready : false;
  const [profile, setProfile] = useState<AccountProfileLike | undefined>(initialProfile);
  const [fertilizer, setFertilizer] = useState<FertilizerContainerLike | undefined>(initialFertilizer);
  const [loading, setLoading] = useState(!initialProfile && !initialFertilizer && !waitingForRuntime);
  const [profileError, setProfileError] = useState(waitingForRuntime ? runtimeWaitingText : '');
  const [fertilizerError, setFertilizerError] = useState(waitingForRuntime ? runtimeWaitingText : '');
  const [range, setRange] = useState<StatsRange>('today');
  const [activityCurrency, setActivityCurrency] = useState<ActivityCurrencyState>(() => (
    initialWarehouse ? resolveStarSand(initialWarehouse) : (waitingForRuntime ? { state: 'unavailable' } : { state: 'loading' })
  ));
  const [statisticsOpen, setStatisticsOpen] = useState(false);
  const statisticsTriggerRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (initialProfile || initialFertilizer) return;
    if (status && !status.ready) {
      setLoading(false);
      setProfileError(runtimeWaitingText);
      setFertilizerError(runtimeWaitingText);
      return;
    }
    let active = true;
    setLoading(true);
    setProfileError('');
    setFertilizerError('');
    loadAccountStatus((payload) => {
      if (!active) return;
      applyPayload(payload, setProfile, setFertilizer, setProfileError, setFertilizerError);
      setLoading(false);
    }, (message) => {
      if (!active) return;
      setProfileError(message);
      setLoading(false);
    });
    return () => {
      active = false;
    };
  }, [initialProfile, initialFertilizer, status?.ready]);

  useEffect(() => {
    if (initialWarehouse) {
      setActivityCurrency(resolveStarSand(initialWarehouse));
      return;
    }
    if (status && !status.ready) {
      setActivityCurrency({ state: 'unavailable' });
      return;
    }
    let active = true;
    setActivityCurrency({ state: 'loading' });
    loadActivityCurrency((next) => {
      if (active) setActivityCurrency(next);
    });
    return () => {
      active = false;
    };
  }, [initialWarehouse, status?.ready]);

  const statsWindow = useMemo(() => buildStatsWindow(profile, range), [profile, range]);

  function refreshAccount() {
    if (status && !status.ready) {
      setLoading(false);
      setProfileError(runtimeWaitingText);
      setFertilizerError(runtimeWaitingText);
      return;
    }
    setLoading(true);
    setProfileError('');
    setFertilizerError('');
    setActivityCurrency({ state: 'loading' });
    loadAccountStatus((payload) => {
      applyPayload(payload, setProfile, setFertilizer, setProfileError, setFertilizerError);
      setLoading(false);
    }, (message) => {
      setProfileError(message);
      setLoading(false);
    });
    loadActivityCurrency(setActivityCurrency);
  }

  function closeStatistics() {
    setStatisticsOpen(false);
    statisticsTriggerRef.current?.focus();
  }

  return (
    <section className="view-stack fill account-status-view">
      <header className="page-header">
        <div>
          <h1>账户状态</h1>
          <p>账户资产、成长进度、活动货币和肥料容器来自当前游戏运行时。</p>
        </div>
        <div className="account-header-actions">
          <button
            ref={statisticsTriggerRef}
            className="secondary-action account-statistics-action"
            type="button"
            aria-label="打开运行统计"
            onClick={() => setStatisticsOpen(true)}
          >
            <BarChart3 size={15} />
            <span>运行统计</span>
          </button>
          <button className="secondary-action account-refresh-action" type="button" onClick={refreshAccount} disabled={loading}>
            <RefreshCw size={15} />
            <span>刷新账户</span>
          </button>
        </div>
      </header>

      <div className="account-status-layout">
        <section className="account-section account-assets-section">
          <div className="asset-section-header">
            <div>
              <h2>账户资产</h2>
              <p>{profileError || (loading ? '正在读取账户资料...' : '已读取当前账户资料。')}</p>
            </div>
            <CircleUserRound size={20} />
          </div>
          <AccountIdentityBar profile={profile} />
          <AccountGrowthProgress profile={profile} />
          <div className="account-metric-grid">
            <CurrencyMetric label="金币" value={profile?.gold ?? profile?.money} />
            <CurrencyMetric label="金豆豆" value={profile?.bean} />
            <CurrencyMetric label="点券" value={profile?.coupon} />
            <CurrencyMetric label="钻石" value={profile?.diamond} />
          </div>
        </section>

        <section className="account-section account-fertilizer-section">
          <div className="asset-section-header">
            <div>
              <h2>肥料容器</h2>
              <p>{fertilizerError || (loading ? '正在读取肥料容器...' : '显示无机和有机肥料容器剩余时间。')}</p>
            </div>
            <Sprout size={20} />
          </div>
          <div className="metric-grid two">
            <Metric
              label="无机剩余"
              value={fertilizerContainerHoursText(fertilizer?.normal)}
              detail={fertilizerDetailText(fertilizer?.normal)}
            />
            <Metric
              label="有机剩余"
              value={fertilizerContainerHoursText(fertilizer?.organic)}
              detail={fertilizerDetailText(fertilizer?.organic)}
            />
          </div>
        </section>

        <section className="account-section account-activity-currency-section">
          <div className="asset-section-header">
            <div>
              <h2>活动货币</h2>
              <p>当前活动物品</p>
            </div>
            <Coins size={20} />
          </div>
          <ActivityCurrencyCard state={activityCurrency} />
        </section>
      </div>
      {statisticsOpen && (
        <AccountStatisticsDialog
          range={range}
          statsWindow={statsWindow}
          onRangeChange={setRange}
          onClose={closeStatistics}
        />
      )}
    </section>
  );
}

function AccountIdentityBar({ profile }: { profile?: AccountProfileLike }) {
  const name = profile?.name || profile?.nick || '-';
  const avatarUrl = profile?.avatarUrl?.trim() || '';
  const gidValue = Number(profile?.gid);
  const gid = Number.isFinite(gidValue) && gidValue > 0 ? String(Math.trunc(gidValue)) : '-';
  const level = profile?.level ?? '-';

  return (
    <div className="account-identity-bar">
      <div className="account-status-identity-avatar">
        {avatarUrl
          ? <FallbackImage key={avatarUrl} src={avatarUrl} alt={name} />
          : <UserRound size={24} aria-hidden="true" />}
      </div>
      <div className="account-status-identity-copy">
        <strong className="account-identity-name" title={name}>{name}</strong>
        <span className="account-identity-gid">GID {gid}</span>
      </div>
      <span className="account-identity-level">Lv. {level}</span>
    </div>
  );
}

function AccountGrowthProgress({ profile }: { profile?: AccountProfileLike }) {
  const progress = normalizeLevelProgress(profile);
  if (!progress) {
    return (
      <div className="account-growth-card account-growth-unavailable">
        <p>经验数据暂不可用</p>
      </div>
    );
  }
  return (
    <div className="account-growth-card">
      <div className="account-growth-details">
        <div className="account-growth-heading"><span>经验升级进度</span><strong>{formatNumber(progress.current)} / {formatNumber(progress.needed)}</strong></div>
        <div className="account-growth-progress" role="progressbar" aria-label="经验升级进度" aria-valuemin={0} aria-valuemax={progress.needed} aria-valuenow={progress.current}>
          <span style={{ width: `${progress.percent}%` }} />
        </div>
        <div className="account-growth-footer"><span>下一等级 Lv. {progress.nextLevel}</span><span>还差 {formatNumber(progress.remaining)}</span></div>
      </div>
    </div>
  );
}

function ActivityCurrencyCard({ state }: { state: ActivityCurrencyState }) {
  if (state.state !== 'available') {
    return <div className="activity-currency-unavailable">暂未获取</div>;
  }
  return (
    <div className="activity-currency-row">
      <img className="activity-currency-mark" src="/items/starsand.png" alt="星砂" />
      <div className="activity-currency-copy"><strong>星砂</strong><span>{state.categoryLabel} · 仓库实时数量</span></div>
      <output>{formatNumber(state.count)}</output>
    </div>
  );
}

function AccountStatisticsDialog({
  range,
  statsWindow,
  onRangeChange,
  onClose,
}: {
  range: StatsRange;
  statsWindow: ReturnType<typeof buildStatsWindow>;
  onRangeChange: (range: StatsRange) => void;
  onClose: () => void;
}) {
  useEffect(() => {
    if (typeof document === 'undefined') return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [onClose]);

  return (
    <div className="account-statistics-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
      <section className="account-statistics-dialog" role="dialog" aria-modal="true" aria-labelledby="account-statistics-title">
        <header className="account-statistics-dialog-header">
          <div><h2 id="account-statistics-title">运行统计</h2><p>{statsWindow.rangeLabel}</p></div>
          <button className="account-statistics-close" type="button" aria-label="关闭运行统计" onClick={onClose}><X size={18} /></button>
        </header>
        <div className="account-range-tabs" aria-label="运行统计范围">
          {statsRanges.map((item) => (
            <button className={item.value === range ? 'account-range-tab active' : 'account-range-tab'} key={item.value} type="button" onClick={() => onRangeChange(item.value)}>{item.label}</button>
          ))}
        </div>
        <div className="account-stats-grid">
          <AccountStatistic icon={Activity} tone="runs" label="运行次数" value={formatNumber(statsWindow.stats.runs)} />
          <AccountStatistic icon={Sprout} tone="collect" label="收获" value={formatNumber(statsWindow.stats.collect)} />
          <AccountStatistic icon={Tractor} tone="farm" label="务农" value={formatNumber(statsWindow.stats.water)} />
          <AccountStatistic icon={ShoppingBasket} tone="steal" label="偷菜" value={formatNumber(statsWindow.stats.steal)} />
          <AccountStatistic icon={HandHeart} tone="help" label="好友帮忙" value={formatNumber(statsWindow.stats.help)} />
          <AccountStatistic icon={Bomb} tone="mischief" label="好友捣乱" value={formatNumber(statsWindow.stats.mischiefGrass + statsWindow.stats.mischiefBug)} />
          <AccountStatistic icon={Coins} tone="sale" label="出售" value={formatNumber(statsWindow.stats.sell)} />
          <AccountStatistic
            icon={ChartNoAxesCombined}
            tone="estimate"
            label="预估收益"
            value={statsWindow.stats.estimateReady ? formatSaleEstimate(statsWindow.stats.saleEstimate) : '-'}
            title={statsWindow.stats.estimateReady ? formatNumber(statsWindow.stats.saleEstimate) : undefined}
          />
        </div>
      </section>
    </div>
  );
}

function AccountStatistic({ icon: Icon, tone, label, value, title }: {
  icon: LucideIcon;
  tone: 'runs' | 'collect' | 'farm' | 'steal' | 'help' | 'mischief' | 'sale' | 'estimate';
  label: string;
  value: string;
  title?: string;
}) {
  return (
    <article className="account-statistic">
      <Icon className={`account-statistic-icon account-statistic-icon-${tone}`} size={20} strokeWidth={1.8} aria-hidden="true" />
      <span className="account-statistic-label" title={label}>{label}</span>
      <strong className="account-statistic-value" title={title}>{value}</strong>
    </article>
  );
}

function Metric({
  label,
  value,
  detail,
  titleValue,
  compact = false,
}: {
  label: string;
  value: string | number;
  detail?: string;
  titleValue?: string | number;
  compact?: boolean;
}) {
  const title = titleValue == null || titleValue === '-' ? undefined : String(titleValue);
  return (
    <article className="metric-card">
      <div className="metric-label">{label}</div>
      <div className={compact ? 'metric-value compact' : 'metric-value'} title={title}>
        {value}
      </div>
      {detail && <div className="metric-detail">{detail}</div>}
    </article>
  );
}

function CurrencyMetric({ label, value }: { label: string; value: unknown }) {
  return <Metric label={label} value={formatCompactChineseNumber(value)} titleValue={formatNumber(value)} compact />;
}

function loadAccountStatus(onSuccess: (payload: AccountStatusPayloadLike) => void, onError: (message: string) => void) {
  FarmAccountStatus()
    .then((payload) => onSuccess(payload as AccountStatusPayloadLike))
    .catch((err) => onError(err instanceof Error ? err.message : String(err)));
}

function applyPayload(
  payload: AccountStatusPayloadLike,
  setProfile: (profile: AccountProfileLike | undefined) => void,
  setFertilizer: (fertilizer: FertilizerContainerLike | undefined) => void,
  setProfileError: (message: string) => void,
  setFertilizerError: (message: string) => void,
) {
  setProfile(payload.profile);
  setFertilizer(payload.fertilizer);
  setProfileError(payload.profileError || '');
  setFertilizerError(payload.fertilizerError || '');
}

function loadActivityCurrency(onResult: (state: ActivityCurrencyState) => void) {
  FarmWarehouse()
    .then((payload) => onResult(resolveStarSand(payload as WarehousePayloadLike)))
    .catch(() => onResult({ state: 'unavailable' }));
}

function resolveStarSand(payload?: WarehousePayloadLike): ActivityCurrencyState {
  if (payload?.status !== 'runtime') return { state: 'unavailable' };
  const item = payload.items?.find((candidate) => candidate.name === '星砂');
  const count = Number(item?.count);
  if (!item || !Number.isFinite(count) || count < 0) return { state: 'unavailable' };
  return { state: 'available', count, categoryLabel: item.categoryLabel || '道具' };
}

function normalizeLevelProgress(profile?: AccountProfileLike) {
  const source = profile?.levelProgress;
  const current = Number(source?.current ?? source?.expInLevel ?? profile?.exp);
  const needed = Number(source?.needed ?? source?.nextLevelExp ?? profile?.nextLevelExp);
  if (!Number.isFinite(current) || current < 0 || !Number.isFinite(needed) || needed <= 0) return null;
  const remainingValue = Number(source?.remaining);
  const remaining = Number.isFinite(remainingValue) && remainingValue >= 0 ? remainingValue : Math.max(0, needed - current);
  const percentValue = Number(source?.percent);
  const percent = Number.isFinite(percentValue) ? Math.min(100, Math.max(0, percentValue)) : Math.min(100, Math.max(0, Math.round((current / needed) * 100)));
  const nextLevel = Number(source?.nextLevel) || Number(profile?.level || 0) + 1;
  return { current, needed, remaining, percent, nextLevel };
}

function formatNumber(value: unknown) {
  if (value == null || value === '') return '-';
  const n = Number(value);
  return Number.isFinite(n) ? n.toLocaleString('zh-CN') : String(value);
}

export function formatSaleEstimate(value: unknown) {
  if (value == null || value === '') return '-';
  const n = Number(value);
  if (!Number.isFinite(n)) return String(value);

  const absolute = Math.abs(n);
  const unit = absolute >= 100_000_000
    ? { divisor: 100_000_000, suffix: '亿' }
    : absolute >= 10_000_000
      ? { divisor: 10_000_000, suffix: '千万' }
      : absolute >= 1_000_000
        ? { divisor: 1_000_000, suffix: '百万' }
        : null;

  if (!unit) return n.toLocaleString('zh-CN');
  const amount = (n / unit.divisor).toLocaleString('zh-CN', {
    maximumFractionDigits: 2,
    useGrouping: false,
  });
  return `${amount}${unit.suffix}`;
}

function formatCompactChineseNumber(value: unknown) {
  if (value == null || value === '') return '-';
  const n = Number(value);
  if (!Number.isFinite(n)) return String(value);
  const sign = n < 0 ? '-' : '';
  const absolute = Math.trunc(Math.abs(n));
  if (absolute >= 100000000) {
    const yi = Math.floor(absolute / 100000000);
    const wan = Math.floor((absolute % 100000000) / 10000);
    return `${sign}${yi}亿${wan > 0 ? `${wan}万` : ''}`;
  }
  if (absolute >= 10000000) {
    return `${sign}${Math.floor(absolute / 10000)}万`;
  }
  return n.toLocaleString('zh-CN');
}

function fertilizerContainerHoursText(value?: FertilizerSlotLike) {
  const hours = Number(value?.remainingHours);
  if (Number.isFinite(hours)) {
    return `${hours.toLocaleString('zh-CN', { minimumFractionDigits: 1, maximumFractionDigits: 1 })} 小时`;
  }
  const seconds = Number(value?.remainingSec);
  if (Number.isFinite(seconds)) {
    return `${(seconds / 3600).toLocaleString('zh-CN', { minimumFractionDigits: 1, maximumFractionDigits: 1 })} 小时`;
  }
  return value?.remainingText || '-';
}

function fertilizerDetailText(value?: FertilizerSlotLike) {
  return value?.available === false ? '当前不可用' : '剩余小时';
}

function toDateKey(value: unknown) {
  const text = String(value || '').trim();
  return /^\d{4}-\d{2}-\d{2}$/.test(text) ? text : '';
}

function normalizeDayStats(value?: DayStatsLike): Required<DayStatsLike> {
  const source = value || {};
  return {
    dateKey: toDateKey(source.dateKey),
    updatedAt: source.updatedAt || '',
    runs: positiveNumber(source.runs),
    collect: positiveNumber(source.collect),
    water: positiveNumber(source.water),
    steal: positiveNumber(source.steal),
    help: positiveNumber(source.help),
    mischiefGrass: positiveNumber(source.mischiefGrass),
    mischiefBug: positiveNumber(source.mischiefBug),
    sell: positiveNumber(source.sell),
    saleEstimate: positiveNumber(source.saleEstimate),
    estimateReady: source.estimateReady === true,
  };
}

function positiveNumber(value: unknown) {
  return Math.max(0, Number(value) || 0);
}

function addDays(dateKey: string, offset: number) {
  const key = toDateKey(dateKey);
  if (!key) return '';
  const [year, month, day] = key.split('-').map(Number);
  const date = new Date(Date.UTC(year, month - 1, day));
  date.setUTCDate(date.getUTCDate() + offset);
  return date.toISOString().slice(0, 10);
}

function readHistoryDays(profile?: AccountProfileLike) {
  const days = (Array.isArray(profile?.statsHistory?.days) ? profile?.statsHistory?.days || [] : [])
    .map(normalizeDayStats)
    .filter((item) => item.dateKey);
  const today = profile?.todayStats ? normalizeDayStats(profile.todayStats) : null;
  if (today?.dateKey) {
    const index = days.findIndex((item) => item.dateKey === today.dateKey);
    if (index >= 0) days[index] = today;
    else days.push(today);
  }
  return days.sort((a, b) => b.dateKey.localeCompare(a.dateKey));
}

function sumDays(days: ReturnType<typeof readHistoryDays>) {
  const out = normalizeDayStats({});
  for (const day of days) {
    for (const key of statKeys) {
      out[key] += positiveNumber(day[key]);
    }
  }
  return out;
}

export function buildStatsWindow(profile: AccountProfileLike | undefined, range: StatsRange) {
  const windowDays = Math.max(1, Number(range === 'today' ? 1 : range.replace(/\D/g, '')) || 1);
  const days = readHistoryDays(profile);
  const todayKey = toDateKey(profile?.statsHistory?.todayKey) || toDateKey(profile?.todayStats?.dateKey) || days[0]?.dateKey || '';

  if (range === 'today' && profile?.todayStats) {
    const stats = normalizeDayStats(profile.todayStats);
    stats.dateKey = stats.dateKey || todayKey;
    return { stats, coveredDays: stats.dateKey ? 1 : 0, windowDays: 1, rangeLabel: '今天' };
  }
  if (!todayKey) {
    return {
      stats: normalizeDayStats({}),
      coveredDays: 0,
      windowDays,
      rangeLabel: windowDays === 1 ? '今天' : `近${windowDays}天`,
    };
  }

  const startKey = addDays(todayKey, 1 - windowDays);
  const selected = days.filter((item) => item.dateKey >= startKey && item.dateKey <= todayKey);
  const stats = sumDays(selected);
  stats.estimateReady = selected.length === windowDays && selected.every((day) => day.estimateReady);
  return {
    stats,
    coveredDays: selected.length,
    windowDays,
    rangeLabel: windowDays === 1 ? '今天' : `${startKey} ~ ${todayKey}`,
  };
}
