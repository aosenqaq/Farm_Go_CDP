import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import { ProgramNoticeDialog } from './ProgramNoticeDialog';

describe('ProgramNoticeDialog', () => {
  it('renders loading, content, and empty states', () => {
    const loading = renderToStaticMarkup(
      <ProgramNoticeDialog open loading notice={null} onClose={() => undefined} onRetry={() => undefined} />,
    );
    expect(loading).toContain('正在获取最新公告');

    const content = renderToStaticMarkup(
      <ProgramNoticeDialog
        open
        loading={false}
        notice={{ content: '维护通知：今晚 22:00 升级' }}
        onClose={() => undefined}
        onRetry={() => undefined}
      />,
    );
    expect(content).toContain('维护通知：今晚 22:00 升级');
    expect(content).toContain('知道了');

    const empty = renderToStaticMarkup(
      <ProgramNoticeDialog open loading={false} notice={{ content: '  ' }} onClose={() => undefined} onRetry={() => undefined} />,
    );
    expect(empty).toContain('暂无公告内容');
    expect(empty).toContain('重新获取');
  });

  it('shows safe error message without raw credentials', () => {
    const html = renderToStaticMarkup(
      <ProgramNoticeDialog
        open
        loading={false}
        notice={{ errorCode: 'license_notice_failed', message: 'program notice fetch failed' }}
        onClose={() => undefined}
        onRetry={vi.fn()}
      />,
    );
    expect(html).toContain('公告获取失败，请稍后重试');
    expect(html).toContain('重新获取');
  });
});
