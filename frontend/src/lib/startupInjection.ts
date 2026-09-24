export type RuntimeStatusLike = {
  target?: string;
  phase?: string;
  connected?: boolean;
  ready?: boolean;
  instanceId?: string;
  lastError?: string;
  progressDetail?: string;
  scriptHash?: string;
};

export type QQPatchStatusLike = {
  phase?: string;
  injected?: boolean;
  inFlight?: boolean;
  lastTrigger?: string;
  action?: string;
  targetPath?: string;
  backupPath?: string;
  scriptHash?: string;
  hostVersion?: string;
  candidatePaths?: string[];
  restartRequired?: boolean;
  error?: string;
};

export type StartupInjectionTone = 'info' | 'success' | 'warning' | 'error';

export type StartupInjectionState = {
  target: string;
  title: string;
  headline: string;
  detail: string;
  tone: StartupInjectionTone;
  primaryMetricLabel: string;
  primaryMetricValue: string;
  secondaryMetricLabel: string;
  secondaryMetricValue: string;
  tertiaryMetricLabel: string;
  tertiaryMetricValue: string;
  showRetry: boolean;
  showLaunch: boolean;
  showRestart: boolean;
  inFlight: boolean;
  shouldShow: boolean;
  targetPath?: string;
  backupPath?: string;
  scriptHash?: string;
  connectionMethod?: string;
  error?: string;
};

type ChannelInfo = {
  target: string;
  title: string;
  platform: string;
  runtimeLabel: string;
};

const channels: Record<string, ChannelInfo> = {
  qq_ws: {
    target: 'qq_ws',
    title: 'QQ 补丁注入',
    platform: 'QQ',
    runtimeLabel: 'QQ WS',
  },
  wechat_cdp: {
    target: 'wechat_cdp',
    title: '微信 Frida 注入',
    platform: '微信',
    runtimeLabel: '微信 CDP',
  },
  yyb_cdp: {
    target: 'yyb_cdp',
    title: '应用宝 Frida 注入',
    platform: '应用宝',
    runtimeLabel: '应用宝 CDP',
  },
};

const phaseLabels: Record<string, string> = {
  idle: '等待启动',
  scanning: '扫描中',
  listening: '正在监听',
  handshaking: '握手中',
  ready: '已就绪',
  disconnected: '已断开',
  error: '失败',
};

export function channelForTarget(target?: string): ChannelInfo {
  return channels[target || ''] || channels.qq_ws;
}

export function normalizeStartupInjectionState(
  status: RuntimeStatusLike,
  patchStatus: QQPatchStatusLike | null,
): StartupInjectionState {
  const target = status.target || '';
  if (!target) return pendingStartupInjectionState();
  const channel = channelForTarget(target);
  if (target === 'qq_ws') return normalizeQQState(channel, status, patchStatus);
  return normalizeWMPFState(channel, status);
}

export function shouldDisplayStartupInjectionDialog(state: StartupInjectionState | null, hidden: boolean) {
  return Boolean(state && state.shouldShow && !hidden);
}

function normalizeQQState(
  channel: ChannelInfo,
  status: RuntimeStatusLike,
  patchStatus: QQPatchStatusLike | null,
): StartupInjectionState {
  const runtimePhase = status.phase || 'idle';
  const runtimeReady = status.ready === true;
  const runtimeConnected = status.connected === true;
  const miniappRunning = runtimeConnected || runtimePhase === 'handshaking' || runtimePhase === 'ready';
  const phase = patchStatus?.phase || 'idle';
  const inFlight = patchStatus?.inFlight === true;
  const injected = patchStatus?.injected === true;
  const failed = phase === 'error' || Boolean(patchStatus?.error);
  const patchCorrect = injected && !failed;
  const restartRequired = patchCorrect && (
    patchStatus?.restartRequired === true ||
    patchStatus?.action === 'patched' ||
    patchStatus?.action === 'replaced'
  );
  const runningHash = status.scriptHash || '';
  const patchedHash = patchStatus?.scriptHash || '';
  const runtimeHashMismatch = runtimeReady && patchCorrect && Boolean(runningHash && patchedHash && runningHash !== patchedHash);
  if (runtimeHashMismatch || (runtimeReady && restartRequired && (!runningHash || !patchedHash))) {
    return {
      target: channel.target,
      title: channel.title,
      headline: 'QQ 小程序补丁已写入',
      detail: '运行中的 QQ 小程序仍在使用旧补丁，已写入新补丁，请重新启动 QQ 小程序加载。',
      tone: 'warning',
      primaryMetricLabel: '运行 Hash',
      primaryMetricValue: runningHash || '-',
      secondaryMetricLabel: '补丁 Hash',
      secondaryMetricValue: patchedHash || '-',
      tertiaryMetricLabel: '上下文',
      tertiaryMetricValue: status.instanceId || '已连接',
      showRetry: false,
      showLaunch: false,
      showRestart: true,
      inFlight,
      shouldShow: true,
      targetPath: patchStatus?.targetPath,
      backupPath: patchStatus?.backupPath,
      scriptHash: patchedHash,
      error: patchStatus?.error,
    };
  }
  if (runtimeReady) {
    return {
      target: channel.target,
      title: channel.title,
      headline: 'QQ链路已连接',
      detail: 'QQ 农场小程序补丁和 WS 链路已就绪，可以使用自动化功能。',
      tone: 'success',
      primaryMetricLabel: '链路',
      primaryMetricValue: phaseLabels[runtimePhase] || runtimePhase,
      secondaryMetricLabel: '小程序连接',
      secondaryMetricValue: '已连接',
      tertiaryMetricLabel: '上下文',
      tertiaryMetricValue: status.instanceId || '已就绪',
      showRetry: false,
      showLaunch: false,
      showRestart: false,
      inFlight: false,
      shouldShow: false,
      targetPath: patchStatus?.targetPath,
      backupPath: patchStatus?.backupPath,
      scriptHash: patchStatus?.scriptHash,
      error: patchStatus?.error,
    };
  }

  const showLaunch = patchCorrect && !miniappRunning;
  const showRestart = patchCorrect && restartRequired && miniappRunning;
  const showRetry = !patchCorrect;

  const headline = inFlight
    ? '正在扫描 QQ 小程序补丁'
    : patchCorrect
      ? restartRequired
        ? 'QQ 小程序补丁已写入'
        : 'QQ 小程序补丁已就绪'
      : failed
        ? '自动注入失败'
        : miniappRunning
          ? '检测到 QQ 小程序补丁不正确'
          : '等待 QQ 小程序补丁注入';
  const detail = patchCorrect
    ? showRestart
      ? '新补丁已写入，需重新启动 QQ 小程序让当前运行实例加载新补丁。'
      : showLaunch
        ? restartRequired
          ? '补丁已写入，请启动 QQ 小程序加载补丁并连接 Farm_Go。'
          : '补丁已正确写入，请启动 QQ 小程序连接 Farm_Go。'
        : 'QQ 小程序补丁已就绪，已检测到运行实例，正在等待 Farm_Go WS 链路握手。'
    : failed
      ? patchStatus?.error || '未能自动写入 QQ 小程序补丁，可指定 game.js 路径后重新注入。'
      : miniappRunning
        ? '当前 QQ 小程序正在运行，但补丁不正确或未写入。请先重新注入，成功后再重新启动小程序。'
        : '正在自动查找最近打开的 QQ 小程序 game.js 并写入调试补丁。';

  return {
    target: channel.target,
    title: channel.title,
    headline,
    detail,
    tone: patchCorrect ? 'success' : failed ? 'error' : inFlight ? 'info' : 'warning',
    primaryMetricLabel: '补丁状态',
    primaryMetricValue: phaseLabels[phase] || phase,
    secondaryMetricLabel: '目标',
    secondaryMetricValue: patchStatus?.targetPath || firstCandidatePath(patchStatus) || '-',
    tertiaryMetricLabel: 'Host 版本',
    tertiaryMetricValue: patchStatus?.hostVersion || '-',
    showRetry,
    showLaunch,
    showRestart,
    inFlight,
    shouldShow: true,
    targetPath: patchStatus?.targetPath,
    backupPath: patchStatus?.backupPath,
    scriptHash: patchStatus?.scriptHash,
    error: patchStatus?.error,
  };
}

function pendingStartupInjectionState(): StartupInjectionState {
  return {
    target: '',
    title: '链路加载中',
    headline: '正在读取启动链路',
    detail: '正在同步本机持久化链路配置。',
    tone: 'info',
    primaryMetricLabel: '链路',
    primaryMetricValue: '待同步',
    secondaryMetricLabel: '小程序连接',
    secondaryMetricValue: '等待状态',
    tertiaryMetricLabel: '上下文',
    tertiaryMetricValue: '-',
    showRetry: false,
    showLaunch: false,
    showRestart: false,
    inFlight: false,
    shouldShow: false,
  };
}

function normalizeWMPFState(channel: ChannelInfo, status: RuntimeStatusLike): StartupInjectionState {
  const phase = status.phase || 'idle';
  const ready = status.ready === true;
  const connected = status.connected === true;
  const progressDetail = status.progressDetail?.trim() || '';
  const failed = phase === 'error' || Boolean(status.lastError);
  const miniappConnected = connected || phase === 'handshaking';
  const showLaunch = !ready && !miniappConnected;
  const headline = ready
    ? `${channel.platform}链路已连接`
    : failed
      ? `${channel.platform}链路暂未就绪`
      : progressDetail
        ? '正在建立可执行上下文'
        : miniappConnected
          ? `正在确认${channel.platform}小程序上下文`
          : `等待${channel.platform}小程序连接`;
  const detail = ready
    ? `${channel.platform} QQ 农场小程序上下文已定位，可以使用自动化功能。`
    : failed
      ? status.lastError || `${channel.platform}链路出现异常，请查看日志定位原因。`
      : progressDetail
        ? `已检测到${channel.platform} QQ 农场小程序连接，正在通过兼容 CDP 路径建立自动化连接。`
        : miniappConnected
          ? `已检测到${channel.platform} QQ 农场小程序连接，正在等待 gameContext 可执行上下文。`
          : `请打开或重新进入${channel.platform} QQ 农场小程序，注入会自动完成。`;

  return {
    target: channel.target,
    title: channel.title,
    headline,
    detail,
    tone: ready ? 'success' : failed ? 'error' : miniappConnected ? 'info' : 'warning',
    primaryMetricLabel: '链路',
    primaryMetricValue: phaseLabels[phase] || phase,
    secondaryMetricLabel: '小程序连接',
    secondaryMetricValue: connected ? '已连接' : '等待连接',
    tertiaryMetricLabel: '上下文',
    tertiaryMetricValue: ready ? status.instanceId || '已就绪' : progressDetail ? '正在确认' : '未就绪',
    showRetry: false,
    showLaunch,
    showRestart: false,
    // Keep the launch button clickable while waiting for the miniapp to connect.
    // Mark in-flight only after a connection is detected and context is still being established.
    inFlight: !ready && !failed && miniappConnected,
    shouldShow: !ready,
    connectionMethod: progressDetail ? '直接 CDP Runtime.enable（兼容路径）' : undefined,
    error: failed ? status.lastError : undefined,
  };
}

function firstCandidatePath(patchStatus: QQPatchStatusLike | null) {
  return Array.isArray(patchStatus?.candidatePaths) ? patchStatus.candidatePaths[0] : '';
}
