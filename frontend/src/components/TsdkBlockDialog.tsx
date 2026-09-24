import { Shield, X } from 'lucide-react';

import {
  compactEventData,
  eventTone,
  formatEventTime,
  targetLabel,
  tsdkBlockEventsNewestFirst,
  type RuntimeEventDto,
} from '../lib/events';

type TsdkBlockDialogProps = {
  open: boolean;
  events: RuntimeEventDto[];
  onClose: () => void;
};

export function TsdkBlockDialog({ open, events, onClose }: TsdkBlockDialogProps) {
  if (!open) return null;

  const tsdkEvents = tsdkBlockEventsNewestFirst(events);

  return (
    <div
      className="dialog-backdrop tsdk-block-backdrop"
      role="presentation"
      onClick={onClose}
    >
      <section
        className="tsdk-block-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="tsdk-block-title"
        onClick={(e) => e.stopPropagation()}
      >
        <header className="tsdk-block-header">
          <div className="tsdk-block-heading">
            <div className="tsdk-block-icon">
              <Shield size={18} aria-hidden="true" />
            </div>
            <div>
              <h2 id="tsdk-block-title">TSDK 拦截日志</h2>
              <p>反作弊模块上传拦截记录（Fetch 网络层）</p>
            </div>
          </div>
          <button
            className="icon-button light"
            type="button"
            aria-label="关闭拦截日志"
            title="关闭"
            onClick={onClose}
          >
            <X size={17} />
          </button>
        </header>

        <div className="tsdk-block-body">
          {tsdkEvents.length === 0 ? (
            <p className="tsdk-block-empty">暂无拦截记录</p>
          ) : (
            <div className="system-log-table" role="table" aria-label="TSDK 拦截日志">
              <div className="system-log-row system-log-head tsdk-log-row" role="row">
                <span>时间</span>
                <span>级别</span>
                <span>来源</span>
                <span>消息</span>
                <span>数据</span>
              </div>
              {tsdkEvents.map((event) => (
                <div
                  className="system-log-row tsdk-log-row"
                  role="row"
                  key={`${event.id}-${event.timestamp}`}
                >
                  <time>{formatEventTime(event.timestamp)}</time>
                  <span className={`level-pill level-${eventTone(event.level)}`}>
                    {event.level || 'info'}
                  </span>
                  <strong>{targetLabel(event.source)}</strong>
                  <span>{event.message}</span>
                  <code title={compactEventData(event.data)}>
                    {compactEventData(event.data) || '-'}
                  </code>
                </div>
              ))}
            </div>
          )}
        </div>

        <footer className="tsdk-block-footer">
          <span className="tsdk-block-count">{tsdkEvents.length} 条记录</span>
          <button className="primary-button" type="button" onClick={onClose}>
            关闭
          </button>
        </footer>
      </section>
    </div>
  );
}
