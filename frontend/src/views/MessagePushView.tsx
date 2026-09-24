import { Activity, AlertTriangle, Bell, Braces, Clock, Plus, Radio, RefreshCw, RotateCcw, Save, Send, ShieldCheck, Trash2 } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';

import * as MessagePushBindings from '../../wailsjs/go/desktop/App';
import {
  MessagePushState,
  SaveMessagePushConfig,
  SendMessagePushDailyTest,
  SendMessagePushTest,
} from '../../wailsjs/go/desktop/App';
import { messagepush } from '../../wailsjs/go/models';
import { SAVE_TOAST_AUTO_DISMISS_MS, SaveSuccessToast } from '../components/SaveSuccessToast';

type ChannelConfig = {
  serverChanSendKey: string;
  pushPlusToken: string;
  qmsgKey: string;
  qmsgType: string;
  qmsgTarget: string;
  wecomWebhook: string;
  dingtalkWebhook: string;
  dingtalkSecret: string;
  feishuWebhook: string;
  telegramBotToken: string;
  telegramChatId: string;
  barkServerUrl: string;
  barkDeviceKey: string;
  ntfyServerUrl: string;
  ntfyTopic: string;
  webhookUrl: string;
  webhookMethod: string;
  webhookHeaders: string;
};

type MessagePushConfig = {
  enabled: boolean;
  abnormalEnabled: boolean;
  suspectedEnabled: boolean;
  recoveryEnabled: boolean;
  dailyEnabled: boolean;
  dailyMarkdownCardEnabled: boolean;
  restartEnabled: boolean;
  dailyTime: string;
  logMonitorEnabled: boolean;
  abnormalTimeoutThreshold: number;
  logScanIntervalSec: number;
  httpTimeoutMs: number;
  pushRetryCount: number;
  selectedChannels: string[];
  channelFormats: Record<string, string>;
  channels: ChannelConfig;
  templates: Record<string, Record<string, MessageTemplate>>;
  logMonitorRules: LogMonitorRule[];
};

type MessageTemplate = {
  enabled: boolean;
  mode: string;
  content: string;
};

type LogMonitorRule = {
  enabled: boolean;
  source: string;
  type: string;
  keyword: string;
};

type TemplateDefinition = {
  type: string;
  label: string;
  variables?: TemplateVariable[];
  channelModes: Array<{ channel: string; modes: string[] }>;
};

type TemplateVariable = {
  path: string;
  label: string;
  description: string;
  example: string;
};

type MessagePushViewState = {
  enabled: boolean;
  config: MessagePushConfig;
  configuredChannels: string[];
  channels: Array<{ type: string; label: string; configured: boolean }>;
  daily: {
    enabled: boolean;
    channels?: string[];
    time: string;
    nextRunAt?: string;
    lastSummaryDateKey?: string;
    lastSummaryAt?: string;
    lastCheckedAt?: string;
    lastSkipReason?: string;
  };
  recentPushes: Array<{ time: string; kind: string; title: string; ok: boolean; channels: string[]; error?: string }>;
  logMonitorEnabled: boolean;
  templateCatalog?: TemplateDefinition[];
};

type SendResult = {
  ok: boolean;
  summary: string;
};

type ConfigKey =
  | 'abnormalEnabled'
  | 'suspectedEnabled'
  | 'recoveryEnabled'
  | 'restartEnabled'
  | 'dailyEnabled'
  | 'logMonitorEnabled';

const channelLabels: Record<string, string> = {
  serverchan: 'Server酱',
  pushplus: 'PushPlus',
  qmsg: 'Qmsg酱',
  wecom: '企业微信',
  dingtalk: '钉钉',
  feishu: '飞书',
  telegram: 'Telegram',
  bark: 'Bark',
  ntfy: 'ntfy',
  webhook: 'Webhook',
};

const typeSwitches: Array<{ key: ConfigKey; label: string; detail: string; icon: typeof AlertTriangle }> = [
  { key: 'abnormalEnabled', label: '异常告警', detail: '运行中断、接口失败、长时间无响应', icon: AlertTriangle },
  { key: 'suspectedEnabled', label: '疑似异常', detail: '可疑超时、状态抖动、需要关注', icon: Activity },
  { key: 'recoveryEnabled', label: '恢复通知', detail: '异常解除、链路恢复、状态回稳', icon: ShieldCheck },
  { key: 'restartEnabled', label: '重启通知', detail: '守护进程触发或手动重启完成', icon: RefreshCw },
  { key: 'dailyEnabled', label: '资产日报', detail: '按固定时间推送每日摘要', icon: Clock },
  { key: 'logMonitorEnabled', label: '日志监控', detail: '关键日志模式命中后的提示', icon: Radio },
];

const commonTemplateVariables: TemplateVariable[] = [
  { path: 'event.time', label: '发生时间', description: '消息事件在 Farm_Go 中被记录的北京时间。', example: '2026-07-12 09:30:00' },
  { path: 'event.type', label: '事件类型', description: '触发本条消息的业务类型。', example: 'abnormal' },
  { path: 'account.gid', label: '运行账户 GID', description: '当前运行账户已确认的 GID。', example: '10001' },
  { path: 'runtime.target', label: '运行目标', description: '发生事件的运行链路或宿主目标。', example: 'qq_ws' },
];

const fallbackTemplateCatalog: TemplateDefinition[] = [
  { type: 'abnormal', label: '异常告警', variables: commonTemplateVariables, channelModes: [] },
  { type: 'suspected', label: '疑似异常', variables: commonTemplateVariables, channelModes: [] },
  { type: 'recovery', label: '恢复通知', variables: commonTemplateVariables, channelModes: [] },
  { type: 'restart', label: '重启通知', variables: commonTemplateVariables, channelModes: [] },
  { type: 'daily', label: '资产日报', variables: [...commonTemplateVariables, { path: 'daily.date', label: '日报日期', description: '本次日报对应的北京日期。', example: '2026-07-12' }, { path: 'daily.summary', label: '日报摘要', description: '日报或测试生成的摘要正文。', example: '今日推送模块已就绪。' }], channelModes: [] },
  { type: 'log_monitor', label: '日志监控', variables: commonTemplateVariables, channelModes: [] },
];

const fallbackTemplate: MessageTemplate = {
  enabled: true,
  mode: 'text',
  content: '{{event.type}} | {{event.time}}\n{{runtime.target}}',
};

const templateBindings = MessagePushBindings as unknown as {
  DefaultMessagePushTemplate: (messageType: string, channel: string) => Promise<MessageTemplate>;
  SendMessagePushTemplateTest: (config: messagepush.Config, messageType: string, channel: string) => Promise<SendResult>;
};

const defaultConfig: MessagePushConfig = {
  enabled: false,
  abnormalEnabled: true,
  suspectedEnabled: true,
  recoveryEnabled: true,
  dailyEnabled: false,
  dailyMarkdownCardEnabled: false,
  restartEnabled: true,
  dailyTime: '09:00',
  logMonitorEnabled: true,
  abnormalTimeoutThreshold: 5,
  logScanIntervalSec: 30,
  httpTimeoutMs: 10000,
  pushRetryCount: 3,
  selectedChannels: ['webhook'],
  channelFormats: {},
  templates: {},
  logMonitorRules: [],
  channels: {
    serverChanSendKey: '',
    pushPlusToken: '',
    qmsgKey: '',
    qmsgType: '',
    qmsgTarget: '',
    wecomWebhook: '',
    dingtalkWebhook: '',
    dingtalkSecret: '',
    feishuWebhook: '',
    telegramBotToken: '',
    telegramChatId: '',
    barkServerUrl: 'https://api.day.app',
    barkDeviceKey: '',
    ntfyServerUrl: 'https://ntfy.sh',
    ntfyTopic: '',
    webhookUrl: '',
    webhookMethod: 'POST',
    webhookHeaders: '',
  },
};

const defaultState: MessagePushViewState = {
  enabled: false,
  config: defaultConfig,
  configuredChannels: [],
  channels: Object.entries(channelLabels).map(([type, label]) => ({ type, label, configured: false })),
  daily: { enabled: false, channels: [], time: '09:00' },
  recentPushes: [],
  logMonitorEnabled: true,
  templateCatalog: fallbackTemplateCatalog,
};

export function MessagePushView() {
  const [state, setState] = useState<MessagePushViewState>(defaultState);
  const [draft, setDraft] = useState<MessagePushConfig>(defaultConfig);
  const [busy, setBusy] = useState<string>('');
  const [notice, setNotice] = useState('');
  const [saveToastVisible, setSaveToastVisible] = useState(false);
  const [selectedTemplateType, setSelectedTemplateType] = useState('abnormal');
  const [selectedTemplateChannel, setSelectedTemplateChannel] = useState('webhook');
  const [templateNotice, setTemplateNotice] = useState('');
  const selectedChannel = draft.selectedChannels?.[0] || 'webhook';

  useEffect(() => {
    let alive = true;
    MessagePushState()
      .then((next) => {
        if (!alive) {
          return;
        }
        const viewState = next as unknown as MessagePushViewState;
        setState(viewState);
        setDraft(mergeConfig(viewState.config));
      })
      .catch((err) => {
        if (alive) {
          setNotice(`读取推送状态失败：${String(err)}`);
        }
      });
    return () => {
      alive = false;
    };
  }, []);

  useEffect(() => {
    if (!saveToastVisible) return;
    const timer = window.setTimeout(() => setSaveToastVisible(false), SAVE_TOAST_AUTO_DISMISS_MS);
    return () => window.clearTimeout(timer);
  }, [saveToastVisible]);

  const configured = useMemo(() => new Set(state.configuredChannels || []), [state.configuredChannels]);
  const templateCatalog = state.templateCatalog?.length ? state.templateCatalog : fallbackTemplateCatalog;
  const templateDefinition = templateCatalog.find((item) => item.type === selectedTemplateType) || templateCatalog[0];
  const templateChannels = configured.size > 0 ? [...configured] : Object.keys(channelLabels);
  const activeTemplateChannel = templateChannels.includes(selectedTemplateChannel) ? selectedTemplateChannel : templateChannels[0] || 'webhook';
  const templateModes = supportedTemplateModes(templateDefinition, activeTemplateChannel);
  const activeTemplate = templateFor(draft, selectedTemplateType, activeTemplateChannel, templateModes[0]);

  async function save() {
    if (draft.logMonitorRules.some((rule) => !hasRuleConstraint(rule))) {
      setNotice('日志监控规则至少需要来源、类型或关键词中的一项');
      return;
    }
    await runAction('save', async () => {
      const next = await SaveMessagePushConfig(draft as unknown as messagepush.Config);
      setState(next as unknown as MessagePushViewState);
      setDraft(mergeConfig((next as unknown as MessagePushViewState).config));
      return '配置已保存';
    });
  }

  async function sendTest(kind: 'test' | 'daily') {
    await runAction(kind, async () => {
      const wailsConfig = draft as unknown as messagepush.Config;
      const result =
        kind === 'daily' ? await SendMessagePushDailyTest(wailsConfig) : await SendMessagePushTest(wailsConfig);
      await refreshState();
      return resultText(result);
    });
  }

  async function testTemplate() {
    await runAction('template-test', async () => {
      const result = await templateBindings.SendMessagePushTemplateTest(
        draft as unknown as messagepush.Config,
        selectedTemplateType,
        activeTemplateChannel,
      );
      setTemplateNotice(resultText(result));
      return resultText(result);
    });
  }

  function updateTemplate(updater: (template: MessageTemplate) => MessageTemplate) {
    setTemplateNotice('');
    setDraft((next) => ({
      ...next,
      templates: {
        ...next.templates,
        [selectedTemplateType]: {
          ...next.templates?.[selectedTemplateType],
          [activeTemplateChannel]: updater(templateFor(next, selectedTemplateType, activeTemplateChannel, templateModes[0])),
        },
      },
    }));
  }

  async function restoreTemplateDefault() {
    setBusy('template-default');
    try {
      const template = await templateBindings.DefaultMessagePushTemplate(selectedTemplateType, activeTemplateChannel);
      updateTemplate(() => template);
      setTemplateNotice('已恢复内置默认模板，保存后生效');
    } catch (err) {
      setTemplateNotice(`恢复默认模板失败：${String(err)}`);
    } finally {
      setBusy('');
    }
  }

  async function refreshState() {
    const next = (await MessagePushState()) as unknown as MessagePushViewState;
    setState(next);
    setDraft(mergeConfig(next.config));
  }

  async function runAction(action: string, task: () => Promise<string>) {
    setBusy(action);
    setNotice('');
    try {
      setNotice(await task());
      if (action === 'save') {
        setSaveToastVisible(true);
      }
    } catch (err) {
      setNotice(String(err));
      if (action === 'save') {
        setSaveToastVisible(false);
      }
    } finally {
      setBusy('');
    }
  }

  return (
    <section className="message-push-view view-stack fill">
      <header className="page-header message-push-header">
        <div>
          <h1>消息推送</h1>
          <p>控制异常、恢复、重启、日报和日志监控通知</p>
        </div>
        <div className="header-actions">
          <button
            className={`guard-switch ${draft.enabled ? 'guard-switch-on' : ''}`}
            type="button"
            onClick={() => setDraft((next) => ({ ...next, enabled: !next.enabled }))}
          >
            <Bell size={16} />
            {draft.enabled ? '总开关已开启' : '总开关已关闭'}
          </button>
          <button className="primary-button" type="button" onClick={save} disabled={busy !== ''}>
            <Save size={16} />
            保存
          </button>
        </div>
      </header>

      <div className="message-push-scroll">
        <section className="message-push-type-grid" aria-label="推送消息类型">
          {typeSwitches.map((item) => {
            const Icon = item.icon;
            const active = Boolean(draft[item.key]);
            return (
              <button
                className={`message-push-type-card ${active ? 'on' : ''}`}
                type="button"
                key={item.key}
                onClick={() => setDraft((next) => ({ ...next, [item.key]: !next[item.key] }))}
              >
                <span className="message-push-type-icon">
                  <Icon size={18} />
                </span>
                <strong>{item.label}</strong>
                <small>{active ? '已开启' : '已关闭'}</small>
                <em>{item.detail}</em>
              </button>
            );
          })}
        </section>

        <div className="message-push-layout">
          <section className="message-push-panel">
            <div className="message-push-panel-title">
              <Radio size={18} />
              <h2>推送渠道</h2>
            </div>
            <div className="message-push-channel-grid">
              {Object.entries(channelLabels).map(([type, label]) => (
                <button
                  type="button"
                  key={type}
                  className={`message-push-channel ${selectedChannel === type ? 'active' : ''}`}
                  onClick={() => setDraft((next) => ({ ...next, selectedChannels: [type] }))}
                >
                  <span>{label}</span>
                  <small>{configured.has(type) || selectedChannel === type ? '可用' : '未配置'}</small>
                </button>
              ))}
            </div>
            <ChannelFields config={draft} channel={selectedChannel} onChange={setDraft} />
          </section>

          <section className="message-push-panel">
            <div className="message-push-panel-title">
              <Activity size={18} />
              <h2>发送控制</h2>
            </div>
            <div className="message-push-field-grid compact">
              <label>
                <span>日报时间</span>
                <input
                  type="time"
                  value={draft.dailyTime || '09:00'}
                  onChange={(event) => setDraft((next) => ({ ...next, dailyTime: event.target.value }))}
                />
              </label>
              <label>
                <span>异常阈值</span>
                <input
                  type="number"
                  min={1}
                  max={99}
                  value={draft.abnormalTimeoutThreshold || 5}
                  onChange={(event) => updateNumber(setDraft, 'abnormalTimeoutThreshold', event.target.value)}
                />
              </label>
              <label>
                <span>日志扫描间隔</span>
                <input
                  type="number"
                  min={5}
                  max={3600}
                  value={draft.logScanIntervalSec || 30}
                  onChange={(event) => updateNumber(setDraft, 'logScanIntervalSec', event.target.value)}
                />
              </label>
              <label>
                <span>重试次数</span>
                <input
                  type="number"
                  min={1}
                  max={10}
                  value={draft.pushRetryCount || 3}
                  onChange={(event) => updateNumber(setDraft, 'pushRetryCount', event.target.value)}
                />
              </label>
              <label>
                <span>HTTP 超时 ms</span>
                <input
                  type="number"
                  min={1000}
                  max={120000}
                  value={draft.httpTimeoutMs || 10000}
                  onChange={(event) => updateNumber(setDraft, 'httpTimeoutMs', event.target.value)}
                />
              </label>
            </div>
            <div className="message-push-actions">
              <button className="secondary-button" type="button" onClick={() => sendTest('test')} disabled={busy !== ''}>
                <Send size={16} />
                测试推送
              </button>
              <button className="secondary-button" type="button" onClick={() => sendTest('daily')} disabled={busy !== ''}>
                <Clock size={16} />
                日报测试
              </button>
            </div>
            <p className={`settings-message ${notice.includes('失败') || notice.includes('error') ? 'error' : ''}`}>{notice}</p>
            <dl className="message-push-daily-state">
              <div>
                <dt>下次日报</dt>
                <dd>{state.daily?.nextRunAt || '等待开启日报'}</dd>
              </div>
              <div>
                <dt>最近检查</dt>
                <dd>{state.daily?.lastCheckedAt || '尚未检查'}</dd>
              </div>
            </dl>
          </section>

          <section className="message-push-panel message-push-template-panel">
            <div className="message-push-panel-title">
              <Braces size={18} />
              <h2>消息模板</h2>
            </div>
            <div className="message-push-template-selectors">
              <label>
                <span>消息类型</span>
                <select
                   value={selectedTemplateType}
                   onChange={(event) => {
                     setSelectedTemplateType(event.target.value);
                    setTemplateNotice('');
                  }}
                >
                  {templateCatalog.map((item) => (
                    <option key={item.type} value={item.type}>{item.label}</option>
                  ))}
                </select>
              </label>
              <label>
                <span>已配置渠道</span>
                <select
                   value={activeTemplateChannel}
                   onChange={(event) => {
                     setSelectedTemplateChannel(event.target.value);
                    setTemplateNotice('');
                  }}
                >
                  {templateChannels.map((channel) => (
                    <option key={channel} value={channel}>{channelLabels[channel] || channel}</option>
                  ))}
                </select>
              </label>
              <label>
                <span>载荷模式</span>
                <select value={activeTemplate.mode} onChange={(event) => updateTemplate((item) => ({ ...item, mode: event.target.value }))}>
                  {templateModes.map((mode) => <option key={mode} value={mode}>{mode}</option>)}
                </select>
              </label>
              <label className="message-push-template-enabled">
                <input type="checkbox" checked={activeTemplate.enabled} onChange={(event) => updateTemplate((item) => ({ ...item, enabled: event.target.checked }))} />
                <span>启用此模板</span>
              </label>
            </div>
            <div className="message-push-template-workspace">
              <label className="message-push-template-editor">
                <span>完整载荷</span>
                <textarea
                  value={activeTemplate.content}
                  onChange={(event) => updateTemplate((item) => ({ ...item, content: event.target.value }))}
                  spellCheck={false}
                />
              </label>
              <aside className="message-push-template-variables" aria-label="可用变量">
                <strong>可用变量</strong>
                <div>
                   {(templateDefinition?.variables || fallbackTemplateCatalog.find((item) => item.type === selectedTemplateType)?.variables || []).map((variable) => (
                     <div className="message-push-template-variable" key={variable.path}>
                       <code>{`{{${variable.path}}}`}</code>
                       <strong>{variable.label}</strong>
                       <span>{variable.description}</span>
                       <small>示例：{variable.example}</small>
                     </div>
                   ))}
                </div>
              </aside>
            </div>
            <div className="message-push-template-actions">
              <button className="secondary-button" type="button" onClick={restoreTemplateDefault} disabled={busy !== ''}>
                <RotateCcw size={16} />
                恢复默认
              </button>
              <button className="secondary-button" type="button" onClick={testTemplate} disabled={busy !== ''}>
                <Send size={16} />
                测试当前模板
              </button>
              <p className={`settings-message ${templateNotice.includes('失败') || templateNotice.includes('错误') ? 'error' : ''}`}>{templateNotice}</p>
            </div>
            {selectedTemplateType === 'log_monitor' && (
              <LogMonitorRules rules={draft.logMonitorRules} onChange={(rules) => setDraft((next) => ({ ...next, logMonitorRules: rules }))} />
            )}
          </section>

          <section className="message-push-panel message-push-recent-panel">
            <div className="message-push-panel-title">
              <Clock size={18} />
              <h2>最近推送</h2>
            </div>
            <div className="message-push-recent-list">
              {(state.recentPushes || []).length === 0 ? (
                <div className="message-push-empty">暂无推送记录</div>
              ) : (
                state.recentPushes.map((record, index) => (
                  <div className="message-push-recent-row" key={`${record.time}-${index}`}>
                    <strong>{record.title}</strong>
                    <span>{record.ok ? '成功' : '失败'}</span>
                    <time>{record.time}</time>
                    <small>{(record.channels || []).map((item) => channelLabels[item] || item).join(' / ')}</small>
                  </div>
                ))
              )}
            </div>
          </section>
        </div>
      </div>
      <SaveSuccessToast visible={saveToastVisible} />
    </section>
  );
}

function ChannelFields({
  config,
  channel,
  onChange,
}: {
  config: MessagePushConfig;
  channel: string;
  onChange: (updater: (next: MessagePushConfig) => MessagePushConfig) => void;
}) {
  const fields: Record<string, Array<{ key: keyof ChannelConfig; label: string; placeholder?: string; multiline?: boolean }>> = {
    serverchan: [{ key: 'serverChanSendKey', label: 'SendKey' }],
    pushplus: [{ key: 'pushPlusToken', label: 'Token' }],
    qmsg: [
      { key: 'qmsgKey', label: 'Key' },
      { key: 'qmsgType', label: '类型', placeholder: 'send / group' },
      { key: 'qmsgTarget', label: '目标 QQ' },
    ],
    wecom: [{ key: 'wecomWebhook', label: '企业微信 Webhook' }],
    dingtalk: [
      { key: 'dingtalkWebhook', label: '钉钉 Webhook' },
      { key: 'dingtalkSecret', label: '加签 Secret' },
    ],
    feishu: [{ key: 'feishuWebhook', label: '飞书 Webhook' }],
    telegram: [
      { key: 'telegramBotToken', label: 'Bot Token' },
      { key: 'telegramChatId', label: 'Chat ID' },
    ],
    bark: [
      { key: 'barkServerUrl', label: 'Bark 服务' },
      { key: 'barkDeviceKey', label: 'Device Key' },
    ],
    ntfy: [
      { key: 'ntfyServerUrl', label: 'ntfy 服务' },
      { key: 'ntfyTopic', label: 'Topic' },
    ],
    webhook: [
      { key: 'webhookUrl', label: 'Webhook URL' },
      { key: 'webhookMethod', label: 'Method', placeholder: 'POST' },
      { key: 'webhookHeaders', label: 'Headers JSON', placeholder: '{"Authorization":"Bearer ..."}', multiline: true },
    ],
  };
  return (
    <div className="message-push-field-grid">
      {(fields[channel] || fields.webhook).map((field) => (
        <label className={field.multiline ? 'wide' : ''} key={String(field.key)}>
          <span>{field.label}</span>
          {field.multiline ? (
            <textarea
              value={String(config.channels?.[field.key] || '')}
              placeholder={field.placeholder}
              onChange={(event) => updateChannel(onChange, field.key, event.target.value)}
            />
          ) : (
            <input
              value={String(config.channels?.[field.key] || '')}
              placeholder={field.placeholder}
              onChange={(event) => updateChannel(onChange, field.key, event.target.value)}
            />
          )}
        </label>
      ))}
    </div>
  );
}

function LogMonitorRules({ rules, onChange }: { rules: LogMonitorRule[]; onChange: (rules: LogMonitorRule[]) => void }) {
  function updateRule(index: number, patch: Partial<LogMonitorRule>) {
    onChange(rules.map((rule, ruleIndex) => (ruleIndex === index ? { ...rule, ...patch } : rule)));
  }

  return (
    <div className="message-push-log-rules">
      <div className="message-push-log-rules-title">
        <div>
          <h3>日志监控规则</h3>
          <p>没有规则时不会发送日志通知。</p>
        </div>
        <button
          className="icon-button"
          type="button"
          title="添加日志监控规则"
          aria-label="添加日志监控规则"
          onClick={() => onChange([...rules, { enabled: true, source: '', type: '', keyword: '' }])}
        >
          <Plus size={16} />
        </button>
      </div>
      {rules.length === 0 ? (
        <div className="message-push-empty">尚未设置规则</div>
      ) : (
        <div className="message-push-log-rule-list">
          {rules.map((rule, index) => (
            <div className="message-push-log-rule" key={`${index}-${rule.source}-${rule.type}`}>
              <label className="message-push-template-enabled">
                <input type="checkbox" checked={rule.enabled} onChange={(event) => updateRule(index, { enabled: event.target.checked })} />
                <span>启用</span>
              </label>
              <input value={rule.source} placeholder="来源" aria-label="规则来源" onChange={(event) => updateRule(index, { source: event.target.value })} />
              <input value={rule.type} placeholder="事件类型" aria-label="规则事件类型" onChange={(event) => updateRule(index, { type: event.target.value })} />
              <input value={rule.keyword} placeholder="关键词" aria-label="规则关键词" onChange={(event) => updateRule(index, { keyword: event.target.value })} />
              <button
                className="icon-button danger"
                type="button"
                title="删除日志监控规则"
                aria-label="删除日志监控规则"
                onClick={() => onChange(rules.filter((_, ruleIndex) => ruleIndex !== index))}
              >
                <Trash2 size={16} />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function mergeConfig(config?: MessagePushConfig): MessagePushConfig {
  return {
    ...defaultConfig,
    ...(config || {}),
    channels: {
      ...defaultConfig.channels,
      ...(config?.channels || {}),
    },
    selectedChannels: config?.selectedChannels?.length ? config.selectedChannels : defaultConfig.selectedChannels,
    channelFormats: config?.channelFormats || {},
    templates: config?.templates || {},
    logMonitorRules: config?.logMonitorRules || [],
  };
}

function supportedTemplateModes(definition: TemplateDefinition | undefined, channel: string) {
  return definition?.channelModes.find((item) => item.channel === channel)?.modes || ['text'];
}

function templateFor(config: MessagePushConfig, messageType: string, channel: string, fallbackMode: string): MessageTemplate {
  const template = config.templates?.[messageType]?.[channel];
  return template ? { ...template } : defaultTemplateFor(messageType, channel, fallbackMode);
}

function defaultTemplateFor(messageType: string, channel: string, mode: string): MessageTemplate {
  return {
    ...fallbackTemplate,
    mode: mode || fallbackTemplate.mode,
    content: mode === 'json' ? `{"type":"${messageType}","target":"{{runtime.target}}"}` : fallbackTemplate.content,
  };
}

function hasRuleConstraint(rule: LogMonitorRule) {
  return Boolean(rule.source.trim() || rule.type.trim() || rule.keyword.trim());
}

function updateChannel(
  onChange: (updater: (next: MessagePushConfig) => MessagePushConfig) => void,
  key: keyof ChannelConfig,
  value: string,
) {
  onChange((next) => ({
    ...next,
    channels: {
      ...next.channels,
      [key]: value,
    },
  }));
}

function updateNumber(
  setDraft: (updater: (next: MessagePushConfig) => MessagePushConfig) => void,
  key: keyof MessagePushConfig,
  value: string,
) {
  setDraft((next) => ({ ...next, [key]: Number(value) }));
}

function resultText(result: SendResult) {
  if (result.ok) {
    return '推送已发送';
  }
  return result.summary || '推送失败';
}
