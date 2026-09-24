import { afterEach, describe, expect, it, vi } from 'vitest';

import { installRemoteBridge } from './remoteBridge';

describe('installRemoteBridge', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('posts Wails-style calls with the current session CSRF token', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true }), { status: 200 }));
    const remoteWindow = { __FARM_GO_REMOTE__: true };

    installRemoteBridge({ window: remoteWindow, csrfToken: 'csrf-token', fetchImpl });
    const result = await remoteWindow.go.desktop.App.FarmAutomationState('account-1');

    expect(result).toEqual({ ok: true });
    expect(fetchImpl).toHaveBeenCalledWith('/api/rpc/FarmAutomationState', {
      method: 'POST',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
        'X-Farm-Go-CSRF': 'csrf-token',
      },
      body: JSON.stringify({ args: ['account-1'] }),
    });
  });

  it('encodes method names and rejects failed RPC responses', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(new Response('missing', { status: 404, statusText: 'Not Found' }));
    const remoteWindow = { __FARM_GO_REMOTE__: true };

    installRemoteBridge({ window: remoteWindow, csrfToken: 'csrf-token', fetchImpl });

    await expect(remoteWindow.go.desktop.App['name with/slash']()).rejects.toThrow('Not Found');
    expect(fetchImpl).toHaveBeenCalledWith('/api/rpc/name%20with%2Fslash', expect.objectContaining({ method: 'POST' }));
  });

  it('does not replace the desktop Wails bridge when remote mode is disabled', () => {
    const app = { ExistingMethod: vi.fn() };
    const desktopWindow = { __FARM_GO_REMOTE__: false, go: { desktop: { App: app } } };

    installRemoteBridge({ window: desktopWindow, fetchImpl: vi.fn() });

    expect(desktopWindow.go.desktop.App).toBe(app);
  });
});
