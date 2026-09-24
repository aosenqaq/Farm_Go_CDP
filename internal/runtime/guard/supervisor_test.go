package guard

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type guardianCall struct {
	method string
	args   []any
}

type fakeGuardianCaller struct {
	mu                 sync.Mutex
	calls              []guardianCall
	errors             []error
	call               func(context.Context, string, []any) (any, error)
	started            chan guardianCall
	release            chan struct{}
	ignoreCancellation bool
}

func (c *fakeGuardianCaller) Call(ctx context.Context, method string, args []any, _ time.Duration) (any, error) {
	call := guardianCall{method: method, args: append([]any(nil), args...)}
	c.mu.Lock()
	c.calls = append(c.calls, call)
	index := len(c.calls) - 1
	var err error
	if index < len(c.errors) {
		err = c.errors[index]
	}
	started := c.started
	release := c.release
	callFn := c.call
	ignoreCancellation := c.ignoreCancellation
	c.mu.Unlock()
	if started != nil {
		started <- call
	}
	if release != nil {
		if ignoreCancellation {
			<-release
		} else {
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	if callFn != nil {
		return callFn(ctx, method, args)
	}
	return nil, err
}

func (c *fakeGuardianCaller) snapshot() []guardianCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]guardianCall, len(c.calls))
	copy(result, c.calls)
	return result
}

func countGuardianCalls(calls []guardianCall, method string) int {
	count := 0
	for _, call := range calls {
		if call.method == method {
			count++
		}
	}
	return count
}

type supervisorAfterCall struct {
	delay time.Duration
	ch    chan time.Time
}

type supervisorTestAfter struct {
	mu    sync.Mutex
	calls []*supervisorAfterCall
}

func (a *supervisorTestAfter) After(delay time.Duration) <-chan time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	call := &supervisorAfterCall{delay: delay, ch: make(chan time.Time, 1)}
	a.calls = append(a.calls, call)
	return call.ch
}

func (a *supervisorTestAfter) waitForCall(t *testing.T, index int, delay time.Duration) {
	t.Helper()
	waitFor(t, func() bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		return len(a.calls) > index && a.calls[index].delay == delay
	})
}

func (a *supervisorTestAfter) fire(index int) {
	a.mu.Lock()
	call := a.calls[index]
	a.mu.Unlock()
	call.ch <- time.Now()
}

func (a *supervisorTestAfter) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.calls)
}

func supervisorEnabledSettings() Settings {
	settings := DefaultSettings()
	settings.Enabled = true
	settings.OtherPlaceLoginEnabled = true
	return settings
}

func readyQQSnapshot() RuntimeSnapshot {
	return RuntimeSnapshot{RuntimeTarget: "qq_ws", Connected: true, Ready: true}
}

func TestSupervisorResynchronizesWorkersWheneverRuntimeBecomesReady(t *testing.T) {
	caller := &fakeGuardianCaller{}
	settings := supervisorEnabledSettings()
	settings.NetworkReconnectIntervalMS = 1300
	settings.NetworkRecoveryTimeoutMS = 18000
	settings.OtherPlaceLoginIntervalMS = 4500
	settings.OtherPlaceLoginDelayMin = 8
	s := NewSupervisor(SupervisorOptions{Caller: caller, Settings: settings})
	s.Start(context.Background())
	defer s.Close()

	s.OnRuntimeStatus(readyQQSnapshot())
	waitFor(t, func() bool { return len(caller.snapshot()) == 2 })

	want := []guardianCall{
		{method: "gameCtl.setReconnectWatcherEnabled", args: []any{true, map[string]any{"intervalMs": 1300, "recoverTimeoutMs": 18000}}},
		{method: "gameCtl.setOtherPlaceLoginReconnectEnabled", args: []any{true, map[string]any{"intervalMs": 4500, "reconnectDelayMs": 480000}}},
	}
	if got := caller.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("worker sync calls = %#v, want %#v", got, want)
	}

	firstGeneration := s.Generation()
	s.OnRuntimeStatus(readyQQSnapshot())
	waitFor(t, func() bool { return len(caller.snapshot()) == 4 })
	if s.Generation() <= firstGeneration {
		t.Fatalf("generation did not advance on repeated ready: before=%d after=%d", firstGeneration, s.Generation())
	}
}

func TestSupervisorUpdateSettingsDisablesMasterAndChildWorkers(t *testing.T) {
	caller := &fakeGuardianCaller{}
	s := NewSupervisor(SupervisorOptions{Caller: caller, Settings: supervisorEnabledSettings()})
	s.Start(context.Background())
	defer s.Close()
	s.OnRuntimeStatus(readyQQSnapshot())
	waitFor(t, func() bool { return len(caller.snapshot()) == 2 })

	settings := supervisorEnabledSettings()
	settings.NetworkReconnectEnabled = false
	s.UpdateSettings(settings)
	waitFor(t, func() bool { return len(caller.snapshot()) == 4 })
	childDisabled := caller.snapshot()[2:]
	if childDisabled[0].args[0] != false || childDisabled[1].args[0] != true {
		t.Fatalf("child switch sync = %#v", childDisabled)
	}

	settings.Enabled = false
	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "waiting"})
	s.HandleRuntimeEvent(RuntimeEvent{Name: "other_place_login_reconnect", Phase: "waiting"})
	s.UpdateSettings(settings)
	waitFor(t, func() bool { return len(caller.snapshot()) == 6 })
	masterDisabled := caller.snapshot()[4:]
	if masterDisabled[0].args[0] != false || masterDisabled[1].args[0] != false {
		t.Fatalf("master switch sync = %#v", masterDisabled)
	}
	status := s.Status()
	if status.Enabled || status.Network.Enabled || status.OtherPlaceLogin.Enabled || status.Phase != PhaseDisabled ||
		status.Network.Running || status.Network.Busy || status.OtherPlaceLogin.Running || status.OtherPlaceLogin.Busy {
		t.Fatalf("disabled status = %#v", status)
	}
}

func TestSupervisorRetriesOnlyReadinessErrorsWithStrictSchedule(t *testing.T) {
	after := &supervisorTestAfter{}
	var mu sync.Mutex
	networkAttempts := 0
	caller := &fakeGuardianCaller{call: func(_ context.Context, method string, _ []any) (any, error) {
		if method != "gameCtl.setReconnectWatcherEnabled" {
			return nil, nil
		}
		mu.Lock()
		defer mu.Unlock()
		networkAttempts++
		if networkAttempts <= 4 {
			return nil, []error{
				errors.New("execution context is not ready"),
				errors.New("gameCtl_not_ready"),
				errors.New("runtime evaluator is not connected"),
				errors.New("missing context"),
			}[networkAttempts-1]
		}
		return nil, nil
	}}
	s := NewSupervisor(SupervisorOptions{Caller: caller, Settings: supervisorEnabledSettings(), After: after.After})
	s.Start(context.Background())
	defer s.Close()
	s.OnRuntimeStatus(readyQQSnapshot())

	delays := []time.Duration{800 * time.Millisecond, 1600 * time.Millisecond, 3200 * time.Millisecond, 5 * time.Second}
	for index, delay := range delays {
		after.waitForCall(t, index, delay)
		calls := caller.snapshot()
		if got := countGuardianCalls(calls, "gameCtl.setReconnectWatcherEnabled"); got != index+1 {
			t.Fatalf("network calls before retry %d = %d", index, got)
		}
		if got := countGuardianCalls(calls, "gameCtl.setOtherPlaceLoginReconnectEnabled"); got != 1 {
			t.Fatalf("successful other-place worker repeated %d times before retry %d", got, index)
		}
		after.fire(index)
	}
	waitFor(t, func() bool { return len(caller.snapshot()) == 6 })
	if after.count() != len(delays) {
		t.Fatalf("retry timer count = %d, want %d", after.count(), len(delays))
	}
}

func TestSupervisorSyncsOtherPlaceWhenNetworkFailsFatally(t *testing.T) {
	after := &supervisorTestAfter{}
	caller := &fakeGuardianCaller{call: func(_ context.Context, method string, _ []any) (any, error) {
		if method == "gameCtl.setReconnectWatcherEnabled" {
			return nil, errors.New("permission denied")
		}
		return nil, nil
	}}
	s := NewSupervisor(SupervisorOptions{Caller: caller, Settings: supervisorEnabledSettings(), After: after.After})
	s.Start(context.Background())
	defer s.Close()
	s.OnRuntimeStatus(readyQQSnapshot())
	waitFor(t, func() bool { return len(caller.snapshot()) == 2 })
	if after.count() != 0 {
		t.Fatalf("fatal network error scheduled %d retries", after.count())
	}
	status := s.Status()
	if status.Network.LastError != "permission denied" || !status.OtherPlaceLogin.Running {
		t.Fatalf("independent worker results = %#v", status)
	}
}

func TestSupervisorDoesNotRetryFatalSyncErrors(t *testing.T) {
	after := &supervisorTestAfter{}
	caller := &fakeGuardianCaller{errors: []error{errors.New("crop is not ready")}}
	s := NewSupervisor(SupervisorOptions{Caller: caller, Settings: supervisorEnabledSettings(), After: after.After})
	s.Start(context.Background())
	defer s.Close()
	s.OnRuntimeStatus(readyQQSnapshot())
	waitFor(t, func() bool { return len(caller.snapshot()) == 2 })
	if after.count() != 0 {
		t.Fatalf("fatal error scheduled %d retries", after.count())
	}
	waitFor(t, func() bool { return s.Status().Network.LastError == "crop is not ready" })
}

func TestSupervisorSettingsGenerationRejectsStaleSyncCompletion(t *testing.T) {
	release := make(chan struct{})
	caller := &fakeGuardianCaller{started: make(chan guardianCall, 4), release: release, ignoreCancellation: true}
	s := NewSupervisor(SupervisorOptions{Caller: caller, Settings: supervisorEnabledSettings()})
	s.Start(context.Background())
	defer s.Close()
	s.OnRuntimeStatus(readyQQSnapshot())
	receive(t, caller.started)
	oldGeneration := s.Generation()

	settings := supervisorEnabledSettings()
	settings.NetworkReconnectEnabled = false
	s.UpdateSettings(settings)
	if s.IsCurrentGeneration(oldGeneration) {
		t.Fatal("old generation remained current after settings update")
	}
	close(release)
	waitFor(t, func() bool { return len(caller.snapshot()) >= 3 })
	waitFor(t, func() bool { return !s.Status().Network.Enabled })
	if got := caller.snapshot(); got[len(got)-2].args[0] != false {
		t.Fatalf("new generation did not disable network: %#v", got)
	}
}

func TestSupervisorCloseCancelsRetryAndIsIdempotent(t *testing.T) {
	after := &supervisorTestAfter{}
	caller := &fakeGuardianCaller{errors: []error{errors.New("execution context is not ready")}}
	s := NewSupervisor(SupervisorOptions{Caller: caller, Settings: supervisorEnabledSettings(), After: after.After})
	s.Start(context.Background())
	s.Start(context.Background())
	s.OnRuntimeStatus(readyQQSnapshot())
	after.waitForCall(t, 0, 800*time.Millisecond)
	s.Close()
	s.Close()
	after.fire(0)
	if got := len(caller.snapshot()); got != 2 {
		t.Fatalf("calls after close = %d, want 2", got)
	}
}

func TestSupervisorCloseCleansLifecycleAndRejectsOldCompletion(t *testing.T) {
	release := make(chan struct{})
	caller := &fakeGuardianCaller{
		started:            make(chan guardianCall, 2),
		release:            release,
		ignoreCancellation: true,
	}
	s := NewSupervisor(SupervisorOptions{Caller: caller, Settings: supervisorEnabledSettings()})
	s.Start(context.Background())
	s.OnRuntimeStatus(readyQQSnapshot())
	receive(t, caller.started)
	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "waiting"})
	s.HandleRuntimeEvent(RuntimeEvent{Name: "other_place_login_reconnect", Phase: "waiting"})

	closed := make(chan struct{})
	go func() {
		s.Close()
		close(closed)
	}()
	waitFor(t, func() bool { return !s.Status().Running })
	close(release)
	waitClosed(t, closed)

	status := s.Status()
	if status.Running || status.Phase != PhaseStandby || status.Process.Armed ||
		status.Network.Running || status.Network.Busy || status.OtherPlaceLogin.Running || status.OtherPlaceLogin.Busy {
		t.Fatalf("closed supervisor status = %#v", status)
	}
	if got := len(caller.snapshot()); got != 1 {
		t.Fatalf("stale completion dispatched more calls: %d", got)
	}
	s.Close()
}

func TestSupervisorNotReadyCancelsQueuedRetry(t *testing.T) {
	after := &supervisorTestAfter{}
	caller := &fakeGuardianCaller{errors: []error{errors.New("execution context is not ready")}}
	s := NewSupervisor(SupervisorOptions{Caller: caller, Settings: supervisorEnabledSettings(), After: after.After})
	s.Start(context.Background())
	defer s.Close()
	s.OnRuntimeStatus(readyQQSnapshot())
	after.waitForCall(t, 0, 800*time.Millisecond)
	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "waiting"})
	if !s.Status().Network.Busy {
		t.Fatal("waiting event did not mark network busy")
	}

	s.OnRuntimeStatus(RuntimeSnapshot{RuntimeTarget: "qq_ws", Connected: true, Ready: false})
	after.fire(0)
	calls := caller.snapshot()
	if len(calls) != 2 ||
		countGuardianCalls(calls, "gameCtl.setReconnectWatcherEnabled") != 1 ||
		countGuardianCalls(calls, "gameCtl.setOtherPlaceLoginReconnectEnabled") != 1 {
		t.Fatalf("calls after runtime became not ready = %#v", calls)
	}
	if status := s.Status(); status.Phase != PhaseStandby || status.Network.Running || status.Network.Busy {
		t.Fatalf("not-ready status = %#v", status)
	}
}

func TestSupervisorNewReadyGenerationRejectsOldError(t *testing.T) {
	release := make(chan struct{})
	caller := &fakeGuardianCaller{
		errors:             []error{errors.New("execution context is not ready")},
		started:            make(chan guardianCall, 4),
		release:            release,
		ignoreCancellation: true,
	}
	s := NewSupervisor(SupervisorOptions{Caller: caller, Settings: supervisorEnabledSettings()})
	s.Start(context.Background())
	defer s.Close()
	s.OnRuntimeStatus(readyQQSnapshot())
	receive(t, caller.started)
	oldGeneration := s.Generation()
	s.OnRuntimeStatus(readyQQSnapshot())
	receive(t, caller.started)
	if s.IsCurrentGeneration(oldGeneration) {
		t.Fatal("old ready generation remained current")
	}
	close(release)
	waitFor(t, func() bool { return len(caller.snapshot()) == 3 })
	waitFor(t, func() bool { return s.Status().Network.LastResult == "enabled" })
	if got := s.Status().Network.LastError; got != "" {
		t.Fatalf("old generation error overwrote new result: %q", got)
	}
}

func TestSupervisorHandlesRuntimeEventsWithCopiesAndReentrantCallback(t *testing.T) {
	coordinatorCalls := make(chan RecoveryKind, 2)
	coordinator := NewRecoveryCoordinator(CoordinatorOptions{Run: func(_ context.Context, request RecoveryRequest) RecoveryResult {
		coordinatorCalls <- request.Kind
		return RecoveryResult{OK: true, Kind: request.Kind}
	}})
	settings := supervisorEnabledSettings()
	settings.FailureRecoveryEnabled = false
	var s *Supervisor
	callbacks := make(chan RuntimeEvent, 4)
	s = NewSupervisor(SupervisorOptions{
		Settings:    settings,
		Coordinator: coordinator,
		OnEvent: func(event RuntimeEvent) {
			_ = s.Status()
			callbacks <- event
		},
	})
	s.Start(context.Background())
	defer s.Close()
	s.OnRuntimeStatus(readyQQSnapshot())

	s.HandleRuntimeEvent(RuntimeEvent{Name: "other_place_login_reconnect", Phase: "due", RuntimeTarget: "qq_ws"})
	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "detected", RuntimeTarget: "qq_ws"})
	if status := s.Status(); status.Phase != PhaseWaitingReconnect || !status.Network.Busy || !status.OtherPlaceLogin.Busy {
		t.Fatalf("recovering child status = %#v", status)
	}
	first := receive(t, coordinatorCalls)
	second := receive(t, coordinatorCalls)
	if first == second || (first != RecoveryNetworkReconnect && second != RecoveryNetworkReconnect) ||
		(first != RecoveryOtherPlaceLogin && second != RecoveryOtherPlaceLogin) {
		t.Fatalf("recovery kinds = %q, %q", first, second)
	}
	receive(t, callbacks)
	receive(t, callbacks)

	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "failed", Error: "offline"})
	s.HandleRuntimeEvent(RuntimeEvent{Name: "other_place_login_reconnect", Phase: "reconnected", Handled: true})
	receive(t, callbacks)
	receive(t, callbacks)
	status := s.Status()
	if status.Phase != PhaseDegraded || status.Network.Busy || status.Network.LastError != "offline" {
		t.Fatalf("network failure status = %#v", status)
	}
	if status.OtherPlaceLogin.Busy || status.OtherPlaceLogin.LastResult != "reconnected" || status.OtherPlaceLogin.LastHandledAt == "" {
		t.Fatalf("other-place status = %#v", status.OtherPlaceLogin)
	}
	status.RecentEvents[0].Phase = "mutated"
	if s.Status().RecentEvents[0].Phase == "mutated" {
		t.Fatal("Status returned aliased recent events")
	}
	settings = s.Settings()
	settings.NetworkReconnectEnabled = false
	s.UpdateSettings(settings)
	waitFor(t, func() bool { return s.Status().Network.LastResult == "disabled" })
	if status := s.Status(); status.Network.LastError != "" || status.Phase != PhaseWatching {
		t.Fatalf("disabled failed worker remained degraded: %#v", status)
	}
}

func TestSupervisorRestartsOnlyAfterTerminalNetworkRecoveryFailure(t *testing.T) {
	restarted := make(chan RestartReason, 1)
	settings := supervisorEnabledSettings()
	process := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(reason RestartReason) (RestartResult, error) {
			restarted <- reason
			return RestartResult{Status: "launch_dispatched"}, nil
		},
	})
	s := NewSupervisor(SupervisorOptions{
		Settings: settings,
		Process:  process,
		Snapshot: readyQQSnapshot,
	})
	s.Start(context.Background())
	defer s.Close()

	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "detected"})
	select {
	case reason := <-restarted:
		t.Fatalf("detected event restarted process: %#v", reason)
	case <-time.After(100 * time.Millisecond):
	}

	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "failed", Error: "reconnect timeout"})
	select {
	case reason := <-restarted:
		if reason.Reason != "network reconnect failed: reconnect timeout" {
			t.Fatalf("restart reason = %q", reason.Reason)
		}
	case <-time.After(time.Second):
		t.Fatal("terminal network recovery failure did not restart process")
	}
}

func TestSupervisorDoesNotRestartTerminalNetworkFailureAfterNetworkRecoveryIsDisabled(t *testing.T) {
	restarted := make(chan RestartReason, 1)
	settings := supervisorEnabledSettings()
	process := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(reason RestartReason) (RestartResult, error) {
			restarted <- reason
			return RestartResult{Status: "launch_dispatched"}, nil
		},
	})
	var blockSnapshots atomic.Bool
	snapshotBlock := make(chan struct{})
	s := NewSupervisor(SupervisorOptions{
		Settings: settings,
		Process:  process,
		Snapshot: func() RuntimeSnapshot {
			if blockSnapshots.Load() {
				<-snapshotBlock
			}
			return readyQQSnapshot()
		},
	})
	s.Start(context.Background())
	defer s.Close()
	blockSnapshots.Store(true)

	s.lifecycleMu.Lock()
	var release sync.Once
	releaseBlockedState := func() {
		release.Do(func() {
			s.lifecycleMu.Unlock()
			close(snapshotBlock)
		})
	}
	defer releaseBlockedState()
	handled := make(chan struct{})
	go func() {
		s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "failed", Error: "reconnect timeout"})
		close(handled)
	}()
	waitFor(t, func() bool { return len(s.Status().RecentEvents) == 1 })
	s.mu.Lock()
	s.settings.NetworkReconnectEnabled = false
	s.mu.Unlock()
	releaseBlockedState()
	waitClosed(t, handled)

	select {
	case reason := <-restarted:
		t.Fatalf("disabled network recovery restarted process: %#v", reason)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSupervisorRecentEventsHaveFixedLimit(t *testing.T) {
	s := NewSupervisor(SupervisorOptions{Settings: supervisorEnabledSettings()})
	for index := 0; index < 80; index++ {
		s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "waiting"})
	}
	status := s.Status()
	if len(status.RecentEvents) != 50 {
		t.Fatalf("recent event limit = %d", len(status.RecentEvents))
	}
}

func TestSupervisorRuntimeEventCallbackPanicIsIsolated(t *testing.T) {
	called := 0
	s := NewSupervisor(SupervisorOptions{
		Settings: supervisorEnabledSettings(),
		OnEvent: func(RuntimeEvent) {
			called++
			panic("callback failed")
		},
	})
	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "waiting"})
	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "reconnected"})
	if called != 2 || len(s.Status().RecentEvents) != 2 {
		t.Fatalf("panic isolation failed: callbacks=%d status=%#v", called, s.Status())
	}
}

func TestSupervisorRunsWhenAutomationSchedulerIsStopped(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	settings := supervisorEnabledSettings()
	process := NewManager(ManagerOptions{Settings: settings, Now: clock.Now, NewTimer: clock.NewTimer})
	s := NewSupervisor(SupervisorOptions{
		Settings: settings,
		Process:  process,
		Snapshot: readyQQSnapshot,
	})
	s.Start(context.Background())
	defer s.Close()
	clock.waitForTimers(t, 1)
	clock.Advance(3 * time.Second)
	waitFor(t, func() bool { return s.Status().Process.LastCheckAt != "" })
	status := s.Status()
	if status.Process.LastHealthyAt == "" || status.Phase != PhaseWatching {
		t.Fatalf("guardian did not run independently: %#v", status)
	}
}

func TestSupervisorDelegatesProcessHealthUsingCurrentSnapshot(t *testing.T) {
	settings := supervisorEnabledSettings()
	settings.TimeoutThreshold = 1
	process := NewManager(ManagerOptions{Settings: settings, Restart: func(RestartReason) (RestartResult, error) {
		return RestartResult{}, errors.New("restart failed")
	}})
	snapshot := RuntimeSnapshot{RuntimeTarget: "wechat_cdp", Connected: true, Ready: true}
	s := NewSupervisor(SupervisorOptions{Settings: settings, Process: process, Snapshot: func() RuntimeSnapshot { return snapshot }})
	s.Start(context.Background())
	defer s.Close()
	s.NoteHealthy()
	if got := s.Status().Process.RuntimeTarget; got != "wechat_cdp" {
		t.Fatalf("healthy runtime target = %q", got)
	}
	s.OnRuntimeStatus(RuntimeSnapshot{RuntimeTarget: "yyb_cdp", Connected: true, Ready: false})
	status := s.Status().Process
	if status.RuntimeTarget != "yyb_cdp" || status.LastCheckAt == "" {
		t.Fatalf("not-ready process status was not updated: %#v", status)
	}
	if !s.NoteRuntimeError(errors.New("context deadline exceeded")) {
		t.Fatal("restartable runtime error was not delegated")
	}
}

func TestSupervisorRetryClassifierUsesExplicitRuntimeReadinessErrors(t *testing.T) {
	retryable := []string{
		"runtime context is not ready",
		"execution context was destroyed",
		"cannot find execution context with id 3",
		"execution context id is required",
		"gameCtl_not_ready",
		"call_path_not_ready: gameCtl.setReconnectWatcherEnabled",
		"missing methods: gameCtl.setReconnectWatcherEnabled",
	}
	for _, message := range retryable {
		if !isRetryableGuardianSyncError(errors.New(message)) {
			t.Errorf("expected retryable runtime readiness error: %q", message)
		}
	}

	fatal := []string{
		"business execution context quota exceeded",
		"runtime context purchase failed",
		"missing method argument: cropId",
		"crop is not ready",
		"permission denied",
	}
	for _, message := range fatal {
		if isRetryableGuardianSyncError(errors.New(message)) {
			t.Errorf("business error was classified retryable: %q", message)
		}
	}
}

func TestSupervisorIsCurrentAliasesGenerationCheck(t *testing.T) {
	s := NewSupervisor(SupervisorOptions{Settings: supervisorEnabledSettings()})
	s.Start(context.Background())
	defer s.Close()
	s.OnRuntimeStatus(readyQQSnapshot())
	generation := s.Generation()
	if !s.IsCurrent(generation) || !s.IsCurrentGeneration(generation) {
		t.Fatalf("current generation %d was rejected", generation)
	}
	s.OnRuntimeStatus(readyQQSnapshot())
	if s.IsCurrent(generation) || s.IsCurrentGeneration(generation) {
		t.Fatalf("stale generation %d remained current", generation)
	}
}

func TestSupervisorStartAndCloseAreOneLifecycleTransaction(t *testing.T) {
	published := make(chan struct{})
	release := make(chan struct{})
	s := NewSupervisor(SupervisorOptions{
		Settings: supervisorEnabledSettings(),
		Snapshot: func() RuntimeSnapshot {
			close(published)
			<-release
			return readyQQSnapshot()
		},
	})
	startDone := make(chan struct{})
	go func() { s.Start(context.Background()); close(startDone) }()
	waitClosed(t, published)
	closeAttempted := make(chan struct{})
	closeDone := make(chan struct{})
	go func() { close(closeAttempted); s.Close(); close(closeDone) }()
	waitClosed(t, closeAttempted)
	assertNotClosed(t, closeDone)
	close(release)
	waitClosed(t, startDone)
	waitClosed(t, closeDone)
	status := s.Status()
	if status.Running || status.Process.Armed {
		t.Fatalf("close was crossed by start: %#v", status)
	}
}

func TestSupervisorUpdateSettingsWhileStoppedKeepsProcessDisarmed(t *testing.T) {
	settings := supervisorEnabledSettings()
	process := NewManager(ManagerOptions{Settings: settings})
	s := NewSupervisor(SupervisorOptions{Settings: settings, Process: process})
	s.UpdateSettings(settings)
	if status := s.Status(); status.Running || status.Process.Armed {
		t.Fatalf("stopped settings update armed process: %#v", status)
	}
	s.Start(context.Background())
	s.Close()
	s.UpdateSettings(settings)
	if status := s.Status(); status.Running || status.Process.Armed {
		t.Fatalf("post-close settings update armed process: %#v", status)
	}
}

func TestSupervisorRecoveryEventsRequireCurrentEnabledLifecycle(t *testing.T) {
	settings := supervisorEnabledSettings()
	coordinator := NewRecoveryCoordinator(CoordinatorOptions{})
	<-coordinator.runToken
	s := NewSupervisor(SupervisorOptions{Settings: settings, Coordinator: coordinator})

	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "detected"})
	assertCoordinatorPendingCount(t, coordinator, 0)

	s.Start(context.Background())
	s.Close()
	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "detected"})
	assertCoordinatorPendingCount(t, coordinator, 0)

	settings.Enabled = false
	s.UpdateSettings(settings)
	s.Start(context.Background())
	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "detected"})
	assertCoordinatorPendingCount(t, coordinator, 0)
	s.Close()

	settings = supervisorEnabledSettings()
	settings.NetworkReconnectEnabled = false
	s.UpdateSettings(settings)
	s.Start(context.Background())
	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "detected"})
	assertCoordinatorPendingCount(t, coordinator, 0)
	s.Close()
	coordinator.runToken <- struct{}{}
}

func TestSupervisorCloseClearsPendingRecoveryEvents(t *testing.T) {
	coordinator := NewRecoveryCoordinator(CoordinatorOptions{})
	<-coordinator.runToken
	s := NewSupervisor(SupervisorOptions{Settings: supervisorEnabledSettings(), Coordinator: coordinator})
	s.Start(context.Background())
	s.HandleRuntimeEvent(RuntimeEvent{Name: "network_reconnect", Phase: "detected"})
	assertCoordinatorPendingCount(t, coordinator, 1)
	s.Close()
	assertCoordinatorPendingCount(t, coordinator, 0)
	coordinator.runToken <- struct{}{}
}

func assertCoordinatorPendingCount(t *testing.T, coordinator *RecoveryCoordinator, want int) {
	t.Helper()
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if got := len(coordinator.pending); got != want {
		t.Fatalf("pending recovery count = %d, want %d", got, want)
	}
}
