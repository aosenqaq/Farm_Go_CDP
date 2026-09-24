package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrRuntimeLinkNotConnected = errors.New("runtime link is not connected")

type Supervisor struct {
	manager *Manager
	links   map[RuntimeTarget]RuntimeLink

	mu     sync.Mutex
	active RuntimeLink
}

func NewSupervisor(manager *Manager, links map[RuntimeTarget]RuntimeLink) *Supervisor {
	if manager == nil {
		manager = NewManager()
	}
	if links == nil {
		links = map[RuntimeTarget]RuntimeLink{}
	}
	return &Supervisor{
		manager: manager,
		links:   links,
	}
}

func (s *Supervisor) Register(target RuntimeTarget, link RuntimeLink) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.links == nil {
		s.links = map[RuntimeTarget]RuntimeLink{}
	}
	if s.active != nil && s.active.Target() == target {
		if err := stopRuntimeLinkOwner(context.Background(), s.active); err != nil {
			s.manager.SetStatus(Status{
				Target:    string(s.active.Target()),
				Phase:     PhaseError,
				LastError: err.Error(),
			})
		}
		s.active = nil
	}
	s.links[target] = link
}

func (s *Supervisor) Switch(ctx context.Context, target RuntimeTarget) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.links[target]
	if next == nil {
		err := fmt.Errorf("runtime link target %q is not registered", target)
		s.manager.SetStatus(Status{
			Target:    string(target),
			Phase:     PhaseError,
			LastError: err.Error(),
		})
		return err
	}

	if s.active != nil {
		if err := stopRuntimeLinkOwner(ctx, s.active); err != nil {
			s.manager.SetStatus(Status{
				Target:    string(s.active.Target()),
				Phase:     PhaseError,
				LastError: err.Error(),
			})
			return err
		}
	}

	s.active = next
	if err := next.Start(ctx); err != nil {
		s.manager.SetStatus(Status{
			Target:    string(target),
			Phase:     PhaseError,
			LastError: err.Error(),
		})
		return err
	}
	s.manager.SetStatus(next.Status())
	return nil
}

func (s *Supervisor) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active == nil {
		return s.manager.Status()
	}
	return s.active.Status()
}

func (s *Supervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	active := s.active
	s.active = nil
	s.mu.Unlock()

	if active == nil {
		return nil
	}
	if err := stopRuntimeLinkOwner(ctx, active); err != nil {
		s.manager.SetStatus(Status{
			Target:    string(active.Target()),
			Phase:     PhaseError,
			LastError: err.Error(),
		})
		return err
	}
	return nil
}

func stopRuntimeLinkOwner(ctx context.Context, link RuntimeLink) error {
	if stopper, ok := link.(RuntimeLinkOwnerStopper); ok {
		return stopper.StopAndWait(ctx)
	}
	return link.Stop(ctx)
}

func (s *Supervisor) Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error) {
	s.mu.Lock()
	active := s.active
	s.mu.Unlock()

	if active == nil {
		return nil, ErrRuntimeLinkNotConnected
	}
	return active.Call(ctx, method, args, timeout)
}
