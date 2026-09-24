package guard

import (
	"context"
	"strings"
	"sync"
	"time"
)

const (
	guardianCallTimeout = 5 * time.Second
	recentEventLimit    = 50
)

var guardianSyncRetryDelays = [...]time.Duration{
	800 * time.Millisecond,
	1600 * time.Millisecond,
	3200 * time.Millisecond,
	5 * time.Second,
}

type workerSyncMask uint8

const (
	syncNetworkWorker workerSyncMask = 1 << iota
	syncOtherPlaceWorker
	syncAllWorkers = syncNetworkWorker | syncOtherPlaceWorker
)

type workerSyncResult struct {
	worker  RecoveryKind
	enabled bool
	err     error
}

type RuntimeCaller interface {
	Call(context.Context, string, []any, time.Duration) (any, error)
}

type SupervisorOptions struct {
	Settings    Settings
	Process     *Manager
	Coordinator *RecoveryCoordinator
	Caller      RuntimeCaller
	Snapshot    func() RuntimeSnapshot
	OnEvent     func(RuntimeEvent)
	After       func(time.Duration) <-chan time.Time
}

type Supervisor struct {
	lifecycleMu sync.Mutex
	mu          sync.Mutex
	settings    Settings
	process     *Manager
	coordinator *RecoveryCoordinator
	caller      RuntimeCaller
	snapshot    func() RuntimeSnapshot
	onEvent     func(RuntimeEvent)
	after       func(time.Duration) <-chan time.Time

	running    bool
	lifecycle  uint64
	runCtx     context.Context
	runCancel  context.CancelFunc
	syncCancel context.CancelFunc
	latest     RuntimeSnapshot
	status     SupervisorStatus
	wg         sync.WaitGroup
}

func NewSupervisor(options SupervisorOptions) *Supervisor {
	settings := normalizeSupervisorSettings(options.Settings)
	process := options.Process
	if process == nil {
		process = NewManager(ManagerOptions{Settings: settings})
	}
	coordinator := options.Coordinator
	if coordinator == nil {
		coordinator = NewRecoveryCoordinator(CoordinatorOptions{})
	}
	after := options.After
	if after == nil {
		after = time.After
	}
	s := &Supervisor{
		settings:    settings,
		process:     process,
		coordinator: coordinator,
		caller:      options.Caller,
		snapshot:    options.Snapshot,
		onEvent:     options.OnEvent,
		after:       after,
	}
	s.status.Enabled = settings.Enabled
	s.status.Phase = initialSupervisorPhase(settings.Enabled)
	s.applyDesiredWorkerStatusLocked(settings)
	return s
}

func (s *Supervisor) Start(ctx context.Context) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.lifecycle++
	lifecycle := s.lifecycle
	runCtx, cancel := context.WithCancel(ctx)
	s.running = true
	s.runCtx = runCtx
	s.runCancel = cancel
	s.status.Running = true
	settings := s.settings
	s.mu.Unlock()

	if !s.isLifecycleCurrent(lifecycle) {
		return
	}
	s.process.UpdateSettings(settings)
	if !s.isLifecycleCurrent(lifecycle) {
		return
	}
	if settings.Enabled {
		s.process.Arm("guardian supervisor")
	} else {
		s.process.Disarm()
	}
	s.coordinator.Start(runCtx)
	if !s.isLifecycleCurrent(lifecycle) {
		return
	}
	s.process.Start(runCtx, s.currentSnapshot)
	if s.snapshot != nil {
		s.onRuntimeStatus(s.snapshot())
	}
}

func (s *Supervisor) Close() {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.lifecycle++
	s.status.Running = false
	clearWorkerActivity(&s.status.Network)
	clearWorkerActivity(&s.status.OtherPlaceLogin)
	s.coordinator.AdvanceGeneration("supervisor_close")
	if s.syncCancel != nil {
		s.syncCancel()
		s.syncCancel = nil
	}
	if s.runCancel != nil {
		s.runCancel()
		s.runCancel = nil
	}
	s.runCtx = nil
	s.mu.Unlock()

	s.coordinator.Close()
	s.clearPendingRecoveries()
	s.process.Disarm()
	s.process.Close()
	s.wg.Wait()
}

func (s *Supervisor) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

func (s *Supervisor) UpdateSettings(settings Settings) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	settings = normalizeSupervisorSettings(settings)
	s.mu.Lock()
	s.settings = settings
	s.status.Enabled = settings.Enabled
	s.applyDesiredWorkerStatusLocked(settings)
	s.coordinator.AdvanceGeneration("settings_changed")
	if s.syncCancel != nil {
		s.syncCancel()
		s.syncCancel = nil
	}
	running := s.running
	ready := s.latest.Ready
	if running && ready {
		s.startSyncLocked(settings, false)
	}
	s.mu.Unlock()

	s.process.UpdateSettings(settings)
	if !running {
		s.process.Disarm()
	} else if settings.Enabled {
		s.process.Arm("guardian supervisor")
	} else {
		s.process.Disarm()
	}
}

func (s *Supervisor) Generation() uint64 {
	return s.coordinator.Generation()
}

func (s *Supervisor) IsCurrentGeneration(generation uint64) bool {
	return s.coordinator.IsCurrent(generation)
}

func (s *Supervisor) IsCurrent(generation uint64) bool {
	return s.coordinator.IsCurrent(generation)
}

func (s *Supervisor) OnRuntimeStatus(snapshot RuntimeSnapshot) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.onRuntimeStatus(snapshot)
}

func (s *Supervisor) onRuntimeStatus(snapshot RuntimeSnapshot) {
	s.process.tick(s.process.now(), snapshot)
	s.mu.Lock()
	s.latest = snapshot
	s.status.RuntimeTarget = resolveRuntimeTarget(snapshot)
	if !snapshot.Ready {
		clearWorkerActivity(&s.status.Network)
		clearWorkerActivity(&s.status.OtherPlaceLogin)
		s.coordinator.AdvanceGeneration("runtime_not_ready")
		if s.syncCancel != nil {
			s.syncCancel()
			s.syncCancel = nil
		}
		s.mu.Unlock()
		return
	}
	settings := s.settings
	if s.running {
		s.startSyncLocked(settings, true)
	} else {
		s.coordinator.AdvanceGeneration("runtime_ready")
	}
	s.mu.Unlock()

	if snapshot.Connected {
		s.process.NoteHealthy(snapshot)
	}
}

func (s *Supervisor) NoteHealthy() {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if !s.isRunning() {
		return
	}
	snapshot := s.currentSnapshot()
	s.mu.Lock()
	s.latest = snapshot
	s.status.RuntimeTarget = resolveRuntimeTarget(snapshot)
	s.mu.Unlock()
	s.process.NoteHealthy(snapshot)
}

func (s *Supervisor) NoteRuntimeError(err error) bool {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if !s.isRunning() {
		return false
	}
	return s.process.NoteRuntimeError(err, s.currentSnapshot())
}

func (s *Supervisor) HandleRuntimeEvent(event RuntimeEvent) {
	now := time.Now().UTC().Format(time.RFC3339)
	var recovery RecoveryKind
	var restartDetail string
	s.mu.Lock()
	s.status.RecentEvents = append([]RuntimeEvent{event}, s.status.RecentEvents...)
	if len(s.status.RecentEvents) > recentEventLimit {
		s.status.RecentEvents = s.status.RecentEvents[:recentEventLimit]
	}
	if event.RuntimeTarget != "" {
		s.status.RuntimeTarget = resolveRuntimeTarget(RuntimeSnapshot{RuntimeTarget: event.RuntimeTarget})
	}
	eventName := strings.ToLower(strings.TrimSpace(event.Name))
	eventPhase := strings.ToLower(strings.TrimSpace(event.Phase))
	worker, kind := s.workerForEventLocked(eventName)
	if worker != nil {
		applyRuntimeEventToWorker(worker, event, now)
		if (eventPhase == "detected" || eventPhase == "due") && s.recoveryEligibleLocked(kind) {
			recovery = kind
		}
	}
	if eventName == "network_reconnect" && eventPhase == "failed" && s.recoveryEligibleLocked(RecoveryNetworkReconnect) {
		restartDetail = strings.TrimSpace(event.Error)
		if restartDetail == "" {
			restartDetail = "reconnect_failed"
		}
	}
	lifecycle := s.lifecycle
	generation := s.coordinator.Generation()
	callback := s.onEvent
	s.mu.Unlock()

	if recovery != "" {
		s.submitRecoveryIfCurrent(lifecycle, generation, RecoveryRequest{
			Kind:          recovery,
			Reason:        event.Name + ":" + event.Phase,
			RuntimeTarget: event.RuntimeTarget,
		})
	}
	if restartDetail != "" {
		s.restartAfterNetworkRecoveryFailureIfCurrent(lifecycle, generation, restartDetail)
	}
	if callback != nil {
		callGuardianEventCallback(callback, event)
	}
}

func (s *Supervisor) Status() SupervisorStatus {
	process := s.process.Status()
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.status
	status.Process = process
	status.RecentEvents = append([]RuntimeEvent(nil), s.status.RecentEvents...)
	status.Phase = aggregateSupervisorPhase(status, s.latest.Ready)
	return status
}

func (s *Supervisor) currentSnapshot() RuntimeSnapshot {
	s.mu.Lock()
	snapshotFn := s.snapshot
	latest := s.latest
	s.mu.Unlock()
	if snapshotFn != nil {
		return snapshotFn()
	}
	return latest
}

func (s *Supervisor) isLifecycleCurrent(lifecycle uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running && s.lifecycle == lifecycle
}

func (s *Supervisor) isRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Supervisor) recoveryEligibleLocked(kind RecoveryKind) bool {
	if !s.running || !s.settings.Enabled {
		return false
	}
	if kind == RecoveryNetworkReconnect {
		return s.settings.NetworkReconnectEnabled
	}
	return kind == RecoveryOtherPlaceLogin && s.settings.OtherPlaceLoginEnabled
}

func (s *Supervisor) submitRecoveryIfCurrent(lifecycle, generation uint64, request RecoveryRequest) bool {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.mu.Lock()
	eligible := s.lifecycle == lifecycle && s.coordinator.IsCurrent(generation) && s.recoveryEligibleLocked(request.Kind)
	s.mu.Unlock()
	if !eligible {
		return false
	}
	return s.coordinator.Submit(request)
}

func (s *Supervisor) restartAfterNetworkRecoveryFailureIfCurrent(lifecycle, generation uint64, detail string) bool {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.mu.Lock()
	eligible := s.lifecycle == lifecycle && s.coordinator.IsCurrent(generation) && s.recoveryEligibleLocked(RecoveryNetworkReconnect)
	s.mu.Unlock()
	if !eligible {
		return false
	}
	return s.process.RestartAfterNetworkRecoveryFailure(s.currentSnapshot(), detail)
}

func (s *Supervisor) clearPendingRecoveries() {
	s.coordinator.mu.Lock()
	clear(s.coordinator.pending)
	s.coordinator.mu.Unlock()
}

func (s *Supervisor) startSyncLocked(settings Settings, advanceGeneration bool) {
	if s.syncCancel != nil {
		s.syncCancel()
	}
	generation := s.coordinator.Generation()
	if advanceGeneration {
		generation = s.coordinator.AdvanceGeneration("runtime_ready")
	}
	ctx, cancel := context.WithCancel(s.runCtx)
	s.syncCancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.syncRuntimeWorkers(ctx, generation, settings)
	}()
}

func (s *Supervisor) syncRuntimeWorkers(ctx context.Context, generation uint64, settings Settings) {
	pending := syncAllWorkers
	for attempt := 0; ; attempt++ {
		results := s.syncRuntimeWorkersOnce(ctx, generation, settings, pending)
		if !s.isSyncCurrent(ctx, generation) {
			return
		}
		pending = 0
		for _, result := range results {
			if result.err == nil {
				s.recordWorkerSyncSuccess(result.worker, generation, result.enabled)
				continue
			}
			s.recordWorkerSyncError(result.worker, generation, result.err)
			if isRetryableGuardianSyncError(result.err) {
				pending |= syncMaskForWorker(result.worker)
			}
		}
		if pending == 0 || attempt >= len(guardianSyncRetryDelays) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-s.after(guardianSyncRetryDelays[attempt]):
		}
		if !s.isSyncCurrent(ctx, generation) {
			return
		}
	}
}

func (s *Supervisor) syncRuntimeWorkersOnce(
	ctx context.Context,
	generation uint64,
	settings Settings,
	pending workerSyncMask,
) []workerSyncResult {
	results := make([]workerSyncResult, 0, 2)
	if pending&syncNetworkWorker != 0 {
		enabled := settings.Enabled && settings.NetworkReconnectEnabled
		results = append(results, workerSyncResult{
			worker:  RecoveryNetworkReconnect,
			enabled: enabled,
			err: s.callRuntimeWorker(ctx, generation,
				"gameCtl.setReconnectWatcherEnabled",
				[]any{enabled, map[string]any{
					"intervalMs":       settings.NetworkReconnectIntervalMS,
					"recoverTimeoutMs": settings.NetworkRecoveryTimeoutMS,
				}}),
		})
	}
	if pending&syncOtherPlaceWorker != 0 {
		enabled := settings.Enabled && settings.OtherPlaceLoginEnabled
		results = append(results, workerSyncResult{
			worker:  RecoveryOtherPlaceLogin,
			enabled: enabled,
			err: s.callRuntimeWorker(ctx, generation,
				"gameCtl.setOtherPlaceLoginReconnectEnabled",
				[]any{enabled, map[string]any{
					"intervalMs":       settings.OtherPlaceLoginIntervalMS,
					"reconnectDelayMs": settings.OtherPlaceLoginDelayMin * 60 * 1000,
				}}),
		})
	}
	return results
}

func (s *Supervisor) callRuntimeWorker(
	ctx context.Context,
	generation uint64,
	method string,
	args []any,
) error {
	if !s.isSyncCurrent(ctx, generation) {
		return context.Canceled
	}
	if s.caller == nil {
		return nil
	}
	_, err := s.caller.Call(ctx, method, args, guardianCallTimeout)
	if !s.isSyncCurrent(ctx, generation) {
		return context.Canceled
	}
	return err
}

func syncMaskForWorker(worker RecoveryKind) workerSyncMask {
	if worker == RecoveryOtherPlaceLogin {
		return syncOtherPlaceWorker
	}
	return syncNetworkWorker
}

func (s *Supervisor) isSyncCurrent(ctx context.Context, generation uint64) bool {
	if ctx.Err() != nil || !s.coordinator.IsCurrent(generation) {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running && s.latest.Ready
}

func (s *Supervisor) recordWorkerSyncSuccess(worker RecoveryKind, generation uint64, enabled bool) {
	if !s.coordinator.IsCurrent(generation) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running || !s.latest.Ready || !s.coordinator.IsCurrent(generation) {
		return
	}
	status := s.workerStatusLocked(worker)
	status.Enabled = enabled
	status.Running = enabled
	status.LastCheckAt = time.Now().UTC().Format(time.RFC3339)
	if status.LastResult == "" || status.LastResult == "sync_failed" ||
		status.LastResult == "enabled" || status.LastResult == "disabled" {
		status.Busy = false
		if enabled {
			status.LastResult = "enabled"
		} else {
			status.LastResult = "disabled"
		}
		status.LastError = ""
	}
}

func (s *Supervisor) recordWorkerSyncError(worker RecoveryKind, generation uint64, err error) {
	if err == nil || !s.coordinator.IsCurrent(generation) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running || !s.coordinator.IsCurrent(generation) {
		return
	}
	status := s.workerStatusLocked(worker)
	status.Running = false
	status.Busy = false
	status.LastCheckAt = time.Now().UTC().Format(time.RFC3339)
	status.LastResult = "sync_failed"
	status.LastError = err.Error()
}

func (s *Supervisor) workerStatusLocked(worker RecoveryKind) *WorkerStatus {
	if worker == RecoveryOtherPlaceLogin {
		return &s.status.OtherPlaceLogin
	}
	return &s.status.Network
}

func (s *Supervisor) workerForEventLocked(name string) (*WorkerStatus, RecoveryKind) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "network_reconnect":
		return &s.status.Network, RecoveryNetworkReconnect
	case "other_place_login_reconnect":
		return &s.status.OtherPlaceLogin, RecoveryOtherPlaceLogin
	default:
		return nil, ""
	}
}

func (s *Supervisor) applyDesiredWorkerStatusLocked(settings Settings) {
	s.status.Network.Enabled = settings.Enabled && settings.NetworkReconnectEnabled
	s.status.OtherPlaceLogin.Enabled = settings.Enabled && settings.OtherPlaceLoginEnabled
	if !s.status.Network.Enabled {
		markWorkerDisabled(&s.status.Network)
	}
	if !s.status.OtherPlaceLogin.Enabled {
		markWorkerDisabled(&s.status.OtherPlaceLogin)
	}
}

func markWorkerDisabled(status *WorkerStatus) {
	clearWorkerActivity(status)
	status.LastResult = "disabled"
	status.LastError = ""
}

func clearWorkerActivity(status *WorkerStatus) {
	status.Running = false
	status.Busy = false
}

func applyRuntimeEventToWorker(status *WorkerStatus, event RuntimeEvent, now string) {
	phase := strings.ToLower(strings.TrimSpace(event.Phase))
	status.LastCheckAt = now
	switch phase {
	case "detected", "due", "waiting":
		status.Busy = true
		status.LastResult = phase
		status.LastError = ""
	case "reconnected":
		status.Busy = false
		status.LastHandledAt = now
		status.LastResult = phase
		status.LastError = ""
	case "failed":
		status.Busy = false
		status.LastHandledAt = now
		status.LastResult = phase
		status.LastError = strings.TrimSpace(event.Error)
		if status.LastError == "" {
			status.LastError = "failed"
		}
	default:
		status.LastResult = phase
	}
	if event.Handled && status.LastHandledAt == "" {
		status.LastHandledAt = now
	}
}

func aggregateSupervisorPhase(status SupervisorStatus, runtimeReady bool) Phase {
	if !status.Enabled {
		return PhaseDisabled
	}
	switch status.Process.Phase {
	case PhaseCircuitOpen, PhaseRestarting, PhaseWaitingReconnect, PhaseDegraded:
		return status.Process.Phase
	}
	if !status.Running || !runtimeReady {
		return PhaseStandby
	}
	if status.Network.LastError != "" || status.OtherPlaceLogin.LastError != "" {
		return PhaseDegraded
	}
	if status.Network.Busy || status.OtherPlaceLogin.Busy {
		return PhaseWaitingReconnect
	}
	return PhaseWatching
}

func initialSupervisorPhase(enabled bool) Phase {
	if enabled {
		return PhaseStandby
	}
	return PhaseDisabled
}

func normalizeSupervisorSettings(settings Settings) Settings {
	settings = normalizeSettings(settings)
	defaults := DefaultSettings()
	if settings.NetworkReconnectIntervalMS <= 0 {
		settings.NetworkReconnectIntervalMS = defaults.NetworkReconnectIntervalMS
	}
	if settings.NetworkRecoveryTimeoutMS <= 0 {
		settings.NetworkRecoveryTimeoutMS = defaults.NetworkRecoveryTimeoutMS
	}
	if settings.OtherPlaceLoginIntervalMS <= 0 {
		settings.OtherPlaceLoginIntervalMS = defaults.OtherPlaceLoginIntervalMS
	}
	if settings.OtherPlaceLoginDelayMin < 0 {
		settings.OtherPlaceLoginDelayMin = defaults.OtherPlaceLoginDelayMin
	}
	return settings
}

func isRetryableGuardianSyncError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if message == "missing context" || strings.HasSuffix(message, ": missing context") ||
		message == "runtime not ready" || strings.HasSuffix(message, ": runtime not ready") {
		return true
	}
	for _, signature := range []string{
		"execution context is not ready",
		"runtime context is not ready",
		"execution context was destroyed",
		"execution context destroyed",
		"execution context not found",
		"cannot find execution context",
		"cannot find context with specified id",
		"execution context id is required",
		"runtime evaluator is not connected",
		"runtime link is not connected",
		"runtime is not connected",
		"cdp not connected",
		"gamectl_not_ready",
		"call_path_not_ready",
		"missing methods:",
		"method missing due injection",
	} {
		if strings.Contains(message, signature) {
			return true
		}
	}
	return false
}

func callGuardianEventCallback(callback func(RuntimeEvent), event RuntimeEvent) {
	defer func() { _ = recover() }()
	callback(event)
}
