import { renderToStaticMarkup } from 'react-dom/server';
// @ts-expect-error The frontend tsconfig intentionally omits Node types; this test reads component source only.
import { readFileSync } from 'node:fs';
import { act, create } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';

import { SAVE_TOAST_AUTO_DISMISS_MS, SaveSuccessToast, SettingsView } from './SettingsView';

vi.mock('../../wailsjs/go/desktop/App', () => ({
  CleanQQMiniappCache: vi.fn(),
  CleanWeChatCache: vi.fn(),
  CleanYYBMiniappCache: vi.fn(),
  PreviewWeChatCacheCleanup: vi.fn(),
  RuntimeSettings: vi.fn().mockResolvedValue({}),
  SaveRuntimeSettings: vi.fn(),
  SwitchRuntimeTarget: vi.fn(),
  LANAccessSettings: vi.fn().mockResolvedValue({}),
  SaveLANAccessSettings: vi.fn(),
}));

describe('SettingsView', () => {
  it('keeps system settings separate from diagnostics and logs', () => {
    const html = renderToStaticMarkup(<SettingsView events={[]} onRefreshEvents={() => undefined} />);

    expect(html).toContain('系统设置');
    expect(html).not.toContain('诊断');
    expect(html).not.toContain('系统日志');
    expect(html).not.toContain('日志中心');
    expect(html).not.toContain('CDP 代理端口');
    expect(html).not.toContain('WMPF 调试端口');
    expect(html).not.toContain('QQ WS 兼容');
    expect(html).not.toContain('/runtime/qqws');
  });

  it('renders the three cache maintenance actions in system settings', () => {
    const html = renderToStaticMarkup(<SettingsView events={[]} onRefreshEvents={() => undefined} />);

    expect(html).toContain('缓存维护');
    expect(html).toContain('清理微信缓存');
    expect(html).toContain('清理 QQ 缓存');
    expect(html).toContain('清理应用宝缓存');
    expect(html).toContain('title="清理微信小程序缓存"');
    expect(html).toContain('title="清理 QQ 小程序缓存"');
    expect(html).toContain('title="清理应用宝小程序缓存"');
    expect(html).not.toContain('>清理微信小程序缓存</span>');
    expect(html).not.toContain('>清理 QQ 小程序缓存</span>');
    expect(html).not.toContain('>清理应用宝小程序缓存</span>');
  });

  it('includes the LAN web console settings panel', () => {
    const html = renderToStaticMarkup(<SettingsView events={[]} onRefreshEvents={() => undefined} />);

    expect(html).toContain('局域网 Web 控制台');
    expect(html).toContain('name="save-lan-access-settings"');
  });

  it('renders system settings with a fixed header and scrollable body region', () => {
    const html = renderToStaticMarkup(<SettingsView events={[]} onRefreshEvents={() => undefined} />);
    const source = readFileSync(new URL('../style.css', import.meta.url), 'utf8');

    expect(html).toContain('settings-view');
    expect(html).toContain('settings-scroll');
    expect(source).toContain('.settings-view');
    expect(source).toContain('.settings-scroll');
    expect(source).toContain('overflow-y: auto');
    expect(source).toMatch(/\.app-view-enter \{[^}]*\n\s+height: 100%;/);
  });

  it('uses a WeChat preview before the final cleanup confirmation', () => {
    const source = readFileSync(new URL('./SettingsView.tsx', import.meta.url), 'utf8');

    expect(source).toContain('PreviewWeChatCacheCleanup');
    expect(source).toContain('CleanWeChatCache');
    expect(source).toContain('将移动');
  });

  it('requires stronger confirmation for QQ and exposes YYB cleanup', () => {
    const source = readFileSync(new URL('./SettingsView.tsx', import.meta.url), 'utf8');

    expect(source).toContain('再次确认');
    expect(source).toContain('CleanQQMiniappCache');
    expect(source).toContain('CleanYYBMiniappCache');
  });

  it('renders the save success toast as a fixed status notification', () => {
    const html = renderToStaticMarkup(<SaveSuccessToast visible />);

    expect(html).toContain('settings-save-toast');
    expect(html).toContain('role="status"');
    expect(html).toContain('保存成功');
    expect(SAVE_TOAST_AUTO_DISMISS_MS).toBe(2600);
  });

  it('triggers an auto-dismiss success toast after settings are saved', () => {
    const source = readFileSync(new URL('./SettingsView.tsx', import.meta.url), 'utf8');

    expect(source).toContain('setSaveToastVisible(true)');
    expect(source).toContain('SAVE_TOAST_AUTO_DISMISS_MS');
    expect(source).toContain('<SaveSuccessToast visible={saveToastVisible} />');
  });

  it('shows the latest status only after a manual update check', async () => {
    const renderer = create(
      <SettingsView
        events={[]}
        onRefreshEvents={() => undefined}
        onCheckUpdates={async () => ({
          current: { number: 3, name: 'v3.0.0' },
          latest: { number: 3, name: 'v3.0.0' },
          available: false,
        })}
      />,
    );

    expect(renderer.toJSON()).not.toContain('已是最新版本');
    await act(async () => {
      await renderer.root.findByProps({ name: 'check-updates' }).props.onClick();
    });

    expect(JSON.stringify(renderer.toJSON())).toContain('已是最新版本');
  });

  it('reports an available update from a manual check to the root dialog owner', async () => {
    const onUpdateAvailable = vi.fn();
    const update = {
      current: { number: 3, name: 'v3.0.0' },
      latest: { number: 4, name: 'v4.0.0', description: '修复稳定性问题', downloadUrl: 'https://example.test/farm-go' },
      available: true,
    };
    const renderer = create(
      <SettingsView
        events={[]}
        onRefreshEvents={() => undefined}
        onCheckUpdates={async () => update}
        onUpdateAvailable={onUpdateAvailable}
      />,
    );

    await act(async () => {
      await renderer.root.findByProps({ name: 'check-updates' }).props.onClick();
    });

    expect(onUpdateAvailable).toHaveBeenCalledWith(update);
    expect(JSON.stringify(renderer.toJSON())).not.toContain('open-update-download');
  });

  it('shows update errors only from a manual check', async () => {
    const onUpdateAvailable = vi.fn();
    const renderer = create(
      <SettingsView
        events={[]}
        onRefreshEvents={() => undefined}
        onCheckUpdates={async () => ({
          current: { number: 3, name: 'v3.0.0' },
          latest: { number: 0, name: '' },
          available: false,
          errorCode: 'update_unavailable',
          message: '更新服务暂不可用',
        })}
        onUpdateAvailable={onUpdateAvailable}
      />,
    );

    await act(async () => {
      await renderer.root.findByProps({ name: 'check-updates' }).props.onClick();
    });

    expect(JSON.stringify(renderer.toJSON())).toContain('更新服务暂不可用');
    expect(onUpdateAvailable).not.toHaveBeenCalled();
    expect(JSON.stringify(renderer.toJSON())).not.toContain('open-update-download');
  });

  it('does not report latest or errored results to the update dialog owner', async () => {
    const onUpdateAvailable = vi.fn();
    const renderer = create(
      <SettingsView
        events={[]}
        onRefreshEvents={() => undefined}
        onCheckUpdates={async () => ({
          current: { number: 3, name: 'v3.0.0' },
          latest: { number: 3, name: 'v3.0.0' },
          available: false,
        })}
        onUpdateAvailable={onUpdateAvailable}
      />,
    );

    await act(async () => {
      await renderer.root.findByProps({ name: 'check-updates' }).props.onClick();
    });

    expect(onUpdateAvailable).not.toHaveBeenCalled();
    expect(JSON.stringify(renderer.toJSON())).not.toContain('open-update-download');
  });

  it('renders default scheduled update controls in the update panel', () => {
    const html = renderToStaticMarkup(
      <SettingsView
        events={[]}
        onRefreshEvents={() => undefined}
        updateCheckPreferences={{ enabled: true, intervalMinutes: 120 }}
      />,
    );

    expect(html).toContain('定时检查更新');
    expect(html).toContain('name="scheduled-update-enabled"');
    expect(html).toContain('name="scheduled-update-interval"');
    expect(html).toContain('value="120"');
    expect(html).toContain('分钟/次');
    expect(html).toContain('name="save-update-preferences"');
  });

  it('disables the interval input when scheduled checks are turned off', () => {
    const renderer = create(
      <SettingsView
        events={[]}
        onRefreshEvents={() => undefined}
        updateCheckPreferences={{ enabled: true, intervalMinutes: 120 }}
      />,
    );

    act(() => {
      renderer.root.findByProps({ name: 'scheduled-update-enabled' }).props.onChange({ target: { checked: false } });
    });

    expect(renderer.root.findByProps({ name: 'scheduled-update-interval' }).props.disabled).toBe(true);
  });

  it.each(['0', '10081', '1.5'])('rejects invalid scheduled update interval %s', async (value) => {
    const onSaveUpdateCheckPreferences = vi.fn();
    const renderer = create(
      <SettingsView
        events={[]}
        onRefreshEvents={() => undefined}
        updateCheckPreferences={{ enabled: true, intervalMinutes: 120 }}
        onSaveUpdateCheckPreferences={onSaveUpdateCheckPreferences}
      />,
    );

    act(() => {
      renderer.root.findByProps({ name: 'scheduled-update-interval' }).props.onChange({ target: { value } });
    });
    await act(async () => {
      await renderer.root.findByProps({ name: 'save-update-preferences' }).props.onClick();
    });

    expect(JSON.stringify(renderer.toJSON())).toContain('请输入 1 到 10080 之间的整数分钟数');
    expect(onSaveUpdateCheckPreferences).not.toHaveBeenCalled();
  });

  it('saves valid scheduled update preferences independently', async () => {
    const onSaveUpdateCheckPreferences = vi.fn(async (value) => value);
    const renderer = create(
      <SettingsView
        events={[]}
        onRefreshEvents={() => undefined}
        updateCheckPreferences={{ enabled: true, intervalMinutes: 120 }}
        onSaveUpdateCheckPreferences={onSaveUpdateCheckPreferences}
      />,
    );

    act(() => {
      renderer.root.findByProps({ name: 'scheduled-update-interval' }).props.onChange({ target: { value: '30' } });
    });
    await act(async () => {
      await renderer.root.findByProps({ name: 'save-update-preferences' }).props.onClick();
    });

    expect(onSaveUpdateCheckPreferences).toHaveBeenCalledWith({ enabled: true, intervalMinutes: 30 });
    expect(JSON.stringify(renderer.toJSON())).toContain('定时检查设置已保存');
  });

  it('shows scheduled update preference save failures inline', async () => {
    const renderer = create(
      <SettingsView
        events={[]}
        onRefreshEvents={() => undefined}
        updateCheckPreferences={{ enabled: true, intervalMinutes: 120 }}
        onSaveUpdateCheckPreferences={async () => {
          throw new Error('保存定时检查设置失败');
        }}
      />,
    );

    await act(async () => {
      await renderer.root.findByProps({ name: 'save-update-preferences' }).props.onClick();
    });

    expect(JSON.stringify(renderer.toJSON())).toContain('保存定时检查设置失败');
  });
});
