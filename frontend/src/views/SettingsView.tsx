import { Loader2, PlugZap, RefreshCw, Save, SlidersHorizontal, Trash2 } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import {
  CleanQQMiniappCache,
  CleanWeChatCache,
  CleanYYBMiniappCache,
  PreviewWeChatCacheCleanup,
  RuntimeSettings,
  SaveRuntimeSettings,
  SwitchRuntimeTarget,
} from '../../wailsjs/go/main/App';
import { LANAccessSettingsPanel } from '../components/LANAccessSettingsPanel';
import { SAVE_TOAST_AUTO_DISMISS_MS, SaveSuccessToast } from '../components/SaveSuccessToast';
import type { RuntimeEventDto } from '../lib/events';
import type { UpdateCheckPreferencesDto, UpdateStateDto } from '../lib/update';

export { SAVE_TOAST_AUTO_DISMISS_MS, SaveSuccessToast } from '../components/SaveSuccessToast';

type LinkTarget = 'qq_ws' | 'wechat_cdp' | 'yyb_cdp';

type Settings = {
  defaultTarget: LinkTarget;
  currentTarget: LinkTarget;
  autoStart: boolean;
  cdpPort: number;
  wmpfDebugPort: number;
  processGuardEnabled: boolean;
  processGuardFailureRecoveryEnabled: boolean;
  processGuardTimeoutThreshold: number;
  processGuardMonitorIntervalMs: number;
  processGuardRestartReconnectGraceSec: number;
  processGuardMaxRestartsPer10Min: number;
  processGuardScheduledRestartEnabled: boolean;
  processGuardScheduledRestartIntervalMin: number;
  processGuardAutoMinimizeAfterRestart: boolean;
  networkReconnectEnabled: boolean;
  networkReconnectIntervalMs: number;
  networkReconnectRecoveryTimeoutMs: number;
  otherPlaceLoginReconnectEnabled: boolean;
  otherPlaceLoginCheckIntervalMs: number;
  otherPlaceLoginReconnectDelayMin: number;
  autoWarehouseSellEnabled: boolean;
  autoWarehouseSellIntervalMinute: number;
  autoWarehouseSellCategories: string[];
  warehouseRefreshOnlyOnAutoSell: boolean;
};

type SettingsViewProps = {
  events: RuntimeEventDto[];
  onRefreshEvents: () => void;
  updateState?: UpdateStateDto | null;
  updateCheckPreferences?: UpdateCheckPreferencesDto;
  onCheckUpdates?: () => Promise<UpdateStateDto>;
  onSaveUpdateCheckPreferences?: (value: UpdateCheckPreferencesDto) => Promise<UpdateCheckPreferencesDto>;
  onUpdateAvailable?: (update: UpdateStateDto) => void;
  onRuntimeSwitched?: () => void | Promise<void>;
};

type MaintenanceSummary = {
  targetCount: number;
  movedCount: number;
  skippedCount: number;
  closedProcesses: number;
  backupDir: string;
  targets: Array<{ relativePath: string }>;
};

const linkOptions: Array<{ value: LinkTarget; label: string }> = [
  { value: 'qq_ws', label: 'QQ WS' },
  { value: 'wechat_cdp', label: '微信 CDP' },
  { value: 'yyb_cdp', label: '应用宝 CDP' },
];

const fallbackSettings: Settings = {
  defaultTarget: 'qq_ws',
  currentTarget: 'qq_ws',
  autoStart: true,
  cdpPort: 62000,
  wmpfDebugPort: 9420,
  processGuardEnabled: false,
  processGuardFailureRecoveryEnabled: true,
  processGuardTimeoutThreshold: 3,
  processGuardMonitorIntervalMs: 3000,
  processGuardRestartReconnectGraceSec: 45,
  processGuardMaxRestartsPer10Min: 4,
  processGuardScheduledRestartEnabled: false,
  processGuardScheduledRestartIntervalMin: 60,
  processGuardAutoMinimizeAfterRestart: false,
  networkReconnectEnabled: true,
  networkReconnectIntervalMs: 1000,
  networkReconnectRecoveryTimeoutMs: 20000,
  otherPlaceLoginReconnectEnabled: false,
  otherPlaceLoginCheckIntervalMs: 5000,
  otherPlaceLoginReconnectDelayMin: 5,
  autoWarehouseSellEnabled: false,
  autoWarehouseSellIntervalMinute: 60,
  autoWarehouseSellCategories: ['fruit'],
  warehouseRefreshOnlyOnAutoSell: true,
};

const defaultUpdateCheckPreferences: UpdateCheckPreferencesDto = {
  enabled: true,
  intervalMinutes: 120,
};

export function SettingsView({
  events,
  onRefreshEvents,
  updateState = null,
  updateCheckPreferences = defaultUpdateCheckPreferences,
  onCheckUpdates,
  onSaveUpdateCheckPreferences,
  onUpdateAvailable,
  onRuntimeSwitched,
}: SettingsViewProps) {
  const [settings, setSettings] = useState<Settings>(fallbackSettings);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('正在读取配置');
  const [error, setError] = useState('');
  const [saveToastVisible, setSaveToastVisible] = useState(false);
  const [manualUpdate, setManualUpdate] = useState<UpdateStateDto | null>(null);
  const [updateChecking, setUpdateChecking] = useState(false);
  const [updateError, setUpdateError] = useState('');
  const [scheduledUpdateEnabled, setScheduledUpdateEnabled] = useState(updateCheckPreferences.enabled);
  const [scheduledUpdateInterval, setScheduledUpdateInterval] = useState(String(updateCheckPreferences.intervalMinutes));
  const [updatePreferencesSaving, setUpdatePreferencesSaving] = useState(false);
  const [updatePreferencesMessage, setUpdatePreferencesMessage] = useState('');
  const [updatePreferencesError, setUpdatePreferencesError] = useState('');
  const appliedUpdatePreferencesKey = useRef(`${updateCheckPreferences.enabled}:${updateCheckPreferences.intervalMinutes}`);
  const [maintenanceBusy, setMaintenanceBusy] = useState<'wechat-preview' | 'wechat-apply' | 'qq' | 'yyb' | null>(null);
  const [maintenanceMessage, setMaintenanceMessage] = useState('');
  const [maintenanceError, setMaintenanceError] = useState('');
  void events;
  void onRefreshEvents;

  useEffect(() => {
    RuntimeSettings()
      .then((value) => {
        setSettings(normalizeSettings(value as Settings));
        setMessage('配置已加载');
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : String(err));
        setMessage('读取失败');
      });
  }, []);

  useEffect(() => {
    if (!saveToastVisible) return;
    const timer = window.setTimeout(() => setSaveToastVisible(false), SAVE_TOAST_AUTO_DISMISS_MS);
    return () => window.clearTimeout(timer);
  }, [saveToastVisible]);

  useEffect(() => {
    const nextKey = `${updateCheckPreferences.enabled}:${updateCheckPreferences.intervalMinutes}`;
    if (appliedUpdatePreferencesKey.current === nextKey) return;
    appliedUpdatePreferencesKey.current = nextKey;
    setScheduledUpdateEnabled(updateCheckPreferences.enabled);
    setScheduledUpdateInterval(String(updateCheckPreferences.intervalMinutes));
  }, [updateCheckPreferences.enabled, updateCheckPreferences.intervalMinutes]);

  async function save() {
    setBusy(true);
    setError('');
    try {
      const saved = (await SaveRuntimeSettings(settings)) as Settings;
      setSettings(normalizeSettings(saved));
      setMessage('配置已保存');
      setSaveToastVisible(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setMessage('保存失败');
      setSaveToastVisible(false);
    } finally {
      setBusy(false);
    }
  }

  async function switchNow() {
    setBusy(true);
    setError('');
    try {
      const status = await SwitchRuntimeTarget(settings.currentTarget);
      setMessage(status.phase === 'error' ? `切换失败：${status.lastError || '启动错误'}` : `已切换到 ${labelFor(settings.currentTarget)}`);
      if (status.phase !== 'error') {
        await onRuntimeSwitched?.();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setMessage('切换失败');
    } finally {
      setBusy(false);
    }
  }

  async function checkUpdates() {
    if (!onCheckUpdates) return;
    setUpdateChecking(true);
    setUpdateError('');
    try {
      const update = await onCheckUpdates();
      setManualUpdate(update);
      if (update.available && !update.errorCode) onUpdateAvailable?.(update);
    } catch (err) {
      setManualUpdate(null);
      setUpdateError(err instanceof Error ? err.message : String(err));
    } finally {
      setUpdateChecking(false);
    }
  }

  async function saveUpdatePreferences() {
    const intervalMinutes = Number(scheduledUpdateInterval);
    if (!Number.isInteger(intervalMinutes) || intervalMinutes < 1 || intervalMinutes > 10080) {
      setUpdatePreferencesError('请输入 1 到 10080 之间的整数分钟数');
      setUpdatePreferencesMessage('');
      return;
    }
    if (!onSaveUpdateCheckPreferences) return;

    setUpdatePreferencesSaving(true);
    setUpdatePreferencesError('');
    setUpdatePreferencesMessage('');
    try {
      const saved = await onSaveUpdateCheckPreferences({ enabled: scheduledUpdateEnabled, intervalMinutes });
      setScheduledUpdateEnabled(saved.enabled);
      setScheduledUpdateInterval(String(saved.intervalMinutes));
      setUpdatePreferencesMessage('定时检查设置已保存');
    } catch (err) {
      setUpdatePreferencesError(err instanceof Error ? err.message : String(err));
    } finally {
      setUpdatePreferencesSaving(false);
    }
  }

  async function handleWechatCleanup() {
    const acknowledged = window.confirm('将预览微信小程序运行缓存；执行清理时会关闭微信相关进程，并将匹配缓存移动到备份目录。继续吗？');
    if (!acknowledged) return;

    setMaintenanceBusy('wechat-preview');
    setMaintenanceError('');
    setMaintenanceMessage('');
    try {
      const preview = (await PreviewWeChatCacheCleanup()) as MaintenanceSummary;
      if (preview.targetCount === 0) {
        setMaintenanceMessage('没有发现需要清理的微信小程序运行缓存。');
        return;
      }
      const shownTargets = preview.targets.slice(0, 12).map((target) => `- ${target.relativePath}`).join('\n');
      const more = preview.targetCount > 12 ? `\n... 还有 ${preview.targetCount - 12} 项` : '';
      const backup = preview.backupDir ? `\n\n备份目录：${preview.backupDir}` : '';
      const confirmed = window.confirm(`将移动 ${preview.targetCount} 项微信小程序运行缓存到备份目录。\n\n${shownTargets}${more}${backup}\n\n确认执行清理？`);
      if (!confirmed) {
        setMaintenanceMessage('已取消，未执行微信小程序缓存清理。');
        return;
      }

      setMaintenanceBusy('wechat-apply');
      const result = (await CleanWeChatCache()) as MaintenanceSummary;
      setMaintenanceMessage(cleanupMessage('微信', result, '请重启微信，并重新打开一次农场小程序以生成新的运行缓存。'));
    } catch (err) {
      setMaintenanceError(err instanceof Error ? err.message : String(err));
    } finally {
      setMaintenanceBusy(null);
    }
  }

  async function handleQQCleanup() {
    const acknowledged = window.confirm('将清理 QQ 小程序运行缓存。执行时会关闭 QQ、QQNT 和 QQ 小程序相关进程，并将缓存移动到备份目录。继续吗？');
    if (!acknowledged) return;
    const confirmed = window.confirm('再次确认：请先保存 QQ 中未完成的操作。是否立即开始清理 QQ 小程序缓存？');
    if (!confirmed) {
      setMaintenanceMessage('已取消，未执行 QQ 小程序缓存清理。');
      return;
    }

    setMaintenanceBusy('qq');
    setMaintenanceError('');
    setMaintenanceMessage('');
    try {
      const result = (await CleanQQMiniappCache()) as MaintenanceSummary;
      setMaintenanceMessage(cleanupMessage('QQ', result, '请重启 QQ，并重新打开一次 QQ 农场小程序以生成新的运行缓存。'));
    } catch (err) {
      setMaintenanceError(err instanceof Error ? err.message : String(err));
    } finally {
      setMaintenanceBusy(null);
    }
  }

  async function handleYYBCleanup() {
    const confirmed = window.confirm('将清理应用宝小程序运行缓存。执行时会关闭应用宝相关进程，并将缓存移动到备份目录。继续吗？');
    if (!confirmed) return;

    setMaintenanceBusy('yyb');
    setMaintenanceError('');
    setMaintenanceMessage('');
    try {
      const result = (await CleanYYBMiniappCache()) as MaintenanceSummary;
      setMaintenanceMessage(cleanupMessage('应用宝', result, '请重启应用宝，并重新打开一次农场小程序以生成新的运行缓存。'));
    } catch (err) {
      setMaintenanceError(err instanceof Error ? err.message : String(err));
    } finally {
      setMaintenanceBusy(null);
    }
  }

  const checkedUpdate = manualUpdate || null;
  const updateMessage = updateError || (checkedUpdate?.errorCode ? checkedUpdate.message || '检查更新失败' : '');

  return (
    <section className="view-stack fill settings-view">
      <header className="page-header">
        <div>
          <h1>系统设置</h1>
          <p>配置当前链路与默认启动链路</p>
        </div>
      </header>

      <div className="settings-scroll">
        <div className="settings-grid settings-grid-single">
          <section className="settings-panel settings-main-panel">
            <div className="section-heading">
              <SlidersHorizontal size={18} />
              <span>运行链路</span>
            </div>
            <div className="settings-form">
              <label>
                <span>当前链路</span>
                <select
                  value={settings.currentTarget}
                  onChange={(event) => setSettings({ ...settings, currentTarget: event.target.value as LinkTarget })}
                >
                  {linkOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                <span>默认启动链路</span>
                <select
                  value={settings.defaultTarget}
                  onChange={(event) => setSettings({ ...settings, defaultTarget: event.target.value as LinkTarget })}
                >
                  {linkOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>
              <label className="check-row">
                <input
                  type="checkbox"
                  checked={settings.autoStart}
                  onChange={(event) => setSettings({ ...settings, autoStart: event.target.checked })}
                />
                <span>启动时自动连接默认链路</span>
              </label>
              <div className="settings-actions">
                <button className="primary-button" type="button" onClick={save} disabled={busy}>
                  {busy ? <Loader2 className="spin" size={17} /> : <Save size={17} />}
                  <span>保存</span>
                </button>
                <button className="secondary-button" type="button" onClick={switchNow} disabled={busy}>
                  <PlugZap size={17} />
                  <span>立即切换</span>
                </button>
              </div>
              <div className={error ? 'settings-message error' : 'settings-message'}>{error || message}</div>
            </div>
          </section>
          <LANAccessSettingsPanel />
          <section className="settings-panel application-update-panel" id="application-update" tabIndex={-1}>
            <div className="section-heading">
              <RefreshCw size={18} />
              <span>应用更新</span>
            </div>
            <div className="application-update-body">
              <div className="application-update-current">当前版本：{updateState?.current?.name || updateState?.current?.number || '待检查'}</div>
              <div className="application-update-preferences">
                <label className="application-update-toggle">
                  <input
                    name="scheduled-update-enabled"
                    type="checkbox"
                    checked={scheduledUpdateEnabled}
                    onChange={(event) => setScheduledUpdateEnabled(event.target.checked)}
                  />
                  <span>定时检查更新</span>
                </label>
                <label className="application-update-interval">
                  <span>检查间隔</span>
                  <span className="application-update-interval-control">
                    <input
                      name="scheduled-update-interval"
                      type="number"
                      min={1}
                      max={10080}
                      step={1}
                      value={scheduledUpdateInterval}
                      disabled={!scheduledUpdateEnabled}
                      onChange={(event) => setScheduledUpdateInterval(event.target.value)}
                    />
                    <span>分钟/次</span>
                  </span>
                </label>
                <button
                  className="secondary-button application-update-save"
                  name="save-update-preferences"
                  type="button"
                  onClick={saveUpdatePreferences}
                  disabled={updatePreferencesSaving || !onSaveUpdateCheckPreferences}
                >
                  {updatePreferencesSaving ? <Loader2 className="spin" size={17} /> : <Save size={17} />}
                  <span>{updatePreferencesSaving ? '正在保存' : '保存定时设置'}</span>
                </button>
              </div>
              {updatePreferencesMessage && <div className="application-update-preference-status" role="status">{updatePreferencesMessage}</div>}
              {updatePreferencesError && <div className="application-update-preference-status error" role="alert">{updatePreferencesError}</div>}
              <button className="secondary-button application-update-check" name="check-updates" type="button" onClick={checkUpdates} disabled={updateChecking || !onCheckUpdates}>
                {updateChecking ? <Loader2 className="spin" size={17} /> : <RefreshCw size={17} />}
                <span>{updateChecking ? '正在检查' : '检查更新'}</span>
              </button>
              {checkedUpdate && !updateMessage && !checkedUpdate.available && <div className="application-update-status" role="status">已是最新版本</div>}
              {updateMessage && <div className="application-update-status error" role="alert">{updateMessage}</div>}
            </div>
          </section>
          <section className="settings-panel settings-maintenance-panel">
            <div className="section-heading">
              <Trash2 size={18} />
              <span>缓存维护</span>
            </div>
            <p className="maintenance-copy">缓存会移动到桌面备份目录，不会直接删除。执行后需要重新打开对应的小程序生成运行缓存。</p>
            <div className="maintenance-actions">
              <button className="secondary-button" type="button" onClick={handleWechatCleanup} disabled={maintenanceBusy !== null} aria-label="清理微信小程序缓存" title="清理微信小程序缓存">
                {maintenanceBusy === 'wechat-preview' || maintenanceBusy === 'wechat-apply' ? <Loader2 className="spin" size={17} /> : <Trash2 size={17} />}
                <span>{maintenanceBusy === 'wechat-preview' ? '正在预览' : maintenanceBusy === 'wechat-apply' ? '正在清理' : '清理微信缓存'}</span>
              </button>
              <button className="secondary-button" type="button" onClick={handleQQCleanup} disabled={maintenanceBusy !== null} aria-label="清理 QQ 小程序缓存" title="清理 QQ 小程序缓存">
                {maintenanceBusy === 'qq' ? <Loader2 className="spin" size={17} /> : <Trash2 size={17} />}
                <span>{maintenanceBusy === 'qq' ? '正在清理' : '清理 QQ 缓存'}</span>
              </button>
              <button className="secondary-button" type="button" onClick={handleYYBCleanup} disabled={maintenanceBusy !== null} aria-label="清理应用宝小程序缓存" title="清理应用宝小程序缓存">
                {maintenanceBusy === 'yyb' ? <Loader2 className="spin" size={17} /> : <Trash2 size={17} />}
                <span>{maintenanceBusy === 'yyb' ? '正在清理' : '清理应用宝缓存'}</span>
              </button>
            </div>
            {(maintenanceMessage || maintenanceError) && <div className={maintenanceError ? 'settings-message error maintenance-summary' : 'settings-message maintenance-summary'}>{maintenanceError || maintenanceMessage}</div>}
          </section>
        </div>
      </div>
      <SaveSuccessToast visible={saveToastVisible} />
    </section>
  );
}

function cleanupMessage(platform: string, result: MaintenanceSummary, restartHint: string) {
  if (result.movedCount === 0) {
    return `${platform} 缓存维护完成：没有发现需要移动的匹配缓存。${restartHint}`;
  }
  const backup = result.backupDir ? `备份目录：${result.backupDir}。` : '';
  const skipped = result.skippedCount > 0 ? `跳过 ${result.skippedCount} 项已不存在的缓存。` : '';
  return `${platform} 缓存维护完成：关闭进程 ${result.closedProcesses} 类，移动缓存 ${result.movedCount} 项。${skipped}${backup}${restartHint}`;
}

function normalizeSettings(value: Partial<Settings>): Settings {
  return {
    ...fallbackSettings,
    ...value,
    defaultTarget: normalizeTarget(value.defaultTarget),
    currentTarget: normalizeTarget(value.currentTarget),
  };
}

function normalizeTarget(value: unknown): LinkTarget {
  if (value === 'wechat_cdp' || value === 'yyb_cdp' || value === 'qq_ws') {
    return value;
  }
  return 'qq_ws';
}

function labelFor(target: LinkTarget) {
  return linkOptions.find((option) => option.value === target)?.label || target;
}
