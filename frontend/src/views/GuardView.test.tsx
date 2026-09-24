import { renderToString } from 'react-dom/server';
import TestRenderer, { act } from 'react-test-renderer';
// @ts-expect-error The frontend tsconfig intentionally omits Node types; this test reads component CSS.
import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

import { GuardView } from './GuardView';

describe('GuardView', () => {
  it('renders guard status and restart controls', () => {
    const html = renderToString(
      <GuardView
        status={{
          enabled: true,
          phase: 'watching',
          runtimeTarget: 'qq_ws',
          timeoutStreak: 1,
          restartCountInWindow: 0,
          maxRestartsPerWindow: 4,
          recentRestartEvents: [],
        }}
        bindingStatus={{ status: 'bound', binding: { pid: 42, processName: 'WeChatAppEx.exe' } }}
        onRefresh={() => undefined}
        onLaunch={() => undefined}
        onRestart={() => undefined}
      />,
    );
    expect(html).toContain('守护服务');
    expect(html).toContain('守护已开启');
    expect(html).toContain('自动绑定');
    expect(html).toContain('异常重启');
    expect(html).not.toContain('重启窗口');
    expect(html).toContain('重启当前小程序');
  });

  it('renders the unified guardian capability sections and event console', () => {
    const html = renderToString(
      <GuardView
        status={{
          enabled: true,
          phase: 'watching',
          runtimeTarget: 'qq_ws',
          restartCountInWindow: 0,
          maxRestartsPerWindow: 4,
          recentRestartEvents: [],
        }}
        bindingStatus={null}
        onRefresh={() => undefined}
        onLaunch={() => undefined}
        onRestart={() => undefined}
      />,
    );

    expect(html).toContain('守护服务');
    expect(html).toContain('进程异常与定时重启');
    expect(html).toContain('网络异常即时重连');
    expect(html).toContain('异地登录延时重连');
    expect(html).toContain('守护事件');
    expect(html).toContain('查看守护事件');
  });

  it('keeps guardian sections in a non-shrinking flex scroll column', () => {
    const html = renderToString(
      <GuardView
        status={{ phase: 'watching', restartCountInWindow: 0, maxRestartsPerWindow: 4, recentRestartEvents: [] }}
        onRefresh={() => undefined}
        onLaunch={() => undefined}
        onRestart={() => undefined}
      />,
    );

    expect(html).not.toContain('guard-scroll');
    const css = readFileSync(new URL('../style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
    expect(css).toMatch(/\.guard-stack\s*\{[^}]*display:\s*flex;[^}]*flex-direction:\s*column;[^}]*overflow-y:\s*auto;/);
    expect(css).toMatch(/\.guard-hero,\s*\.guard-capability-grid\s*\{[^}]*flex:\s*0\s+0\s+auto;/);
  });

  it('opens a scrollable dialog for persisted guardian events', () => {
    let renderer: TestRenderer.ReactTestRenderer;
    act(() => {
      renderer = TestRenderer.create(
        <GuardView
          status={{ phase: 'watching', restartCountInWindow: 0, maxRestartsPerWindow: 4, recentRestartEvents: [] }}
          events={[
            { id: 3, timestamp: '2026-07-11T12:03:00Z', level: 'info', source: 'guardian', type: 'guardian.other_place_login.reconnected', message: 'other place login reconnected', data: { runtimeTarget: 'qq_ws' } },
            { id: 2, timestamp: '2026-07-11T12:02:00Z', level: 'info', source: 'guardian', type: 'guardian.other_place_login.waiting', message: 'other place login waiting', data: { runtimeTarget: 'qq_ws' } },
            { id: 1, timestamp: '2026-07-11T12:01:00Z', level: 'info', source: 'guardian', type: 'guardian.network.detected', message: 'network detected', data: { runtimeTarget: 'qq_ws' } },
          ]}
          bindingStatus={null}
          onRefresh={() => undefined}
          onLaunch={() => undefined}
          onRestart={() => undefined}
        />,
      );
    });

    const closed = renderer!.toJSON();
    const closedText = JSON.stringify(closed);
    expect(closedText).toContain('守护事件');
    expect(closedText).not.toContain('other place login reconnected');

    const openButton = renderer!.root.find((node) =>
      node.type === 'button' && String(node.props.title || '') === '查看守护事件',
    );
    act(() => {
      openButton.props.onClick();
    });

    const openText = JSON.stringify(renderer!.toJSON());
    expect(openText).toContain('role":"dialog"');
    expect(openText).toContain('最近运行时观察与恢复结果，可滚动查看近期记录');
    expect(openText).toContain('other place login reconnected');
    expect(openText).toContain('network detected');
    expect(openText).not.toContain('"-","type":"time"');
  });

  it('renders an enable action when process guard is disabled', () => {
    const html = renderToString(
      <GuardView
        status={{
          enabled: false,
          phase: 'disabled',
          runtimeTarget: 'wechat_cdp',
          restartCountInWindow: 0,
          maxRestartsPerWindow: 4,
          recentRestartEvents: [],
        }}
        bindingStatus={null}
        onRefresh={() => undefined}
        onLaunch={() => undefined}
        onRestart={() => undefined}
        onToggleEnabled={() => undefined}
        toggling={false}
      />,
    );

    expect(html).toContain('守护已关闭');
    expect(html).toContain('启用守护');
  });

  it('does not render QQ WS before the guard runtime target is synchronized', () => {
    const html = renderToString(
      <GuardView
        status={{
          phase: 'standby',
          runtimeTarget: '',
          restartCountInWindow: 0,
          maxRestartsPerWindow: 4,
          recentRestartEvents: [],
        }}
        bindingStatus={null}
        onRefresh={() => undefined}
        onLaunch={() => undefined}
        onRestart={() => undefined}
        onToggleEnabled={() => undefined}
      />,
    );

    expect(html).toContain('待同步');
    expect(html).not.toContain('QQ WS');
  });

  it('does not expose host candidate process rows in the guard interface', () => {
    const html = renderToString(
      <GuardView
        status={{
          phase: 'watching',
          runtimeTarget: 'wechat_cdp',
          restartCountInWindow: 0,
          maxRestartsPerWindow: 4,
          recentRestartEvents: [],
        }}
        bindingStatus={{ status: 'bound', binding: { pid: 42, processName: 'WeChatAppEx.exe' } }}
        onRefresh={() => undefined}
        onLaunch={() => undefined}
        onRestart={() => undefined}
        onToggleEnabled={() => undefined}
      />,
    );

    expect(html).not.toContain('宿主候选');
    expect(html).not.toContain('未捕获窗口标题');
  });

  it('renders manual and automatic restart events in the recorder dialog', () => {
    let renderer: TestRenderer.ReactTestRenderer;
    act(() => {
      renderer = TestRenderer.create(
        <GuardView
          status={{
            phase: 'watching',
            runtimeTarget: 'wechat_cdp',
            restartCountInWindow: 2,
            maxRestartsPerWindow: 4,
            recentRestartEvents: [
              {
                at: '2026-07-07T03:00:00Z',
                runtimeTarget: 'wechat_cdp',
                reason: 'manual restart',
                trigger: 'manual',
                ok: true,
              },
              {
                at: '2026-07-07T03:01:00Z',
                runtimeTarget: 'wechat_cdp',
                reason: 'context deadline exceeded',
                trigger: 'auto',
                ok: true,
              },
            ],
          }}
          bindingStatus={{ status: 'bound', binding: { pid: 42, processName: 'WeChatAppEx.exe' } }}
          onRefresh={() => undefined}
          onLaunch={() => undefined}
          onRestart={() => undefined}
          onToggleEnabled={() => undefined}
        />,
      );
    });

    const openButton = renderer!.root.find((node) =>
      node.type === 'button' && String(node.props.title || '') === '查看守护事件',
    );
    act(() => {
      openButton.props.onClick();
    });

    const openText = JSON.stringify(renderer!.toJSON());
    expect(openText).toContain('手动');
    expect(openText).toContain('自动');
    expect(openText).toContain('manual restart');
    expect(openText).toContain('context deadline exceeded');
  });


  it('renders the restart auto-minimize switch inside process advanced settings', () => {
    const html = renderToString(
      <GuardView
        status={{
          enabled: true, phase: 'watching', restartCountInWindow: 0,
          maxRestartsPerWindow: 4, recentRestartEvents: [],
          process: { autoMinimizeAfterRestart: true },
        }}
        onRefresh={() => undefined}
        onLaunch={() => undefined}
        onRestart={() => undefined}
        onSaveSettings={() => undefined}
      />,
    );
    expect(html).toContain('参数设置');
    expect(html).toContain('重启后自动最小化窗口');
    expect(html).toContain('guard-settings-toggle');
  });

  it('renders and saves scheduled restart settings inside process advanced settings', () => {
    const saves: Array<Record<string, unknown>> = [];
    let renderer: TestRenderer.ReactTestRenderer;
    act(() => {
      renderer = TestRenderer.create(
        <GuardView
          status={{
            enabled: true,
            phase: 'watching',
            restartCountInWindow: 0,
            maxRestartsPerWindow: 4,
            recentRestartEvents: [],
            process: { scheduledRestartEnabled: true },
            settings: { timeoutThreshold: 3, monitorIntervalMs: 3000, scheduledRestartIntervalMin: 90 },
          }}
          onRefresh={() => undefined}
          onLaunch={() => undefined}
          onRestart={() => undefined}
          onSaveSettings={(input) => saves.push(input)}
        />,
      );
    });

    const text = JSON.stringify(renderer!.toJSON());
    expect(text).toContain('定时重启');
    expect(text).toContain('重启间隔');

    const inputs = renderer!.root.findAllByType('input');
    const intervalInput = inputs.find((input) => input.props.defaultValue === 90);
    expect(intervalInput).toBeDefined();
    const toggleRow = renderer!.root.findAll((node) => node.props.className === 'guard-settings-toggle').find((row) =>
      row.findAllByType('span').some((span) => span.children.join('') === '定时重启'),
    );
    const toggle = toggleRow?.findByType('button');
    expect(toggle).toBeDefined();

    act(() => {
      toggle.props.onClick();
      intervalInput!.props.onBlur({ currentTarget: { value: '120' } });
    });

    expect(saves).toEqual([
      { scheduledRestartEnabled: false },
      { scheduledRestartIntervalMin: 120 },
    ]);
  });

  it('renders and saves persisted numeric settings for all guardian cards', () => {
    const saves: Array<Record<string, unknown>> = [];
    let renderer: TestRenderer.ReactTestRenderer;
    act(() => {
      renderer = TestRenderer.create(
        <GuardView
          status={{
            enabled: true,
            phase: 'watching',
            restartCountInWindow: 0,
            maxRestartsPerWindow: 4,
            recentRestartEvents: [],
            settings: {
              timeoutThreshold: 7,
              monitorIntervalMs: 4321,
              networkReconnectIntervalMs: 1300,
              networkRecoveryTimeoutMs: 18000,
              otherPlaceLoginIntervalMs: 4500,
              otherPlaceLoginDelayMin: 8,
            },
          }}
          onRefresh={() => undefined}
          onLaunch={() => undefined}
          onRestart={() => undefined}
          onSaveSettings={(input) => saves.push(input)}
        />,
      );
    });

    const inputs = renderer!.root.findAllByType('input');
    expect(inputs.map((input) => input.props.defaultValue)).toEqual([7, 4321, 60, 1300, 18000, 4500, 8]);

    const values = ['9', '5432', '75', '1400', '19000', '4600', '10'];
    act(() => {
      inputs.forEach((input, index) => input.props.onBlur({ currentTarget: { value: values[index] } }));
    });
    expect(saves).toEqual([
      { timeoutThreshold: 9 },
      { monitorIntervalMs: 5432 },
      { scheduledRestartIntervalMin: 75 },
      { networkReconnectIntervalMs: 1400 },
      { networkRecoveryTimeoutMs: 19000 },
      { otherPlaceLoginIntervalMs: 4600 },
      { otherPlaceLoginDelayMin: 10 },
    ]);
  });

});
