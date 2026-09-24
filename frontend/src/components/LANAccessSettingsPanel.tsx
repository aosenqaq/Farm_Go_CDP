import { Globe2, Loader2, Save } from 'lucide-react';
import { useEffect, useState } from 'react';

import { LANAccessSettings, SaveLANAccessSettings } from '../../wailsjs/go/main/App';

type LANAccessMode = 'lan' | 'tunnel';

type LANAccessStatus = {
  enabled: boolean;
  mode: LANAccessMode;
  port: number;
  passwordConfigured: boolean;
  running: boolean;
  address: string;
  error: string;
};

type LANAccessDraft = Pick<LANAccessStatus, 'enabled' | 'mode' | 'port'>;

const fallbackStatus: LANAccessStatus = {
  enabled: false,
  mode: 'lan',
  port: 8788,
  passwordConfigured: false,
  running: false,
  address: '',
  error: '',
};

function normalizeStatus(value: unknown): LANAccessStatus {
  const source = value && typeof value === 'object' ? value as Partial<LANAccessStatus> : {};
  return {
    enabled: source.enabled === true,
    mode: source.mode === 'tunnel' ? 'tunnel' : 'lan',
    port: Number.isInteger(source.port) && (source.port as number) > 0 && (source.port as number) <= 65535
      ? source.port as number
      : fallbackStatus.port,
    passwordConfigured: source.passwordConfigured === true,
    running: source.running === true,
    address: typeof source.address === 'string' ? source.address : '',
    error: typeof source.error === 'string' ? source.error : '',
  };
}

function listenerTarget(draft: LANAccessDraft, status: LANAccessStatus) {
  if (status.address && status.mode === draft.mode && status.port === draft.port) return status.address;
  if (draft.mode === 'tunnel') return `127.0.0.1:${draft.port}`;
  return '未检测到可用的局域网 IPv4 地址';
}

function serviceStatus(status: LANAccessStatus) {
  if (status.error) return '配置错误';
  if (!status.enabled) return '未启用';
  return status.running ? '正在运行' : '等待启动';
}

export function LANAccessSettingsPanel() {
  const [status, setStatus] = useState<LANAccessStatus>(fallbackStatus);
  const [draft, setDraft] = useState<LANAccessDraft>(fallbackStatus);
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState('正在读取 LAN 控制台配置');
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    void LANAccessSettings()
      .then((value) => {
        if (!active) return;
        const next = normalizeStatus(value);
        setStatus(next);
        setDraft(next);
        setMessage('配置已加载');
      })
      .catch((reason) => {
        if (!active) return;
        setError(reason instanceof Error ? reason.message : String(reason));
        setMessage('读取失败');
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  async function save() {
    const port = Number(draft.port);
    const passwordChanged = password !== '' || confirmPassword !== '';
    setError('');
    setMessage('');

    if (!Number.isInteger(port) || port < 1024 || port > 65535) {
      setError('端口必须是 1024 到 65535 之间的整数');
      return;
    }
    if (passwordChanged && [...password].length < 12) {
      setError('新访问密码至少 12 位');
      return;
    }
    if (password !== confirmPassword) {
      setError('两次输入的访问密码不一致');
      return;
    }
    if (draft.enabled && !status.passwordConfigured && password === '') {
      setError('启用 LAN 控制台前请设置至少 12 位的访问密码');
      return;
    }

    setSaving(true);
    try {
      const saved = normalizeStatus(await SaveLANAccessSettings({
        enabled: draft.enabled,
        mode: draft.mode,
        port,
        password,
        confirmPassword,
      }));
      setStatus(saved);
      setDraft(saved);
      setPassword('');
      setConfirmPassword('');
      setMessage('LAN 控制台设置已保存');
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setSaving(false);
    }
  }

  const target = listenerTarget(draft, status);
  const targetLabel = draft.mode === 'lan' ? '局域网地址' : 'FRP 目标';

  return (
    <section className="settings-panel lan-access-panel">
      <div className="section-heading">
        <Globe2 size={18} />
        <span>局域网 Web 控制台</span>
      </div>
      <div className="lan-access-status" aria-live="polite">
        <div>
          <span>服务状态</span>
          <strong className={status.error ? 'error' : status.running ? 'ready' : ''}>{serviceStatus(status)}</strong>
        </div>
        <div>
          <span>{targetLabel}</span>
          <code>{target}</code>
        </div>
      </div>
      <div className="settings-form lan-access-form">
        <label className="check-row">
          <input
            name="lan-access-enabled"
            type="checkbox"
            checked={draft.enabled}
            onChange={(event) => setDraft((current) => ({ ...current, enabled: event.target.checked }))}
            disabled={loading || saving}
          />
          <span>开启局域网访问</span>
        </label>
        <label>
          <span>访问模式</span>
          <select
            name="lan-access-mode"
            value={draft.mode}
            onChange={(event) => setDraft((current) => ({ ...current, mode: event.target.value === 'tunnel' ? 'tunnel' : 'lan' }))}
            disabled={loading || saving}
          >
            <option value="lan">局域网</option>
            <option value="tunnel">安全隧道（仅 HTTPS）</option>
          </select>
        </label>
        <label>
          <span>监听端口</span>
          <input
            name="lan-access-port"
            type="number"
            min={1024}
            max={65535}
            step={1}
            value={draft.port}
            onChange={(event) => setDraft((current) => ({ ...current, port: Number(event.target.value) }))}
            disabled={loading || saving}
          />
        </label>
        <label>
          <span>{status.passwordConfigured ? '新访问密码（留空不修改）' : '访问密码（至少 12 位）'}</span>
          <input
            name="lan-access-password"
            type="password"
            className="lan-access-password-input"
            value={password}
            autoComplete="new-password"
            onChange={(event) => setPassword(event.target.value)}
            disabled={loading || saving}
          />
        </label>
        <label>
          <span>确认访问密码</span>
          <input
            name="lan-access-confirm-password"
            type="password"
            className="lan-access-password-input"
            value={confirmPassword}
            autoComplete="new-password"
            onChange={(event) => setConfirmPassword(event.target.value)}
            disabled={loading || saving}
          />
        </label>
        <div className="settings-actions">
          <button className="primary-button" name="save-lan-access-settings" type="button" onClick={save} disabled={loading || saving}>
            {saving ? <Loader2 className="spin" size={17} /> : <Save size={17} />}
            <span>{saving ? '正在保存' : '保存设置'}</span>
          </button>
        </div>
        {(error || message) && <div className={error ? 'settings-message error' : 'settings-message'}>{error || message}</div>}
      </div>
    </section>
  );
}
