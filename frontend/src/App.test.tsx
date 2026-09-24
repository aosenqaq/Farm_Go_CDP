import { renderToStaticMarkup } from 'react-dom/server';
// @ts-expect-error The frontend tsconfig intentionally omits Node types; this test reads component source only.
import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

import AuthorizedApp from './AuthorizedApp';
import App from './App';
import {
  rankingDataVersionsAfterImport,
  rankingDataVersionsAfterSocialAction,
  normalizeUpdateCheckPreferences,
  isDesktopOnlyControlAvailable,
  runtimeHashCheckKey,
  runtimePatchCheckDecision,
  runtimePatchMismatchDecision,
  shouldApplyScopedDogGuardResult,
  shouldRefreshSocialAfterDogGuardAction,
  socialRefreshPolicyForTab,
} from './AuthorizedApp';
import { createAccountScopeGeneration } from './lib/accountScope';

describe('App startup shell', () => {
  it('opens the workspace without a card gate', () => {
    const html = renderToStaticMarkup(<App />);

    expect(html).toContain('Farm Go');
    expect(html).not.toContain('卡密验证');
  });

  it('keeps runtime calls behind the authorized app boundary', () => {
    const source = readFileSync(new URL('./App.tsx', import.meta.url), 'utf8');

    expect(source).toContain('<AuthorizedApp />');
    expect(source).not.toContain('LicenseLogin');
    expect(source).not.toContain('RuntimeStatus');
  });

  it('mounts the authorized UI without waiting for an update check', () => {
    const html = renderToStaticMarkup(<AuthorizedApp />);

    expect(html).toContain('Farm Go');
    expect(html).toContain('工作台');
  });

  it('normalizes recurring update preferences to enabled every 120 minutes', () => {
    expect(normalizeUpdateCheckPreferences(undefined)).toEqual({ enabled: true, intervalMinutes: 120 });
    expect(normalizeUpdateCheckPreferences({ enabled: false, intervalMinutes: 45 })).toEqual({ enabled: false, intervalMinutes: 45 });
    expect(normalizeUpdateCheckPreferences({ enabled: true, intervalMinutes: 0 })).toEqual({ enabled: true, intervalMinutes: 120 });
    expect(normalizeUpdateCheckPreferences({ enabled: true, intervalMinutes: 10081 })).toEqual({ enabled: true, intervalMinutes: 120 });
  });

  it('owns recurring update checks at the authorized application root', () => {
    const source = readFileSync(new URL('./AuthorizedApp.tsx', import.meta.url), 'utf8');

    expect(source).toContain('UpdateCheckPreferences');
    expect(source).toContain('SaveUpdateCheckPreferences');
    expect(source).toContain('startRecurringUpdateChecks');
    expect(source).toContain('updateCheckRequestRef');
    expect(source).toContain('if (!updateCheckPreferences.enabled) return;');
    expect(source).toContain('updateCheckPreferences={updateCheckPreferences}');
    expect(source).toContain('onSaveUpdateCheckPreferences={saveUpdateCheckPreferences}');
  });

  it('uses fetch polling for recurring dashboard and dog guard reads', () => {
    const source = readFileSync(new URL('./AuthorizedApp.tsx', import.meta.url), 'utf8');

    expect(source).toContain('pollDashboard');
    expect(source).toContain('pollDogGuard');
    expect(source).toContain('createPollingController');
    expect(source).not.toMatch(/\b(?:RuntimeStatus|RuntimeEvents|GuardianStatus|AutoBindHostProcess)\(/);
    expect(source).not.toContain('window.setInterval(refreshStatus, 2500)');
    expect(source).not.toContain('window.setInterval(refreshSocialDogGuard, 1800)');
    expect(source).not.toContain('window.setInterval(refreshPatchStatus, 1500)');
    expect(source).not.toContain('window.setInterval(checkQQRuntimePatch, 10000)');
  });

  it('keeps global TSDK events in dedicated polling state across account resets', () => {
    const source = readFileSync(new URL('./AuthorizedApp.tsx', import.meta.url), 'utf8');

    expect(source).toContain('const [tsdkEvents, setTsdkEvents] = useState<RuntimeEventDto[]>([]);');
    expect(source).toContain('setTsdkEvents(value.tsdkEvents);');
    expect(source).toContain('tsdkEvents={tsdkEvents}');
    expect(source).not.toContain('setTsdkEvents([]);');
  });

  it('checks an automatic QQ patch once per patch generation, not per WS reconnect', () => {
    const firstConnection = { target: 'qq_ws', phase: 'ready', ready: true, connected: true, instanceId: 'one' } as const;
    const reconnected = { ...firstConnection, instanceId: 'two' } as const;
    const stopped = { target: 'qq_ws', phase: 'idle', ready: false, connected: false } as const;

    expect(runtimePatchCheckDecision('', firstConnection, 'farm-hash')).toEqual({
      nextCheckedKey: 'qq_ws:farm-hash',
      shouldRun: true,
    });
    expect(runtimePatchCheckDecision('qq_ws:farm-hash', reconnected, 'farm-hash')).toEqual({
      nextCheckedKey: 'qq_ws:farm-hash',
      shouldRun: false,
    });
    expect(runtimePatchCheckDecision('qq_ws:farm-hash', reconnected, 'new-farm-hash').shouldRun).toBe(true);
    expect(runtimePatchCheckDecision('qq_ws:farm-hash', stopped, 'farm-hash')).toEqual({
      nextCheckedKey: '',
      shouldRun: false,
    });
  });

  it('checks the runtime hash again for a reconnected QQ WS instance', () => {
    const first = { target: 'qq_ws', phase: 'ready', ready: true, connected: true, instanceId: 'one' } as const;
    const reconnected = { ...first, instanceId: 'two' } as const;

    expect(runtimeHashCheckKey(first, 'farm-hash')).toBe('qq_ws:one:farm-hash');
    expect(runtimeHashCheckKey(reconnected, 'farm-hash')).toBe('qq_ws:two:farm-hash');
  });

  it('reveals an unchanged runtime patch mismatch only once', () => {
    expect(runtimePatchMismatchDecision('', 'hq-hash', 'farm-hash')).toEqual({
      nextMismatchKey: 'hq-hash:farm-hash',
      shouldReveal: true,
      shouldClear: false,
    });
    expect(runtimePatchMismatchDecision('hq-hash:farm-hash', 'hq-hash', 'farm-hash')).toEqual({
      nextMismatchKey: 'hq-hash:farm-hash',
      shouldReveal: false,
      shouldClear: false,
    });
    expect(runtimePatchMismatchDecision('hq-hash:farm-hash', 'farm-hash', 'farm-hash')).toEqual({
      nextMismatchKey: '',
      shouldReveal: false,
      shouldClear: true,
    });
  });

  it('keeps QQ patch installation stable while refreshing runtime hashes per connection', () => {
    const source = readFileSync(new URL('./AuthorizedApp.tsx', import.meta.url), 'utf8');

    expect(source).toContain("const runtimeHashCheckedKey = useRef('');");
    expect(source).toContain("const revealedPatchMismatchKey = useRef('');");
    expect(source).toContain("runtimePatchCheckDecision(autoPatchCheckedKey.current, status, patchStatus?.scriptHash || '')");
    expect(source).toContain("const hashCheckKey = runtimeHashCheckKey(status, patchStatus?.scriptHash || '');");
    expect(source).toContain('runtimePatchMismatchDecision(');
    expect(source).toContain('autoPatchCheckedKey.current = runtimePatchCheckKey(status, patchResult.scriptHash);');
    expect(source).not.toContain("autoPatchCheckedKey.current = '';\n      setRuntimeScriptHash('');");
  });

  it('excludes desktop process, patch, and update controls from the remote console', () => {
    const source = readFileSync(new URL('./AuthorizedApp.tsx', import.meta.url), 'utf8');

    expect(isDesktopOnlyControlAvailable(false)).toBe(true);
    expect(isDesktopOnlyControlAvailable(true)).toBe(false);
    expect(source).toContain('const desktopControlsAvailable = isDesktopOnlyControlAvailable(remote);');
    expect(source).toContain('if (!desktopControlsAvailable) return;');
    expect(source).toContain('{desktopControlsAvailable && <StartupInjectionDialog');
    expect(source).toContain('{desktopControlsAvailable && <UpdateDetailsDialog');
  });

  it('mounts close confirmation controls outside the authorization gate', () => {
    const source = readFileSync(new URL('./App.tsx', import.meta.url), 'utf8');

    expect(source).toContain('<CloseConfirmationController />');
  });

  it('does not preserve a transform on app views that contain fixed dialogs', () => {
    const css = readFileSync(new URL('./style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');

    expect(css).toMatch(/\.app-view-enter \{[^}]*height: 100%;[^}]*animation: app-view-enter 180ms ease-out;/);
    expect(css).not.toContain('animation: app-view-enter 180ms ease-out both;');
  });

  it('centers the desktop sidebar brand as one wordmark', () => {
    const css = readFileSync(new URL('./style.css', import.meta.url), 'utf8').replace(/\r\n/g, '\n');

    expect(css).toMatch(/\.brand-block \{[^}]*display: flex;[^}]*justify-content: center;/);
  });

    it('clears the confirmed account scope when the runtime target changes', () => {
    const source = readFileSync(new URL('./AuthorizedApp.tsx', import.meta.url), 'utf8');

    expect(source).toContain('const lastRuntimeTargetRef = useRef<string | null>(null);');
    expect(source).toContain('function clearConfirmedRuntimeAccount()');
    expect(source).toContain('async function handleRuntimeSwitched()');
    expect(source).toContain("accountScopeGenerationRef.current!.setScope('default')");
    expect(source).toContain('confirmedRuntimeAccountRef.current = null');
    expect(source).toContain('previousTarget === nextTarget');
    expect(source).toContain('onRuntimeSwitched={handleRuntimeSwitched}');
    expect(source).toContain('}, [status.target]);');
  });

it('remounts the farm workspace only after an account is confirmed', () => {
    const source = readFileSync(new URL('./AuthorizedApp.tsx', import.meta.url), 'utf8');

    expect(source).toContain('key={accountScopeRenderKey}');
    expect(source).toContain("const confirmedAccountScope = runtimeAccount?.confirmed && runtimeAccount.accountKey ? runtimeAccount.accountKey : 'default';");
    expect(source).not.toContain('accountScopeGenerationRef.current.setScope(confirmedAccountScope)');
    expect(source).not.toContain('key={runtimeAccount.accountKey}');
    expect(source).not.toContain('key={runtimeAccount?.accountKey}');
  });

  it('remounts message push with the same confirmed account scope generation', () => {
    const source = readFileSync(new URL('./AuthorizedApp.tsx', import.meta.url), 'utf8');

    expect(source).toContain('<MessagePushView key={accountScopeRenderKey} />');
  });

  it('passes the captured preference scope through the Wails save call', () => {
    const source = readFileSync(new URL('./AuthorizedApp.tsx', import.meta.url), 'utf8');

    expect(source).toContain('SaveFarmSocialRankingPreferences(scope, preferences as any)');
    expect(source).not.toContain('persist: async (_scope, preferences)');
  });

  it('returns executable social refresh policies for each tab class', () => {
    expect(socialRefreshPolicyForTab('automation')).toEqual({
      refreshState: true,
      refreshRankingPreferences: false,
      refreshDogGuard: false,
      pollDogGuard: false,
    });
    expect(socialRefreshPolicyForTab('social')).toEqual({
      refreshState: true,
      refreshRankingPreferences: true,
      refreshDogGuard: true,
      pollDogGuard: true,
    });
    expect(socialRefreshPolicyForTab('workspace')).toEqual({
      refreshState: false,
      refreshRankingPreferences: false,
      refreshDogGuard: false,
      pollDogGuard: false,
    });
  });

  it('loads ranking pages on demand without owning full ranking state or prefetching rows', () => {
    const source = readFileSync(new URL('./AuthorizedApp.tsx', import.meta.url), 'utf8');

    expect(source).not.toContain('const [socialRankings');
    expect(source).not.toContain('refreshSocialRankings');
    expect(source).toContain('const [socialRankingPreferences');
    expect(source).toContain('const loadSocialRankingPage');
    expect(source).toContain('FarmSocialRankings(request as any)');
    expect(source).toContain('onSocialRankings={loadSocialRankingPage}');
  });

  it('keeps explicit visitor refresh version-neutral so the dialog owns its single reload', () => {
    const source = readFileSync(new URL('./AuthorizedApp.tsx', import.meta.url), 'utf8');
    const handler = source.match(/const refreshSocialVisitors[\s\S]*?\n  };/)?.[0] || '';

    expect(handler).toContain('FarmSocialRefreshVisitors()');
    expect(handler).not.toContain('setSocialRankingDataVersions');
  });

  it('invalidates only stolen-by-me data after a successful steal action', () => {
    const current = { stolenByMe: 2, visitors: 5 };

    expect(rankingDataVersionsAfterSocialAction(current, 'steal', true)).toEqual({ stolenByMe: 3, visitors: 5 });
    expect(rankingDataVersionsAfterSocialAction(current, 'help', true)).toBe(current);
    expect(rankingDataVersionsAfterSocialAction(current, 'steal', false)).toBe(current);
  });

  it('invalidates imported ranking groups independently after a successful import', () => {
    const current = { stolenByMe: 2, visitors: 5 };

    expect(rankingDataVersionsAfterImport(current, ['steal_records'], true)).toEqual({ stolenByMe: 3, visitors: 5 });
    expect(rankingDataVersionsAfterImport(current, ['visitor_records'], true)).toEqual({ stolenByMe: 2, visitors: 6 });
    expect(rankingDataVersionsAfterImport(current, ['steal_records', 'visitor_records'], true)).toEqual({ stolenByMe: 3, visitors: 6 });
    expect(rankingDataVersionsAfterImport(current, ['steal_records'], false)).toBe(current);
  });

  it('refreshes social state immediately only after dog guard clear', () => {
    expect(shouldRefreshSocialAfterDogGuardAction('clear')).toBe(true);
    expect(shouldRefreshSocialAfterDogGuardAction('start')).toBe(false);
    expect(shouldRefreshSocialAfterDogGuardAction('stop')).toBe(false);
  });

  it('rejects a dog guard result captured for a stale account scope', () => {
    const scope = createAccountScopeGeneration('gid:10001');
    const token = scope.capture();

    expect(shouldApplyScopedDogGuardResult(scope, token)).toBe(true);
    scope.setScope('gid:10002');
    expect(shouldApplyScopedDogGuardResult(scope, token)).toBe(false);
  });
});
