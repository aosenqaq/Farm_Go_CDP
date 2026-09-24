package runtime

import "sync"

type Manager struct {
	mu        sync.RWMutex
	status    Status
	listeners []func(previous Status, next Status)
}

func NewManager() *Manager {
	return &Manager{status: InitialStatus()}
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) SetStatus(status Status) {
	m.mu.Lock()
	previous := m.status
	m.status = status
	listeners := append([]func(previous Status, next Status){}, m.listeners...)
	m.mu.Unlock()

	for _, listener := range listeners {
		listener(previous, status)
	}
}

func (m *Manager) OnStatusChange(listener func(previous Status, next Status)) {
	if listener == nil {
		return
	}
	m.mu.Lock()
	m.listeners = append(m.listeners, listener)
	m.mu.Unlock()
}
