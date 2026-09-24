package main

import (
	"context"
	"sync"

	"Farm_Go/internal/farm/automation"
)

// automationExecutionGate coordinates all user automation work. Guardian work
// deliberately stays outside this gate so it can recover the runtime independently.
type automationExecutionGate struct {
	mu          sync.Mutex
	activeGod   int
	activeSafe  bool
	waitingSafe int
	changed     chan struct{}
}

func newAutomationExecutionGate() *automationExecutionGate {
	return &automationExecutionGate{changed: make(chan struct{})}
}

func (g *automationExecutionGate) Acquire(ctx context.Context, mode automation.RunMode) (func(), error) {
	isSafe := mode.IsSafe()
	g.mu.Lock()
	if isSafe {
		g.waitingSafe++
	}
	for {
		if g.tryAcquireLocked(mode) {
			if isSafe {
				g.waitingSafe--
			}
			g.mu.Unlock()
			return g.release(mode), nil
		}
		changed := g.changed
		g.mu.Unlock()

		select {
		case <-ctx.Done():
			g.mu.Lock()
			if isSafe {
				g.waitingSafe--
				g.notifyLocked()
			}
			g.mu.Unlock()
			return nil, ctx.Err()
		case <-changed:
			g.mu.Lock()
		}
	}
}

func (g *automationExecutionGate) TryAcquire(mode automation.RunMode) (func(), bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.tryAcquireLocked(mode) {
		return nil, false
	}
	return g.release(mode), true
}

func (g *automationExecutionGate) tryAcquireLocked(mode automation.RunMode) bool {
	if mode.IsSafe() {
		if g.activeSafe || g.activeGod > 0 {
			return false
		}
		g.activeSafe = true
		return true
	}
	if g.activeSafe || g.waitingSafe > 0 {
		return false
	}
	g.activeGod++
	return true
}

func (g *automationExecutionGate) release(mode automation.RunMode) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			if mode.IsSafe() {
				g.activeSafe = false
			} else if g.activeGod > 0 {
				g.activeGod--
			}
			g.notifyLocked()
			g.mu.Unlock()
		})
	}
}

func (g *automationExecutionGate) notifyLocked() {
	close(g.changed)
	g.changed = make(chan struct{})
}
