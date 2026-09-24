import { act, create } from 'react-test-renderer';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const wails = vi.hoisted(() => ({
  LANAccessSettings: vi.fn(),
  SaveLANAccessSettings: vi.fn(),
}));

vi.mock('../../wailsjs/go/desktop/App', () => wails);

import { LANAccessSettingsPanel } from './LANAccessSettingsPanel';

const loadedStatus = {
  enabled: true,
  mode: 'lan',
  port: 8788,
  passwordConfigured: true,
  running: true,
  address: '192.168.12.34:8788',
  error: '',
};

async function flushEffects() {
  await Promise.resolve();
  await Promise.resolve();
}

describe('LANAccessSettingsPanel', () => {
  beforeEach(() => {
    wails.LANAccessSettings.mockReset();
    wails.SaveLANAccessSettings.mockReset();
    wails.LANAccessSettings.mockResolvedValue(loadedStatus);
  });

  it('shows the running LAN status and listener target returned by the desktop service', async () => {
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(<LANAccessSettingsPanel />);
      await flushEffects();
    });

    const markup = JSON.stringify(renderer.toJSON());
    expect(wails.LANAccessSettings).toHaveBeenCalledTimes(1);
    expect(markup).toContain('局域网 Web 控制台');
    expect(markup).toContain('正在运行');
    expect(markup).toContain('192.168.12.34:8788');
    expect(markup).toContain('局域网地址');
    renderer.unmount();
  });

  it('uses the LAN access and secure tunnel labels required by the desktop settings', async () => {
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(<LANAccessSettingsPanel />);
      await flushEffects();
    });

    const markup = JSON.stringify(renderer.toJSON());
    expect(markup).toContain('开启局域网访问');
    expect(markup).toContain('局域网');
    expect(markup).toContain('安全隧道');
    expect(markup).toContain('安全隧道（仅 HTTPS）');
    expect(markup).not.toContain('安全隧道（127.0.0.1）');
    expect(renderer.root.findByProps({ name: 'lan-access-port' }).props.min).toBe(1024);
    renderer.unmount();
  });

  it('uses the visible LAN password field style hook', async () => {
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(<LANAccessSettingsPanel />);
      await flushEffects();
    });

    const password = renderer.root.findByProps({ name: 'lan-access-password' });
    const confirmation = renderer.root.findByProps({ name: 'lan-access-confirm-password' });
    expect(password.props.type).toBe('password');
    expect(confirmation.props.type).toBe('password');
    expect(password.props.className).toBe('lan-access-password-input');
    expect(confirmation.props.className).toBe('lan-access-password-input');
    renderer.unmount();
  });

  it('uses the concise LAN settings save label', async () => {
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(<LANAccessSettingsPanel />);
      await flushEffects();
    });

    const markup = JSON.stringify(renderer.toJSON());
    expect(markup).toContain('保存设置');
    expect(markup).not.toContain('保存 LAN 设置');
    renderer.unmount();
  });

  it('keeps the save request local when a new password is shorter than twelve characters', async () => {
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(<LANAccessSettingsPanel />);
      await flushEffects();
    });

    act(() => {
      renderer.root.findByProps({ name: 'lan-access-password' }).props.onChange({ target: { value: '12345678901' } });
      renderer.root.findByProps({ name: 'lan-access-confirm-password' }).props.onChange({ target: { value: '12345678901' } });
    });
    await act(async () => {
      await renderer.root.findByProps({ name: 'save-lan-access-settings' }).props.onClick();
    });

    expect(JSON.stringify(renderer.toJSON())).toContain('至少 12 位');
    expect(wails.SaveLANAccessSettings).not.toHaveBeenCalled();
    renderer.unmount();
  });

  it('rejects a port below the LAN listener range before saving', async () => {
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(<LANAccessSettingsPanel />);
      await flushEffects();
    });

    act(() => {
      renderer.root.findByProps({ name: 'lan-access-port' }).props.onChange({ target: { value: '1023' } });
    });
    await act(async () => {
      await renderer.root.findByProps({ name: 'save-lan-access-settings' }).props.onClick();
    });

    expect(JSON.stringify(renderer.toJSON())).toContain('1024 到 65535');
    expect(wails.SaveLANAccessSettings).not.toHaveBeenCalled();
    renderer.unmount();
  });

  it('updates the listener target before save when the listening scope changes', async () => {
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(<LANAccessSettingsPanel />);
      await flushEffects();
    });

    act(() => {
      renderer.root.findByProps({ name: 'lan-access-mode' }).props.onChange({ target: { value: 'tunnel' } });
    });

    const markup = JSON.stringify(renderer.toJSON());
    expect(markup).toContain('FRP 目标');
    expect(markup).toContain('127.0.0.1:8788');
    renderer.unmount();
  });

  it('does not present a wildcard listener as a usable LAN address', async () => {
    wails.LANAccessSettings.mockResolvedValue({ ...loadedStatus, address: '' });
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(<LANAccessSettingsPanel />);
      await flushEffects();
    });

    expect(JSON.stringify(renderer.toJSON())).toContain('未检测到可用的局域网 IPv4 地址');
    renderer.unmount();
  });

  it('saves a matching twelve-character password and refreshed listener status', async () => {
    const savedStatus = { ...loadedStatus, mode: 'tunnel', address: '127.0.0.1:9788', port: 9788 };
    wails.SaveLANAccessSettings.mockResolvedValue(savedStatus);
    let renderer!: ReturnType<typeof create>;
    await act(async () => {
      renderer = create(<LANAccessSettingsPanel />);
      await flushEffects();
    });

    act(() => {
      renderer.root.findByProps({ name: 'lan-access-enabled' }).props.onChange({ target: { checked: true } });
      renderer.root.findByProps({ name: 'lan-access-mode' }).props.onChange({ target: { value: 'tunnel' } });
      renderer.root.findByProps({ name: 'lan-access-port' }).props.onChange({ target: { value: '9788' } });
      renderer.root.findByProps({ name: 'lan-access-password' }).props.onChange({ target: { value: '123456789012' } });
      renderer.root.findByProps({ name: 'lan-access-confirm-password' }).props.onChange({ target: { value: '123456789012' } });
    });
    await act(async () => {
      await renderer.root.findByProps({ name: 'save-lan-access-settings' }).props.onClick();
    });

    expect(wails.SaveLANAccessSettings).toHaveBeenCalledWith({
      enabled: true,
      mode: 'tunnel',
      port: 9788,
      password: '123456789012',
      confirmPassword: '123456789012',
    });
    expect(JSON.stringify(renderer.toJSON())).toContain('127.0.0.1:9788');
    expect(renderer.root.findByProps({ name: 'lan-access-password' }).props.value).toBe('');
    renderer.unmount();
  });
});
