package qqlink

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"Farm_Go/internal/config"
	farmruntime "Farm_Go/internal/runtime"
	"Farm_Go/internal/runtime/qqws"
)

type Link struct {
	adapter *qqws.Adapter
	manager *farmruntime.Manager

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func New(cfg config.QQWSConfig, manager *farmruntime.Manager) *Link {
	return &Link{
		adapter: qqws.New(cfg, manager),
		manager: manager,
	}
}

func (l *Link) Target() farmruntime.RuntimeTarget {
	return farmruntime.RuntimeTargetQQWS
}

func (l *Link) Start(ctx context.Context) error {
	l.mu.Lock()
	if l.cancel != nil {
		l.cancel()
	}
	runCtx, cancel := context.WithCancel(ctx)
	l.cancel = cancel
	done := make(chan struct{})
	l.done = done
	l.mu.Unlock()

	go func() {
		defer close(done)
		if err := l.adapter.Start(runCtx); err != nil {
			slog.Error("qqws adapter stopped", "error", err)
		}
	}()
	return nil
}

func (l *Link) Stop(ctx context.Context) error {
	l.mu.Lock()
	cancel := l.cancel
	l.cancel = nil
	l.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	l.adapter.RequestStop()
	l.manager.SetStatus(farmruntime.Status{
		Target: string(farmruntime.RuntimeTargetQQWS),
		Phase:  farmruntime.PhaseDisconnected,
	})
	return nil
}

func (l *Link) StopAndWait(ctx context.Context) error {
	l.mu.Lock()
	done := l.done
	l.mu.Unlock()
	if err := l.Stop(ctx); err != nil {
		return err
	}
	if err := l.adapter.CloseAndWaitContext(ctx); err != nil {
		return err
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (l *Link) Status() farmruntime.Status {
	return l.manager.Status()
}

func (l *Link) Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error) {
	return l.adapter.Call(ctx, method, args, timeout)
}

func (l *Link) OnRuntimeEvent(handler func(map[string]any)) func() {
	return l.adapter.OnRuntimeEvent(handler)
}

// OnLog forwards log packets from the JS host script to the given handler.
func (l *Link) OnLog(handler func(level, message string, data map[string]any)) func() {
	return l.adapter.OnLog(handler)
}
