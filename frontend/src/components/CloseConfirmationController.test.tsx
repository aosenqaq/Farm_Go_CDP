import { act, create } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

const state = vi.hoisted(() => ({
  listener: null as (() => void) | null,
  stop: vi.fn(),
}));

vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn((_event: string, listener: () => void) => {
    state.listener = listener;
    return state.stop;
  }),
}));

vi.mock('../../wailsjs/go/main/App', () => ({
  ExitApplication: vi.fn().mockResolvedValue(undefined),
  MinimizeToTray: vi.fn().mockResolvedValue(undefined),
}));

import { ExitApplication, MinimizeToTray } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { CloseConfirmationController } from './CloseConfirmationDialog';

describe('CloseConfirmationController', () => {
  it('opens from the close event, routes decisions, and cleans up the listener', async () => {
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(<CloseConfirmationController />);
      await Promise.resolve();
    });

    expect(EventsOn).toHaveBeenCalledWith('app:close-requested', expect.any(Function));

    await act(async () => {
      state.listener?.();
    });
    expect(renderer.root.findByProps({ name: 'exit-application' })).toBeDefined();

    await act(async () => {
      renderer.root.findByProps({ name: 'minimize-to-tray' }).props.onClick();
      await Promise.resolve();
    });
    expect(MinimizeToTray).toHaveBeenCalledOnce();
    expect(renderer.root.findAllByProps({ name: 'exit-application' })).toHaveLength(0);

    await act(async () => {
      state.listener?.();
    });
    await act(async () => {
      renderer.root.findByProps({ name: 'exit-application' }).props.onClick();
      await Promise.resolve();
    });
    expect(ExitApplication).toHaveBeenCalledOnce();

    act(() => {
      renderer.unmount();
    });
    expect(state.stop).toHaveBeenCalledOnce();
  });
});
