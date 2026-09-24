import { Bot, CalendarClock, Check, GripVertical, Loader2, Play, Power, RefreshCw, RotateCcw, Save, Settings, X } from 'lucide-react';
import { useEffect, useRef, useState, type DragEvent, type SetStateAction } from 'react';

import { FallbackImage } from '../components/FallbackImage';
import { SAVE_TOAST_AUTO_DISMISS_MS, SaveSuccessToast } from '../components/SaveSuccessToast';

export type FarmAutomationFeatureGroup = {
  id: string;
  label: string;
  summary: string;
  enabled: boolean;
  settingKeys: string[];
};

const mobileFeatureSectionDefinitions = [
  { id: 'own_farm', label: '自己的农场', featureGroupIds: ['own_base', 'planting', 'fertilizer'] },
  { id: 'friends', label: '好友互动', featureGroupIds: ['friends'] },
  { id: 'rewards_and_shop', label: '奖励与商店', featureGroupIds: ['rewards', 'mystery_shop'] },
] as const;

export function groupAutomationFeatureGroupsForMobile(featureGroups: FarmAutomationFeatureGroup[]) {
  const byId = new Map(featureGroups.map((group) => [group.id, group]));
  const assigned = new Set<string>();
  const sections = mobileFeatureSectionDefinitions
    .map(({ id, label, featureGroupIds }) => {
      const groups = featureGroupIds
        .map((featureGroupId) => byId.get(featureGroupId))
        .filter((group): group is FarmAutomationFeatureGroup => Boolean(group));
      groups.forEach((group) => assigned.add(group.id));
      return { id, label, groups };
    })
    .filter((section) => section.groups.length > 0);
  const otherGroups = featureGroups.filter((group) => !assigned.has(group.id));
  return otherGroups.length > 0 ? [...sections, { id: 'other', label: '其他功能', groups: otherGroups }] : sections;
}

export type FarmAutomationSchedulerTask = {
  id: string;
  label: string;
  enabledConfigKey?: string;
  intervalConfigKey?: string;
  priority: number;
  intervalSec: number;
  enabled: boolean;
  dailyDoneToday?: boolean;
  nextRunAt?: string;
  lastStartedAt?: string;
  lastFinishedAt?: string;
  lastSuccessAt?: string;
  lastError?: string;
  lastResultSummary?: string;
};

export type FarmAutomationState = {
  running: boolean;
  runMode?: FarmAutomationRunMode;
  summary: Record<string, number>;
  featureGroups: FarmAutomationFeatureGroup[];
  scheduler: {
    enabled: boolean;
    minGapMs: number;
    runningTaskId?: string;
    tasks: FarmAutomationSchedulerTask[];
  };
  config: Record<string, any>;
};

export type FarmAutomationRunMode = 'safe' | 'god';

export type FarmAutomationActionResult = {
  ok: boolean;
  status: string;
  taskId?: string;
  message: string;
  actionCount?: number;
  attemptedFriends?: number;
  successfulFriends?: number;
  failedFriends?: number;
  skippedFriends?: number;
};

export type MysteryShopPurchaseRecord = {
  id: string;
  occurredAt: string;
  itemName: string;
  count: number;
  unitPrice: number;
  currencyId: number;
  currencyName: string;
  discount: number;
};

export const AUTOMATION_TOAST_AUTO_DISMISS_MS = 3500;

type BackpackSeedOption = {
  seedId: number;
  name: string;
  level: number | null;
  count: number | null;
  priority: boolean;
  disabled: boolean;
  plantable: boolean;
  plantableReason?: string;
  plantableMessage?: string;
  plantSize?: number | null;
};

type StealCropOption = {
  plantId: number;
  seedId: number;
  name: string;
  level: number | null;
  imageUrl: string;
  sortGroup: number | null;
  sortOrder: number | null;
};

type AutomationViewProps = {
  state: FarmAutomationState;
  remote?: boolean;
  dogGuardFriendCount?: number;
  onRunTask: (taskId: string) => void | Promise<void>;
  onToggleAutomation?: (running: boolean) => void | Promise<void>;
  onSaveState?: (state: FarmAutomationState) => void | Promise<FarmAutomationState | void>;
  onSetRunMode?: (runMode: FarmAutomationRunMode) => Promise<FarmAutomationState | void>;
  onRefreshBackpackSeeds?: (config: Record<string, any>) => Promise<any>;
  onRefreshStealCropOptions?: () => Promise<any>;
  lastActionResult?: FarmAutomationActionResult | null;
  onActionResultConsumed?: () => void;
  mysteryShopPurchaseRecords?: MysteryShopPurchaseRecord[];
  initialSchedulerOpen?: boolean;
  initialSettingsGroupId?: string;
};

const schedulerTaskConfigKeyById: Record<string, string> = {
  own_base: 'autoFarmOneClickEnabled',
  land_upgrade: 'autoFarmLandUpgradeEnabled',
  own_collect: 'autoFarmOwnCollectEnabled',
  own_plant: 'autoFarmPlantEnabled',
  fertilizer_fill: 'autoFarmFertilizerFillEnabled',
  own_fertilizer: 'autoFarmFertilizerEnabled',
  friend_steal: 'autoFarmFriendEnabled',
  friend_help: 'autoFarmFriendHelpEnabled',
  friend_mischief: 'autoFarmFriendMischiefEnabled',
  reward_claim: 'autoRewardClaimEnabled',
  svip_daily_gift: 'autoFarmSvipDailyGiftEnabled',
  monthly_card_reward: 'autoFarmMonthlyCardRewardEnabled',
  mall_daily_fertilizer: 'autoFarmMallDailyFertilizerEnabled',
  share_reward: 'autoFarmShareRewardEnabled',
  mail_reward: 'autoFarmMailRewardEnabled',
  qian_xing_travel_reward: 'autoFarmQianXingTravelRewardEnabled',
  xing_su_auto_light_up: 'autoFarmXingSuAutoLightUpEnabled',
  limited_seed_draw: 'autoFarmLimitedSeedDrawEnabled',
  mystery_shop_auto_buy: 'autoFarmMysteryShopAutoBuyEnabled',
  auto_warehouse_sell: 'autoWarehouseSellEnabled',
};

const schedulerTaskIntervalConfigKeyById: Record<string, string> = {
  own_base: 'autoFarmOwnBaseIntervalSec',
  land_upgrade: 'autoFarmLandUpgradeIntervalSec',
  own_collect: 'autoFarmOwnCollectIntervalSec',
  own_plant: 'autoFarmPlantIntervalSec',
  fertilizer_fill: 'autoFarmFertilizerFillIntervalSec',
  own_fertilizer: 'autoFarmFertilizerIntervalSec',
  friend_steal: 'autoFarmFriendStealIntervalSec',
  friend_help: 'autoFarmFriendHelpIntervalSec',
  friend_mischief: 'autoFarmFriendMischiefIntervalSec',
  reward_claim: 'autoRewardClaimIntervalSec',
  svip_daily_gift: 'autoFarmSvipDailyGiftIntervalSec',
  monthly_card_reward: 'autoFarmMonthlyCardRewardIntervalSec',
  mall_daily_fertilizer: 'autoFarmMallDailyFertilizerIntervalSec',
  share_reward: 'autoFarmShareRewardIntervalSec',
  mail_reward: 'autoFarmMailRewardIntervalSec',
  qian_xing_travel_reward: 'autoFarmQianXingTravelRewardIntervalSec',
  xing_su_auto_light_up: 'autoFarmXingSuAutoLightUpIntervalSec',
  limited_seed_draw: 'autoFarmLimitedSeedDrawIntervalSec',
  mystery_shop_auto_buy: 'autoFarmMysteryShopAutoBuyIntervalSec',
  auto_warehouse_sell: 'autoWarehouseSellIntervalSec',
};

const schedulerTaskScheduleModeConfigKeyById: Record<string, string> = {
  reward_claim: 'autoRewardClaimScheduleMode',
  svip_daily_gift: 'autoFarmSvipDailyGiftScheduleMode',
  monthly_card_reward: 'autoFarmMonthlyCardRewardScheduleMode',
  mall_daily_fertilizer: 'autoFarmMallDailyFertilizerScheduleMode',
  share_reward: 'autoFarmShareRewardScheduleMode',
  mail_reward: 'autoFarmMailRewardScheduleMode',
  qian_xing_travel_reward: 'autoFarmQianXingTravelRewardScheduleMode',
  xing_su_auto_light_up: 'autoFarmXingSuAutoLightUpScheduleMode',
  limited_seed_draw: 'autoFarmLimitedSeedDrawScheduleMode',
};

export function isSchedulerTaskIntervalDisabled(config: Record<string, any>, taskId: string) {
  const modeKey = schedulerTaskScheduleModeConfigKeyById[taskId];
  return Boolean(modeKey) && String(config?.[modeKey] || 'interval') === 'daily_time';
}

function formatInterval(seconds: number) {
  if (seconds >= 3600 && seconds % 3600 === 0) return `${seconds / 3600}h`;
  if (seconds >= 60 && seconds % 60 === 0) return `${seconds / 60}m`;
  return `${seconds}s`;
}

function formatSchedulerTime(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const pad = (item: number) => String(item).padStart(2, '0');
  return `${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function mysteryShopDiscountText(discount: number) {
  if (!Number.isFinite(discount) || discount <= 0) return '无折扣';
  if (discount % 10 === 0) return `${discount / 10}折`;
  return `${Number((discount / 10).toFixed(1))}折`;
}

function taskLastRunAt(task: FarmAutomationSchedulerTask) {
  return task.lastFinishedAt || task.lastStartedAt || '';
}

function minutesFromSeconds(seconds: number) {
  return Math.max(1, Math.ceil(seconds / 60));
}

function rewardTimingKeys(intervalSecKey: string) {
  const prefix = intervalSecKey.replace(/IntervalSec$/, '');
  return {
    intervalMinKey: `${prefix}IntervalMin`,
    scheduleModeKey: `${prefix}ScheduleMode`,
    scheduleTimeKey: `${prefix}ScheduleTime`,
  };
}

function actionResultTitle(result: FarmAutomationActionResult) {
  if (result.status === 'ok' || result.ok) return '执行成功';
  if (result.status === 'runtime_not_ready') return '运行时未就绪';
  if (result.status === 'runtime_ready') return '运行时已连通';
  if (result.status === 'not_migrated') return '脚本未迁移';
  if (result.status === 'failed') return '执行失败';
  return result.taskId || '手动任务';
}

function isMysteryShopActionResult(result: FarmAutomationActionResult | null | undefined): result is FarmAutomationActionResult {
  return result?.taskId === 'mystery_shop_read' || result?.taskId === 'mystery_shop_auto_buy';
}

function mysteryShopActionResultCopy(result: FarmAutomationActionResult) {
  if (result.taskId === 'mystery_shop_read') {
    const countMatch = result.message.match(/商品\s*(\d+)\s*个/);
    const count = countMatch ? Number(countMatch[1]) : null;
    if (Number.isFinite(count)) {
      return {
        title: `当前协议返回 ${count} 个商品`,
        detail: count === 0 ? '当前没有读取到神秘商店商品' : result.message,
      };
    }
    return {
      title: result.ok ? '读取当前神秘商店商品完成' : '读取当前神秘商店商品失败',
      detail: result.message,
    };
  }
  return {
    title: result.ok ? '按预设自动购买已执行' : '按预设自动购买失败',
    detail: result.message,
  };
}

function usesBackpackPlantStrategy(config: Record<string, any>) {
  return config.autoFarmPlantPrimaryMode === 'backpack_first' || config.autoFarmPlantSecondaryMode === 'backpack_first';
}

function enforceGuardOnlyHelpAvailability(state: FarmAutomationState, dogGuardFriendCount: number): FarmAutomationState {
  if (dogGuardFriendCount > 0 || state.config.autoFarmFriendHelpGuardDogOnly !== true) return state;
  return {
    ...state,
    config: {
      ...state.config,
      autoFarmFriendHelpGuardDogOnly: false,
    },
  };
}

export function AutomationView({
  state,
  remote = false,
  dogGuardFriendCount = 0,
  onRunTask,
  onToggleAutomation,
  onSaveState,
  onSetRunMode,
  onRefreshBackpackSeeds,
  onRefreshStealCropOptions,
  lastActionResult,
  onActionResultConsumed,
  mysteryShopPurchaseRecords = [],
  initialSchedulerOpen = false,
  initialSettingsGroupId = '',
}: AutomationViewProps) {
  const [schedulerOpen, setSchedulerOpen] = useState(initialSchedulerOpen);
  const [settingsGroup, setSettingsGroup] = useState<FarmAutomationFeatureGroup | null>(
    () => state.featureGroups.find((group) => group.id === initialSettingsGroupId) || null,
  );
  const [draftState, setDraftStateValue] = useState(() => enforceGuardOnlyHelpAvailability(cloneAutomationState(state), dogGuardFriendCount));
  const draftStateRef = useRef(draftState);
  const [saving, setSaving] = useState(false);
  const [saveMessage, setSaveMessage] = useState('');
  const [seedRefreshing, setSeedRefreshing] = useState(false);
  const [seedRefreshMessage, setSeedRefreshMessage] = useState('');
  const [cropOptionsOpen, setCropOptionsOpen] = useState(false);
  const [cropRefreshing, setCropRefreshing] = useState(false);
  const [cropRefreshMessage, setCropRefreshMessage] = useState('');
  const [visibleActionResult, setVisibleActionResult] = useState<FarmAutomationActionResult | null>(lastActionResult || null);
  const [saveToastVisible, setSaveToastVisible] = useState(false);
  const [automationToastMessage, setAutomationToastMessage] = useState('');
  const [automationToggling, setAutomationToggling] = useState(false);
  const [runModeOpen, setRunModeOpen] = useState(false);
  const [runModeConfirmationOpen, setRunModeConfirmationOpen] = useState(false);
  const [selectedRunMode, setSelectedRunMode] = useState<FarmAutomationRunMode | null>(null);
  const [runModeSaving, setRunModeSaving] = useState(false);
  const [runModeError, setRunModeError] = useState('');
  const onActionResultConsumedRef = useRef(onActionResultConsumed);
  const autoRefreshedBackpackSettingsKey = useRef('');
  const autoRefreshedCropOptionsKey = useRef('');
  const schedulerState = schedulerOpen ? draftState : state;
  const enabledTasks = state.scheduler.tasks.filter((task) => task.enabled).length;
  const totalTasks = state.scheduler.tasks.length;
  const currentTaskLabel = taskLabelById(state.scheduler.tasks, state.scheduler.runningTaskId) || '等待中';
  const nextTaskLabel = nextSchedulerTaskLabel(state.scheduler.tasks);

  function setDraftState(update: SetStateAction<FarmAutomationState>) {
    const next = typeof update === 'function' ? update(draftStateRef.current) : update;
    draftStateRef.current = next;
    setDraftStateValue(next);
  }

  function draftFromIncoming(next: FarmAutomationState) {
    return enforceGuardOnlyHelpAvailability(cloneAutomationState(next), dogGuardFriendCount);
  }

  function saveableDraft() {
    return enforceGuardOnlyHelpAvailability(normalizeAutomationState(draftStateRef.current), dogGuardFriendCount);
  }

  useEffect(() => {
    if (!lastActionResult) {
      setVisibleActionResult(null);
      return;
    }
    setVisibleActionResult(lastActionResult);
    const timer = window.setTimeout(() => setVisibleActionResult(null), AUTOMATION_TOAST_AUTO_DISMISS_MS);
    return () => window.clearTimeout(timer);
  }, [lastActionResult]);

  useEffect(() => {
    onActionResultConsumedRef.current = onActionResultConsumed;
  }, [onActionResultConsumed]);

  useEffect(() => () => onActionResultConsumedRef.current?.(), []);

  useEffect(() => {
    if (!saveToastVisible) return;
    const timer = window.setTimeout(() => setSaveToastVisible(false), SAVE_TOAST_AUTO_DISMISS_MS);
    return () => window.clearTimeout(timer);
  }, [saveToastVisible]);

  useEffect(() => {
    if (!automationToastMessage) return;
    const timer = window.setTimeout(() => setAutomationToastMessage(''), SAVE_TOAST_AUTO_DISMISS_MS);
    return () => window.clearTimeout(timer);
  }, [automationToastMessage]);

  useEffect(() => {
    if (!schedulerOpen && !settingsGroup) {
      setDraftState(draftFromIncoming(state));
      return;
    }
    setDraftState((current) => enforceGuardOnlyHelpAvailability(mergeAutomationRuntimeState(current, state), dogGuardFriendCount));
  }, [schedulerOpen, settingsGroup, state]);

  useEffect(() => {
    setDraftState((current) => enforceGuardOnlyHelpAvailability(current, dogGuardFriendCount));
  }, [dogGuardFriendCount]);

  useEffect(() => {
    if (settingsGroup?.id !== 'planting' || !onRefreshBackpackSeeds) return;
    if (!usesBackpackPlantStrategy(draftState.config)) return;
    if (Array.isArray(draftState.config.autoFarmPlantBackpackSeedOptions) && draftState.config.autoFarmPlantBackpackSeedOptions.length > 0) return;
    if (autoRefreshedBackpackSettingsKey.current === 'planting') return;
    autoRefreshedBackpackSettingsKey.current = 'planting';
    void refreshBackpackSeeds(false);
  }, [
    settingsGroup?.id,
    onRefreshBackpackSeeds,
    draftState.config.autoFarmPlantPrimaryMode,
    draftState.config.autoFarmPlantSecondaryMode,
  ]);

  useEffect(() => {
    if (settingsGroup?.id !== 'friends' || !onRefreshStealCropOptions) return;
    if (!remote && Array.isArray(draftState.config.autoFarmFriendStealCropOptions) && draftState.config.autoFarmFriendStealCropOptions.length > 0) return;
    if (autoRefreshedCropOptionsKey.current === 'friends') return;
    autoRefreshedCropOptionsKey.current = 'friends';
    void refreshStealCropOptions();
  }, [settingsGroup?.id, onRefreshStealCropOptions, remote]);

  function openScheduler() {
    setDraftState(draftFromIncoming(state));
    setSaveMessage('');
    setSchedulerOpen(true);
  }

  function openSettings(group: FarmAutomationFeatureGroup) {
    setDraftState(draftFromIncoming(state));
    autoRefreshedBackpackSettingsKey.current = '';
    autoRefreshedCropOptionsKey.current = '';
    setSaveMessage('');
    setSeedRefreshMessage('');
    setCropRefreshMessage('');
    setSettingsGroup(group);
  }

  function updateScheduler(next: Partial<FarmAutomationState['scheduler']>) {
    setDraftState((current) =>
      normalizeAutomationState({
        ...current,
        scheduler: {
          ...current.scheduler,
          ...next,
        },
      }),
    );
  }

  function updateTask(taskId: string, next: Partial<FarmAutomationSchedulerTask>) {
    if (taskId === 'auto_warehouse_sell' && next.intervalSec !== undefined) {
      next = { ...next, intervalSec: Math.max(60, Math.ceil(Number(next.intervalSec) / 60) * 60) };
    }
    setDraftState((current) => {
      const currentTask = current.scheduler.tasks.find((task) => task.id === taskId);
      let nextState: FarmAutomationState = {
        ...current,
        config: currentTask ? schedulerConfigForTaskUpdate(current.config, currentTask, next) : current.config,
        scheduler: {
          ...current.scheduler,
          tasks: current.scheduler.tasks.map((task) => (task.id === taskId ? { ...task, ...next } : task)),
        },
      };
      if (taskId === 'own_fertilizer' && typeof next.enabled === 'boolean') {
        nextState = syncFertilizerMasterEnabledState(nextState, next.enabled);
      }
      return normalizeAutomationState(nextState, { intervalSource: 'scheduler' });
    });
  }

  function updateConfig(key: string, value: any) {
    setDraftState((current) =>
      normalizeAutomationState({
        ...current,
        config: {
          ...current.config,
          [key]: value,
        },
      }),
    );
  }

  async function refreshBackpackSeeds(forceRefresh = true) {
    if (!onRefreshBackpackSeeds || seedRefreshing) return;
    setSeedRefreshing(true);
    setSeedRefreshMessage('');
    try {
      const result = await onRefreshBackpackSeeds({
        ...draftState.config,
        refresh: forceRefresh,
      });
      const list = backpackSeedListFromRefreshResult(result);
      setDraftState((current) =>
        normalizeAutomationState({
          ...current,
          config: {
            ...current.config,
            autoFarmPlantBackpackSeedOptions: list,
          },
        }),
      );
      setSeedRefreshMessage(list.length > 0 ? `已刷新 ${list.length} 个背包种子` : '未读取到背包种子');
    } catch (error) {
      setSeedRefreshMessage(error instanceof Error ? error.message : String(error));
    } finally {
      setSeedRefreshing(false);
    }
  }

  async function refreshStealCropOptions() {
    if (!onRefreshStealCropOptions || cropRefreshing) return;
    setCropRefreshing(true);
    setCropRefreshMessage('');
    try {
      const result = await onRefreshStealCropOptions();
      const list = stealCropOptionsFromRefreshResult(result);
      setDraftState((current) =>
        normalizeAutomationState({
          ...current,
          config: {
            ...current.config,
            autoFarmFriendStealCropOptions: list,
          },
        }),
      );
      setCropRefreshMessage(list.length > 0 ? `已加载 ${list.length} 个作物` : '未读取到作物选项');
    } catch (error) {
      setCropRefreshMessage(error instanceof Error ? error.message : String(error));
    } finally {
      setCropRefreshing(false);
    }
  }

  async function saveScheduler() {
    if (!onSaveState) return;
    setSaving(true);
    setSaveMessage('');
    try {
      const saved = await onSaveState(saveableDraft());
      if (saved) {
        setDraftState(draftFromIncoming(saved));
      }
      setSaveMessage('调度配置已保存');
      setVisibleActionResult(null);
      setSaveToastVisible(true);
    } catch (error) {
      setSaveMessage(error instanceof Error ? error.message : String(error));
      setSaveToastVisible(false);
    } finally {
      setSaving(false);
    }
  }

  async function saveSettings() {
    if (!onSaveState) return;
    const validationError =
      friendStealRandomDelayValidationError(draftStateRef.current.config) ||
      plantRandomDelayValidationError(draftStateRef.current.config);
    if (validationError) {
      setSaveMessage(validationError);
      setSaveToastVisible(false);
      return;
    }
    setSaving(true);
    setSaveMessage('');
    try {
      const saved = await onSaveState(saveableDraft());
      if (saved) {
        setDraftState(draftFromIncoming(saved));
      }
      setSaveMessage('设置已保存');
      setVisibleActionResult(null);
      setSaveToastVisible(true);
    } catch (error) {
      setSaveMessage(error instanceof Error ? error.message : String(error));
      setSaveToastVisible(false);
    } finally {
      setSaving(false);
    }
  }

  async function toggleFeatureGroup(groupId: string, enabled: boolean) {
    let nextState = {
      ...state,
      featureGroups: state.featureGroups.map((group) => (group.id === groupId ? { ...group, enabled } : group)),
    };
    if (groupId === 'mystery_shop') {
      nextState = syncMysteryShopFeatureCardState(nextState, enabled);
    }
    nextState = enforceGuardOnlyHelpAvailability(normalizeAutomationState(nextState), dogGuardFriendCount);
    setDraftState(nextState);
    if (!onSaveState) return;
    setSaving(true);
    setSaveMessage('');
    try {
      const saved = await onSaveState(nextState);
      if (saved) {
        setDraftState(draftFromIncoming(saved));
      }
    } catch (error) {
      setSaveMessage(error instanceof Error ? error.message : String(error));
    } finally {
      setSaving(false);
    }
  }

  async function toggleAutomation() {
    if (!onToggleAutomation || automationToggling) return;
    setAutomationToggling(true);
    setAutomationToastMessage('');
    try {
      const nextRunning = !state.running;
      await onToggleAutomation(nextRunning);
      setAutomationToastMessage(nextRunning ? '启动成功' : '自动化已停止');
    } catch (error) {
      setAutomationToastMessage(error instanceof Error ? error.message : String(error));
    } finally {
      setAutomationToggling(false);
    }
  }

  const currentRunMode: FarmAutomationRunMode = state.runMode === 'god' ? 'god' : 'safe';
  const selectedRunModeLabel = selectedRunMode === 'god' ? '仙人模式' : '安全模式';

  function openRunMode() {
    setSelectedRunMode(currentRunMode);
    setRunModeError('');
    setRunModeOpen(true);
  }

  function openRunModeConfirmation() {
    if (!selectedRunMode || selectedRunMode === currentRunMode) return;
    setRunModeError('');
    setRunModeOpen(false);
    setRunModeConfirmationOpen(true);
  }

  function returnToRunModeSelection() {
    if (runModeSaving) return;
    setRunModeError('');
    setRunModeConfirmationOpen(false);
    setRunModeOpen(true);
  }

  async function saveRunMode() {
    if (!onSetRunMode || !selectedRunMode || selectedRunMode === currentRunMode || runModeSaving) return;
    setRunModeSaving(true);
    setRunModeError('');
    try {
      const saved = await onSetRunMode(selectedRunMode);
      if (saved) setDraftState(draftFromIncoming(saved));
      setRunModeConfirmationOpen(false);
      setRunModeOpen(false);
      setAutomationToastMessage(`已切换到${selectedRunModeLabel}`);
    } catch (error) {
      setRunModeError(error instanceof Error ? error.message : String(error));
    } finally {
      setRunModeSaving(false);
    }
  }

  function configBool(key: string, fallback = false) {
    return boolFromConfig(draftState.config[key], fallback);
  }

  function configNumber(key: string, fallback: number) {
    return numberFromConfig(draftState.config[key], fallback);
  }

  function configValue(key: string, fallback: any) {
    return draftState.config[key] ?? fallback;
  }

  const automationStarting = automationToggling && !state.running;

  function renderAutomationToggle(className: string) {
    return (
      <button
        aria-busy={automationToggling || undefined}
        aria-label={automationToggling
          ? state.running ? '正在停止自动化' : '正在启动自动化'
          : state.running ? '停止自动化' : '启动自动化'}
        className={`primary-button async-action-button ${className}`}
        disabled={automationToggling || !onToggleAutomation}
        type="button"
        onClick={toggleAutomation}
      >
        {automationStarting ? <Loader2 className="spin" size={16} /> : <Power size={16} />}
        <span>{automationStarting ? '启动中' : automationToggling ? '处理中' : state.running ? '停止' : '启动'}</span>
      </button>
    );
  }

  return (
    <section className="view-stack fill automation-view">
      <header className="page-header automation-header">
        <div>
          <h1>农场自动化</h1>
        </div>
        <div className="header-actions">
          <button aria-label="修改运行模式" className="automation-run-mode-button" disabled={!onSetRunMode} type="button" onClick={openRunMode}>
            <span>运行模式：</span>
            <strong>{currentRunMode === 'safe' ? '安全模式' : '仙人模式'}</strong>
            <Settings size={15} />
          </button>
          <button className="secondary-button automation-wide-action dark" type="button" onClick={openScheduler}>
            <CalendarClock size={16} />
            调度中心
          </button>
          {renderAutomationToggle('automation-header-toggle')}
        </div>
      </header>

      <div className="automation-runtime-panel">
        <div className="automation-summary-strip">
          <div>
            <span>状态</span>
            <strong>{state.running ? '运行中' : '已停止'}</strong>
          </div>
          <div>
            <span>启用任务</span>
            <strong>
              {enabledTasks} / {totalTasks}
            </strong>
          </div>
          <div>
            <span>本轮</span>
            <strong>{currentTaskLabel}</strong>
          </div>
          <div>
            <span>下次执行</span>
            <strong>{nextTaskLabel}</strong>
          </div>
          <div className="automation-mobile-run-control">
            {renderAutomationToggle('automation-runtime-toggle')}
          </div>
        </div>

        {visibleActionResult ? (
          <div
            className={visibleActionResult.ok ? 'social-toast ok automation-action-toast' : 'social-toast automation-action-toast'}
            role="status"
            aria-live="polite"
          >
            {visibleActionResult.ok ? <Check size={16} /> : <X size={16} />}
            <span>
              {actionResultTitle(visibleActionResult)}：{visibleActionResult.message}
            </span>
          </div>
        ) : null}
        {automationToastMessage ? (
          <div className="social-toast ok settings-save-toast automation-session-toast" role="status" aria-live="polite">
            <Check size={16} />
            <span>{automationToastMessage}</span>
          </div>
        ) : null}
      </div>

      <div className="automation-feature-grid">
        {groupAutomationFeatureGroupsForMobile(state.featureGroups).map((section) => (
          <section className="automation-feature-section" key={section.id}>
            <h2 className="automation-feature-section-title">{section.label}</h2>
            <div className="automation-feature-section-list">
              {section.groups.map((group) => (
                <article className="automation-feature-card" key={group.id}>
                  <div className="automation-feature-icon">
                    <Bot size={18} />
                  </div>
                  <div className="automation-feature-copy">
                    <strong>{group.label}</strong>
                    <p>{group.summary}</p>
                  </div>
                  <button
                    aria-label={`${group.enabled ? '关闭' : '开启'}${group.label}`}
                    className={group.enabled ? 'automation-switch on' : 'automation-switch'}
                    disabled={saving}
                    type="button"
                    onClick={() => toggleFeatureGroup(group.id, !group.enabled)}
                  >
                    {group.enabled ? 'ON' : 'OFF'}
                  </button>
                  <button className="icon-button light" type="button" aria-label={`${group.label}设置`} onClick={() => openSettings(group)}>
                    <Settings size={16} />
                  </button>
                </article>
              ))}
            </div>
          </section>
        ))}
      </div>

      {schedulerOpen ? (
        <div className="dialog-backdrop automation-dialog-backdrop" role="presentation">
          <section className="automation-scheduler-dialog" role="dialog" aria-label="调度中心">
            <header>
              <div>
                <h2>调度中心</h2>
                <p>任务按优先级和间隔执行；启用状态会同步到对应自动化功能。</p>
              </div>
              <button aria-label="关闭调度中心" className="icon-button light" type="button" onClick={() => setSchedulerOpen(false)}>
                <X size={16} />
              </button>
            </header>
            <div className="automation-scheduler-controls">
              <label className="automation-scheduler-check">
                <input
                  checked={schedulerState.scheduler.enabled}
                  name="scheduler-enabled"
                  type="checkbox"
                  onChange={(event) => updateScheduler({ enabled: event.target.checked })}
                />
                <span>调度启用</span>
              </label>
              <label>
                最小间隔(ms)
                <input
                  min={0}
                  name="scheduler-min-gap"
                  type="number"
                  value={schedulerState.scheduler.minGapMs}
                  onChange={(event) => updateScheduler({ minGapMs: numberFromInput(event.currentTarget.value, 350) })}
                />
              </label>
              <button className="automation-save-button" disabled={saving || !onSaveState} type="button" onClick={saveScheduler}>
                <Save size={14} />
                {saving ? '保存中' : '保存调度'}
              </button>
              {saveMessage ? <span className="automation-save-message">{saveMessage}</span> : null}
            </div>
            <div className="automation-task-table">
              <div className="automation-task-row head">
                <span>任务</span>
                <span>优先级</span>
                <span>间隔</span>
                <span>上次执行</span>
                <span>预计下次</span>
                <span>状态</span>
                <span>执行</span>
              </div>
              {schedulerState.scheduler.tasks.map((task) => (
                <div className="automation-task-row" key={task.id}>
                  <div className="automation-task-name">
                    <strong>{task.label}</strong>
                    {task.dailyDoneToday ? <span className="automation-task-done-badge">今日已完成</span> : null}
                  </div>
                  <label className="automation-task-number-field automation-task-priority-field">
                    <span>优先级</span>
                    <input
                      min={1}
                      name={`priority-${task.id}`}
                      type="number"
                      value={task.priority}
                      onChange={(event) => updateTask(task.id, { priority: numberFromInput(event.currentTarget.value, task.priority) })}
                    />
                  </label>
                  <label className="automation-task-number-field automation-task-interval-field">
                    <span>间隔(秒)</span>
                    <input
                      disabled={isSchedulerTaskIntervalDisabled(schedulerState.config, task.id)}
                      min={1}
                      name={`interval-${task.id}`}
                      type="number"
                      value={task.intervalSec}
                      onChange={(event) => updateTask(task.id, { intervalSec: numberFromInput(event.currentTarget.value, task.intervalSec) })}
                      title={formatInterval(task.intervalSec)}
                    />
                  </label>
                  <span className="automation-task-time automation-task-last-time" title={taskLastRunAt(task)}>
                    {formatSchedulerTime(taskLastRunAt(task))}
                  </span>
                  <span className="automation-task-time automation-task-next-time" title={task.nextRunAt || ''}>
                    {formatSchedulerTime(task.nextRunAt)}
                  </span>
                  <div className="automation-task-actions">
                    <label className="automation-task-check">
                      <input
                        checked={task.enabled}
                        name={`enabled-${task.id}`}
                        type="checkbox"
                        onChange={(event) => updateTask(task.id, { enabled: event.target.checked })}
                      />
                      <span>{task.enabled ? '启用' : '关闭'}</span>
                    </label>
                    <button className="automation-run-button" type="button" onClick={() => onRunTask(task.id)}>
                      <Play size={13} />
                      跑
                    </button>
                  </div>
                </div>
              ))}
            </div>
            <footer className="automation-scheduler-footer">
              <button aria-label="取消调度编辑" className="secondary-button" disabled={saving} type="button" onClick={() => setSchedulerOpen(false)}>
                取消
              </button>
              <button aria-label="保存调度" className="primary-button" disabled={saving || !onSaveState} type="button" onClick={saveScheduler}>
                <Save size={14} />
                {saving ? '保存中' : '保存调度'}
              </button>
            </footer>
          </section>
        </div>
      ) : null}

      {settingsGroup ? (
        <div className="dialog-backdrop automation-dialog-backdrop" role="presentation">
          <section className="automation-settings-dialog" role="dialog" aria-label={`${settingsGroup.label}设置`}>
            <header>
              <div>
                <h2>{settingsGroup.label}</h2>
                <p>{settingsGroup.summary}</p>
              </div>
              <button aria-label={`${settingsGroup.label}设置关闭`} className="icon-button light" type="button" onClick={() => setSettingsGroup(null)}>
                <X size={16} />
              </button>
            </header>
            <div className="automation-settings-body">
              <fieldset disabled={!settingsGroup.enabled}>
                {renderSettingsGroup(settingsGroup, draftState.scheduler.tasks, onRunTask, lastActionResult, mysteryShopPurchaseRecords, configBool, configNumber, configValue, updateConfig, {
                  canRefreshBackpackSeeds: !!onRefreshBackpackSeeds,
                  seedRefreshing,
                  seedRefreshMessage,
                  refreshBackpackSeeds,
                  canRefreshStealCropOptions: !!onRefreshStealCropOptions,
                  cropRefreshing,
                  cropRefreshMessage,
                  refreshStealCropOptions,
                  cropOptionsOpen,
                  setCropOptionsOpen,
                  dogGuardFriendCount,
                })}
              </fieldset>
            </div>
            <footer className="automation-settings-footer">
              <button className="automation-save-button" disabled={saving || !onSaveState} type="button" onClick={saveSettings}>
                <Save size={14} />
                {saving ? '保存中' : '保存设置'}
              </button>
              {saveMessage ? <span className="automation-settings-save-message">{saveMessage}</span> : null}
            </footer>
          </section>
        </div>
      ) : null}
      {runModeOpen ? (
        <div className="dialog-backdrop automation-dialog-backdrop" role="presentation">
          <section className="automation-run-mode-dialog" role="dialog" aria-label="运行模式">
            <header>
              <div>
                <h2>运行模式</h2>
              </div>
              <button aria-label="关闭运行模式" className="icon-button light" disabled={runModeSaving} type="button" onClick={() => setRunModeOpen(false)}>
                <X size={16} />
              </button>
            </header>
            <div className="automation-run-mode-body">
              <div className="automation-run-mode-options">
                <button
                  aria-label="选择安全模式"
                  aria-pressed={selectedRunMode === 'safe'}
                  className={selectedRunMode === 'safe' ? 'automation-run-mode-option safe selected' : 'automation-run-mode-option safe'}
                  disabled={currentRunMode === 'safe'}
                  type="button"
                  onClick={() => setSelectedRunMode('safe')}
                >
                  <strong>安全模式</strong>
                  <p>慢一点，一件做完再做下一件，动作之间停一停。</p>
                  {currentRunMode === 'safe' ? <span>当前模式</span> : null}
                </button>
                <button
                  aria-label="选择仙人模式"
                  aria-pressed={selectedRunMode === 'god'}
                  className={selectedRunMode === 'god' ? 'automation-run-mode-option god selected' : 'automation-run-mode-option god'}
                  disabled={currentRunMode === 'god'}
                  type="button"
                  onClick={() => setSelectedRunMode('god')}
                >
                  <strong>仙人模式</strong>
                  <p>能一起做的尽量同时做，完成快、操作紧凑。</p>
                  {currentRunMode === 'god' ? <span>当前模式</span> : null}
                </button>
              </div>
              <p className="automation-run-mode-guardian">守护服务不受影响，会照常独立运行。</p>
              {runModeError ? <p className="automation-run-mode-error">{runModeError}</p> : null}
            </div>
            <footer>
              <button className="secondary-button" disabled={runModeSaving} type="button" onClick={() => setRunModeOpen(false)}>
                取消
              </button>
              <button aria-label="继续确认运行模式" className="primary-button" disabled={runModeSaving || !onSetRunMode || !selectedRunMode || selectedRunMode === currentRunMode} type="button" onClick={openRunModeConfirmation}>
                下一步
              </button>
            </footer>
          </section>
        </div>
      ) : null}
      {runModeConfirmationOpen ? (
        <div className="dialog-backdrop automation-dialog-backdrop" role="presentation">
          <section className="automation-run-mode-dialog confirmation" role="dialog" aria-label="确认运行模式">
            <header>
              <div>
                <h2>确认切换为{selectedRunModeLabel}</h2>
              </div>
              <button aria-label="关闭运行模式确认" className="icon-button light" disabled={runModeSaving} type="button" onClick={() => setRunModeConfirmationOpen(false)}>
                <X size={16} />
              </button>
            </header>
            <div className="automation-run-mode-body confirmation-body">
              <strong>{selectedRunMode === 'safe' ? '之后会慢一点，一件做完再做下一件。' : '之后能一起做的任务会尽量同时做，完成更快、操作更紧凑。'}</strong>
              <p>这会影响软件运行时的任务调度。守护服务不受影响，会照常独立运行。</p>
              {runModeError ? <p className="automation-run-mode-error">{runModeError}</p> : null}
            </div>
            <footer>
              <button aria-label="返回运行模式选择" className="secondary-button" disabled={runModeSaving} type="button" onClick={returnToRunModeSelection}>
                返回
              </button>
              <button aria-label="确认切换运行模式" className="primary-button" disabled={runModeSaving || !onSetRunMode || !selectedRunMode} type="button" onClick={saveRunMode}>
                <Check size={16} />
                {runModeSaving ? '保存中' : '确认切换'}
              </button>
            </footer>
          </section>
        </div>
      ) : null}
      <SaveSuccessToast visible={saveToastVisible} />
    </section>
  );
}

function cloneAutomationState(state: FarmAutomationState): FarmAutomationState {
  return {
    ...state,
    summary: { ...state.summary },
    config: { ...(state.config || {}) },
    featureGroups: state.featureGroups.map((group) => ({ ...group, settingKeys: [...group.settingKeys] })),
    scheduler: {
      ...state.scheduler,
      tasks: state.scheduler.tasks.map((task) => ({ ...task })),
    },
  };
}

export function syncFertilizerMasterEnabledState(state: FarmAutomationState, forcedEnabled?: boolean): FarmAutomationState {
  const fertilizerGroup = state.featureGroups.find((group) => group.id === 'fertilizer');
  const fertilizerTask = state.scheduler.tasks.find((task) => task.id === 'own_fertilizer');
  const enabled = typeof forcedEnabled === 'boolean'
    ? forcedEnabled
    : fertilizerGroup?.enabled ?? boolFromConfig(state.config.autoFarmFertilizerEnabled, fertilizerTask?.enabled ?? false);

  return {
    ...state,
    featureGroups: state.featureGroups.map((group) => (group.id === 'fertilizer' ? { ...group, enabled } : group)),
    config: {
      ...state.config,
      'autoFarmFeatureGroupEnabled.fertilizer': enabled,
      autoFarmFertilizerEnabled: enabled,
    },
    scheduler: {
      ...state.scheduler,
      tasks: state.scheduler.tasks.map((task) => (task.id === 'own_fertilizer' ? { ...task, enabled } : task)),
    },
  };
}

function syncMysteryShopFeatureCardState(state: FarmAutomationState, enabled: boolean): FarmAutomationState {
  return {
    ...state,
    config: {
      ...state.config,
      'autoFarmFeatureGroupEnabled.mystery_shop': enabled,
      autoFarmMysteryShopAutoBuyEnabled: enabled,
    },
    scheduler: {
      ...state.scheduler,
      tasks: state.scheduler.tasks.map((task) => (task.id === 'mystery_shop_auto_buy' ? { ...task, enabled } : task)),
    },
  };
}

function normalizeAutomationState(state: FarmAutomationState, options: { intervalSource?: 'config' | 'scheduler' } = {}): FarmAutomationState {
  state = syncFertilizerMasterEnabledState(state);
  const tasks = state.scheduler.tasks.map((task) => ({
    ...task,
    priority: Math.max(1, Math.trunc(Number(task.priority) || 1)),
    intervalSec: Math.max(1, Math.trunc(Number(task.intervalSec) || 1)),
  }));
  const enabledTasks = tasks.filter((task) => task.enabled).length;
  return syncAutomationSchedulerEnabledState(
    syncAutomationSchedulerIntervals(
      {
        ...state,
        config: { ...(state.config || {}) },
        summary: {
          ...state.summary,
          enabledTasks,
          totalTasks: tasks.length,
        },
        scheduler: {
          ...state.scheduler,
          minGapMs: Math.max(0, Math.trunc(Number(state.scheduler.minGapMs) || 0)),
          tasks,
        },
      },
      { source: options.intervalSource || 'config' },
    ),
  );
}

function schedulerConfigForTaskUpdate(
  config: Record<string, any>,
  task: FarmAutomationSchedulerTask,
  next: Partial<FarmAutomationSchedulerTask>,
): Record<string, any> {
  let changed = false;
  const nextConfig = { ...config };
  const enabledKey = schedulerEnabledConfigKey(task);
  if (typeof next.enabled === 'boolean' && enabledKey) {
    nextConfig[enabledKey] = next.enabled;
    changed = true;
  }
  const intervalKey = schedulerIntervalConfigKey(task);
  if (typeof next.intervalSec !== 'undefined' && intervalKey) {
    nextConfig[intervalKey] = normalizeIntervalSec(next.intervalSec);
    changed = true;
  }
  return changed ? nextConfig : config;
}

export function syncAutomationSchedulerIntervals(
  state: FarmAutomationState,
  options: { source?: 'config' | 'scheduler' } = {},
): FarmAutomationState {
  const source = options.source || 'config';
  const config = { ...(state.config || {}) };
  const tasks = state.scheduler.tasks.map((task) => {
    const intervalKey = schedulerIntervalConfigKey(task);
    const taskInterval = normalizeIntervalSec(task.intervalSec);
    if (!intervalKey) {
      return { ...task, intervalSec: taskInterval };
    }
    if (source === 'scheduler') {
      config[intervalKey] = taskInterval;
      return { ...task, intervalSec: taskInterval };
    }
    const configInterval = normalizeIntervalSec(config[intervalKey] ?? taskInterval);
    config[intervalKey] = configInterval;
    return { ...task, intervalSec: configInterval };
  });
  return {
    ...state,
    config,
    summary: {
      ...state.summary,
      enabledTasks: tasks.filter((task) => task.enabled).length,
      totalTasks: tasks.length,
    },
    scheduler: {
      ...state.scheduler,
      tasks,
    },
  };
}

export function syncAutomationSchedulerEnabledState(state: FarmAutomationState): FarmAutomationState {
  const config = { ...(state.config || {}) };
  const tasks = state.scheduler.tasks.map((task) => {
    const enabledKey = schedulerEnabledConfigKey(task);
    if (!enabledKey || !Object.prototype.hasOwnProperty.call(config, enabledKey)) return task;
    const enabled = boolFromConfig(config[enabledKey], task.enabled);
    config[enabledKey] = enabled;
    return { ...task, enabled };
  });
  return {
    ...state,
    config,
    summary: {
      ...state.summary,
      enabledTasks: tasks.filter((task) => task.enabled).length,
      totalTasks: tasks.length,
    },
    scheduler: {
      ...state.scheduler,
      tasks,
    },
  };
}

function schedulerEnabledConfigKey(task: FarmAutomationSchedulerTask) {
  return task.enabledConfigKey || schedulerTaskConfigKeyById[task.id] || '';
}

function schedulerIntervalConfigKey(task: FarmAutomationSchedulerTask) {
  return task.intervalConfigKey || schedulerTaskIntervalConfigKeyById[task.id] || '';
}

function schedulerConfigKeysForTask(tasks: FarmAutomationSchedulerTask[], taskId: string) {
  const task = tasks.find((item) => item.id === taskId);
  return {
    enabledKey: task ? schedulerEnabledConfigKey(task) : schedulerTaskConfigKeyById[taskId] || '',
    intervalKey: task ? schedulerIntervalConfigKey(task) : schedulerTaskIntervalConfigKeyById[taskId] || '',
  };
}

function normalizeIntervalSec(value: any) {
  return Math.max(1, Math.trunc(Number(value) || 1));
}

function taskLabelById(tasks: FarmAutomationSchedulerTask[], taskId?: string) {
  if (!taskId) return '';
  return tasks.find((task) => task.id === taskId)?.label || taskId;
}

function nextSchedulerTaskLabel(tasks: FarmAutomationSchedulerTask[]) {
  const enabled = tasks.filter((task) => task.enabled);
  if (enabled.length === 0) return '-';
  const withNextRun = enabled
    .map((task) => ({ task, time: task.nextRunAt ? new Date(task.nextRunAt).getTime() : Number.NaN }))
    .filter((item) => Number.isFinite(item.time))
    .sort((left, right) => left.time - right.time || right.task.priority - left.task.priority);
  if (withNextRun.length > 0) return withNextRun[0].task.label;
  return [...enabled].sort((left, right) => right.priority - left.priority)[0]?.label || '-';
}

export function mergeAutomationRuntimeState(draft: FarmAutomationState, refreshed: FarmAutomationState): FarmAutomationState {
  const refreshedTasks = new Map(refreshed.scheduler.tasks.map((task) => [task.id, task]));
  const mergedTasks = draft.scheduler.tasks.map((task) => {
    const refreshedTask = refreshedTasks.get(task.id);
    if (!refreshedTask) return task;
    return {
      ...task,
      dailyDoneToday: refreshedTask.dailyDoneToday,
      nextRunAt: refreshedTask.nextRunAt,
      lastStartedAt: refreshedTask.lastStartedAt,
      lastFinishedAt: refreshedTask.lastFinishedAt,
      lastSuccessAt: refreshedTask.lastSuccessAt,
      lastError: refreshedTask.lastError,
      lastResultSummary: refreshedTask.lastResultSummary,
    };
  });
  return {
    ...draft,
    running: refreshed.running,
    summary: {
      ...refreshed.summary,
      enabledTasks: mergedTasks.filter((task) => task.enabled).length,
      totalTasks: mergedTasks.length,
    },
    config: mergeDailyDoneConfig(draft.config, refreshed.config),
    scheduler: {
      ...draft.scheduler,
      runningTaskId: refreshed.scheduler.runningTaskId,
      tasks: mergedTasks,
    },
  };
}

function mergeDailyDoneConfig(draftConfig: Record<string, any>, refreshedConfig: Record<string, any>) {
  const next = { ...(draftConfig || {}) };
  Object.entries(refreshedConfig || {}).forEach(([key, value]) => {
    if (key.endsWith('DailyDoneDate')) {
      next[key] = value;
    }
  });
  return next;
}

function numberFromInput(value: string, fallback: number) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return fallback;
  return Math.trunc(parsed);
}

const AUTO_PLANT_RANDOM_DELAY_MAX_MS = 10_000;

function plantRandomDelayValidationError(config: Record<string, any>) {
  const min = Number(config.autoFarmPlantRandomizedDelayMinMs ?? 100);
  const max = Number(config.autoFarmPlantRandomizedDelayMaxMs ?? 500);
  if (
    !Number.isInteger(min) ||
    !Number.isInteger(max) ||
    min < 0 ||
    max < 0 ||
    min > AUTO_PLANT_RANDOM_DELAY_MAX_MS ||
    max > AUTO_PLANT_RANDOM_DELAY_MAX_MS
  ) {
    return '种植随机延迟必须是 0 到 10000 的整数';
  }
  if (max < min) {
    return '最大种植随机延迟不能小于最小种植随机延迟';
  }
  return '';
}

function friendStealRandomDelayValidationError(config: Record<string, any>) {
  const min = Number(config.autoFarmFriendStealRandomDelayMinMs ?? 1000);
  const max = Number(config.autoFarmFriendStealRandomDelayMaxMs ?? 5000);
  if (!Number.isInteger(min) || !Number.isInteger(max) || min < 0 || max < 0) {
    return '随机偷取延迟必须是非负整数';
  }
  if (max < min) {
    return '最大随机偷取延迟不能小于最小随机偷取延迟';
  }
  return '';
}

function boolFromConfig(value: any, fallback: boolean) {
  if (typeof value === 'boolean') return value;
  if (typeof value === 'number') return value !== 0;
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase();
    if (['1', 'true', 'yes', 'on'].includes(normalized)) return true;
    if (['0', 'false', 'no', 'off'].includes(normalized)) return false;
  }
  return fallback;
}

function numberFromConfig(value: any, fallback: number) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return fallback;
  return Math.trunc(parsed);
}

function numberListText(value: any) {
  if (!Array.isArray(value)) return '';
  return value.map((item) => String(Math.trunc(Number(item) || 0))).filter((item) => item !== '0').join(', ');
}

function numberListFromText(value: string) {
  const seen = new Set<number>();
  return value
    .split(/[\s,，;；]+/)
    .map((item) => Math.trunc(Number(item)))
    .filter((item) => {
      if (!Number.isFinite(item) || item <= 0 || seen.has(item)) return false;
      seen.add(item);
      return true;
    });
}

function numberListFromConfig(value: any) {
  if (!Array.isArray(value)) return [];
  return value.map((item) => Math.trunc(Number(item))).filter((item) => Number.isFinite(item) && item > 0);
}

function backpackSeedListFromRefreshResult(value: any) {
  if (Array.isArray(value)) return value;
  if (Array.isArray(value?.list)) return value.list;
  if (Array.isArray(value?.data?.list)) return value.data.list;
  if (value?.ok === false) throw new Error(String(value.error || '背包种子刷新失败'));
  return [];
}

function stealCropOptionsFromRefreshResult(value: any) {
  if (value?.ok === false) throw new Error(String(value.error || '作物选项读取失败'));
  if (Array.isArray(value)) return stealCropOptionsFromConfig(value);
  if (Array.isArray(value?.list)) return stealCropOptionsFromConfig(value.list);
  if (Array.isArray(value?.data?.list)) return stealCropOptionsFromConfig(value.data.list);
  return [];
}

function stringListFromConfig(value: any) {
  if (!Array.isArray(value)) return [];
  return value.map((item) => String(item || '').trim()).filter(Boolean);
}

function stringListText(value: any) {
  return stringListFromConfig(value).join('\n');
}

function stringListFromText(value: string) {
  const seen = new Set<string>();
  return value
    .split(/[\r\n,，;；]+/)
    .map((item) => item.trim())
    .filter((item) => {
      if (!item || seen.has(item)) return false;
      seen.add(item);
      return true;
    });
}

function toggleStringList(current: string[], value: string, enabled: boolean) {
  if (enabled) return current.includes(value) ? current : [...current, value];
  return current.filter((item) => item !== value);
}

function toggleNumberList(current: number[], value: number, enabled: boolean) {
  if (enabled) return current.includes(value) ? current : [...current, value];
  return current.filter((item) => item !== value);
}

function firstFiniteNumber(...values: any[]) {
  for (const value of values) {
    const parsed = Math.trunc(Number(value));
    if (Number.isFinite(parsed) && parsed > 0) return parsed;
  }
  return 0;
}

function firstFiniteCount(...values: any[]) {
  for (const value of values) {
    const parsed = Math.trunc(Number(value));
    if (Number.isFinite(parsed) && parsed >= 0) return parsed;
  }
  return null;
}

function backpackSeedOptionsFromConfig(value: any, priorityIds: number[], disabledIds: number[]): BackpackSeedOption[] {
  const rawOptions = Array.isArray(value) ? value : [];
  const prioritySet = new Set(priorityIds);
  const disabledSet = new Set(disabledIds);
  const byId = new Map<number, BackpackSeedOption>();

  rawOptions.forEach((item) => {
    const seedId = firstFiniteNumber(item?.seedId, item?.id, item?.itemId, item?.seed_id);
    if (!seedId) return;
    // Trust backend/runtime filtering; drop obvious non-seed leftovers from stale local config.
    if (isExplicitNonSeedOption(item)) return;
    const level = firstFiniteNumber(item?.level, item?.requiredLevel, item?.landLevelNeed, item?.land_level_need);
    const count = firstFiniteCount(item?.count, item?.stock, item?.inventory, item?.quantity);
    byId.set(seedId, {
      seedId,
      name: String(item?.name || item?.seedName || item?.label || '未知种子'),
      level: level || null,
      count,
      priority: prioritySet.has(seedId),
      disabled: disabledSet.has(seedId) || item?.disabled === true,
      plantable: item?.plantable !== false,
      plantableReason: typeof item?.plantableReason === 'string' ? item.plantableReason : undefined,
      plantableMessage: typeof item?.plantableMessage === 'string' ? item.plantableMessage : undefined,
      plantSize: firstFiniteNumber(item?.plantSize, item?.size, item?.gridSize) || null,
    });
  });

  // Do not invent placeholder rows for priority IDs missing from the refreshed seed list.
  // Force-priority retention is handled by FarmBackpackSeedOptions on refresh; inventing
  // "未知种子" here reintroduces stale non-seed IDs (dog food / gift packs) into the UI.

  return Array.from(byId.values()).sort((a, b) => {
    const aPriority = priorityIds.indexOf(a.seedId);
    const bPriority = priorityIds.indexOf(b.seedId);
    if (aPriority >= 0 && bPriority >= 0) return aPriority - bPriority;
    if (aPriority >= 0) return -1;
    if (bPriority >= 0) return 1;
    return 0;
  });
}

function isExplicitNonSeedOption(item: any) {
  if (!item || typeof item !== 'object') return false;
  const type = firstFiniteNumber(item?.type, item?.itemType, item?.item_type);
  const interactionType = String(item?.interactionType || item?.interaction_type || '').trim().toLowerCase();
  if (type && type !== 5 && interactionType !== 'plant') return true;
  return false;
}

function stealCropOptionsFromConfig(value: any): StealCropOption[] {
  const rawOptions = Array.isArray(value) ? value : [];
  const byId = new Map<number, StealCropOption>();

  rawOptions.forEach((item, index) => {
    const plantId = firstFiniteNumber(item?.plantId, item?.plantID, item?.id, item?.cropId, item?.cropID);
    if (!plantId || byId.has(plantId)) return;
    const seedId = firstFiniteNumber(item?.seedId, item?.seedID, item?.itemId, item?.itemID);
    const level = firstFiniteCount(item?.level, item?.requiredLevel, item?.landLevelNeed, item?.land_level_need);
    const sortGroup = firstFiniteSortValue(item?.sortGroup, item?.sort_group);
    const sortOrder = firstFiniteSortValue(item?.sortOrder, item?.sort_order, index);
    byId.set(plantId, {
      plantId,
      seedId,
      name: String(item?.name || item?.plantName || item?.label || '未知作物'),
      level,
      imageUrl: String(item?.imageUrl || item?.imageURL || item?.icon || ''),
      sortGroup,
      sortOrder,
    });
  });

  return Array.from(byId.values()).sort((a, b) => {
    const aGroup = a.sortGroup ?? Number.MAX_SAFE_INTEGER;
    const bGroup = b.sortGroup ?? Number.MAX_SAFE_INTEGER;
    if (aGroup !== bGroup) return aGroup - bGroup;
    const aLevel = a.level ?? Number.MAX_SAFE_INTEGER;
    const bLevel = b.level ?? Number.MAX_SAFE_INTEGER;
    if (aLevel !== bLevel) return aLevel - bLevel;
    const aOrder = a.sortOrder ?? Number.MAX_SAFE_INTEGER;
    const bOrder = b.sortOrder ?? Number.MAX_SAFE_INTEGER;
    if (aOrder !== bOrder) return aOrder - bOrder;
    return a.plantId - b.plantId;
  });
}

function firstFiniteSortValue(...values: any[]) {
  for (const value of values) {
    const parsed = Math.trunc(Number(value));
    if (Number.isFinite(parsed) && parsed >= 0) return parsed;
  }
  return null;
}

export function resetBackpackSeedPriority(value: any) {
  const seen = new Set<number>();
  const rawOptions = Array.isArray(value) ? value : [];
  return rawOptions
    .map((item) => ({
      seedId: firstFiniteNumber(item?.seedId, item?.id, item?.itemId, item?.seed_id),
      plantable: item?.plantable !== false,
    }))
    .filter((item) => {
      const seedId = item.seedId;
      if (!item.plantable) return false;
      if (!seedId || seen.has(seedId)) return false;
      seen.add(seedId);
      return true;
    })
    .map((item) => item.seedId);
}

export function reorderBackpackSeedPriority(values: number[], allSeedIds: number[], sourceId: number, targetId: number) {
  if (sourceId === targetId) return values;
  const visibleIds = uniquePositiveNumbers(allSeedIds);
  const sourceOrderIndex = visibleIds.indexOf(sourceId);
  const targetOrderIndex = visibleIds.indexOf(targetId);
  if (sourceOrderIndex < 0 || targetOrderIndex < 0) return values;
  const ordered = visibleIds.filter((seedId) => seedId !== sourceId);
  const targetIndex = ordered.indexOf(targetId);
  if (targetIndex < 0) return values;
  ordered.splice(sourceOrderIndex < targetOrderIndex ? targetIndex + 1 : targetIndex, 0, sourceId);
  return ordered;
}

function uniquePositiveNumbers(values: number[]) {
  const seen = new Set<number>();
  return values.filter((value) => {
    const normalized = Math.trunc(Number(value));
    if (!Number.isFinite(normalized) || normalized <= 0 || seen.has(normalized)) return false;
    seen.add(normalized);
    return true;
  });
}

function renderSettingsGroup(
  group: FarmAutomationFeatureGroup,
  schedulerTasks: FarmAutomationSchedulerTask[],
  onRunTask: (taskId: string) => void | Promise<void>,
  lastActionResult: FarmAutomationActionResult | null | undefined,
  mysteryShopPurchaseRecords: MysteryShopPurchaseRecord[],
  configBool: (key: string, fallback?: boolean) => boolean,
  configNumber: (key: string, fallback: number) => number,
  configValue: (key: string, fallback: any) => any,
  updateConfig: (key: string, value: any) => void,
  actions: {
    canRefreshBackpackSeeds: boolean;
    seedRefreshing: boolean;
    seedRefreshMessage: string;
    refreshBackpackSeeds: (forceRefresh?: boolean) => void;
    canRefreshStealCropOptions: boolean;
    cropRefreshing: boolean;
    cropRefreshMessage: string;
    refreshStealCropOptions: () => void;
    cropOptionsOpen: boolean;
    setCropOptionsOpen: (value: boolean | ((current: boolean) => boolean)) => void;
    dogGuardFriendCount: number;
  },
) {
  const plantModeOptions = [
    ['none', '不种植'],
    ['backpack_first', '背包优先'],
    ['highest_level', '最高等级作物'],
    ['max_exp', '经验/小时最高'],
    ['max_fert_exp', '普肥经验/小时最高'],
    ['max_profit', '净利润/小时最高'],
    ['max_fert_profit', '普肥净利润/小时最高'],
  ];
  const plantFertilizerOptions = [
    ['none', '不施肥'],
    ['normal', '普通化肥'],
    ['organic', '有机化肥'],
    ['smart_normal', '智能无机肥'],
    ['smart_organic', '智能有机肥'],
  ];
  const rushFertilizerOptions = [
    ['none', '不催熟'],
    ['normal', '普通化肥催熟'],
    ['organic', '有机化肥催熟'],
  ];
  const fertilizerLandTypes = [
    ['purpleGold', '紫金土地'],
    ['gold', '金土地'],
    ['black', '黑土地'],
    ['red', '红土地'],
    ['normal', '普通土地'],
  ];
  const rewardItems = [
    ['自动领取任务奖励', 'reward_claim', 30],
    ['SVIP每日礼包', 'svip_daily_gift', 43200],
    ['月卡奖励', 'monthly_card_reward', 43200],
    ['商城每日肥料', 'mall_daily_fertilizer', 43200],
    ['分享奖励', 'share_reward', 43200],
    ['邮件奖励', 'mail_reward', 43200],
    ['千星游记奖励领取', 'qian_xing_travel_reward', 7200],
    ['自动点亮星宿', 'xing_su_auto_light_up', 7200],
  ] as const;
  const mysteryCurrencies = [
    [1001, '金币'],
    [1002, '点券'],
    [1004, '钻石'],
    [1005, '金豆豆'],
  ] as const;
  const mysteryDiscounts = [
    [0, '全部折扣'],
    [10, '1折及以下'],
    [20, '2折及以下'],
    [30, '3折及以下'],
    [40, '4折及以下'],
    [50, '5折及以下'],
    [60, '6折及以下'],
    [70, '7折及以下'],
    [80, '8折及以下'],
    [90, '9折及以下'],
  ] as const;
  if (group.id === 'own_base') {
    const ownBaseItems = [
      ['一键务农', 'own_base', 30, true],
      ['自动收获', 'own_collect', 30, true],
      ['土地自动升级', 'land_upgrade', 43200, false],
    ] as const;
    return (
      <div className="automation-settings-grid">
        {ownBaseItems.map(([label, taskId, fallback, enabledFallback]) => {
          const { enabledKey, intervalKey } = schedulerConfigKeysForTask(schedulerTasks, taskId);
          return (
            <div className="automation-settings-pair" key={taskId}>
              <label className="automation-settings-check">
                <input
                  checked={configBool(enabledKey, enabledFallback)}
                  name={`config-${enabledKey}`}
                  type="checkbox"
                  onChange={(event) => updateConfig(enabledKey, event.currentTarget.checked)}
                />
                <span>{label}</span>
              </label>
              <label className="automation-settings-field">
                {label}间隔(秒)
                <input
                  min={5}
                  name={`config-${intervalKey}`}
                  type="number"
                  value={configNumber(intervalKey, fallback)}
                  onChange={(event) => updateConfig(intervalKey, numberFromInput(event.currentTarget.value, fallback))}
                />
              </label>
            </div>
          );
        })}
      </div>
    );
  }
  if (group.id === 'planting') {
    const { intervalKey: plantIntervalKey } = schedulerConfigKeysForTask(schedulerTasks, 'own_plant');
    const priorityIds = numberListFromConfig(configValue('autoFarmPlantBackpackSeedPriority', []));
    const disabledIds = numberListFromConfig(configValue('autoFarmPlantBackpackSeedDisabled', []));
    const rawSeedOptions = configValue('autoFarmPlantBackpackSeedOptions', configValue('backpackSeedOptions', []));
    const seedOptions = backpackSeedOptionsFromConfig(
      rawSeedOptions,
      priorityIds,
      disabledIds,
    );
    const visibleSeedIds = seedOptions.filter((seed) => seed.plantable !== false).map((seed) => seed.seedId);

    function updatePriority(next: number[]) {
      updateConfig('autoFarmPlantBackpackSeedPriority', next);
    }

    function toggleSeedDisabled(seedId: number) {
      const seed = seedOptions.find((option) => option.seedId === seedId);
      if (seed?.plantable === false) return;
      updateConfig('autoFarmPlantBackpackSeedDisabled', toggleNumberList(disabledIds, seedId, !disabledIds.includes(seedId)));
    }

    function dropSeed(event: DragEvent<HTMLDivElement>, targetId: number) {
      event.preventDefault();
      if (!visibleSeedIds.includes(targetId)) return;
      const sourceId = Math.trunc(Number(event.dataTransfer.getData('text/plain')));
      if (!Number.isFinite(sourceId) || sourceId <= 0) return;
      if (!visibleSeedIds.includes(sourceId)) return;
      updatePriority(reorderBackpackSeedPriority(priorityIds, visibleSeedIds, sourceId, targetId));
    }

    return (
      <div className="automation-settings-grid">
        <label className="automation-settings-field">
          主种植策略
          <select
            name="config-autoFarmPlantPrimaryMode"
            value={String(configValue('autoFarmPlantPrimaryMode', 'none'))}
            onChange={(event) => {
              updateConfig('autoFarmPlantPrimaryMode', event.currentTarget.value);
              updateConfig('autoFarmPlantMode', event.currentTarget.value);
            }}
          >
            {plantModeOptions.map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </label>
        <label className="automation-settings-field">
          副种植策略
          <select
            name="config-autoFarmPlantSecondaryMode"
            value={String(configValue('autoFarmPlantSecondaryMode', 'none'))}
            onChange={(event) => updateConfig('autoFarmPlantSecondaryMode', event.currentTarget.value)}
          >
            {plantModeOptions.map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </label>
        <label className="automation-settings-field">
          种植间隔(秒)
          <input
            min={5}
            name={`config-${plantIntervalKey}`}
            type="number"
            value={configNumber(plantIntervalKey, 30)}
            onChange={(event) => updateConfig(plantIntervalKey, numberFromInput(event.currentTarget.value, 30))}
          />
        </label>
        <label className="automation-settings-check">
          <input
            checked={configBool('autoFarmFourGridPlantEnabled', false)}
            name="config-autoFarmFourGridPlantEnabled"
            type="checkbox"
            onChange={(event) => updateConfig('autoFarmFourGridPlantEnabled', event.currentTarget.checked)}
          />
          <span>开启四格种植</span>
        </label>
        <label className="automation-settings-check">
          <input
            checked={configBool('autoFarmPlantBackpackForcePriority', false)}
            name="config-autoFarmPlantBackpackForcePriority"
            type="checkbox"
            onChange={(event) => updateConfig('autoFarmPlantBackpackForcePriority', event.currentTarget.checked)}
          />
          <span>强制使用背包优先级</span>
        </label>
        <label className="automation-settings-check">
          <input
            checked={configBool('autoFarmPlantRandomOrderEnabled', false)}
            name="config-autoFarmPlantRandomOrderEnabled"
            type="checkbox"
            onChange={(event) => updateConfig('autoFarmPlantRandomOrderEnabled', event.currentTarget.checked)}
          />
          <span>随机土地顺序</span>
        </label>
        <div className="automation-random-delay-card">
          <label className="automation-settings-check">
            <input
              checked={configBool('autoFarmPlantRandomizedEnabled', false)}
              name="config-autoFarmPlantRandomizedEnabled"
              type="checkbox"
              onChange={(event) => updateConfig('autoFarmPlantRandomizedEnabled', event.currentTarget.checked)}
            />
            <span>启用随机种植延迟</span>
          </label>
          <label className="automation-settings-field">
            随机延迟最小(ms)
            <input
              disabled={!configBool('autoFarmPlantRandomizedEnabled', false)}
              max={AUTO_PLANT_RANDOM_DELAY_MAX_MS}
              min={0}
              name="config-autoFarmPlantRandomizedDelayMinMs"
              type="number"
              value={configNumber('autoFarmPlantRandomizedDelayMinMs', 100)}
              onChange={(event) => updateConfig('autoFarmPlantRandomizedDelayMinMs', numberFromInput(event.currentTarget.value, 100))}
            />
          </label>
          <label className="automation-settings-field">
            随机延迟最大(ms)
            <input
              disabled={!configBool('autoFarmPlantRandomizedEnabled', false)}
              max={AUTO_PLANT_RANDOM_DELAY_MAX_MS}
              min={0}
              name="config-autoFarmPlantRandomizedDelayMaxMs"
              type="number"
              value={configNumber('autoFarmPlantRandomizedDelayMaxMs', 500)}
              onChange={(event) => updateConfig('autoFarmPlantRandomizedDelayMaxMs', numberFromInput(event.currentTarget.value, 500))}
            />
          </label>
        </div>
        <section className="automation-backpack-selector automation-settings-span" aria-label="背包种子选择器">
          <div className="automation-backpack-selector-header">
            <strong>背包种子选择器</strong>
            <div className="automation-backpack-refresh">
              {actions.seedRefreshMessage ? <span>{actions.seedRefreshMessage}</span> : null}
              <button
                disabled={!actions.canRefreshBackpackSeeds || actions.seedRefreshing}
                type="button"
                onClick={() => actions.refreshBackpackSeeds(true)}
              >
                <RefreshCw size={13} />
                {actions.seedRefreshing ? '刷新中' : '刷新背包'}
              </button>
              <button
                disabled={!seedOptions.length}
                type="button"
                onClick={() => updatePriority(resetBackpackSeedPriority(rawSeedOptions))}
              >
                <RotateCcw size={13} />
                重置
              </button>
            </div>
          </div>
          <div className="automation-backpack-table">
            <div className="automation-backpack-row head">
              <span>优先</span>
              <span>种子</span>
              <span>等级</span>
              <span>库存</span>
              <span>操作</span>
            </div>
            {seedOptions.length ? (
              seedOptions.map((seed) => {
                const hasPriority = priorityIds.includes(seed.seedId);
                const unavailable = seed.plantable === false;
                const disabled = seed.disabled || unavailable;
                const reason = unavailable
                  ? seed.plantableMessage || (seed.plantSize && seed.plantSize > 1 ? '四格作物未开启，当前不可选择' : '当前不可选择')
                  : '';
                return (
                  <div
                    className={disabled ? 'automation-backpack-row disabled' : 'automation-backpack-row'}
                    draggable={!unavailable}
                    key={seed.seedId}
                    title={reason || undefined}
                    onDragOver={(event) => {
                      if (!unavailable) event.preventDefault();
                    }}
                    onDragStart={(event) => {
                      if (unavailable) {
                        event.preventDefault();
                        return;
                      }
                      event.dataTransfer.setData('text/plain', String(seed.seedId));
                    }}
                    onDrop={(event) => dropSeed(event, seed.seedId)}
                  >
                    <label className="automation-backpack-priority">
                      <input
                        checked={hasPriority}
                        disabled={unavailable}
                        name={`config-autoFarmPlantBackpackSeedPriority-${seed.seedId}`}
                        type="checkbox"
                        onChange={(event) => {
                          if (unavailable) return;
                          updatePriority(toggleNumberList(priorityIds, seed.seedId, event.currentTarget.checked));
                        }}
                      />
                    </label>
                    <strong className="automation-backpack-name">
                      <GripVertical size={14} />
                      <span className="automation-backpack-name-text">
                        <span>{seed.name}</span>
                        {reason ? <em className="automation-backpack-reason">{reason}</em> : null}
                      </span>
                    </strong>
                    <span>{seed.level ? `Lv.${seed.level}` : '-'}</span>
                    <span>{seed.count ?? '-'}</span>
                    <span className="automation-backpack-action">
                      <button
                        className={seed.disabled ? 'automation-backpack-disable active' : 'automation-backpack-disable'}
                        disabled={unavailable}
                        type="button"
                        onClick={() => toggleSeedDisabled(seed.seedId)}
                      >
                        {seed.disabled ? '启用' : '禁用'}
                      </button>
                    </span>
                  </div>
                );
              })
            ) : (
              <div className="automation-backpack-empty">暂无背包种子数据，刷新背包后会在这里调整顺序。</div>
            )}
          </div>
        </section>
      </div>
    );
  }
  if (group.id === 'fertilizer') {
    const { intervalKey: fertilizerIntervalKey } = schedulerConfigKeysForTask(schedulerTasks, 'own_fertilizer');
    const selectedLandTypes = stringListFromConfig(configValue('autoFarmFertilizerLandTypes', ['purpleGold', 'gold', 'black', 'red', 'normal']));
    const delayedSubmitEnabled = configBool('autoFarmFertilizerDelayedSubmitEnabled', false);
    const delayedSubmitScopes = stringListFromConfig(
      configValue('autoFarmFertilizerDelayedSubmitScopes', ['planting', 'rush', 'manual']),
    );
    return (
      <div className="automation-settings-grid">
        <label className="automation-settings-field">
          种植施肥策略
          <select
            name="config-autoFarmPlantFertilizerMode"
            value={String(configValue('autoFarmPlantFertilizerMode', 'none'))}
            onChange={(event) => updateConfig('autoFarmPlantFertilizerMode', event.currentTarget.value)}
          >
            {plantFertilizerOptions.map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </label>
        <label className="automation-settings-field">
          施肥间隔(秒)
          <input
            min={5}
            name={`config-${fertilizerIntervalKey}`}
            type="number"
            value={configNumber(fertilizerIntervalKey, 30)}
            onChange={(event) => updateConfig(fertilizerIntervalKey, numberFromInput(event.currentTarget.value, 30))}
          />
        </label>
        <label className="automation-settings-field">
          催熟策略
          <select
            name="config-autoFarmRushFertilizerMode"
            value={String(configValue('autoFarmRushFertilizerMode', 'none'))}
            onChange={(event) => updateConfig('autoFarmRushFertilizerMode', event.currentTarget.value)}
          >
            {rushFertilizerOptions.map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
        </label>
        <label className="automation-settings-field">
          催熟阈值(秒)
          <input
            min={0}
            name="config-autoFarmFertilizerRushThresholdSec"
            type="number"
            value={configNumber('autoFarmFertilizerRushThresholdSec', 300)}
            onChange={(event) => updateConfig('autoFarmFertilizerRushThresholdSec', numberFromInput(event.currentTarget.value, 300))}
          />
        </label>
        <label className="automation-settings-check">
          <input
            checked={delayedSubmitEnabled}
            name="config-autoFarmFertilizerDelayedSubmitEnabled"
            type="checkbox"
            onChange={(event) => updateConfig('autoFarmFertilizerDelayedSubmitEnabled', event.currentTarget.checked)}
          />
          <span>延迟施肥</span>
        </label>
        {delayedSubmitEnabled && (
          <>
            <label className="automation-settings-field">
              地块间隔(毫秒)
              <input
                min={0}
                name="config-autoFarmFertilizerDelayedSubmitIntervalMs"
                type="number"
                value={Math.max(0, configNumber('autoFarmFertilizerDelayedSubmitIntervalMs', 500))}
                onChange={(event) =>
                  updateConfig(
                    'autoFarmFertilizerDelayedSubmitIntervalMs',
                    Math.max(0, numberFromInput(event.currentTarget.value, 500)),
                  )
                }
              />
            </label>
            <div className="automation-settings-span">
              <strong>作用范围</strong>
              <div className="automation-settings-options">
                {[
                  ['planting', '种植策略'],
                  ['rush', '催熟策略'],
                  ['manual', '手动执行'],
                ].map(([value, label]) => (
                  <label className="automation-settings-check compact" key={value}>
                    <input
                      checked={delayedSubmitScopes.includes(value)}
                      name={`config-autoFarmFertilizerDelayedSubmitScopes-${value}`}
                      type="checkbox"
                      onChange={(event) =>
                        updateConfig(
                          'autoFarmFertilizerDelayedSubmitScopes',
                          toggleStringList(delayedSubmitScopes, value, event.currentTarget.checked),
                        )
                      }
                    />
                    <span>{label}</span>
                  </label>
                ))}
              </div>
            </div>
          </>
        )}
        <label className="automation-settings-check">
          <input
            checked={configBool('autoFarmFertilizerMultiSeason', false)}
            name="config-autoFarmFertilizerMultiSeason"
            type="checkbox"
            onChange={(event) => updateConfig('autoFarmFertilizerMultiSeason', event.currentTarget.checked)}
          />
          <span>多季节作物补肥</span>
        </label>
        <label className="automation-settings-check">
          <input
            checked={configBool('autoFarmFertilizerHarvestLinkEnabled', false)}
            name="config-autoFarmFertilizerHarvestLinkEnabled"
            type="checkbox"
            onChange={(event) => updateConfig('autoFarmFertilizerHarvestLinkEnabled', event.currentTarget.checked)}
          />
          <span>催熟联动收获</span>
        </label>
        <label className="automation-settings-check">
          <input
            checked={configBool('autoFarmFertilizerContinuousRushEnabled', false)}
            name="config-autoFarmFertilizerContinuousRushEnabled"
            type="checkbox"
            onChange={(event) => updateConfig('autoFarmFertilizerContinuousRushEnabled', event.currentTarget.checked)}
          />
          <span>连续催熟</span>
        </label>
        <label className="automation-settings-check">
          <input
            checked={configBool('autoFarmFertilizerFillEnabled', false)}
            name="config-autoFarmFertilizerFillEnabled"
            type="checkbox"
            onChange={(event) => updateConfig('autoFarmFertilizerFillEnabled', event.currentTarget.checked)}
          />
          <span>自动填充肥料</span>
        </label>
        <label className="automation-settings-field">
          填充肥料间隔(秒)
          <input
            min={5}
            name="config-autoFarmFertilizerFillIntervalSec"
            type="number"
            value={configNumber('autoFarmFertilizerFillIntervalSec', 43200)}
            onChange={(event) => updateConfig('autoFarmFertilizerFillIntervalSec', numberFromInput(event.currentTarget.value, 43200))}
          />
        </label>
        <div className="automation-settings-span">
          <strong>施肥土地类型</strong>
          <div className="automation-settings-options">
            {fertilizerLandTypes.map(([value, label]) => (
              <label className="automation-settings-check compact" key={value}>
                <input
                  checked={selectedLandTypes.includes(value)}
                  name={`config-autoFarmFertilizerLandTypes-${value}`}
                  type="checkbox"
                  onChange={(event) =>
                    updateConfig('autoFarmFertilizerLandTypes', toggleStringList(selectedLandTypes, value, event.currentTarget.checked))
                  }
                />
                <span>{label}</span>
              </label>
            ))}
          </div>
        </div>
      </div>
    );
  }
  if (group.id === 'rewards') {
    return (
      <div className="automation-settings-grid">
        {rewardItems.map(([label, taskId, fallback]) => {
          const { enabledKey, intervalKey } = schedulerConfigKeysForTask(schedulerTasks, taskId);
          const { intervalMinKey, scheduleModeKey, scheduleTimeKey } = rewardTimingKeys(
            schedulerTaskIntervalConfigKeyById[taskId] || intervalKey,
          );
          const fallbackMinutes = minutesFromSeconds(fallback);
          const mode = String(configValue(scheduleModeKey, 'interval')) === 'daily_time' ? 'daily_time' : 'interval';
          const minutesFallback = configValue(intervalMinKey, undefined) === undefined
            ? minutesFromSeconds(configNumber(intervalKey, fallback))
            : fallbackMinutes;
          const intervalMinutes = Math.max(1, configNumber(intervalMinKey, minutesFallback));

          return (
            <section className="automation-reward-schedule-card" key={taskId}>
              <label className="automation-settings-check">
                <input
                  checked={configBool(enabledKey, false)}
                  name={`config-${enabledKey}`}
                  type="checkbox"
                  onChange={(event) => updateConfig(enabledKey, event.currentTarget.checked)}
                />
                <span>{label}</span>
              </label>
              <label className="automation-settings-field">
                执行模式
                <select
                  name={`config-${scheduleModeKey}`}
                  value={mode}
                  onChange={(event) => updateConfig(scheduleModeKey, event.currentTarget.value)}
                >
                  <option value="interval">间隔执行</option>
                  <option value="daily_time">指定时间执行</option>
                </select>
              </label>
              {mode === 'daily_time' ? (
                <label className="automation-settings-field">
                  指定时间
                  <input
                    name={`config-${scheduleTimeKey}`}
                    type="time"
                    value={String(configValue(scheduleTimeKey, '08:00'))}
                    onChange={(event) => updateConfig(scheduleTimeKey, event.currentTarget.value)}
                  />
                </label>
              ) : (
                <label className="automation-settings-field">
                  间隔(分钟)
                  <input
                    min={1}
                    name={`config-${intervalMinKey}`}
                    type="number"
                    value={intervalMinutes}
                    onChange={(event) => {
                      const nextMinutes = Math.max(1, numberFromInput(event.currentTarget.value, fallbackMinutes));
                      updateConfig(intervalMinKey, nextMinutes);
                      updateConfig(intervalKey, nextMinutes * 60);
                    }}
                  />
                  <input name={`config-${intervalKey}`} type="hidden" value={configNumber(intervalKey, intervalMinutes * 60)} />
                </label>
              )}
            </section>
          );
        })}
      </div>
    );
  }
  if (group.id === 'mystery_shop') {
    const { intervalKey: mysteryShopIntervalKey } = schedulerConfigKeysForTask(schedulerTasks, 'mystery_shop_auto_buy');
    const currencyIds = numberListFromConfig(configValue('autoFarmMysteryShopCurrencyIds', [1001, 1002]));
    const mysteryResult = isMysteryShopActionResult(lastActionResult) ? mysteryShopActionResultCopy(lastActionResult) : null;
    return (
      <div className="automation-settings-grid">
        <section className="automation-mystery-section automation-settings-span">
          <strong>商店操作</strong>
          <div className="automation-mystery-actions">
            <button className="automation-mystery-read-button" type="button" onClick={() => onRunTask('mystery_shop_read')}>
              <RefreshCw size={13} />
              读取当前神秘商店商品和价格
            </button>
            <button className="automation-mystery-buy-button" type="button" onClick={() => onRunTask('mystery_shop_auto_buy')}>
              <Play size={13} />
              按预设自动购买
            </button>
          </div>
          {mysteryResult ? (
            <div className={lastActionResult?.ok ? 'automation-mystery-result ok' : 'automation-mystery-result'}>
              <strong>{mysteryResult.title}</strong>
              <span>{mysteryResult.detail}</span>
            </div>
          ) : null}
        </section>
        <section className="automation-mystery-section automation-settings-span">
          <strong>购买规则</strong>
          <div className="automation-mystery-rules">
            <label className="automation-settings-field">
              自动购买间隔(秒)
              <input
                min={5}
                name={`config-${mysteryShopIntervalKey}`}
                type="number"
                value={configNumber(mysteryShopIntervalKey, 43200)}
                onChange={(event) => updateConfig(mysteryShopIntervalKey, numberFromInput(event.currentTarget.value, 43200))}
              />
            </label>
            <label className="automation-settings-field">
              折扣阈值
              <select
                name="config-autoFarmMysteryShopDiscountThreshold"
                value={configNumber('autoFarmMysteryShopDiscountThreshold', 0)}
                onChange={(event) => updateConfig('autoFarmMysteryShopDiscountThreshold', numberFromInput(event.currentTarget.value, 0))}
              >
                {mysteryDiscounts.map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </select>
            </label>
          </div>
        </section>
        <section className="automation-mystery-section automation-settings-span">
          <strong>允许货币</strong>
          <div className="automation-settings-options compact automation-mystery-currencies">
            {mysteryCurrencies.map(([value, label]) => (
              <label className="automation-settings-check compact" key={value}>
                <input
                  checked={currencyIds.includes(value)}
                  name={`config-autoFarmMysteryShopCurrencyIds-${value}`}
                  type="checkbox"
                  onChange={(event) =>
                    updateConfig('autoFarmMysteryShopCurrencyIds', toggleNumberList(currencyIds, value, event.currentTarget.checked))
                  }
                />
                <span>{label}</span>
              </label>
            ))}
          </div>
        </section>
        <section className="automation-mystery-section automation-settings-span">
          <strong>购买记录</strong>
          {mysteryShopPurchaseRecords.length ? (
            <div className="automation-mystery-history">
              {mysteryShopPurchaseRecords.map((record) => (
                <div className="automation-mystery-history-row" key={record.id}>
                  <time title={record.occurredAt}>{formatSchedulerTime(record.occurredAt)}</time>
                  <strong>{`${record.itemName || '未知物品'} x${record.count}`}</strong>
                  <span>{`${record.unitPrice} ${record.currencyName || record.currencyId}`}</span>
                  <em>{mysteryShopDiscountText(record.discount)}</em>
                </div>
              ))}
            </div>
          ) : (
            <p className="automation-mystery-history-empty">暂无成功购买记录</p>
          )}
        </section>
      </div>
    );
  }
  if (group.id === 'friends') {
    const friendStealKeys = schedulerConfigKeysForTask(schedulerTasks, 'friend_steal');
    const friendHelpKeys = schedulerConfigKeysForTask(schedulerTasks, 'friend_help');
    const friendMischiefKeys = schedulerConfigKeysForTask(schedulerTasks, 'friend_mischief');
    const friendStealRandomDelayEnabled = configBool('autoFarmFriendStealRandomDelayEnabled', false);
    const cropListMode = String(configValue('autoFarmFriendStealPlantListMode', 'blacklist')) === 'whitelist' ? 'whitelist' : 'blacklist';
    const cropModeName = cropListMode === 'whitelist' ? '白名单' : '黑名单';
    const activeCropListKey =
      cropListMode === 'whitelist' ? 'autoFarmFriendStealPlantWhitelist' : 'autoFarmFriendStealPlantBlacklist';
    const activeCropIds = numberListFromConfig(configValue(activeCropListKey, []));
    const cropOptions = stealCropOptionsFromConfig(configValue('autoFarmFriendStealCropOptions', []));
    const quietHoursMode = String(configValue('autoFarmFriendQuietHoursMode', 'sleep')) === 'work' ? 'work' : 'sleep';
    const quietHoursScopesValue = configValue('autoFarmFriendQuietHoursScopes', ['steal', 'help']);
    const quietHoursScopes = Array.isArray(quietHoursScopesValue)
      ? stringListFromConfig(quietHoursScopesValue).filter((scope) => ['steal', 'help', 'mischief'].includes(scope))
      : ['steal', 'help'];
    const quietHoursScopeOptions = [
      ['steal', '偷菜'],
      ['help', '帮助'],
      ['mischief', '捣乱'],
    ] as const;
    const cropStrategyOptions =
      cropListMode === 'whitelist'
        ? [
            [1, '跳过未命中农场'],
            [2, '只跳过未命中作物'],
          ]
        : [
            [1, '跳过黑名单农场'],
            [2, '跳过黑名单作物'],
          ];
    return (
      <div className="automation-settings-grid">
        <div className="automation-friend-action-row steal">
          <label className="automation-settings-check">
            <input
              checked={configBool(friendStealKeys.enabledKey, false)}
              name={`config-${friendStealKeys.enabledKey}`}
              type="checkbox"
              onChange={(event) => updateConfig(friendStealKeys.enabledKey, event.currentTarget.checked)}
            />
            <span>好友偷菜</span>
          </label>
          <label className="automation-settings-field">
            偷菜间隔(秒)
            <input
              min={0}
              name={`config-${friendStealKeys.intervalKey}`}
              type="number"
              value={configNumber(friendStealKeys.intervalKey, 90)}
              onChange={(event) => updateConfig(friendStealKeys.intervalKey, numberFromInput(event.currentTarget.value, 90))}
            />
          </label>
          <label className="automation-settings-field">
            黑名单跳过冷却(分钟)
            <input
              min={0}
              name="config-autoFarmFriendBlacklistCooldownMin"
              type="number"
              value={configNumber('autoFarmFriendBlacklistCooldownMin', 10)}
              onChange={(event) => updateConfig('autoFarmFriendBlacklistCooldownMin', numberFromInput(event.currentTarget.value, 10))}
            />
          </label>
          <label className="automation-settings-check">
            <input
              checked={friendStealRandomDelayEnabled}
              name="config-autoFarmFriendStealRandomDelayEnabled"
              type="checkbox"
              onChange={(event) => updateConfig('autoFarmFriendStealRandomDelayEnabled', event.currentTarget.checked)}
            />
            <span>随机偷取延迟</span>
          </label>
          <label className="automation-settings-field">
            最小延迟(ms)
            <input
              disabled={!friendStealRandomDelayEnabled}
              min={0}
              name="config-autoFarmFriendStealRandomDelayMinMs"
              type="number"
              value={configNumber('autoFarmFriendStealRandomDelayMinMs', 1000)}
              onChange={(event) => updateConfig('autoFarmFriendStealRandomDelayMinMs', numberFromInput(event.currentTarget.value, 1000))}
            />
          </label>
          <label className="automation-settings-field">
            最大延迟(ms)
            <input
              disabled={!friendStealRandomDelayEnabled}
              min={0}
              name="config-autoFarmFriendStealRandomDelayMaxMs"
              type="number"
              value={configNumber('autoFarmFriendStealRandomDelayMaxMs', 5000)}
              onChange={(event) => updateConfig('autoFarmFriendStealRandomDelayMaxMs', numberFromInput(event.currentTarget.value, 5000))}
            />
          </label>
          <label className="automation-settings-check">
            <input
              checked={configBool('autoFarmFriendStealPlantBlacklistEnabled', false)}
              name="config-autoFarmFriendStealPlantBlacklistEnabled"
              type="checkbox"
              onChange={(event) => updateConfig('autoFarmFriendStealPlantBlacklistEnabled', event.currentTarget.checked)}
            />
            <span>启用作物偷取名单</span>
          </label>
          <label className="automation-settings-field">
            运行模式
            <select
              name="config-autoFarmFriendStealPlantListMode"
              value={cropListMode}
              onChange={(event) => updateConfig('autoFarmFriendStealPlantListMode', event.currentTarget.value)}
            >
              <option value="blacklist">黑名单</option>
              <option value="whitelist">白名单</option>
            </select>
          </label>
          <label className="automation-settings-field">
            {cropModeName}策略
            <select
              name="config-autoFarmFriendStealPlantBlacklistStrategy"
              value={configNumber('autoFarmFriendStealPlantBlacklistStrategy', 1)}
              onChange={(event) => updateConfig('autoFarmFriendStealPlantBlacklistStrategy', numberFromInput(event.currentTarget.value, 1))}
            >
              {cropStrategyOptions.map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </select>
          </label>
          <section className="automation-crop-list-card">
            <div className="automation-crop-list-header">
              <div>
                <strong>{cropModeName}作物选择列表</strong>
                <span>
                  已选择 {activeCropIds.length} 个，可选 {cropOptions.length} 个
                </span>
              </div>
              <div className="automation-crop-list-actions">
                {actions.cropRefreshMessage ? <span>{actions.cropRefreshMessage}</span> : null}
                {actions.canRefreshStealCropOptions ? (
                  <button type="button" disabled={actions.cropRefreshing} onClick={() => actions.refreshStealCropOptions()}>
                    <RefreshCw size={13} />
                    {actions.cropRefreshing ? '刷新中' : '刷新作物'}
                  </button>
                ) : null}
                <button type="button" onClick={() => actions.setCropOptionsOpen((open) => !open)}>
                  {actions.cropOptionsOpen ? `收起${cropModeName}作物列表` : `展开${cropModeName}作物列表`}
                </button>
              </div>
            </div>
            {actions.cropOptionsOpen ? (
              cropOptions.length > 0 ? (
                <div className="automation-crop-grid">
                  {cropOptions.map((option) => {
                    const selected = activeCropIds.includes(option.plantId);
                    return (
                      <button
                        aria-pressed={selected}
                        className={`automation-crop-option${selected ? ' selected' : ''}`}
                        key={option.plantId}
                        type="button"
                        onClick={() => updateConfig(activeCropListKey, toggleNumberList(activeCropIds, option.plantId, !selected))}
                      >
                        {option.imageUrl ? (
                          <FallbackImage alt="" src={option.imageUrl} />
                        ) : (
                          <span className="automation-crop-placeholder">{option.name.slice(0, 1)}</span>
                        )}
                        <span>{option.name}</span>
                        <small>
                          #{option.plantId}
                          {option.level !== null ? ` · Lv.${option.level}` : ''}
                        </small>
                      </button>
                    );
                  })}
                </div>
              ) : (
                <p className="automation-crop-empty">暂无作物选项，请刷新后再选择。</p>
              )
            ) : null}
          </section>
        </div>
        <div className="automation-friend-action-row help">
          <label className="automation-settings-check">
            <input
              checked={configBool(friendHelpKeys.enabledKey, false)}
              name={`config-${friendHelpKeys.enabledKey}`}
              type="checkbox"
              onChange={(event) => updateConfig(friendHelpKeys.enabledKey, event.currentTarget.checked)}
            />
            <span>好友帮忙</span>
          </label>
          <label className="automation-settings-field">
            帮忙间隔(秒)
            <input
              min={0}
              name={`config-${friendHelpKeys.intervalKey}`}
              type="number"
              value={configNumber(friendHelpKeys.intervalKey, 90)}
              onChange={(event) => updateConfig(friendHelpKeys.intervalKey, numberFromInput(event.currentTarget.value, 90))}
            />
          </label>
          <label className="automation-settings-field">
            每批帮助好友数量
            <input
              min={0}
              name="config-autoFarmFriendHelpMaxFriends"
              type="number"
              value={configNumber('autoFarmFriendHelpMaxFriends', 5)}
              onChange={(event) => updateConfig('autoFarmFriendHelpMaxFriends', numberFromInput(event.currentTarget.value, 5))}
            />
          </label>
          <label className="automation-settings-field">
            帮忙每日上限
            <input
              min={0}
              name="config-autoFarmFriendHelpDailyLimit"
              type="number"
              value={configNumber('autoFarmFriendHelpDailyLimit', 30)}
              onChange={(event) => updateConfig('autoFarmFriendHelpDailyLimit', numberFromInput(event.currentTarget.value, 30))}
            />
          </label>
          <label
            className={`automation-settings-check guard-only${actions.dogGuardFriendCount > 0 ? '' : ' disabled'}`}
            title={actions.dogGuardFriendCount > 0 ? undefined : '请先读取护主犬好友'}
          >
            <input
              checked={actions.dogGuardFriendCount > 0 && configBool('autoFarmFriendHelpGuardDogOnly', false)}
              disabled={actions.dogGuardFriendCount <= 0}
              name="config-autoFarmFriendHelpGuardDogOnly"
              type="checkbox"
              onChange={(event) => updateConfig('autoFarmFriendHelpGuardDogOnly', event.currentTarget.checked)}
            />
            <span>仅帮助护主犬好友</span>
            {actions.dogGuardFriendCount <= 0 ? <small>请先读取护主犬好友</small> : null}
          </label>
        </div>
        <div className="automation-friend-action-row">
          <label className="automation-settings-check">
            <input
              checked={configBool(friendMischiefKeys.enabledKey, false)}
              name={`config-${friendMischiefKeys.enabledKey}`}
              type="checkbox"
              onChange={(event) => updateConfig(friendMischiefKeys.enabledKey, event.currentTarget.checked)}
            />
            <span>好友捣乱</span>
          </label>
          <label className="automation-settings-field">
            捣乱间隔(秒)
            <input
              min={0}
              name={`config-${friendMischiefKeys.intervalKey}`}
              type="number"
              value={configNumber(friendMischiefKeys.intervalKey, 90)}
              onChange={(event) => updateConfig(friendMischiefKeys.intervalKey, numberFromInput(event.currentTarget.value, 90))}
            />
          </label>
          <p className="automation-settings-note">捣乱次数按游戏回包识别；提示今日上限后，当日本账号后续自动轮次直接跳过。</p>
        </div>
        <section className="automation-friend-quiet-card">
          <div className="automation-friend-quiet-primary">
            <label className="automation-settings-check">
              <input
                checked={configBool('autoFarmFriendQuietHoursEnabled', false)}
                name="config-autoFarmFriendQuietHoursEnabled"
                type="checkbox"
                onChange={(event) => updateConfig('autoFarmFriendQuietHoursEnabled', event.currentTarget.checked)}
              />
              <span>启用静默时间</span>
            </label>
            <div className="automation-quiet-choice-field">
              <strong>运行模式</strong>
              <div aria-label="运行模式" className="automation-quiet-segmented" role="radiogroup">
                {([
                  ['sleep', '指定时间休眠'],
                  ['work', '指定时间工作'],
                ] as const).map(([value, label]) => (
                  <button
                    aria-checked={quietHoursMode === value}
                    aria-label={label}
                    className={quietHoursMode === value ? 'active' : ''}
                    key={value}
                    role="radio"
                    type="button"
                    onClick={() => updateConfig('autoFarmFriendQuietHoursMode', value)}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>
            <div className="automation-quiet-choice-field">
              <strong>作用范围</strong>
              <div className="automation-quiet-scopes">
                {quietHoursScopeOptions.map(([value, label]) => (
                  <label key={value}>
                    <input
                      checked={quietHoursScopes.includes(value)}
                      name={`config-autoFarmFriendQuietHoursScopes-${value}`}
                      type="checkbox"
                      onChange={(event) =>
                        updateConfig(
                          'autoFarmFriendQuietHoursScopes',
                          toggleStringList(quietHoursScopes, value, event.currentTarget.checked),
                        )
                      }
                    />
                    <span>{label}</span>
                  </label>
                ))}
              </div>
            </div>
          </div>
          <div className="automation-friend-quiet-times">
            <label className="automation-settings-field">
              开始时间
              <input
                name="config-autoFarmFriendQuietHoursStart"
                type="time"
                value={String(configValue('autoFarmFriendQuietHoursStart', '23:00'))}
                onChange={(event) => updateConfig('autoFarmFriendQuietHoursStart', event.currentTarget.value)}
              />
            </label>
            <label className="automation-settings-field">
              结束时间
              <input
                name="config-autoFarmFriendQuietHoursEnd"
                type="time"
                value={String(configValue('autoFarmFriendQuietHoursEnd', '07:00'))}
                onChange={(event) => updateConfig('autoFarmFriendQuietHoursEnd', event.currentTarget.value)}
              />
            </label>
          </div>
        </section>
      </div>
    );
  }
  return <p className="automation-settings-note">未识别的设置组。</p>;
}
