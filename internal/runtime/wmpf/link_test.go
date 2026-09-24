package wmpf

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	farmruntime "Farm_Go/internal/runtime"
	"Farm_Go/internal/runtime/cdp"
)

func TestCDPLinkStartupSequence(t *testing.T) {
	manager := farmruntime.NewManager()
	bridge := &fakeBridge{}
	hook := &fakeHookLoader{}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge:     bridge,
		HookLoader: hook,
	})

	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("start link: %v", err)
	}

	if !bridge.started {
		t.Fatal("bridge was not started")
	}
	if !hook.started {
		t.Fatal("frida hook loader was not started")
	}
	status := manager.Status()
	if status.Target != "wechat_cdp" || status.Phase != farmruntime.PhaseListening {
		t.Fatalf("unexpected status %#v", status)
	}
}

func TestCDPLinkStartBridgeFailureCleansConnectHandle(t *testing.T) {
	bridge := &fakeBridge{startErr: errors.New("bridge failed")}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: bridge})
	if err := link.Start(context.Background()); err == nil {
		t.Fatal("expected bridge start failure")
	}
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("stop after bridge failure: %v", err)
	}
}

func TestCDPLinkHookFailureClosesStartedBridgeAndConnectHandle(t *testing.T) {
	bridge := &fakeBridge{}
	hook := &fakeHookLoader{startErr: errors.New("hook failed")}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: bridge, HookLoader: hook})
	if err := link.Start(context.Background()); err == nil {
		t.Fatal("expected hook start failure")
	}
	if !bridge.closed {
		t.Fatal("started bridge was not closed after hook failure")
	}
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("stop after hook failure: %v", err)
	}
}

func TestCDPLinkConcurrentStopInvalidatesBlockedBridgeStart(t *testing.T) {
	bridge := &blockingStartBridge{entered: make(chan struct{}), release: make(chan struct{})}
	manager := farmruntime.NewManager()
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{Bridge: bridge})
	started := make(chan error, 1)
	go func() { started <- link.Start(context.Background()) }()
	<-bridge.entered
	stopped := make(chan error, 1)
	go func() { stopped <- link.StopAndWait(context.Background()) }()
	waitForConnectHandleInvalidated(t, link)
	close(bridge.release)
	if err := <-started; err == nil || !strings.Contains(err.Error(), "invalidated") {
		t.Fatalf("expected invalidated start, got %v", err)
	}
	if err := <-stopped; err != nil {
		t.Fatalf("stop and wait: %v", err)
	}
	if status := manager.Status(); status.Phase != farmruntime.PhaseDisconnected {
		t.Fatalf("blocked start published after stop: %#v", status)
	}
}

func TestCDPLinkConcurrentStopInvalidatesBlockedHookStart(t *testing.T) {
	bridge := &fakeBridge{}
	hook := &blockingHookLoader{entered: make(chan struct{}), release: make(chan struct{})}
	manager := farmruntime.NewManager()
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{Bridge: bridge, HookLoader: hook})
	started := make(chan error, 1)
	go func() { started <- link.Start(context.Background()) }()
	<-hook.entered
	stopped := make(chan error, 1)
	go func() { stopped <- link.StopAndWait(context.Background()) }()
	waitForConnectHandleInvalidated(t, link)
	close(hook.release)
	if err := <-started; err == nil || !strings.Contains(err.Error(), "invalidated") {
		t.Fatalf("expected invalidated start, got %v", err)
	}
	if err := <-stopped; err != nil {
		t.Fatalf("stop and wait: %v", err)
	}
	if !bridge.closed || manager.Status().Phase != farmruntime.PhaseDisconnected {
		t.Fatalf("hook race did not settle bridge/status: bridge=%#v status=%#v", bridge, manager.Status())
	}
}

func TestCDPLinkCallbackStopDefersBridgeCloseUntilBlockedStartFinishes(t *testing.T) {
	bridge := newBlockingLifecycleBridge()
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: bridge})
	exerciseCallbackStopDuringBlockedStart(t, link, bridge.entered, bridge.startRelease, bridge.closeEntered, bridge.closeRelease, bridge.overlapCount, bridge.closeCallCount)
}

func TestCDPLinkCallbackStopDefersHookCloseUntilBlockedStartFinishes(t *testing.T) {
	hook := newBlockingLifecycleHook()
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: &fakeBridge{}, HookLoader: hook})
	exerciseCallbackStopDuringBlockedStart(t, link, hook.entered, hook.startRelease, hook.closeEntered, hook.closeRelease, hook.overlapCount, hook.closeCallCount)
}

func exerciseCallbackStopDuringBlockedStart(t *testing.T, link *CDPLink, startEntered <-chan struct{}, startRelease chan struct{}, closeEntered <-chan struct{}, closeRelease chan struct{}, overlapCount func() int, closeCallCount func() int) {
	t.Helper()
	started := make(chan error, 1)
	go func() { started <- link.Start(context.Background()) }()
	<-startEntered

	callbackReturned := make(chan error, 1)
	link.OnRuntimeEvent(func(map[string]any) { callbackReturned <- link.Stop(context.Background()) })
	go link.handleCDPEvent("Runtime.bindingCalled", map[string]any{
		"name":    runtimeEventBindingName,
		"payload": `{"kind":"guardian"}`,
	})
	select {
	case err := <-callbackReturned:
		if err != nil {
			t.Fatalf("callback stop: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("callback Stop blocked behind dependency Start")
	}
	select {
	case <-closeEntered:
		t.Fatal("dependency Close overlapped dependency Start")
	default:
	}

	ownerStopped := make(chan error, 1)
	go func() { ownerStopped <- link.StopAndWait(context.Background()) }()
	select {
	case err := <-ownerStopped:
		t.Fatalf("StopAndWait returned before dependency Start finished: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(startRelease)
	if err := <-started; err == nil || !strings.Contains(err.Error(), "invalidated") {
		t.Fatalf("expected invalidated start, got %v", err)
	}
	<-closeEntered
	select {
	case err := <-ownerStopped:
		t.Fatalf("StopAndWait returned before dependency Close finished: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(closeRelease)
	if err := <-ownerStopped; err != nil {
		t.Fatalf("StopAndWait: %v", err)
	}
	if overlaps := overlapCount(); overlaps != 0 {
		t.Fatalf("dependency Start and Close overlapped %d times", overlaps)
	}
	if calls := closeCallCount(); calls != 1 {
		t.Fatalf("callback Stop and owner StopAndWait called dependency Close %d times, want 1", calls)
	}
	link.mu.Lock()
	currentCleanup := link.cleanupHandle
	link.mu.Unlock()
	if currentCleanup != nil {
		t.Fatalf("StopAndWait left cleanup handle %p", currentCleanup)
	}
}

func TestCDPLinkStartDrainsPendingCleanupBeforeRestartingDependency(t *testing.T) {
	bridge := &orderedLifecycleBridge{startEntered: make(chan struct{})}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: bridge, Evaluator: &fakeEvaluator{}})
	cleanup := &runtimeCleanupHandle{done: make(chan struct{})}
	link.mu.Lock()
	link.cleanupHandle = cleanup
	link.mu.Unlock()

	started := make(chan error, 1)
	go func() { started <- link.Start(context.Background()) }()
	startedBeforeCleanup := false
	select {
	case <-bridge.startEntered:
		startedBeforeCleanup = true
	case <-time.After(25 * time.Millisecond):
	}
	go link.runDependencyCleanup(cleanup)
	if err := <-started; err != nil {
		t.Fatalf("restart after cleanup: %v", err)
	}
	<-cleanup.done
	bridge.mu.Lock()
	operations := append([]string(nil), bridge.operations...)
	running := bridge.running
	bridge.mu.Unlock()
	if strings.Join(operations, ",") != "close,start" || !running {
		t.Fatalf("dependency restart order=%v running=%v, want [close start] and running", operations, running)
	}
	if status := link.Status(); status.Phase != farmruntime.PhaseListening {
		t.Fatalf("restart status: %#v", status)
	}
	if startedBeforeCleanup {
		t.Fatal("replacement Start ran before pending cleanup")
	}
}

func TestCDPLinkConcurrentStopsCoalesceDependencyCleanup(t *testing.T) {
	bridge := newBlockingLifecycleBridge()
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: bridge})
	ownerStopped := make(chan error, 1)
	go func() { ownerStopped <- link.StopAndWait(context.Background()) }()
	<-bridge.closeEntered

	const stopCount = 8
	stopped := make(chan error, stopCount)
	for i := 0; i < stopCount; i++ {
		go func() { stopped <- link.Stop(context.Background()) }()
	}
	for i := 0; i < stopCount; i++ {
		if err := <-stopped; err != nil {
			t.Fatalf("concurrent Stop: %v", err)
		}
	}
	close(bridge.closeRelease)
	if err := <-ownerStopped; err != nil {
		t.Fatalf("owner StopAndWait: %v", err)
	}
	if calls := bridge.closeCallCount(); calls != 1 {
		t.Fatalf("concurrent Stops called dependency Close %d times, want 1", calls)
	}
	link.mu.Lock()
	current := link.cleanupHandle
	link.mu.Unlock()
	if current != nil {
		t.Fatalf("owner StopAndWait left cleanup handle %p", current)
	}
}

func TestCDPLinkCleanupPublishesCompletionAfterReleasingStartGate(t *testing.T) {
	bridge := newBlockingLifecycleBridge()
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: bridge})
	stopped := make(chan error, 1)
	go func() { stopped <- link.StopAndWait(context.Background()) }()
	<-bridge.closeEntered

	link.mu.Lock()
	close(bridge.closeRelease)
	gateAcquired := make(chan struct{})
	gateRelease := make(chan struct{})
	go func() {
		link.startMu.Lock()
		close(gateAcquired)
		<-gateRelease
		link.startMu.Unlock()
	}()
	acquiredBeforeTerminal := false
	select {
	case <-gateAcquired:
		acquiredBeforeTerminal = true
	case <-time.After(100 * time.Millisecond):
	}
	select {
	case err := <-stopped:
		t.Fatalf("StopAndWait returned before cleanup terminal publish: %v", err)
	default:
	}
	close(gateRelease)
	link.mu.Unlock()
	if err := <-stopped; err != nil {
		t.Fatalf("StopAndWait: %v", err)
	}
	if !acquiredBeforeTerminal {
		t.Fatal("cleanup did not release startMu before publishing terminal state")
	}
}

func TestCDPLinkLateOwnerReceivesCompletedCleanupError(t *testing.T) {
	closeErr := errors.New("bridge close failed")
	bridge := newSequencedCloseBridge(closeErr, nil)
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: bridge})
	if err := link.Stop(context.Background()); err != nil {
		t.Fatalf("callback Stop: %v", err)
	}
	<-bridge.closeEntered
	link.mu.Lock()
	cleanup := link.cleanupHandle
	link.mu.Unlock()
	close(bridge.closeRelease)
	<-cleanup.done

	if err := link.StopAndWait(context.Background()); !errors.Is(err, closeErr) {
		t.Fatalf("late owner got %v, want %v", err, closeErr)
	}
	if calls := bridge.closeCallCount(); calls != 1 {
		t.Fatalf("late owner retried before consuming completed result: Close calls=%d", calls)
	}
}

func TestCDPLinkConcurrentOwnersReceiveSameCleanupError(t *testing.T) {
	closeErr := errors.New("bridge close failed")
	bridge := newSequencedCloseBridge(closeErr)
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: bridge})
	first := make(chan error, 1)
	second := make(chan error, 1)
	go func() { first <- link.StopAndWait(context.Background()) }()
	<-bridge.closeEntered
	go func() { second <- link.StopAndWait(context.Background()) }()
	waitForOwnerStopCount(t, link, 2)
	close(bridge.closeRelease)
	if err := <-first; !errors.Is(err, closeErr) {
		t.Fatalf("first owner got %v, want %v", err, closeErr)
	}
	if err := <-second; !errors.Is(err, closeErr) {
		t.Fatalf("second owner got %v, want %v", err, closeErr)
	}
}

func TestCDPLinkDelayedOwnerReceivesCapturedCleanupError(t *testing.T) {
	closeErr := errors.New("bridge close failed")
	bridge := newSequencedCloseBridge(closeErr)
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: bridge})
	first := make(chan error, 1)
	go func() { first <- link.StopAndWait(context.Background()) }()
	<-bridge.closeEntered

	blocker := &runtimeConnectHandle{done: make(chan struct{})}
	link.mu.Lock()
	if link.connectHandles == nil {
		link.connectHandles = make(map[*runtimeConnectHandle]struct{})
	}
	link.connectHandles[blocker] = struct{}{}
	link.mu.Unlock()
	second := make(chan error, 1)
	go func() { second <- link.StopAndWait(context.Background()) }()
	waitForOwnerStopCount(t, link, 2)
	close(bridge.closeRelease)
	if err := <-first; !errors.Is(err, closeErr) {
		t.Fatalf("first owner got %v, want %v", err, closeErr)
	}
	link.mu.Lock()
	delete(link.connectHandles, blocker)
	link.mu.Unlock()
	close(blocker.done)
	if err := <-second; !errors.Is(err, closeErr) {
		t.Fatalf("delayed owner got %v, want captured %v", err, closeErr)
	}
}

func waitForOwnerStopCount(t *testing.T, link *CDPLink, count int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		link.mu.Lock()
		current := link.ownerStopCount
		link.mu.Unlock()
		if current == count {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("owner stop count did not reach %d", count)
}

func TestCDPLinkRetriesOnlyBridgeAfterPartialCleanupFailure(t *testing.T) {
	bridgeErr := errors.New("bridge close failed")
	hook := newSequencedCloseHook(nil)
	bridge := newSequencedCloseBridge(bridgeErr, nil)
	close(bridge.closeRelease)
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: bridge, HookLoader: hook})
	if err := link.StopAndWait(context.Background()); !errors.Is(err, bridgeErr) {
		t.Fatalf("first StopAndWait: %v", err)
	}
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("retry StopAndWait: %v", err)
	}
	if hook.closeCallCount() != 1 || bridge.closeCallCount() != 2 {
		t.Fatalf("partial retry counts: hook=%d bridge=%d, want 1/2", hook.closeCallCount(), bridge.closeCallCount())
	}
}

func TestCDPLinkRetriesOnlyHookAfterPartialCleanupFailure(t *testing.T) {
	hookErr := errors.New("hook close failed")
	hook := newSequencedCloseHook(hookErr, nil)
	bridge := newSequencedCloseBridge(nil)
	close(bridge.closeRelease)
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: bridge, HookLoader: hook})
	if err := link.StopAndWait(context.Background()); !errors.Is(err, hookErr) {
		t.Fatalf("first StopAndWait: %v", err)
	}
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("retry StopAndWait: %v", err)
	}
	if hook.closeCallCount() != 2 || bridge.closeCallCount() != 1 {
		t.Fatalf("partial retry counts: hook=%d bridge=%d, want 2/1", hook.closeCallCount(), bridge.closeCallCount())
	}
}

func TestCDPLinkStartRetriesIncompleteCleanupBeforeDependencyStart(t *testing.T) {
	bridgeErr := errors.New("bridge close failed")
	bridge := newSequencedCloseBridge(bridgeErr, nil)
	close(bridge.closeRelease)
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: bridge, Evaluator: &fakeEvaluator{}})
	if err := link.StopAndWait(context.Background()); !errors.Is(err, bridgeErr) {
		t.Fatalf("first StopAndWait: %v", err)
	}
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("Start after cleanup retry: %v", err)
	}
	bridge.mu.Lock()
	operations := append([]string(nil), bridge.operations...)
	bridge.mu.Unlock()
	if strings.Join(operations, ",") != "close,close,start" {
		t.Fatalf("operations=%v, want [close close start]", operations)
	}
}

func TestCDPLinkFailedStartReturnsCleanupErrorAndLeavesRetryableDependency(t *testing.T) {
	startErr := errors.New("bridge start failed")
	closeErr := errors.New("bridge close failed")
	bridge := newSequencedCloseBridge(closeErr, nil)
	bridge.startErr = startErr
	close(bridge.closeRelease)
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: bridge})
	err := link.Start(context.Background())
	if !errors.Is(err, startErr) || !errors.Is(err, closeErr) {
		t.Fatalf("Start error=%v, want start and cleanup errors", err)
	}
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("retry cleanup: %v", err)
	}
	if calls := bridge.closeCallCount(); calls != 2 {
		t.Fatalf("cleanup Close calls=%d, want 2", calls)
	}
}

func TestCDPLinkReplacementWaitsForOldPartialCleanupRetry(t *testing.T) {
	closeErr := errors.New("old bridge close failed")
	bridge := newReplacementPartialCloseBridge(closeErr)
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: bridge, Evaluator: &fakeEvaluator{}})
	oldStarted := make(chan error, 1)
	go func() { oldStarted <- link.Start(context.Background()) }()
	<-bridge.firstStartEntered
	replacementStarted := make(chan error, 1)
	go func() { replacementStarted <- link.Start(context.Background()) }()
	waitForConnectHandleInvalidated(t, link)
	close(bridge.firstStartRelease)
	if err := <-oldStarted; err == nil || !strings.Contains(err.Error(), "invalidated") {
		t.Fatalf("old Start: %v", err)
	}
	select {
	case <-bridge.replacementStartEntered:
		t.Fatal("replacement dependency Start ran before failed cleanup retry")
	case err := <-replacementStarted:
		if !errors.Is(err, closeErr) {
			t.Fatalf("replacement Start got %v, want %v", err, closeErr)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("replacement neither returned cleanup error nor started")
	}
	retryStarted := make(chan error, 1)
	go func() { retryStarted <- link.Start(context.Background()) }()
	<-bridge.retryCloseEntered
	select {
	case <-bridge.replacementStartEntered:
		t.Fatal("replacement dependency Start ran before cleanup retry completed")
	default:
	}
	close(bridge.retryCloseRelease)
	if err := <-retryStarted; err != nil {
		t.Fatalf("retry replacement Start: %v", err)
	}
	if bridge.closeCallCount() != 2 {
		t.Fatalf("bridge Close calls=%d, want 2", bridge.closeCallCount())
	}
}

func TestCDPLinkStartWaitsForOwnerStopAndRecoversAfterward(t *testing.T) {
	evaluator := &ownerCloseEvaluator{called: make(chan struct{}), release: make(chan struct{})}
	bridge := &orderedLifecycleBridge{startEntered: make(chan struct{})}
	manager := farmruntime.NewManager()
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{Bridge: bridge, Evaluator: evaluator})
	ownerStopped := make(chan error, 1)
	go func() { ownerStopped <- link.StopAndWait(context.Background()) }()
	<-evaluator.called

	link.setEvaluator(&fakeEvaluator{})
	started := make(chan error, 1)
	go func() { started <- link.Start(context.Background()) }()
	select {
	case <-bridge.startEntered:
		t.Fatal("replacement Start ran before owner StopAndWait returned")
	case <-time.After(25 * time.Millisecond):
	}
	close(evaluator.release)
	if err := <-ownerStopped; err != nil {
		t.Fatalf("owner StopAndWait: %v", err)
	}
	if err := <-started; err != nil {
		t.Fatalf("replacement Start: %v", err)
	}
	if status := manager.Status(); status.Phase != farmruntime.PhaseListening {
		t.Fatalf("replacement did not recover Listening: %#v", status)
	}
}

func TestCDPLinkRapidSecondStartFailureDoesNotLoseFirstHandle(t *testing.T) {
	bridge := &rapidStartBridge{firstEntered: make(chan struct{}), firstRelease: make(chan struct{})}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: bridge})
	first := make(chan error, 1)
	go func() { first <- link.Start(context.Background()) }()
	<-bridge.firstEntered
	second := make(chan error, 1)
	go func() { second <- link.Start(context.Background()) }()
	waitForConnectHandleInvalidated(t, link)
	close(bridge.firstRelease)
	if err := <-second; err == nil {
		t.Fatal("expected second start failure")
	}
	if err := <-first; err == nil || !strings.Contains(err.Error(), "invalidated") {
		t.Fatalf("expected first start invalidated, got %v", err)
	}
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("stop after rapid starts: %v", err)
	}
	link.mu.Lock()
	remainingHandles := len(link.connectHandles)
	link.mu.Unlock()
	if remainingHandles != 0 {
		t.Fatalf("StopAndWait returned with %d registered handles", remainingHandles)
	}
}

func TestCDPLinkBlockedStartDoesNotCloseSuccessfulReplacementBridge(t *testing.T) {
	bridge := &replacementStartBridge{firstEntered: make(chan struct{}), firstRelease: make(chan struct{})}
	manager := farmruntime.NewManager()
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{Bridge: bridge, Evaluator: &fakeEvaluator{}})
	first := make(chan error, 1)
	go func() { first <- link.Start(context.Background()) }()
	<-bridge.firstEntered
	second := make(chan error, 1)
	go func() { second <- link.Start(context.Background()) }()
	waitForConnectHandleInvalidated(t, link)
	close(bridge.firstRelease)
	if err := <-first; err == nil || !strings.Contains(err.Error(), "invalidated") {
		t.Fatalf("expected first start invalidated, got %v", err)
	}
	if err := <-second; err != nil {
		t.Fatalf("replacement start: %v", err)
	}
	bridge.mu.Lock()
	running := bridge.running
	bridge.mu.Unlock()
	if !running || manager.Status().Phase != farmruntime.PhaseListening {
		t.Fatalf("old start closed replacement bridge: running=%v status=%#v", running, manager.Status())
	}
	_ = link.StopAndWait(context.Background())
}

func waitForConnectHandleInvalidated(t *testing.T, link *CDPLink) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		link.mu.Lock()
		invalidated := false
		for handle := range link.connectHandles {
			if handle.invalidated {
				invalidated = true
				break
			}
		}
		link.mu.Unlock()
		if invalidated {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("connect handle was not invalidated")
}

func TestCDPLinkContextReadyTransition(t *testing.T) {
	manager := farmruntime.NewManager()
	bridge := &fakeBridge{state: DebugBridgeState{MiniappConnected: true}}
	evaluator := &fakeEvaluator{value: map[string]any{"hasCc": true}}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetYYBCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge:    bridge,
		Evaluator: evaluator,
	})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("start link: %v", err)
	}

	if err := link.UpdateRuntimeContext(context.Background(), cdp.ExecutionContext{ID: 7, Name: "gameContext"}); err != nil {
		t.Fatalf("update context: %v", err)
	}

	status := manager.Status()
	if status.Target != "yyb_cdp" || status.Phase != farmruntime.PhaseReady || !status.Ready {
		t.Fatalf("expected ready yyb status, got %#v", status)
	}
	if status.InstanceID != "gameContext" || status.HostVersion == "" || status.LastSeenAt == "" {
		t.Fatalf("expected ready detail fields, got %#v", status)
	}
}

func TestUpdateRuntimeContextDoesNotCommitAfterConcurrentStop(t *testing.T) {
	evaluator := &blockingEvaluator{entered: make(chan struct{}), release: make(chan struct{})}
	manager := farmruntime.NewManager()
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetYYBCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge:    &fakeBridge{state: DebugBridgeState{MiniappConnected: true}},
		Evaluator: evaluator,
	})
	result := make(chan error, 1)
	go func() {
		result <- link.UpdateRuntimeContext(context.Background(), cdp.ExecutionContext{ID: 7, Name: "gameContext"})
	}()
	<-evaluator.entered
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("stop and wait: %v", err)
	}
	close(evaluator.release)
	if err := <-result; err == nil || !strings.Contains(err.Error(), "invalidated") {
		t.Fatalf("expected invalidated setup, got %v", err)
	}
	status := manager.Status()
	if status.Ready || status.Phase != farmruntime.PhaseDisconnected {
		t.Fatalf("stopped probe committed Ready: %#v", status)
	}
}

func TestUpdateRuntimeContextStaleResultsCannotOverwriteReplacementStatus(t *testing.T) {
	tests := []struct {
		name  string
		value any
		err   error
	}{
		{name: "evaluate error", err: errors.New("probe failed")},
		{name: "not ready", value: map[string]any{"hasCc": false, "hasGameCtl": false}},
		{name: "ready", value: map[string]any{"hasCc": true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evaluator := &blockingResultEvaluator{entered: make(chan struct{}), release: make(chan struct{}), value: test.value, err: test.err}
			manager := farmruntime.NewManager()
			link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetYYBCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
				Bridge:    &fakeBridge{state: DebugBridgeState{MiniappConnected: true}},
				Evaluator: evaluator,
			})
			result := make(chan error, 1)
			go func() {
				result <- link.UpdateRuntimeContext(context.Background(), cdp.ExecutionContext{ID: 7, Name: "old"})
			}()
			<-evaluator.entered
			if err := link.StopAndWait(context.Background()); err != nil {
				t.Fatalf("StopAndWait: %v", err)
			}
			link.setEvaluator(&fakeEvaluator{})
			manager.SetStatus(farmruntime.Status{Target: string(link.Target()), Phase: farmruntime.PhaseListening, Connected: true})
			close(evaluator.release)
			if err := <-result; err == nil || !strings.Contains(err.Error(), "invalidated") {
				t.Fatalf("stale result returned %v, want invalidated", err)
			}
			if status := manager.Status(); status.Phase != farmruntime.PhaseListening {
				t.Fatalf("stale result overwrote replacement status: %#v", status)
			}
		})
	}
}

func TestUpdateRuntimeContextConcurrentSameIdentityUsesNewestSetupToken(t *testing.T) {
	tests := []struct {
		name          string
		oldValue      any
		oldErr        error
		newValue      any
		newErr        error
		newResultText string
		finalPhase    farmruntime.Phase
	}{
		{
			name:       "old error after new ready",
			oldErr:     errors.New("old probe failed"),
			newValue:   map[string]any{"hasCc": true},
			finalPhase: farmruntime.PhaseReady,
		},
		{
			name:       "old not ready after new ready",
			oldValue:   map[string]any{"hasCc": false, "hasGameCtl": false},
			newValue:   map[string]any{"hasCc": true},
			finalPhase: farmruntime.PhaseReady,
		},
		{
			name:          "old success after new error",
			oldValue:      map[string]any{"hasCc": true},
			newErr:        errors.New("new probe failed"),
			newResultText: "new probe failed",
			finalPhase:    farmruntime.PhaseError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evaluator := &concurrentSetupEvaluator{
				firstEntered: make(chan struct{}),
				firstRelease: make(chan struct{}),
				firstValue:   test.oldValue,
				firstErr:     test.oldErr,
				secondValue:  test.newValue,
				secondErr:    test.newErr,
			}
			manager := farmruntime.NewManager()
			link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetYYBCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
				Bridge:    &fakeBridge{state: DebugBridgeState{MiniappConnected: true}},
				Evaluator: evaluator,
			})
			oldResult := make(chan error, 1)
			go func() {
				oldResult <- link.UpdateRuntimeContext(context.Background(), cdp.ExecutionContext{ID: 7, Name: "same"})
			}()
			<-evaluator.firstEntered
			newErr := link.UpdateRuntimeContext(context.Background(), cdp.ExecutionContext{ID: 7, Name: "same"})
			if test.newResultText == "" {
				if newErr != nil {
					t.Fatalf("new setup: %v", newErr)
				}
			} else if newErr == nil || !strings.Contains(newErr.Error(), test.newResultText) {
				t.Fatalf("new setup got %v, want %q", newErr, test.newResultText)
			}
			if status := manager.Status(); status.Phase != test.finalPhase {
				t.Fatalf("new setup status: %#v", status)
			}
			close(evaluator.firstRelease)
			if err := <-oldResult; err == nil || !strings.Contains(err.Error(), "invalidated") {
				t.Fatalf("old setup returned %v, want invalidated", err)
			}
			if status := manager.Status(); status.Phase != test.finalPhase {
				t.Fatalf("old setup overwrote new status: %#v", status)
			}
		})
	}
}

func TestEvaluatorReplacementClearsOldContextAndPendingBeforeLifecycleEvent(t *testing.T) {
	oldEvaluator := &fakeRuntimeEventClient{}
	link := NewCDPLink(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager())
	oldGeneration := link.setEvaluator(oldEvaluator)
	oldToken := link.beginRuntimeContextSetup(oldEvaluator, oldGeneration, 7)
	if oldToken == 0 {
		t.Fatal("failed to begin old setup")
	}
	link.mu.Lock()
	link.context = cdp.ExecutionContext{ID: 7, Name: "old"}
	link.mu.Unlock()

	newEvaluator := &fakeRuntimeEventClient{}
	newGeneration := link.setEvaluator(newEvaluator)
	link.handleCDPEventFrom(newGeneration, "Runtime.executionContextDestroyed", map[string]any{"executionContextId": 7})
	if current := link.currentEvaluator(); current != newEvaluator {
		t.Fatalf("old context/pending detached replacement evaluator: got %T", current)
	}
	link.mu.Lock()
	contextID := link.context.ID
	pendingToken := link.pendingSetupToken
	link.mu.Unlock()
	if contextID != 0 || pendingToken != 0 {
		t.Fatalf("replacement retained old state: context=%d token=%d", contextID, pendingToken)
	}
}

func TestUpdateRuntimeContextWithoutEvaluatorDoesNotCreatePhantomPending(t *testing.T) {
	link := NewCDPLink(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager())
	if err := link.UpdateRuntimeContext(context.Background(), cdp.ExecutionContext{ID: 7, Name: "missing"}); err == nil {
		t.Fatal("expected missing evaluator error")
	}
	link.mu.Lock()
	pendingToken := link.pendingSetupToken
	pendingContextID := link.pendingContextID
	pendingSource := link.pendingSource
	link.mu.Unlock()
	if pendingToken != 0 || pendingContextID != 0 || pendingSource != nil {
		t.Fatalf("nil evaluator left phantom pending: token=%d context=%d source=%T", pendingToken, pendingContextID, pendingSource)
	}

	evaluator := &fakeRuntimeEventClient{}
	generation := link.setEvaluator(evaluator)
	link.handleCDPEventFrom(generation, "Runtime.executionContextDestroyed", map[string]any{"executionContextId": 7})
	if current := link.currentEvaluator(); current != evaluator {
		t.Fatalf("phantom pending detached new evaluator: got %T", current)
	}
}

func TestOldSetupCannotClearReplacementPendingOrCommit(t *testing.T) {
	oldEvaluator := &fakeRuntimeEventClient{}
	link := NewCDPLink(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager())
	oldGeneration := link.setEvaluator(oldEvaluator)
	oldToken := link.beginRuntimeContextSetup(oldEvaluator, oldGeneration, 7)
	newEvaluator := &fakeRuntimeEventClient{}
	newGeneration := link.setEvaluator(newEvaluator)
	newToken := link.beginRuntimeContextSetup(newEvaluator, newGeneration, 7)
	if oldToken == 0 || newToken == 0 {
		t.Fatalf("setup tokens: old=%d new=%d", oldToken, newToken)
	}

	link.abandonRuntimeContextSetup(oldEvaluator, oldGeneration, 7, oldToken)
	if link.commitRuntimeContext(oldEvaluator, oldGeneration, oldToken, cdp.ExecutionContext{ID: 7, Name: "old"}) {
		t.Fatal("old setup committed after evaluator replacement")
	}
	link.mu.Lock()
	pendingToken := link.pendingSetupToken
	pendingSource := link.pendingSource
	pendingGeneration := link.pendingGeneration
	link.mu.Unlock()
	if pendingToken != newToken || pendingSource != newEvaluator || pendingGeneration != newGeneration {
		t.Fatalf("old setup cleared replacement pending: token=%d source=%T generation=%d", pendingToken, pendingSource, pendingGeneration)
	}
}

func TestCDPLinkRejectsContextWithoutRuntimeSignals(t *testing.T) {
	manager := farmruntime.NewManager()
	bridge := &fakeBridge{state: DebugBridgeState{MiniappConnected: true}}
	evaluator := &fakeEvaluator{value: map[string]any{"hasGameCtl": false, "methodCount": 0}}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetYYBCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge:    bridge,
		Evaluator: evaluator,
	})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("start link: %v", err)
	}

	err := link.UpdateRuntimeContext(context.Background(), cdp.ExecutionContext{ID: 7, Name: "gameContext"})

	if err == nil || !strings.Contains(err.Error(), "runtime context is not ready") {
		t.Fatalf("expected runtime context is not ready, got %v", err)
	}
	status := manager.Status()
	if status.Ready || status.Phase == farmruntime.PhaseReady {
		t.Fatalf("context without runtime signals should not be ready: %#v", status)
	}
}

func TestCDPLinkFallsBackToDefaultRuntimeWhenReportedContextsAreWeak(t *testing.T) {
	client := newBlockingRuntimeClient(true, false)
	client.contexts = []cdp.ExecutionContext{{
		ID:     7,
		Name:   "worker",
		Origin: "https://servicewechat.com",
	}}
	client.evaluateValues = map[int]any{
		7: map[string]any{
			"hasDocument": true,
			"hasWx":       true,
		},
		0: map[string]any{
			"hasCc":         true,
			"hasGameGlobal": true,
		},
	}
	link := NewCDPLink(
		ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP),
		CDPLinkConfig{},
		farmruntime.NewManager(),
	)

	selected, err := link.waitForGameCtlExecutionContext(context.Background(), client, "gameContext", time.Second)
	if err != nil {
		t.Fatalf("select execution context: %v", err)
	}
	if selected.ID != defaultExecutionContextID || selected.Name != "default" {
		t.Fatalf("expected default execution context, got %#v", selected)
	}
	if got := client.evaluatedContextIDs(); !slices.Contains(got, 7) || !slices.Contains(got, 0) {
		t.Fatalf("expected explicit and default probes, got %v", got)
	}
}

func TestCDPLinkPrefersStrongReportedContextBeforeDefaultFallback(t *testing.T) {
	client := newBlockingRuntimeClient(true, false)
	client.contexts = []cdp.ExecutionContext{{
		ID:     7,
		Name:   "gameContext",
		Origin: "https://servicewechat.com",
	}}
	client.evaluateValues = map[int]any{
		7: map[string]any{"hasCc": true},
		0: map[string]any{"hasCc": true, "hasGameCtl": true},
	}
	link := NewCDPLink(
		ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP),
		CDPLinkConfig{},
		farmruntime.NewManager(),
	)

	selected, err := link.waitForGameCtlExecutionContext(context.Background(), client, "gameContext", time.Second)
	if err != nil {
		t.Fatalf("select execution context: %v", err)
	}
	if selected.ID != 7 {
		t.Fatalf("expected reported game context, got %#v", selected)
	}
	if got := client.evaluatedContextIDs(); !slices.Equal(got, []int{7}) {
		t.Fatalf("default fallback should not run after a strong explicit match, got %v", got)
	}
}

func TestCDPLinkRejectsWeakReportedAndDefaultContexts(t *testing.T) {
	client := newBlockingRuntimeClient(true, false)
	client.contexts = []cdp.ExecutionContext{{ID: 7, Name: "worker"}}
	client.evaluateValues = map[int]any{
		7: map[string]any{"hasDocument": true},
		0: map[string]any{"hasGameGlobal": true},
	}
	link := NewCDPLink(
		ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP),
		CDPLinkConfig{},
		farmruntime.NewManager(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	selected, err := link.waitForGameCtlExecutionContext(ctx, client, "gameContext", time.Second)
	if err == nil {
		t.Fatalf("expected weak contexts to remain unready, selected %#v", selected)
	}
	if !strings.Contains(err.Error(), "runtime context is not ready") && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected weak-context error: %v", err)
	}
}

func TestRuntimeReadinessPendingErrorsAreNotFatal(t *testing.T) {
	for _, message := range []string{
		"execution context is not ready",
		"gameCtl_not_ready",
		`miniapp jscontext "gameContext" not found`,
		"runtime evaluator is not connected",
	} {
		if !isRuntimeReadinessPendingError(errors.New(message)) {
			t.Fatalf("expected pending readiness error for %q", message)
		}
	}

	if isRuntimeReadinessPendingError(errors.New("cdp websocket closed")) {
		t.Fatal("transport errors should remain fatal")
	}
}

func TestCDPLinkRetriesAfterMiniappReconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := farmruntime.NewManager()
	bridge := newReconnectBridge()
	evaluator := &fakeEvaluator{value: map[string]any{"hasCc": true}}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge:    bridge,
		Evaluator: evaluator,
	})

	if err := link.Start(ctx); err != nil {
		t.Fatalf("start link: %v", err)
	}
	bridge.setConnected(true)
	if err := link.UpdateRuntimeContext(ctx, cdp.ExecutionContext{ID: 1, Name: "gameContext"}); err != nil {
		t.Fatalf("ready context: %v", err)
	}
	bridge.setConnected(false)
	link.OnMiniappDisconnected(errors.New("miniapp closed"))

	status := manager.Status()
	if status.Phase != farmruntime.PhaseDisconnected || status.Ready {
		t.Fatalf("expected disconnected after close, got %#v", status)
	}

	bridge.setConnected(true)
	link.OnMiniappConnected()

	status = manager.Status()
	if status.Phase != farmruntime.PhaseHandshaking || !status.Connected {
		t.Fatalf("expected handshaking after reconnect, got %#v", status)
	}
}

func TestCDPLinkButtonScriptSourcePrefersEmbeddedSourceForAllProfiles(t *testing.T) {
	for _, target := range []farmruntime.RuntimeTarget{
		farmruntime.RuntimeTargetWeChatCDP,
		farmruntime.RuntimeTargetYYBCDP,
	} {
		t.Run(string(target), func(t *testing.T) {
			link := NewCDPLink(
				ProfileForTarget(target),
				CDPLinkConfig{
					ButtonScript:     "globalThis.gameCtl = {};",
					ButtonScriptPath: filepath.Join(t.TempDir(), "missing", "button.js"),
				},
				farmruntime.NewManager(),
			)

			source, err := link.buttonScriptSource()
			if err != nil {
				t.Fatalf("load embedded button script: %v", err)
			}
			if string(source) != "globalThis.gameCtl = {};" {
				t.Fatalf("unexpected embedded source %q", source)
			}
		})
	}
}

func TestCDPLinkButtonScriptSourceFallsBackToDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "button.js")
	if err := os.WriteFile(path, []byte("globalThis.gameCtl = { disk: true };"), 0o644); err != nil {
		t.Fatalf("write disk button script: %v", err)
	}
	link := NewCDPLink(
		ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP),
		CDPLinkConfig{ButtonScriptPath: path},
		farmruntime.NewManager(),
	)

	source, err := link.buttonScriptSource()
	if err != nil {
		t.Fatalf("load disk button script: %v", err)
	}
	if string(source) != "globalThis.gameCtl = { disk: true };" {
		t.Fatalf("unexpected disk source %q", source)
	}
}

func TestCDPLinkSupportsWeChatAndYYBProfiles(t *testing.T) {
	wx := NewCDPLink(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager())
	yyb := NewCDPLink(ProfileForTarget(farmruntime.RuntimeTargetYYBCDP), CDPLinkConfig{}, farmruntime.NewManager())

	if wx.Target() != farmruntime.RuntimeTargetWeChatCDP {
		t.Fatalf("unexpected wx target %q", wx.Target())
	}
	if yyb.Target() != farmruntime.RuntimeTargetYYBCDP {
		t.Fatalf("unexpected yyb target %q", yyb.Target())
	}
}

func TestNewCDPLinkConfiguresFridaHookWhenEnabled(t *testing.T) {
	link := NewCDPLink(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{
		DebugPort:    9420,
		FridaRoot:    testFridaRoot(),
		FridaEnabled: true,
	}, farmruntime.NewManager())

	if link.hook == nil {
		t.Fatal("expected frida hook loader")
	}
	loader := link.hook.(*FridaHookLoader)
	if loader.debugWebSocketURL != "ws://127.0.0.1:9420/" {
		t.Fatalf("unexpected debug websocket url %q", loader.debugWebSocketURL)
	}
}

func TestCDPLinkDiagnostics(t *testing.T) {
	manager := farmruntime.NewManager()
	bridge := &fakeBridge{state: DebugBridgeState{MiniappConnected: true, CDPConnected: true}}
	evaluator := &fakeEvaluator{value: map[string]any{"hasGameCtl": true, "methodCount": 3}}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge:    bridge,
		Evaluator: evaluator,
	})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("start link: %v", err)
	}
	if err := link.UpdateRuntimeContext(context.Background(), cdp.ExecutionContext{ID: 7, Name: "gameContext"}); err != nil {
		t.Fatalf("update context: %v", err)
	}

	host, err := link.Call(context.Background(), "host.describe", nil, time.Second)
	if err != nil {
		t.Fatalf("host.describe: %v", err)
	}
	hostMap := host.(map[string]any)
	if hostMap["target"] != "wechat_cdp" {
		t.Fatalf("unexpected host.describe %#v", hostMap)
	}

	probe, err := link.Call(context.Background(), "gameCtl.probe", nil, time.Second)
	if err != nil {
		t.Fatalf("gameCtl.probe: %v", err)
	}
	if probe.(map[string]any)["methodCount"] != 3 {
		t.Fatalf("unexpected probe %#v", probe)
	}
	if evaluator.contextID != 7 {
		t.Fatalf("expected selected context id, got %d", evaluator.contextID)
	}
}

func TestCDPLinkCallsGameCtlRuntimeMethod(t *testing.T) {
	manager := farmruntime.NewManager()
	bridge := &fakeBridge{state: DebugBridgeState{MiniappConnected: true, CDPConnected: true}}
	evaluator := &fakeEvaluator{value: map[string]any{"hasGameCtl": true, "name": "Dpo.L", "level": float64(118)}}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge:    bridge,
		Evaluator: evaluator,
	})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("start link: %v", err)
	}
	if err := link.UpdateRuntimeContext(context.Background(), cdp.ExecutionContext{ID: 7, Name: "gameContext"}); err != nil {
		t.Fatalf("update context: %v", err)
	}

	profile, err := link.Call(context.Background(), "gameCtl.getPlayerProfile", []any{map[string]any{"silent": true}}, time.Second)
	if err != nil {
		t.Fatalf("gameCtl.getPlayerProfile: %v", err)
	}

	if profile.(map[string]any)["name"] != "Dpo.L" {
		t.Fatalf("unexpected profile %#v", profile)
	}
	if evaluator.contextID != 7 {
		t.Fatalf("expected selected context id, got %d", evaluator.contextID)
	}
	if !strings.Contains(evaluator.expression, `"getPlayerProfile"`) || !strings.Contains(evaluator.expression, `"silent":true`) {
		t.Fatalf("expected expression to call getPlayerProfile with args, got %s", evaluator.expression)
	}
}

func TestProbeExpressionUsesEmptyRequiredMethodsList(t *testing.T) {
	expression := probeExpression(nil)

	if strings.Contains(expression, "const requiredMethods = null") {
		t.Fatalf("probe expression should use an empty array for nil required methods: %s", expression)
	}
	if !strings.Contains(expression, "const requiredMethods = [];") {
		t.Fatalf("probe expression should declare an empty required methods array: %s", expression)
	}
	if strings.Contains(expression, "farmRoot: ctl && typeof ctl.findFarmRoot") {
		t.Fatalf("probe expression should not return a raw farm root object: %s", expression)
	}
	if !strings.Contains(expression, "ctl.fullPath(root)") {
		t.Fatalf("probe expression should serialize farm root through fullPath: %s", expression)
	}
}

func TestCDPLinkForwardsDecodedRuntimeBindingEvents(t *testing.T) {
	link := NewCDPLink(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager())
	var calls []string
	link.OnRuntimeEvent(func(event map[string]any) {
		calls = append(calls, "first")
		event["kind"] = "mutated"
		event["nested"].(map[string]any)["value"] = "mutated"
		event["items"].([]any)[0].(map[string]any)["value"] = "mutated"
	})
	link.OnRuntimeEvent(func(map[string]any) {
		calls = append(calls, "panic")
		panic("handler failure")
	})
	link.OnRuntimeEvent(func(event map[string]any) {
		if event["nested"].(map[string]any)["value"] != "original" || event["items"].([]any)[0].(map[string]any)["value"] != "original" {
			calls = append(calls, "nested-mutated")
			return
		}
		calls = append(calls, event["kind"].(string)+":"+event["runtimeTarget"].(string))
	})
	unsubscribe := link.OnRuntimeEvent(func(map[string]any) { calls = append(calls, "unsubscribed") })
	unsubscribe()
	unsubscribe()

	link.handleCDPEvent("Runtime.bindingCalled", map[string]any{"name": "wrong", "payload": `{"kind":"ignored"}`})
	link.handleCDPEvent("Runtime.bindingCalled", map[string]any{"name": runtimeEventBindingName, "payload": `not-json`})
	link.handleCDPEvent("Runtime.bindingCalled", map[string]any{"name": runtimeEventBindingName, "payload": `[]`})
	link.handleCDPEvent("Page.loadEventFired", map[string]any{})
	link.handleCDPEvent("Runtime.bindingCalled", map[string]any{
		"name":    runtimeEventBindingName,
		"payload": `{"kind":"guardian","nested":{"value":"original"},"items":[{"value":"original"}]}`,
	})

	if got := strings.Join(calls, ","); got != "first,panic,guardian:wechat_cdp" {
		t.Fatalf("unexpected runtime event calls %q", got)
	}
}

func TestCDPLinkInvalidatesSelectedDestroyedContext(t *testing.T) {
	evaluator := &fakeClosableEvaluator{closed: make(chan struct{})}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{
		Bridge:    &fakeBridge{state: DebugBridgeState{MiniappConnected: true}},
		Evaluator: evaluator,
	})
	link.mu.Lock()
	link.context = cdp.ExecutionContext{ID: 7, Name: "gameContext"}
	link.mu.Unlock()

	link.handleCDPEvent("Runtime.executionContextDestroyed", map[string]any{"executionContextId": 7})

	if link.hasRuntimeConnection() {
		t.Fatal("destroyed selected context remained connected")
	}
	select {
	case <-evaluator.closed:
	case <-time.After(time.Second):
		t.Fatal("destroyed context evaluator was not closed")
	}
}

func TestCDPLinkInvalidatesSelectedContextWhenContextsCleared(t *testing.T) {
	evaluator := &fakeClosableEvaluator{closed: make(chan struct{})}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{
		Bridge:    &fakeBridge{state: DebugBridgeState{MiniappConnected: true}},
		Evaluator: evaluator,
	})
	link.mu.Lock()
	link.context = cdp.ExecutionContext{ID: 7, Name: "gameContext"}
	link.mu.Unlock()

	link.handleCDPEvent("Runtime.executionContextsCleared", map[string]any{})

	if link.hasRuntimeConnection() {
		t.Fatal("cleared selected context remained connected")
	}
	select {
	case <-evaluator.closed:
	case <-time.After(time.Second):
		t.Fatal("cleared context evaluator was not closed")
	}
}

func TestCDPLinkLifecycleCallbackRetainsOwnerEvaluatorForStopJoin(t *testing.T) {
	tests := []struct {
		name   string
		method string
		params map[string]any
	}{
		{name: "destroyed", method: "Runtime.executionContextDestroyed", params: map[string]any{"executionContextId": 7}},
		{name: "cleared", method: "Runtime.executionContextsCleared", params: map[string]any{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			oldEvaluator := &ownerCloseEvaluator{called: make(chan struct{}), release: make(chan struct{}), requested: make(chan struct{})}
			link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{
				Bridge:    &fakeBridge{state: DebugBridgeState{MiniappConnected: true}},
				Evaluator: oldEvaluator,
			})
			link.mu.Lock()
			link.context = cdp.ExecutionContext{ID: 7, Name: "old"}
			link.mu.Unlock()
			link.handleCDPEvent(test.method, test.params)
			select {
			case <-oldEvaluator.requested:
			default:
				t.Fatal("lifecycle callback did not request evaluator Close")
			}
			link.mu.Lock()
			pending := len(link.closingEvaluators)
			link.mu.Unlock()
			if pending != 1 {
				t.Fatalf("lifecycle callback left %d pending evaluators, want 1", pending)
			}

			newEvaluator := &ownerCloseEvaluator{called: make(chan struct{}), release: make(chan struct{})}
			close(newEvaluator.release)
			link.setEvaluator(newEvaluator)
			stopped := make(chan error, 1)
			go func() { stopped <- link.StopAndWait(context.Background()) }()
			<-oldEvaluator.called
			select {
			case err := <-stopped:
				t.Fatalf("StopAndWait returned before old evaluator join: %v", err)
			case <-time.After(25 * time.Millisecond):
			}
			close(oldEvaluator.release)
			if err := <-stopped; err != nil {
				t.Fatalf("StopAndWait: %v", err)
			}
			oldEvaluator.mu.Lock()
			oldCalls := oldEvaluator.calls
			oldEvaluator.mu.Unlock()
			newEvaluator.mu.Lock()
			newCalls := newEvaluator.calls
			newEvaluator.mu.Unlock()
			if oldCalls == 0 || newCalls == 0 {
				t.Fatalf("evaluator joins: old=%d new=%d", oldCalls, newCalls)
			}
		})
	}
}

func TestCDPLinkIgnoresLateLifecycleEventFromOldClient(t *testing.T) {
	link := NewCDPLink(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager())
	oldClient := &fakeRuntimeEventClient{}
	oldGeneration := link.setEvaluator(oldClient)
	if err := link.registerRuntimeEventBinding(context.Background(), oldClient, oldGeneration); err != nil {
		t.Fatalf("register old client: %v", err)
	}
	newClient := &fakeRuntimeEventClient{}
	newGeneration := link.setEvaluator(newClient)
	if err := link.registerRuntimeEventBinding(context.Background(), newClient, newGeneration); err != nil {
		t.Fatalf("register new client: %v", err)
	}
	link.mu.Lock()
	link.context = cdp.ExecutionContext{ID: 9, Name: "replacement"}
	link.mu.Unlock()

	oldClient.handler("Runtime.executionContextsCleared", map[string]any{})

	if link.currentEvaluator() != newClient {
		t.Fatal("late old-client event cleared the new evaluator")
	}
	link.mu.Lock()
	selectedID := link.context.ID
	link.mu.Unlock()
	if selectedID != 9 {
		t.Fatalf("late old-client event cleared replacement context: %d", selectedID)
	}
}

func TestCDPLinkRejectsContextCommitDestroyedAfterWrapper(t *testing.T) {
	client := &fakeRuntimeEventClient{}
	manager := farmruntime.NewManager()
	link := NewCDPLink(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager)
	generation := link.setEvaluator(client)
	setupToken := link.beginRuntimeContextSetup(client, generation, 7)
	if setupToken == 0 {
		t.Fatal("failed to begin context setup")
	}
	// The wrapper evaluation has succeeded; destruction arrives before context commit.
	link.handleCDPEventFrom(generation, "Runtime.executionContextDestroyed", map[string]any{"executionContextId": 7})

	if link.commitRuntimeContext(client, generation, setupToken, cdp.ExecutionContext{ID: 7, Name: "gameContext"}) {
		t.Fatal("destroyed pending context was committed")
	}
	if manager.Status().Ready || manager.Status().Phase == farmruntime.PhaseReady {
		t.Fatalf("destroyed pending context marked runtime ready: %#v", manager.Status())
	}
}

func TestCDPLinkReadyListenerCanReenterHostDescribe(t *testing.T) {
	manager := farmruntime.NewManager()
	client := &fakeRuntimeEventClient{}
	link := NewCDPLink(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager)
	generation := link.setEvaluator(client)
	setupToken := link.beginRuntimeContextSetup(client, generation, 7)
	if setupToken == 0 {
		t.Fatal("failed to begin context setup")
	}
	listenerDone := make(chan struct{})
	manager.OnStatusChange(func(_ farmruntime.Status, next farmruntime.Status) {
		if next.Ready {
			_, _ = link.Call(context.Background(), "host.describe", nil, time.Second)
			close(listenerDone)
		}
	})
	committed := make(chan bool, 1)
	go func() {
		committed <- link.commitRuntimeContext(client, generation, setupToken, cdp.ExecutionContext{ID: 7, Name: "gameContext"})
	}()
	select {
	case ok := <-committed:
		if !ok {
			t.Fatal("context commit was rejected")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Ready listener reentry deadlocked on link mutex")
	}
	select {
	case <-listenerDone:
	case <-time.After(time.Second):
		t.Fatal("Ready listener was not called")
	}
}

func TestCDPLinkPreservesRuntimeTargetFromBindingPayload(t *testing.T) {
	link := NewCDPLink(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager())
	var received map[string]any
	link.OnRuntimeEvent(func(event map[string]any) { received = event })

	link.handleCDPEvent("Runtime.bindingCalled", map[string]any{
		"name":    runtimeEventBindingName,
		"payload": `{"kind":"guardian","runtimeTarget":"custom"}`,
	})

	if received["runtimeTarget"] != "custom" {
		t.Fatalf("runtimeTarget was overwritten: %#v", received)
	}
}

func TestCDPLinkUnsubscribeRemovesHandlerOrderEntry(t *testing.T) {
	link := NewCDPLink(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager())
	unsubscribe := link.OnRuntimeEvent(func(map[string]any) {})
	unsubscribe()
	unsubscribe()

	link.mu.Lock()
	orderLength := len(link.runtimeEventOrder)
	link.mu.Unlock()
	if orderLength != 0 {
		t.Fatalf("unsubscribe left %d runtime event order tombstones", orderLength)
	}
}

func TestRuntimeEventHandlerCanStopCDPLink(t *testing.T) {
	evaluator := &callbackStopEvaluator{
		closeRequested:   make(chan struct{}),
		callbackReturned: make(chan struct{}),
	}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{
		Bridge:    &fakeBridge{state: DebugBridgeState{MiniappConnected: true}},
		Evaluator: evaluator,
	})
	link.OnRuntimeEvent(func(map[string]any) {
		if err := link.Stop(context.Background()); err != nil {
			t.Errorf("stop link: %v", err)
		}
		evaluator.signalCallbackReturned()
	})

	finished := make(chan struct{})
	go func() {
		link.handleCDPEvent("Runtime.bindingCalled", map[string]any{
			"name":    runtimeEventBindingName,
			"payload": `{"kind":"guardian"}`,
		})
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(100 * time.Millisecond):
		evaluator.signalCallbackReturned()
		t.Fatal("CDPLink.Stop deadlocked inside runtime event callback")
	}
}

func TestRuntimeEventCallbackRequestsEvaluatorCloseBeforeOwnerJoin(t *testing.T) {
	evaluator := &ownerCloseEvaluator{
		called:    make(chan struct{}),
		release:   make(chan struct{}),
		requested: make(chan struct{}),
	}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{
		Bridge:    &fakeBridge{},
		Evaluator: evaluator,
	})
	callbackReturned := make(chan error, 1)
	link.OnRuntimeEvent(func(map[string]any) { callbackReturned <- link.Stop(context.Background()) })
	go link.handleCDPEvent("Runtime.bindingCalled", map[string]any{
		"name":    runtimeEventBindingName,
		"payload": `{"kind":"guardian"}`,
	})
	select {
	case err := <-callbackReturned:
		if err != nil {
			t.Fatalf("callback stop: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("callback Stop blocked on evaluator owner join")
	}
	select {
	case <-evaluator.requested:
	default:
		t.Fatal("callback Stop did not request evaluator close")
	}
	link.mu.Lock()
	pending := len(link.closingEvaluators)
	link.mu.Unlock()
	if pending != 1 {
		t.Fatalf("callback Stop left %d pending evaluators, want 1", pending)
	}

	ownerStopped := make(chan error, 1)
	go func() { ownerStopped <- link.StopAndWait(context.Background()) }()
	<-evaluator.called
	select {
	case err := <-ownerStopped:
		t.Fatalf("StopAndWait returned before evaluator release: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(evaluator.release)
	if err := <-ownerStopped; err != nil {
		t.Fatalf("StopAndWait: %v", err)
	}
	link.mu.Lock()
	pending = len(link.closingEvaluators)
	link.mu.Unlock()
	if pending != 0 {
		t.Fatalf("successful owner join left %d pending evaluators", pending)
	}
}

func TestCDPLinkOldStopCannotOverwriteReplacementListeningStatus(t *testing.T) {
	evaluator := &blockingCloseOnlyEvaluator{entered: make(chan struct{}), release: make(chan struct{})}
	manager := farmruntime.NewManager()
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge:    &fakeBridge{},
		Evaluator: evaluator,
	})
	stopped := make(chan error, 1)
	go func() { stopped <- link.Stop(context.Background()) }()
	<-evaluator.entered

	link.setEvaluator(&fakeEvaluator{})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("replacement Start: %v", err)
	}
	if status := manager.Status(); status.Phase != farmruntime.PhaseListening {
		t.Fatalf("replacement did not publish Listening: %#v", status)
	}
	close(evaluator.release)
	if err := <-stopped; err != nil {
		t.Fatalf("old Stop: %v", err)
	}
	if status := manager.Status(); status.Phase != farmruntime.PhaseListening {
		t.Fatalf("old Stop overwrote replacement status: %#v", status)
	}
}

func TestCDPLinkOwnerStopAndWaitJoinsEvaluator(t *testing.T) {
	evaluator := &ownerCloseEvaluator{called: make(chan struct{}), release: make(chan struct{})}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{
		Bridge:    &fakeBridge{},
		Evaluator: evaluator,
	})
	stopped := make(chan error, 1)
	go func() { stopped <- link.StopAndWait(context.Background()) }()
	<-evaluator.called
	select {
	case err := <-stopped:
		t.Fatalf("StopAndWait returned before evaluator join: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(evaluator.release)
	if err := <-stopped; err != nil {
		t.Fatalf("StopAndWait: %v", err)
	}
}

func TestCDPLinkCanceledStopSettlesStatusAndSecondStopJoins(t *testing.T) {
	evaluator := &ownerCloseEvaluator{called: make(chan struct{}), release: make(chan struct{})}
	manager := farmruntime.NewManager()
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{Bridge: &fakeBridge{}, Evaluator: evaluator})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := link.StopAndWait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled stop, got %v", err)
	}
	if status := manager.Status(); status.Phase != farmruntime.PhaseDisconnected || status.Ready {
		t.Fatalf("canceled stop did not settle status: %#v", status)
	}
	close(evaluator.release)
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("second stop did not finish join: %v", err)
	}
}

func TestCDPLinkJoinsOldAndNewEvaluatorsAfterCanceledStop(t *testing.T) {
	oldEval := &ownerCloseEvaluator{called: make(chan struct{}), release: make(chan struct{})}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: &fakeBridge{}, Evaluator: oldEval})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := link.StopAndWait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled old stop, got %v", err)
	}
	newEval := &ownerCloseEvaluator{called: make(chan struct{}), release: make(chan struct{})}
	link.setEvaluator(newEval)
	close(oldEval.release)
	close(newEval.release)
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("join old and new evaluators: %v", err)
	}
	oldEval.mu.Lock()
	oldCalls := oldEval.calls
	oldEval.mu.Unlock()
	newEval.mu.Lock()
	newCalls := newEval.calls
	newEval.mu.Unlock()
	if oldCalls < 2 || newCalls < 1 {
		t.Fatalf("not all evaluators joined: old=%d new=%d", oldCalls, newCalls)
	}
}

func TestCDPLinkCloseOnlyUncomparableEvaluatorClosesOnce(t *testing.T) {
	counter := &closeCounter{}
	evaluator := uncomparableCloseEvaluator{values: []int{1}, counter: counter}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: &fakeBridge{}, Evaluator: evaluator})
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("first stop: %v", err)
	}
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("second stop: %v", err)
	}
	counter.mu.Lock()
	calls := counter.calls
	counter.mu.Unlock()
	if calls != 1 {
		t.Fatalf("close-only evaluator called %d times", calls)
	}
}

func TestCDPLinkFailedEvaluatorJoinIsRetriedThenRemoved(t *testing.T) {
	evaluator := &retryOwnerEvaluator{errs: []error{errors.New("permanent for now"), nil}}
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{Bridge: &fakeBridge{}, Evaluator: evaluator})
	if err := link.StopAndWait(context.Background()); err == nil {
		t.Fatal("expected first join error")
	}
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("retry join: %v", err)
	}
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("post-success stop: %v", err)
	}
	if evaluator.calls != 2 {
		t.Fatalf("unexpected join calls %d", evaluator.calls)
	}
}

func TestCDPLinkStopAndWaitLeavesNoConnectWorkerOrStatusUpdates(t *testing.T) {
	manager := farmruntime.NewManager()
	bridge := newReconnectBridge()
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{Bridge: bridge})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("start link: %v", err)
	}
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("stop and wait: %v", err)
	}
	statusAfterStop := manager.Status()
	updates := make(chan farmruntime.Status, 1)
	manager.OnStatusChange(func(_ farmruntime.Status, next farmruntime.Status) { updates <- next })

	bridge.setConnected(true)
	time.Sleep(150 * time.Millisecond)
	if got := manager.Status(); got != statusAfterStop {
		t.Fatalf("status changed after owner stop: before=%#v after=%#v", statusAfterStop, got)
	}
	select {
	case update := <-updates:
		t.Fatalf("connect worker published after owner stop: %#v", update)
	default:
	}
	link.mu.Lock()
	handle := link.connectHandle
	link.mu.Unlock()
	if handle != nil {
		t.Fatal("connect worker handle remained after StopAndWait")
	}
}

func TestCDPLinkLateConnectedWorkerCannotReplaceOrDetachNewEvaluator(t *testing.T) {
	bridge := newReconnectBridge()
	bridge.setConnected(true)
	oldClient := newBlockingRuntimeClient(false, false)
	newClient := newBlockingRuntimeClient(true, true)
	clients := make(chan runtimeConnectionClient, 2)
	clients <- oldClient
	clients <- newClient
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager(), CDPLinkDeps{
		Bridge:        bridge,
		ClientFactory: func() runtimeConnectionClient { return <-clients },
	})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	<-oldClient.connectEntered
	if err := link.Stop(context.Background()); err != nil {
		t.Fatalf("Stop old worker: %v", err)
	}
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("replacement Start: %v", err)
	}
	<-newClient.sendEntered
	if current := link.currentEvaluator(); current != newClient {
		t.Fatalf("replacement evaluator not installed: %T", current)
	}

	close(oldClient.connectRelease)
	<-oldClient.closed
	if current := link.currentEvaluator(); current != newClient {
		t.Fatalf("late old worker replaced or detached new evaluator: got %T", current)
	}

	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("owner StopAndWait: %v", err)
	}
	if oldClient.closeWaitCallCount() == 0 || newClient.closeWaitCallCount() == 0 {
		t.Fatalf("owner joins: old=%d new=%d", oldClient.closeWaitCallCount(), newClient.closeWaitCallCount())
	}
	link.mu.Lock()
	remainingHandles := len(link.connectHandles)
	remainingEvaluators := len(link.closingEvaluators)
	link.mu.Unlock()
	if remainingHandles != 0 || remainingEvaluators != 0 || link.currentEvaluator() != nil {
		t.Fatalf("owner returned with handles=%d evaluators=%d current=%T", remainingHandles, remainingEvaluators, link.currentEvaluator())
	}
}

func TestCDPLinkStaleReconnectLoopCannotDetachOrOverwriteReplacement(t *testing.T) {
	bridge := newBlockingReconnectStateBridge()
	oldClient := newReadyRuntimeClient()
	newClient := newReadyRuntimeClient()
	clients := make(chan runtimeConnectionClient, 2)
	clients <- oldClient
	clients <- newClient
	manager := farmruntime.NewManager()
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge:        bridge,
		ClientFactory: func() runtimeConnectionClient { return <-clients },
	})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	waitForCurrentEvaluator(t, link, oldClient)
	waitForStatusPhase(t, manager, farmruntime.PhaseReady)
	link.mu.Lock()
	oldHandle := link.connectHandle
	link.mu.Unlock()
	bridge.blockNextStateAsDisconnected()
	<-bridge.stateBlocked

	if err := link.Stop(context.Background()); err != nil {
		t.Fatalf("Stop old worker: %v", err)
	}
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("replacement Start: %v", err)
	}
	waitForCurrentEvaluator(t, link, newClient)
	waitForStatusPhase(t, manager, farmruntime.PhaseReady)
	close(bridge.stateRelease)
	<-oldHandle.done
	if current := link.currentEvaluator(); current != newClient {
		t.Fatalf("stale disconnected loop detached replacement evaluator: got %T", current)
	}
	if status := manager.Status(); status.Phase == farmruntime.PhaseDisconnected || status.Phase == farmruntime.PhaseHandshaking {
		t.Fatalf("stale loop overwrote replacement status: %#v", status)
	}
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("owner StopAndWait: %v", err)
	}
}

func waitForCurrentEvaluator(t *testing.T, link *CDPLink, evaluator runtimeEvaluator) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if link.currentEvaluator() == evaluator {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("current evaluator did not become %T", evaluator)
}

func waitForStatusPhase(t *testing.T, manager *farmruntime.Manager, phase farmruntime.Phase) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if manager.Status().Phase == phase {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("runtime status did not become %s: %#v", phase, manager.Status())
}

func TestCDPLinkInstallsRuntimeEventBridgeOnEveryConnection(t *testing.T) {
	link := NewCDPLink(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager())
	for reconnect := 0; reconnect < 2; reconnect++ {
		client := &fakeRuntimeEventClient{}
		generation := link.setEvaluator(client)
		if err := link.registerRuntimeEventBinding(context.Background(), client, generation); err != nil {
			t.Fatalf("register binding on connection %d: %v", reconnect+1, err)
		}
		if err := link.installRuntimeEventWrapper(context.Background(), client, 7, time.Second); err != nil {
			t.Fatalf("install wrapper on connection %d: %v", reconnect+1, err)
		}

		if len(client.sends) != 1 || client.sends[0].method != "Runtime.addBinding" || client.sends[0].params["name"] != runtimeEventBindingName {
			t.Fatalf("unexpected addBinding request on connection %d: %#v", reconnect+1, client.sends)
		}
		if client.contextID != 7 {
			t.Fatalf("wrapper used wrong context on connection %d: %d", reconnect+1, client.contextID)
		}
		if !strings.Contains(client.expression, "globalThis.__qqFarmRuntimeEventBridge = function (event)") ||
			!strings.Contains(client.expression, "globalThis.__qqFarmRuntimeEventBinding(JSON.stringify(event))") {
			t.Fatalf("unexpected wrapper expression on connection %d: %s", reconnect+1, client.expression)
		}
		if strings.Contains(client.expression, "globalThis.__qqFarmRuntimeEventBinding =") {
			t.Fatalf("wrapper overwrites native binding: %s", client.expression)
		}
	}
}

func TestCDPLinkRuntimeEventSetupFailuresAreReturned(t *testing.T) {
	link := NewCDPLink(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, farmruntime.NewManager())
	bindingErr := errors.New("binding failed")
	bindingClient := &fakeRuntimeEventClient{sendErr: bindingErr}
	if err := link.registerRuntimeEventBinding(context.Background(), bindingClient, link.setEvaluator(bindingClient)); !errors.Is(err, bindingErr) {
		t.Fatalf("expected binding error, got %v", err)
	}
	wrapperErr := errors.New("wrapper failed")
	if err := link.installRuntimeEventWrapper(context.Background(), &fakeRuntimeEventClient{evaluateErr: wrapperErr}, 7, time.Second); !errors.Is(err, wrapperErr) {
		t.Fatalf("expected wrapper error, got %v", err)
	}
}

type fakeBridge struct {
	started  bool
	closed   bool
	state    DebugBridgeState
	startErr error
}

type blockingStartBridge struct {
	fakeBridge
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

type blockingLifecycleBridge struct {
	fakeBridge
	mu           sync.Mutex
	activeStarts int
	overlaps     int
	closeCalls   int
	entered      chan struct{}
	startRelease chan struct{}
	closeEntered chan struct{}
	closeRelease chan struct{}
	startOnce    sync.Once
	closeOnce    sync.Once
}

type orderedLifecycleBridge struct {
	fakeBridge
	mu           sync.Mutex
	operations   []string
	running      bool
	startEntered chan struct{}
	startOnce    sync.Once
}

type sequencedCloseBridge struct {
	fakeBridge
	mu           sync.Mutex
	closeErrs    []error
	startErr     error
	closeCalls   int
	operations   []string
	closeEntered chan struct{}
	closeRelease chan struct{}
	closeOnce    sync.Once
}

func newSequencedCloseBridge(closeErrs ...error) *sequencedCloseBridge {
	return &sequencedCloseBridge{
		closeErrs:    closeErrs,
		closeEntered: make(chan struct{}),
		closeRelease: make(chan struct{}),
	}
}

func newBlockingLifecycleBridge() *blockingLifecycleBridge {
	return &blockingLifecycleBridge{
		entered:      make(chan struct{}),
		startRelease: make(chan struct{}),
		closeEntered: make(chan struct{}),
		closeRelease: make(chan struct{}),
	}
}

type blockingHookLoader struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

type blockingLifecycleHook struct {
	mu           sync.Mutex
	activeStarts int
	overlaps     int
	closeCalls   int
	entered      chan struct{}
	startRelease chan struct{}
	closeEntered chan struct{}
	closeRelease chan struct{}
	startOnce    sync.Once
	closeOnce    sync.Once
}

type sequencedCloseHook struct {
	mu         sync.Mutex
	closeErrs  []error
	closeCalls int
}

func newSequencedCloseHook(closeErrs ...error) *sequencedCloseHook {
	return &sequencedCloseHook{closeErrs: closeErrs}
}

func newBlockingLifecycleHook() *blockingLifecycleHook {
	return &blockingLifecycleHook{
		entered:      make(chan struct{}),
		startRelease: make(chan struct{}),
		closeEntered: make(chan struct{}),
		closeRelease: make(chan struct{}),
	}
}

type rapidStartBridge struct {
	fakeBridge
	mu           sync.Mutex
	calls        int
	firstEntered chan struct{}
	firstRelease chan struct{}
}

type replacementStartBridge struct {
	fakeBridge
	mu           sync.Mutex
	calls        int
	running      bool
	firstEntered chan struct{}
	firstRelease chan struct{}
}

type replacementPartialCloseBridge struct {
	fakeBridge
	mu                      sync.Mutex
	startCalls              int
	closeCalls              int
	firstCloseErr           error
	firstStartEntered       chan struct{}
	firstStartRelease       chan struct{}
	replacementStartEntered chan struct{}
	retryCloseEntered       chan struct{}
	retryCloseRelease       chan struct{}
	firstStartOnce          sync.Once
	replacementStartOnce    sync.Once
	retryCloseOnce          sync.Once
}

func newReplacementPartialCloseBridge(firstCloseErr error) *replacementPartialCloseBridge {
	return &replacementPartialCloseBridge{
		firstCloseErr:           firstCloseErr,
		firstStartEntered:       make(chan struct{}),
		firstStartRelease:       make(chan struct{}),
		replacementStartEntered: make(chan struct{}),
		retryCloseEntered:       make(chan struct{}),
		retryCloseRelease:       make(chan struct{}),
	}
}

type reconnectBridge struct {
	mu    sync.Mutex
	state DebugBridgeState
}

type blockingReconnectStateBridge struct {
	mu             sync.Mutex
	blockNextState bool
	stateBlocked   chan struct{}
	stateRelease   chan struct{}
	blockOnce      sync.Once
}

func newBlockingReconnectStateBridge() *blockingReconnectStateBridge {
	return &blockingReconnectStateBridge{
		stateBlocked: make(chan struct{}),
		stateRelease: make(chan struct{}),
	}
}

func (b *blockingReconnectStateBridge) blockNextStateAsDisconnected() {
	b.mu.Lock()
	b.blockNextState = true
	b.mu.Unlock()
}

func newReconnectBridge() *reconnectBridge {
	return &reconnectBridge{}
}

func (b *reconnectBridge) setConnected(connected bool) {
	b.mu.Lock()
	b.state.MiniappConnected = connected
	if connected {
		b.state.MiniappClients = 1
	} else {
		b.state.MiniappClients = 0
	}
	b.mu.Unlock()
}

func (b *reconnectBridge) Start(ctx context.Context) error {
	return nil
}

func (b *reconnectBridge) Close() error {
	return nil
}

func (b *reconnectBridge) State() DebugBridgeState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

func (b *reconnectBridge) CDPURL() string {
	return "ws://127.0.0.1:62000"
}

func (b *reconnectBridge) WaitJSContext(ctx context.Context, preferredName string, timeout time.Duration) (JSContext, error) {
	return JSContext{ID: "ctx-1", Name: preferredName}, nil
}

func (b *reconnectBridge) ConnectJSContext(id string) error {
	return nil
}

func (b *blockingReconnectStateBridge) Start(context.Context) error { return nil }
func (b *blockingReconnectStateBridge) Close() error                { return nil }

func (b *blockingReconnectStateBridge) State() DebugBridgeState {
	b.mu.Lock()
	block := b.blockNextState
	if block {
		b.blockNextState = false
	}
	b.mu.Unlock()
	if block {
		b.blockOnce.Do(func() { close(b.stateBlocked) })
		<-b.stateRelease
		return DebugBridgeState{}
	}
	return DebugBridgeState{MiniappConnected: true, MiniappClients: 1}
}

func (b *blockingReconnectStateBridge) CDPURL() string { return "ws://127.0.0.1:62000" }

func (b *blockingReconnectStateBridge) WaitJSContext(_ context.Context, preferredName string, _ time.Duration) (JSContext, error) {
	return JSContext{ID: "ctx-1", Name: preferredName}, nil
}

func (b *blockingReconnectStateBridge) ConnectJSContext(string) error { return nil }

func (f *fakeBridge) Start(ctx context.Context) error {
	f.started = true
	return f.startErr
}

func (b *blockingStartBridge) Start(context.Context) error {
	b.once.Do(func() { close(b.entered) })
	<-b.release
	b.started = true
	return nil
}

func (b *blockingLifecycleBridge) Start(context.Context) error {
	b.mu.Lock()
	b.activeStarts++
	b.mu.Unlock()
	b.startOnce.Do(func() { close(b.entered) })
	<-b.startRelease
	b.mu.Lock()
	b.activeStarts--
	b.mu.Unlock()
	return nil
}

func (b *blockingLifecycleBridge) Close() error {
	b.mu.Lock()
	b.closeCalls++
	if b.activeStarts != 0 {
		b.overlaps++
	}
	b.mu.Unlock()
	b.closeOnce.Do(func() { close(b.closeEntered) })
	<-b.closeRelease
	return nil
}

func (b *blockingLifecycleBridge) closeCallCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closeCalls
}

func (b *orderedLifecycleBridge) Start(context.Context) error {
	b.mu.Lock()
	b.operations = append(b.operations, "start")
	b.running = true
	b.mu.Unlock()
	b.startOnce.Do(func() { close(b.startEntered) })
	return nil
}

func (b *orderedLifecycleBridge) Close() error {
	b.mu.Lock()
	b.operations = append(b.operations, "close")
	b.running = false
	b.mu.Unlock()
	return nil
}

func (b *sequencedCloseBridge) Start(context.Context) error {
	b.mu.Lock()
	b.operations = append(b.operations, "start")
	b.mu.Unlock()
	return b.startErr
}

func (b *sequencedCloseBridge) Close() error {
	b.mu.Lock()
	b.closeCalls++
	b.operations = append(b.operations, "close")
	b.mu.Unlock()
	b.closeOnce.Do(func() { close(b.closeEntered) })
	<-b.closeRelease
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.closeErrs) == 0 {
		return nil
	}
	err := b.closeErrs[0]
	b.closeErrs = b.closeErrs[1:]
	return err
}

func (b *sequencedCloseBridge) closeCallCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closeCalls
}

func (b *blockingLifecycleBridge) overlapCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.overlaps
}

func (f *fakeBridge) Close() error {
	f.closed = true
	return nil
}

func (f *fakeBridge) State() DebugBridgeState {
	return f.state
}

func (f *fakeBridge) CDPURL() string {
	return "ws://127.0.0.1:62000"
}

func (f *fakeBridge) WaitJSContext(ctx context.Context, preferredName string, timeout time.Duration) (JSContext, error) {
	return JSContext{}, context.DeadlineExceeded
}

func (f *fakeBridge) ConnectJSContext(id string) error {
	return nil
}

type fakeHookLoader struct {
	started  bool
	startErr error
}

func (f *fakeHookLoader) Start(ctx context.Context) error {
	f.started = true
	return f.startErr
}

func (h *blockingHookLoader) Start(context.Context) error {
	h.once.Do(func() { close(h.entered) })
	<-h.release
	return nil
}

func (h *blockingLifecycleHook) Start(context.Context) error {
	h.mu.Lock()
	h.activeStarts++
	h.mu.Unlock()
	h.startOnce.Do(func() { close(h.entered) })
	<-h.startRelease
	h.mu.Lock()
	h.activeStarts--
	h.mu.Unlock()
	return nil
}

func (h *blockingLifecycleHook) Close() error {
	h.mu.Lock()
	h.closeCalls++
	if h.activeStarts != 0 {
		h.overlaps++
	}
	h.mu.Unlock()
	h.closeOnce.Do(func() { close(h.closeEntered) })
	<-h.closeRelease
	return nil
}

func (h *blockingLifecycleHook) closeCallCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.closeCalls
}

func (h *blockingLifecycleHook) overlapCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.overlaps
}

func (h *sequencedCloseHook) Start(context.Context) error { return nil }

func (h *sequencedCloseHook) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closeCalls++
	if len(h.closeErrs) == 0 {
		return nil
	}
	err := h.closeErrs[0]
	h.closeErrs = h.closeErrs[1:]
	return err
}

func (h *sequencedCloseHook) closeCallCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.closeCalls
}

type fakeEvaluator struct {
	value      any
	contextID  int
	expression string
}

type blockingEvaluator struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

type blockingResultEvaluator struct {
	entered chan struct{}
	release chan struct{}
	value   any
	err     error
	once    sync.Once
}

type concurrentSetupEvaluator struct {
	mu           sync.Mutex
	calls        int
	firstEntered chan struct{}
	firstRelease chan struct{}
	firstValue   any
	firstErr     error
	secondValue  any
	secondErr    error
}

type fakeRuntimeEventClient struct {
	sends       []fakeRuntimeEventSend
	handler     func(string, map[string]any)
	sendErr     error
	evaluateErr error
	contextID   int
	expression  string
}

type fakeClosableEvaluator struct {
	closed chan struct{}
}

type callbackStopEvaluator struct {
	closeRequested   chan struct{}
	callbackReturned chan struct{}
	closeOnce        sync.Once
	callbackOnce     sync.Once
}

type ownerCloseEvaluator struct {
	called      chan struct{}
	release     chan struct{}
	requested   chan struct{}
	once        sync.Once
	requestOnce sync.Once
	mu          sync.Mutex
	calls       int
}

type blockingCloseOnlyEvaluator struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

type closeCounter struct {
	mu    sync.Mutex
	calls int
}
type uncomparableCloseEvaluator struct {
	values  []int
	counter *closeCounter
}
type retryOwnerEvaluator struct {
	errs  []error
	calls int
}

type blockingRuntimeClient struct {
	connectEntered chan struct{}
	connectRelease chan struct{}
	connectOnce    sync.Once
	sendEntered    chan struct{}
	sendOnce       sync.Once
	blockSend      bool
	closed         chan struct{}
	closeOnce      sync.Once
	mu             sync.Mutex
	closeWaitCalls int
	contexts       []cdp.ExecutionContext
	evaluateValue  any
	evaluateValues map[int]any
	evaluatedIDs   []int
}

func newBlockingRuntimeClient(connectReady bool, blockSend bool) *blockingRuntimeClient {
	client := &blockingRuntimeClient{
		connectEntered: make(chan struct{}),
		connectRelease: make(chan struct{}),
		sendEntered:    make(chan struct{}),
		blockSend:      blockSend,
		closed:         make(chan struct{}),
	}
	if connectReady {
		close(client.connectRelease)
	}
	return client
}

func newReadyRuntimeClient() *blockingRuntimeClient {
	client := newBlockingRuntimeClient(true, false)
	client.contexts = []cdp.ExecutionContext{{ID: 7, Name: "gameContext"}}
	client.evaluateValue = map[string]any{"hasCc": true, "hasGameCtl": true, "hasCanvas": true}
	return client
}

type fakeRuntimeEventSend struct {
	method string
	params map[string]any
}

func (f *fakeRuntimeEventClient) Send(_ context.Context, method string, params map[string]any, _ time.Duration) (map[string]any, error) {
	f.sends = append(f.sends, fakeRuntimeEventSend{method: method, params: params})
	return map[string]any{}, f.sendErr
}

func (f *fakeRuntimeEventClient) Evaluate(_ context.Context, expression string, contextID int, _ time.Duration) (any, error) {
	f.expression = expression
	f.contextID = contextID
	return nil, f.evaluateErr
}

func (f *fakeRuntimeEventClient) OnEvent(handler func(string, map[string]any)) func() {
	f.handler = handler
	return func() { f.handler = nil }
}

func (f *fakeClosableEvaluator) Evaluate(context.Context, string, int, time.Duration) (any, error) {
	return nil, nil
}

func (f *fakeClosableEvaluator) Close() {
	close(f.closed)
}

func (f *callbackStopEvaluator) Evaluate(context.Context, string, int, time.Duration) (any, error) {
	return nil, nil
}

func (f *callbackStopEvaluator) Close() {
	f.closeOnce.Do(func() { close(f.closeRequested) })
}

func (f *callbackStopEvaluator) WaitClosed() {
	<-f.callbackReturned
}

func (f *callbackStopEvaluator) signalCallbackReturned() {
	f.callbackOnce.Do(func() { close(f.callbackReturned) })
}

func (f *ownerCloseEvaluator) Evaluate(context.Context, string, int, time.Duration) (any, error) {
	return nil, nil
}

func (f *ownerCloseEvaluator) Close() {
	if f.requested != nil {
		f.requestOnce.Do(func() { close(f.requested) })
	}
}

func (f *ownerCloseEvaluator) CloseAndWaitContext(ctx context.Context) error {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	f.once.Do(func() { close(f.called) })
	select {
	case <-f.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *blockingCloseOnlyEvaluator) Evaluate(context.Context, string, int, time.Duration) (any, error) {
	return nil, nil
}

func (e *blockingCloseOnlyEvaluator) Close() {
	e.once.Do(func() { close(e.entered) })
	<-e.release
}

func (e uncomparableCloseEvaluator) Evaluate(context.Context, string, int, time.Duration) (any, error) {
	return nil, nil
}
func (e uncomparableCloseEvaluator) Close() {
	e.counter.mu.Lock()
	e.counter.calls++
	e.counter.mu.Unlock()
}
func (e *retryOwnerEvaluator) Evaluate(context.Context, string, int, time.Duration) (any, error) {
	return nil, nil
}
func (e *retryOwnerEvaluator) Close() {}
func (e *retryOwnerEvaluator) CloseAndWaitContext(context.Context) error {
	e.calls++
	if len(e.errs) == 0 {
		return nil
	}
	err := e.errs[0]
	e.errs = e.errs[1:]
	return err
}

func (c *blockingRuntimeClient) Connect(context.Context, string) error {
	c.connectOnce.Do(func() { close(c.connectEntered) })
	<-c.connectRelease
	return nil
}

func (c *blockingRuntimeClient) Send(ctx context.Context, _ string, _ map[string]any, _ time.Duration) (map[string]any, error) {
	c.sendOnce.Do(func() { close(c.sendEntered) })
	if c.blockSend {
		<-ctx.Done()
	}
	return nil, ctx.Err()
}

func (c *blockingRuntimeClient) OnEvent(func(string, map[string]any)) func() {
	return func() {}
}

func (c *blockingRuntimeClient) Evaluate(_ context.Context, _ string, contextID int, _ time.Duration) (any, error) {
	c.mu.Lock()
	c.evaluatedIDs = append(c.evaluatedIDs, contextID)
	value, configured := c.evaluateValues[contextID]
	fallback := c.evaluateValue
	c.mu.Unlock()
	if configured {
		return value, nil
	}
	return fallback, nil
}

func (c *blockingRuntimeClient) Contexts() []cdp.ExecutionContext {
	return append([]cdp.ExecutionContext(nil), c.contexts...)
}

func (c *blockingRuntimeClient) Close() { c.signalClosed() }

func (c *blockingRuntimeClient) CloseAndWait() {
	c.mu.Lock()
	c.closeWaitCalls++
	c.mu.Unlock()
	c.signalClosed()
}

func (c *blockingRuntimeClient) CloseAndWaitContext(context.Context) error {
	c.CloseAndWait()
	return nil
}

func (c *blockingRuntimeClient) signalClosed() {
	c.closeOnce.Do(func() { close(c.closed) })
}

func (c *blockingRuntimeClient) closeWaitCallCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeWaitCalls
}

func (c *blockingRuntimeClient) evaluatedContextIDs() []int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]int(nil), c.evaluatedIDs...)
}

func (b *rapidStartBridge) Start(context.Context) error {
	b.mu.Lock()
	b.calls++
	call := b.calls
	b.mu.Unlock()
	if call == 1 {
		close(b.firstEntered)
		<-b.firstRelease
		return nil
	}
	return errors.New("second start failed")
}

func (b *replacementStartBridge) Start(context.Context) error {
	b.mu.Lock()
	b.calls++
	call := b.calls
	b.mu.Unlock()
	if call == 1 {
		close(b.firstEntered)
		<-b.firstRelease
	}
	b.mu.Lock()
	b.running = true
	b.mu.Unlock()
	return nil
}

func (b *replacementPartialCloseBridge) Start(context.Context) error {
	b.mu.Lock()
	b.startCalls++
	call := b.startCalls
	b.mu.Unlock()
	if call == 1 {
		b.firstStartOnce.Do(func() { close(b.firstStartEntered) })
		<-b.firstStartRelease
		return nil
	}
	b.replacementStartOnce.Do(func() { close(b.replacementStartEntered) })
	return nil
}

func (b *replacementPartialCloseBridge) Close() error {
	b.mu.Lock()
	b.closeCalls++
	call := b.closeCalls
	err := b.firstCloseErr
	b.mu.Unlock()
	if call == 1 {
		return err
	}
	b.retryCloseOnce.Do(func() { close(b.retryCloseEntered) })
	<-b.retryCloseRelease
	return nil
}

func (b *replacementPartialCloseBridge) closeCallCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closeCalls
}

func (b *replacementStartBridge) Close() error {
	b.mu.Lock()
	b.running = false
	b.mu.Unlock()
	return nil
}

func (f *fakeEvaluator) Evaluate(ctx context.Context, expression string, contextID int, timeout time.Duration) (any, error) {
	f.contextID = contextID
	f.expression = expression
	return f.value, nil
}

func (f *blockingEvaluator) Evaluate(context.Context, string, int, time.Duration) (any, error) {
	f.once.Do(func() { close(f.entered) })
	<-f.release
	return map[string]any{"hasCc": true}, nil
}

func (f *blockingResultEvaluator) Evaluate(context.Context, string, int, time.Duration) (any, error) {
	f.once.Do(func() { close(f.entered) })
	<-f.release
	return f.value, f.err
}

func (e *concurrentSetupEvaluator) Evaluate(context.Context, string, int, time.Duration) (any, error) {
	e.mu.Lock()
	e.calls++
	call := e.calls
	e.mu.Unlock()
	if call == 1 {
		close(e.firstEntered)
		<-e.firstRelease
		return e.firstValue, e.firstErr
	}
	return e.secondValue, e.secondErr
}
