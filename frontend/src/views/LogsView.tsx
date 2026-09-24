import { Check, Download, RefreshCw, ScrollText, X } from 'lucide-react';
import { useEffect, useState } from 'react';

import {
  compactEventData,
  eventTone,
  formatEventTime,
  isTaskLifecycleNoise,
  isTaskResultEvent,
  isTsdkBlockEvent,
  targetLabel,
  workbenchTaskName,
  workbenchTaskResult,
  type RuntimeEventDto,
} from '../lib/events';

export const LOG_EXPORT_TOAST_AUTO_DISMISS_MS = 3500;

type LogsViewProps = {
  events: RuntimeEventDto[];
  onRefresh: () => void;
  onExportLogs?: (kind: 'task' | 'system') => Promise<{ path?: string } | void> | void;
  compact?: boolean;
};

export function LogsView({ events, onRefresh, onExportLogs, compact = false }: LogsViewProps) {
  const [activeLogTab, setActiveLogTab] = useState<'system' | 'task'>('task');
  const [exporting, setExporting] = useState(false);
  const [exportToast, setExportToast] = useState<{ ok: boolean; message: string } | null>(null);
  const taskEvents = events.filter(isTaskResultEvent);
  const systemEvents = events.filter(
    (event) => !isTaskResultEvent(event) && !isTaskLifecycleNoise(event) && !isTsdkBlockEvent(event),
  );
  const visibleEvents = activeLogTab === 'task' ? taskEvents : systemEvents;
  const activeTitle = activeLogTab === 'task' ? '任务运行日志' : '系统日志';

  useEffect(() => {
    if (!exportToast) return;
    const timer = window.setTimeout(() => setExportToast(null), LOG_EXPORT_TOAST_AUTO_DISMISS_MS);
    return () => window.clearTimeout(timer);
  }, [exportToast]);

  async function handleExport() {
    if (!onExportLogs || exporting) return;
    setExporting(true);
    try {
      const result = await onExportLogs(activeLogTab);
      const path = result && typeof result === 'object' ? result.path : undefined;
      setExportToast({
        ok: true,
        message: path ? `已导出${activeTitle}到：${path}` : `已导出${activeTitle}`,
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      if (message.includes('已取消')) {
        setExportToast({ ok: false, message: '已取消导出' });
      } else {
        setExportToast({ ok: false, message: message || '导出失败' });
      }
    } finally {
      setExporting(false);
    }
  }

  return (
    <section className={compact ? 'logs-surface compact-tool-surface' : 'logs-view view-stack fill'}>
      {!compact && (
        <header className="page-header">
          <div>
            <h1>日志中心</h1>
            <p>任务运行结果与系统连接、切换、诊断和错误记录分开展示</p>
          </div>
          <div className="header-actions">
            <span className="refresh-caption">自动刷新</span>
            {onExportLogs && (
              <button
                className="secondary-button"
                type="button"
                onClick={handleExport}
                disabled={exporting}
                title={`导出当前${activeTitle}`}
              >
                <Download size={16} />
                {exporting ? '导出中…' : '导出日志'}
              </button>
            )}
            <button className="icon-button" type="button" onClick={onRefresh} title="刷新日志">
              <RefreshCw size={17} />
            </button>
          </div>
        </header>
      )}
      {compact && (
        <div className="compact-toolbar">
          <span className="refresh-caption">自动刷新</span>
          {onExportLogs && (
            <button
              className="secondary-button"
              type="button"
              onClick={handleExport}
              disabled={exporting}
              title={`导出当前${activeTitle}`}
            >
              <Download size={16} />
              {exporting ? '导出中…' : '导出日志'}
            </button>
          )}
          <button className="icon-button" type="button" onClick={onRefresh} title="刷新日志">
            <RefreshCw size={17} />
          </button>
        </div>
      )}

      {exportToast && (
        <div className={exportToast.ok ? 'social-toast ok log-export-toast' : 'social-toast log-export-toast'} role="status" aria-live="polite">
          {exportToast.ok ? <Check size={16} /> : <X size={16} />}
          <span>{exportToast.message}</span>
        </div>
      )}

      <div className="log-tabs" role="tablist" aria-label="日志类型">
        <button
          type="button"
          role="tab"
          aria-selected={activeLogTab === 'task'}
          className={activeLogTab === 'task' ? 'log-tab log-tab-active' : 'log-tab'}
          onClick={() => setActiveLogTab('task')}
        >
          <span>任务运行日志</span>
          <strong>{taskEvents.length}</strong>
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={activeLogTab === 'system'}
          className={activeLogTab === 'system' ? 'log-tab log-tab-active' : 'log-tab'}
          onClick={() => setActiveLogTab('system')}
        >
          <span>系统日志</span>
          <strong>{systemEvents.length}</strong>
        </button>
      </div>

      <section className="system-log-panel log-panel-enter" key={activeLogTab}>
        {visibleEvents.length === 0 ? (
          <div className="empty-log">
            <ScrollText size={22} />
            <span>暂无{activeTitle}</span>
          </div>
        ) : activeLogTab === 'task' ? (
          <div className="system-log-table" role="table" aria-label={activeTitle}>
            <div className="system-log-row system-log-head task-log-row" role="row">
              <span>时间</span>
              <span>级别</span>
              <span>任务</span>
              <span>运行结果</span>
              <span>数据</span>
            </div>
            {visibleEvents.map((event) => (
              <div className="system-log-row task-log-row" role="row" key={`${event.id}-${event.timestamp}`}>
                <time>{formatEventTime(event.timestamp)}</time>
                <span className={`level-pill level-${eventTone(event.level)}`}>{event.level || 'info'}</span>
                <strong title={workbenchTaskName(event)}>{workbenchTaskName(event)}</strong>
                <span title={workbenchTaskResult(event)}>{workbenchTaskResult(event)}</span>
                <code title={compactEventData(event.data)}>{compactEventData(event.data) || '-'}</code>
              </div>
            ))}
          </div>
        ) : (
          <div className="system-log-table" role="table" aria-label={activeTitle}>
            <div className="system-log-row system-log-head" role="row">
              <span>时间</span>
              <span>级别</span>
              <span>来源</span>
              <span>类型</span>
              <span>消息</span>
              <span>数据</span>
            </div>
            {visibleEvents.map((event) => (
              <div className="system-log-row" role="row" key={`${event.id}-${event.timestamp}`}>
                <time>{formatEventTime(event.timestamp)}</time>
                <span className={`level-pill level-${eventTone(event.level)}`}>{event.level || 'info'}</span>
                <strong>{targetLabel(event.source)}</strong>
                <code>{event.type}</code>
                <span>{event.message}</span>
                <code title={compactEventData(event.data)}>{compactEventData(event.data) || '-'}</code>
              </div>
            ))}
          </div>
        )}
      </section>
    </section>
  );
}
