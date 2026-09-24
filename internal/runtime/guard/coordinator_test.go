package guard

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRecoveryCoordinatorRunsPendingRequestsInPriorityOrder(t *testing.T) {
	results := make(chan RecoveryKind, 3)
	coordinator := NewRecoveryCoordinator(CoordinatorOptions{
		Run: func(_ context.Context, request RecoveryRequest) RecoveryResult {
			results <- request.Kind
			return RecoveryResult{OK: true, Kind: request.Kind}
		},
	})
	defer coordinator.Close()

	for _, kind := range []RecoveryKind{RecoveryProcessRestart, RecoveryOtherPlaceLogin, RecoveryNetworkReconnect} {
		if !coordinator.Submit(RecoveryRequest{Kind: kind}) {
			t.Fatalf("Submit(%q) = false", kind)
		}
	}
	coordinator.Start(context.Background())

	want := []RecoveryKind{RecoveryNetworkReconnect, RecoveryOtherPlaceLogin, RecoveryProcessRestart}
	for i, expected := range want {
		if got := receive(t, results); got != expected {
			t.Fatalf("execution %d = %q, want %q", i, got, expected)
		}
	}
}

func TestRecoveryCoordinatorCoalescesPendingRequests(t *testing.T) {
	requests := make(chan RecoveryRequest, 1)
	coordinator := NewRecoveryCoordinator(CoordinatorOptions{
		Run: func(_ context.Context, request RecoveryRequest) RecoveryResult {
			requests <- request
			return RecoveryResult{OK: true, Kind: request.Kind}
		},
	})
	defer coordinator.Close()

	firstAt := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	if !coordinator.Submit(RecoveryRequest{Kind: RecoveryNetworkReconnect, Reason: "first", RequestedAt: firstAt}) {
		t.Fatal("first Submit returned false")
	}
	if coordinator.Submit(RecoveryRequest{Kind: RecoveryNetworkReconnect, Reason: "latest", RuntimeTarget: "qq"}) {
		t.Fatal("duplicate pending Submit returned true")
	}
	coordinator.Start(context.Background())

	request := receive(t, requests)
	if request.Reason != "latest" || request.RuntimeTarget != "qq" {
		t.Fatalf("coalesced request = %+v", request)
	}
	if !request.RequestedAt.Equal(firstAt) {
		t.Fatalf("RequestedAt = %v, want original %v", request.RequestedAt, firstAt)
	}
	assertNoReceive(t, requests)
}

func TestRecoveryCoordinatorSerializesRunCallbacks(t *testing.T) {
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondStarted := make(chan struct{})
	var active atomic.Int32
	var maxActive atomic.Int32
	coordinator := NewRecoveryCoordinator(CoordinatorOptions{
		Run: func(_ context.Context, request RecoveryRequest) RecoveryResult {
			current := active.Add(1)
			for old := maxActive.Load(); current > old && !maxActive.CompareAndSwap(old, current); old = maxActive.Load() {
			}
			defer active.Add(-1)
			if request.Kind == RecoveryProcessRestart {
				close(firstStarted)
				<-releaseFirst
			} else {
				close(secondStarted)
			}
			return RecoveryResult{OK: true, Kind: request.Kind}
		},
	})
	defer coordinator.Close()

	coordinator.Submit(RecoveryRequest{Kind: RecoveryProcessRestart})
	coordinator.Start(context.Background())
	waitClosed(t, firstStarted)
	coordinator.Submit(RecoveryRequest{Kind: RecoveryNetworkReconnect})
	assertNotClosed(t, secondStarted)
	close(releaseFirst)
	waitClosed(t, secondStarted)
	if got := maxActive.Load(); got != 1 {
		t.Fatalf("maximum concurrent Run callbacks = %d, want 1", got)
	}
}

func TestRecoveryCoordinatorBracketsOnlyProcessRestart(t *testing.T) {
	var mu sync.Mutex
	var events []string
	done := make(chan struct{}, 2)
	coordinator := NewRecoveryCoordinator(CoordinatorOptions{
		Run: func(_ context.Context, request RecoveryRequest) RecoveryResult {
			mu.Lock()
			events = append(events, "run:"+string(request.Kind))
			mu.Unlock()
			if request.Kind == RecoveryProcessRestart {
				panic("restart failed")
			}
			return RecoveryResult{OK: true, Kind: request.Kind}
		},
		OnPause: func(paused bool) {
			mu.Lock()
			events = append(events, "pause:"+map[bool]string{true: "true", false: "false"}[paused])
			mu.Unlock()
		},
		OnResult: func(RecoveryRequest, RecoveryResult) { done <- struct{}{} },
	})
	defer coordinator.Close()

	coordinator.Submit(RecoveryRequest{Kind: RecoveryOtherPlaceLogin})
	coordinator.Submit(RecoveryRequest{Kind: RecoveryProcessRestart})
	coordinator.Start(context.Background())
	receive(t, done)
	receive(t, done)

	mu.Lock()
	defer mu.Unlock()
	want := []string{
		"run:other_place_login",
		"pause:true",
		"run:process_restart",
		"pause:false",
	}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}
}

func TestRecoveryCoordinatorSurvivesRunPanic(t *testing.T) {
	results := make(chan RecoveryResult, 2)
	coordinator := NewRecoveryCoordinator(CoordinatorOptions{
		Run: func(_ context.Context, request RecoveryRequest) RecoveryResult {
			if request.Kind == RecoveryNetworkReconnect {
				panic("network panic")
			}
			return RecoveryResult{OK: true, Kind: request.Kind}
		},
		OnResult: func(_ RecoveryRequest, result RecoveryResult) { results <- result },
	})
	defer coordinator.Close()

	coordinator.Submit(RecoveryRequest{Kind: RecoveryNetworkReconnect})
	coordinator.Submit(RecoveryRequest{Kind: RecoveryOtherPlaceLogin})
	coordinator.Start(context.Background())

	failed := receive(t, results)
	if failed.OK || failed.Kind != RecoveryNetworkReconnect || failed.Error == "" {
		t.Fatalf("panic result = %+v", failed)
	}
	succeeded := receive(t, results)
	if !succeeded.OK || succeeded.Kind != RecoveryOtherPlaceLogin {
		t.Fatalf("subsequent result = %+v", succeeded)
	}
}

func TestRecoveryCoordinatorReplacementAndCloseSuppressStaleResults(t *testing.T) {
	started := make(chan RecoveryKind, 3)
	releases := map[RecoveryKind]chan struct{}{
		RecoveryNetworkReconnect: make(chan struct{}),
		RecoveryOtherPlaceLogin:  make(chan struct{}),
		RecoveryProcessRestart:   make(chan struct{}),
	}
	results := make(chan RecoveryKind, 3)
	coordinator := NewRecoveryCoordinator(CoordinatorOptions{
		Run: func(_ context.Context, request RecoveryRequest) RecoveryResult {
			started <- request.Kind
			<-releases[request.Kind]
			return RecoveryResult{OK: true, Kind: request.Kind}
		},
		OnResult: func(request RecoveryRequest, _ RecoveryResult) { results <- request.Kind },
	})
	defer coordinator.Close()

	coordinator.Submit(RecoveryRequest{Kind: RecoveryNetworkReconnect})
	coordinator.Start(context.Background())
	if got := receive(t, started); got != RecoveryNetworkReconnect {
		t.Fatalf("first started = %q", got)
	}
	coordinator.Start(context.Background())
	coordinator.Submit(RecoveryRequest{Kind: RecoveryOtherPlaceLogin})
	close(releases[RecoveryNetworkReconnect])
	if got := receive(t, started); got != RecoveryOtherPlaceLogin {
		t.Fatalf("second started = %q", got)
	}
	assertNoReceive(t, results)

	coordinator.Close()
	coordinator.Start(context.Background())
	coordinator.Submit(RecoveryRequest{Kind: RecoveryProcessRestart})
	close(releases[RecoveryOtherPlaceLogin])
	if got := receive(t, started); got != RecoveryProcessRestart {
		t.Fatalf("third started = %q", got)
	}
	assertNoReceive(t, results)

	close(releases[RecoveryProcessRestart])
	if got := receive(t, results); got != RecoveryProcessRestart {
		t.Fatalf("current result = %q", got)
	}
}

func TestRecoveryCoordinatorCloseWinsBeforeResultPublication(t *testing.T) {
	runStarted := make(chan struct{})
	releaseRun := make(chan struct{})
	publicationReached := make(chan struct{})
	releasePublication := make(chan struct{})
	results := make(chan RecoveryKind, 2)
	var blockOnce sync.Once

	coordinator := NewRecoveryCoordinator(CoordinatorOptions{
		Run: func(_ context.Context, request RecoveryRequest) RecoveryResult {
			if request.Kind == RecoveryNetworkReconnect {
				close(runStarted)
				<-releaseRun
			}
			return RecoveryResult{OK: true, Kind: request.Kind}
		},
		OnResult: func(request RecoveryRequest, _ RecoveryResult) { results <- request.Kind },
	})
	coordinator.beforePublish = func() {
		blockOnce.Do(func() {
			close(publicationReached)
			<-releasePublication
		})
	}
	defer coordinator.Close()

	coordinator.Submit(RecoveryRequest{Kind: RecoveryNetworkReconnect})
	coordinator.Start(context.Background())
	waitClosed(t, runStarted)
	close(releaseRun)
	waitClosed(t, publicationReached)

	coordinator.Close()
	close(releasePublication)
	coordinator.Start(context.Background())
	coordinator.Submit(RecoveryRequest{Kind: RecoveryOtherPlaceLogin})

	if got := receive(t, results); got != RecoveryOtherPlaceLogin {
		t.Fatalf("published result = %q, want current generation result", got)
	}
	assertNoReceive(t, results)
}

func TestRecoveryCoordinatorOnResultCanCloseCoordinator(t *testing.T) {
	callbackDone := make(chan struct{})
	var coordinator *RecoveryCoordinator
	coordinator = NewRecoveryCoordinator(CoordinatorOptions{
		Run: func(_ context.Context, request RecoveryRequest) RecoveryResult {
			return RecoveryResult{OK: true, Kind: request.Kind}
		},
		OnResult: func(RecoveryRequest, RecoveryResult) {
			coordinator.Close()
			close(callbackDone)
		},
	})
	defer coordinator.Close()

	coordinator.Submit(RecoveryRequest{Kind: RecoveryNetworkReconnect})
	coordinator.Start(context.Background())
	waitClosed(t, callbackDone)
}

func TestRecoveryCoordinatorOnResultCanReplaceWorker(t *testing.T) {
	results := make(chan RecoveryKind, 2)
	var coordinator *RecoveryCoordinator
	coordinator = NewRecoveryCoordinator(CoordinatorOptions{
		Run: func(_ context.Context, request RecoveryRequest) RecoveryResult {
			return RecoveryResult{OK: true, Kind: request.Kind}
		},
		OnResult: func(request RecoveryRequest, _ RecoveryResult) {
			results <- request.Kind
			if request.Kind == RecoveryNetworkReconnect {
				coordinator.Start(context.Background())
				coordinator.Submit(RecoveryRequest{Kind: RecoveryOtherPlaceLogin})
			}
		},
	})
	defer coordinator.Close()

	coordinator.Submit(RecoveryRequest{Kind: RecoveryNetworkReconnect})
	coordinator.Start(context.Background())
	if got := receive(t, results); got != RecoveryNetworkReconnect {
		t.Fatalf("first result = %q", got)
	}
	if got := receive(t, results); got != RecoveryOtherPlaceLogin {
		t.Fatalf("replacement result = %q", got)
	}
}

func TestRecoveryCoordinatorSurvivesOnResultPanic(t *testing.T) {
	resultReceived := make(chan RecoveryKind, 1)
	coordinator := NewRecoveryCoordinator(CoordinatorOptions{
		Run: func(_ context.Context, request RecoveryRequest) RecoveryResult {
			return RecoveryResult{OK: true, Kind: request.Kind}
		},
		OnResult: func(request RecoveryRequest, _ RecoveryResult) {
			if request.Kind == RecoveryNetworkReconnect {
				panic("result callback failed")
			}
			resultReceived <- request.Kind
		},
	})
	defer coordinator.Close()

	coordinator.Submit(RecoveryRequest{Kind: RecoveryNetworkReconnect})
	coordinator.Submit(RecoveryRequest{Kind: RecoveryOtherPlaceLogin})
	coordinator.Start(context.Background())
	if got := receive(t, resultReceived); got != RecoveryOtherPlaceLogin {
		t.Fatalf("result after callback panic = %q", got)
	}
}

func TestRecoveryCoordinatorRuntimeGeneration(t *testing.T) {
	coordinator := NewRecoveryCoordinator(CoordinatorOptions{})
	defer coordinator.Close()

	if got := coordinator.Generation(); got != 0 || !coordinator.IsCurrent(0) {
		t.Fatalf("initial generation = %d, IsCurrent(0) = %v", got, coordinator.IsCurrent(0))
	}
	coordinator.Start(context.Background())
	coordinator.Start(context.Background())
	if got := coordinator.Generation(); got != 0 {
		t.Fatalf("Start advanced runtime generation to %d", got)
	}

	const increments = 100
	var wg sync.WaitGroup
	values := make(chan uint64, increments)
	for i := 0; i < increments; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			values <- coordinator.AdvanceGeneration("test")
		}()
	}
	wg.Wait()
	close(values)

	seen := make(map[uint64]bool, increments)
	for value := range values {
		seen[value] = true
	}
	if len(seen) != increments || coordinator.Generation() != increments {
		t.Fatalf("generation values = %d unique, final = %d", len(seen), coordinator.Generation())
	}
	if coordinator.IsCurrent(increments-1) || !coordinator.IsCurrent(increments) {
		t.Fatalf("IsCurrent mismatch at final generation %d", coordinator.Generation())
	}
}

func TestRecoveryCoordinatorRejectsUnknownKind(t *testing.T) {
	coordinator := NewRecoveryCoordinator(CoordinatorOptions{})
	defer coordinator.Close()
	if coordinator.Submit(RecoveryRequest{Kind: RecoveryKind("unknown")}) {
		t.Fatal("Submit accepted unknown recovery kind")
	}
}

func TestRecoveryCoordinatorStartAndCloseAreIdempotent(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	runDone := make(chan struct{})
	coordinator := NewRecoveryCoordinator(CoordinatorOptions{
		Run: func(_ context.Context, _ RecoveryRequest) RecoveryResult {
			started <- struct{}{}
			<-release
			close(runDone)
			return RecoveryResult{OK: true, Kind: RecoveryNetworkReconnect}
		},
	})

	coordinator.Start(context.Background())
	coordinator.Start(context.Background())
	coordinator.Submit(RecoveryRequest{Kind: RecoveryNetworkReconnect})
	receive(t, started)
	coordinator.Close()
	coordinator.Close()
	close(release)
	waitClosed(t, runDone)

	coordinator.Submit(RecoveryRequest{Kind: RecoveryOtherPlaceLogin})
	assertNoReceive(t, started)
}

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for channel value")
		var zero T
		return zero
	}
}

func waitClosed(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for channel close")
	}
}

func assertNotClosed(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal("channel unexpectedly closed")
	default:
	}
}

func assertNoReceive[T any](t *testing.T, ch <-chan T) {
	t.Helper()
	select {
	case value := <-ch:
		t.Fatalf("unexpected channel value: %v", value)
	default:
	}
}
