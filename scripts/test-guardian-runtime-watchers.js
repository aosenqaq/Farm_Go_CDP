#!/usr/bin/env node
'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

class FakeClock {
  constructor(now) {
    this.now = now;
    this.nextId = 1;
    this.tasks = new Map();
  }

  setTimeout(fn, delay, ...args) {
    const id = this.nextId++;
    this.tasks.set(id, {
      id,
      dueAt: this.now + Math.max(0, Number(delay) || 0),
      fn,
      args,
    });
    return id;
  }

  clearTimeout(id) {
    this.tasks.delete(id);
  }

  async flushMicrotasks() {
    for (let i = 0; i < 12; i += 1) await Promise.resolve();
  }

  async advance(ms) {
    const target = this.now + Math.max(0, Number(ms) || 0);
    for (;;) {
      let next = null;
      for (const task of this.tasks.values()) {
        if (task.dueAt <= target && (!next || task.dueAt < next.dueAt || (task.dueAt === next.dueAt && task.id < next.id))) {
          next = task;
        }
      }
      if (!next) break;
      this.tasks.delete(next.id);
      this.now = next.dueAt;
      next.fn(...next.args);
      await this.flushMicrotasks();
    }
    this.now = target;
    await this.flushMicrotasks();
  }

  get pendingCount() {
    return this.tasks.size;
  }

  get pendingIds() {
    return Array.from(this.tasks.keys()).sort((a, b) => a - b);
  }
}

class FakeNode {
  constructor(name) {
    this.name = name;
    this.parent = null;
    this.children = [];
    this.components = [];
    this.active = true;
    this.activeInHierarchy = true;
  }

  addChild(node) {
    node.parent = this;
    this.children.push(node);
  }

  getComponent(selector) {
    if (typeof selector === 'string') {
      return this.components.find((component) => component.constructor && component.constructor.name === selector) || null;
    }
    return this.components.find((component) => component instanceof selector) || null;
  }
}

class FakeButton {
  constructor(clickEvents) {
    this.clickEvents = clickEvents;
  }
}

class FakeLabel {
  constructor(string) {
    this.string = string;
  }
}

class FakeRichText extends FakeLabel {}

class ServerKickOutUICom {
  constructor(node, okNode, runtime) {
    this.node = node;
    this.mode = 0;
    this.normaltitle = { string: '提示' };
    this.normalcontent = { string: '' };
    this.content = { string: '' };
    this.operate = { content: '' };
    this.normalokLabel = { string: '确定' };
    this.normalcancelLabel = { string: '取消' };
    this.normalbtnOk = { node: okNode };
    this.normalbtnCancel = null;
    this.runtime = runtime;
  }

  confirm() {
    this.runtime.confirmCount += 1;
    if (this.runtime.hideOnConfirm) this.runtime.setPromptVisible(false);
  }
}

class GenericReloginPromptComp {
  constructor(node, runtime) {
    this.node = node;
    this.runtime = runtime;
  }

  relogin() {
    this.runtime.confirmCount += 1;
    if (this.runtime.hideOnConfirm) this.runtime.setPromptVisible(false);
  }

  exitGame() {
    this.runtime.exitCount += 1;
  }
}

function createRuntime(options) {
  options = options || {};
  const clock = new FakeClock(1_700_000_000_000);
  const events = [];
  const runtime = {
    confirmCount: 0,
    exitCount: 0,
    hideOnConfirm: options.hideOnConfirm !== false,
  };
  const scene = new FakeNode('Scene');
  const genericRelogin = options.promptKind === 'generic_relogin';
  const promptNode = new FakeNode(genericRelogin ? 'CommonPrompt' : 'ServerKickOut');
  scene.addChild(promptNode);

  function setNodeTreeActive(node, visible) {
    if (!node) return;
    node.active = !!visible;
    node.activeInHierarchy = !!visible;
    for (const child of node.children || []) setNodeTreeActive(child, visible);
  }

  runtime.setPromptVisible = (visible) => {
    setNodeTreeActive(promptNode, visible);
  };

  let component = null;
  if (genericRelogin) {
    const addTextNode = (parent, name, text, TextComponent) => {
      const node = new FakeNode(name);
      node.components.push(new TextComponent(text));
      parent.addChild(node);
      return node;
    };
    const addPromptButton = (parent, name, label, handler) => {
      const node = new FakeNode(name);
      node.components.push(new FakeButton([{
        target: parent,
        component: 'GenericReloginPromptComp',
        handler,
        customEventData: '',
      }]));
      addTextNode(node, 'label', label, FakeLabel);
      parent.addChild(node);
      return node;
    };

    component = new GenericReloginPromptComp(promptNode, runtime);
    promptNode.components.push(component);
    addTextNode(promptNode, 'title', '提示信息', FakeLabel);
    addTextNode(
      promptNode,
      'content',
      options.promptContent || '平台登录失败，是否重新登录？',
      FakeRichText,
    );
    addPromptButton(promptNode, 'btn_exit', '退出游戏', 'exitGame');
    addPromptButton(promptNode, 'btn_relogin', options.reloginLabel || '重新登录', 'relogin');
    if (options.duplicateRelogin) {
      addPromptButton(promptNode, 'btn_relogin_duplicate', '重新登录', 'relogin');
    }
  } else {
    const okNode = new FakeNode('Ok');
    promptNode.addChild(okNode);
    component = new ServerKickOutUICom(promptNode, okNode, runtime);
    promptNode.components.push(component);
    if (options.withHandler !== false) {
      okNode.components.push(new FakeButton([{
        target: promptNode,
        component: 'ServerKickOutUICom',
        handler: 'confirm',
        customEventData: '',
      }]));
    } else {
      component.normalbtnOk = null;
    }
  }

  runtime.setPromptKind = (kind) => {
    if (!(component instanceof ServerKickOutUICom)) return;
    component.normalcontent.string = kind === 'network'
      ? '网络连接超时，请重新连接'
      : (kind === 'other_place' ? '您的账号已在其他地方登录' : '');
  };

  function find(relativePath, root) {
    const parts = String(relativePath || '').split('/').filter(Boolean);
    let current = root;
    for (const part of parts) {
      current = current && current.children.find((child) => child.name === part);
      if (!current) return null;
    }
    return current || null;
  }

  const FakeDate = class extends Date {
    constructor(...args) {
      super(...(args.length ? args : [clock.now]));
    }
    static now() {
      return clock.now;
    }
  };

  const quietConsole = { log() {}, info() {}, warn() {}, error() {}, dir() {} };
  const context = {
    console: quietConsole,
    Date: FakeDate,
    JSON,
    Math,
    Number,
    String,
    Object,
    Array,
    Map,
    Set,
    Promise,
    RegExp,
    Error,
    isFinite,
    parseInt,
    parseFloat,
    setTimeout: clock.setTimeout.bind(clock),
    clearTimeout: clock.clearTimeout.bind(clock),
    document: { dispatchEvent() {} },
    canvas: { width: 100, height: 100 },
    __qqFarmRuntimeEventBridge(event) {
      events.push(event);
    },
  };
  context.globalThis = context;
  context.GameGlobal = context;
  context.cc = {
    director: { getScene: () => scene },
    find,
    game: { canvas: context.canvas },
    Button: FakeButton,
    UITransform: function UITransform() {},
    Camera: { main: null },
    Vec3: class Vec3 {
      constructor(x = 0, y = 0, z = 0) {
        this.x = x;
        this.y = y;
        this.z = z;
      }
    },
    Node: { EventType: { TOUCH_START: 'touch-start', TOUCH_END: 'touch-end' } },
    Label: FakeLabel,
    RichText: FakeRichText,
  };

  vm.createContext(context);
  const source = fs.readFileSync(path.join(__dirname, '..', 'resources', 'wmpf', 'button.js'), 'utf8');
  vm.runInContext(source, context, { filename: 'button.js', timeout: 5000 });

  runtime.clock = clock;
  runtime.context = context;
  runtime.events = events;
  runtime.gameCtl = context.gameCtl;
  if (!genericRelogin) runtime.setPromptKind(options.promptKind || 'network');
  runtime.setPromptVisible(options.visible !== false);
  return runtime;
}

function eventsByName(runtime, name) {
  return runtime.events.filter((event) => event.name === name);
}

function assertStateKeys(state, keys) {
  for (const key of keys) {
    assert.ok(Object.prototype.hasOwnProperty.call(state, key), `state should include ${key}`);
  }
}

async function testNetworkWatcherLifecycleAndIdempotency() {
  const runtime = createRuntime();
  const { gameCtl, clock } = runtime;
  const first = gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 1000,
    delayMs: 1000,
    waitAfter: 0,
    recoverTimeoutMs: 20000,
  });
  assert.equal(first.enabled, true);
  assert.equal(first.running, true);
  assert.equal(clock.pendingCount, 1);
  const firstTimerIds = clock.pendingIds;

  const unchanged = gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 1000,
    delayMs: 1000,
    waitAfter: 0,
    recoverTimeoutMs: 20000,
  });
  assert.equal(unchanged.running, true);
  assert.equal(unchanged.generation, first.generation);
  assert.deepEqual(clock.pendingIds, firstTimerIds, 'same-config start preserves the active timer');
  const unchangedBySetter = gameCtl.setReconnectWatcherEnabled(true, {
    silent: true,
    intervalMs: 1000,
    delayMs: 1000,
    waitAfter: 0,
    recoverTimeoutMs: 20000,
  });
  assert.equal(unchangedBySetter.generation, first.generation);
  assert.deepEqual(clock.pendingIds, firstTimerIds, 'same-config enabled setter preserves the active timer');

  const restarted = gameCtl.startReconnectWatcher({ silent: true, intervalMs: 1200, delayMs: 1000, waitAfter: 0, recoverTimeoutMs: 20000 });
  assert.ok(restarted.generation > unchanged.generation);
  assert.notDeepEqual(clock.pendingIds, firstTimerIds, 'changed options restart with a new timer');
  assert.equal(clock.pendingCount, 1);

  await clock.advance(1000);
  const reconnectEvents = eventsByName(runtime, 'network_reconnect');
  assert.equal(runtime.confirmCount, 1);
  assert.equal(reconnectEvents.length, 1);
  assert.equal(reconnectEvents[0].phase, 'reconnected');
  assert.equal(reconnectEvents[0].handled, true);
  assert.equal(reconnectEvents[0].via, 'server_kickout_ok_node');
  assert.equal(reconnectEvents[0].error, null);
  assert.equal(typeof reconnectEvents[0].checkedAt, 'number');
  assert.equal(clock.pendingCount, 1, 'successful check reschedules one timer');

  gameCtl.stopReconnectWatcher({ silent: true });
  assert.equal(clock.pendingCount, 0);
}

async function testNetworkFailureAndNoPromptSilence() {
  const runtime = createRuntime({ withHandler: false });
  const { gameCtl, clock } = runtime;
  gameCtl.startReconnectWatcher({ silent: true, intervalMs: 1000, delayMs: 1000, waitAfter: 0 });
  await clock.advance(1000);

  const failed = eventsByName(runtime, 'network_reconnect');
  assert.equal(failed.length, 1);
  assert.equal(failed[0].phase, 'failed');
  assert.equal(failed[0].handled, false);
  assert.equal(failed[0].via, null);
  assert.equal(failed[0].error, 'reconnect_handler_not_found');
  assert.equal(typeof failed[0].checkedAt, 'number');

  runtime.setPromptVisible(false);
  await clock.advance(1000);
  assert.equal(eventsByName(runtime, 'network_reconnect').length, 1, 'missing prompt does not emit failed lifecycle events');
  gameCtl.stopReconnectWatcher({ silent: true });
}

async function testNetworkRecoveringTimeoutStaysSilentAcrossChecks() {
  const runtime = createRuntime({ visible: false });
  const { gameCtl, clock } = runtime;
  runtime.context.gameState = { state: 'ReLogin' };
  gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 300,
    delayMs: 50,
    waitAfter: 0,
    recoverTimeoutMs: 100,
  });

  await clock.advance(1200);
  assert.equal(
    eventsByName(runtime, 'network_reconnect').length,
    0,
    'recovering without a prompt remains silent across watcher checks',
  );
  gameCtl.stopReconnectWatcher({ silent: true });
  assert.equal(clock.pendingCount, 0);
}

async function testHandledNetworkRecoveryTimeoutEmitsDiagnosticFailure() {
  const runtime = createRuntime({ hideOnConfirm: false });
  const { gameCtl, clock } = runtime;
  gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 300,
    delayMs: 50,
    waitAfter: 0,
    recoverTimeoutMs: 100,
  });

  await clock.advance(300);
  const firstGeneration = gameCtl.getReconnectWatcherState({ silent: true }).generation;
  const unchanged = gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 300,
    delayMs: 50,
    waitAfter: 0,
    recoverTimeoutMs: 100,
  });
  assert.equal(unchanged.generation, firstGeneration);
  await clock.advance(1200);
  const failed = eventsByName(runtime, 'network_reconnect');
  assert.equal(failed.length, 1);
  assert.equal(runtime.confirmCount, 1);
  assert.equal(failed[0].phase, 'failed');
  assert.equal(failed[0].handled, true);
  assert.equal(failed[0].error, 'recover_timeout');
  assert.equal(gameCtl.getReconnectWatcherState({ silent: true }).lastResult.reason, 'reconnect_episode_suppressed');
  gameCtl.stopReconnectWatcher({ silent: true });
  assert.equal(clock.pendingCount, 0);
}

async function testNetworkTerminalEpisodeResetsAfterPromptDisappears() {
  const runtime = createRuntime({ hideOnConfirm: false });
  const { gameCtl, clock } = runtime;
  gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 300,
    delayMs: 50,
    waitAfter: 0,
    recoverTimeoutMs: 100,
  });

  await clock.advance(300);
  assert.equal(runtime.confirmCount, 1);
  assert.equal(eventsByName(runtime, 'network_reconnect').length, 1);
  runtime.setPromptVisible(false);
  await clock.advance(300);
  runtime.setPromptVisible(true);
  await clock.advance(550);
  assert.equal(runtime.confirmCount, 2, 'a prompt that reappears starts a new reconnect episode');
  assert.equal(eventsByName(runtime, 'network_reconnect').length, 2);
  gameCtl.stopReconnectWatcher({ silent: true });
  assert.equal(clock.pendingCount, 0);
}

async function testNetworkTerminalEpisodeResetsAfterChangedConfig() {
  const runtime = createRuntime({ hideOnConfirm: false });
  const { gameCtl, clock } = runtime;
  gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 300,
    delayMs: 50,
    waitAfter: 0,
    recoverTimeoutMs: 100,
  });

  await clock.advance(300);
  assert.equal(runtime.confirmCount, 1);
  const beforeRestart = gameCtl.getReconnectWatcherState({ silent: true });
  const restarted = gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 300,
    delayMs: 50,
    waitAfter: 0,
    recoverTimeoutMs: 100,
    channelId: 9,
  });
  assert.ok(restarted.generation > beforeRestart.generation);
  await clock.advance(300);
  assert.equal(runtime.confirmCount, 2, 'changed config starts a new reconnect episode');
  assert.equal(eventsByName(runtime, 'network_reconnect').length, 2);
  gameCtl.stopReconnectWatcher({ silent: true });
  assert.equal(clock.pendingCount, 0);
  gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 300,
    delayMs: 50,
    waitAfter: 0,
    recoverTimeoutMs: 100,
    channelId: 9,
  });
  await clock.advance(300);
  assert.equal(runtime.confirmCount, 3, 'stop clears the terminal episode before a later restart');
  assert.equal(eventsByName(runtime, 'network_reconnect').length, 3);
  gameCtl.stopReconnectWatcher({ silent: true });
  assert.equal(clock.pendingCount, 0);
}

async function testMissingHandlerFailureDoesNotRepeatForSamePrompt() {
  const runtime = createRuntime({ withHandler: false });
  const { gameCtl, clock } = runtime;
  gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 300,
    delayMs: 50,
    waitAfter: 0,
  });

  await clock.advance(1200);
  const failed = eventsByName(runtime, 'network_reconnect');
  assert.equal(failed.length, 1, 'missing handler emits one terminal event for the visible prompt episode');
  assert.equal(failed[0].error, 'reconnect_handler_not_found');
  assert.equal(gameCtl.getReconnectWatcherState({ silent: true }).lastResult.reason, 'reconnect_episode_suppressed');
  gameCtl.stopReconnectWatcher({ silent: true });
  assert.equal(clock.pendingCount, 0);
}

async function testNetworkEnableDisableAndCompleteState() {
  const runtime = createRuntime();
  const { gameCtl, clock } = runtime;
  const started = gameCtl.setReconnectWatcherEnabled(true, {
    silent: true,
    intervalMs: 1000,
    delayMs: 1000,
    waitAfter: 0,
    recoverTimeoutMs: 4321,
  });
  assertStateKeys(started, [
    'enabled', 'running', 'busy', 'generation', 'intervalMs', 'recoverTimeoutMs',
    'lastCheckAt', 'lastHandledAt', 'lastResult',
  ]);
  assert.equal(started.recoverTimeoutMs, 4321);
  const activeGeneration = started.generation;

  const stopped = gameCtl.setReconnectWatcherEnabled(false, { silent: true });
  assert.equal(stopped.enabled, false);
  assert.equal(stopped.running, false);
  assert.ok(stopped.generation > activeGeneration);
  assert.equal(clock.pendingCount, 0);
  await clock.advance(5000);
  assert.equal(runtime.confirmCount, 0, 'disabled generations do not execute');
  assert.equal(eventsByName(runtime, 'network_reconnect').length, 0);

  const resumed = gameCtl.setReconnectWatcherEnabled(true, { silent: true, intervalMs: 1000, delayMs: 1000 });
  assert.equal(resumed.enabled, true);
  assert.equal(resumed.running, true);
  assert.ok(resumed.generation > stopped.generation);
  assert.equal(clock.pendingCount, 1);
  gameCtl.stopReconnectWatcher({ silent: true });
  assert.equal(clock.pendingCount, 0);
}

async function testDisabledNetworkGenerationCannotRescheduleFromAsyncFinally() {
  const runtime = createRuntime({ hideOnConfirm: false });
  const { gameCtl, clock } = runtime;
  gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 1000,
    delayMs: 1000,
    waitAfter: 0,
    recoverTimeoutMs: 500,
  });
  await clock.advance(1000);
  const busy = gameCtl.getReconnectWatcherState({ silent: true });
  assert.equal(busy.busy, true);
  assert.equal(clock.pendingCount, 1, 'recovery polling owns the only pending timer');
  const recoveryTimerIds = clock.pendingIds;
  const unchangedBusy = gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 1000,
    delayMs: 1000,
    waitAfter: 0,
    recoverTimeoutMs: 500,
  });
  assert.equal(unchangedBusy.busy, true, 'same-config start does not clear busy state');
  assert.equal(unchangedBusy.generation, busy.generation);
  assert.deepEqual(clock.pendingIds, recoveryTimerIds, 'same-config busy start preserves pending recovery wait');

  const stopped = gameCtl.setReconnectWatcherEnabled(false, { silent: true });
  const stoppedGeneration = stopped.generation;
  assert.equal(clock.pendingCount, 0, 'stop immediately cancels watcher-owned recovery polling');
  await clock.advance(1000);
  const afterOldFinally = gameCtl.getReconnectWatcherState({ silent: true });
  assert.equal(afterOldFinally.enabled, false);
  assert.equal(afterOldFinally.running, false);
  assert.equal(afterOldFinally.busy, false);
  assert.equal(afterOldFinally.generation, stoppedGeneration);
  assert.equal(clock.pendingCount, 0);
  assert.equal(eventsByName(runtime, 'network_reconnect').length, 0, 'stale async results emit no lifecycle event');
}

async function testNetworkStopCancelsPendingClickWait() {
  const runtime = createRuntime();
  const { gameCtl, clock } = runtime;
  gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 1000,
    delayMs: 1000,
    waitAfter: 500,
    recoverTimeoutMs: 20000,
  });
  await clock.advance(1000);
  assert.equal(runtime.confirmCount, 1);
  assert.equal(gameCtl.getReconnectWatcherState({ silent: true }).busy, true);
  assert.equal(clock.pendingCount, 1, 'network click waitAfter owns one pending timer');

  gameCtl.stopReconnectWatcher({ silent: true });
  assert.equal(clock.pendingCount, 0, 'stop synchronously clears network click waitAfter');
  const stoppedGeneration = gameCtl.getReconnectWatcherState({ silent: true }).generation;
  const stoppedAgain = gameCtl.setReconnectWatcherEnabled(false, { silent: true });
  assert.equal(stoppedAgain.generation, stoppedGeneration, 'repeated network stop is generation-idempotent');
  assert.equal(clock.pendingCount, 0);
  await clock.flushMicrotasks();
  assert.equal(gameCtl.getReconnectWatcherState({ silent: true }).busy, false, 'cancelled click Promise exits');
  assert.equal(eventsByName(runtime, 'network_reconnect').length, 0);
  await clock.advance(5000);
  assert.equal(clock.pendingCount, 0);
  assert.equal(runtime.confirmCount, 1);
  assert.equal(eventsByName(runtime, 'network_reconnect').length, 0);
}

async function testNetworkChangedConfigCancelsBusyGeneration() {
  const runtime = createRuntime({ hideOnConfirm: false });
  const { gameCtl, clock } = runtime;
  gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 1000,
    delayMs: 1000,
    waitAfter: 0,
    recoverTimeoutMs: 5000,
  });
  await clock.advance(1000);
  const busy = gameCtl.getReconnectWatcherState({ silent: true });
  const oldWaitIds = clock.pendingIds;
  assert.equal(busy.busy, true);
  assert.equal(oldWaitIds.length, 1);
  assert.equal(runtime.confirmCount, 1);

  const restarted = gameCtl.setReconnectWatcherEnabled(true, {
    silent: true,
    intervalMs: 1200,
    delayMs: 5000,
    waitAfter: 0,
    recoverTimeoutMs: 5000,
    channelId: 9,
  });
  assert.ok(restarted.generation > busy.generation);
  assert.equal(restarted.busy, false);
  assert.equal(clock.pendingCount, 1, 'changed config leaves only the new generation timer');
  assert.notDeepEqual(clock.pendingIds, oldWaitIds);
  const newTimerIds = clock.pendingIds;

  await clock.flushMicrotasks();
  assert.deepEqual(clock.pendingIds, newTimerIds, 'old async completion does not reschedule');
  assert.equal(runtime.confirmCount, 1);
  assert.equal(eventsByName(runtime, 'network_reconnect').length, 0);
  await clock.advance(4999);
  assert.deepEqual(clock.pendingIds, newTimerIds);
  assert.equal(runtime.confirmCount, 1, 'old generation does not click again');
  assert.equal(eventsByName(runtime, 'network_reconnect').length, 0);
  gameCtl.stopReconnectWatcher({ silent: true });
  assert.equal(clock.pendingCount, 0);
}

async function testGenericReloginPopupClicksOnlyRelogin() {
  const runtime = createRuntime({ promptKind: 'generic_relogin' });
  const { gameCtl, clock } = runtime;
  gameCtl.startReconnectWatcher({
    silent: true,
    intervalMs: 1000,
    delayMs: 1000,
    waitAfter: 0,
    recoverTimeoutMs: 20000,
  });

  await clock.advance(1000);

  const events = eventsByName(runtime, 'network_reconnect');
  assert.equal(runtime.confirmCount, 1, 'generic popup triggers relogin once');
  assert.equal(runtime.exitCount, 0, 'generic popup never triggers exit game');
  assert.equal(events.length, 1);
  assert.equal(events[0].phase, 'reconnected');
  assert.equal(events[0].handled, true);
  assert.equal(events[0].via, 'relogin_ok_node');
  gameCtl.stopReconnectWatcher({ silent: true });
}

async function testGenericReloginPopupRejectsIncompleteMatches() {
  const cases = [
    { promptContent: '普通提示，是否继续？', reloginLabel: '重新登录' },
    { promptContent: '平台登录失败，是否重新登录？', reloginLabel: '确定' },
    { promptContent: '平台登录失败，是否重新登录？', reloginLabel: '重新登录', duplicateRelogin: true },
  ];

  for (const testCase of cases) {
    const runtime = createRuntime({ promptKind: 'generic_relogin', ...testCase });
    const { gameCtl, clock } = runtime;
    gameCtl.startReconnectWatcher({ silent: true, intervalMs: 1000, delayMs: 1000, waitAfter: 0 });
    await clock.advance(1000);
    assert.equal(runtime.confirmCount, 0);
    assert.equal(runtime.exitCount, 0);
    assert.equal(eventsByName(runtime, 'network_reconnect').length, 0);
    gameCtl.stopReconnectWatcher({ silent: true });
  }
}

async function testOtherPlacePromptDeadlineSurvivesHiddenPrompt() {
  const runtime = createRuntime({ promptKind: 'other_place' });
  const { gameCtl, clock } = runtime;
  const first = gameCtl.startOtherPlaceLoginReconnectWatcher({
    silent: true,
    intervalMs: 1000,
    reconnectDelayMs: 1000,
    waitAfter: 0,
  });
  assertStateKeys(first, ['enabled', 'running', 'busy', 'generation']);
  assert.equal(first.enabled, true);
  assert.equal(first.running, true);
  assert.equal(clock.pendingCount, 1);

  await clock.advance(50);
  const detectedAt = gameCtl.getOtherPlaceLoginReconnectState({ silent: true }).firstDetectedAt;
  assert.equal(typeof detectedAt, 'number');
  runtime.setPromptVisible(false);
  const timerIdsBeforeSameStart = clock.pendingIds;

  const unchanged = gameCtl.startOtherPlaceLoginReconnectWatcher({
    silent: true,
    intervalMs: 1000,
    reconnectDelayMs: 1000,
    waitAfter: 0,
  });
  assert.equal(unchanged.generation, first.generation);
  assert.deepEqual(clock.pendingIds, timerIdsBeforeSameStart, 'same-config other-place start preserves its timer');
  assert.equal(unchanged.firstDetectedAt, detectedAt);
  const unchangedBySetter = gameCtl.setOtherPlaceLoginReconnectEnabled(true, {
    silent: true,
    intervalMs: 1000,
    reconnectDelayMs: 1000,
    waitAfter: 0,
  });
  assert.equal(unchangedBySetter.generation, first.generation);
  assert.deepEqual(clock.pendingIds, timerIdsBeforeSameStart, 'same-config other-place enabled setter preserves its timer');

  const restarted = gameCtl.startOtherPlaceLoginReconnectWatcher({
    silent: true,
    intervalMs: 1000,
    reconnectDelayMs: 1000,
    waitAfter: 0,
    channelId: 7,
  });
  assert.ok(restarted.generation > unchanged.generation);
  assert.notDeepEqual(clock.pendingIds, timerIdsBeforeSameStart, 'changed other-place options restart its timer');
  assert.equal(restarted.firstDetectedAt, detectedAt, 'restart preserves the original detection deadline');
  assert.equal(clock.pendingCount, 1);
  await clock.advance(1200);
  assert.equal(runtime.confirmCount, 0);
  assert.equal(gameCtl.getOtherPlaceLoginReconnectState({ silent: true }).firstDetectedAt, detectedAt);

  runtime.setPromptVisible(true);
  await clock.advance(1000);
  assert.equal(runtime.confirmCount, 1);
  assert.equal(eventsByName(runtime, 'other_place_login_reconnect').filter((event) => event.phase === 'reconnected').length, 1);

  gameCtl.setOtherPlaceLoginReconnectEnabled(true, { silent: true, intervalMs: 1000, reconnectDelayMs: 1000 });
  gameCtl.startOtherPlaceLoginReconnectWatcher({ silent: true, intervalMs: 1000, reconnectDelayMs: 1000 });
  assert.equal(clock.pendingCount, 1, 'repeated enable/start keeps one other-place timer');
  await clock.advance(3000);
  assert.equal(runtime.confirmCount, 1);
  assert.equal(eventsByName(runtime, 'other_place_login_reconnect').filter((event) => event.phase === 'reconnected').length, 1);

  gameCtl.stopOtherPlaceLoginReconnectWatcher({ silent: true });
  gameCtl.stopReconnectWatcher({ silent: true });
  const eventCount = runtime.events.length;
  assert.equal(clock.pendingCount, 0, 'stopping both watchers clears all fake timers');
  await clock.advance(10000);
  assert.equal(clock.pendingCount, 0);
  assert.equal(runtime.events.length, eventCount, 'stopped watchers emit no later events');
}

async function testOtherPlaceStopCancelsPendingClickWait() {
  const runtime = createRuntime({ promptKind: 'other_place' });
  const { gameCtl, clock } = runtime;
  gameCtl.startOtherPlaceLoginReconnectWatcher({
    silent: true,
    intervalMs: 1000,
    reconnectDelayMs: 0,
    waitAfter: 500,
  });
  await clock.advance(50);
  assert.equal(runtime.confirmCount, 1);
  const busy = gameCtl.getOtherPlaceLoginReconnectState({ silent: true });
  assert.equal(busy.busy, true);
  assert.equal(clock.pendingCount, 1, 'click waitAfter owns one pending timer');
  const clickWaitTimerIds = clock.pendingIds;
  const unchangedBusy = gameCtl.setOtherPlaceLoginReconnectEnabled(true, {
    silent: true,
    intervalMs: 1000,
    reconnectDelayMs: 0,
    waitAfter: 500,
  });
  assert.equal(unchangedBusy.busy, true, 'same-config enabled setter does not clear busy state');
  assert.equal(unchangedBusy.generation, busy.generation);
  assert.deepEqual(clock.pendingIds, clickWaitTimerIds, 'same-config busy setter preserves pending click wait');

  gameCtl.stopOtherPlaceLoginReconnectWatcher({ silent: true });
  assert.equal(clock.pendingCount, 0, 'stop immediately cancels watcher-owned click waitAfter');
  const stoppedGeneration = gameCtl.getOtherPlaceLoginReconnectState({ silent: true }).generation;
  const stoppedAgain = gameCtl.setOtherPlaceLoginReconnectEnabled(false, { silent: true });
  assert.equal(stoppedAgain.generation, stoppedGeneration, 'repeated other-place stop is generation-idempotent');
  assert.equal(clock.pendingCount, 0);
  await clock.advance(2000);
  assert.equal(runtime.confirmCount, 1);
  assert.equal(eventsByName(runtime, 'other_place_login_reconnect').filter((event) => event.phase === 'reconnected').length, 0);
  assert.equal(clock.pendingCount, 0);
}

async function testOtherPlaceChangedConfigCancelsBusyGeneration() {
  const runtime = createRuntime({ promptKind: 'other_place' });
  const { gameCtl, clock } = runtime;
  gameCtl.startOtherPlaceLoginReconnectWatcher({
    silent: true,
    intervalMs: 1000,
    reconnectDelayMs: 0,
    waitAfter: 500,
  });
  await clock.advance(50);
  const busy = gameCtl.getOtherPlaceLoginReconnectState({ silent: true });
  const oldWaitIds = clock.pendingIds;
  assert.equal(busy.busy, true);
  assert.equal(runtime.confirmCount, 1);
  assert.equal(oldWaitIds.length, 1);

  const restarted = gameCtl.startOtherPlaceLoginReconnectWatcher({
    silent: true,
    intervalMs: 1000,
    delayMs: 1000,
    reconnectDelayMs: 0,
    waitAfter: 500,
    channelId: 7,
  });
  assert.ok(restarted.generation > busy.generation);
  assert.equal(restarted.busy, false);
  assert.equal(clock.pendingCount, 1, 'changed config leaves only the new other-place timer');
  assert.notDeepEqual(clock.pendingIds, oldWaitIds);
  const newTimerIds = clock.pendingIds;

  await clock.flushMicrotasks();
  assert.deepEqual(clock.pendingIds, newTimerIds, 'old other-place async completion does not reschedule');
  assert.equal(runtime.confirmCount, 1);
  assert.equal(eventsByName(runtime, 'other_place_login_reconnect').filter((event) => event.phase === 'reconnected').length, 0);
  await clock.advance(999);
  assert.deepEqual(clock.pendingIds, newTimerIds);
  assert.equal(runtime.confirmCount, 1, 'old other-place generation does not click twice');
  assert.equal(eventsByName(runtime, 'other_place_login_reconnect').filter((event) => event.phase === 'reconnected').length, 0);
  gameCtl.stopOtherPlaceLoginReconnectWatcher({ silent: true });
  assert.equal(clock.pendingCount, 0);
}

async function run() {
  await testNetworkWatcherLifecycleAndIdempotency();
  await testNetworkFailureAndNoPromptSilence();
  await testNetworkRecoveringTimeoutStaysSilentAcrossChecks();
  await testHandledNetworkRecoveryTimeoutEmitsDiagnosticFailure();
  await testNetworkTerminalEpisodeResetsAfterPromptDisappears();
  await testNetworkTerminalEpisodeResetsAfterChangedConfig();
  await testMissingHandlerFailureDoesNotRepeatForSamePrompt();
  await testNetworkEnableDisableAndCompleteState();
  await testDisabledNetworkGenerationCannotRescheduleFromAsyncFinally();
  await testNetworkStopCancelsPendingClickWait();
  await testNetworkChangedConfigCancelsBusyGeneration();
  await testGenericReloginPopupClicksOnlyRelogin();
  await testGenericReloginPopupRejectsIncompleteMatches();
  await testOtherPlacePromptDeadlineSurvivesHiddenPrompt();
  await testOtherPlaceStopCancelsPendingClickWait();
  await testOtherPlaceChangedConfigCancelsBusyGeneration();
  console.log('guardian runtime watcher tests passed');
}

run().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
