package wmpf

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	farmruntime "Farm_Go/internal/runtime"
	"Farm_Go/internal/runtime/cdp"
)

const cdpLinkExecutionContextWait = 8 * time.Second
const defaultExecutionContextID = -1
const runtimeEventBindingName = "__qqFarmRuntimeEventBinding"

type CDPLinkConfig struct {
	DebugPort        int
	LegacyDebugPort  int
	CDPPort          int
	WMPFVersion      string
	WMPFPID          int
	FridaRoot        string
	FridaResources   fs.FS
	FridaPython      string
	FridaEnabled     bool
	ButtonScript     string
	ButtonScriptPath string
}

type CDPLinkDeps struct {
	Bridge        bridgeRunner
	HookLoader    hookLoader
	Evaluator     runtimeEvaluator
	ClientFactory func() runtimeConnectionClient
}

type bridgeRunner interface {
	Start(ctx context.Context) error
	Close() error
	State() DebugBridgeState
	CDPURL() string
	WaitJSContext(ctx context.Context, preferredName string, timeout time.Duration) (JSContext, error)
	ConnectJSContext(id string) error
}

type hookLoader interface {
	Start(ctx context.Context) error
}

type hookCloser interface{ Close() error }

type runtimeEvaluator interface {
	Evaluate(ctx context.Context, expression string, contextID int, timeout time.Duration) (any, error)
}

type runtimeEvaluatorCloser interface {
	Close()
}

type runtimeEvaluatorOwnerCloser interface {
	CloseAndWaitContext(context.Context) error
}

type runtimeEventClient interface {
	runtimeEvaluator
	Send(ctx context.Context, method string, params map[string]any, timeout time.Duration) (map[string]any, error)
	OnEvent(handler func(method string, params map[string]any)) func()
}

type runtimeConnectionClient interface {
	runtimeEventClient
	Connect(ctx context.Context, url string) error
	CloseAndWait()
	Contexts() []cdp.ExecutionContext
}

type CDPLink struct {
	profile   Profile
	cfg       CDPLinkConfig
	manager   *farmruntime.Manager
	bridge    bridgeRunner
	hook      hookLoader
	eval      runtimeEvaluator
	newClient func() runtimeConnectionClient

	mu                        sync.Mutex
	statusMu                  sync.Mutex
	startMu                   sync.Mutex
	context                   cdp.ExecutionContext
	contextGeneration         uint64
	cancel                    context.CancelFunc
	runtimeEventSeq           uint64
	runtimeEventHandlers      map[uint64]func(map[string]any)
	runtimeEventOrder         []uint64
	runtimeGeneration         uint64
	nextSetupToken            uint64
	pendingSetupToken         uint64
	pendingContextID          int
	pendingGeneration         uint64
	pendingSource             runtimeEvaluator
	closingEvaluators         []closingEvaluatorEntry
	nextEvaluatorID           uint64
	evaluatorID               uint64
	connectGeneration         uint64
	connectHandle             *runtimeConnectHandle
	connectHandles            map[*runtimeConnectHandle]struct{}
	desiredStopped            bool
	hookStopped               bool
	bridgeStopped             bool
	dependencyOwnerGeneration uint64
	cleanupHandle             *runtimeCleanupHandle
	ownerStopCount            int
	ownerStopDone             chan struct{}
}

type runtimeConnectHandle struct {
	generation        uint64
	done              chan struct{}
	invalidated       bool
	evaluator         runtimeEvaluator
	evaluatorID       uint64
	runtimeGeneration uint64
}

type closingEvaluatorEntry struct {
	id        uint64
	evaluator runtimeEvaluator
}

type runtimeCleanupHandle struct {
	done      chan struct{}
	completed bool
	result    dependencyCleanupResult
}

type dependencyCleanupResult struct {
	hookAttempted   bool
	hookErr         error
	bridgeAttempted bool
	bridgeErr       error
}

func (r dependencyCleanupResult) err() error {
	return errors.Join(r.hookErr, r.bridgeErr)
}

func NewCDPLink(profile Profile, cfg CDPLinkConfig, manager *farmruntime.Manager) *CDPLink {
	bridge := NewDebugBridge(DebugBridgeOptions{
		DebugPort:       cfg.DebugPort,
		LegacyDebugPort: cfg.LegacyDebugPort,
		CDPPort:         cfg.CDPPort,
	})
	deps := CDPLinkDeps{Bridge: bridge}
	if cfg.FridaEnabled {
		deps.HookLoader = NewFridaHookLoader(FridaHookLoaderOptions{
			Profile:           profile,
			Root:              cfg.FridaRoot,
			Resources:         cfg.FridaResources,
			Python:            cfg.FridaPython,
			DebugWebSocketURL: debugWebSocketURL(cfg.DebugPort),
		})
	}
	return NewCDPLinkWithDeps(profile, cfg, manager, deps)
}

func NewCDPLinkWithDeps(profile Profile, cfg CDPLinkConfig, manager *farmruntime.Manager, deps CDPLinkDeps) *CDPLink {
	if manager == nil {
		manager = farmruntime.NewManager()
	}
	if deps.Bridge == nil {
		deps.Bridge = NewDebugBridge(DebugBridgeOptions{
			DebugPort:       cfg.DebugPort,
			LegacyDebugPort: cfg.LegacyDebugPort,
			CDPPort:         cfg.CDPPort,
		})
	}
	link := &CDPLink{
		profile:              profile,
		cfg:                  cfg,
		manager:              manager,
		bridge:               deps.Bridge,
		hook:                 deps.HookLoader,
		eval:                 deps.Evaluator,
		newClient:            deps.ClientFactory,
		runtimeEventHandlers: make(map[uint64]func(map[string]any)),
		hookStopped:          true,
	}
	if link.newClient == nil {
		link.newClient = func() runtimeConnectionClient { return cdp.NewClient(8 * time.Second) }
	}
	if _, ok := deps.HookLoader.(hookCloser); ok {
		link.hookStopped = false
	}
	if deps.Evaluator != nil {
		link.nextEvaluatorID = 1
		link.evaluatorID = 1
	}
	return link
}

func (l *CDPLink) Target() farmruntime.RuntimeTarget {
	return l.profile.Target
}

func (l *CDPLink) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	handle, cleanup, previousCancel, autoConnect, err := l.beginStart(ctx, cancel)
	if err != nil {
		cancel()
		return err
	}
	if previousCancel != nil {
		previousCancel()
	}
	if err := l.acquireStartOperation(ctx, handle, cleanup); err != nil {
		cancel()
		return l.finishStartError(handle, err)
	}
	defer l.startMu.Unlock()

	if err = l.bridge.Start(ctx); err != nil {
		cancel()
		closeErr := l.closeDependenciesAfterFailedStart(handle)
		return l.finishStartError(handle, errors.Join(err, closeErr))
	}
	if l.hook != nil {
		if _, ok := l.hook.(hookCloser); ok {
			l.mu.Lock()
			l.hookStopped = false
			l.mu.Unlock()
		}
		if err = l.hook.Start(ctx); err != nil {
			cancel()
			closeErr := l.closeDependenciesAfterFailedStart(handle)
			return l.finishStartError(handle, errors.Join(err, closeErr))
		}
	}
	l.statusMu.Lock()
	l.mu.Lock()
	valid := l.connectHandle == handle && l.connectGeneration == handle.generation && !handle.invalidated && !l.desiredStopped
	l.mu.Unlock()
	if !valid {
		l.statusMu.Unlock()
		cancel()
		_ = l.closeDependenciesAfterFailedStart(handle)
		l.finishConnectHandle(handle)
		return errors.New("runtime start invalidated")
	}
	l.manager.SetStatus(farmruntime.Status{
		Target:    string(l.Target()),
		Phase:     farmruntime.PhaseListening,
		Connected: l.bridge.State().MiniappConnected,
	})
	l.statusMu.Unlock()
	if autoConnect {
		go func() {
			defer l.finishConnectHandle(handle)
			l.connectRuntimeWhenReady(runCtx, handle)
		}()
	} else {
		l.finishConnectHandle(handle)
	}
	return nil
}

func (l *CDPLink) beginStart(ctx context.Context, cancel context.CancelFunc) (*runtimeConnectHandle, *runtimeCleanupHandle, context.CancelFunc, bool, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, false, err
		}
		l.mu.Lock()
		ownerStopDone := l.ownerStopDone
		if ownerStopDone == nil {
			previousCancel := l.cancel
			if l.connectHandle != nil {
				l.connectHandle.invalidated = true
			}
			l.cancel = cancel
			l.connectGeneration++
			handle := &runtimeConnectHandle{generation: l.connectGeneration, done: make(chan struct{})}
			if l.connectHandles == nil {
				l.connectHandles = make(map[*runtimeConnectHandle]struct{})
			}
			l.connectHandles[handle] = struct{}{}
			l.connectHandle = handle
			cleanup := l.cleanupHandle
			autoConnect := l.eval == nil
			l.mu.Unlock()
			return handle, cleanup, previousCancel, autoConnect, nil
		}
		l.mu.Unlock()
		select {
		case <-ownerStopDone:
		case <-ctx.Done():
			return nil, nil, nil, false, ctx.Err()
		}
	}
}

// acquireStartOperation returns with startMu held. A Start that wins the mutex
// race against an older cleanup yields the gate and waits for that cleanup.
func (l *CDPLink) acquireStartOperation(ctx context.Context, handle *runtimeConnectHandle, cleanup *runtimeCleanupHandle) error {
	for {
		if cleanup != nil {
			if err := l.consumeDependencyCleanup(ctx, cleanup); err != nil {
				return err
			}
			cleanup = nil
		}
		l.mu.Lock()
		valid := l.connectHandle == handle && l.connectGeneration == handle.generation && !handle.invalidated
		cleanup = l.cleanupHandle
		startCleanup := false
		if valid && cleanup == nil && l.needsCleanupBeforeStartLocked(handle.generation) {
			cleanup = &runtimeCleanupHandle{done: make(chan struct{})}
			l.cleanupHandle = cleanup
			startCleanup = true
		}
		l.mu.Unlock()
		if !valid {
			return errors.New("runtime start invalidated")
		}
		if startCleanup {
			go l.runDependencyCleanup(cleanup)
		}
		if cleanup != nil {
			continue
		}

		l.startMu.Lock()
		l.mu.Lock()
		valid = l.connectHandle == handle && l.connectGeneration == handle.generation && !handle.invalidated
		cleanup = l.cleanupHandle
		if valid && cleanup == nil {
			l.desiredStopped = false
			l.bridgeStopped = false
			l.dependencyOwnerGeneration = handle.generation
			l.mu.Unlock()
			return nil
		}
		l.mu.Unlock()
		l.startMu.Unlock()
		if !valid {
			return errors.New("runtime start invalidated")
		}
	}
}

func (l *CDPLink) Stop(ctx context.Context) error {
	return l.stop(ctx, false)
}

func (l *CDPLink) StopAndWait(ctx context.Context) error {
	return l.stop(ctx, true)
}

func (l *CDPLink) stop(ctx context.Context, wait bool) error {
	l.mu.Lock()
	if wait {
		if l.ownerStopCount == 0 {
			l.ownerStopDone = make(chan struct{})
		}
		l.ownerStopCount++
	}
	cancel := l.cancel
	l.cancel = nil
	if !l.desiredStopped {
		l.connectGeneration++
	}
	stopGeneration := l.connectGeneration
	l.desiredStopped = true
	if l.connectHandle != nil {
		l.connectHandle.invalidated = true
	}
	handles := make([]*runtimeConnectHandle, 0, len(l.connectHandles))
	for handle := range l.connectHandles {
		handles = append(handles, handle)
	}
	cleanup := l.cleanupHandle
	startCleanup := false
	if cleanup == nil && !l.dependenciesStoppedLocked() {
		cleanup = &runtimeCleanupHandle{done: make(chan struct{})}
		l.cleanupHandle = cleanup
		startCleanup = true
	}
	l.mu.Unlock()
	if wait {
		defer l.finishOwnerStop()
	}
	if startCleanup {
		go l.runDependencyCleanup(cleanup)
	}
	if cancel != nil {
		cancel()
	}
	var firstErr error
	if err := l.clearRuntimeConnectionForStop(ctx, false, stopGeneration); err != nil && firstErr == nil {
		firstErr = err
	}
	if wait {
		for _, handle := range handles {
			select {
			case <-handle.done:
			case <-ctx.Done():
				if firstErr == nil {
					firstErr = ctx.Err()
				}
			}
		}
		if err := l.drainClosingEvaluators(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		if cleanup != nil {
			if err := l.consumeDependencyCleanup(ctx, cleanup); err != nil && firstErr == nil {
				firstErr = err
			}
		} else if err := l.waitForDependencyCleanup(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	l.statusMu.Lock()
	l.mu.Lock()
	publishDisconnected := l.connectGeneration == stopGeneration && l.desiredStopped
	l.mu.Unlock()
	if publishDisconnected {
		l.manager.SetStatus(farmruntime.Status{
			Target: string(l.Target()),
			Phase:  farmruntime.PhaseDisconnected,
		})
	}
	l.statusMu.Unlock()
	return firstErr
}

func (l *CDPLink) runDependencyCleanup(handle *runtimeCleanupHandle) {
	l.startMu.Lock()
	result := l.closeOwnedDependencies()
	l.startMu.Unlock()
	l.mu.Lock()
	if l.cleanupHandle == handle {
		l.applyDependencyCleanupResultLocked(result)
	}
	handle.result = result
	handle.completed = true
	close(handle.done)
	l.mu.Unlock()
}

func (l *CDPLink) closeDependenciesAfterFailedStart(handle *runtimeConnectHandle) error {
	l.mu.Lock()
	cleanupOwnsClose := l.cleanupHandle != nil || l.desiredStopped
	l.mu.Unlock()
	if cleanupOwnsClose {
		return nil
	}
	result := l.closeOwnedDependencies()
	l.mu.Lock()
	l.applyDependencyCleanupResultLocked(result)
	if l.connectHandle == handle && l.connectGeneration == handle.generation && !handle.invalidated {
		l.desiredStopped = true
	}
	l.mu.Unlock()
	return result.err()
}

func (l *CDPLink) closeOwnedDependencies() dependencyCleanupResult {
	l.mu.Lock()
	closeHook := !l.hookStopped
	closeBridge := !l.bridgeStopped
	l.mu.Unlock()
	var result dependencyCleanupResult
	if closeHook {
		result.hookAttempted = true
		if closer, ok := l.hook.(hookCloser); ok {
			result.hookErr = closer.Close()
		}
	}
	if closeBridge {
		result.bridgeAttempted = true
		result.bridgeErr = l.bridge.Close()
	}
	return result
}

func (l *CDPLink) waitForDependencyCleanup(ctx context.Context) error {
	l.mu.Lock()
	cleanup := l.cleanupHandle
	l.mu.Unlock()
	if cleanup == nil {
		return nil
	}
	return l.consumeDependencyCleanup(ctx, cleanup)
}

func (l *CDPLink) consumeDependencyCleanup(ctx context.Context, handle *runtimeCleanupHandle) error {
	select {
	case <-handle.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	l.mu.Lock()
	err := handle.result.err()
	if l.cleanupHandle == handle && handle.completed {
		l.cleanupHandle = nil
	}
	l.mu.Unlock()
	return err
}

func (l *CDPLink) applyDependencyCleanupResultLocked(result dependencyCleanupResult) {
	if result.hookAttempted && result.hookErr == nil {
		l.hookStopped = true
	}
	if result.bridgeAttempted && result.bridgeErr == nil {
		l.bridgeStopped = true
	}
	if l.dependenciesStoppedLocked() {
		l.dependencyOwnerGeneration = 0
	}
}

func (l *CDPLink) dependenciesStoppedLocked() bool {
	return l.hookStopped && l.bridgeStopped
}

func (l *CDPLink) needsCleanupBeforeStartLocked(generation uint64) bool {
	if l.dependenciesStoppedLocked() {
		return false
	}
	return l.desiredStopped || l.dependencyOwnerGeneration != 0 && l.dependencyOwnerGeneration != generation
}

func (l *CDPLink) finishOwnerStop() {
	l.mu.Lock()
	l.ownerStopCount--
	if l.ownerStopCount != 0 {
		l.mu.Unlock()
		return
	}
	done := l.ownerStopDone
	l.ownerStopDone = nil
	l.mu.Unlock()
	close(done)
}

func (l *CDPLink) finishStartError(handle *runtimeConnectHandle, startErr error) error {
	l.statusMu.Lock()
	l.mu.Lock()
	valid := l.connectHandle == handle && l.connectGeneration == handle.generation && !handle.invalidated
	l.mu.Unlock()
	l.finishConnectHandle(handle)
	if valid {
		l.manager.SetStatus(farmruntime.Status{Target: string(l.Target()), Phase: farmruntime.PhaseError, LastError: startErr.Error()})
	}
	l.statusMu.Unlock()
	if !valid {
		return errors.New("runtime start invalidated")
	}
	return startErr
}

func (l *CDPLink) finishConnectHandle(handle *runtimeConnectHandle) {
	l.mu.Lock()
	delete(l.connectHandles, handle)
	if l.connectHandle == handle {
		l.connectHandle = nil
	}
	l.mu.Unlock()
	close(handle.done)
}

func (l *CDPLink) Status() farmruntime.Status {
	return l.manager.Status()
}

func (l *CDPLink) OnRuntimeEvent(handler func(map[string]any)) func() {
	if handler == nil {
		return func() {}
	}
	l.mu.Lock()
	l.runtimeEventSeq++
	id := l.runtimeEventSeq
	l.runtimeEventHandlers[id] = handler
	l.runtimeEventOrder = append(l.runtimeEventOrder, id)
	l.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			delete(l.runtimeEventHandlers, id)
			for index, orderedID := range l.runtimeEventOrder {
				if orderedID == id {
					copy(l.runtimeEventOrder[index:], l.runtimeEventOrder[index+1:])
					l.runtimeEventOrder = l.runtimeEventOrder[:len(l.runtimeEventOrder)-1]
					break
				}
			}
			l.mu.Unlock()
		})
	}
}

func (l *CDPLink) UpdateRuntimeContext(ctx context.Context, context cdp.ExecutionContext) error {
	if context.ID == 0 {
		return errors.New("execution context id is required")
	}
	l.mu.Lock()
	evaluator := l.eval
	generation := l.runtimeGeneration
	l.mu.Unlock()
	setupToken := l.beginRuntimeContextSetup(evaluator, generation, context.ID)
	if evaluator == nil || setupToken == 0 {
		return errors.New("runtime context setup invalidated")
	}
	defer l.abandonRuntimeContextSetup(evaluator, generation, context.ID, setupToken)
	if evaluator != nil {
		value, err := evaluator.Evaluate(ctx, runtimeContextProbeExpression(), context.ID, 8*time.Second)
		if err != nil {
			if !l.publishRuntimeStatusIfCurrent(evaluator, generation, context.ID, setupToken, farmruntime.Status{
				Target:    string(l.Target()),
				Phase:     farmruntime.PhaseError,
				LastError: err.Error(),
			}) {
				return errors.New("runtime context setup invalidated")
			}
			return err
		}
		probe := contextProbeFromValue(context, value)
		if _, ok := cdp.SelectBestProbe([]cdp.ContextProbe{probe}); !ok {
			err := errors.New("runtime context is not ready")
			if !l.publishRuntimeStatusIfCurrent(evaluator, generation, context.ID, setupToken, farmruntime.Status{
				Target:    string(l.Target()),
				Phase:     farmruntime.PhaseHandshaking,
				Connected: l.bridge.State().MiniappConnected,
				LastError: err.Error(),
			}) {
				return errors.New("runtime context setup invalidated")
			}
			return err
		}
	}
	if !l.commitRuntimeContext(evaluator, generation, setupToken, context) {
		return errors.New("runtime context setup invalidated")
	}
	return nil
}

func (l *CDPLink) publishRuntimeStatusIfCurrent(evaluator runtimeEvaluator, generation uint64, contextID int, setupToken uint64, status farmruntime.Status) bool {
	l.statusMu.Lock()
	defer l.statusMu.Unlock()
	l.mu.Lock()
	valid := l.runtimeGeneration == generation && l.eval == evaluator &&
		l.pendingSetupToken == setupToken && l.pendingGeneration == generation && l.pendingSource == evaluator && l.pendingContextID == contextID
	l.mu.Unlock()
	if !valid {
		return false
	}
	l.manager.SetStatus(status)
	return true
}

func (l *CDPLink) markRuntimeReady(context cdp.ExecutionContext) {
	l.manager.SetStatus(farmruntime.Status{
		Target:      string(l.Target()),
		Phase:       farmruntime.PhaseReady,
		Connected:   l.bridge.State().MiniappConnected,
		Ready:       true,
		InstanceID:  context.Name,
		HostVersion: l.hostVersion(),
		LastSeenAt:  time.Now().Format(time.RFC3339),
	})
}

func (l *CDPLink) OnMiniappDisconnected(err error) {
	_ = l.clearRuntimeConnection(context.Background(), true)
	lastError := ""
	if err != nil {
		lastError = err.Error()
	}
	l.manager.SetStatus(farmruntime.Status{
		Target:    string(l.Target()),
		Phase:     farmruntime.PhaseDisconnected,
		LastError: lastError,
	})
}

func (l *CDPLink) onMiniappDisconnectedForConnect(handle *runtimeConnectHandle, disconnectErr error) {
	l.statusMu.Lock()
	l.mu.Lock()
	valid := l.connectHandle == handle && l.connectGeneration == handle.generation && !handle.invalidated && !l.desiredStopped
	if !valid {
		l.mu.Unlock()
		l.statusMu.Unlock()
		return
	}
	evaluator := handle.evaluator
	evaluatorID := handle.evaluatorID
	generation := handle.runtimeGeneration
	pending := false
	if evaluator != nil && l.eval == evaluator && l.evaluatorID == evaluatorID && l.runtimeGeneration == generation {
		l.eval = nil
		l.evaluatorID = 0
		l.context = cdp.ExecutionContext{}
		l.contextGeneration = 0
		l.pendingSetupToken = 0
		l.pendingContextID = 0
		l.pendingGeneration = 0
		l.pendingSource = nil
		l.runtimeGeneration++
		if !containsEvaluatorEntry(l.closingEvaluators, evaluatorID) {
			l.closingEvaluators = append(l.closingEvaluators, closingEvaluatorEntry{id: evaluatorID, evaluator: evaluator})
		}
		pending = true
	}
	handle.evaluator = nil
	handle.evaluatorID = 0
	handle.runtimeGeneration = 0
	l.mu.Unlock()
	lastError := ""
	if disconnectErr != nil {
		lastError = disconnectErr.Error()
	}
	l.manager.SetStatus(farmruntime.Status{
		Target:    string(l.Target()),
		Phase:     farmruntime.PhaseDisconnected,
		LastError: lastError,
	})
	l.statusMu.Unlock()
	if pending {
		_ = l.processClosingEvaluatorEntries(context.Background(), true, []closingEvaluatorEntry{{id: evaluatorID, evaluator: evaluator}})
	}
}

func (l *CDPLink) OnMiniappConnected() {
	l.manager.SetStatus(farmruntime.Status{
		Target:    string(l.Target()),
		Phase:     farmruntime.PhaseHandshaking,
		Connected: true,
	})
}

func (l *CDPLink) Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error) {
	switch method {
	case "host.describe":
		return l.describeHost(), nil
	case "gameCtl.probe":
		evaluator := l.currentEvaluator()
		if evaluator == nil {
			return nil, errors.New("runtime evaluator is not connected")
		}
		l.mu.Lock()
		context := l.context
		l.mu.Unlock()
		if context.ID == 0 {
			return nil, errors.New("execution context is not ready")
		}
		return evaluator.Evaluate(ctx, probeExpression(nil), evaluateContextID(context.ID), timeout)
	}

	if strings.HasPrefix(method, "gameCtl.") {
		evaluator := l.currentEvaluator()
		if evaluator == nil {
			return nil, errors.New("runtime evaluator is not connected")
		}
		l.mu.Lock()
		context := l.context
		l.mu.Unlock()
		if context.ID == 0 {
			return nil, errors.New("execution context is not ready")
		}
		expression, err := gameCtlCallExpression(method, args)
		if err != nil {
			return nil, err
		}
		return evaluator.Evaluate(ctx, expression, evaluateContextID(context.ID), timeout)
	}

	return nil, errors.New("unsupported runtime method")
}

func (l *CDPLink) describeHost() map[string]any {
	l.mu.Lock()
	context := l.context
	l.mu.Unlock()

	return map[string]any{
		"target":          string(l.Target()),
		"profile":         l.profile.Name,
		"status":          l.manager.Status(),
		"transport":       l.bridge.State(),
		"cdpURL":          l.bridge.CDPURL(),
		"wmpfVersion":     l.cfg.WMPFVersion,
		"wmpfPID":         l.cfg.WMPFPID,
		"selectedContext": context,
	}
}

func (l *CDPLink) hostVersion() string {
	if l.cfg.WMPFVersion != "" {
		return l.cfg.WMPFVersion
	}
	if cdpURL := l.bridge.CDPURL(); cdpURL != "" {
		return "CDP " + cdpURL
	}
	return string(l.Target()) + " CDP"
}

func (l *CDPLink) connectRuntimeWhenReady(ctx context.Context, handle *runtimeConnectHandle) {
	for ctx.Err() == nil {
		if !l.isCurrentConnectHandle(handle) {
			return
		}
		state := l.bridge.State()
		if !state.MiniappConnected {
			if l.hasRuntimeConnectionForHandle(handle) {
				l.onMiniappDisconnectedForConnect(handle, errors.New("miniapp disconnected"))
			}
			sleepContext(ctx, 100*time.Millisecond)
			continue
		}

		if l.hasRuntimeConnectionForHandle(handle) {
			sleepContext(ctx, 100*time.Millisecond)
			continue
		}

		if !l.publishConnectStatus(handle, farmruntime.Status{
			Target:    string(l.Target()),
			Phase:     farmruntime.PhaseHandshaking,
			Connected: true,
		}) {
			return
		}
		if err := l.connectRuntimeOnce(ctx, handle); err != nil {
			if ctx.Err() != nil {
				return
			}
			phase := farmruntime.PhaseError
			if isRuntimeReadinessPendingError(err) {
				phase = farmruntime.PhaseHandshaking
			}
			if !l.publishConnectStatus(handle, farmruntime.Status{
				Target:    string(l.Target()),
				Phase:     phase,
				Connected: l.bridge.State().MiniappConnected,
				LastError: err.Error(),
			}) {
				return
			}
			sleepContext(ctx, 250*time.Millisecond)
		}
	}
}

func (l *CDPLink) isCurrentConnectHandle(handle *runtimeConnectHandle) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.connectHandle == handle && l.connectGeneration == handle.generation && !handle.invalidated && !l.desiredStopped
}

func (l *CDPLink) hasRuntimeConnectionForHandle(handle *runtimeConnectHandle) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return handle.evaluator != nil
}

func (l *CDPLink) publishConnectStatus(handle *runtimeConnectHandle, status farmruntime.Status) bool {
	l.statusMu.Lock()
	defer l.statusMu.Unlock()
	l.mu.Lock()
	valid := l.connectHandle == handle && l.connectGeneration == handle.generation && !handle.invalidated && !l.desiredStopped
	l.mu.Unlock()
	if !valid {
		return false
	}
	l.manager.SetStatus(status)
	return true
}

func (l *CDPLink) connectRuntimeOnce(ctx context.Context, handle *runtimeConnectHandle) (err error) {
	jsContext, directFallback, err := l.waitForJSContextOrSetupContext(ctx, "gameContext", cdpLinkExecutionContextWait)
	if err != nil {
		return err
	}
	if !directFallback {
		if err := l.bridge.ConnectJSContext(jsContext.ID); err != nil {
			return err
		}
	} else if l.bridge.State().MiniappConnected {
		if !l.publishConnectStatus(handle, farmruntime.Status{
			Target:         string(l.Target()),
			Phase:          farmruntime.PhaseHandshaking,
			Connected:      true,
			ProgressDetail: "miniapp setupContext received; using direct CDP Runtime.enable fallback",
		}) {
			return errors.New("runtime connect invalidated")
		}
	}

	client := l.newClient()
	if err := client.Connect(ctx, l.bridge.CDPURL()); err != nil {
		client.CloseAndWait()
		return err
	}
	generation, evaluatorID, installed := l.installEvaluatorForConnect(client, handle)
	if !installed {
		client.CloseAndWait()
		return errors.New("runtime connect invalidated")
	}
	defer func() {
		if err != nil {
			_ = l.detachRuntimeEvaluatorOwned(context.Background(), true, client, evaluatorID, generation)
			l.clearConnectHandleEvaluator(handle, client, evaluatorID, generation)
		}
	}()
	if err := l.registerRuntimeEventBinding(ctx, client, generation); err != nil {
		return err
	}
	// Block TSDK telemetry at the network layer before the game context is ready.
	// Non-fatal: WeChat CDP may run on an older version that doesn't support Fetch domain.
	if fetchErr := l.enableFetchInterception(ctx, client, generation); fetchErr != nil {
		slog.Warn("[TSDK-BLOCK] Fetch interception unavailable", "target", l.Target(), "err", fetchErr)
	}
	context, err := l.waitForGameCtlExecutionContext(ctx, client, "gameContext", cdpLinkExecutionContextWait)
	if err != nil {
		return err
	}
	setupToken := l.beginRuntimeContextSetupForConnect(handle, client, generation, context.ID)
	if setupToken == 0 {
		return errors.New("runtime context setup invalidated")
	}
	if err := l.ensureGameCtlReady(ctx, client, context.ID, cdpLinkExecutionContextWait); err != nil {
		return err
	}
	if err := l.installRuntimeEventWrapper(ctx, client, evaluateContextID(context.ID), cdpLinkExecutionContextWait); err != nil {
		return err
	}
	if !l.commitRuntimeContextForConnect(handle, client, generation, setupToken, context) {
		return errors.New("runtime context setup invalidated")
	}
	return nil
}

func (l *CDPLink) registerRuntimeEventBinding(ctx context.Context, client runtimeEventClient, generation uint64) error {
	client.OnEvent(func(method string, params map[string]any) {
		l.handleCDPEventFrom(generation, method, params)
	})
	_, err := client.Send(ctx, "Runtime.addBinding", map[string]any{
		"name": runtimeEventBindingName,
	}, 8*time.Second)
	return err
}

// tsdkHosts lists all known Tencent security / telemetry domains that TSDK
// uses for data collection and anti-cheat uploads.  Extend this list whenever
// a new reporting endpoint is observed in the wild.
var tsdkHosts = []string{
	"anticheatexpert",
	"tss.qq.com",
	"btrace.qq.com",
	"beacon.qq.com",
	"bcs.qq.com",
	"aegis.qq.com",
	"hisvc.qq.com",
	"speed.qq.com",
	"trace.qq.com",
}

// isTsdkURL reports whether url is a TSDK / Tencent security telemetry endpoint.
func isTsdkURL(url string) bool {
	for _, h := range tsdkHosts {
		if strings.Contains(url, h) {
			return true
		}
	}
	return false
}

// enableFetchInterception registers a CDP Fetch-domain interceptor that blocks
// TSDK telemetry uploads at the network level for all known reporting domains.
// This must be called before waitForGameCtlExecutionContext so it catches
// requests that happen before button.js is injected.
// A failure is non-fatal: it is logged and the connect sequence continues.
func (l *CDPLink) enableFetchInterception(ctx context.Context, client runtimeEventClient, generation uint64) error {
	// Listen for Fetch.requestPaused events. We hold the client reference in the
	// closure so we can respond immediately without needing a stored reference.
	client.OnEvent(func(method string, params map[string]any) {
		if method != "Fetch.requestPaused" {
			return
		}
		if generation != 0 && !l.isCurrentRuntimeGeneration(generation) {
			return
		}
		requestID := stringFromMap(params, "requestId")
		if requestID == "" {
			return
		}
		// Extract URL from nested "request" object.
		var requestURL string
		if req, ok := params["request"].(map[string]any); ok {
			requestURL = stringFromMap(req, "url")
		}
		replyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if isTsdkURL(requestURL) {
			slog.Info("[TSDK-BLOCK] CDP Fetch intercepted", "target", l.Target(), "url", requestURL)
			// Return a fake 200 JSON response so TSDK treats the upload as successful
			// and does not retry or flag the environment as abnormal.
			fakeBody := base64.StdEncoding.EncodeToString([]byte("{}"))
			_, _ = client.Send(replyCtx, "Fetch.fulfillRequest", map[string]any{
				"requestId":    requestID,
				"responseCode": 200,
				"responseHeaders": []map[string]any{
					{"name": "Content-Type", "value": "application/json"},
					{"name": "Content-Length", "value": "2"},
				},
				"body": fakeBody,
			}, 5*time.Second)
			// Forward a tsdkBlock event so it appears in the Farm_Go log center.
			l.forwardRuntimeEvent(map[string]any{
				"name":          "tsdkBlock",
				"level":         "warn",
				"message":       "[TSDK-BLOCK] upload blocked via CDP Fetch",
				"extra":         map[string]any{"url": requestURL},
				"runtimeTarget": string(l.Target()),
			})
		} else {
			// Let all other requests through.
			_, _ = client.Send(replyCtx, "Fetch.continueRequest", map[string]any{
				"requestId": requestID,
			}, 5*time.Second)
		}
	})
	// Enable the Fetch domain for all known TSDK / Tencent telemetry patterns.
	tsdkPatterns := make([]map[string]any, 0, len(tsdkHosts))
	for _, h := range tsdkHosts {
		tsdkPatterns = append(tsdkPatterns, map[string]any{
			"urlPattern":   "*" + h + "*",
			"requestStage": "Request",
		})
	}
	_, err := client.Send(ctx, "Fetch.enable", map[string]any{
		"patterns": tsdkPatterns,
	}, 8*time.Second)
	// Emit an init event so the frontend indicator can reflect the real state.
	if err == nil {
		l.forwardRuntimeEvent(map[string]any{
			"name":          "tsdkBlock",
			"level":         "info",
			"message":       "[TSDK-BLOCK] Fetch interception enabled",
			"extra":         map[string]any{"status": "init_ok"},
			"runtimeTarget": string(l.Target()),
		})
	} else {
		l.forwardRuntimeEvent(map[string]any{
			"name":          "tsdkBlock",
			"level":         "error",
			"message":       "[TSDK-BLOCK] Fetch interception failed: " + err.Error(),
			"extra":         map[string]any{"status": "init_err"},
			"runtimeTarget": string(l.Target()),
		})
	}
	return err
}

func (l *CDPLink) installRuntimeEventWrapper(ctx context.Context, client runtimeEvaluator, contextID int, timeout time.Duration) error {
	_, err := client.Evaluate(ctx, runtimeEventBridgeExpression(), contextID, timeout)
	return err
}

func (l *CDPLink) handleCDPEvent(method string, params map[string]any) {
	l.handleCDPEventFrom(0, method, params)
}

func (l *CDPLink) handleCDPEventFrom(generation uint64, method string, params map[string]any) {
	switch method {
	case "Runtime.executionContextDestroyed":
		l.invalidateRuntimeConnectionForEvent(generation, intFromMap(params, "executionContextId"), false)
		return
	case "Runtime.executionContextsCleared":
		l.invalidateRuntimeConnectionForEvent(generation, 0, true)
		return
	}
	if generation != 0 && !l.isCurrentRuntimeGeneration(generation) {
		return
	}
	if method != "Runtime.bindingCalled" || stringFromMap(params, "name") != runtimeEventBindingName {
		return
	}
	payload := stringFromMap(params, "payload")
	var event map[string]any
	if payload == "" || json.Unmarshal([]byte(payload), &event) != nil || event == nil {
		return
	}
	if _, ok := event["runtimeTarget"]; !ok {
		event["runtimeTarget"] = string(l.Target())
	}
	l.forwardRuntimeEvent(event)
}

func (l *CDPLink) invalidateRuntimeConnection() {
	l.invalidateRuntimeConnectionForEvent(0, 0, true)
}

func (l *CDPLink) invalidateRuntimeConnectionForEvent(generation uint64, destroyedContextID int, clearAll bool) {
	l.statusMu.Lock()
	l.mu.Lock()
	if generation != 0 && (l.runtimeGeneration != generation || l.eval == nil) {
		l.mu.Unlock()
		l.statusMu.Unlock()
		return
	}
	if !clearAll {
		committedMatch := l.context.ID == destroyedContextID && l.contextGeneration == l.runtimeGeneration
		pendingMatch := l.pendingSetupToken != 0 && l.pendingSource == l.eval &&
			l.pendingGeneration == l.runtimeGeneration && l.pendingContextID == destroyedContextID
		if !committedMatch && !pendingMatch {
			l.mu.Unlock()
			l.statusMu.Unlock()
			return
		}
	}
	evaluator := l.eval
	evaluatorID := l.evaluatorID
	runtimeGeneration := l.runtimeGeneration
	entry, pending, detached := l.detachCurrentEvaluatorLocked(evaluator, evaluatorID, runtimeGeneration)
	l.mu.Unlock()
	if detached {
		l.manager.SetStatus(farmruntime.Status{
			Target:    string(l.Target()),
			Phase:     farmruntime.PhaseHandshaking,
			Connected: l.bridge.State().MiniappConnected,
		})
	}
	l.statusMu.Unlock()
	if pending {
		_ = l.processClosingEvaluatorEntries(context.Background(), false, []closingEvaluatorEntry{entry})
	}
}

func (l *CDPLink) beginRuntimeContextSetup(source runtimeEvaluator, generation uint64, contextID int) uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if source == nil || l.runtimeGeneration != generation || l.eval != source || contextID == 0 {
		return 0
	}
	l.nextSetupToken++
	l.pendingSetupToken = l.nextSetupToken
	l.pendingContextID = contextID
	l.pendingGeneration = generation
	l.pendingSource = source
	return l.pendingSetupToken
}

func (l *CDPLink) beginRuntimeContextSetupForConnect(handle *runtimeConnectHandle, source runtimeEvaluator, generation uint64, contextID int) uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if source == nil || l.connectHandle != handle || l.connectGeneration != handle.generation || handle.invalidated || l.desiredStopped ||
		l.runtimeGeneration != generation || l.eval != source || contextID == 0 {
		return 0
	}
	l.nextSetupToken++
	l.pendingSetupToken = l.nextSetupToken
	l.pendingContextID = contextID
	l.pendingGeneration = generation
	l.pendingSource = source
	return l.pendingSetupToken
}

func (l *CDPLink) abandonRuntimeContextSetup(source runtimeEvaluator, generation uint64, contextID int, setupToken uint64) {
	l.mu.Lock()
	if l.pendingSetupToken == setupToken && l.pendingSource == source && l.pendingGeneration == generation && l.pendingContextID == contextID {
		l.pendingSetupToken = 0
		l.pendingSource = nil
		l.pendingGeneration = 0
		l.pendingContextID = 0
	}
	l.mu.Unlock()
}

func (l *CDPLink) commitRuntimeContext(source runtimeEvaluator, generation uint64, setupToken uint64, context cdp.ExecutionContext) bool {
	l.statusMu.Lock()
	defer l.statusMu.Unlock()
	l.mu.Lock()
	if l.runtimeGeneration != generation || l.eval != source ||
		l.pendingSetupToken != setupToken || l.pendingGeneration != generation || l.pendingSource != source || l.pendingContextID != context.ID {
		l.mu.Unlock()
		return false
	}
	l.context = context
	l.contextGeneration = generation
	l.pendingSetupToken = 0
	l.pendingContextID = 0
	l.pendingGeneration = 0
	l.pendingSource = nil
	l.mu.Unlock()
	l.manager.SetStatus(farmruntime.Status{
		Target:      string(l.Target()),
		Phase:       farmruntime.PhaseReady,
		Connected:   l.bridge.State().MiniappConnected,
		Ready:       true,
		InstanceID:  context.Name,
		HostVersion: l.hostVersion(),
		LastSeenAt:  time.Now().Format(time.RFC3339),
	})
	return true
}

func (l *CDPLink) commitRuntimeContextForConnect(handle *runtimeConnectHandle, source runtimeEvaluator, generation uint64, setupToken uint64, context cdp.ExecutionContext) bool {
	l.statusMu.Lock()
	defer l.statusMu.Unlock()
	l.mu.Lock()
	if l.connectHandle != handle || l.connectGeneration != handle.generation || handle.invalidated || l.desiredStopped ||
		l.runtimeGeneration != generation || l.eval != source ||
		l.pendingSetupToken != setupToken || l.pendingGeneration != generation || l.pendingSource != source || l.pendingContextID != context.ID {
		l.mu.Unlock()
		return false
	}
	l.context = context
	l.contextGeneration = generation
	l.pendingSetupToken = 0
	l.pendingContextID = 0
	l.pendingGeneration = 0
	l.pendingSource = nil
	l.mu.Unlock()
	l.markRuntimeReady(context)
	return true
}

func (l *CDPLink) isCurrentRuntimeGeneration(generation uint64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.runtimeGeneration == generation && l.eval != nil
}

func (l *CDPLink) forwardRuntimeEvent(event map[string]any) {
	l.mu.Lock()
	handlers := make([]func(map[string]any), 0, len(l.runtimeEventHandlers))
	for _, id := range l.runtimeEventOrder {
		if handler := l.runtimeEventHandlers[id]; handler != nil {
			handlers = append(handlers, handler)
		}
	}
	l.mu.Unlock()

	for _, handler := range handlers {
		l.invokeRuntimeEventHandler(handler, event)
	}
}

func (l *CDPLink) invokeRuntimeEventHandler(handler func(map[string]any), event map[string]any) {
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Warn("runtime event handler panicked",
				"transport", string(l.Target()),
				"event", runtimeEventDescription(event),
				"panic", fmt.Sprint(recovered),
			)
		}
	}()
	handler(farmruntime.CloneJSONMap(event))
}

func runtimeEventDescription(event map[string]any) string {
	for _, key := range []string{"name", "type", "kind"} {
		if value, ok := event[key].(string); ok && value != "" {
			return value
		}
	}
	return "unknown"
}

func runtimeEventBridgeExpression() string {
	return `globalThis.__qqFarmRuntimeEventBridge = function (event) {
  try {
    globalThis.__qqFarmRuntimeEventBinding(JSON.stringify(event));
    return true;
  } catch (_) {
    return false;
  }
};`
}

func (l *CDPLink) waitForGameCtlExecutionContext(ctx context.Context, client runtimeConnectionClient, preferredName string, timeout time.Duration) (cdp.ExecutionContext, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error = errors.New("execution context is not ready")
	lastProbeAt := map[int]time.Time{}
	startedAt := time.Now()
	for {
		contexts := orderedExecutionContexts(client.Contexts(), preferredName)
		probes := make([]cdp.ContextProbe, 0, len(contexts))
		for _, context := range contexts {
			probe, err := probeRuntimeContext(ctx, client, context, lastProbeAt)
			if err != nil {
				lastErr = err
			} else if probe.ID != 0 {
				probes = append(probes, probe)
			}
		}
		if time.Since(startedAt) >= 250*time.Millisecond {
			probe, err := probeRuntimeContext(ctx, client, cdp.ExecutionContext{ID: defaultExecutionContextID, Name: "default"}, lastProbeAt)
			if err != nil {
				lastErr = err
			} else if probe.ID != 0 {
				probes = append(probes, probe)
			}
		}
		if best, ok := cdp.SelectBestProbe(probes); ok {
			if best.ID == defaultExecutionContextID {
				return cdp.ExecutionContext{ID: defaultExecutionContextID, Name: "default", Origin: best.Origin}, nil
			}
			for _, context := range contexts {
				if context.ID == best.ID {
					return context, nil
				}
			}
		}
		if len(probes) > 0 {
			lastErr = errors.New("runtime context is not ready")
		}
		select {
		case <-ctx.Done():
			return cdp.ExecutionContext{}, ctx.Err()
		case <-deadline.C:
			return cdp.ExecutionContext{}, lastErr
		case <-ticker.C:
		}
	}
}

func probeRuntimeContext(ctx context.Context, client runtimeEvaluator, context cdp.ExecutionContext, lastProbeAt map[int]time.Time) (cdp.ContextProbe, error) {
	if lastProbe := lastProbeAt[context.ID]; !lastProbe.IsZero() && time.Since(lastProbe) < 500*time.Millisecond {
		return cdp.ContextProbe{}, nil
	}
	lastProbeAt[context.ID] = time.Now()
	value, err := client.Evaluate(ctx, runtimeContextProbeExpression(), evaluateContextID(context.ID), 3*time.Second)
	if err != nil {
		return cdp.ContextProbe{}, err
	}
	return contextProbeFromValue(context, value), nil
}

func (l *CDPLink) ensureGameCtlReady(ctx context.Context, client runtimeEvaluator, contextID int, timeout time.Duration) error {
	if contextID == 0 {
		return errors.New("execution context id is required")
	}
	evalContextID := evaluateContextID(contextID)
	source, err := l.buttonScriptSource()
	if err != nil {
		return err
	}
	if len(source) == 0 {
		return nil
	}
	hashBytes := sha1.Sum(source)
	scriptHash := hex.EncodeToString(hashBytes[:])
	requiredMethods := []string{"getPlayerProfile", "getFertilizerContainerStatus"}
	if probe, err := client.Evaluate(ctx, probeExpression(requiredMethods), evalContextID, 3*time.Second); err == nil && probeHasReadyGameCtl(probe, scriptHash, requiredMethods) {
		return nil
	}
	injection := fmt.Sprintf(`(async () => {
%s
; if (globalThis.gameCtl && typeof globalThis.gameCtl === "object") {
  globalThis.gameCtl.__scriptHash = %s;
}
; return { injected: true, scriptHash: %s };
})()`, string(source), mustJSON(scriptHash), mustJSON(scriptHash))
	if _, err := client.Evaluate(ctx, injection, evalContextID, timeout); err != nil {
		return err
	}
	probe, err := client.Evaluate(ctx, probeExpression(requiredMethods), evalContextID, 3*time.Second)
	if err != nil {
		return err
	}
	if !probeHasReadyGameCtl(probe, scriptHash, requiredMethods) {
		return errors.New("gameCtl_not_ready")
	}
	return nil
}

func evaluateContextID(contextID int) int {
	if contextID == defaultExecutionContextID {
		return 0
	}
	return contextID
}

func (l *CDPLink) buttonScriptPath() string {
	if l.cfg.ButtonScriptPath != "" {
		return l.cfg.ButtonScriptPath
	}
	if l.cfg.FridaRoot == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(l.cfg.FridaRoot), "button.js")
}

func (l *CDPLink) buttonScriptSource() ([]byte, error) {
	if l.cfg.ButtonScript != "" {
		return []byte(l.cfg.ButtonScript), nil
	}
	path := l.buttonScriptPath()
	if path == "" {
		return nil, nil
	}
	return os.ReadFile(path)
}

func orderedExecutionContexts(contexts []cdp.ExecutionContext, preferredName string) []cdp.ExecutionContext {
	ordered := append([]cdp.ExecutionContext(nil), contexts...)
	sort.SliceStable(ordered, func(i, j int) bool {
		leftPreferred := preferredName != "" && ordered[i].Name == preferredName
		rightPreferred := preferredName != "" && ordered[j].Name == preferredName
		if leftPreferred != rightPreferred {
			return leftPreferred
		}
		return ordered[i].ID < ordered[j].ID
	})
	return ordered
}

func (l *CDPLink) waitForJSContextOrSetupContext(ctx context.Context, preferredName string, timeout time.Duration) (JSContext, bool, error) {
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type waitResult struct {
		context JSContext
		err     error
	}
	result := make(chan waitResult, 1)
	go func() {
		context, err := l.bridge.WaitJSContext(waitCtx, preferredName, timeout)
		result <- waitResult{context: context, err: err}
	}()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case outcome := <-result:
			if outcome.err != nil {
				return JSContext{}, false, outcome.err
			}
			return outcome.context, false, nil
		case <-ticker.C:
			if bridgeSawRemoteDebugTraffic(l.bridge.State()) {
				return JSContext{}, true, nil
			}
		case <-deadline.C:
			return JSContext{}, false, fmt.Errorf("miniapp jscontext %q not found", preferredName)
		case <-ctx.Done():
			return JSContext{}, false, ctx.Err()
		}
	}
}

func bridgeSawMiniappSetupContext(state DebugBridgeState) bool {
	for i := len(state.RecentMiniappEvents) - 1; i >= 0; i-- {
		if state.RecentMiniappEvents[i].Category == "setupContext" {
			return true
		}
	}
	return false
}

func bridgeSawRemoteDebugTraffic(state DebugBridgeState) bool {
	if bridgeSawMiniappSetupContext(state) {
		return true
	}
	for i := len(state.RecentMiniappEvents) - 1; i >= 0; i-- {
		event := state.RecentMiniappEvents[i]
		if event.Category == "chromeDevtoolsResult" && strings.HasPrefix(event.Summary, "Runtime.") {
			return true
		}
	}
	return false
}

func (l *CDPLink) currentEvaluator() runtimeEvaluator {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.eval
}

func (l *CDPLink) setEvaluator(evaluator runtimeEvaluator) uint64 {
	l.statusMu.Lock()
	defer l.statusMu.Unlock()
	l.mu.Lock()
	generation, _ := l.installEvaluatorLocked(evaluator)
	l.mu.Unlock()
	return generation
}

func (l *CDPLink) installEvaluatorForConnect(evaluator runtimeEvaluator, handle *runtimeConnectHandle) (uint64, uint64, bool) {
	l.statusMu.Lock()
	defer l.statusMu.Unlock()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.connectHandle != handle || l.connectGeneration != handle.generation || handle.invalidated || l.desiredStopped {
		return 0, 0, false
	}
	if l.eval != nil && !containsEvaluatorEntry(l.closingEvaluators, l.evaluatorID) {
		l.closingEvaluators = append(l.closingEvaluators, closingEvaluatorEntry{id: l.evaluatorID, evaluator: l.eval})
	}
	generation, evaluatorID := l.installEvaluatorLocked(evaluator)
	handle.evaluator = evaluator
	handle.evaluatorID = evaluatorID
	handle.runtimeGeneration = generation
	return generation, evaluatorID, true
}

func (l *CDPLink) installEvaluatorLocked(evaluator runtimeEvaluator) (uint64, uint64) {
	oldEvaluator := l.eval
	if l.connectHandle != nil && l.connectHandle.evaluator == oldEvaluator {
		l.connectHandle.evaluator = nil
		l.connectHandle.evaluatorID = 0
		l.connectHandle.runtimeGeneration = 0
	}
	l.context = cdp.ExecutionContext{}
	l.contextGeneration = 0
	l.pendingSetupToken = 0
	l.pendingContextID = 0
	l.pendingGeneration = 0
	l.pendingSource = nil
	l.runtimeGeneration++
	l.nextEvaluatorID++
	l.evaluatorID = l.nextEvaluatorID
	l.eval = evaluator
	return l.runtimeGeneration, l.evaluatorID
}

func (l *CDPLink) clearConnectHandleEvaluator(handle *runtimeConnectHandle, evaluator runtimeEvaluator, evaluatorID uint64, generation uint64) {
	l.mu.Lock()
	if handle.evaluator == evaluator && handle.evaluatorID == evaluatorID && handle.runtimeGeneration == generation {
		handle.evaluator = nil
		handle.evaluatorID = 0
		handle.runtimeGeneration = 0
	}
	l.mu.Unlock()
}

func (l *CDPLink) hasRuntimeConnection() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.eval != nil || l.context.ID != 0
}

func (l *CDPLink) clearRuntimeConnection(ctx context.Context, wait bool) error {
	return l.clearRuntimeConnectionOwned(ctx, wait, 0, false)
}

func (l *CDPLink) clearRuntimeConnectionForStop(ctx context.Context, wait bool, stopGeneration uint64) error {
	return l.clearRuntimeConnectionOwned(ctx, wait, stopGeneration, true)
}

func (l *CDPLink) clearRuntimeConnectionOwned(ctx context.Context, wait bool, stopGeneration uint64, gated bool) error {
	l.statusMu.Lock()
	l.mu.Lock()
	if !gated || l.connectGeneration == stopGeneration && l.desiredStopped {
		evaluator := l.eval
		evaluatorID := l.evaluatorID
		l.eval = nil
		l.evaluatorID = 0
		l.context = cdp.ExecutionContext{}
		l.contextGeneration = 0
		l.pendingSetupToken = 0
		l.pendingContextID = 0
		l.pendingGeneration = 0
		l.pendingSource = nil
		l.runtimeGeneration++
		if evaluator != nil && !containsEvaluatorEntry(l.closingEvaluators, evaluatorID) {
			l.closingEvaluators = append(l.closingEvaluators, closingEvaluatorEntry{id: evaluatorID, evaluator: evaluator})
		}
	}
	evaluators := append([]closingEvaluatorEntry(nil), l.closingEvaluators...)
	l.mu.Unlock()
	l.statusMu.Unlock()
	if wait {
		return l.drainClosingEvaluators(ctx)
	}
	return l.processClosingEvaluatorEntries(ctx, false, evaluators)
}

func (l *CDPLink) detachRuntimeEvaluatorOwned(ctx context.Context, wait bool, evaluator runtimeEvaluator, evaluatorID uint64, generation uint64) error {
	l.statusMu.Lock()
	l.mu.Lock()
	entry, pending, ownedCurrent := l.detachCurrentEvaluatorLocked(evaluator, evaluatorID, generation)
	if !ownedCurrent {
		pending = containsEvaluatorEntry(l.closingEvaluators, evaluatorID)
		entry = closingEvaluatorEntry{id: evaluatorID, evaluator: evaluator}
	}
	l.mu.Unlock()
	l.statusMu.Unlock()
	if !pending {
		return nil
	}
	return l.processClosingEvaluatorEntries(ctx, wait, []closingEvaluatorEntry{entry})
}

func (l *CDPLink) detachCurrentEvaluatorLocked(evaluator runtimeEvaluator, evaluatorID uint64, generation uint64) (closingEvaluatorEntry, bool, bool) {
	entry := closingEvaluatorEntry{id: evaluatorID, evaluator: evaluator}
	if evaluator == nil || l.eval != evaluator || l.evaluatorID != evaluatorID || l.runtimeGeneration != generation {
		return entry, false, false
	}
	l.eval = nil
	l.evaluatorID = 0
	l.context = cdp.ExecutionContext{}
	l.contextGeneration = 0
	l.pendingSetupToken = 0
	l.pendingContextID = 0
	l.pendingGeneration = 0
	l.pendingSource = nil
	l.runtimeGeneration++
	if l.connectHandle != nil && l.connectHandle.evaluator == evaluator &&
		l.connectHandle.evaluatorID == evaluatorID && l.connectHandle.runtimeGeneration == generation {
		l.connectHandle.evaluator = nil
		l.connectHandle.evaluatorID = 0
		l.connectHandle.runtimeGeneration = 0
	}
	pending := containsEvaluatorEntry(l.closingEvaluators, evaluatorID)
	if !pending {
		l.closingEvaluators = append(l.closingEvaluators, entry)
		pending = true
	}
	return entry, pending, true
}

func (l *CDPLink) drainClosingEvaluators(ctx context.Context) error {
	for {
		l.mu.Lock()
		evaluators := append([]closingEvaluatorEntry(nil), l.closingEvaluators...)
		l.mu.Unlock()
		if len(evaluators) == 0 {
			return nil
		}
		if err := l.processClosingEvaluatorEntries(ctx, true, evaluators); err != nil {
			return err
		}
	}
}

func (l *CDPLink) processClosingEvaluatorEntries(ctx context.Context, wait bool, evaluators []closingEvaluatorEntry) error {
	var firstErr error
	for _, entry := range evaluators {
		closing := entry.evaluator
		if closer, ok := closing.(runtimeEvaluatorOwnerCloser); ok {
			if wait {
				err := closer.CloseAndWaitContext(ctx)
				if err == nil {
					l.removeClosingEvaluator(entry.id)
				} else if firstErr == nil {
					firstErr = err
				}
			} else if requester, ok := closing.(runtimeEvaluatorCloser); ok {
				requester.Close()
			}
			continue
		}
		if closer, ok := closing.(runtimeEvaluatorCloser); ok {
			closer.Close()
			l.removeClosingEvaluator(entry.id)
		} else {
			l.removeClosingEvaluator(entry.id)
		}
	}
	return firstErr
}

func containsEvaluatorEntry(evaluators []closingEvaluatorEntry, id uint64) bool {
	for _, current := range evaluators {
		if current.id == id {
			return true
		}
	}
	return false
}

func (l *CDPLink) removeClosingEvaluator(id uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for index, current := range l.closingEvaluators {
		if current.id == id {
			copy(l.closingEvaluators[index:], l.closingEvaluators[index+1:])
			l.closingEvaluators = l.closingEvaluators[:len(l.closingEvaluators)-1]
			return
		}
	}
}

func sleepContext(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func waitForExecutionContext(ctx context.Context, client *cdp.Client, preferredName string, timeout time.Duration) (cdp.ExecutionContext, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		contexts := client.Contexts()
		var fallback cdp.ExecutionContext
		for _, context := range contexts {
			if fallback.ID == 0 {
				fallback = context
			}
			if preferredName != "" && context.Name == preferredName {
				return context, nil
			}
		}
		if fallback.ID != 0 {
			return fallback, nil
		}
		select {
		case <-ctx.Done():
			return cdp.ExecutionContext{}, ctx.Err()
		case <-deadline.C:
			return cdp.ExecutionContext{}, errors.New("execution context is not ready")
		case <-ticker.C:
		}
	}
}

func runtimeContextProbeExpression() string {
	return `(() => {
const G = globalThis;
const cc = G.cc || (G.GameGlobal && G.GameGlobal.cc);
const ctl = G.gameCtl || (G.GameGlobal && G.GameGlobal.gameCtl);
const scene = cc && cc.director && typeof cc.director.getScene === "function" ? cc.director.getScene() : null;
return {
  hasCc: !!cc,
  hasGameCtl: !!ctl,
  hasGameGlobal: !!G.GameGlobal,
  hasWx: !!G.wx,
  hasDocument: !!(G.document || (G.GameGlobal && G.GameGlobal.document)),
  hasCanvas: !!(G.canvas || (G.GameGlobal && G.GameGlobal.canvas) || (cc && cc.game && cc.game.canvas)),
  scene: scene ? scene.name : null,
  href: G.location && typeof G.location.href === "string" ? G.location.href : null
};
})()`
}

func contextProbeFromValue(context cdp.ExecutionContext, value any) cdp.ContextProbe {
	probe := cdp.ContextProbe{
		ID:     context.ID,
		Name:   context.Name,
		Origin: context.Origin,
	}
	raw, ok := value.(map[string]any)
	if !ok {
		probe.Error = "probe result is not an object"
		probe.Score = cdp.ScoreContextProbe(probe)
		return probe
	}
	probe.HasCc = boolFromMap(raw, "hasCc")
	probe.HasGameCtl = boolFromMap(raw, "hasGameCtl")
	probe.HasGameGlobal = boolFromMap(raw, "hasGameGlobal")
	probe.HasWx = boolFromMap(raw, "hasWx")
	probe.HasDocument = boolFromMap(raw, "hasDocument")
	probe.HasCanvas = boolFromMap(raw, "hasCanvas")
	probe.Scene = stringFromMap(raw, "scene")
	probe.Href = stringFromMap(raw, "href")
	probe.Score = cdp.ScoreContextProbe(probe)
	return probe
}

func probeExpression(requiredMethods []string) string {
	if requiredMethods == nil {
		requiredMethods = []string{}
	}
	methodsJSON := mustJSON(requiredMethods)
	return fmt.Sprintf(`(() => {
const ctl = globalThis.gameCtl || (globalThis.GameGlobal && globalThis.GameGlobal.gameCtl);
const scene = ctl && typeof ctl.scene === "function" && ctl.scene() ? ctl.scene().name : null;
let farmRoot = null;
try {
  const root = ctl && typeof ctl.findFarmRoot === "function" ? ctl.findFarmRoot() : null;
  farmRoot = root && typeof ctl.fullPath === "function" ? ctl.fullPath(root) : !!root;
} catch (_) {
  farmRoot = null;
}
const requiredMethods = %s;
const methods = {};
for (let i = 0; i < requiredMethods.length; i++) {
  const key = requiredMethods[i];
  methods[key] = !!(ctl && typeof ctl[key] === "function");
}
return {
  hasGameCtl: !!ctl,
  scriptHash: ctl && typeof ctl.__scriptHash === "string" ? ctl.__scriptHash : null,
  methods,
  methodCount: ctl ? Object.keys(ctl).filter((key) => typeof ctl[key] === "function").length : 0,
  scene,
  farmRoot
};
})()`, methodsJSON)
}

func probeHasReadyGameCtl(value any, scriptHash string, requiredMethods []string) bool {
	probe, ok := value.(map[string]any)
	if !ok {
		return false
	}
	ready, ok := probe["hasGameCtl"].(bool)
	if !ok || !ready {
		return false
	}
	if scriptHash != "" {
		if got, _ := probe["scriptHash"].(string); got != scriptHash {
			return false
		}
	}
	methods, _ := probe["methods"].(map[string]any)
	for _, method := range requiredMethods {
		if method == "" {
			continue
		}
		present, _ := methods[method].(bool)
		if !present {
			return false
		}
	}
	return true
}

func boolFromMap(raw map[string]any, key string) bool {
	value, _ := raw[key].(bool)
	return value
}

func stringFromMap(raw map[string]any, key string) string {
	value, _ := raw[key].(string)
	return value
}

func intFromMap(raw map[string]any, key string) int {
	value, ok := raw[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func mustJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "null"
	}
	return string(raw)
}

func isRuntimeReadinessPendingError(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "execution context is not ready") ||
		strings.Contains(text, "gameCtl_not_ready") ||
		strings.Contains(text, "miniapp jscontext") ||
		strings.Contains(text, "runtime evaluator is not connected")
}

func debugWebSocketURL(port int) string {
	if port <= 0 {
		return ""
	}
	return fmt.Sprintf("ws://127.0.0.1:%d/", port)
}

func gameCtlCallExpression(path string, args []any) (string, error) {
	methodName := strings.TrimPrefix(path, "gameCtl.")
	if methodName == "" || strings.Contains(methodName, ".") {
		return "", fmt.Errorf("unsupported runtime method: %s", path)
	}
	methodJSON, err := json.Marshal(methodName)
	if err != nil {
		return "", err
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`(() => {
const ctl = globalThis.gameCtl || (globalThis.GameGlobal && globalThis.GameGlobal.gameCtl);
const methodName = %s;
const args = %s;
if (!ctl) throw new Error("gameCtl_not_ready");
const fn = ctl[methodName];
if (typeof fn !== "function") throw new Error("call_path_not_ready: gameCtl." + methodName);
return fn.apply(ctl, Array.isArray(args) ? args : []);
})()`, methodJSON, argsJSON), nil
}
