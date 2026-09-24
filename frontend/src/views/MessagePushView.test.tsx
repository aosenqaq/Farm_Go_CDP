import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import { MessagePushView } from './MessagePushView';
import messagePushViewSource from './MessagePushView.tsx?raw';

vi.mock('../../wailsjs/go/main/App', () => ({
  MessagePushState: vi.fn(() =>
    Promise.resolve({
      config: {
        enabled: true,
        abnormalEnabled: true,
        suspectedEnabled: true,
        recoveryEnabled: true,
        restartEnabled: true,
        dailyEnabled: false,
        logMonitorEnabled: true,
        dailyTime: '09:00',
        selectedChannels: ['webhook'],
        channelFormats: {},
        channels: { webhookUrl: 'https://example.test/hook', webhookMethod: 'POST' },
      },
      configuredChannels: ['webhook'],
      channels: [{ type: 'webhook', label: 'Webhook', configured: true }],
      daily: { enabled: false, time: '09:00' },
      recentPushes: [],
      logMonitorEnabled: true,
    }),
  ),
  SaveMessagePushConfig: vi.fn(),
  SendMessagePushTest: vi.fn(),
  SendMessagePushDailyTest: vi.fn(),
  PreviewMessagePushTemplate: vi.fn(),
  SendMessagePushTemplateTest: vi.fn(),
}));

describe('MessagePushView', () => {
  it('renders the six independent message type switches', () => {
    const html = renderToStaticMarkup(<MessagePushView />);
    expect(html).toContain('异常告警');
    expect(html).toContain('疑似异常');
    expect(html).toContain('恢复通知');
    expect(html).toContain('重启通知');
    expect(html).toContain('资产日报');
    expect(html).toContain('日志监控');
  });

  it('renders channel configuration and test actions', () => {
    const html = renderToStaticMarkup(<MessagePushView />);
    expect(html).toContain('推送渠道');
    expect(html).toContain('测试推送');
    expect(html).toContain('日报测试');
    expect(html).not.toContain('待迁移');
  });

  it('shows a right-bottom save success toast after saving config', () => {
    expect(messagePushViewSource).toContain('setSaveToastVisible(true)');
    expect(messagePushViewSource).toContain('<SaveSuccessToast visible={saveToastVisible} />');
  });

  it('renders the template workflow and log-monitor rule editor', () => {
    const html = renderToStaticMarkup(<MessagePushView />);
    expect(html).toContain('模板');
    expect(html).toContain('可用变量');
    expect(html).not.toContain('预览');
    expect(html).toContain('恢复默认');
    expect(html).toContain('测试当前模板');
  });

  it('uses the current-draft template test binding without a preview action', () => {
    expect(messagePushViewSource).not.toContain('PreviewMessagePushTemplate');
    expect(messagePushViewSource).not.toContain('预览');
    expect(messagePushViewSource).toContain('SendMessagePushTemplateTest');
    expect(messagePushViewSource).toContain('日志监控规则');
  });

  it('reads the generated template catalog channelModes contract', () => {
    expect(messagePushViewSource).toContain('channelModes');
    expect(messagePushViewSource).not.toContain('definition?.channels.find');
  });

  it('renders variable labels, descriptions, and examples', () => {
    expect(messagePushViewSource).toContain('variable.label');
    expect(messagePushViewSource).toContain('variable.description');
    expect(messagePushViewSource).toContain('variable.example');
  });
});
