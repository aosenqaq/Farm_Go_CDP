import { AlertCircle, CheckCircle2, Radio } from 'lucide-react';

import { eventTone, formatEventTime, targetLabel, type RuntimeEventDto } from '../lib/events';

type LogViewerProps = {
  phase: string;
  lastError?: string;
  events: RuntimeEventDto[];
};

export function LogViewer({ phase, lastError, events }: LogViewerProps) {
  const lines =
    events.length > 0
      ? events.slice(0, 6).map((event) => ({
          key: `${event.id}-${event.timestamp}`,
          icon: event.level === 'error' ? AlertCircle : event.level === 'warn' ? Radio : CheckCircle2,
          time: formatEventTime(event.timestamp),
          source: targetLabel(event.source),
          text: event.message,
          tone: eventTone(event.level),
        }))
      : [
          { key: 'status', icon: CheckCircle2, time: '-', source: 'runtime', text: `当前运行时状态：${phase}`, tone: 'soft' },
          {
            key: 'error',
            icon: lastError ? AlertCircle : Radio,
            time: '-',
            source: 'runtime',
            text: lastError ? `最近错误：${lastError}` : '等待系统日志写入',
            tone: lastError ? 'danger' : 'muted',
          },
        ];

  return (
    <section className="log-panel">
      <div className="panel-title">最近事件 · 自动刷新</div>
      <div className="log-lines">
        {lines.map((line) => {
          const Icon = line.icon;
          return (
            <div className={`log-entry log-${line.tone}`} key={line.key}>
              <Icon size={15} />
              <time>{line.time}</time>
              <strong>{line.source}</strong>
              <span>{line.text}</span>
            </div>
          );
        })}
      </div>
    </section>
  );
}
