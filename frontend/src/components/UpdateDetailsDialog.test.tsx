import { renderToStaticMarkup } from 'react-dom/server';
import { act, create } from 'react-test-renderer';
// @ts-expect-error The frontend tsconfig intentionally omits Node types; this test reads component styles only.
import { readFileSync } from 'node:fs';
import { describe, expect, it, vi } from 'vitest';

import { UpdateDetailsDialog } from './UpdateDetailsDialog';

const availableUpdate = {
  current: { number: 3, name: 'v3.0.0' },
  latest: {
    number: 4,
    name: 'v4.0.0',
    description: '修复稳定性问题',
    downloadUrl: 'https://example.test/farm-go',
  },
  available: true,
};

describe('UpdateDetailsDialog', () => {
  it('stacks update details and actions on small screens', () => {
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
    const mobileStyles = css.slice(css.indexOf('@media (max-width: 560px)'));

    expect(mobileStyles).toContain('.update-details-versions {\n    grid-template-columns: 1fr;');
    expect(mobileStyles).toContain('.update-details-actions {\n    align-items: stretch;\n    flex-direction: column-reverse;');
  });

  it('shows update details without opening the browser', () => {
    const onDownload = vi.fn();
    const html = renderToStaticMarkup(
      <UpdateDetailsDialog open update={availableUpdate} onClose={() => undefined} onDownload={onDownload} />,
    );

    expect(html).toContain('v3.0.0');
    expect(html).toContain('v4.0.0');
    expect(html).toContain('修复稳定性问题');
    expect(html).toContain('去下载');
    expect(onDownload).not.toHaveBeenCalled();
  });

  it('offers download only for a validated backend URL', () => {
    const onDownload = vi.fn();
    const renderer = create(
      <UpdateDetailsDialog open update={availableUpdate} onClose={() => undefined} onDownload={onDownload} />,
    );

    act(() => {
      renderer.root.findByProps({ name: 'open-update-download' }).props.onClick();
    });

    expect(onDownload).toHaveBeenCalledTimes(1);
    const invalidHtml = renderToStaticMarkup(
      <UpdateDetailsDialog
        open
        update={{ ...availableUpdate, latest: { ...availableUpdate.latest, downloadUrl: 'file:///not-allowed' } }}
        onClose={() => undefined}
        onDownload={() => undefined}
      />,
    );
    expect(invalidHtml).not.toContain('去下载');
  });
});
