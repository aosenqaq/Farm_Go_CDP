import { useCallback, useEffect, useRef, useState } from 'react';

import {
  CheckForUpdates,
  ConfirmRuntimeAccount,
  CurrentRuntimeAccount,
  ExportRuntimeLogs,
  FarmAutomationState,
  FarmBackpackSeedOptions,
  FarmMysteryShopPurchaseRecords,
  FarmStealCropOptions,
  FarmSocialAction,
  FarmSocialDogGuardAction,
  FarmSocialDogGuardState,
  FarmSocialExport,
  FarmSocialImport,
  FarmSocialProtocolBlockList,
  FarmSocialRankingPreferences,
  FarmSocialRankings,
  FarmSocialRefreshVisitors,
  FarmSocialState,
  InstallQQDebugPatch,
  IdentifyRuntimeAccount,
  LaunchHostProcess,
  QQDebugPatchStatus,
  RestartHostProcess,
  RunDiagnostic,
  RunFarmAutomationTask,
  SaveFarmAutomationState,
  SetFarmAutomationRunMode,
  SaveFarmSocialRankingPreferences,
  SaveTextFile,
  SaveGuardianSettings,
  SaveUpdateCheckPreferences,
  StartFarmAutomationScheduler,
  StopFarmAutomationScheduler,
  UpdateCheckPreferences,
} from '../wailsjs/go/desktop/App';
import { BrowserOpenURL } from '../wailsjs/runtime/runtime';
import { AccountIdentityDialog, type RuntimeAccountIdentity } from './components/AccountIdentityDialog';
import { AppShell, type Tab } from './components/AppShell';
import { StartupInjectionDialog } from './components/StartupInjectionDialog';
import { UpdateDetailsDialog } from './components/UpdateDetailsDialog';
import {
  createAccountScopeGeneration,
  resolveAccountConfirmation,
  type AccountScopeGeneration,
  type AccountScopeToken,
} from './lib/accountScope';
import type { RuntimeEventDto } from './lib/events';
import type { RankingDataVersions, RankingPage, RankingPageRequest } from './lib/socialRankingPages';
import { PollApiError, pollDashboard, pollDogGuard } from './lib/pollApi';
import { createPollingController } from './lib/pollingController';
import { getValidatedUpdateDownloadURL, type UpdateCheckPreferencesDto, type UpdateStateDto } from './lib/update';
import { createSingleFlightUpdateCheck, startRecurringUpdateChecks } from './lib/updateCheckScheduler';
import { createScopedLatestSaveQueue, type ScopedLatestSaveQueue } from './lib/scopedLatestSaveQueue';
import {
  normalizeStartupInjectionState,
  shouldDisplayStartupInjectionDialog,
  type QQPatchStatusLike,
} from './lib/startupInjection';
import { AccountStatusView } from './views/AccountStatusView';
import type { FarmAutomationActionResult, FarmAutomationState as FarmAutomationViewState, MysteryShopPurchaseRecord } from './views/AutomationView';
import { FarmWorkspaceView } from './views/FarmWorkspaceView';
import { GuardView, type GuardStatusDto, type HostBindingStatusDto } from './views/GuardView';
import { LogsView } from './views/LogsView';
import { MessagePushView } from './views/MessagePushView';
import type { RuntimeStatusDto, WorkspaceRunStatistics } from './views/OverviewView';
import { SettingsView } from './views/SettingsView';
import type {
  DogGuardState,
  ImportExportPayload,
  RankingPreferences,
  SaveTextFileRequest,
  SaveTextFileResult,
  SocialActionRequest,
  SocialActionResult,
  SocialState,
} from './views/SocialView';

const initialStatus: RuntimeStatusDto = {
  target: '',
  phase: 'idle',
  connected: false,
  ready: false,
};

const initialGuardStatus: GuardStatusDto = {
  phase: 'disabled',
  runtimeTarget: '',
  timeoutStreak: 0,
  threshold: 3,
  restartCountInWindow: 0,
  maxRestartsPerWindow: 4,
  recentRestartEvents: [],
};

export type SocialRefreshPolicy = {
  refreshState: boolean;
  refreshRankingPreferences: boolean;
  refreshDogGuard: boolean;
  pollDogGuard: boolean;
};

const noSocialRefresh: SocialRefreshPolicy = {
  refreshState: false,
  refreshRankingPreferences: false,
  refreshDogGuard: false,
  pollDogGuard: false,
};

export function socialRefreshPolicyForTab(tab: Tab): SocialRefreshPolicy {
  if (tab === 'automation') return { ...noSocialRefresh, refreshState: true };
  if (tab !== 'social') return noSocialRefresh;
  return {
    refreshState: true,
    refreshRankingPreferences: true,
    refreshDogGuard: true,
    pollDogGuard: true,
  };
}

export function shouldRefreshSocialAfterDogGuardAction(action: string) {
  return action === 'clear';
}

export function shouldApplyScopedDogGuardResult(scope: AccountScopeGeneration, token: AccountScopeToken) {
  return scope.isCurrent(token);
}

export function rankingDataVersionsAfterSocialAction(
  current: RankingDataVersions,
  action: string,
  ok: boolean,
): RankingDataVersions {
  if (!ok || action !== 'steal') return current;
  return { ...current, stolenByMe: current.stolenByMe + 1 };
}

export function rankingDataVersionsAfterImport(
  current: RankingDataVersions,
  groups: string[],
  ok: boolean,
): RankingDataVersions {
  if (!ok) return current;
  const imported = new Set(groups);
  return {
    stolenByMe: imported.has('steal_records') ? current.stolenByMe + 1 : current.stolenByMe,
    visitors: imported.has('visitor_records') ? current.visitors + 1 : current.visitors,
  };
}

export function runtimePatchCheckKey(status: RuntimeStatusDto, patchHash = '') {
  if (status.target !== 'qq_ws' || status.ready !== true || status.connected !== true) return '';
  return `${status.target}:${patchHash || 'unverified'}`;
}

export function runtimePatchCheckDecision(checkedKey: string, status: RuntimeStatusDto, patchHash = '') {
  const nextCheckedKey = runtimePatchCheckKey(status, patchHash);
  return {
    nextCheckedKey,
    shouldRun: nextCheckedKey !== '' && nextCheckedKey !== checkedKey,
  };
}

export function runtimeHashCheckKey(status: RuntimeStatusDto, patchHash = '') {
  if (status.target !== 'qq_ws' || status.ready !== true || status.connected !== true) return '';
  return `${status.target}:${status.instanceId || 'ready'}:${patchHash || 'unverified'}`;
}

export function runtimePatchMismatchDecision(revealedKey: string, runningHash: string, patchHash: string) {
  const nextMismatchKey = runningHash && patchHash && runningHash !== patchHash
    ? `${runningHash}:${patchHash}`
    : '';
  return {
    nextMismatchKey,
    shouldReveal: nextMismatchKey !== '' && nextMismatchKey !== revealedKey,
    shouldClear: nextMismatchKey === '' && revealedKey !== '',
  };
}

function scriptHashFromDiagnostic(value: unknown) {
  const payload = value as { ok?: boolean; result?: any; scriptHash?: unknown } | null;
  if (!payload || payload.ok === false) return '';
  const directHash = payload.scriptHash;
  if (typeof directHash === 'string') return directHash;
  const resultHash = payload.result?.scriptHash;
  return typeof resultHash === 'string' ? resultHash : '';
}

type AuthorizedAppProps = {
  remote?: boolean;
};

const defaultUpdateCheckPreferences: UpdateCheckPreferencesDto = {
  enabled: true,
  intervalMinutes: 120,
};

export function normalizeUpdateCheckPreferences(value?: Partial<UpdateCheckPreferencesDto> | null): UpdateCheckPreferencesDto {
  const interval = Number(value?.intervalMinutes);
  return {
    enabled: typeof value?.enabled === 'boolean' ? value.enabled : defaultUpdateCheckPreferences.enabled,
    intervalMinutes: Number.isInteger(interval) && interval >= 1 && interval <= 10080
      ? interval
      : defaultUpdateCheckPreferences.intervalMinutes,
  };
}

export function isDesktopOnlyControlAvailable(remote: boolean) {
  return !remote;
}

function AuthorizedApp({ remote = false }: AuthorizedAppProps) {
  const [activeTab, setActiveTab] = useState<Tab>('workspace');
  const [status, setStatus] = useState<RuntimeStatusDto>(initialStatus);
  const [events, setEvents] = useState<RuntimeEventDto[]>([]);
  const [tsdkEvents, setTsdkEvents] = useState<RuntimeEventDto[]>([]);
  const [guardStatus, setGuardStatus] = useState<GuardStatusDto>(initialGuardStatus);
  const [bindingStatus, setBindingStatus] = useState<HostBindingStatusDto | null>(null);
  const [patchStatus, setPatchStatus] = useState<QQPatchStatusLike | null>(null);
  const [runtimeScriptHash, setRuntimeScriptHash] = useState('');
  const [runtimeAccount, setRuntimeAccount] = useState<RuntimeAccountIdentity | null>(null);
  const [accountIdentifying, setAccountIdentifying] = useState(false);
  const [accountConfirming, setAccountConfirming] = useState(false);
  const [accountError, setAccountError] = useState('');
  const [automationState, setAutomationState] = useState<FarmAutomationViewState | null>(null);
  const [automationActionResult, setAutomationActionResult] = useState<FarmAutomationActionResult | null>(null);
  const [mysteryShopPurchaseRecords, setMysteryShopPurchaseRecords] = useState<MysteryShopPurchaseRecord[]>([]);
  const [runStatistics, setRunStatistics] = useState<WorkspaceRunStatistics | null>(null);
  const [socialState, setSocialState] = useState<SocialState | null>(null);
  const [socialRankingPreferences, setSocialRankingPreferences] = useState<RankingPreferences>({
    stolenByMeViewMode: 'timeline',
    stolenFromMeViewMode: 'timeline',
  });
  const [socialRankingDataVersions, setSocialRankingDataVersions] = useState<RankingDataVersions>({
    stolenByMe: 0,
    visitors: 0,
  });
  const [socialDogGuardState, setSocialDogGuardState] = useState<DogGuardState | null>(null);
  const [socialProtocolBlockList, setSocialProtocolBlockList] = useState<SocialState['friends']>([]);
  const [socialActionResult, setSocialActionResult] = useState<SocialActionResult | null>(null);
  const [injectionHidden, setInjectionHidden] = useState(false);
  const [patching, setPatching] = useState(false);
  const [launching, setLaunching] = useState(false);
  const [restarting, setRestarting] = useState(false);
  const [guardToggling, setGuardToggling] = useState(false);
  const [targetPath, setTargetPath] = useState('');
  const [updateState, setUpdateState] = useState<UpdateStateDto | null>(null);
  const [dialogUpdate, setDialogUpdate] = useState<UpdateStateDto | null>(null);
  const [updateCheckPreferences, setUpdateCheckPreferences] = useState<UpdateCheckPreferencesDto>(defaultUpdateCheckPreferences);
  const activeTabRef = useRef<Tab>('workspace');
  const dashboardPollingRef = useRef<ReturnType<typeof createPollingController> | null>(null);
  const autoPatchCheckedKey = useRef('');
  const runtimeHashCheckedKey = useRef('');
  const revealedPatchMismatchKey = useRef('');
  const updateCheckRequestRef = useRef<(() => Promise<UpdateStateDto>) | null>(null);
  const updateChecksActiveRef = useRef(true);
  const lastIdentityProbeKey = useRef('');
  const lastRuntimeTargetRef = useRef<string | null>(null);
  const confirmedRuntimeAccountRef = useRef<RuntimeAccountIdentity | null>(null);
  const confirmedAccountScope = runtimeAccount?.confirmed && runtimeAccount.accountKey ? runtimeAccount.accountKey : 'default';
  const accountScopeGenerationRef = useRef<ReturnType<typeof createAccountScopeGeneration> | null>(null);
  if (!accountScopeGenerationRef.current) {
    accountScopeGenerationRef.current = createAccountScopeGeneration(confirmedAccountScope);
  }
  const accountScopeRenderKey = accountScopeGenerationRef.current.renderKey();
  const socialRankingPreferenceSaveQueueRef = useRef<ScopedLatestSaveQueue<RankingPreferences> | null>(null);
  const desktopControlsAvailable = isDesktopOnlyControlAvailable(remote);
  const dismissAutomationActionResult = useCallback(() => setAutomationActionResult(null), []);
  const dismissSocialActionResult = useCallback(() => setSocialActionResult(null), []);

  const changeActiveTab = useCallback((nextTab: Tab) => {
    activeTabRef.current = nextTab;
    setActiveTab(nextTab);
    if (nextTab !== 'automation') dismissAutomationActionResult();
    if (nextTab !== 'social') dismissSocialActionResult();
  }, [dismissAutomationActionResult, dismissSocialActionResult]);

  function createSocialRankingPreferenceSaveQueue() {
    return createScopedLatestSaveQueue<RankingPreferences>({
      getCurrentScope: () => accountScopeGenerationRef.current!.currentScope(),
      persist: async (scope, preferences) => (
        await SaveFarmSocialRankingPreferences(scope, preferences as any)
      ) as RankingPreferences,
      onCommitted: (scope, preferences) => {
        if (accountScopeGenerationRef.current!.currentScope() !== scope) return;
        setSocialRankingPreferences(preferences);
      },
    });
  }

  useEffect(() => () => {
    socialRankingPreferenceSaveQueueRef.current?.dispose();
    socialRankingPreferenceSaveQueueRef.current = null;
  }, []);

  if (!updateCheckRequestRef.current) {
    updateCheckRequestRef.current = createSingleFlightUpdateCheck(() => CheckForUpdates() as Promise<UpdateStateDto>);
  }
  const runUpdateCheck = useCallback(() => updateCheckRequestRef.current!(), []);

  const checkForUpdates = useCallback(async () => {
    const next = await runUpdateCheck();
    if (updateChecksActiveRef.current) setUpdateState(next);
    return next;
  }, [runUpdateCheck]);

  const checkForUpdatesAutomatically = useCallback(async () => {
    try {
      const next = await runUpdateCheck();
      if (updateChecksActiveRef.current && !next.errorCode) setUpdateState(next);
    } catch {
      // Automatic checks must not interrupt the authorized workspace.
    }
  }, [runUpdateCheck]);

  useEffect(() => {
    if (!desktopControlsAvailable) {
      updateChecksActiveRef.current = false;
      return;
    }
    updateChecksActiveRef.current = true;
    void checkForUpdatesAutomatically();
    return () => {
      updateChecksActiveRef.current = false;
    };
  }, [checkForUpdatesAutomatically, desktopControlsAvailable]);

  useEffect(() => {
    if (!desktopControlsAvailable) return;
    let active = true;
    void UpdateCheckPreferences()
      .then((value) => {
        if (active) setUpdateCheckPreferences(normalizeUpdateCheckPreferences(value));
      })
      .catch(() => {
        if (active) setUpdateCheckPreferences(defaultUpdateCheckPreferences);
      });
    return () => {
      active = false;
    };
  }, [desktopControlsAvailable]);

  useEffect(() => {
    if (!desktopControlsAvailable) return;
    if (!updateCheckPreferences.enabled) return;
    return startRecurringUpdateChecks({
      intervalMinutes: updateCheckPreferences.intervalMinutes,
      check: checkForUpdatesAutomatically,
    });
  }, [checkForUpdatesAutomatically, desktopControlsAvailable, updateCheckPreferences]);

  const saveUpdateCheckPreferences = useCallback(async (input: UpdateCheckPreferencesDto) => {
    const saved = normalizeUpdateCheckPreferences(await SaveUpdateCheckPreferences(input as any));
    setUpdateCheckPreferences(saved);
    return saved;
  }, []);

  const refreshAutomation = useCallback(() => {
    const token = accountScopeGenerationRef.current!.capture();
    FarmAutomationState()
      .then((value) => {
        if (accountScopeGenerationRef.current!.isCurrent(token)) {
          setAutomationState(value as FarmAutomationViewState);
        }
      })
      .catch((error) => {
        if (accountScopeGenerationRef.current!.isCurrent(token)) console.error(error);
      });
  }, []);

  const refreshMysteryShopPurchaseRecords = useCallback(() => {
    const token = accountScopeGenerationRef.current!.capture();
    FarmMysteryShopPurchaseRecords()
      .then((value) => {
        if (accountScopeGenerationRef.current!.isCurrent(token)) {
          setMysteryShopPurchaseRecords(Array.isArray(value) ? value as MysteryShopPurchaseRecord[] : []);
        }
      })
      .catch((error) => {
        if (accountScopeGenerationRef.current!.isCurrent(token)) console.error(error);
      });
  }, []);

  const refreshSocial = useCallback((refresh = false) => {
    const token = accountScopeGenerationRef.current!.capture();
    FarmSocialState(refresh)
      .then((value) => {
        if (accountScopeGenerationRef.current!.isCurrent(token)) setSocialState(value as SocialState);
      })
      .catch((error) => {
        if (accountScopeGenerationRef.current!.isCurrent(token)) console.error(error);
      });
  }, []);

  const loadSocialRankingPage = useCallback(async (request: RankingPageRequest) => {
    const token = accountScopeGenerationRef.current!.capture();
    const value = await FarmSocialRankings(request as any);
    if (!accountScopeGenerationRef.current!.isCurrent(token)) {
      throw new Error('账号已切换，排行榜响应已失效。');
    }
    return value as RankingPage;
  }, []);

  const refreshSocialVisitors = async () => {
    const token = accountScopeGenerationRef.current!.capture();
    const result = (await FarmSocialRefreshVisitors()) as SocialActionResult;
    if (!accountScopeGenerationRef.current!.isCurrent(token)) {
      throw new Error('账号已切换，访客刷新响应已失效。');
    }
    return result;
  };

  const refreshSocialRankingPreferences = useCallback(() => {
    const token = accountScopeGenerationRef.current!.capture();
    FarmSocialRankingPreferences()
      .then((preferences) => {
        if (!accountScopeGenerationRef.current!.isCurrent(token)) return;
        setSocialRankingPreferences(preferences as RankingPreferences);
      })
      .catch((error) => {
        if (accountScopeGenerationRef.current!.isCurrent(token)) console.error(error);
      });
  }, []);

  const refreshSocialDogGuard = useCallback(() => {
    const token = accountScopeGenerationRef.current!.capture();
    FarmSocialDogGuardState()
      .then((value) => {
        if (accountScopeGenerationRef.current!.isCurrent(token)) setSocialDogGuardState(value as DogGuardState);
      })
      .catch((error) => {
        if (accountScopeGenerationRef.current!.isCurrent(token)) console.error(error);
      });
  }, []);

  const refreshSocialProtocolBlockList = useCallback(() => {
    const token = accountScopeGenerationRef.current!.capture();
    FarmSocialProtocolBlockList(false)
      .then((value: any) => {
        if (accountScopeGenerationRef.current!.isCurrent(token)) {
          setSocialProtocolBlockList(Array.isArray(value?.friends) ? value.friends : []);
        }
      })
      .catch((error) => {
        if (accountScopeGenerationRef.current!.isCurrent(token)) console.error(error);
      });
  }, []);

  const refreshStatus = useCallback(() => {
    dashboardPollingRef.current?.refresh();
  }, []);

  useEffect(() => {
    const polling = createPollingController({
      intervalMs: 2500,
      read: async (signal) => ({
        token: accountScopeGenerationRef.current!.capture(),
        value: await pollDashboard(activeTab, signal),
      }),
      apply: ({ token, value }) => {
        if (!accountScopeGenerationRef.current!.isCurrent(token)) return;
        setStatus(value.status);
        setGuardStatus(normalizeGuardianStatus(value.guardStatus));
        setBindingStatus(value.bindingStatus);
        setEvents(value.events);
        setTsdkEvents(value.tsdkEvents);
        setAutomationState(value.automationState);
        if (value.runStatistics) setRunStatistics(value.runStatistics);
        setPatchStatus(value.patchStatus);
      },
      reportError: console.error,
      shouldStop: (error) => error instanceof PollApiError && error.status === 401,
    });
    dashboardPollingRef.current = polling;
    polling.start();
    return () => {
      if (dashboardPollingRef.current === polling) dashboardPollingRef.current = null;
      polling.stop();
    };
  }, [activeTab]);

  useEffect(() => {
    if (activeTab === 'automation') refreshMysteryShopPurchaseRecords();
  }, [activeTab, refreshMysteryShopPurchaseRecords]);

  useEffect(() => {
    const policy = socialRefreshPolicyForTab(activeTab);
    if (!policy.refreshState) return;
    refreshSocial(false);
    if (policy.refreshRankingPreferences) refreshSocialRankingPreferences();
    if (policy.refreshDogGuard && !policy.pollDogGuard) refreshSocialDogGuard();
    if (!policy.pollDogGuard) return;
    const polling = createPollingController({
      intervalMs: 1800,
      read: async (signal) => ({
        token: accountScopeGenerationRef.current!.capture(),
        value: await pollDogGuard(signal),
      }),
      apply: ({ token, value }) => {
        if (accountScopeGenerationRef.current!.isCurrent(token)) setSocialDogGuardState(value);
      },
      reportError: console.error,
      shouldStop: (error) => error instanceof PollApiError && error.status === 401,
    });
    polling.start();
    return () => polling.stop();
  }, [activeTab, refreshSocial, refreshSocialDogGuard, refreshSocialRankingPreferences]);

  useEffect(() => {
    setInjectionHidden(false);
  }, [status.target]);

  useEffect(() => {
    const nextTarget = status.target || '';
    const previousTarget = lastRuntimeTargetRef.current;
    lastRuntimeTargetRef.current = nextTarget;
    // Skip first mount and the first non-empty status hydration so restored
    // confirmed accounts are not wiped before the user switches chains.
    if (previousTarget === null || previousTarget === '' || nextTarget === '' || previousTarget === nextTarget) {
      return;
    }

    // Runtime chain switches invalidate the previous session GID binding.
    clearConfirmedRuntimeAccount();
  }, [status.target]);

  useEffect(() => {
    if (!desktopControlsAvailable) return;
    const decision = runtimePatchCheckDecision(autoPatchCheckedKey.current, status, patchStatus?.scriptHash || '');
    if (decision.nextCheckedKey === '') {
      if (status.target !== 'qq_ws') {
        autoPatchCheckedKey.current = '';
        runtimeHashCheckedKey.current = '';
        revealedPatchMismatchKey.current = '';
        setRuntimeScriptHash('');
      }
      return;
    }
    if (!decision.shouldRun || patching || patchStatus?.inFlight) return;
    autoPatchCheckedKey.current = decision.nextCheckedKey;

    let active = true;
    async function checkQQRuntimePatch() {
      try {
        const patchResult = (await InstallQQDebugPatch('')) as QQPatchStatusLike;
        if (!active) return;
        setPatchStatus(patchResult);
        autoPatchCheckedKey.current = runtimePatchCheckKey(status, patchResult.scriptHash);
      } catch (error) {
        console.error(error);
      }
    }

    void checkQQRuntimePatch();
    return () => {
      active = false;
    };
  }, [
    patchStatus?.inFlight,
    patchStatus?.scriptHash,
    patching,
    desktopControlsAvailable,
    status.connected,
    status.instanceId,
    status.ready,
    status.target,
  ]);

  useEffect(() => {
    if (!desktopControlsAvailable) return;
    const hashCheckKey = runtimeHashCheckKey(status, patchStatus?.scriptHash || '');
    if (hashCheckKey === '') {
      if (status.target !== 'qq_ws') {
        runtimeHashCheckedKey.current = '';
        revealedPatchMismatchKey.current = '';
        setRuntimeScriptHash('');
      }
      return;
    }
    if (runtimeHashCheckedKey.current === hashCheckKey) return;
    runtimeHashCheckedKey.current = hashCheckKey;

    let active = true;
    async function checkQQRuntimeHash() {
      try {
        const describe = await RunDiagnostic('host.describe', { args: [] });
        if (!active) return;
        const runningHash = scriptHashFromDiagnostic(describe);
        setRuntimeScriptHash(runningHash);

        const mismatch = runtimePatchMismatchDecision(
          revealedPatchMismatchKey.current,
          runningHash,
          patchStatus?.scriptHash || '',
        );
        if (mismatch.shouldReveal) {
          revealedPatchMismatchKey.current = mismatch.nextMismatchKey;
          setInjectionHidden(false);
        } else if (mismatch.shouldClear) {
          revealedPatchMismatchKey.current = '';
        }
      } catch (error) {
        console.error(error);
      }
    }

    void checkQQRuntimeHash();
    return () => {
      active = false;
    };
  }, [
    desktopControlsAvailable,
    patchStatus?.scriptHash,
    status.connected,
    status.instanceId,
    status.ready,
    status.target,
  ]);

  const injectionStatus = { ...status, scriptHash: runtimeScriptHash };
  const injectionState = normalizeStartupInjectionState(injectionStatus, patchStatus);
  const injectionOpen = shouldDisplayStartupInjectionDialog(injectionState, injectionHidden);

  const probeRuntimeAccount = useCallback(
    async (force = false) => {
      if (accountIdentifying || status.ready !== true || injectionOpen) return;
      const key = `${status.target || 'runtime'}:${status.instanceId || 'ready'}`;
      if (!force && lastIdentityProbeKey.current === key && runtimeAccount) return;
      lastIdentityProbeKey.current = key;
      setAccountIdentifying(true);
      setAccountError('');
      try {
        const account = (await IdentifyRuntimeAccount()) as RuntimeAccountIdentity;
        setRuntimeAccount(account);
        setAccountError(account.error || '');
      } catch (error) {
        setAccountError(error instanceof Error ? error.message : String(error));
      } finally {
        setAccountIdentifying(false);
      }
    },
    [accountIdentifying, injectionOpen, runtimeAccount, status.instanceId, status.ready, status.target],
  );

  useEffect(() => {
    const token = accountScopeGenerationRef.current!.capture();
    CurrentRuntimeAccount()
      .then((account) => {
        const next = account as RuntimeAccountIdentity;
        if (!accountScopeGenerationRef.current!.isCurrent(token) || !next.confirmed) return;
        accountScopeGenerationRef.current!.setScope(next.accountKey || 'default');
        confirmedRuntimeAccountRef.current = next;
        setRuntimeAccount(next);
      })
      .catch(console.error);
  }, []);

  useEffect(() => {
    if (status.ready !== true) {
      lastIdentityProbeKey.current = '';
      setRuntimeAccount((current) => (current?.confirmed ? current : null));
      setAccountError('');
      return;
    }
    if (runtimeAccount?.confirmed || injectionOpen) return;
    probeRuntimeAccount(false);
  }, [injectionOpen, probeRuntimeAccount, runtimeAccount?.confirmed, status.ready]);

  async function retryQQPatch() {
    setPatching(true);
    try {
      await InstallQQDebugPatch(targetPath.trim());
      setPatchStatus((await QQDebugPatchStatus()) as QQPatchStatusLike);
      refreshStatus();
    } catch (error) {
      setPatchStatus({
        phase: 'error',
        injected: false,
        inFlight: false,
        error: error instanceof Error ? error.message : String(error),
      });
    } finally {
      setPatching(false);
    }
  }

  async function launchHost() {
    setLaunching(true);
    try {
      await LaunchHostProcess();
      refreshStatus();
    } finally {
      setLaunching(false);
    }
  }

  async function restartHost() {
    setRestarting(true);
    try {
      await RestartHostProcess();
      refreshStatus();
      setPatchStatus((await QQDebugPatchStatus()) as QQPatchStatusLike);
    } finally {
      setRestarting(false);
    }
  }

  function clearConfirmedRuntimeAccount() {
    lastIdentityProbeKey.current = '';
    confirmedRuntimeAccountRef.current = null;
    accountScopeGenerationRef.current!.setScope('default');
    setRuntimeAccount(null);
    setAccountError('');
    dismissAutomationActionResult();
    dismissSocialActionResult();
    setSocialState(null);
    setSocialRankingPreferences({ stolenByMeViewMode: 'timeline', stolenFromMeViewMode: 'timeline' });
    setSocialRankingDataVersions({ stolenByMe: 0, visitors: 0 });
    setSocialDogGuardState(null);
    setSocialProtocolBlockList([]);
    setAutomationState(null);
    setMysteryShopPurchaseRecords([]);
    setRunStatistics(null);
    setEvents([]);
  }

  async function handleRuntimeSwitched() {
    clearConfirmedRuntimeAccount();
    refreshStatus();
  }

  async function confirmRuntimeAccount() {
    if (!runtimeAccount?.gid) return;
    setAccountConfirming(true);
    setAccountError('');
    try {
      const confirmed = (await ConfirmRuntimeAccount(runtimeAccount as any)) as RuntimeAccountIdentity;
      const resolution = resolveAccountConfirmation(confirmedRuntimeAccountRef.current, confirmed);
      setAccountError(confirmed.error || '');
      if (resolution.accepted) {
        accountScopeGenerationRef.current!.setScope(confirmed.accountKey!);
        confirmedRuntimeAccountRef.current = confirmed;
        setRuntimeAccount(confirmed);
        dismissAutomationActionResult();
        dismissSocialActionResult();
        setSocialState(null);
        setSocialRankingPreferences({ stolenByMeViewMode: 'timeline', stolenFromMeViewMode: 'timeline' });
        setSocialRankingDataVersions({ stolenByMe: 0, visitors: 0 });
        setSocialDogGuardState(null);
        setSocialProtocolBlockList([]);
        setAutomationState(null);
        setMysteryShopPurchaseRecords([]);
        setRunStatistics(null);
        setEvents([]);
        refreshStatus();
        refreshMysteryShopPurchaseRecords();
        refreshSocial(false);
        refreshSocialRankingPreferences();
        refreshSocialDogGuard();
      } else {
        setRuntimeAccount(resolution.account);
      }
    } catch (error) {
      setAccountError(error instanceof Error ? error.message : String(error));
    } finally {
      setAccountConfirming(false);
    }
  }

  async function toggleProcessGuard(enabled: boolean) {
    setGuardToggling(true);
    setGuardStatus((current) => normalizeGuardStatus({
      ...current,
      enabled,
      armed: enabled,
      phase: enabled ? current.phase === 'disabled' ? 'standby' : current.phase : 'disabled',
    }));
    try {
      await SaveGuardianSettings({ enabled });
      refreshStatus();
    } catch (error) {
      console.error(error);
      refreshStatus();
    } finally {
      setGuardToggling(false);
    }
  }

  async function saveGuardianSettings(input: Record<string, unknown>) {
    setGuardToggling(true);
    try {
      await SaveGuardianSettings(input);
    } catch (error) {
      console.error(error);
    } finally {
      setGuardToggling(false);
      refreshStatus();
    }
  }

  async function runAutomationTask(taskId: string) {
    const token = accountScopeGenerationRef.current!.capture();
    try {
      const result = (await RunFarmAutomationTask(taskId)) as FarmAutomationActionResult;
      if (accountScopeGenerationRef.current!.isCurrent(token)) {
        if (activeTabRef.current === 'automation') setAutomationActionResult(result);
        refreshAutomation();
        if (taskId === 'mystery_shop_auto_buy') refreshMysteryShopPurchaseRecords();
      }
    } catch (error) {
      if (!accountScopeGenerationRef.current!.isCurrent(token)) return;
      if (activeTabRef.current === 'automation') {
        setAutomationActionResult({
          ok: false,
          status: 'failed',
          taskId,
          message: error instanceof Error ? error.message : String(error),
        });
      }
    }
  }

  async function toggleAutomation(running: boolean) {
    if (running) {
      await StartFarmAutomationScheduler();
    } else {
      await StopFarmAutomationScheduler();
    }
    refreshStatus();
  }

  async function saveAutomationState(nextState: FarmAutomationViewState) {
    const token = accountScopeGenerationRef.current!.capture();
    const saved = (await SaveFarmAutomationState(nextState as any)) as FarmAutomationViewState;
    if (accountScopeGenerationRef.current!.isCurrent(token)) setAutomationState(saved);
    return saved;
  }

  async function setAutomationRunMode(runMode: 'safe' | 'god') {
    const token = accountScopeGenerationRef.current!.capture();
    const saved = (await SetFarmAutomationRunMode(runMode)) as FarmAutomationViewState;
    if (accountScopeGenerationRef.current!.isCurrent(token)) setAutomationState(saved);
    return saved;
  }

  async function runSocialAction(request: SocialActionRequest) {
    const token = accountScopeGenerationRef.current!.capture();
    try {
      const result = (await FarmSocialAction(request as any)) as SocialActionResult;
      if (!accountScopeGenerationRef.current!.isCurrent(token)) return result;
      if (activeTabRef.current === 'social') setSocialActionResult(result);
      setSocialRankingDataVersions((current) => rankingDataVersionsAfterSocialAction(current, request.action, result.ok));
      if (
        result.ok ||
        [
          'blacklist_toggle',
          'whitelist_toggle',
          'blacklist_clear',
          'whitelist_clear',
          'blacklist_cleanup',
          'cleanup_invalid_rules',
          'blacklist_remove_batch',
          'whitelist_remove_batch',
          'rules_save',
        ].includes(request.action)
      ) {
        refreshSocial(false);
      }
      return result;
    } catch (error) {
      const result = {
        ok: false,
        status: 'failed',
        message: error instanceof Error ? error.message : String(error),
      };
      if (accountScopeGenerationRef.current!.isCurrent(token) && activeTabRef.current === 'social') {
        setSocialActionResult(result);
      }
      return result;
    }
  }

  async function runSocialDogGuardAction(request: {
    action: string;
    refresh?: boolean;
    scanIntervalMs?: number;
    skipScanned?: boolean;
    excludeGuardDog?: boolean;
  }) {
    const token = accountScopeGenerationRef.current!.capture();
    const next = (await FarmSocialDogGuardAction(request as any)) as DogGuardState;
    if (shouldApplyScopedDogGuardResult(accountScopeGenerationRef.current!, token)) {
      setSocialDogGuardState(next);
      if (shouldRefreshSocialAfterDogGuardAction(request.action)) refreshSocial(false);
    }
    return next;
  }

  async function exportSocial(request: { groups: string[] }) {
    return (await FarmSocialExport(request as any)) as ImportExportPayload;
  }

  async function importSocial(payload: ImportExportPayload) {
    const token = accountScopeGenerationRef.current!.capture();
    const result = (await FarmSocialImport(payload as any)) as SocialActionResult;
    if (!accountScopeGenerationRef.current!.isCurrent(token)) return result;
    if (activeTabRef.current === 'social') setSocialActionResult(result);
    refreshSocial(false);
    setSocialRankingDataVersions((current) => rankingDataVersionsAfterImport(current, payload.groups || [], result.ok));
    refreshSocialDogGuard();
    return result;
  }

  async function saveSocialRankingPreferences(preferences: RankingPreferences) {
    if (!socialRankingPreferenceSaveQueueRef.current) {
      socialRankingPreferenceSaveQueueRef.current = createSocialRankingPreferenceSaveQueue();
    }
    return socialRankingPreferenceSaveQueueRef.current!.save(
      accountScopeGenerationRef.current!.currentScope(),
      preferences,
    );
  }

  async function saveTextFile(request: SaveTextFileRequest) {
    return (await SaveTextFile(request as any)) as SaveTextFileResult;
  }

  async function exportRuntimeLogs(kind: 'task' | 'system') {
    return (await ExportRuntimeLogs(kind)) as SaveTextFileResult;
  }

  async function refreshBackpackSeedOptions(config: Record<string, any>) {
    return FarmBackpackSeedOptions({
      selectedSeedIds: config.autoFarmPlantBackpackSeedPriority || [],
      disabledSeedIds: config.autoFarmPlantBackpackSeedDisabled || [],
      forcePriority: !!config.autoFarmPlantBackpackForcePriority,
      fourGridPlantEnabled: !!config.autoFarmFourGridPlantEnabled,
      refresh: !!config.refresh,
    });
  }

  async function refreshStealCropOptions() {
    return FarmStealCropOptions();
  }

  const accountGateOpen = status.ready === true && !runtimeAccount?.confirmed && !injectionOpen;

  const openUpdateDetails = useCallback((update: UpdateStateDto | null = updateState) => {
    if (update?.available) setDialogUpdate(update);
  }, [updateState]);

  const openUpdateDownload = useCallback(() => {
    const downloadURL = getValidatedUpdateDownloadURL(dialogUpdate);
    if (downloadURL) void BrowserOpenURL(downloadURL);
  }, [dialogUpdate]);

  return (
    <AppShell
      activeTab={activeTab}
      remote={remote}
      account={runtimeAccount}
      update={desktopControlsAvailable ? updateState : null}
      onOpenUpdateDetails={() => openUpdateDetails()}
      onTabChange={changeActiveTab}
    >
      <div className="app-view-enter" key={activeTab}>
        {isWorkspaceArea(activeTab) && (
          <FarmWorkspaceView
            key={accountScopeRenderKey}
            area={activeTab}
            remote={remote}
            status={status}
            guardStatus={guardStatus}
            bindingStatus={bindingStatus}
            events={events}
            tsdkEvents={tsdkEvents}
            runStatistics={runStatistics}
            automationState={automationState}
            automationActionResult={automationActionResult}
            mysteryShopPurchaseRecords={mysteryShopPurchaseRecords}
            onAutomationActionResultConsumed={dismissAutomationActionResult}
            socialState={socialState}
            socialRankingPreferences={socialRankingPreferences}
            socialRankingDataVersions={socialRankingDataVersions}
            socialDogGuardState={socialDogGuardState}
            socialProtocolBlockList={socialProtocolBlockList}
            socialActionResult={socialActionResult}
            onSocialActionResultConsumed={dismissSocialActionResult}
            onRunAutomationTask={runAutomationTask}
            onToggleAutomation={toggleAutomation}
            onSaveAutomationState={saveAutomationState}
            onSetAutomationRunMode={setAutomationRunMode}
            onRefreshBackpackSeeds={refreshBackpackSeedOptions}
            onRefreshStealCropOptions={refreshStealCropOptions}
            onRefreshSocial={refreshSocial}
            onSocialAction={runSocialAction}
            onSocialRankings={loadSocialRankingPage}
            onRefreshSocialVisitors={refreshSocialVisitors}
            onSaveSocialRankingPreferences={saveSocialRankingPreferences}
            onSocialDogGuardAction={runSocialDogGuardAction}
            onSocialProtocolBlockList={refreshSocialProtocolBlockList}
            onSocialExport={exportSocial}
            onSocialImport={importSocial}
            onSaveTextFile={saveTextFile}
            onRefresh={refreshStatus}
            onLaunch={launchHost}
            onRestart={restartHost}
            onToggleGuard={toggleProcessGuard}
            toggling={guardToggling}
          />
        )}
        {activeTab === 'account' && <AccountStatusView status={status} />}
        {!remote && activeTab === 'guard' && (
          <GuardView
            status={guardStatus}
            events={events}
            bindingStatus={bindingStatus}
            onRefresh={refreshStatus}
            onLaunch={launchHost}
            onRestart={restartHost}
            onToggleEnabled={toggleProcessGuard}
            onSaveSettings={saveGuardianSettings}
            toggling={guardToggling}
          />
        )}
        {!remote && activeTab === 'logs' && <LogsView events={events} onRefresh={refreshStatus} onExportLogs={exportRuntimeLogs} />}
        {!remote && activeTab === 'message_push' && <MessagePushView key={accountScopeRenderKey} />}
        {!remote && activeTab === 'settings' && (
          <SettingsView
            events={events}
            onRefreshEvents={refreshStatus}
            updateState={updateState}
            updateCheckPreferences={updateCheckPreferences}
            onCheckUpdates={checkForUpdates}
            onSaveUpdateCheckPreferences={saveUpdateCheckPreferences}
            onUpdateAvailable={openUpdateDetails}
            onRuntimeSwitched={handleRuntimeSwitched}
          />
        )}
      </div>
      {desktopControlsAvailable && <StartupInjectionDialog
        open={injectionOpen}
        state={injectionState}
        targetPath={targetPath}
        patching={patching}
        launching={launching}
        restarting={restarting}
        onTargetPathChange={setTargetPath}
        onRetry={retryQQPatch}
        onLaunch={launchHost}
        onRestart={restartHost}
        onHide={() => setInjectionHidden(true)}
      />}
      {desktopControlsAvailable && <UpdateDetailsDialog
        open={Boolean(dialogUpdate)}
        update={dialogUpdate}
        onClose={() => setDialogUpdate(null)}
        onDownload={openUpdateDownload}
      />}
      <AccountIdentityDialog
        open={accountGateOpen}
        account={runtimeAccount}
        identifying={accountIdentifying}
        confirming={accountConfirming}
        error={accountError}
        onConfirm={confirmRuntimeAccount}
        onRetry={() => {
          lastIdentityProbeKey.current = '';
          setRuntimeAccount(null);
          void probeRuntimeAccount(true);
        }}
      />
    </AppShell>
  );
}

export default AuthorizedApp;

function isWorkspaceArea(tab: Tab): tab is Extract<Tab, 'workspace' | 'automation' | 'assets' | 'social'> {
  return tab === 'workspace' || tab === 'automation' || tab === 'assets' || tab === 'social';
}

function normalizeGuardStatus(value: Partial<GuardStatusDto>): GuardStatusDto {
  return {
    ...initialGuardStatus,
    ...value,
    recentRestartEvents: value.recentRestartEvents || [],
  };
}

function normalizeGuardianStatus(value: { process?: object } & object): GuardStatusDto {
  const record = value as Record<string, unknown>;
  const process = (record.process || {}) as Record<string, unknown>;
  return normalizeGuardStatus({
    ...record,
    enabled: record.enabled === true,
    armed: process.armed === true,
    timeoutStreak: Number(process.timeoutStreak || 0),
    threshold: Number(process.threshold || 3),
    lastTimeoutAt: typeof process.lastTimeoutAt === 'string' ? process.lastTimeoutAt : '',
    lastHealthyAt: typeof process.lastHealthyAt === 'string' ? process.lastHealthyAt : '',
    lastRestartAt: typeof process.lastRestartAt === 'string' ? process.lastRestartAt : '',
    reconnectGraceUntil: typeof process.reconnectGraceUntil === 'string' ? process.reconnectGraceUntil : '',
    lastReason: typeof process.lastReason === 'string' ? process.lastReason : '',
    lastActionError: typeof process.lastActionError === 'string' ? process.lastActionError : '',
    restartCountInWindow: Number(process.restartCountInWindow || 0),
    maxRestartsPerWindow: Number(process.maxRestartsPerWindow || 4),
    recentRestartEvents: Array.isArray(process.recentRestartEvents) ? process.recentRestartEvents as GuardStatusDto['recentRestartEvents'] : [],
  });
}
