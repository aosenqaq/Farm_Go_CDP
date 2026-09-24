import { describe, expect, it } from 'vitest';

import {
  channelForTarget,
  normalizeStartupInjectionState,
  shouldDisplayStartupInjectionDialog,
} from './startupInjection';

describe('startup injection state', () => {
  it('labels each runtime channel separately', () => {
    expect(channelForTarget('qq_ws').title).toBe('QQ 补丁注入');
    expect(channelForTarget('wechat_cdp').title).toBe('微信 Frida 注入');
    expect(channelForTarget('yyb_cdp').title).toBe('应用宝 Frida 注入');
  });

  it('keeps the QQ startup patch dialog visible while the startup patch is running', () => {
    const state = normalizeStartupInjectionState(
      { target: 'qq_ws', phase: 'listening', connected: false, ready: false },
      { phase: 'scanning', injected: false, inFlight: true, lastTrigger: 'startup' },
    );

    expect(state.title).toBe('QQ 补丁注入');
    expect(state.headline).toBe('正在扫描 QQ 小程序补丁');
    expect(state.showRetry).toBe(true);
    expect(shouldDisplayStartupInjectionDialog(state, false)).toBe(true);
  });

  it('asks QQ users to restart the miniapp when a changed patch was written', () => {
    const state = normalizeStartupInjectionState(
      { target: 'qq_ws', phase: 'listening', connected: false, ready: false },
      { phase: 'ready', injected: true, action: 'patched', restartRequired: true },
    );

    expect(state.headline).toBe('QQ 小程序补丁已写入');
    expect(state.detail).toContain('启动 QQ 小程序');
    expect(state.showRetry).toBe(false);
    expect(state.showLaunch).toBe(true);
    expect(state.showRestart).toBe(false);
  });

  it('hides the QQ startup dialog once the runtime is connected and ready', () => {
    const state = normalizeStartupInjectionState(
      { target: 'qq_ws', phase: 'ready', connected: true, ready: true, instanceId: 'qq-farm' },
      { phase: 'ready', injected: true, action: 'already_latest' },
    );

    expect(state.headline).toBe('QQ链路已连接');
    expect(state.shouldShow).toBe(false);
    expect(shouldDisplayStartupInjectionDialog(state, false)).toBe(false);
  });

  it('prompts restart when QQ runtime is ready but the running hash differs from the patched hash', () => {
    const state = normalizeStartupInjectionState(
      { target: 'qq_ws', phase: 'ready', connected: true, ready: true, instanceId: 'qq-farm', scriptHash: 'old-hash' },
      { phase: 'ready', injected: true, action: 'patched', scriptHash: 'new-hash', restartRequired: true },
    );

    expect(state.headline).toBe('QQ 小程序补丁已写入');
    expect(state.detail).toContain('重新启动 QQ 小程序');
    expect(state.showRetry).toBe(false);
    expect(state.showLaunch).toBe(false);
    expect(state.showRestart).toBe(true);
    expect(shouldDisplayStartupInjectionDialog(state, false)).toBe(true);
  });

  it('asks QQ users to launch the miniapp when the patch is correct but the miniapp is not running', () => {
    const state = normalizeStartupInjectionState(
      { target: 'qq_ws', phase: 'listening', connected: false, ready: false },
      { phase: 'ready', injected: true, action: 'already_latest' },
    );

    expect(state.headline).toBe('QQ 小程序补丁已就绪');
    expect(state.detail).toContain('启动 QQ 小程序');
    expect(state.showRetry).toBe(false);
    expect(state.showLaunch).toBe(true);
    expect(state.showRestart).toBe(false);
    expect(shouldDisplayStartupInjectionDialog(state, false)).toBe(true);
  });

  it('shows reinjection first when QQ is running with an incorrect patch', () => {
    const state = normalizeStartupInjectionState(
      { target: 'qq_ws', phase: 'handshaking', connected: true, ready: false },
      { phase: 'error', injected: false, error: '自动补丁写入后校验失败' },
    );

    expect(state.headline).toBe('自动注入失败');
    expect(state.showRetry).toBe(true);
    expect(state.showLaunch).toBe(false);
    expect(state.showRestart).toBe(false);
  });

  it('asks QQ users to restart after a changed patch was written while QQ is running', () => {
    const state = normalizeStartupInjectionState(
      { target: 'qq_ws', phase: 'handshaking', connected: true, ready: false },
      { phase: 'ready', injected: true, action: 'patched', restartRequired: true },
    );

    expect(state.headline).toBe('QQ 小程序补丁已写入');
    expect(state.detail).toContain('重新启动 QQ 小程序');
    expect(state.showRetry).toBe(false);
    expect(state.showLaunch).toBe(false);
    expect(state.showRestart).toBe(true);
  });

  it('prompts WeChat users to open the miniapp until the WMPF context is ready', () => {
    const state = normalizeStartupInjectionState(
      { target: 'wechat_cdp', phase: 'listening', connected: false, ready: false },
      null,
    );

    expect(state.title).toBe('微信 Frida 注入');
    expect(state.headline).toBe('等待微信小程序连接');
    expect(state.detail).toContain('重新进入微信 QQ 农场小程序');
    expect(state.showRetry).toBe(false);
    expect(state.showLaunch).toBe(true);
    expect(state.inFlight).toBe(false);
    expect(shouldDisplayStartupInjectionDialog(state, false)).toBe(true);
  });

  it('shows launch action for YYB while the miniapp is disconnected', () => {
    const state = normalizeStartupInjectionState(
      { target: 'yyb_cdp', phase: 'listening', connected: false, ready: false },
      null,
    );

    expect(state.title).toBe('应用宝 Frida 注入');
    expect(state.headline).toBe('等待应用宝小程序连接');
    expect(state.showLaunch).toBe(true);
    expect(state.inFlight).toBe(false);
    expect(state.showRestart).toBe(false);
  });

  it('hides launch action once WeChat miniapp connection is detected', () => {
    const state = normalizeStartupInjectionState(
      { target: 'wechat_cdp', phase: 'handshaking', connected: true, ready: false },
      null,
    );

    expect(state.showLaunch).toBe(false);
    expect(state.inFlight).toBe(true);
  });

  it('uses application-channel copy for YYB instead of WeChat copy', () => {
    const state = normalizeStartupInjectionState(
      { target: 'yyb_cdp', phase: 'handshaking', connected: true, ready: false },
      null,
    );

    expect(state.title).toBe('应用宝 Frida 注入');
    expect(state.headline).toBe('正在确认应用宝小程序上下文');
    expect(state.detail).toContain('应用宝 QQ 农场小程序');
    expect(state.showLaunch).toBe(false);
  });

  it('presents the WeChat setupContext fallback as handshake progress', () => {
    const state = normalizeStartupInjectionState(
      {
        target: 'wechat_cdp',
        phase: 'handshaking',
        connected: true,
        ready: false,
        progressDetail: 'miniapp setupContext received; using direct CDP Runtime.enable fallback',
      },
      null,
    );

    expect(state.headline).toBe('正在建立可执行上下文');
    expect(state.tone).toBe('info');
    expect(state.detail).toContain('兼容 CDP 路径');
    expect(state.connectionMethod).toContain('Runtime.enable');
    expect(state.error).toBeUndefined();
    expect(state.showLaunch).toBe(false);
  });

  it('uses the same setupContext progress treatment for YYB', () => {
    const state = normalizeStartupInjectionState(
      {
        target: 'yyb_cdp',
        phase: 'handshaking',
        connected: true,
        ready: false,
        progressDetail: 'miniapp setupContext received; using direct CDP Runtime.enable fallback',
      },
      null,
    );

    expect(state.headline).toBe('正在建立可执行上下文');
    expect(state.detail).toContain('应用宝');
    expect(state.connectionMethod).toContain('Runtime.enable');
    expect(state.showLaunch).toBe(false);
  });

  it('does not show the QQ patch dialog before the persisted target is loaded', () => {
    const state = normalizeStartupInjectionState(
      { target: '', phase: 'idle', connected: false, ready: false },
      null,
    );

    expect(state.target).toBe('');
    expect(state.title).toBe('链路加载中');
    expect(state.showRetry).toBe(false);
    expect(shouldDisplayStartupInjectionDialog(state, false)).toBe(false);
  });
});
