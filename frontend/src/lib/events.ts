export type EventTone = 'soft' | 'warn' | 'danger' | 'muted';

export type RuntimeEventDto = {
  id: number;
  timestamp: unknown;
  level: string;
  source: string;
  type: string;
  message: string;
  data?: Record<string, unknown>;
};

export function targetLabel(target?: string) {
  if (target === 'qq_ws') return 'QQ WS';
  if (target === 'wechat_cdp') return '微信 CDP';
  if (target === 'yyb_cdp') return '应用宝 CDP';
  return target || '-';
}

export function eventTone(level?: string): EventTone {
  if (level === 'error') return 'danger';
  if (level === 'warn') return 'warn';
  if (level === 'info') return 'soft';
  return 'muted';
}

export function formatEventTime(timestamp: unknown) {
  if (!timestamp) return '-';
  const date = timestamp instanceof Date ? timestamp : new Date(String(timestamp));
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  });
}

export function compactEventData(data?: Record<string, unknown>) {
  if (!data || Object.keys(data).length === 0) return '';
  return JSON.stringify(data);
}

const automationTaskLabels: Record<string, string> = {
  own_base: '一键务农',
  land_upgrade: '土地自动升级',
  reward_claim: '自动领取任务奖励',
  svip_daily_gift: 'SVIP每日礼包',
  monthly_card_reward: '月卡奖励',
  mall_daily_fertilizer: '商城每日肥料',
  share_reward: '自动领取分享奖励',
  mail_reward: '自动领取邮件奖励',
  qian_xing_travel_reward: '千星游记奖励领取',
  xing_su_auto_light_up: '自动点亮星宿',
  he_feng_travel_reward: '限时活动/荷风游记奖励领取',
  limited_seed_draw: '荷风游记抽奖',
  mystery_shop_auto_buy: '神秘商店自动购买',
  mystery_shop_read: '神秘商店查看',
  own_collect: '自动收获',
  own_plant: '自动种植',
  fertilizer_fill: '自动填充化肥',
  own_fertilizer: '自动施肥',
  friend_steal: '好友偷菜',
  friend_help: '好友帮忙',
  friend_mischief: '好友捣乱',
  auto_warehouse_sell: '仓库自动出售',
  warehouse_sell: '仓库出售',
};

export function automationTaskLabel(taskId?: string) {
  const id = String(taskId || '').trim();
  if (!id) return '';
  return automationTaskLabels[id] || id;
}

export function automationTaskIdFromEvent(event: Pick<RuntimeEventDto, 'data' | 'type'>) {
  const data = event.data || {};
  const raw = data.taskId ?? data.requested ?? data.task_id;
  return String(raw || '').trim();
}

export function isTaskLifecycleNoise(event: Pick<RuntimeEventDto, 'type' | 'message'>) {
  const type = String(event.type || '').toLowerCase();
  if (type === 'task.start') return true;
  const message = String(event.message || '').trim().toLowerCase();
  return message === 'automation task started' || message === 'automation task completed';
}

export function isWorkbenchTaskEvent(event: RuntimeEventDto) {
  const type = String(event.type || '').toLowerCase();
  if (type === 'task.done' || type === 'task.failed') return true;
  if (isTaskLifecycleNoise(event)) return false;
  if (type.startsWith('auto_farm.settings') || type.includes('settings.')) return false;
  const taskId = automationTaskIdFromEvent(event);
  if (!taskId) return false;
  const source = String(event.source || '').toLowerCase();
  return source === 'auto_farm' || source === 'task' || source.startsWith('farm_');
}

export function isTaskResultEvent(event: RuntimeEventDto) {
  return isWorkbenchTaskEvent(event);
}

export function taskResultStatusLabel(event: Pick<RuntimeEventDto, 'type' | 'level' | 'data'>) {
  const type = String(event.type || '').toLowerCase();
  if (type === 'task.failed' || event.level === 'error' || event.level === 'warn') return '失败';
  const status = String(event.data?.status || '').toLowerCase();
  if (status === 'failed' || status === 'error') return '失败';
  if (status === 'ok' || type === 'task.done') return '完成';
  return '完成';
}

export function workbenchTaskName(event: RuntimeEventDto) {
  const taskId = automationTaskIdFromEvent(event);
  return automationTaskLabel(taskId) || '自动化任务';
}

export function workbenchTaskResult(event: RuntimeEventDto) {
  const message = String(event.message || '').trim();
  if (message) return message;
  const type = String(event.type || '').toLowerCase();
  if (type === 'task.failed') return '任务执行失败';
  if (type === 'task.done') return '任务执行完成';
  return '任务已执行';
}

// Returns true for TSDK-BLOCK events emitted by the Fetch interception layer.
// These are kept separate from system logs and displayed in the TSDK indicator panel.
export function isTsdkBlockEvent(event: Pick<RuntimeEventDto, 'type' | 'message'>) {
  return event.type === 'qqhost.log' && String(event.message || '').includes('[TSDK-BLOCK]');
}

export function tsdkBlockEventsNewestFirst(events: RuntimeEventDto[]) {
  return events
    .filter(isTsdkBlockEvent)
    .map((event, index) => ({ event, index }))
    .sort((left, right) => {
      const idOrder = Number(right.event.id) - Number(left.event.id);
      if (Number.isFinite(idOrder) && idOrder !== 0) return idOrder;

      const timeOrder = new Date(String(right.event.timestamp)).getTime()
        - new Date(String(left.event.timestamp)).getTime();
      return Number.isFinite(timeOrder) && timeOrder !== 0 ? timeOrder : left.index - right.index;
    })
    .map(({ event }) => event);
}

// Derives the signal state for the TSDK-BLOCK indicator.
//   'healthy' — interception confirmed active (visible block event / Layer1 ready / CDP link is up)
//   'danger'  — explicit init_err event recorded with no success
//   'muted'   — no signal yet (game not connected or not a CDP/WS target)
//
// Note: the init_ok event is emitted before account confirmation, so it is stored
// under the "default" account key and is never visible in the filtered event list.
// For CDP targets we therefore fall back to status.ready as the healthy indicator.
export function tsdkBlockSignal(
  events: RuntimeEventDto[],
  status?: { target?: string; ready?: boolean },
): 'healthy' | 'danger' | 'muted' {
  const tsdk = events.filter(isTsdkBlockEvent);

  // Any visible non-error TSDK event means interception is active.
  if (tsdk.some((e) => e.level !== 'error')) return 'healthy';

  // The init_ok / Layer1-ready event fires before account confirmation, so it is
  // stored under the "default" key and is never visible in the filtered event list.
  // When any runtime target is ready, TSDK blocking is already active (game.js
  // patch for QQ WS, Fetch.enable for CDP links) — infer healthy from status.
  if (status?.ready === true && status.target) return 'healthy';

  // Explicit init failure visible in the event list.
  if (tsdk.some((e) => e.data?.status === 'init_err')) return 'danger';

  return 'muted';
}

