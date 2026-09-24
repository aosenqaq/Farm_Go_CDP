package guard

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type RecoveryKind string

const (
	RecoveryNetworkReconnect RecoveryKind = "network_reconnect"
	RecoveryOtherPlaceLogin  RecoveryKind = "other_place_login"
	RecoveryProcessRestart   RecoveryKind = "process_restart"
)

type RecoveryRequest struct {
	Kind          RecoveryKind
	Reason        string
	RuntimeTarget string
	RequestedAt   time.Time
}

type RecoveryResult struct {
	OK    bool
	Kind  RecoveryKind
	Error string
}

var ErrStaleRuntimeGeneration = errors.New("stale runtime generation")

type CoordinatorOptions struct {
	Run      func(context.Context, RecoveryRequest) RecoveryResult
	OnPause  func(bool)
	OnResult func(RecoveryRequest, RecoveryResult)
}

type RecoveryCoordinator struct {
	options CoordinatorOptions

	publicationMu sync.Mutex
	beforePublish func()

	mu              sync.Mutex
	pending         map[RecoveryKind]RecoveryRequest
	workerLifecycle uint64
	workerCancel    context.CancelFunc
	workerWake      chan struct{}

	runToken chan struct{}

	runtimeGeneration atomic.Uint64
}

func NewRecoveryCoordinator(options CoordinatorOptions) *RecoveryCoordinator {
	coordinator := &RecoveryCoordinator{
		options:  options,
		pending:  make(map[RecoveryKind]RecoveryRequest),
		runToken: make(chan struct{}, 1),
	}
	coordinator.runToken <- struct{}{}
	return coordinator
}

func (c *RecoveryCoordinator) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	c.publicationMu.Lock()
	defer c.publicationMu.Unlock()
	c.mu.Lock()
	if c.workerCancel != nil {
		c.workerCancel()
	}
	c.workerLifecycle++
	lifecycle := c.workerLifecycle
	workerCtx, cancel := context.WithCancel(ctx)
	wake := make(chan struct{}, 1)
	c.workerCancel = cancel
	c.workerWake = wake
	c.mu.Unlock()

	signal(wake)
	go c.work(workerCtx, lifecycle, wake)
}

func (c *RecoveryCoordinator) Close() {
	c.publicationMu.Lock()
	defer c.publicationMu.Unlock()
	c.mu.Lock()
	if c.workerCancel != nil {
		c.workerCancel()
		c.workerCancel = nil
	}
	c.workerWake = nil
	c.workerLifecycle++
	c.mu.Unlock()
}

// Submit coalesces duplicate pending kinds. The original RequestedAt is kept,
// while later non-empty reason and target values refresh the pending metadata.
func (c *RecoveryCoordinator) Submit(request RecoveryRequest) bool {
	if !validRecoveryKind(request.Kind) {
		return false
	}
	if request.RequestedAt.IsZero() {
		request.RequestedAt = time.Now()
	}

	c.mu.Lock()
	if pending, exists := c.pending[request.Kind]; exists {
		if request.Reason != "" {
			pending.Reason = request.Reason
		}
		if request.RuntimeTarget != "" {
			pending.RuntimeTarget = request.RuntimeTarget
		}
		c.pending[request.Kind] = pending
		c.mu.Unlock()
		return false
	}
	c.pending[request.Kind] = request
	c.mu.Unlock()

	c.signalWorker()
	return true
}

func (c *RecoveryCoordinator) Generation() uint64 {
	return c.runtimeGeneration.Load()
}

func (c *RecoveryCoordinator) AdvanceGeneration(reason string) uint64 {
	_ = reason
	return c.runtimeGeneration.Add(1)
}

func (c *RecoveryCoordinator) IsCurrent(generation uint64) bool {
	return c.runtimeGeneration.Load() == generation
}

func (c *RecoveryCoordinator) work(ctx context.Context, lifecycle uint64, wake <-chan struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-wake:
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-c.runToken:
			}

			if !c.processNext(ctx, lifecycle) {
				if ctx.Err() != nil || !c.isWorkerCurrent(lifecycle) {
					return
				}
				break
			}
		}
	}
}

func (c *RecoveryCoordinator) processNext(ctx context.Context, lifecycle uint64) bool {
	defer func() { c.runToken <- struct{}{} }()
	request, ok := c.takeNext(ctx, lifecycle)
	if !ok {
		return false
	}
	result := c.execute(ctx, request)
	c.publishResult(ctx, lifecycle, request, result)
	return true
}

func (c *RecoveryCoordinator) publishResult(
	ctx context.Context,
	lifecycle uint64,
	request RecoveryRequest,
	result RecoveryResult,
) {
	if c.beforePublish != nil {
		c.beforePublish()
	}
	c.publicationMu.Lock()
	callback := c.options.OnResult
	admitted := c.isWorkerCurrent(lifecycle) && ctx.Err() == nil && callback != nil
	c.publicationMu.Unlock()
	if !admitted {
		return
	}
	// Result callbacks are best-effort until the coordinator has a logger.
	defer func() { _ = recover() }()
	callback(request, result)
}

func (c *RecoveryCoordinator) takeNext(ctx context.Context, lifecycle uint64) (RecoveryRequest, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx.Err() != nil || c.workerLifecycle != lifecycle || c.workerCancel == nil {
		return RecoveryRequest{}, false
	}
	for _, kind := range [...]RecoveryKind{
		RecoveryNetworkReconnect,
		RecoveryOtherPlaceLogin,
		RecoveryProcessRestart,
	} {
		if request, ok := c.pending[kind]; ok {
			delete(c.pending, kind)
			return request, true
		}
	}
	return RecoveryRequest{}, false
}

func (c *RecoveryCoordinator) execute(ctx context.Context, request RecoveryRequest) (result RecoveryResult) {
	result.Kind = request.Kind
	defer func() {
		if recovered := recover(); recovered != nil {
			result = RecoveryResult{
				OK:    false,
				Kind:  request.Kind,
				Error: fmt.Sprintf("panic: %v", recovered),
			}
		}
	}()

	if request.Kind == RecoveryProcessRestart && c.options.OnPause != nil {
		defer c.options.OnPause(false)
		c.options.OnPause(true)
	}
	if c.options.Run == nil {
		return RecoveryResult{OK: false, Kind: request.Kind, Error: "recovery run callback is not configured"}
	}
	result = c.options.Run(ctx, request)
	if result.Kind == "" {
		result.Kind = request.Kind
	}
	return result
}

func (c *RecoveryCoordinator) isWorkerCurrent(lifecycle uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.workerLifecycle == lifecycle && c.workerCancel != nil
}

func (c *RecoveryCoordinator) signalWorker() {
	c.mu.Lock()
	wake := c.workerWake
	c.mu.Unlock()
	signal(wake)
}

func signal(wake chan<- struct{}) {
	if wake == nil {
		return
	}
	select {
	case wake <- struct{}{}:
	default:
	}
}

func validRecoveryKind(kind RecoveryKind) bool {
	switch kind {
	case RecoveryNetworkReconnect, RecoveryOtherPlaceLogin, RecoveryProcessRestart:
		return true
	default:
		return false
	}
}
