import { renderToStaticMarkup } from 'react-dom/server';
import TestRenderer from 'react-test-renderer';
import { describe, expect, it } from 'vitest';

import { StartupInjectionDialog } from './StartupInjectionDialog';

describe('StartupInjectionDialog', () => {
  it('marks launch and restart actions as busy while their operations are pending', () => {
    const scenarios = [
      { launching: true, restarting: false, showLaunch: true, showRestart: false, label: '启动中' },
      { launching: false, restarting: true, showLaunch: false, showRestart: true, label: '重启中' },
    ];

    for (const scenario of scenarios) {
      const renderer = TestRenderer.create(
        <StartupInjectionDialog
          open
          state={{
            target: 'qq_ws',
            title: 'QQ 补丁注入',
            headline: 'QQ 小程序补丁已就绪',
            detail: '等待启动操作完成。',
            tone: 'success',
            primaryMetricLabel: '补丁状态',
            primaryMetricValue: '已就绪',
            secondaryMetricLabel: '目标',
            secondaryMetricValue: 'game.js',
            tertiaryMetricLabel: 'Host 版本',
            tertiaryMetricValue: '1.0.0',
            showRetry: false,
            showLaunch: scenario.showLaunch,
            showRestart: scenario.showRestart,
            inFlight: false,
            shouldShow: true,
          }}
          targetPath=""
          patching={false}
          launching={scenario.launching}
          restarting={scenario.restarting}
          onTargetPathChange={() => undefined}
          onRetry={() => undefined}
          onLaunch={() => undefined}
          onRestart={() => undefined}
          onHide={() => undefined}
        />,
      );

      const busyButton = renderer.root.findByProps({ 'aria-busy': true });
      expect(busyButton.props.className).toContain('async-action-button');
      expect(busyButton.props.disabled).toBe(true);
      expect(busyButton.findAllByProps({ className: 'spin' })).toHaveLength(1);
      expect(busyButton.findAllByType('span').map((node) => node.children.join(''))).toContain(scenario.label);
      renderer.unmount();
    }
  });

  it('renders QQ patch dialog fields', () => {
    const html = renderToStaticMarkup(
      <StartupInjectionDialog
        open
        state={{
          target: 'qq_ws',
          title: 'QQ 补丁注入',
          headline: '等待 QQ 小程序补丁注入',
          detail: '正在自动查找最近打开的 QQ 小程序 game.js 并写入调试补丁。',
          tone: 'warning',
          primaryMetricLabel: '补丁状态',
          primaryMetricValue: '扫描中',
          secondaryMetricLabel: '目标',
          secondaryMetricValue: 'game.js',
          tertiaryMetricLabel: 'Host 版本',
          tertiaryMetricValue: '1.0.0',
          showRetry: true,
          showLaunch: false,
          showRestart: false,
          inFlight: false,
          shouldShow: true,
        }}
        targetPath=""
        patching={false}
        onTargetPathChange={() => undefined}
        onRetry={() => undefined}
        onHide={() => undefined}
      />,
    );

    expect(html).toContain('QQ 补丁注入');
    expect(html).toContain('重新注入');
    expect(html).toContain('手动兜底路径');
    expect(html).toContain('本次隐藏');

    const actionsIndex = html.indexOf('injection-actions');
    const retryBlockIndex = html.indexOf('injection-retry');
    const pathIndex = html.indexOf('手动兜底路径');
    const footerSlice = html.slice(actionsIndex);
    expect(actionsIndex).toBeGreaterThan(-1);
    expect(retryBlockIndex).toBeGreaterThan(-1);
    expect(pathIndex).toBeGreaterThan(retryBlockIndex);
    expect(pathIndex).toBeLessThan(actionsIndex);
    expect(footerSlice).toContain('重新注入');
    expect(footerSlice).toContain('本次隐藏');
    expect(footerSlice.indexOf('重新注入')).toBeLessThan(footerSlice.indexOf('本次隐藏'));
  });

  it('renders a restart action when QQ patch changed', () => {
    const html = renderToStaticMarkup(
      <StartupInjectionDialog
        open
        state={{
          target: 'qq_ws',
          title: 'QQ 补丁注入',
          headline: 'QQ 小程序补丁已写入',
          detail: '需要重启 QQ 小程序加载新补丁。',
          tone: 'success',
          primaryMetricLabel: '补丁状态',
          primaryMetricValue: '已就绪',
          secondaryMetricLabel: '目标',
          secondaryMetricValue: 'game.js',
          tertiaryMetricLabel: 'Host 版本',
          tertiaryMetricValue: '1.0.0',
          showRetry: false,
          showLaunch: false,
          showRestart: true,
          inFlight: false,
          shouldShow: true,
        }}
        targetPath=""
        patching={false}
        restarting={false}
        onTargetPathChange={() => undefined}
        onRetry={() => undefined}
        onRestart={() => undefined}
        onHide={() => undefined}
      />,
    );

    expect(html).toContain('重新启动');
    expect(html).not.toContain('重新注入');
  });

  it('renders a launch action when QQ patch is correct but the miniapp is closed', () => {
    const html = renderToStaticMarkup(
      <StartupInjectionDialog
        open
        state={{
          target: 'qq_ws',
          title: 'QQ 补丁注入',
          headline: 'QQ 小程序补丁已就绪',
          detail: '补丁已正确写入，请启动 QQ 小程序。',
          tone: 'success',
          primaryMetricLabel: '补丁状态',
          primaryMetricValue: '已就绪',
          secondaryMetricLabel: '目标',
          secondaryMetricValue: 'game.js',
          tertiaryMetricLabel: 'Host 版本',
          tertiaryMetricValue: '1.0.0',
          showRetry: false,
          showLaunch: true,
          showRestart: false,
          inFlight: false,
          shouldShow: true,
        }}
        targetPath=""
        patching={false}
        launching={false}
        onTargetPathChange={() => undefined}
        onRetry={() => undefined}
        onLaunch={() => undefined}
        onHide={() => undefined}
      />,
    );

    expect(html).toContain('启动小程序');
    expect(html).not.toContain('重新注入');
  });

  it('renders WMPF guidance with a launch action while disconnected', () => {
    const html = renderToStaticMarkup(
      <StartupInjectionDialog
        open
        state={{
          target: 'wechat_cdp',
          title: '微信 Frida 注入',
          headline: '等待微信小程序连接',
          detail: '请打开或重新进入微信 QQ 农场小程序。',
          tone: 'warning',
          primaryMetricLabel: '链路',
          primaryMetricValue: '正在监听',
          secondaryMetricLabel: '小程序连接',
          secondaryMetricValue: '等待连接',
          tertiaryMetricLabel: '上下文',
          tertiaryMetricValue: '未就绪',
          showRetry: false,
          showLaunch: true,
          showRestart: false,
          inFlight: false,
          shouldShow: true,
        }}
        targetPath=""
        patching={false}
        launching={false}
        onTargetPathChange={() => undefined}
        onRetry={() => undefined}
        onLaunch={() => undefined}
        onHide={() => undefined}
      />,
    );

    expect(html).toContain('微信 Frida 注入');
    expect(html).toContain('启动小程序');
    expect(html).toContain('本次隐藏');
    expect(html).not.toContain('重新注入');
    expect(html).not.toContain('手动兜底路径');
  });

  it('renders CDP fallback as handshake progress instead of an error', () => {
    const html = renderToStaticMarkup(
      <StartupInjectionDialog
        open
        state={{
          target: 'wechat_cdp',
          title: '微信 Frida 注入',
          headline: '正在建立可执行上下文',
          detail: '已检测到微信 QQ 农场小程序连接，正在通过兼容 CDP 路径建立自动化连接。',
          tone: 'info',
          primaryMetricLabel: '链路',
          primaryMetricValue: '握手中',
          secondaryMetricLabel: '小程序连接',
          secondaryMetricValue: '已连接',
          tertiaryMetricLabel: '上下文',
          tertiaryMetricValue: '正在确认',
          connectionMethod: '直接 CDP Runtime.enable（兼容路径）',
          showRetry: false,
          showLaunch: false,
          showRestart: false,
          inFlight: true,
          shouldShow: true,
        }}
        targetPath=""
        patching={false}
        onTargetPathChange={() => undefined}
        onRetry={() => undefined}
        onHide={() => undefined}
      />,
    );

    expect(html).toContain('injection-progress');
    expect(html).toContain('连接方式');
    expect(html).toContain('直接 CDP Runtime.enable（兼容路径）');
    expect(html).not.toContain('错误');
  });
});
