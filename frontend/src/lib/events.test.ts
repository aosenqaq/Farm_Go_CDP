import { describe, expect, it } from 'vitest';

import { automationTaskLabel, eventTone, formatEventTime, isTaskLifecycleNoise, isWorkbenchTaskEvent, targetLabel, tsdkBlockEventsNewestFirst, workbenchTaskName, workbenchTaskResult } from './events';

describe('targetLabel', () => {
  it('maps runtime target ids to readable labels', () => {
    expect(targetLabel('qq_ws')).toBe('QQ WS');
    expect(targetLabel('wechat_cdp')).toBe('微信 CDP');
    expect(targetLabel('yyb_cdp')).toBe('应用宝 CDP');
    expect(targetLabel('settings')).toBe('settings');
  });
});

describe('eventTone', () => {
  it('maps event levels to visual tones', () => {
    expect(eventTone('info')).toBe('soft');
    expect(eventTone('warn')).toBe('warn');
    expect(eventTone('error')).toBe('danger');
    expect(eventTone('debug')).toBe('muted');
  });
});

describe('formatEventTime', () => {
  it('formats valid timestamps with local time', () => {
    expect(formatEventTime('2026-07-06T12:34:56Z')).toMatch(/\d{2}:\d{2}:\d{2}/);
  });

  it('falls back when timestamp is missing', () => {
    expect(formatEventTime(null)).toBe('-');
  });
});

describe('workbench task event helpers', () => {
  it('maps task ids to Chinese labels', () => {
    expect(automationTaskLabel('own_collect')).toBe('自动收获');
    expect(automationTaskLabel('auto_warehouse_sell')).toBe('仓库自动出售');
	    expect(automationTaskLabel('qian_xing_travel_reward')).toBe('千星游记奖励领取');
    expect(automationTaskLabel('unknown_task')).toBe('unknown_task');
  });

  it('treats task.start as lifecycle noise', () => {
    expect(isTaskLifecycleNoise({
      type: 'task.start',
      message: 'automation task started',
    })).toBe(true);
  });

  it('keeps only finished task results for the workbench feed', () => {
    expect(isWorkbenchTaskEvent({
      id: 1,
      timestamp: '2026-07-13T12:00:00+08:00',
      level: 'info',
      source: 'auto_farm',
      type: 'task.start',
      message: 'automation task started',
      data: { taskId: 'own_plant' },
    })).toBe(false);

    expect(isWorkbenchTaskEvent({
      id: 2,
      timestamp: '2026-07-13T12:00:01+08:00',
      level: 'info',
      source: 'auto_farm',
      type: 'task.done',
      message: '没有检测到可种植空地，本轮自动种植跳过。',
      data: { taskId: 'own_plant' },
    })).toBe(true);

    expect(isWorkbenchTaskEvent({
      id: 3,
      timestamp: '2026-07-13T12:00:02+08:00',
      level: 'info',
      source: 'settings',
      type: 'settings.save',
      message: '设置已保存',
    })).toBe(false);
  });

  it('renders task name and result text for workbench rows', () => {
    const event = {
      id: 4,
      timestamp: '2026-07-13T12:00:03+08:00',
      level: 'info',
      source: 'auto_farm',
      type: 'task.done',
      message: '商城每日肥料：今日已领取或无可领取奖励。',
      data: { taskId: 'mall_daily_fertilizer' },
    };

    expect(workbenchTaskName(event)).toBe('商城每日肥料');
    expect(workbenchTaskResult(event)).toBe('商城每日肥料：今日已领取或无可领取奖励。');
  });
});

describe('TSDK block event helpers', () => {
  it('filters TSDK logs and returns newest first without mutating input', () => {
    const events = [
      { id: 1, timestamp: '2026-08-02T10:00:00+08:00', level: 'info', source: 'qq_ws', type: 'qqhost.log', message: '[TSDK-BLOCK] start' },
      { id: 3, timestamp: '2026-08-02T10:00:02+08:00', level: 'info', source: 'qq_ws', type: 'qqhost.log', message: '[TSDK-BLOCK] ready' },
      { id: 2, timestamp: '2026-08-02T10:00:01+08:00', level: 'info', source: 'account', type: 'account.confirmed', message: 'confirmed' },
    ];
    const originalIds = events.map((event) => event.id);

    expect(tsdkBlockEventsNewestFirst(events).map((event) => event.id)).toEqual([3, 1]);
    expect(events.map((event) => event.id)).toEqual(originalIds);
  });
});

