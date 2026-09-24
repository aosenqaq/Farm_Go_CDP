import { act, create } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const state = vi.hoisted(() => ({ authorizedMounts: 0, remote: false, licenseStatus: null as unknown }));

vi.mock('./AuthorizedApp', () => ({
  default: function AuthorizedAppStub({ remote = false, licenseStatus }: { remote?: boolean; licenseStatus?: unknown }) {
    state.authorizedMounts += 1;
    state.remote = remote;
    state.licenseStatus = licenseStatus;
    return <div>authorized remote console</div>;
  },
}));

vi.mock('./lib/remoteBridge', () => ({
  syncRemoteSession: vi.fn(),
}));

vi.mock('../wailsjs/go/main/App', () => ({
  LicenseStatus: vi.fn(),
}));

import RemoteApp from './RemoteApp';
import { syncRemoteSession } from './lib/remoteBridge';
import { LicenseStatus } from '../wailsjs/go/main/App';

async function flushEffects() {
  await Promise.resolve();
  await Promise.resolve();
}

describe('RemoteApp', () => {
  beforeEach(() => {
    state.authorizedMounts = 0;
    state.remote = false;
    state.licenseStatus = null;
    vi.mocked(syncRemoteSession).mockReset();
    vi.mocked(LicenseStatus).mockReset();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('starts with a remote session check and renders the access-password form', async () => {
    vi.mocked(syncRemoteSession).mockResolvedValue({ authorized: false, csrfToken: '' });
    let view!: ReturnType<typeof create>;

    await act(async () => {
      view = create(<RemoteApp />);
      await flushEffects();
    });

    const markup = JSON.stringify(view.toJSON());
    expect(syncRemoteSession).toHaveBeenCalledTimes(1);
    expect(markup).toContain('验证访问密码');
    expect(markup).toContain('访问密码');
    expect(markup).toContain('进入控制台');
    expect(markup).not.toContain('在此设备记住卡密');
    view.unmount();
  });

  it('renders the authorized app for an already authorized remote session', async () => {
    vi.mocked(syncRemoteSession).mockResolvedValue({ authorized: true, csrfToken: 'existing-csrf' });
    let view!: ReturnType<typeof create>;

    await act(async () => {
      view = create(<RemoteApp />);
      await flushEffects();
    });

    expect(JSON.stringify(view.toJSON())).toContain('authorized remote console');
    expect(state.remote).toBe(true);
    view.unmount();
  });

  it('does not send an access password from an HTTP secure tunnel page', async () => {
    vi.stubGlobal('window', {
      __FARM_GO_TUNNEL__: true,
      location: { protocol: 'http:' },
    });
    vi.mocked(syncRemoteSession).mockResolvedValue({ authorized: false, csrfToken: '' });
    const fetchImpl = vi.fn();
    vi.stubGlobal('fetch', fetchImpl);
    let view!: ReturnType<typeof create>;

    await act(async () => {
      view = create(<RemoteApp />);
      await flushEffects();
    });
    act(() => {
      view.root.findByProps({ id: 'remote-password' }).props.onChange({ target: { value: 'correct horse battery staple' } });
    });
    await act(async () => {
      view.root.findByType('form').props.onSubmit({ preventDefault() {} });
      await flushEffects();
    });

    expect(fetchImpl).not.toHaveBeenCalled();
    expect(JSON.stringify(view.toJSON())).toContain('安全隧道必须通过 HTTPS FRP 入口访问');
    view.unmount();
  });


  it('posts the access password then re-syncs the session before entering the console', async () => {
    vi.stubGlobal('window', {
      __FARM_GO_TUNNEL__: true,
      location: { protocol: 'https:' },
    });
    vi.mocked(syncRemoteSession)
      .mockResolvedValueOnce({ authorized: false, csrfToken: '' })
      .mockResolvedValueOnce({ authorized: true, csrfToken: 'refreshed-csrf' });
    const fetchImpl = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetchImpl);
    let view!: ReturnType<typeof create>;

    await act(async () => {
      view = create(<RemoteApp />);
      await flushEffects();
    });
    act(() => {
      view.root.findByProps({ id: 'remote-password' }).props.onChange({ target: { value: 'correct horse battery staple' } });
    });
    await act(async () => {
      view.root.findByType('form').props.onSubmit({ preventDefault() {} });
      await flushEffects();
    });

    expect(fetchImpl).toHaveBeenCalledWith('/api/auth/login', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: 'correct horse battery staple' }),
    });
    expect(syncRemoteSession).toHaveBeenCalledTimes(2);
    expect(JSON.stringify(view.toJSON())).toContain('authorized remote console');
    view.unmount();
  });

  it('continues to post the access password from a LAN HTTP page', async () => {
    vi.stubGlobal('window', { location: { protocol: 'http:' } });
    vi.mocked(syncRemoteSession)
      .mockResolvedValueOnce({ authorized: false, csrfToken: '' })
      .mockResolvedValueOnce({ authorized: true, csrfToken: 'refreshed-csrf' });
    const fetchImpl = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetchImpl);
    let view!: ReturnType<typeof create>;

    await act(async () => {
      view = create(<RemoteApp />);
      await flushEffects();
    });
    act(() => {
      view.root.findByProps({ id: 'remote-password' }).props.onChange({ target: { value: 'correct horse battery staple' } });
    });
    await act(async () => {
      view.root.findByType('form').props.onSubmit({ preventDefault() {} });
      await flushEffects();
    });

    expect(fetchImpl).toHaveBeenCalledWith('/api/auth/login', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: 'correct horse battery staple' }),
    });
    expect(JSON.stringify(view.toJSON())).toContain('authorized remote console');
    view.unmount();
  });

  it('keeps the password gate visible after a rejected login', async () => {
    vi.mocked(syncRemoteSession).mockResolvedValue({ authorized: false, csrfToken: '' });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 401, statusText: 'Unauthorized' })));
    let view!: ReturnType<typeof create>;

    await act(async () => {
      view = create(<RemoteApp />);
      await flushEffects();
    });
    act(() => {
      view.root.findByProps({ id: 'remote-password' }).props.onChange({ target: { value: 'wrong password' } });
    });
    await act(async () => {
      view.root.findByType('form').props.onSubmit({ preventDefault() {} });
      await flushEffects();
    });

    const markup = JSON.stringify(view.toJSON());
    expect(markup).toContain('访问密码验证失败，请重试');
    expect(markup).not.toContain('authorized remote console');
    view.unmount();
  });
});
