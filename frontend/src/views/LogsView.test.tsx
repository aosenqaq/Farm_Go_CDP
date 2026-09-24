import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { LOG_EXPORT_TOAST_AUTO_DISMISS_MS, LogsView } from './LogsView';

const logsViewSource = readFileSync(join(dirname(fileURLToPath(import.meta.url)), 'LogsView.tsx'), 'utf8');

describe('LogsView', () => {
  it('separates task logs from system logs with tabs', () => {
    const html = renderToStaticMarkup(
      <LogsView
        events={[
          {
            id: 1,
            timestamp: '2026-07-07T05:00:00Z',
            level: 'info',
            source: 'auto_farm',
            type: 'task.done',
            message: '没有检测到可种植空地，本轮自动种植跳过。',
            data: { taskId: 'own_plant' },
          },
          {
            id: 2,
            timestamp: '2026-07-07T05:01:00Z',
            level: 'info',
            source: 'wechat_cdp',
            type: 'runtime.status',
            message: 'wechat_cdp ready',
          },
        ]}
        onRefresh={() => undefined}
      />,
    );

    expect(html).toContain('任务运行日志');
    expect(html).toContain('系统日志');
    expect(html).toContain('自动种植');
    expect(html).toContain('没有检测到可种植空地，本轮自动种植跳过。');
    expect(html).not.toContain('wechat_cdp ready');
  });

  it('hides task start noise and shows task names instead of auto_farm', () => {
    const html = renderToStaticMarkup(
      <LogsView
        events={[
          {
            id: 1,
            timestamp: '2026-07-13T12:10:57+08:00',
            level: 'info',
            source: 'auto_farm',
            type: 'task.start',
            message: 'automation task started',
            data: { taskId: 'own_plant' },
          },
          {
            id: 2,
            timestamp: '2026-07-13T12:10:57+08:00',
            level: 'info',
            source: 'auto_farm',
            type: 'task.done',
            message: '没有检测到可种植空地，本轮自动种植跳过。',
            data: { taskId: 'own_plant', ok: true, status: 'ok' },
          },
          {
            id: 3,
            timestamp: '2026-07-13T12:10:50+08:00',
            level: 'info',
            source: 'auto_farm',
            type: 'auto_farm.settings.save',
            message: '自动农场设置已保存',
          },
        ]}
        onRefresh={() => undefined}
      />,
    );

    expect(html).toContain('自动种植');
    expect(html).toContain('运行结果');
    expect(html).toContain('没有检测到可种植空地，本轮自动种植跳过。');
    expect(html).not.toContain('automation task started');
    expect(html).not.toContain('task.start');
    expect(html).not.toContain('>auto_farm<');
    expect(html).not.toContain('自动农场设置已保存');
  });

  it('uses a dedicated three-row page layout for tabs and log table', () => {
    const html = renderToStaticMarkup(<LogsView events={[]} onRefresh={() => undefined} />);

    expect(html).toContain('class="logs-view view-stack fill"');
    expect(html.indexOf('class="log-tabs"')).toBeLessThan(html.indexOf('system-log-panel'));
  });

  it('marks the visible log panel as an animated tab surface', () => {
    const html = renderToStaticMarkup(<LogsView events={[]} onRefresh={() => undefined} />);

    expect(html).toContain('log-panel-enter');
  });

  it('renders an export button when export handler is provided', () => {
    const html = renderToStaticMarkup(
      <LogsView events={[]} onRefresh={() => undefined} onExportLogs={async () => ({ path: 'C:\\\\logs\\\\task.txt' })} />,
    );

    expect(html).toContain('导出日志');
    expect(html).toContain('导出当前任务运行日志');
  });

  it('uses the global floating toast pattern for export feedback', () => {
    expect(LOG_EXPORT_TOAST_AUTO_DISMISS_MS).toBe(3500);
    expect(logsViewSource).toContain('social-toast');
    expect(logsViewSource).toContain('log-export-toast');
    expect(logsViewSource).not.toContain('log-export-notice');
  });
});
