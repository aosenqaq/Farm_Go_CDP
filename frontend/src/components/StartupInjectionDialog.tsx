import { AlertTriangle, CheckCircle2, Loader2, MemoryStick, Play, PlugZap, RefreshCw, RotateCw, X } from 'lucide-react';

import type { StartupInjectionState } from '../lib/startupInjection';

type StartupInjectionDialogProps = {
  open: boolean;
  state: StartupInjectionState | null;
  targetPath: string;
  patching: boolean;
  launching?: boolean;
  restarting?: boolean;
  onTargetPathChange: (value: string) => void;
  onRetry: () => void;
  onLaunch?: () => void;
  onRestart?: () => void;
  onHide: () => void;
};

export function StartupInjectionDialog({
  open,
  state,
  targetPath,
  patching,
  launching = false,
  restarting = false,
  onTargetPathChange,
  onRetry,
  onLaunch,
  onRestart,
  onHide,
}: StartupInjectionDialogProps) {
  if (!open || !state) return null;

  const Icon = state.tone === 'success' ? CheckCircle2 : state.tone === 'error' ? AlertTriangle : PlugZap;

  return (
    <div className="dialog-backdrop" role="presentation">
      <section className={`injection-dialog injection-${state.tone}`} role="dialog" aria-modal="true" aria-label={state.title}>
        <header className="injection-header">
          <div className="injection-icon">
            {state.inFlight || patching ? <Loader2 className="spin" size={22} /> : <Icon size={22} />}
          </div>
          <div>
            <h2>{state.title}</h2>
            <p>{state.headline}</p>
          </div>
          <button className="icon-button light" type="button" onClick={onHide} title="本次隐藏">
            <X size={17} />
          </button>
        </header>

        {state.connectionMethod ? <div className="injection-progress" role="progressbar" aria-label="注入握手进度"><span /></div> : null}

        <div className="injection-metrics">
          <Metric label={state.primaryMetricLabel} value={state.primaryMetricValue} />
          <Metric label={state.secondaryMetricLabel} value={state.secondaryMetricValue} />
          <Metric label={state.tertiaryMetricLabel} value={state.tertiaryMetricValue} />
        </div>

        <div className="injection-message">
          <MemoryStick size={18} />
          <span>{state.detail}</span>
        </div>

        {state.connectionMethod ? (
          <div className="injection-connection-method">
            <span>连接方式</span>
            <strong>{state.connectionMethod}</strong>
          </div>
        ) : null}

        {state.showRetry ? (
          <div className="injection-retry">
            <label>
              <span>手动兜底路径</span>
              <input
                value={targetPath}
                onChange={(event) => onTargetPathChange(event.target.value)}
                placeholder="留空自动查找；也可粘贴 game.js 或版本目录路径"
              />
            </label>
          </div>
        ) : null}

        {(state.targetPath || state.backupPath || state.scriptHash || state.error) && (
          <dl className="injection-detail-list">
            {state.targetPath ? <Detail label="目标" value={state.targetPath} /> : null}
            {state.backupPath ? <Detail label="备份" value={state.backupPath} /> : null}
            {state.scriptHash ? <Detail label="Hash" value={state.scriptHash} /> : null}
            {state.error ? <Detail label="错误" value={state.error} /> : null}
          </dl>
        )}

        <footer className="injection-actions">
          {state.showRetry ? (
            <button
              className="primary-button injection-retry-button"
              type="button"
              onClick={onRetry}
              disabled={patching || state.inFlight}
            >
              {patching || state.inFlight ? <Loader2 className="spin" size={17} /> : <RefreshCw size={17} />}
              <span>{patching || state.inFlight ? '执行中' : '重新注入'}</span>
            </button>
          ) : null}
          {state.showLaunch ? (
            <button
              aria-busy={launching || undefined}
              className="primary-button async-action-button"
              type="button"
              onClick={onLaunch}
              disabled={!onLaunch || launching || patching || state.inFlight}
            >
              {launching ? <Loader2 className="spin" size={17} /> : <Play size={17} />}
              <span>{launching ? '启动中' : '启动小程序'}</span>
            </button>
          ) : null}
          {state.showRestart ? (
            <button
              aria-busy={restarting || undefined}
              className="primary-button async-action-button"
              type="button"
              onClick={onRestart}
              disabled={!onRestart || restarting || patching || state.inFlight}
            >
              {restarting ? <Loader2 className="spin" size={17} /> : <RotateCw size={17} />}
              <span>{restarting ? '重启中' : '重新启动'}</span>
            </button>
          ) : null}
          <button className="secondary-button" type="button" onClick={onHide}>
            本次隐藏
          </button>
        </footer>
      </section>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="injection-metric">
      <span>{label}</span>
      <strong title={value}>{value}</strong>
    </div>
  );
}

function Detail({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt>{label}</dt>
      <dd title={value}>{value}</dd>
    </div>
  );
}
