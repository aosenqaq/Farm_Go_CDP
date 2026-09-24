package guard

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type guardTestTimer struct {
	clock  *guardTestClock
	due    time.Time
	ch     chan time.Time
	active bool
	stops  int
	resets int
}

type guardTestClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*guardTestTimer
}

func newGuardTestClock(now time.Time) *guardTestClock {
	return &guardTestClock{now: now}
}

func (c *guardTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *guardTestClock) NewTimer(delay time.Duration) ManagerTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	timer := &guardTestTimer{clock: c, due: c.now.Add(delay), ch: make(chan time.Time, 1), active: true}
	c.timers = append(c.timers, timer)
	return timer
}

func (t *guardTestTimer) C() <-chan time.Time {
	return t.ch
}

func (t *guardTestTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	wasActive := t.active
	t.active = false
	t.stops++
	return wasActive
}

func (t *guardTestTimer) Reset(delay time.Duration) bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	wasActive := t.active
	t.due = t.clock.now.Add(delay)
	t.active = true
	t.resets++
	return wasActive
}

func (c *guardTestClock) Advance(delay time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(delay)
	now := c.now
	var due []*guardTestTimer
	for _, timer := range c.timers {
		if timer.active && !timer.due.After(now) {
			timer.active = false
			due = append(due, timer)
		}
	}
	c.mu.Unlock()
	for _, timer := range due {
		timer.ch <- now
	}
}

func (c *guardTestClock) waitForTimers(t *testing.T, count int) {
	t.Helper()
	waitFor(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return len(c.timers) >= count
	})
}

func (c *guardTestClock) timerDelay(index int) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.timers[index].due.Sub(c.now)
}

func (c *guardTestClock) timerCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}

func (c *guardTestClock) timerResetCount(index int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.timers[index].resets
}

func (c *guardTestClock) timerStopCount(index int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.timers[index].stops
}

func (c *guardTestClock) waitForTimerResets(t *testing.T, index, count int) {
	t.Helper()
	waitFor(t, func() bool { return c.timerResetCount(index) >= count })
}

func (c *guardTestClock) waitForTimerStops(t *testing.T, index, count int) {
	t.Helper()
	waitFor(t, func() bool { return c.timerStopCount(index) >= count })
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition was not met before timeout")
}

func waitForRestartIdle(t *testing.T, m *Manager) {
	t.Helper()
	waitFor(t, func() bool {
		m.mu.Lock()
		defer m.mu.Unlock()
		return !m.restartInFlight
	})
}

func waitForLifecycleDeliveryIdle(t *testing.T, m *Manager) {
	t.Helper()
	waitFor(t, func() bool {
		m.lifecycleMu.Lock()
		defer m.lifecycleMu.Unlock()
		return !m.lifecycleDelivering && len(m.lifecycleQueue) == 0
	})
}

func enabledSettings() Settings {
	return Settings{
		Enabled:                  true,
		FailureRecoveryEnabled:   true,
		TimeoutThreshold:         2,
		MonitorIntervalMS:        1000,
		RestartReconnectGraceSec: 5,
		MaxRestartsPer10Min:      4,
	}
}

func lifecycleKinds(events []LifecycleEvent) []LifecycleKind {
	kinds := make([]LifecycleKind, 0, len(events))
	for _, event := range events {
		kinds = append(kinds, event.Kind)
	}
	return kinds
}

func equalLifecycleKinds(got, want []LifecycleKind) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func TestManagerEmitsSuspectedThenRecovery(t *testing.T) {
	events := make(chan LifecycleEvent, 2)
	settings := enabledSettings()
	settings.TimeoutThreshold = 3
	manager := NewManager(ManagerOptions{
		Settings: settings,
		OnLifecycleEvent: func(event LifecycleEvent) {
			events <- event
		},
	})
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	manager.NoteHealthy(RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true})

	gotEvents := []LifecycleEvent{<-events, <-events}
	if got, want := lifecycleKinds(gotEvents), []LifecycleKind{LifecycleSuspected, LifecycleRecovery}; !equalLifecycleKinds(got, want) {
		t.Fatalf("event kinds = %v, want %v", got, want)
	}
	if event := gotEvents[0]; event.RuntimeTarget != "qq_ws" || event.Error != "timeout" || event.Streak != 1 || event.Threshold != 3 || event.At == "" {
		t.Fatalf("unexpected suspected event: %#v", event)
	}
}

func TestManagerRestartsAfterThreeRuntimeIsNotConnectedErrors(t *testing.T) {
	restarted := make(chan RestartReason, 1)
	settings := enabledSettings()
	settings.TimeoutThreshold = 3
	manager := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(reason RestartReason) (RestartResult, error) {
			restarted <- reason
			return RestartResult{Status: "launch_dispatched"}, nil
		},
	})
	manager.Arm("test")

	for range 3 {
		if !manager.NoteRuntimeError(errors.New("runtime is not connected"), RuntimeSnapshot{RuntimeTarget: "qq_ws"}) {
			t.Fatal("runtime-is-not-connected error was not handled")
		}
	}

	select {
	case reason := <-restarted:
		if reason.RuntimeTarget != "qq_ws" || reason.Reason != "runtime is not connected" {
			t.Fatalf("unexpected restart reason: %#v", reason)
		}
	case <-time.After(time.Second):
		t.Fatal("automatic restart was not requested")
	}
}

func TestManagerReportsRecoveryDurationFromFirstFailure(t *testing.T) {
	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	events := make(chan LifecycleEvent, 2)
	manager := NewManager(ManagerOptions{
		Settings: enabledSettings(),
		Now:      func() time.Time { return now },
		OnLifecycleEvent: func(event LifecycleEvent) {
			events <- event
		},
	})
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	<-events
	now = now.Add(42 * time.Second)
	manager.NoteHealthy(RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true})
	recovery := <-events
	if recovery.Kind != LifecycleRecovery || recovery.DurationMS != 42000 {
		t.Fatalf("recovery=%#v", recovery)
	}
}

func TestManagerEmitsRestartCompletionThenAbnormalForAutomaticFailure(t *testing.T) {
	events := make(chan LifecycleEvent, 3)
	settings := enabledSettings()
	settings.TimeoutThreshold = 1
	manager := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(RestartReason) (RestartResult, error) {
			return RestartResult{Status: "failed"}, errors.New("restart failed")
		},
		OnLifecycleEvent: func(event LifecycleEvent) {
			events <- event
		},
	})
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})

	got := make([]LifecycleEvent, 0, 3)
	for len(got) < 3 {
		select {
		case event := <-events:
			got = append(got, event)
		case <-time.After(time.Second):
			t.Fatalf("received %d lifecycle events, want 3", len(got))
		}
	}
	if kinds := lifecycleKinds(got); !equalLifecycleKinds(kinds, []LifecycleKind{LifecycleSuspected, LifecycleRestartCompleted, LifecycleAbnormal}) {
		t.Fatalf("event kinds = %v", kinds)
	}
	if event := got[1]; event.Trigger != "auto" || event.Result.Status != "failed" || event.Error != "restart failed" {
		t.Fatalf("unexpected restart completion: %#v", event)
	}
}

func TestManagerEmitsRestartCompletionForManualAndScheduledRestarts(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	events := make(chan LifecycleEvent, 2)
	settings := enabledSettings()
	settings.ScheduledRestartEnabled = true
	settings.ScheduledRestartIntervalMin = 1
	manager := NewManager(ManagerOptions{
		Settings: settings,
		Now:      clock.Now,
		Restart: func(RestartReason) (RestartResult, error) {
			return RestartResult{Status: "launch_dispatched"}, nil
		},
		OnLifecycleEvent: func(event LifecycleEvent) {
			events <- event
		},
	})
	if _, err := manager.ManualRestart(RuntimeSnapshot{RuntimeTarget: "qq_ws"}, "operator request"); err != nil {
		t.Fatalf("manual restart: %v", err)
	}
	clock.Advance(time.Minute)
	manager.tick(clock.Now(), RuntimeSnapshot{RuntimeTarget: "qq_ws"})

	got := make([]LifecycleEvent, 0, 2)
	for len(got) < 2 {
		select {
		case event := <-events:
			got = append(got, event)
		case <-time.After(time.Second):
			t.Fatalf("received %d lifecycle events, want 2", len(got))
		}
	}
	if got[0].Kind != LifecycleRestartCompleted || got[0].Trigger != "manual" || got[1].Kind != LifecycleRestartCompleted || got[1].Trigger != "scheduled" {
		t.Fatalf("unexpected completion events: %#v", got)
	}
}

func TestManagerDoesNotEmitRecoveryForPeriodicHealthyChecks(t *testing.T) {
	var events atomic.Int32
	manager := NewManager(ManagerOptions{
		Settings: enabledSettings(),
		OnLifecycleEvent: func(event LifecycleEvent) {
			events.Add(1)
		},
	})
	manager.NoteHealthy(RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true})
	manager.NoteHealthy(RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true})
	manager.tick(time.Unix(100, 0), RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true})

	if events.Load() != 0 {
		t.Fatalf("periodic healthy checks emitted %d lifecycle events", events.Load())
	}
}

func TestManagerContainsLifecycleCallbackPanic(t *testing.T) {
	var calls atomic.Int32
	settings := enabledSettings()
	settings.TimeoutThreshold = 3
	manager := NewManager(ManagerOptions{
		Settings: settings,
		OnLifecycleEvent: func(LifecycleEvent) {
			calls.Add(1)
			panic("callback failure")
		},
	})
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	manager.NoteHealthy(RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true})

	waitFor(t, func() bool { return calls.Load() == 2 })
	if calls.Load() != 2 {
		t.Fatalf("callback calls = %d, want 2", calls.Load())
	}
}

func TestManagerLifecycleStateSurvivesDisabledFailureRecovery(t *testing.T) {
	events := make(chan LifecycleEvent, 2)
	settings := enabledSettings()
	settings.FailureRecoveryEnabled = false
	manager := NewManager(ManagerOptions{
		Settings: settings,
		OnLifecycleEvent: func(event LifecycleEvent) {
			events <- event
		},
	})
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	manager.NoteHealthy(RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true})

	if kinds := lifecycleKinds([]LifecycleEvent{<-events, <-events}); !equalLifecycleKinds(kinds, []LifecycleKind{LifecycleSuspected, LifecycleRecovery}) {
		t.Fatalf("event kinds = %v", kinds)
	}
}

func TestManagerLifecycleStateSurvivesRestartInFlight(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	events := make(chan LifecycleEvent, 3)
	settings := enabledSettings()
	settings.TimeoutThreshold = 1
	manager := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(RestartReason) (RestartResult, error) {
			close(entered)
			<-release
			return RestartResult{Status: "launch_dispatched"}, nil
		},
		OnLifecycleEvent: func(event LifecycleEvent) {
			events <- event
		},
	})
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	if event := <-events; event.Kind != LifecycleSuspected {
		t.Fatalf("first event = %#v, want suspected", event)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("restart callback did not start")
	}
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	select {
	case event := <-events:
		t.Fatalf("restart-in-flight error emitted %#v, want no duplicate suspicion", event)
	default:
	}
	manager.NoteHealthy(RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true})
	select {
	case event := <-events:
		if event.Kind != LifecycleRecovery {
			t.Fatalf("healthy event = %#v, want recovery", event)
		}
	case <-time.After(time.Second):
		t.Fatal("healthy runtime did not emit recovery")
	}
	close(release)
	waitForRestartIdle(t, manager)
}

func TestManagerCircuitLifecycleEventsOnlyEmitOnTransition(t *testing.T) {
	events := make(chan LifecycleEvent, 5)
	settings := enabledSettings()
	settings.TimeoutThreshold = 1
	settings.MaxRestartsPer10Min = 1
	manager := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(RestartReason) (RestartResult, error) {
			return RestartResult{}, errors.New("restart failed")
		},
		OnLifecycleEvent: func(event LifecycleEvent) {
			events <- event
		},
	})
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	for i := 0; i < 3; i++ {
		select {
		case <-events:
		case <-time.After(time.Second):
			t.Fatal("automatic restart lifecycle events did not arrive")
		}
	}
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	event := <-events
	if event.Kind != LifecycleAbnormal || event.Error != circuitError {
		t.Fatalf("circuit event = %#v", event)
	}
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	select {
	case event := <-events:
		t.Fatalf("repeated circuit event = %#v", event)
	default:
	}
}

func TestManagerLifecycleCallbackCanReenterManager(t *testing.T) {
	callbackReturned := make(chan struct{}, 1)
	var manager *Manager
	manager = NewManager(ManagerOptions{
		Settings: enabledSettings(),
		OnLifecycleEvent: func(LifecycleEvent) {
			_ = manager.Status()
			callbackReturned <- struct{}{}
		},
	})
	manager.Arm("test")
	completed := make(chan struct{})
	go func() {
		manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
		close(completed)
	}()
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("lifecycle callback blocked while reentering manager")
	}
	select {
	case <-callbackReturned:
	case <-time.After(time.Second):
		t.Fatal("lifecycle callback did not return")
	}
}

func TestManagerSerializesConcurrentLifecycleDelivery(t *testing.T) {
	suspectedEntered := make(chan struct{})
	releaseSuspected := make(chan struct{})
	delivered := make(chan LifecycleKind, 2)
	settings := enabledSettings()
	settings.FailureRecoveryEnabled = false
	manager := NewManager(ManagerOptions{
		Settings: settings,
		OnLifecycleEvent: func(event LifecycleEvent) {
			if event.Kind == LifecycleSuspected {
				close(suspectedEntered)
				<-releaseSuspected
			}
			delivered <- event.Kind
		},
	})
	manager.Arm("test")
	go manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	select {
	case <-suspectedEntered:
	case <-time.After(time.Second):
		t.Fatal("suspected callback did not start")
	}
	healthyDone := make(chan struct{})
	go func() {
		manager.NoteHealthy(RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true})
		close(healthyDone)
	}()
	select {
	case <-healthyDone:
	case <-time.After(time.Second):
		t.Fatal("healthy transition blocked behind lifecycle callback")
	}
	close(releaseSuspected)
	got := []LifecycleKind{<-delivered, <-delivered}
	if !equalLifecycleKinds(got, []LifecycleKind{LifecycleSuspected, LifecycleRecovery}) {
		t.Fatalf("delivery order = %v", got)
	}
}

func TestManagerLifecycleCallbackCanCloseFromMonitorLoop(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	callbackDone := make(chan struct{})
	settings := enabledSettings()
	settings.TimeoutThreshold = 1
	var manager *Manager
	manager = NewManager(ManagerOptions{
		Settings: settings, Now: clock.Now, NewTimer: clock.NewTimer,
		Restart: func(RestartReason) (RestartResult, error) {
			return RestartResult{Status: "launch_dispatched"}, nil
		},
		OnLifecycleEvent: func(event LifecycleEvent) {
			if event.Kind != LifecycleRecovery {
				return
			}
			manager.Close()
			close(callbackDone)
		},
	})
	manager.Start(context.Background(), func() RuntimeSnapshot {
		return RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true}
	})
	clock.waitForTimers(t, 1)
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	waitFor(t, func() bool { return manager.Status().Phase == PhaseWaitingReconnect })
	clock.waitForTimerResets(t, 0, 1)
	clock.Advance(time.Second)
	select {
	case <-callbackDone:
	case <-time.After(time.Second):
		t.Fatal("lifecycle callback deadlocked while closing monitor loop")
	}
	manager.mu.Lock()
	running := manager.run != nil
	manager.mu.Unlock()
	if running {
		t.Fatal("manager still running after callback close")
	}
	waitForLifecycleDeliveryIdle(t, manager)
}

func TestManagerLifecycleCallbackCanStartFromMonitorLoop(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	callbackDone := make(chan struct{})
	settings := enabledSettings()
	settings.TimeoutThreshold = 1
	var manager *Manager
	snapshot := func() RuntimeSnapshot {
		return RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true}
	}
	manager = NewManager(ManagerOptions{
		Settings: settings, Now: clock.Now, NewTimer: clock.NewTimer,
		Restart: func(RestartReason) (RestartResult, error) {
			return RestartResult{Status: "launch_dispatched"}, nil
		},
		OnLifecycleEvent: func(event LifecycleEvent) {
			if event.Kind != LifecycleRecovery {
				return
			}
			manager.Start(context.Background(), snapshot)
			close(callbackDone)
		},
	})
	manager.Start(context.Background(), snapshot)
	clock.waitForTimers(t, 1)
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	waitFor(t, func() bool { return manager.Status().Phase == PhaseWaitingReconnect })
	clock.waitForTimerResets(t, 0, 1)
	clock.Advance(time.Second)
	select {
	case <-callbackDone:
	case <-time.After(time.Second):
		t.Fatal("lifecycle callback deadlocked while restarting monitor loop")
	}
	manager.Close()
	waitForLifecycleDeliveryIdle(t, manager)
}

func TestManagerClearsLifecycleStateOnDisarmAndGuardDisable(t *testing.T) {
	events := make(chan LifecycleEvent, 3)
	settings := enabledSettings()
	settings.FailureRecoveryEnabled = false
	manager := NewManager(ManagerOptions{
		Settings: settings,
		OnLifecycleEvent: func(event LifecycleEvent) {
			events <- event
		},
	})
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	manager.Disarm()
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	settings.Enabled = false
	manager.UpdateSettings(settings)
	settings.Enabled = true
	manager.UpdateSettings(settings)
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("timeout"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})

	if kinds := lifecycleKinds([]LifecycleEvent{<-events, <-events, <-events}); !equalLifecycleKinds(kinds, []LifecycleKind{LifecycleSuspected, LifecycleSuspected, LifecycleSuspected}) {
		t.Fatalf("event kinds = %v", kinds)
	}
}

func TestManagerTriggersRestartAfterTimeoutThreshold(t *testing.T) {
	restarted := make(chan struct{}, 1)
	settings := enabledSettings()
	manager := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(RestartReason) (RestartResult, error) {
			restarted <- struct{}{}
			return RestartResult{Status: "launch_dispatched"}, nil
		},
		Now: func() time.Time { return time.Unix(100, 0) },
	})
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "wechat_cdp"})
	manager.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "wechat_cdp"})
	select {
	case <-restarted:
	case <-time.After(time.Second):
		t.Fatal("automatic restart was not triggered")
	}
	waitFor(t, func() bool { return manager.Status().Phase == PhaseWaitingReconnect })
	status := manager.Status()
	if len(status.RecentRestartEvents) != 1 || status.RecentRestartEvents[0].Trigger != "auto" {
		t.Fatalf("expected automatic restart event, got %#v", status.RecentRestartEvents)
	}
}

func TestManagerRestartsAfterTerminalNetworkRecoveryFailure(t *testing.T) {
	restarted := make(chan RestartReason, 1)
	settings := enabledSettings()
	settings.TimeoutThreshold = 3
	manager := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(reason RestartReason) (RestartResult, error) {
			restarted <- reason
			return RestartResult{Status: "launch_dispatched"}, nil
		},
	})
	manager.Arm("test")

	if !manager.RestartAfterNetworkRecoveryFailure(RuntimeSnapshot{RuntimeTarget: "qq_ws"}, "reconnect timeout") {
		t.Fatal("terminal network recovery failure did not reserve an automatic restart")
	}

	select {
	case reason := <-restarted:
		if reason.Manual || reason.Scheduled {
			t.Fatalf("restart reason was not automatic: %#v", reason)
		}
		if reason.Reason != "network reconnect failed: reconnect timeout" {
			t.Fatalf("restart reason = %q", reason.Reason)
		}
	case <-time.After(time.Second):
		t.Fatal("automatic restart was not triggered")
	}
	waitFor(t, func() bool { return len(manager.Status().RecentRestartEvents) == 1 })
	if trigger := manager.Status().RecentRestartEvents[0].Trigger; trigger != "auto" {
		t.Fatalf("restart trigger = %q, want automatic", trigger)
	}
}

func TestManagerReportsLifecycleForTerminalNetworkRecoveryFailure(t *testing.T) {
	releaseRestart := make(chan struct{})
	events := make(chan LifecycleEvent, 2)
	settings := enabledSettings()
	settings.TimeoutThreshold = 3
	manager := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(RestartReason) (RestartResult, error) {
			<-releaseRestart
			return RestartResult{Status: "launch_dispatched"}, nil
		},
		OnLifecycleEvent: func(event LifecycleEvent) {
			events <- event
		},
	})
	manager.Arm("test")

	if !manager.RestartAfterNetworkRecoveryFailure(RuntimeSnapshot{RuntimeTarget: "qq_ws"}, "reconnect timeout") {
		t.Fatal("terminal network recovery failure did not reserve an automatic restart")
	}
	if streak := manager.Status().TimeoutStreak; streak != 1 {
		t.Fatalf("timeout streak = %d, want 1", streak)
	}

	select {
	case event := <-events:
		if event.Kind != LifecycleSuspected || event.RuntimeTarget != "qq_ws" || event.Error != "network reconnect failed: reconnect timeout" || event.Streak != 1 || event.Threshold != 3 {
			t.Fatalf("unexpected suspected event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("terminal network recovery failure did not emit suspected lifecycle event")
	}

	manager.NoteHealthy(RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true})
	select {
	case event := <-events:
		if event.Kind != LifecycleRecovery {
			t.Fatalf("healthy runtime event = %#v, want recovery", event)
		}
	case <-time.After(time.Second):
		t.Fatal("healthy runtime did not emit recovery lifecycle event")
	}
	close(releaseRestart)
	waitForRestartIdle(t, manager)
}

func TestManagerReportsTerminalNetworkRecoveryFailureWhenRecoveryDisabled(t *testing.T) {
	restarted := make(chan struct{}, 1)
	events := make(chan LifecycleEvent, 1)
	settings := enabledSettings()
	settings.FailureRecoveryEnabled = false
	settings.TimeoutThreshold = 3
	manager := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(RestartReason) (RestartResult, error) {
			restarted <- struct{}{}
			return RestartResult{Status: "launch_dispatched"}, nil
		},
		OnLifecycleEvent: func(event LifecycleEvent) {
			events <- event
		},
	})
	manager.Arm("test")

	if manager.RestartAfterNetworkRecoveryFailure(RuntimeSnapshot{RuntimeTarget: "qq_ws"}, "reconnect timeout") {
		t.Fatal("terminal network recovery failure reserved a restart while recovery was disabled")
	}
	if streak := manager.Status().TimeoutStreak; streak != 0 {
		t.Fatalf("suppressed terminal failure timeout streak = %d, want 0", streak)
	}

	select {
	case event := <-events:
		if event.Kind != LifecycleSuspected || event.Error != "network reconnect failed: reconnect timeout" || event.Streak != 1 {
			t.Fatalf("unexpected lifecycle event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("terminal network recovery failure did not emit suspected lifecycle event")
	}
	select {
	case <-restarted:
		t.Fatal("terminal network recovery failure dispatched a restart while recovery was disabled")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestManagerOpensCircuitAfterTooManyRestarts(t *testing.T) {
	now := time.Unix(100, 0)
	settings := enabledSettings()
	settings.TimeoutThreshold = 1
	settings.MaxRestartsPer10Min = 1
	manager := NewManager(ManagerOptions{
		Settings: settings,
		Restart:  func(RestartReason) (RestartResult, error) { return RestartResult{}, errors.New("restart failed") },
		Now:      func() time.Time { return now },
	})
	manager.Arm("test")
	manager.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	waitFor(t, func() bool { return len(manager.Status().RecentRestartEvents) == 1 })
	manager.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	if manager.Status().Phase != PhaseCircuitOpen {
		t.Fatalf("expected circuit open, got %#v", manager.Status())
	}
}

func TestManagerManualRestartWithoutCallbackPreservesCircuitBehavior(t *testing.T) {
	settings := enabledSettings()
	m := NewManager(ManagerOptions{Settings: settings})
	_, err := m.ManualRestart(RuntimeSnapshot{RuntimeTarget: "qq_ws"}, "test")
	if err == nil {
		t.Fatal("expected missing callback error")
	}
	status := m.Status()
	if !status.CircuitOpen || status.Phase != PhaseCircuitOpen || status.RestartCountInWindow != 0 {
		t.Fatalf("missing callback did not preserve circuit behavior: %#v", status)
	}
}

func TestManagerReconnectedRestartSkipsReconnectGrace(t *testing.T) {
	settings := enabledSettings()
	manager := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(RestartReason) (RestartResult, error) {
			return RestartResult{Status: RestartStatusReconnected, LaunchDispatched: true}, nil
		},
	})
	manager.Arm("test")

	result, err := manager.ManualRestart(RuntimeSnapshot{RuntimeTarget: "wechat_cdp"}, "operator request")
	if err != nil || result.Status != RestartStatusReconnected {
		t.Fatalf("restart = %#v, err = %v", result, err)
	}
	status := manager.Status()
	if status.Phase != PhaseWatching || status.ReconnectGraceUntil != "" || status.ReconnectGraceRemainingMS != 0 {
		t.Fatalf("reconnected restart entered grace: %#v", status)
	}
}

func TestManagerRunsScheduledRestartWithoutBeingArmed(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	restarted := make(chan RestartReason, 1)
	settings := enabledSettings()
	settings.ScheduledRestartEnabled = true
	settings.ScheduledRestartIntervalMin = 1
	m := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(reason RestartReason) (RestartResult, error) {
			restarted <- reason
			return RestartResult{Status: "launch_dispatched"}, nil
		},
		Now: clock.Now, NewTimer: clock.NewTimer,
	})
	m.Start(context.Background(), func() RuntimeSnapshot { return RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true} })
	defer m.Close()
	clock.waitForTimers(t, 1)
	clock.Advance(time.Minute)
	select {
	case reason := <-restarted:
		if !reason.Scheduled || reason.Manual {
			t.Fatalf("expected scheduled reason: %#v", reason)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduled restart was not triggered")
	}
}

func TestManagerScheduledRestartStatusUpdates(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	entered := make(chan struct{})
	release := make(chan struct{})
	settings := enabledSettings()
	settings.ScheduledRestartEnabled = true
	settings.ScheduledRestartIntervalMin = 1
	m := NewManager(ManagerOptions{
		Settings: settings, Now: clock.Now, NewTimer: clock.NewTimer,
		Restart: func(RestartReason) (RestartResult, error) {
			close(entered)
			<-release
			return RestartResult{Status: "launch_dispatched"}, nil
		},
	})
	m.Start(context.Background(), func() RuntimeSnapshot { return RuntimeSnapshot{RuntimeTarget: "qq_ws"} })
	defer m.Close()
	initial := m.Status()
	if initial.ScheduledRestartNextAt == "" || initial.ScheduledRestartRemainingMs != int64(time.Minute/time.Millisecond) {
		t.Fatalf("unexpected initial schedule: %#v", initial)
	}
	clock.waitForTimers(t, 1)
	clock.Advance(time.Minute)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("scheduled restart callback did not start")
	}
	running := m.Status()
	if running.ScheduledRestartLastResult != "running" || running.ScheduledRestartLastAt == "" || running.ScheduledRestartNextAt == "" {
		t.Fatalf("unexpected running schedule status: %#v", running)
	}
	close(release)
	waitFor(t, func() bool { return m.Status().ScheduledRestartLastResult == "ok" })
	status := m.Status()
	if status.ScheduledRestartLastError != "" || len(status.RecentRestartEvents) != 1 || status.RecentRestartEvents[0].Trigger != "scheduled" {
		t.Fatalf("unexpected completed schedule status: %#v", status)
	}
}

func TestManagerScheduledRestartDisabledClearsNextTime(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	settings := enabledSettings()
	settings.ScheduledRestartEnabled = true
	settings.ScheduledRestartIntervalMin = 2
	m := NewManager(ManagerOptions{Settings: settings, Now: clock.Now, NewTimer: clock.NewTimer})
	if m.Status().ScheduledRestartNextAt == "" {
		t.Fatal("expected initial scheduled restart time")
	}
	settings.ScheduledRestartEnabled = false
	m.UpdateSettings(settings)
	status := m.Status()
	if status.ScheduledRestartNextAt != "" || status.ScheduledRestartRemainingMs != 0 {
		t.Fatalf("disabled schedule was not cleared: %#v", status)
	}
}

func TestManagerScheduledRestartIntervalChangeResetsNextTime(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	settings := enabledSettings()
	settings.ScheduledRestartEnabled = true
	settings.ScheduledRestartIntervalMin = 2
	m := NewManager(ManagerOptions{Settings: settings, Now: clock.Now, NewTimer: clock.NewTimer})
	clock.Advance(30 * time.Second)
	settings.ScheduledRestartIntervalMin = 3
	m.UpdateSettings(settings)
	status := m.Status()
	expected := clock.Now().Add(3 * time.Minute).UTC().Format(time.RFC3339)
	if status.ScheduledRestartNextAt != expected || status.ScheduledRestartRemainingMs != int64(3*time.Minute/time.Millisecond) {
		t.Fatalf("schedule interval was not reset: %#v", status)
	}
}

func TestManagerAutomaticallyClosesCircuitAfterWindowExpires(t *testing.T) {
	now := time.Unix(100, 0)
	settings := enabledSettings()
	settings.TimeoutThreshold = 1
	settings.MaxRestartsPer10Min = 1
	m := NewManager(ManagerOptions{
		Settings: settings,
		Restart:  func(RestartReason) (RestartResult, error) { return RestartResult{}, errors.New("restart failed") },
		Now:      func() time.Time { return now },
	})
	m.Arm("test")
	m.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	waitFor(t, func() bool { return len(m.Status().RecentRestartEvents) == 1 })
	m.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	if !m.Status().CircuitOpen {
		t.Fatal("circuit did not open")
	}
	now = now.Add(11 * time.Minute)
	m.tick(now, RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	status := m.Status()
	if status.CircuitOpen || status.Phase != PhaseWatching || status.LastActionError != "" {
		t.Fatalf("circuit did not recover: %#v", status)
	}
}

func TestManagerClosesQuotaCircuitAfterVisibleErrorIsCleared(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	settings := enabledSettings()
	settings.TimeoutThreshold = 1
	settings.MaxRestartsPer10Min = 1
	m := NewManager(ManagerOptions{
		Settings: settings,
		Restart:  func(RestartReason) (RestartResult, error) { return RestartResult{}, errors.New("restart failed") },
		Now:      clock.Now,
		NewTimer: clock.NewTimer,
	})
	m.Arm("test")
	m.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	waitFor(t, func() bool { return len(m.Status().RecentRestartEvents) == 1 })
	m.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	if !m.Status().CircuitOpen {
		t.Fatal("quota circuit did not open")
	}
	m.NoteHealthy(RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true})
	if status := m.Status(); !status.CircuitOpen || status.LastActionError != "" {
		t.Fatalf("test did not clear only the visible circuit error: %#v", status)
	}
	m.Start(context.Background(), func() RuntimeSnapshot { return RuntimeSnapshot{RuntimeTarget: "qq_ws"} })
	defer m.Close()
	clock.waitForTimers(t, 1)
	clock.Advance(11 * time.Minute)
	waitFor(t, func() bool { return !m.Status().CircuitOpen })
	if status := m.Status(); status.Phase != PhaseWatching || status.RestartCountInWindow != 0 {
		t.Fatalf("quota circuit did not recover through worker loop: %#v", status)
	}
}

func TestManagerWorkerDelayIsCappedAtOneMinute(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	settings := enabledSettings()
	settings.MonitorIntervalMS = int((5 * time.Minute) / time.Millisecond)
	m := NewManager(ManagerOptions{Settings: settings, Now: clock.Now, NewTimer: clock.NewTimer})
	m.Start(context.Background(), nil)
	defer m.Close()
	clock.waitForTimers(t, 1)
	if delay := clock.timerDelay(0); delay != time.Minute {
		t.Fatalf("worker delay was not capped: %s", delay)
	}
}

func TestManagerManualAndScheduledRestartsDoNotConsumeAutomaticQuota(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	settings := enabledSettings()
	settings.MaxRestartsPer10Min = 1
	settings.ScheduledRestartEnabled = true
	settings.ScheduledRestartIntervalMin = 1
	var manualCalls atomic.Int32
	var scheduledCalls atomic.Int32
	m := NewManager(ManagerOptions{
		Settings: settings, Now: clock.Now,
		Restart: func(reason RestartReason) (RestartResult, error) {
			switch restartTrigger(reason) {
			case "manual":
				manualCalls.Add(1)
			case "scheduled":
				scheduledCalls.Add(1)
			}
			return RestartResult{}, errors.New("restart failed")
		},
	})
	if _, err := m.ManualRestart(RuntimeSnapshot{RuntimeTarget: "qq_ws"}, "operator request"); err == nil {
		t.Fatal("expected manual restart failure")
	}
	clock.Advance(time.Minute)
	m.tick(clock.Now(), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	waitForRestartIdle(t, m)
	status := m.Status()
	if status.CircuitOpen || status.RestartCountInWindow != 0 {
		t.Fatalf("routine restarts consumed automatic quota: %#v", status)
	}
	if manualCalls.Load() != 1 || scheduledCalls.Load() != 1 {
		t.Fatalf("restart calls: manual=%d scheduled=%d", manualCalls.Load(), scheduledCalls.Load())
	}
}

func TestManagerAutomaticCircuitAllowsRoutineRestarts(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	settings := enabledSettings()
	settings.TimeoutThreshold = 1
	settings.MaxRestartsPer10Min = 1
	settings.ScheduledRestartEnabled = true
	settings.ScheduledRestartIntervalMin = 1
	var automaticCalls atomic.Int32
	var manualCalls atomic.Int32
	var scheduledCalls atomic.Int32
	m := NewManager(ManagerOptions{
		Settings: settings, Now: clock.Now,
		Restart: func(reason RestartReason) (RestartResult, error) {
			switch restartTrigger(reason) {
			case "auto":
				automaticCalls.Add(1)
			case "manual":
				manualCalls.Add(1)
			case "scheduled":
				scheduledCalls.Add(1)
			}
			return RestartResult{}, errors.New("restart failed")
		},
	})
	m.Arm("test")
	snapshot := RuntimeSnapshot{RuntimeTarget: "qq_ws"}
	m.NoteRuntimeError(errors.New("context deadline exceeded"), snapshot)
	waitForRestartIdle(t, m)
	m.NoteRuntimeError(errors.New("context deadline exceeded"), snapshot)
	if status := m.Status(); !status.CircuitOpen || status.RestartCountInWindow != 1 {
		t.Fatalf("automatic circuit did not open: %#v", status)
	}

	if _, err := m.ManualRestart(snapshot, "operator request"); err == nil {
		t.Fatal("expected manual restart failure")
	}
	clock.Advance(time.Minute)
	m.tick(clock.Now(), snapshot)
	waitForRestartIdle(t, m)
	m.NoteRuntimeError(errors.New("context deadline exceeded"), snapshot)

	if automaticCalls.Load() != 1 || manualCalls.Load() != 1 || scheduledCalls.Load() != 1 {
		t.Fatalf("restart calls: automatic=%d manual=%d scheduled=%d", automaticCalls.Load(), manualCalls.Load(), scheduledCalls.Load())
	}
	if status := m.Status(); !status.CircuitOpen || status.RestartCountInWindow != 1 || status.Phase != PhaseCircuitOpen {
		t.Fatalf("unexpected automatic circuit status: %#v", status)
	}
}

func TestManagerGlobalSafetyLimitRejectsNinthMixedRestart(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	settings := enabledSettings()
	settings.TimeoutThreshold = 1
	settings.MaxRestartsPer10Min = 8
	settings.ScheduledRestartEnabled = true
	settings.ScheduledRestartIntervalMin = 1
	var calls atomic.Int32
	m := NewManager(ManagerOptions{
		Settings: settings, Now: clock.Now,
		Restart: func(RestartReason) (RestartResult, error) {
			calls.Add(1)
			return RestartResult{}, errors.New("restart failed")
		},
	})
	m.Arm("test")
	snapshot := RuntimeSnapshot{RuntimeTarget: "qq_ws"}
	for index := 0; index < 4; index++ {
		m.NoteRuntimeError(errors.New("context deadline exceeded"), snapshot)
		waitForRestartIdle(t, m)
	}
	for index := 0; index < 2; index++ {
		if _, err := m.ManualRestart(snapshot, "operator request"); err == nil {
			t.Fatal("expected manual restart failure")
		}
	}
	for index := 0; index < 2; index++ {
		clock.Advance(time.Minute)
		m.tick(clock.Now(), snapshot)
		waitForRestartIdle(t, m)
	}

	result, err := m.ManualRestart(snapshot, "operator request")
	const expectedError = "10 minutes total restart safety limit reached"
	if err != nil || result.Status != "circuit_open" || result.Reason != expectedError {
		t.Fatalf("ninth restart result = %#v, err = %v", result, err)
	}
	status := m.Status()
	if calls.Load() != 8 || !status.CircuitOpen || status.RestartCountInWindow != 4 || status.LastActionError != expectedError {
		t.Fatalf("global safety status = %#v, calls = %d", status, calls.Load())
	}
}

func TestManagerCircuitCausesExpireIndependently(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	settings := enabledSettings()
	settings.TimeoutThreshold = 1
	settings.MaxRestartsPer10Min = 4
	var automaticCalls atomic.Int32
	var totalCalls atomic.Int32
	m := NewManager(ManagerOptions{
		Settings: settings, Now: clock.Now,
		Restart: func(reason RestartReason) (RestartResult, error) {
			totalCalls.Add(1)
			if restartTrigger(reason) == "auto" {
				automaticCalls.Add(1)
			}
			return RestartResult{}, errors.New("restart failed")
		},
	})
	snapshot := RuntimeSnapshot{RuntimeTarget: "qq_ws"}
	for index := 0; index < 4; index++ {
		_, _ = m.ManualRestart(snapshot, "operator request")
	}
	clock.Advance(5 * time.Minute)
	m.Arm("test")
	for index := 0; index < 4; index++ {
		m.NoteRuntimeError(errors.New("context deadline exceeded"), snapshot)
		waitForRestartIdle(t, m)
	}
	m.NoteRuntimeError(errors.New("context deadline exceeded"), snapshot)
	result, err := m.ManualRestart(snapshot, "operator request")
	if err != nil || result.Status != "circuit_open" || result.Reason != globalCircuitError {
		t.Fatalf("global circuit result = %#v, err = %v", result, err)
	}

	clock.Advance(6 * time.Minute)
	status := m.Status()
	if !status.CircuitOpen || status.LastActionError != circuitError || status.RestartCountInWindow != 4 {
		t.Fatalf("automatic circuit did not survive global expiry: %#v", status)
	}
	if _, err := m.ManualRestart(snapshot, "operator request"); err == nil {
		t.Fatal("expected allowed manual restart to reach callback")
	}
	m.NoteRuntimeError(errors.New("context deadline exceeded"), snapshot)
	if automaticCalls.Load() != 4 || totalCalls.Load() != 9 {
		t.Fatalf("restart calls after global expiry: automatic=%d total=%d", automaticCalls.Load(), totalCalls.Load())
	}

	clock.Advance(5 * time.Minute)
	if status := m.Status(); status.CircuitOpen || status.RestartCountInWindow != 0 {
		t.Fatalf("automatic circuit did not expire: %#v", status)
	}
	m.NoteRuntimeError(errors.New("context deadline exceeded"), snapshot)
	waitForRestartIdle(t, m)
	if automaticCalls.Load() != 5 || totalCalls.Load() != 10 {
		t.Fatalf("automatic restart did not resume: automatic=%d total=%d", automaticCalls.Load(), totalCalls.Load())
	}
}

func TestManagerSafetyCircuitWakeUsesOldestTotalAttempt(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	settings := enabledSettings()
	settings.MonitorIntervalMS = int(time.Minute / time.Millisecond)
	m := NewManager(ManagerOptions{
		Settings: settings, Now: clock.Now,
		Restart: func(RestartReason) (RestartResult, error) {
			return RestartResult{}, errors.New("restart failed")
		},
	})
	snapshot := RuntimeSnapshot{RuntimeTarget: "qq_ws"}
	for index := 0; index < globalRestartLimit; index++ {
		_, _ = m.ManualRestart(snapshot, "operator request")
	}
	result, err := m.ManualRestart(snapshot, "operator request")
	if err != nil || result.Status != "circuit_open" {
		t.Fatalf("safety circuit result = %#v, err = %v", result, err)
	}
	clock.Advance(9*time.Minute + 30*time.Second)
	if delay := m.nextWakeDelay(); delay != 30*time.Second {
		t.Fatalf("safety circuit wake delay = %s, want 30s", delay)
	}
}

func TestManagerScheduledRestartFailureUpdatesStatus(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	settings := enabledSettings()
	settings.ScheduledRestartEnabled = true
	settings.ScheduledRestartIntervalMin = 1
	m := NewManager(ManagerOptions{
		Settings: settings, Now: clock.Now,
		Restart: func(RestartReason) (RestartResult, error) {
			return RestartResult{}, errors.New("scheduled launch failed")
		},
	})
	clock.Advance(time.Minute)
	m.tick(clock.Now(), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	waitFor(t, func() bool { return m.Status().ScheduledRestartLastResult == "error" })
	status := m.Status()
	if status.ScheduledRestartLastError != "scheduled launch failed" || len(status.RecentRestartEvents) != 1 || status.RecentRestartEvents[0].Trigger != "scheduled" || status.RecentRestartEvents[0].OK {
		t.Fatalf("scheduled failure status was incomplete: %#v", status)
	}
}

func TestManagerSuccessfulManualRestartRecordsManualHistory(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	settings := enabledSettings()
	m := NewManager(ManagerOptions{
		Settings: settings, Now: clock.Now,
		Restart: func(reason RestartReason) (RestartResult, error) {
			if !reason.Manual || reason.Scheduled || reason.Reason != "operator request" {
				t.Fatalf("unexpected manual reason: %#v", reason)
			}
			return RestartResult{Status: "launch_dispatched"}, nil
		},
	})
	result, err := m.ManualRestart(RuntimeSnapshot{RuntimeTarget: "wechat_cdp"}, "operator request")
	if err != nil || result.Status != "launch_dispatched" {
		t.Fatalf("manual restart failed: result=%#v err=%v", result, err)
	}
	status := m.Status()
	if status.Phase != PhaseWaitingReconnect || len(status.RecentRestartEvents) != 1 || status.RecentRestartEvents[0].Trigger != "manual" || !status.RecentRestartEvents[0].OK {
		t.Fatalf("manual restart history was not preserved: %#v", status)
	}
}

func TestManagerRestartCallbackDoesNotHoldMutex(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	settings := enabledSettings()
	settings.TimeoutThreshold = 1
	m := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(RestartReason) (RestartResult, error) {
			close(entered)
			<-release
			return RestartResult{Status: "launch_dispatched"}, nil
		},
	})
	m.Arm("test")
	m.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("restart callback did not start")
	}
	statusDone := make(chan struct{})
	go func() { _ = m.Status(); close(statusDone) }()
	select {
	case <-statusDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Status blocked behind restart callback")
	}
	healthyDone := make(chan struct{})
	go func() {
		m.NoteHealthy(RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true})
		close(healthyDone)
	}()
	select {
	case <-healthyDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("NoteHealthy blocked behind restart callback")
	}
	close(release)
}

func TestManagerCloseDiscardsStaleAutomaticRestartCompletion(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	callbackReturned := make(chan struct{})
	settings := enabledSettings()
	settings.TimeoutThreshold = 1
	m := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(RestartReason) (RestartResult, error) {
			close(entered)
			<-release
			close(callbackReturned)
			return RestartResult{Status: "launch_dispatched"}, nil
		},
	})
	m.Arm("test")
	m.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("restart callback did not start")
	}
	m.Close()
	before := m.Status()
	close(release)
	select {
	case <-callbackReturned:
	case <-time.After(time.Second):
		t.Fatal("restart callback did not return")
	}
	waitForRestartIdle(t, m)
	after := m.Status()
	if len(after.RecentRestartEvents) != len(before.RecentRestartEvents) || after.ReconnectGraceUntil != "" || after.Phase != PhaseWatching || after.LastActionError != before.LastActionError {
		t.Fatalf("stale restart completion was not reconciled: before=%#v after=%#v", before, after)
	}
}

func TestManagerReplacementStartReconcilesStaleScheduledRestart(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	entered := make(chan struct{})
	release := make(chan struct{})
	settings := enabledSettings()
	settings.ScheduledRestartEnabled = true
	settings.ScheduledRestartIntervalMin = 1
	m := NewManager(ManagerOptions{
		Settings: settings, Now: clock.Now, NewTimer: clock.NewTimer,
		Restart: func(RestartReason) (RestartResult, error) {
			close(entered)
			<-release
			return RestartResult{Status: "launch_dispatched"}, nil
		},
	})
	snapshot := func() RuntimeSnapshot { return RuntimeSnapshot{RuntimeTarget: "qq_ws"} }
	m.Start(context.Background(), snapshot)
	clock.waitForTimers(t, 1)
	clock.Advance(time.Minute)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("scheduled restart callback did not start")
	}
	if status := m.Status(); status.Phase != PhaseRestarting || status.ScheduledRestartLastResult != "running" {
		t.Fatalf("scheduled restart was not reserved: %#v", status)
	}
	m.Start(context.Background(), snapshot)
	clock.waitForTimers(t, 2)
	close(release)
	waitForRestartIdle(t, m)
	status := m.Status()
	if status.Phase != PhaseStandby || status.ScheduledRestartLastResult == "running" || len(status.RecentRestartEvents) != 0 || status.ReconnectGraceUntil != "" {
		t.Fatalf("stale scheduled restart was not reconciled: %#v", status)
	}
	m.Close()
}

func TestManagerCloseDiscardsStaleManualRestartCompletion(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	manualDone := make(chan error, 1)
	settings := enabledSettings()
	m := NewManager(ManagerOptions{
		Settings: settings,
		Restart: func(RestartReason) (RestartResult, error) {
			close(entered)
			<-release
			return RestartResult{}, errors.New("stale manual failure")
		},
	})
	go func() {
		_, err := m.ManualRestart(RuntimeSnapshot{RuntimeTarget: "qq_ws"}, "operator request")
		manualDone <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("manual restart callback did not start")
	}
	m.Close()
	close(release)
	select {
	case err := <-manualDone:
		if err == nil || err.Error() != "stale manual failure" {
			t.Fatalf("manual caller did not receive callback error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("manual restart did not return")
	}
	waitForRestartIdle(t, m)
	status := m.Status()
	if status.Phase != PhaseStandby || len(status.RecentRestartEvents) != 0 || status.LastActionError != "" || status.ReconnectGraceUntil != "" {
		t.Fatalf("stale manual restart mutated status: %#v", status)
	}
}

func TestManagerReconnectGraceSuppressesRepeatsAndExpires(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	var restarts atomic.Int32
	settings := enabledSettings()
	settings.TimeoutThreshold = 1
	settings.RestartReconnectGraceSec = 5
	m := NewManager(ManagerOptions{
		Settings: settings, Now: clock.Now, NewTimer: clock.NewTimer,
		Restart: func(RestartReason) (RestartResult, error) {
			restarts.Add(1)
			return RestartResult{Status: "launch_dispatched"}, nil
		},
	})
	m.Arm("test")
	m.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	waitFor(t, func() bool { return m.Status().Phase == PhaseWaitingReconnect })
	m.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	if restarts.Load() != 1 || m.Status().ReconnectGraceRemainingMS != 5000 {
		t.Fatalf("restart repeated during grace: restarts=%d status=%#v", restarts.Load(), m.Status())
	}
	clock.Advance(6 * time.Second)
	m.tick(clock.Now(), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	if status := m.Status(); status.Phase != PhaseWatching || status.ReconnectGraceRemainingMS != 0 || status.ReconnectGraceUntil != "" {
		t.Fatalf("grace did not expire cleanly: %#v", status)
	}
	m.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	waitFor(t, func() bool { return restarts.Load() == 2 })
}

func TestManagerReconnectGraceSuppressesScheduledRestart(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	var restarts atomic.Int32
	settings := enabledSettings()
	settings.RestartReconnectGraceSec = 120
	settings.ScheduledRestartEnabled = true
	settings.ScheduledRestartIntervalMin = 1
	m := NewManager(ManagerOptions{
		Settings: settings, Now: clock.Now, NewTimer: clock.NewTimer,
		Restart: func(RestartReason) (RestartResult, error) {
			restarts.Add(1)
			return RestartResult{Status: "launch_dispatched"}, nil
		},
	})
	m.Start(context.Background(), func() RuntimeSnapshot { return RuntimeSnapshot{RuntimeTarget: "qq_ws"} })
	defer m.Close()
	clock.waitForTimers(t, 1)
	clock.Advance(time.Minute)
	waitFor(t, func() bool { return m.Status().Phase == PhaseWaitingReconnect })
	clock.waitForTimerResets(t, 0, 1)
	clock.Advance(time.Minute)
	wantCheck := clock.Now().UTC().Format(time.RFC3339)
	waitFor(t, func() bool { return m.Status().LastCheckAt == wantCheck })
	if restarts.Load() != 1 {
		t.Fatalf("scheduled restart repeated during grace: %d", restarts.Load())
	}
}

func TestManagerHealthySnapshotClearsReconnectGrace(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	settings := enabledSettings()
	settings.TimeoutThreshold = 1
	m := NewManager(ManagerOptions{
		Settings: settings, Now: clock.Now,
		Restart: func(RestartReason) (RestartResult, error) { return RestartResult{Status: "launch_dispatched"}, nil },
	})
	m.Arm("test")
	m.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	waitFor(t, func() bool { return m.Status().Phase == PhaseWaitingReconnect })
	m.tick(clock.Now(), RuntimeSnapshot{RuntimeTarget: "qq_ws", Ready: true, Connected: true})
	if status := m.Status(); status.Phase != PhaseWatching || status.ReconnectGraceUntil != "" {
		t.Fatalf("healthy snapshot did not clear grace: %#v", status)
	}
}

func TestManagerCloseStopsLoopAndStartDoesNotDuplicateIt(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	var checks atomic.Int32
	settings := enabledSettings()
	settings.MonitorIntervalMS = 1000
	m := NewManager(ManagerOptions{Settings: settings, Now: clock.Now, NewTimer: clock.NewTimer})
	snapshot := func() RuntimeSnapshot { checks.Add(1); return RuntimeSnapshot{RuntimeTarget: "qq_ws"} }
	m.Start(context.Background(), snapshot)
	clock.waitForTimers(t, 1)
	m.Start(context.Background(), snapshot)
	clock.waitForTimers(t, 2)
	clock.waitForTimerStops(t, 0, 1)
	clock.Advance(time.Second)
	waitFor(t, func() bool { return checks.Load() == 1 })
	if got, want := m.Status().LastCheckAt, clock.Now().UTC().Format(time.RFC3339); got != want {
		t.Fatalf("loop tick did not update LastCheckAt: got %q want %q", got, want)
	}
	if checks.Load() != 1 {
		t.Fatalf("duplicate loops ran: checks=%d", checks.Load())
	}
	stopsBeforeClose := clock.timerStopCount(1)
	m.Close()
	m.Close()
	clock.waitForTimerStops(t, 1, stopsBeforeClose+1)
	clock.Advance(time.Minute)
	if checks.Load() != 1 {
		t.Fatalf("loop continued after Close: checks=%d", checks.Load())
	}
}

func TestManagerReusesCurrentLoopTimerAndTargetsWake(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	settings := enabledSettings()
	settings.MonitorIntervalMS = 5000
	m := NewManager(ManagerOptions{Settings: settings, Now: clock.Now, NewTimer: clock.NewTimer})
	m.Start(context.Background(), nil)
	clock.waitForTimers(t, 1)
	m.Start(context.Background(), nil)
	clock.waitForTimers(t, 2)
	clock.waitForTimerStops(t, 0, 1)
	for reset := 1; reset <= 3; reset++ {
		settings.MonitorIntervalMS += 1000
		m.UpdateSettings(settings)
		clock.waitForTimerResets(t, 1, reset)
	}
	if count := clock.timerCount(); count != 2 {
		t.Fatalf("worker abandoned timers on wake: count=%d", count)
	}
	if resets := clock.timerResetCount(0); resets != 0 {
		t.Fatalf("canceled loop consumed current wake: old resets=%d", resets)
	}
	m.Close()
	clock.waitForTimerStops(t, 1, 1)
}

func TestManagerFailureRecoveryDisabledStillRunsScheduledRestart(t *testing.T) {
	clock := newGuardTestClock(time.Unix(100, 0))
	restarted := make(chan RestartReason, 2)
	settings := enabledSettings()
	settings.FailureRecoveryEnabled = false
	settings.TimeoutThreshold = 1
	settings.ScheduledRestartEnabled = true
	settings.ScheduledRestartIntervalMin = 1
	m := NewManager(ManagerOptions{
		Settings: settings, Now: clock.Now, NewTimer: clock.NewTimer,
		Restart: func(reason RestartReason) (RestartResult, error) {
			restarted <- reason
			return RestartResult{Status: "launch_dispatched"}, nil
		},
	})
	m.Arm("test")
	m.NoteRuntimeError(errors.New("context deadline exceeded"), RuntimeSnapshot{RuntimeTarget: "qq_ws"})
	select {
	case reason := <-restarted:
		t.Fatalf("failure recovery restart ran while disabled: %#v", reason)
	default:
	}
	m.Start(context.Background(), func() RuntimeSnapshot { return RuntimeSnapshot{RuntimeTarget: "qq_ws"} })
	defer m.Close()
	clock.waitForTimers(t, 1)
	clock.Advance(time.Minute)
	select {
	case reason := <-restarted:
		if !reason.Scheduled {
			t.Fatalf("expected scheduled restart, got %#v", reason)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduled restart did not run")
	}
}
