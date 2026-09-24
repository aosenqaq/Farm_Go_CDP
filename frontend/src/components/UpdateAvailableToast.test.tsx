import { renderToStaticMarkup } from 'react-dom/server';
import { act, create } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { UpdateAvailableToast } from './UpdateAvailableToast';

describe('UpdateAvailableToast', () => {
  it('opens update details without navigating to Settings or the browser', () => {
    const html = renderToStaticMarkup(
      <UpdateAvailableToast
        update={{
          current: { number: 3, name: 'v3.0.0' },
          latest: { number: 4, name: 'v4.0.0' },
          available: true,
        }}
        onOpenDetails={() => undefined}
      />,
    );

    expect(html).toContain('查看更新');
    expect(html).toContain('update-available-toast');
    expect(html).not.toContain('打开下载页面');
  });

  it('calls the update details action when 查看更新 is clicked', () => {
    const onOpenDetails = vi.fn();
    const renderer = create(
      <UpdateAvailableToast
        update={{
          current: { number: 3, name: 'v3.0.0' },
          latest: { number: 4, name: 'v4.0.0' },
          available: true,
        }}
        onOpenDetails={onOpenDetails}
      />,
    );

    act(() => {
      renderer.root.findByProps({ name: 'open-update-details' }).props.onClick();
    });

    expect(onOpenDetails).toHaveBeenCalledTimes(1);
  });
});
