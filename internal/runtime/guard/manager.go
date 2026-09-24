package guard

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	restartWindow        = 10 * time.Minute
	maxWorkerSleep       = time.Minute
	globalRestartLimit   = 8
	circuitError         = "10 minutes automatic restart limit reached"
	globalCircuitError   = "10 minutes total restart safety limit reached"
	callbackCircuitError = "restart callback is not configured"
)

type circuitCause uint8

const (
	circuitCauseAutomatic circuitCause = iota + 1
	circuitCauseSafety
	circuitCauseCallback
)

type RuntimeSnapshot struct {
	RuntimeTarget  string `json:"runtimeTarget,omitempty"`
	ResolvedTarget string `json:"resolvedTarget,omitempty"`
	Ready          bool   `json:"ready,omitempty"`
	Connected      bool   `json:"connected,omitempty"`
}

type RestartReason struct {
	RuntimeTarget string          `json:"runtimeTarget"`
	Reason        string          `json:"reason"`
	Snapshot      RuntimeSnapshot `json:"snapshot"`
	Manual        bool            `json:"manual,omitempty"`
	Scheduled     bool            `json:"scheduled,omitempty"`
}

type RestartEvent struct {
	At            string `json:"at"`
	RuntimeTarget string `json:"runtimeTarget"`
	Reason        string `json:"reason"`
	Trigger       string `json:"trigger"`
	OK            bool   `json:"ok"`
	Error         string `json:"error,omitempty"`
}

type Status struct {
	Enabled                     bool           `json:"enabled"`
	Armed                       bool           `json:"armed"`
	ArmReason                   string         `json:"armReason,omitempty"`
	Phase                       Phase          `json:"phase"`
	RuntimeTarget               string         `json:"runtimeTarget"`
	TimeoutStreak               int            `json:"timeoutStreak"`
	Threshold                   int            `json:"threshold"`
	LastCheckAt                 string         `json:"lastCheckAt,omitempty"`
	LastTimeoutAt               string         `json:"lastTimeoutAt,omitempty"`
	LastHealthyAt               string         `json:"lastHealthyAt,omitempty"`
	LastRestartAt               string         `json:"lastRestartAt,omitempty"`
	ReconnectGraceUntil         string         `json:"reconnectGraceUntil,omitempty"`
	ReconnectGraceRemainingMS   int64          `json:"reconnectGraceRemainingMs"`
	LastReason                  string         `json:"lastReason,omitempty"`
	LastActionError             string         `json:"lastActionError,omitempty"`
	RestartCountInWindow        int            `json:"restartCountInWindow"`
	MaxRestartsPerWindow        int            `json:"maxRestartsPerWindow"`
	CircuitOpen                 bool           `json:"circuitOpen"`
	ActionMode                  string         `json:"actionMode,omitempty"`
	RecentRestartReason         string         `json:"recentRestartReason,omitempty"`
	RecentRestartEvents         []RestartEvent `json:"recentRestartEvents"`
	ScheduledRestartEnabled     bool           `json:"scheduledRestartEnabled"`
	ScheduledRestartIntervalMin int            `json:"scheduledRestartIntervalMin"`
	ScheduledRestartNextAt      string         `json:"scheduledRestartNextAt,omitempty"`
	ScheduledRestartRemainingMs int64          `json:"scheduledRestartRemainingMs"`
	ScheduledRestartLastAt      string         `json:"scheduledRestartLastAt,omitempty"`
	ScheduledRestartLastResult  string         `json:"scheduledRestartLastResult,omitempty"`
	ScheduledRestartLastError   string         `json:"scheduledRestartLastError,omitempty"`
	AutoMinimizeAfterRestart    bool           `json:"autoMinimizeAfterRestart"`
}

type ManagerOptions struct {
	Settings         Settings
	Restart          func(RestartReason) (RestartResult, error)
	OnLifecycleEvent func(LifecycleEvent)
	Now              func() time.Time
	NewTimer         func(time.Duration) ManagerTimer
}

type ManagerTimer interface {
	C() <-chan time.Time
	Stop() bool
	Reset(time.Duration) bool
}

type realManagerTimer struct {
	timer *time.Timer
}

func (t *realManagerTimer) C() <-chan time.Time {
	return t.timer.C
}

func (t *realManagerTimer) Stop() bool {
	return t.timer.Stop()
}

func (t *realManagerTimer) Reset(delay time.Duration) bool {
	return t.timer.Reset(delay)
}

type restartJob struct {
	id         uint64
	generation uint64
	reason     RestartReason
	trigger    string
	call       func(RestartReason) (RestartResult, error)
}

type managerRun struct {
	generation uint64
	cancel     context.CancelFunc
	wake       chan struct{}
	done       chan struct{}
}

type lifecycleNotification struct {
	callback func(LifecycleEvent)
	event    LifecycleEvent
}

func (notification lifecycleNotification) deliver() {
	if notification.callback == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	notification.callback(notification.event)
}

type Manager struct {
	mu                    sync.Mutex
	lifecycleMu           sync.Mutex
	settings              Settings
	restart               func(RestartReason) (RestartResult, error)
	onLifecycleEvent      func(LifecycleEvent)
	now                   func() time.Time
	newTimer              func(time.Duration) ManagerTimer
	armed                 bool
	armReason             string
	status                Status
	automaticRestartTimes []time.Time
	totalRestartTimes     []time.Time
	restartInFlight       bool
	nextRestartID         uint64
	activeRestartID       uint64
	automaticCircuitOpen  bool
	safetyCircuitOpen     bool
	callbackCircuitOpen   bool
	scheduledRestartNext  time.Time
	reconnectGraceUntil   time.Time
	lifecycleUnhealthy    bool
	lifecycleUnhealthyAt  time.Time
	lifecycleQueue        []lifecycleNotification
	lifecycleDelivering   bool
	generation            uint64
	run                   *managerRun
}

func NewManager(options ManagerOptions) *Manager {
	settings := normalizeSettings(options.Settings)
	now := options.Now
	if now == nil {
		now = time.Now
	}
	newTimer := options.NewTimer
	if newTimer == nil {
		newTimer = func(delay time.Duration) ManagerTimer {
			return &realManagerTimer{timer: time.NewTimer(delay)}
		}
	}
	phase := PhaseDisabled
	if settings.Enabled {
		phase = PhaseStandby
	}
	m := &Manager{
		settings:         settings,
		restart:          options.Restart,
		onLifecycleEvent: options.OnLifecycleEvent,
		now:              now,
		newTimer:         newTimer,
		status: Status{
			Enabled:                     settings.Enabled,
			Phase:                       phase,
			RuntimeTarget:               "qq_ws",
			Threshold:                   settings.TimeoutThreshold,
			MaxRestartsPerWindow:        settings.MaxRestartsPer10Min,
			ScheduledRestartEnabled:     settings.ScheduledRestartEnabled,
			ScheduledRestartIntervalMin: settings.ScheduledRestartIntervalMin,
			AutoMinimizeAfterRestart:    settings.AutoMinimizeAfterRestart,
		},
	}
	if settings.Enabled && settings.ScheduledRestartEnabled {
		m.setScheduledRestartNextLocked(now())
	}
	return m
}

func (m *Manager) Start(ctx context.Context, snapshot func() RuntimeSnapshot) {
	if ctx == nil {
		ctx = context.Background()
	}
	m.mu.Lock()
	previous := m.run
	if previous != nil {
		previous.cancel()
	}
	m.generation++
	runCtx, cancel := context.WithCancel(ctx)
	run := &managerRun{
		generation: m.generation,
		cancel:     cancel,
		wake:       make(chan struct{}, 1),
		done:       make(chan struct{}),
	}
	m.run = run
	m.mu.Unlock()
	if previous != nil {
		<-previous.done
	}
	go m.loop(runCtx, run, snapshot)
}

func (m *Manager) Close() {
	m.mu.Lock()
	run := m.run
	m.run = nil
	m.generation++
	m.mu.Unlock()
	if run != nil {
		run.cancel()
		<-run.done
	}
}

func (m *Manager) loop(ctx context.Context, run *managerRun, snapshot func() RuntimeSnapshot) {
	timer := m.newTimer(m.nextWakeDelay())
	defer func() {
		stopManagerTimer(timer)
		close(run.done)
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-run.wake:
			resetManagerTimer(timer, m.nextWakeDelay())
		case <-timer.C():
			if !m.isCurrentRun(run) {
				return
			}
			var current RuntimeSnapshot
			if snapshot != nil {
				current = snapshot()
			}
			if !m.isCurrentRun(run) {
				return
			}
			m.tick(m.now(), current)
			resetManagerTimer(timer, m.nextWakeDelay())
		}
	}
}

func (m *Manager) isCurrentRun(run *managerRun) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.run == run && m.generation == run.generation
}

func (m *Manager) lifecycleNotificationLocked(event LifecycleEvent) lifecycleNotification {
	return lifecycleNotification{callback: m.onLifecycleEvent, event: event}
}

func (m *Manager) queueLifecycleNotificationsLocked(notifications ...lifecycleNotification) {
	m.lifecycleMu.Lock()
	for _, notification := range notifications {
		if notification.callback != nil {
			m.lifecycleQueue = append(m.lifecycleQueue, notification)
		}
	}
	m.lifecycleMu.Unlock()
}

func (m *Manager) deliverLifecycleNotifications() {
	m.lifecycleMu.Lock()
	if m.lifecycleDelivering || len(m.lifecycleQueue) == 0 {
		m.lifecycleMu.Unlock()
		return
	}
	m.lifecycleDelivering = true
	m.lifecycleMu.Unlock()
	go m.drainLifecycleNotifications()
}

func (m *Manager) drainLifecycleNotifications() {
	for {
		m.lifecycleMu.Lock()
		if len(m.lifecycleQueue) == 0 {
			m.lifecycleDelivering = false
			m.lifecycleQueue = nil
			m.lifecycleMu.Unlock()
			return
		}
		notification := m.lifecycleQueue[0]
		m.lifecycleQueue[0] = lifecycleNotification{}
		m.lifecycleQueue = m.lifecycleQueue[1:]
		m.lifecycleMu.Unlock()
		notification.deliver()
	}
}

func (m *Manager) recoveryNotificationLocked(now time.Time, runtimeTarget, previousError string) lifecycleNotification {
	durationMS := int64(0)
	if !m.lifecycleUnhealthyAt.IsZero() {
		durationMS = now.Sub(m.lifecycleUnhealthyAt).Milliseconds()
		if durationMS < 0 {
			durationMS = 0
		}
	}
	return m.lifecycleNotificationLocked(LifecycleEvent{
		Kind:          LifecycleRecovery,
		RuntimeTarget: runtimeTarget,
		Error:         previousError,
		Streak:        m.status.TimeoutStreak,
		Threshold:     m.settings.TimeoutThreshold,
		DurationMS:    durationMS,
		At:            now.UTC().Format(time.RFC3339),
	})
}

func (m *Manager) nextWakeDelay() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	delay := time.Duration(m.settings.MonitorIntervalMS) * time.Millisecond
	if delay <= 0 || delay > maxWorkerSleep {
		delay = maxWorkerSleep
	}
	shorten := func(at time.Time) {
		if at.IsZero() {
			return
		}
		remaining := at.Sub(now)
		if remaining < 0 {
			remaining = 0
		}
		if remaining < delay {
			delay = remaining
		}
	}
	shorten(m.scheduledRestartNext)
	shorten(m.reconnectGraceUntil)
	if m.automaticCircuitOpen && len(m.automaticRestartTimes) > 0 {
		shorten(m.automaticRestartTimes[0].Add(restartWindow))
	}
	if m.safetyCircuitOpen && len(m.totalRestartTimes) > 0 {
		shorten(m.totalRestartTimes[0].Add(restartWindow))
	}
	return delay
}

func (m *Manager) UpdateSettings(settings Settings) {
	m.mu.Lock()
	previous := m.settings
	m.settings = normalizeSettings(settings)
	m.status.Enabled = m.settings.Enabled
	m.status.Threshold = m.settings.TimeoutThreshold
	m.status.MaxRestartsPerWindow = m.settings.MaxRestartsPer10Min
	m.status.ScheduledRestartEnabled = m.settings.ScheduledRestartEnabled
	m.status.ScheduledRestartIntervalMin = m.settings.ScheduledRestartIntervalMin
	m.status.AutoMinimizeAfterRestart = m.settings.AutoMinimizeAfterRestart
	scheduleChanged := previous.Enabled != m.settings.Enabled ||
		previous.ScheduledRestartEnabled != m.settings.ScheduledRestartEnabled ||
		previous.ScheduledRestartIntervalMin != m.settings.ScheduledRestartIntervalMin
	if !m.settings.Enabled || !m.settings.ScheduledRestartEnabled {
		m.clearScheduledRestartLocked()
	} else if scheduleChanged || m.scheduledRestartNext.IsZero() {
		m.setScheduledRestartNextLocked(m.now())
	}
	if !m.settings.Enabled {
		m.lifecycleUnhealthy = false
		m.lifecycleUnhealthyAt = time.Time{}
		m.status.Phase = PhaseDisabled
		m.clearReconnectGraceLocked()
	} else if m.status.CircuitOpen {
		m.status.Phase = PhaseCircuitOpen
	} else if !m.restartInFlight && m.reconnectGraceUntil.IsZero() {
		m.setWatchingPhaseLocked()
	}
	m.signalWakeLocked()
	m.mu.Unlock()
}

func (m *Manager) Settings() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings
}

func (m *Manager) Arm(reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.armed = true
	m.armReason = strings.TrimSpace(reason)
	m.status.Armed = true
	m.status.ArmReason = m.armReason
	if m.settings.Enabled && !m.status.CircuitOpen && !m.restartInFlight && m.reconnectGraceUntil.IsZero() {
		m.status.Phase = PhaseWatching
	}
}

func (m *Manager) Disarm() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.armed = false
	m.armReason = ""
	m.status.Armed = false
	m.status.ArmReason = ""
	m.status.TimeoutStreak = 0
	m.lifecycleUnhealthy = false
	m.lifecycleUnhealthyAt = time.Time{}
	if !m.restartInFlight && m.reconnectGraceUntil.IsZero() {
		m.setWatchingPhaseLocked()
	}
}

func (m *Manager) NoteHealthy(snapshot RuntimeSnapshot) {
	m.mu.Lock()
	now := m.now()
	runtimeTarget := resolveRuntimeTarget(snapshot)
	wasUnhealthy := m.lifecycleUnhealthy
	previousError := m.status.LastReason
	notification := lifecycleNotification{}
	if wasUnhealthy {
		notification = m.recoveryNotificationLocked(now, runtimeTarget, previousError)
	}
	m.lifecycleUnhealthy = false
	m.lifecycleUnhealthyAt = time.Time{}
	m.status.RuntimeTarget = runtimeTarget
	m.status.LastHealthyAt = now.UTC().Format(time.RFC3339)
	m.status.LastReason = ""
	m.status.LastActionError = ""
	m.status.TimeoutStreak = 0
	m.clearReconnectGraceLocked()
	if !m.restartInFlight {
		m.setWatchingPhaseLocked()
	}
	m.queueLifecycleNotificationsLocked(notification)
	m.mu.Unlock()
	m.deliverLifecycleNotifications()
}

func (m *Manager) NoteRuntimeError(err error, snapshot RuntimeSnapshot) bool {
	reason := normalizeError(err)
	if reason == "" {
		return false
	}
	m.mu.Lock()
	runtimeTarget := resolveRuntimeTarget(snapshot)
	m.status.RuntimeTarget = runtimeTarget
	if !m.armed || !isRestartableRuntimeError(reason) {
		m.mu.Unlock()
		return false
	}
	now := m.now()
	notifications := make([]lifecycleNotification, 0, 2)
	m.trimRestartWindowLocked(now)
	m.maybeCloseCircuitLocked()
	m.status.LastTimeoutAt = now.UTC().Format(time.RFC3339)
	m.status.LastReason = reason
	if !m.lifecycleUnhealthy {
		m.lifecycleUnhealthy = true
		m.lifecycleUnhealthyAt = now
		notifications = append(notifications, m.lifecycleNotificationLocked(LifecycleEvent{
			Kind:          LifecycleSuspected,
			RuntimeTarget: runtimeTarget,
			Error:         reason,
			Streak:        m.status.TimeoutStreak + 1,
			Threshold:     m.settings.TimeoutThreshold,
			At:            now.UTC().Format(time.RFC3339),
		}))
	}
	if !m.settings.Enabled || !m.settings.FailureRecoveryEnabled || m.restartInFlight || !m.reconnectGraceUntil.IsZero() {
		m.queueLifecycleNotificationsLocked(notifications...)
		m.mu.Unlock()
		m.deliverLifecycleNotifications()
		return true
	}
	if m.status.CircuitOpen {
		m.queueLifecycleNotificationsLocked(notifications...)
		m.mu.Unlock()
		m.deliverLifecycleNotifications()
		return true
	}
	if len(m.automaticRestartTimes) >= m.settings.MaxRestartsPer10Min {
		notifications = append(notifications, m.openCircuitLocked(circuitCauseAutomatic))
		m.queueLifecycleNotificationsLocked(notifications...)
		m.mu.Unlock()
		m.deliverLifecycleNotifications()
		return true
	}
	m.status.TimeoutStreak++
	m.status.Phase = PhaseDegraded
	if m.status.TimeoutStreak < m.settings.TimeoutThreshold {
		m.queueLifecycleNotificationsLocked(notifications...)
		m.mu.Unlock()
		m.deliverLifecycleNotifications()
		return true
	}
	job, _, _, notification := m.reserveRestartLocked(RestartReason{RuntimeTarget: runtimeTarget, Reason: reason, Snapshot: snapshot})
	notifications = append(notifications, notification)
	m.queueLifecycleNotificationsLocked(notifications...)
	m.mu.Unlock()
	m.deliverLifecycleNotifications()
	if job != nil {
		go m.executeRestart(*job)
	}
	return true
}

func (m *Manager) RestartAfterNetworkRecoveryFailure(snapshot RuntimeSnapshot, detail string) bool {
	reason := "network reconnect failed"
	if detail = strings.TrimSpace(detail); detail != "" {
		reason += ": " + detail
	}
	m.mu.Lock()
	runtimeTarget := resolveRuntimeTarget(snapshot)
	m.status.RuntimeTarget = runtimeTarget
	if !m.armed {
		m.mu.Unlock()
		return false
	}
	now := m.now()
	notifications := make([]lifecycleNotification, 0, 2)
	m.trimRestartWindowLocked(now)
	m.maybeCloseCircuitLocked()
	m.status.LastTimeoutAt = now.UTC().Format(time.RFC3339)
	m.status.LastReason = reason
	if !m.lifecycleUnhealthy {
		m.lifecycleUnhealthy = true
		m.lifecycleUnhealthyAt = now
		notifications = append(notifications, m.lifecycleNotificationLocked(LifecycleEvent{
			Kind:          LifecycleSuspected,
			RuntimeTarget: runtimeTarget,
			Error:         reason,
			Streak:        m.status.TimeoutStreak + 1,
			Threshold:     m.settings.TimeoutThreshold,
			At:            now.UTC().Format(time.RFC3339),
		}))
	}
	if !m.settings.Enabled || !m.settings.FailureRecoveryEnabled || m.restartInFlight || !m.reconnectGraceUntil.IsZero() {
		m.queueLifecycleNotificationsLocked(notifications...)
		m.mu.Unlock()
		m.deliverLifecycleNotifications()
		return false
	}
	if m.status.CircuitOpen {
		m.queueLifecycleNotificationsLocked(notifications...)
		m.mu.Unlock()
		m.deliverLifecycleNotifications()
		return false
	}
	if len(m.automaticRestartTimes) >= m.settings.MaxRestartsPer10Min {
		notifications = append(notifications, m.openCircuitLocked(circuitCauseAutomatic))
		m.queueLifecycleNotificationsLocked(notifications...)
		m.mu.Unlock()
		m.deliverLifecycleNotifications()
		return false
	}
	m.status.TimeoutStreak++
	job, _, _, notification := m.reserveRestartLocked(RestartReason{
		RuntimeTarget: runtimeTarget,
		Reason:        reason,
		Snapshot:      snapshot,
	})
	notifications = append(notifications, notification)
	m.queueLifecycleNotificationsLocked(notifications...)
	m.mu.Unlock()
	m.deliverLifecycleNotifications()
	if job == nil {
		return false
	}
	go m.executeRestart(*job)
	return true
}

func (m *Manager) ManualRestart(snapshot RuntimeSnapshot, reason string) (RestartResult, error) {
	if strings.TrimSpace(reason) == "" {
		reason = "manual restart"
	}
	runtimeTarget := resolveRuntimeTarget(snapshot)
	m.mu.Lock()
	m.status.RuntimeTarget = runtimeTarget
	job, result, err, notification := m.reserveRestartLocked(RestartReason{
		RuntimeTarget: runtimeTarget,
		Reason:        reason,
		Snapshot:      snapshot,
		Manual:        true,
	})
	m.queueLifecycleNotificationsLocked(notification)
	m.mu.Unlock()
	m.deliverLifecycleNotifications()
	if job == nil {
		return result, err
	}
	return m.executeRestart(*job)
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.trimRestartWindowLocked(now)
	m.maybeCloseCircuitLocked()
	status := m.status
	status.RestartCountInWindow = len(m.automaticRestartTimes)
	status.ReconnectGraceRemainingMS = remainingMilliseconds(now, m.reconnectGraceUntil)
	status.ScheduledRestartRemainingMs = remainingMilliseconds(now, m.scheduledRestartNext)
	status.RecentRestartEvents = append([]RestartEvent{}, m.status.RecentRestartEvents...)
	status.AutoMinimizeAfterRestart = m.settings.AutoMinimizeAfterRestart
	return status
}

func (m *Manager) tick(now time.Time, snapshot RuntimeSnapshot) {
	m.mu.Lock()
	m.status.LastCheckAt = now.UTC().Format(time.RFC3339)
	m.status.RuntimeTarget = resolveRuntimeTarget(snapshot)
	m.trimRestartWindowLocked(now)
	m.maybeCloseCircuitLocked()
	notification := m.updateReconnectGraceLocked(now, snapshot)
	job, circuitNotification := m.maybeScheduleRestartLocked(now, snapshot)
	m.queueLifecycleNotificationsLocked(notification, circuitNotification)
	m.mu.Unlock()
	m.deliverLifecycleNotifications()
	if job != nil {
		go m.executeRestart(*job)
	}
}

func (m *Manager) reserveRestartLocked(reason RestartReason) (*restartJob, RestartResult, error, lifecycleNotification) {
	now := m.now()
	m.trimRestartWindowLocked(now)
	m.maybeCloseCircuitLocked()
	trigger := restartTrigger(reason)
	if m.safetyCircuitOpen || len(m.totalRestartTimes) >= globalRestartLimit {
		notification := m.openCircuitLocked(circuitCauseSafety)
		return nil, RestartResult{Status: "circuit_open", Reason: m.status.LastActionError}, nil, notification
	}
	if m.settings.Enabled && trigger == "auto" && (m.automaticCircuitOpen || len(m.automaticRestartTimes) >= m.settings.MaxRestartsPer10Min) {
		notification := m.openCircuitLocked(circuitCauseAutomatic)
		return nil, RestartResult{Status: "circuit_open", Reason: m.status.LastActionError}, nil, notification
	}
	if m.restartInFlight {
		return nil, RestartResult{Status: "restart_running", Reason: "restart already running"}, nil, lifecycleNotification{}
	}
	m.totalRestartTimes = append(m.totalRestartTimes, now)
	if m.restart == nil {
		if trigger == "auto" {
			m.automaticRestartTimes = append(m.automaticRestartTimes, now)
			m.status.RestartCountInWindow = len(m.automaticRestartTimes)
		}
		m.status.LastRestartAt = now.UTC().Format(time.RFC3339)
		m.status.RecentRestartReason = reason.Reason
		m.status.ActionMode = restartActionMode(reason.RuntimeTarget)
		notification := m.openCircuitLocked(circuitCauseCallback)
		return nil, RestartResult{}, errors.New(callbackCircuitError), notification
	}
	if trigger == "auto" {
		m.automaticRestartTimes = append(m.automaticRestartTimes, now)
		m.status.RestartCountInWindow = len(m.automaticRestartTimes)
	}
	m.restartInFlight = true
	m.status.Phase = PhaseRestarting
	m.status.LastRestartAt = now.UTC().Format(time.RFC3339)
	m.status.RecentRestartReason = reason.Reason
	m.status.ActionMode = restartActionMode(reason.RuntimeTarget)
	m.nextRestartID++
	m.activeRestartID = m.nextRestartID
	return &restartJob{
		id:         m.activeRestartID,
		generation: m.generation,
		reason:     reason,
		trigger:    trigger,
		call:       m.restart,
	}, RestartResult{}, nil, lifecycleNotification{}
}

func (m *Manager) executeRestart(job restartJob) (RestartResult, error) {
	result, err := job.call(job.reason)
	now := m.now()
	m.mu.Lock()
	if m.activeRestartID != job.id {
		m.mu.Unlock()
		return result, err
	}
	m.restartInFlight = false
	m.activeRestartID = 0
	if job.generation != m.generation {
		m.reconcileStaleRestartLocked(job)
		m.signalWakeLocked()
		m.mu.Unlock()
		return result, err
	}
	notifications := []lifecycleNotification{m.lifecycleNotificationLocked(LifecycleEvent{
		Kind:          LifecycleRestartCompleted,
		RuntimeTarget: job.reason.RuntimeTarget,
		Trigger:       job.trigger,
		Result:        result,
		Streak:        m.status.TimeoutStreak,
		Threshold:     m.settings.TimeoutThreshold,
		At:            now.UTC().Format(time.RFC3339),
	})}
	if err != nil {
		notifications[0].event.Error = err.Error()
		m.status.LastActionError = err.Error()
		m.recordRestartEventLockedAt(now, job.reason, job.trigger, false, err.Error())
		if job.reason.Scheduled {
			m.status.ScheduledRestartLastResult = "error"
			m.status.ScheduledRestartLastError = err.Error()
		}
		if m.settings.Enabled {
			if m.status.CircuitOpen {
				m.status.Phase = PhaseCircuitOpen
			} else {
				m.status.Phase = PhaseDegraded
			}
		}
		if job.trigger == "auto" {
			notifications = append(notifications, m.lifecycleNotificationLocked(LifecycleEvent{
				Kind:          LifecycleAbnormal,
				RuntimeTarget: job.reason.RuntimeTarget,
				Error:         err.Error(),
				Trigger:       job.trigger,
				Result:        result,
				Streak:        m.status.TimeoutStreak,
				Threshold:     m.settings.TimeoutThreshold,
				At:            now.UTC().Format(time.RFC3339),
			}))
		}
		m.queueLifecycleNotificationsLocked(notifications...)
		m.mu.Unlock()
		m.deliverLifecycleNotifications()
		return result, err
	}
	m.status.LastActionError = ""
	m.status.TimeoutStreak = 0
	m.recordRestartEventLockedAt(now, job.reason, job.trigger, true, "")
	if job.reason.Scheduled {
		m.status.ScheduledRestartLastResult = "ok"
		m.status.ScheduledRestartLastError = ""
	}
	if result.Status == RestartStatusReconnected {
		m.clearReconnectGraceLocked()
		m.setWatchingPhaseLocked()
	} else {
		m.enterWaitingReconnectLocked(now)
	}
	m.signalWakeLocked()
	m.queueLifecycleNotificationsLocked(notifications...)
	m.mu.Unlock()
	m.deliverLifecycleNotifications()
	return result, nil
}

func (m *Manager) reconcileStaleRestartLocked(job restartJob) {
	if job.reason.Scheduled && m.status.ScheduledRestartLastResult == "running" {
		m.status.ScheduledRestartLastResult = ""
		m.status.ScheduledRestartLastError = ""
	}
	if !m.settings.Enabled {
		m.status.Phase = PhaseDisabled
	} else if m.status.CircuitOpen {
		m.status.Phase = PhaseCircuitOpen
	} else if !m.reconnectGraceUntil.IsZero() {
		m.status.Phase = PhaseWaitingReconnect
	} else if m.armed {
		m.status.Phase = PhaseWatching
	} else {
		m.status.Phase = PhaseStandby
	}
}

func (m *Manager) maybeScheduleRestartLocked(now time.Time, snapshot RuntimeSnapshot) (*restartJob, lifecycleNotification) {
	if !m.settings.Enabled || !m.settings.ScheduledRestartEnabled {
		m.clearScheduledRestartLocked()
		return nil, lifecycleNotification{}
	}
	if m.scheduledRestartNext.IsZero() {
		m.setScheduledRestartNextLocked(now)
		return nil, lifecycleNotification{}
	}
	if now.Before(m.scheduledRestartNext) {
		return nil, lifecycleNotification{}
	}
	if !m.reconnectGraceUntil.IsZero() {
		m.setScheduledRestartNextLocked(now)
		return nil, lifecycleNotification{}
	}
	m.status.ScheduledRestartLastAt = now.UTC().Format(time.RFC3339)
	m.setScheduledRestartNextLocked(now)
	reason := RestartReason{
		RuntimeTarget: resolveRuntimeTarget(snapshot),
		Reason:        "scheduled restart",
		Snapshot:      snapshot,
		Scheduled:     true,
	}
	job, result, err, notification := m.reserveRestartLocked(reason)
	if job != nil {
		m.status.ScheduledRestartLastResult = "running"
		m.status.ScheduledRestartLastError = ""
		return job, notification
	}
	m.status.ScheduledRestartLastResult = "error"
	if err != nil {
		m.status.ScheduledRestartLastError = err.Error()
	} else {
		m.status.ScheduledRestartLastError = result.Reason
	}
	return nil, notification
}

func (m *Manager) updateReconnectGraceLocked(now time.Time, snapshot RuntimeSnapshot) lifecycleNotification {
	if m.reconnectGraceUntil.IsZero() {
		return lifecycleNotification{}
	}
	if snapshot.Ready && snapshot.Connected {
		notification := lifecycleNotification{}
		if m.lifecycleUnhealthy {
			notification = m.recoveryNotificationLocked(now, m.status.RuntimeTarget, m.status.LastReason)
		}
		m.status.LastHealthyAt = now.UTC().Format(time.RFC3339)
		m.status.LastReason = ""
		m.status.TimeoutStreak = 0
		m.lifecycleUnhealthy = false
		m.lifecycleUnhealthyAt = time.Time{}
		m.clearReconnectGraceLocked()
		m.setWatchingPhaseLocked()
		return notification
	}
	if !now.Before(m.reconnectGraceUntil) {
		m.clearReconnectGraceLocked()
		m.setWatchingPhaseLocked()
		return lifecycleNotification{}
	}
	if !m.status.CircuitOpen {
		m.status.Phase = PhaseWaitingReconnect
	}
	return lifecycleNotification{}
}

func (m *Manager) enterWaitingReconnectLocked(now time.Time) {
	if !m.settings.Enabled || m.status.CircuitOpen {
		m.clearReconnectGraceLocked()
		m.setWatchingPhaseLocked()
		return
	}
	m.reconnectGraceUntil = now.Add(time.Duration(m.settings.RestartReconnectGraceSec) * time.Second)
	m.status.Phase = PhaseWaitingReconnect
	m.status.ReconnectGraceUntil = m.reconnectGraceUntil.UTC().Format(time.RFC3339)
}

func (m *Manager) clearReconnectGraceLocked() {
	m.reconnectGraceUntil = time.Time{}
	m.status.ReconnectGraceUntil = ""
	m.status.ReconnectGraceRemainingMS = 0
}

func (m *Manager) setScheduledRestartNextLocked(now time.Time) {
	m.scheduledRestartNext = now.Add(time.Duration(m.settings.ScheduledRestartIntervalMin) * time.Minute)
	m.status.ScheduledRestartNextAt = m.scheduledRestartNext.UTC().Format(time.RFC3339)
}

func (m *Manager) clearScheduledRestartLocked() {
	m.scheduledRestartNext = time.Time{}
	m.status.ScheduledRestartNextAt = ""
	m.status.ScheduledRestartRemainingMs = 0
}

func (m *Manager) openCircuitLocked(cause circuitCause) lifecycleNotification {
	wasOpen := m.status.CircuitOpen
	switch cause {
	case circuitCauseAutomatic:
		m.automaticCircuitOpen = true
	case circuitCauseSafety:
		m.safetyCircuitOpen = true
	case circuitCauseCallback:
		m.callbackCircuitOpen = true
	}
	m.status.CircuitOpen = true
	m.status.Phase = PhaseCircuitOpen
	m.status.LastActionError = m.circuitErrorLocked()
	if wasOpen {
		return lifecycleNotification{}
	}
	return m.lifecycleNotificationLocked(LifecycleEvent{
		Kind:          LifecycleAbnormal,
		RuntimeTarget: m.status.RuntimeTarget,
		Error:         m.status.LastActionError,
		Streak:        m.status.TimeoutStreak,
		Threshold:     m.settings.TimeoutThreshold,
		At:            m.now().UTC().Format(time.RFC3339),
	})
}

func (m *Manager) maybeCloseCircuitLocked() {
	changed := false
	if m.automaticCircuitOpen && len(m.automaticRestartTimes) < m.settings.MaxRestartsPer10Min {
		m.automaticCircuitOpen = false
		changed = true
	}
	if m.safetyCircuitOpen && len(m.totalRestartTimes) < globalRestartLimit {
		m.safetyCircuitOpen = false
		changed = true
	}
	if !changed {
		return
	}
	m.status.CircuitOpen = m.automaticCircuitOpen || m.safetyCircuitOpen || m.callbackCircuitOpen
	if m.status.CircuitOpen {
		m.status.LastActionError = m.circuitErrorLocked()
	} else {
		m.status.LastActionError = ""
	}
	if !m.status.CircuitOpen && !m.restartInFlight && m.reconnectGraceUntil.IsZero() {
		m.setWatchingPhaseLocked()
	}
}

func (m *Manager) circuitErrorLocked() string {
	if m.callbackCircuitOpen {
		return callbackCircuitError
	}
	if m.safetyCircuitOpen {
		return globalCircuitError
	}
	if m.automaticCircuitOpen {
		return circuitError
	}
	return ""
}

func (m *Manager) setWatchingPhaseLocked() {
	if !m.settings.Enabled {
		m.status.Phase = PhaseDisabled
	} else if m.status.CircuitOpen {
		m.status.Phase = PhaseCircuitOpen
	} else if m.armed {
		m.status.Phase = PhaseWatching
	} else {
		m.status.Phase = PhaseStandby
	}
}

func (m *Manager) trimRestartWindowLocked(now time.Time) {
	m.automaticRestartTimes = trimRestartTimes(now, m.automaticRestartTimes)
	m.totalRestartTimes = trimRestartTimes(now, m.totalRestartTimes)
	m.status.RestartCountInWindow = len(m.automaticRestartTimes)
}

func trimRestartTimes(now time.Time, items []time.Time) []time.Time {
	keep := items[:0]
	for _, item := range items {
		if now.Sub(item) <= restartWindow {
			keep = append(keep, item)
		}
	}
	return keep
}

func (m *Manager) recordRestartEventLockedAt(now time.Time, reason RestartReason, trigger string, ok bool, message string) {
	event := RestartEvent{
		At:            now.UTC().Format(time.RFC3339),
		RuntimeTarget: reason.RuntimeTarget,
		Reason:        reason.Reason,
		Trigger:       trigger,
		OK:            ok,
		Error:         message,
	}
	m.status.RecentRestartEvents = append([]RestartEvent{event}, m.status.RecentRestartEvents...)
	if len(m.status.RecentRestartEvents) > 30 {
		m.status.RecentRestartEvents = m.status.RecentRestartEvents[:30]
	}
}

func (m *Manager) signalWakeLocked() {
	if m.run == nil {
		return
	}
	select {
	case m.run.wake <- struct{}{}:
	default:
	}
}

func stopManagerTimer(timer ManagerTimer) {
	if timer.Stop() {
		return
	}
	select {
	case <-timer.C():
	default:
	}
}

func resetManagerTimer(timer ManagerTimer, delay time.Duration) {
	stopManagerTimer(timer)
	timer.Reset(delay)
}

func remainingMilliseconds(now, until time.Time) int64 {
	if until.IsZero() || !until.After(now) {
		return 0
	}
	return until.Sub(now).Milliseconds()
}

func restartTrigger(reason RestartReason) string {
	if reason.Manual {
		return "manual"
	}
	if reason.Scheduled {
		return "scheduled"
	}
	return "auto"
}

func normalizeSettings(settings Settings) Settings {
	defaults := DefaultSettings()
	if settings.TimeoutThreshold <= 0 {
		settings.TimeoutThreshold = defaults.TimeoutThreshold
	}
	if settings.MonitorIntervalMS <= 0 {
		settings.MonitorIntervalMS = defaults.MonitorIntervalMS
	}
	if settings.RestartReconnectGraceSec <= 0 {
		settings.RestartReconnectGraceSec = defaults.RestartReconnectGraceSec
	}
	if settings.MaxRestartsPer10Min <= 0 {
		settings.MaxRestartsPer10Min = defaults.MaxRestartsPer10Min
	}
	if settings.ScheduledRestartIntervalMin <= 0 {
		settings.ScheduledRestartIntervalMin = defaults.ScheduledRestartIntervalMin
	}
	return settings
}

func resolveRuntimeTarget(snapshot RuntimeSnapshot) string {
	target := strings.ToLower(strings.TrimSpace(snapshot.ResolvedTarget))
	if target == "" {
		target = strings.ToLower(strings.TrimSpace(snapshot.RuntimeTarget))
	}
	switch target {
	case "qq", "qq_ws":
		return "qq_ws"
	case "yyb", "yyb_cdp":
		return "yyb_cdp"
	case "wechat", "wx", "cdp", "wechat_cdp":
		return "wechat_cdp"
	default:
		if target == "" {
			return "qq_ws"
		}
		return target
	}
}

func restartActionMode(runtimeTarget string) string {
	switch resolveRuntimeTarget(RuntimeSnapshot{RuntimeTarget: runtimeTarget}) {
	case "qq_ws":
		return "qq_restart"
	case "yyb_cdp":
		return "yyb_restart"
	default:
		return "wx_restart"
	}
}

func normalizeError(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}

func isRestartableRuntimeError(message string) bool {
	text := strings.ToLower(message)
	fragments := []string{
		"context deadline exceeded",
		"timeout",
		"timed out",
		"cdp timeout",
		"execution context",
		"runtime is not connected",
		"runtime not connected",
		"client disconnected",
		"missing methods",
	}
	for _, fragment := range fragments {
		if strings.Contains(text, fragment) {
			return true
		}
	}
	return false
}
