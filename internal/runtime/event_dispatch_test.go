package runtime

import (
	"reflect"
	"testing"
	"time"
)

func TestOrderedDispatcherKeepsProducerNonBlockingAndPreservesOrder(t *testing.T) {
	dispatcher := NewOrderedDispatcher("test", CloneJSONMap, func(event map[string]any) string {
		value, _ := event["sequence"].(string)
		return value
	})
	dispatcher.Start()
	defer func() {
		dispatcher.Close()
		dispatcher.Wait()
	}()

	blocked := make(chan struct{})
	entered := make(chan struct{})
	delivered := make(chan string, 2)
	dispatcher.Subscribe(func(event map[string]any) {
		if event["sequence"] == "first" {
			close(entered)
			<-blocked
		}
		delivered <- event["sequence"].(string)
	})

	if !dispatcher.Dispatch(map[string]any{"sequence": "first"}) {
		t.Fatal("first event was not queued")
	}
	<-entered
	dispatched := make(chan bool, 1)
	go func() {
		dispatched <- dispatcher.Dispatch(map[string]any{"sequence": "second"})
	}()
	select {
	case queued := <-dispatched:
		if !queued {
			t.Fatal("second event was not queued")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("blocked handler stalled producer")
	}
	close(blocked)

	got := []string{<-delivered, <-delivered}
	if !reflect.DeepEqual(got, []string{"first", "second"}) {
		t.Fatalf("events delivered out of order: %#v", got)
	}
}

func TestOrderedDispatcherClonesNestedJSONAndUnsubscribesIdempotently(t *testing.T) {
	dispatcher := NewOrderedDispatcher("test", CloneJSONMap, nil)
	dispatcher.Start()
	defer func() {
		dispatcher.Close()
		dispatcher.Wait()
	}()

	original := map[string]any{
		"nested": map[string]any{"value": "original"},
		"items":  []any{map[string]any{"value": "original"}},
	}
	firstDone := make(chan struct{})
	unsubscribe := dispatcher.Subscribe(func(event map[string]any) {
		event["nested"].(map[string]any)["value"] = "mutated"
		event["items"].([]any)[0].(map[string]any)["value"] = "mutated"
		close(firstDone)
	})
	result := make(chan map[string]any, 1)
	dispatcher.Subscribe(func(event map[string]any) { result <- event })

	dispatcher.Dispatch(original)
	<-firstDone
	got := <-result
	if got["nested"].(map[string]any)["value"] != "original" || got["items"].([]any)[0].(map[string]any)["value"] != "original" {
		t.Fatalf("nested mutation leaked between handlers: %#v", got)
	}
	if original["nested"].(map[string]any)["value"] != "original" {
		t.Fatalf("dispatch mutated source: %#v", original)
	}

	unsubscribe()
	unsubscribe()
}

func TestOrderedDispatcherStartAndCloseAreIdempotentAndRestartable(t *testing.T) {
	dispatcher := NewOrderedDispatcher("test", CloneJSONMap, nil)
	delivered := make(chan string, 2)
	dispatcher.Subscribe(func(event map[string]any) { delivered <- event["value"].(string) })

	dispatcher.Start()
	dispatcher.Start()
	if !dispatcher.Dispatch(map[string]any{"value": "first"}) {
		t.Fatal("dispatch failed after start")
	}
	if got := <-delivered; got != "first" {
		t.Fatalf("unexpected first delivery %q", got)
	}
	dispatcher.Close()
	dispatcher.Wait()
	dispatcher.Close()
	if dispatcher.Dispatch(map[string]any{"value": "closed"}) {
		t.Fatal("dispatch succeeded after close")
	}

	dispatcher.Start()
	if !dispatcher.Dispatch(map[string]any{"value": "second"}) {
		t.Fatal("dispatch failed after restart")
	}
	if got := <-delivered; got != "second" {
		t.Fatalf("unexpected second delivery %q", got)
	}
	dispatcher.Close()
	dispatcher.Wait()
}

func TestOrderedDispatcherCallbackCanRequestClose(t *testing.T) {
	dispatcher := NewOrderedDispatcher("test", CloneJSONMap, nil)
	dispatcher.Start()
	callbackReturned := make(chan struct{})
	dispatcher.Subscribe(func(map[string]any) {
		dispatcher.RequestStop()
		close(callbackReturned)
	})

	dispatcher.Dispatch(map[string]any{"value": "close"})
	select {
	case <-callbackReturned:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Close deadlocked inside dispatcher callback")
	}
}

func TestOrderedDispatcherOwnerCloseWaitsAndDropsQueuedCallbacks(t *testing.T) {
	dispatcher := NewOrderedDispatcher("test", CloneJSONMap, nil)
	dispatcher.Start()
	entered := make(chan struct{})
	release := make(chan struct{})
	delivered := make(chan string, 2)
	dispatcher.Subscribe(func(event map[string]any) {
		if event["value"] == "active" {
			close(entered)
			<-release
		}
		delivered <- event["value"].(string)
	})
	dispatcher.Dispatch(map[string]any{"value": "active"})
	<-entered
	dispatcher.Dispatch(map[string]any{"value": "queued"})

	closed := make(chan struct{})
	go func() {
		dispatcher.CloseAndWait()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("owner CloseAndWait returned before active callback finished")
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("owner CloseAndWait did not join dispatcher")
	}
	if got := <-delivered; got != "active" {
		t.Fatalf("unexpected active delivery %q", got)
	}
	select {
	case got := <-delivered:
		t.Fatalf("queued callback ran after owner close: %q", got)
	case <-time.After(25 * time.Millisecond):
	}
}

func TestOrderedDispatcherStartWaitsForConcurrentCloseAndRestartsWorker(t *testing.T) {
	dispatcher := NewOrderedDispatcher("test", CloneJSONMap, nil)
	dispatcher.Start()
	entered := make(chan struct{})
	release := make(chan struct{})
	delivered := make(chan string, 2)
	dispatcher.Subscribe(func(event map[string]any) {
		if event["value"] == "old" {
			close(entered)
			<-release
		}
		delivered <- event["value"].(string)
	})
	dispatcher.Dispatch(map[string]any{"value": "old"})
	<-entered

	dispatcher.Close()
	started := make(chan struct{})
	go func() {
		dispatcher.Start()
		close(started)
	}()
	select {
	case <-started:
		t.Fatal("Start returned before closing worker exited")
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("Start did not restart after concurrent Close")
	}
	if !dispatcher.Dispatch(map[string]any{"value": "new"}) {
		t.Fatal("dispatch failed after concurrent restart")
	}
	if got := []string{<-delivered, <-delivered}; !reflect.DeepEqual(got, []string{"old", "new"}) {
		t.Fatalf("unexpected deliveries after restart: %#v", got)
	}
	dispatcher.Close()
	dispatcher.Wait()
}

func TestOrderedDispatcherUnsubscribeRemovesOrderEntry(t *testing.T) {
	dispatcher := NewOrderedDispatcher("test", CloneJSONMap, nil)
	unsubscribe := dispatcher.Subscribe(func(map[string]any) {})
	unsubscribe()
	unsubscribe()

	dispatcher.mu.Lock()
	orderLength := len(dispatcher.handlerOrder)
	dispatcher.mu.Unlock()
	if orderLength != 0 {
		t.Fatalf("unsubscribe left %d handler order tombstones", orderLength)
	}
}

func TestOrderedDispatcherBoundedOverflowPreservesPriorityEvents(t *testing.T) {
	dispatcher := NewOrderedDispatcherWithOptions("test", CloneJSONMap, func(event map[string]any) string {
		return event["value"].(string)
	}, OrderedDispatcherOptions[map[string]any]{
		Capacity: 2,
		Priority: func(event map[string]any) int {
			priority, _ := event["priority"].(bool)
			if priority {
				return EventPriorityGuardian
			}
			return EventPriorityOrdinary
		},
	})
	dispatcher.Start()
	defer func() {
		dispatcher.Close()
		dispatcher.Wait()
	}()

	entered := make(chan struct{})
	release := make(chan struct{})
	delivered := make(chan string, 3)
	dispatcher.Subscribe(func(event map[string]any) {
		if event["value"] == "blocked" {
			close(entered)
			<-release
		}
		delivered <- event["value"].(string)
	})
	dispatcher.Dispatch(map[string]any{"value": "blocked"})
	<-entered
	dispatcher.Dispatch(map[string]any{"value": "ordinary-1"})
	dispatcher.Dispatch(map[string]any{"value": "ordinary-2"})
	if !dispatcher.Dispatch(map[string]any{"value": "guardian", "priority": true}) {
		t.Fatal("priority event was rejected by ordinary backlog")
	}
	if dispatcher.Dispatch(map[string]any{"value": "ordinary-3"}) {
		t.Fatal("ordinary overflow event was accepted")
	}
	if dropped := dispatcher.Dropped(); dropped != 2 {
		t.Fatalf("unexpected overflow count %d", dropped)
	}
	close(release)

	got := []string{<-delivered, <-delivered, <-delivered}
	if !reflect.DeepEqual(got, []string{"blocked", "ordinary-2", "guardian"}) {
		t.Fatalf("unexpected overflow deliveries %#v", got)
	}
}

func TestOrderedDispatcherLifecycleEvictsGuardianBacklog(t *testing.T) {
	dispatcher := NewOrderedDispatcherWithOptions("test", CloneJSONMap, nil, OrderedDispatcherOptions[map[string]any]{
		Capacity: 2,
		Priority: func(event map[string]any) int { return event["priority"].(int) },
	})
	dispatcher.Start()
	defer dispatcher.CloseAndWait()
	entered := make(chan struct{})
	release := make(chan struct{})
	delivered := make(chan string, 3)
	dispatcher.Subscribe(func(event map[string]any) {
		if event["name"] == "active" {
			close(entered)
			<-release
		}
		delivered <- event["name"].(string)
	})
	dispatcher.Dispatch(map[string]any{"name": "active", "priority": EventPriorityOrdinary})
	<-entered
	dispatcher.Dispatch(map[string]any{"name": "binding-1", "priority": EventPriorityGuardian})
	dispatcher.Dispatch(map[string]any{"name": "binding-2", "priority": EventPriorityGuardian})
	if !dispatcher.Dispatch(map[string]any{"name": "context-cleared", "priority": EventPriorityLifecycle}) {
		t.Fatal("lifecycle event was rejected by guardian backlog")
	}
	if dispatcher.Dispatch(map[string]any{"name": "ordinary", "priority": EventPriorityOrdinary}) {
		t.Fatal("ordinary event displaced higher-priority backlog")
	}
	close(release)

	got := []string{<-delivered, <-delivered, <-delivered}
	if !reflect.DeepEqual(got, []string{"active", "binding-2", "context-cleared"}) {
		t.Fatalf("unexpected multi-level priority deliveries %#v", got)
	}
}
