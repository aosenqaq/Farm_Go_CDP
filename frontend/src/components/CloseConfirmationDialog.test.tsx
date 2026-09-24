import { act, create } from 'react-test-renderer';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import { CloseConfirmationDialog } from './CloseConfirmationDialog';

describe('CloseConfirmationDialog', () => {
  it('renders exit as the autofocus primary action', () => {
    const html = renderToStaticMarkup(
      <CloseConfirmationDialog open onExit={() => undefined} onMinimize={() => undefined} onCancel={() => undefined} />,
    );

    expect(html).toContain('关闭 Farm_Go');
    expect(html).toContain('最小化到托盘');
    expect(html).toContain('autofocus');
  });

  it('routes each explicit decision to its callback', () => {
    const onExit = vi.fn();
    const onMinimize = vi.fn();
    const onCancel = vi.fn();
    const renderer = create(
      <CloseConfirmationDialog open onExit={onExit} onMinimize={onMinimize} onCancel={onCancel} />,
    );

    act(() => {
      renderer.root.findByProps({ name: 'exit-application' }).props.onClick();
    });
    act(() => {
      renderer.root.findByProps({ name: 'minimize-to-tray' }).props.onClick();
    });
    act(() => {
      renderer.root.findByProps({ name: 'cancel-close' }).props.onClick();
    });

    expect(onExit).toHaveBeenCalledOnce();
    expect(onMinimize).toHaveBeenCalledOnce();
    expect(onCancel).toHaveBeenCalledOnce();
  });
});
