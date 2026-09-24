import { useCallback, useEffect, useState } from 'react';
import {
  Bomb,
  Coins,
  HandHeart,
  Megaphone,
  RefreshCw,
  ShoppingBasket,
  Sprout,
  Tractor,
  type LucideIcon,
} from 'lucide-react';

import { ProgramNoticeDialog, type ProgramNoticeDto } from '../components/ProgramNoticeDialog';
import { TsdkBlockDialog } from '../components/TsdkBlockDialog';
import { StatusBadge } from '../components/StatusBadge';
import { GetProgramNotice } from '../../wailsjs/go/main/App';
import { eventTone, formatEventTime, isWorkbenchTaskEvent, targetLabel, tsdkBlockSignal, workbenchTaskName, workbenchTaskResult, type RuntimeEventDto } from '../lib/events';
import type { GuardStatusDto } from './GuardView';

export type RuntimeStatusDto = {
  target: string;
  phase: string;
  connected: boolean;
  ready: boolean;
  instanceId?: string;
  hostVersion?: string;
  lastSeenAt?: string;
  progressDetail?: string;
  lastError?: string;
};

export type WorkspaceRunStatistics = {
  startedAt?: string;
  durationSeconds: number;
  collect: number;
  farm: number;
  steal: number;
  help: number;
  mischief: number;
  saleEstimate: number;
  estimateReady: boolean;
};

type OverviewViewProps = {
  status: RuntimeStatusDto;
  guardStatus?: GuardStatusDto;
  events: RuntimeEventDto[];
  tsdkEvents?: RuntimeEventDto[];
  onRefresh: () => void;
  runStatistics?: WorkspaceRunStatistics | null;
};

export function OverviewView({ status, guardStatus, events, tsdkEvents = [], onRefresh, runStatistics }: OverviewViewProps) {
  const gatewayState = status.phase === 'error' ? '异常' : '正常';
  const controlState = status.connected || status.ready ? '在线' : status.phase === 'listening' ? '监听中' : '等待接入';
  const runtimeState = status.target ? targetLabel(status.target) : '待同步';
  const guardState = guardStatusLabel(statusGuardPhase(status, guardStatus));
  const instanceValue = status.instanceId || (status.ready ? '当前上下文' : '未连接');
  const hostVersionValue = status.hostVersion || (status.ready ? 'CDP 已接入' : '-');
  const taskEvents = events.filter(isWorkbenchTaskEvent);
  const eventLines = taskEvents.length > 0
    ? taskEvents.slice(0, 6).map((event) => ({
        key: `${event.id}-${event.timestamp}`,
        time: formatEventTime(event.timestamp),
        source: workbenchTaskName(event),
        text: workbenchTaskResult(event),
        tone: eventTone(event.level),
      }))
    : [
        { key: 'empty', time: '-', source: '任务日志', text: '暂无任务运行结果', tone: 'muted' },
      ];

  const tsdkSignal = tsdkBlockSignal(tsdkEvents, status);
  const tsdkLabel = tsdkSignal === 'healthy' ? '已拦截' : tsdkSignal === 'danger' ? '启动失败' : '未启用';
  const [tsdkDialogOpen, setTsdkDialogOpen] = useState(false);

  const liveDurationSeconds = useLiveRunDurationSeconds(runStatistics?.startedAt, runStatistics?.durationSeconds);
  const saleEstimate = runStatistics?.estimateReady ? formatSaleEstimate(runStatistics.saleEstimate) : null;
  const [noticeOpen, setNoticeOpen] = useState(false);
  const [noticeLoading, setNoticeLoading] = useState(false);
  const [notice, setNotice] = useState<ProgramNoticeDto | null>(null);

  const loadProgramNotice = useCallback(async () => {
    setNoticeLoading(true);
    try {
      const next = await GetProgramNotice() as ProgramNoticeDto;
      setNotice(next);
    } catch (err) {
      setNotice({
        errorCode: 'license_notice_failed',
        message: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setNoticeLoading(false);
    }
  }, []);

  const openProgramNotice = useCallback(() => {
    setNoticeOpen(true);
    void loadProgramNotice();
  }, [loadProgramNotice]);

  return (
    <section className="view-stack overview-view">
      <header className="page-header">
        <div>
          <h1>运行时工作台</h1>
          <p>全链路连接状态与事件记录</p>
        </div>
        <div className="header-actions">
          <button className="secondary-button" type="button" onClick={openProgramNotice} title="查看程序公告" aria-label="查看程序公告">
            <Megaphone size={16} />
            <span className="header-action-label header-action-label-desktop">查看公告</span>
            <span className="header-action-label header-action-label-mobile">公告</span>
          </button>
          <StatusBadge phase={status.phase} />
          <button className="icon-button" type="button" onClick={onRefresh} title="刷新状态">
            <RefreshCw size={17} />
          </button>
        </div>
      </header>

      <section className="workbench-readiness" aria-label="运行时就绪状态">
        <div className="workbench-hero">
          <p className="workbench-eyebrow">当前链路</p>
          <h2>{runtimeState} {status.ready ? '已就绪' : controlState}</h2>
          <p className={`workbench-conclusion workbench-conclusion-${status.ready ? 'ready' : 'pending'}`}>
            {status.ready ? '可安全执行自动化' : '正在等待运行时完成连接'}
          </p>
        </div>
        <div className="workbench-signals" aria-label="服务状态">
          <span className="workbench-signal"><i className={gatewayState === '正常' ? 'signal-healthy' : 'signal-muted'} />网关 {gatewayState}</span>
          <span className="workbench-signal"><i className={guardState === '监控中' ? 'signal-healthy' : 'signal-muted'} />守护 {guardState}</span>
          <button
            type="button"
            className="workbench-signal workbench-signal-clickable"
            onClick={() => setTsdkDialogOpen(true)}
            title="点击查看 TSDK 拦截详情"
          >
            <i className={`signal-${tsdkSignal}`} />
            TSDK {tsdkLabel}
          </button>
        </div>
      </section>

      <div className="workbench-content">
        <section className="workbench-events" aria-label="最近事件">
          <h2>最近事件</h2>
          <div className="workbench-event-lines">
            {eventLines.map((line) => (
              <div className={`workbench-event workbench-event-${line.tone}`} key={line.key}>
                <time>{line.time}</time>
                <strong>{line.source}</strong>
                <span>{line.text}</span>
              </div>
            ))}
          </div>
        </section>
        <aside className="workbench-route-facts">
          <h2>当前链路</h2>
          <dl className="detail-list">
            <div>
              <dt>目标</dt>
              <dd>{runtimeState}</dd>
            </div>
            <div>
              <dt>连接</dt>
              <dd>{status.connected ? '已连接' : '未连接'}</dd>
            </div>
            <div>
              <dt>就绪</dt>
              <dd>{status.ready ? '已就绪' : '未就绪'}</dd>
            </div>
            <div>
              <dt>实例 ID</dt>
              <dd>{instanceValue}</dd>
            </div>
            <div>
              <dt>Host 版本</dt>
              <dd>{hostVersionValue}</dd>
            </div>
          </dl>
        </aside>
      </div>
      <section className="workbench-run-statistics" aria-label="本次运行统计">
        <div className="workbench-run-statistics-header">
          <h2>本次运行统计</h2>
          <time dateTime={runStatistics?.startedAt || undefined}>{formatDuration(liveDurationSeconds)}</time>
        </div>
        <div className="workbench-run-statistics-grid">
          <WorkbenchMetric icon={Sprout} tone="collect" label="收获次数" value={formatMetricNumber(runStatistics?.collect)} />
          <WorkbenchMetric icon={Tractor} tone="farm" label="务农次数" value={formatMetricNumber(runStatistics?.farm)} />
          <WorkbenchMetric icon={ShoppingBasket} tone="steal" label="偷菜次数" value={formatMetricNumber(runStatistics?.steal)} />
          <WorkbenchMetric icon={HandHeart} tone="help" label="帮助次数" value={formatMetricNumber(runStatistics?.help)} />
          <WorkbenchMetric icon={Bomb} tone="mischief" label="捣乱次数" value={formatMetricNumber(runStatistics?.mischief)} />
          <WorkbenchMetric icon={Coins} tone="sale" label="出售预估收益" value={saleEstimate?.display || '-'} title={saleEstimate?.title} />
        </div>
      </section>
      <ProgramNoticeDialog
        open={noticeOpen}
        loading={noticeLoading}
        notice={notice}
        onClose={() => setNoticeOpen(false)}
        onRetry={() => { void loadProgramNotice(); }}
      />
      <TsdkBlockDialog
        open={tsdkDialogOpen}
        events={tsdkEvents}
        onClose={() => setTsdkDialogOpen(false)}
      />
    </section>
  );
}


type WorkbenchMetricProps = {
  icon: LucideIcon;
  tone: 'collect' | 'farm' | 'steal' | 'help' | 'mischief' | 'sale';
  label: string;
  value: string;
  title?: string;
};

function WorkbenchMetric({ icon: Icon, tone, label, value, title }: WorkbenchMetricProps) {
  return (
    <article className="workbench-statistic">
      <Icon className={`workbench-statistic-icon workbench-statistic-icon-${tone}`} size={20} strokeWidth={1.8} aria-hidden="true" />
      <span className="workbench-statistic-label" title={label}>{label}</span>
      <strong className="workbench-statistic-value" title={title}>{value}</strong>
    </article>
  );
}

function formatMetricNumber(value: unknown) {
  const number = Number(value);
  return Number.isFinite(number) ? number.toLocaleString('zh-CN') : '0';
}

function formatSaleEstimate(value: unknown) {
  const amount = Number(value);
  const number = Number.isFinite(amount) ? amount : 0;
  const formatCompact = (divisor: number, suffix: string) => ({
    display: `${(number / divisor).toLocaleString('zh-CN', { maximumFractionDigits: 2, useGrouping: false })}${suffix}`,
    title: formatMetricNumber(number),
  });

  if (number >= 100_000_000) return formatCompact(100_000_000, '亿');
  if (number >= 10_000) return formatCompact(10_000, '万');
  return { display: formatMetricNumber(number), title: formatMetricNumber(number) };
}

function formatDuration(value: unknown) {
  const totalSeconds = Math.max(0, Math.floor(Number(value) || 0));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  return [hours, minutes, seconds].map((item) => String(item).padStart(2, '0')).join(':');
}

function useLiveRunDurationSeconds(startedAt?: string, fallbackSeconds?: number) {
  const startedAtMs = parseStartedAtMs(startedAt);
  const fallback = Math.max(0, Math.floor(Number(fallbackSeconds) || 0));
  const [nowMs, setNowMs] = useState(() => Date.now());

  useEffect(() => {
    if (startedAtMs == null) {
      setNowMs(Date.now());
      return;
    }
    setNowMs(Date.now());
    const timer = window.setInterval(() => setNowMs(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [startedAtMs]);

  if (startedAtMs == null) return fallback;
  return Math.max(0, Math.floor((nowMs - startedAtMs) / 1000));
}

function parseStartedAtMs(value?: string) {
  const raw = String(value || '').trim();
  if (!raw) return null;
  const ms = Date.parse(raw);
  return Number.isFinite(ms) ? ms : null;
}

function statusGuardPhase(status: RuntimeStatusDto, guardStatus?: GuardStatusDto) {
  if (guardStatus?.phase) return guardStatus.phase;
  if (status.ready || status.connected) return 'watching';
  return '';
}

function guardStatusLabel(phase?: string) {
  const labels: Record<string, string> = {
    disabled: '未启用',
    standby: '待命',
    watching: '监控中',
    degraded: '异常观察',
    restarting: '重启中',
    waiting_reconnect: '等待重连',
    circuit_open: '已熔断',
  };
  return labels[phase || ''] || '待接入';
}
