import { useMemo, useState } from 'react';
import { Play, Power, RefreshCw, RotateCcw, ScrollText, ShieldCheck, X, Zap } from 'lucide-react';
import { formatEventTime, targetLabel, type RuntimeEventDto } from '../lib/events';

export type GuardStatusDto = {
  enabled?: boolean;
  armed?: boolean;
  phase: string;
  runtimeTarget?: string;
  timeoutStreak?: number;
  threshold?: number;
  lastTimeoutAt?: string;
  lastHealthyAt?: string;
  lastRestartAt?: string;
  reconnectGraceUntil?: string;
  lastReason?: string;
  lastActionError?: string;
  restartCountInWindow: number;
  maxRestartsPerWindow: number;
  recentRestartReason?: string;
  recentRestartEvents: Array<{
    at: string;
    runtimeTarget: string;
    reason: string;
    trigger: string;
    ok: boolean;
    error?: string;
  }>;
  process?: {
    armed?: boolean;
    scheduledRestartEnabled?: boolean;
    autoMinimizeAfterRestart?: boolean;
    reconnectGraceRemainingMs?: number;
  };
  settings?: {
    timeoutThreshold?: number;
    monitorIntervalMs?: number;
    scheduledRestartIntervalMin?: number;
    networkReconnectIntervalMs?: number;
    networkRecoveryTimeoutMs?: number;
    otherPlaceLoginIntervalMs?: number;
    otherPlaceLoginDelayMin?: number;
  };
  network?: GuardWorkerStatusDto;
  otherPlaceLogin?: GuardWorkerStatusDto;
  recentEvents?: GuardianEventDto[];
};

export type GuardWorkerStatusDto = {
  enabled?: boolean;
  running?: boolean;
  busy?: boolean;
  lastCheckAt?: string;
  lastHandledAt?: string;
  lastResult?: string;
  lastError?: string;
};

export type GuardianEventDto = {
  name: string;
  phase: string;
  runtimeTarget?: string;
  remainingMs?: number;
  handled?: boolean;
  error?: string;
};

export type HostBindingStatusDto = {
  status: string;
  binding?: {
    pid: number;
    processName: string;
  } | null;
};

type GuardViewProps = {
  status: GuardStatusDto;
  events?: RuntimeEventDto[];
  bindingStatus?: HostBindingStatusDto | null;
  onRefresh: () => void;
  onLaunch: () => void;
  onRestart: () => void;
  onToggleEnabled?: (enabled: boolean) => void;
  onSaveSettings?: (input: Record<string, unknown>) => void;
  toggling?: boolean;
};

const phaseLabel: Record<string, string> = {
  disabled: '未启用',
  standby: '待命',
  watching: '监控中',
  degraded: '异常观察',
  restarting: '重启中',
  waiting_reconnect: '等待重连',
  circuit_open: '已熔断',
};

export function GuardView({ status, events = [], bindingStatus, onRefresh, onLaunch, onRestart, onToggleEnabled, onSaveSettings, toggling = false }: GuardViewProps) {
  const [eventsOpen, setEventsOpen] = useState(false);
  const phase = phaseLabel[status.phase] || status.phase || '未知';
  const restartQuota = `${status.restartCountInWindow || 0}/${status.maxRestartsPerWindow || 0}`;
  const target = status.runtimeTarget ? labelForTarget(status.runtimeTarget) : '待同步';
  const bindingText = bindingLabel(bindingStatus);
  const enabled = status.enabled === true || status.armed === true || (status.enabled !== false && status.phase !== 'disabled');
  const guardSwitchText = enabled ? '守护已开启' : '守护已关闭';
  const guardSwitchAction = enabled ? '关闭守护' : '启用守护';
  const eventRows = useMemo(() => guardEvents(status, events), [status, events]);

  return (
    <section className="view-stack guard-stack">
      <header className="page-header">
        <div>
          <h1>守护服务</h1>
          <p>独立监控进程、网络和异地登录，不依赖自动化调度器运行</p>
        </div>
        <div className="header-actions">
          <span className={status.phase === 'circuit_open' ? 'status-badge status-bad' : 'status-badge status-good'}>
            {phase}
          </span>
          <button className="secondary-button guard-events-button" type="button" onClick={() => setEventsOpen(true)} title="查看守护事件">
            <ScrollText size={16} />
            <span>守护事件</span>
            {eventRows.length > 0 ? <em>{eventRows.length}</em> : null}
          </button>
          <button className="icon-button" type="button" onClick={onRefresh} title="刷新守护状态">
            <RefreshCw size={17} />
          </button>
        </div>
      </header>

      <div className="guard-hero">
          <section className="guard-command-panel">
            <div className="section-heading guard-command-heading">
              <div className="section-heading-title">
                <ShieldCheck size={18} />
                <span>当前守护</span>
              </div>
              <button
                className={enabled ? 'guard-switch guard-switch-on' : 'guard-switch'}
                type="button"
                onClick={() => onToggleEnabled?.(!enabled)}
                disabled={toggling || !onToggleEnabled}
                title={guardSwitchAction}
              >
                <Power size={15} />
                <span>{guardSwitchText}</span>
              </button>
            </div>
            <div className="guard-command-grid">
              <Metric label="链路" value={target} />
              <Metric label="状态" value={phase} />
              <Metric label="自动绑定" value={bindingText} compact />
              <Metric label="超时计数" value={`${status.timeoutStreak || 0}/${status.threshold || 0}`} />
              <Metric label="异常重启" value={restartQuota} />
            </div>
            <div className="guard-actions">
              <button className="primary-button guard-wide-button" type="button" onClick={onRestart}>
                <RotateCcw size={17} />
                <span>重启当前小程序</span>
              </button>
              <button className="secondary-button guard-wide-button" type="button" onClick={onLaunch}>
                <Play size={17} />
                <span>启动小程序</span>
              </button>
            </div>
            {(status.lastReason || status.lastActionError) && (
              <div className="guard-alert">
                <Zap size={16} />
                <span>{status.lastActionError || status.lastReason}</span>
              </div>
            )}
          </section>

      </div>

      <section className="guard-capability-grid">
          <WorkerSection
            title="进程异常与定时重启"
            enabled={enabled}
            detail={status.process?.armed || enabled ? '进程监控已布防' : '进程监控待命'}
            onToggle={(value) => onToggleEnabled?.(value)}
            fields={[
              { label: '超时阈值', key: 'timeoutThreshold', value: status.settings?.timeoutThreshold ?? status.threshold ?? 3, unit: '次' },
              { label: '检测间隔', key: 'monitorIntervalMs', value: status.settings?.monitorIntervalMs ?? 3000, unit: 'ms' },
              { label: '重启间隔', key: 'scheduledRestartIntervalMin', value: status.settings?.scheduledRestartIntervalMin ?? 60, unit: '分钟' },
            ]}
            onSave={onSaveSettings}
            toggles={[
              {
                key: 'scheduledRestartEnabled',
                label: '定时重启',
                enabled: status.process?.scheduledRestartEnabled === true,
              },
              {
                key: 'autoMinimizeAfterRestart',
                label: '重启后自动最小化窗口',
                enabled: status.process?.autoMinimizeAfterRestart === true,
              },
            ]}
          />
          <WorkerSection
            title="网络异常即时重连"
            enabled={status.network?.enabled === true}
            detail={workerDetail(status.network, '网络监控已布防', '网络监控待命')}
            onToggle={(value) => onSaveSettings?.({ networkReconnectEnabled: value })}
            fields={[
              { label: '检测间隔', key: 'networkReconnectIntervalMs', value: status.settings?.networkReconnectIntervalMs ?? 1000, unit: 'ms' },
              { label: '恢复超时', key: 'networkRecoveryTimeoutMs', value: status.settings?.networkRecoveryTimeoutMs ?? 20000, unit: 'ms' },
            ]}
            onSave={onSaveSettings}
          />
          <WorkerSection
            title="异地登录延时重连"
            enabled={status.otherPlaceLogin?.enabled === true}
            detail={workerDetail(status.otherPlaceLogin, '异地监控已布防', '异地监控待命')}
            onToggle={(value) => onSaveSettings?.({ otherPlaceLoginEnabled: value })}
            fields={[
              { label: '检测间隔', key: 'otherPlaceLoginIntervalMs', value: status.settings?.otherPlaceLoginIntervalMs ?? 5000, unit: 'ms' },
              { label: '延时重连', key: 'otherPlaceLoginDelayMin', value: status.settings?.otherPlaceLoginDelayMin ?? 5, unit: '分钟' },
            ]}
            onSave={onSaveSettings}
          />
      </section>

      <GuardEventsDialog open={eventsOpen} events={eventRows} onClose={() => setEventsOpen(false)} />
    </section>
  );
}

function GuardEventsDialog({
  open,
  events,
  onClose,
}: {
  open: boolean;
  events: ReturnType<typeof guardEvents>;
  onClose: () => void;
}) {
  if (!open) return null;

  return (
    <div className="dialog-backdrop guard-events-backdrop" role="presentation" onClick={onClose}>
      <aside
        className="guard-events-dialog"
        role="dialog"
        aria-modal="true"
        aria-label="守护事件"
        onClick={(event) => event.stopPropagation()}
      >
        <header className="guard-events-dialog-header">
          <div>
            <h2>守护事件</h2>
            <p>最近运行时观察与恢复结果，可滚动查看近期记录</p>
          </div>
          <button className="icon-button" type="button" aria-label="关闭守护事件" onClick={onClose}>
            <X size={18} />
          </button>
        </header>

        <div className="system-log-table guard-events-dialog-table">
          <div className="system-log-row system-log-head guard-event-row">
            <span>时间</span>
            <span>链路</span>
            <span>触发</span>
            <span>结果</span>
            <span>原因</span>
          </div>
          {events.length === 0 ? (
            <div className="empty-log">暂无守护事件</div>
          ) : (
            events.map((event) => (
              <div className="system-log-row guard-event-row" key={(event as { key?: string }).key || `${event.at}-${event.reason}`}>
                <time>{event.at}</time>
                <strong>{labelForTarget(event.runtimeTarget)}</strong>
                <span>{triggerLabel(event.trigger)}</span>
                <span className={event.ok ? 'level-pill level-soft' : 'level-pill level-danger'}>{event.ok ? '成功' : '失败'}</span>
                <code>{event.error || event.reason}</code>
              </div>
            ))
          )}
        </div>
      </aside>
    </div>
  );
}

function WorkerSection({
  title,
  enabled,
  detail,
  onToggle,
  fields,
  onSave,
  toggles = [],
}: {
  title: string;
  enabled: boolean;
  detail: string;
  onToggle: (enabled: boolean) => void;
  fields: Array<{ label: string; key: string; value: number; unit: string }>;
  onSave?: (input: Record<string, unknown>) => void;
  toggles?: Array<{ key: string; label: string; enabled: boolean }>;
}) {
  return (
    <section className="guard-worker-panel">
      <div className="guard-worker-title">
        <div><h2>{title}</h2></div>
        <button className={enabled ? 'guard-switch guard-switch-on' : 'guard-switch'} type="button" onClick={() => onToggle(!enabled)}>
          <Power size={15} /><span>{enabled ? '已开启' : '已关闭'}</span>
        </button>
      </div>
      <p className="guard-worker-detail">{detail}</p>
      <details className="guard-advanced-settings">
        <summary>参数设置</summary>
        <div className="settings-form guard-settings-form">
          {fields.map((field) => (
            <label key={field.key}>{field.label}
              <span className="guard-number-input"><input type="number" min="0" defaultValue={field.value} onBlur={(event) => onSave?.({ [field.key]: Number(event.currentTarget.value) })} /><em>{field.unit}</em></span>
            </label>
          ))}
          {toggles.map((toggle) => (
            <div className="guard-settings-toggle" key={toggle.key}>
              <span>{toggle.label}</span>
              <button
                className={toggle.enabled ? 'guard-switch guard-switch-on' : 'guard-switch'}
                type="button"
                onClick={() => onSave?.({ [toggle.key]: !toggle.enabled })}
              >
                <Power size={15} />
                <span>{toggle.enabled ? '已开启' : '已关闭'}</span>
              </button>
            </div>
          ))}
        </div>
      </details>
    </section>
  );
}

function workerDetail(worker: GuardWorkerStatusDto | undefined, armedText: string, standbyText: string) {
  if (!worker) return standbyText;
  if (worker.lastError) return worker.lastError;
  const result = String(worker.lastResult || '').trim();
  if (result && !isGenericWorkerResult(result)) return result;
  if (worker.busy) return '恢复操作进行中';
  if (worker.enabled === true || worker.running === true) return armedText;
  return standbyText;
}

function isGenericWorkerResult(value: string) {
  const normalized = value.trim().toLowerCase();
  return normalized === 'enabled' || normalized === 'disabled' || normalized === 'ok' || normalized === 'true' || normalized === 'false';
}

function guardEvents(status: GuardStatusDto, events: RuntimeEventDto[]) {
  const persisted = events.filter((event) => event.type.startsWith('guardian.'));
  if (persisted.length) {
    return persisted.map((event) => ({
      at: formatEventTime(event.timestamp),
      runtimeTarget: targetLabel(String(event.data?.runtimeTarget || '')),
      reason: event.message,
      trigger: event.type.split('.').at(-1) || '-',
      ok: event.level !== 'error' && event.level !== 'warn',
      error: event.level === 'error' || event.level === 'warn' ? event.message : '',
      key: String(event.id),
    }));
  }
  if (status.recentEvents?.length) {
    return status.recentEvents.map((event, index) => ({
      at: '-', runtimeTarget: event.runtimeTarget || '', reason: event.error || event.name, trigger: event.phase, ok: event.error === undefined,
      error: event.error, key: `${event.name}-${event.phase}-${index}`,
    }));
  }
  return (status.recentRestartEvents || []).map((event) => ({ ...event, at: formatTime(event.at), key: `${event.at}-${event.reason}` }));
}

function Metric({ label, value, compact = false }: { label: string; value: string; compact?: boolean }) {
  return (
    <div className="guard-metric">
      <span>{label}</span>
      <strong className={compact ? 'guard-metric-compact' : ''}>{value}</strong>
    </div>
  );
}

function labelForTarget(target: string) {
  if (target === 'wechat_cdp' || target === '微信 CDP') return '微信 CDP';
  if (target === 'yyb_cdp' || target === '应用宝 CDP') return '应用宝 CDP';
  if (target === 'qq_ws' || target === 'QQ WS') return 'QQ WS';
  return target || 'QQ WS';
}

function bindingLabel(status?: HostBindingStatusDto | null) {
  if (!status) return '扫描中';
  if (status.status === 'bound' || status.status === 'already_bound') {
    return status.binding ? `已绑定 PID ${status.binding.pid}` : '已绑定';
  }
  if (status.status === 'ambiguous') return '多候选待确认';
  if (status.status === 'not_found') return '未发现';
  if (status.status === 'error') return '绑定异常';
  return status.status || '扫描中';
}

function formatTime(value: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleTimeString();
}

function triggerLabel(trigger: string) {
  if (trigger === 'manual') return '手动';
  if (trigger === 'auto') return '自动';
  if (trigger === 'scheduled') return '定时';
  return trigger || '-';
}
