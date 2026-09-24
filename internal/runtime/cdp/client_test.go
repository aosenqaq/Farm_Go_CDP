package cdp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	farmruntime "Farm_Go/internal/runtime"
	"github.com/gorilla/websocket"
)

func startCDPTestServer(t *testing.T, handler func(*websocket.Conn)) string {
	t.Helper()

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		handler(conn)
	}))
	t.Cleanup(server.Close)
	return "ws" + strings.TrimPrefix(server.URL, "http")
}

func readCDPMessage(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()

	var msg map[string]any
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read cdp message: %v", err)
	}
	return msg
}

func writeCDPResult(t *testing.T, conn *websocket.Conn, id any, result map[string]any) {
	t.Helper()

	if err := conn.WriteJSON(map[string]any{"id": id, "result": result}); err != nil {
		t.Fatalf("write cdp result: %v", err)
	}
}

func TestClientMatchesCommandResponsesByID(t *testing.T) {
	ready := make(chan string, 1)
	url := startCDPTestServer(t, func(conn *websocket.Conn) {
		defer conn.Close()

		enable := readCDPMessage(t, conn)
		if enable["method"] != "Runtime.enable" {
			t.Fatalf("expected Runtime.enable, got %#v", enable)
		}
		writeCDPResult(t, conn, enable["id"], map[string]any{})
		ready <- "connected"

		first := readCDPMessage(t, conn)
		second := readCDPMessage(t, conn)
		writeCDPResult(t, conn, second["id"], map[string]any{"name": second["method"]})
		writeCDPResult(t, conn, first["id"], map[string]any{"name": first["method"]})
	})

	client := NewClient(100 * time.Millisecond)
	if err := client.Connect(context.Background(), url); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()
	<-ready

	type outcome struct {
		value map[string]any
		err   error
	}
	fooCh := make(chan outcome, 1)
	barCh := make(chan outcome, 1)
	go func() {
		value, err := client.Send(context.Background(), "Runtime.foo", map[string]any{}, time.Second)
		fooCh <- outcome{value: value, err: err}
	}()
	go func() {
		value, err := client.Send(context.Background(), "Runtime.bar", map[string]any{}, time.Second)
		barCh <- outcome{value: value, err: err}
	}()

	foo := <-fooCh
	bar := <-barCh
	if foo.err != nil || bar.err != nil {
		t.Fatalf("unexpected errors foo=%v bar=%v", foo.err, bar.err)
	}
	if foo.value["name"] != "Runtime.foo" {
		t.Fatalf("foo resolved with wrong result %#v", foo.value)
	}
	if bar.value["name"] != "Runtime.bar" {
		t.Fatalf("bar resolved with wrong result %#v", bar.value)
	}
}

func TestClientEvaluateReturnsRemoteObjectValue(t *testing.T) {
	url := startCDPTestServer(t, func(conn *websocket.Conn) {
		defer conn.Close()
		enable := readCDPMessage(t, conn)
		writeCDPResult(t, conn, enable["id"], map[string]any{})

		eval := readCDPMessage(t, conn)
		if eval["method"] != "Runtime.evaluate" {
			t.Fatalf("expected Runtime.evaluate, got %#v", eval)
		}
		params := eval["params"].(map[string]any)
		if params["expression"] != "1 + 1" || params["contextId"] != float64(7) {
			body, _ := json.Marshal(params)
			t.Fatalf("unexpected evaluate params %s", body)
		}
		writeCDPResult(t, conn, eval["id"], map[string]any{
			"result": map[string]any{"type": "number", "value": 2},
		})
	})

	client := NewClient(time.Second)
	if err := client.Connect(context.Background(), url); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	value, err := client.Evaluate(context.Background(), "1 + 1", 7, time.Second)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if value != float64(2) {
		t.Fatalf("unexpected value %#v", value)
	}
}

func TestClientTracksRuntimeExecutionContexts(t *testing.T) {
	url := startCDPTestServer(t, func(conn *websocket.Conn) {
		defer conn.Close()
		enable := readCDPMessage(t, conn)
		writeCDPResult(t, conn, enable["id"], map[string]any{})
		_ = conn.WriteJSON(map[string]any{
			"method": "Runtime.executionContextCreated",
			"params": map[string]any{
				"context": map[string]any{
					"id":     42,
					"name":   "gameContext",
					"origin": "https://servicewechat.com",
				},
			},
		})
		time.Sleep(50 * time.Millisecond)
	})

	client := NewClient(time.Second)
	if err := client.Connect(context.Background(), url); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		contexts := client.Contexts()
		if len(contexts) == 1 && contexts[0].ID == 42 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("context was not tracked: %#v", client.Contexts())
}

func TestClientTracksSyntheticContextFromRuntimeConsoleEvent(t *testing.T) {
	url := startCDPTestServer(t, func(conn *websocket.Conn) {
		defer conn.Close()
		enable := readCDPMessage(t, conn)
		writeCDPResult(t, conn, enable["id"], map[string]any{})
		_ = conn.WriteJSON(map[string]any{
			"method": "Runtime.consoleAPICalled",
			"params": map[string]any{
				"type":               "trace",
				"executionContextId": 2,
			},
		})
		time.Sleep(50 * time.Millisecond)
	})

	client := NewClient(time.Second)
	if err := client.Connect(context.Background(), url); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		contexts := client.Contexts()
		if len(contexts) == 1 && contexts[0].ID == 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("synthetic context was not tracked: %#v", client.Contexts())
}

func TestClientForwardsEventsAfterContextBookkeeping(t *testing.T) {
	client := NewClient(time.Second)
	client.startEventDispatch()
	defer client.Close()
	methods := make(chan string, 3)
	client.OnEvent(func(method string, params map[string]any) {
		methods <- method
		if method == "Runtime.executionContextCreated" {
			contexts := client.Contexts()
			if len(contexts) != 1 || contexts[0].ID != 42 {
				t.Fatalf("context bookkeeping did not precede callback: %#v", contexts)
			}
		}
	})

	client.handleEvent("Runtime.executionContextCreated", map[string]any{
		"context": map[string]any{"id": 42, "name": "gameContext"},
	})
	client.handleEvent("Runtime.bindingCalled", map[string]any{
		"name": "__qqFarmRuntimeEventBinding", "payload": `{"kind":"guardian"}`,
	})
	client.handleEvent("Page.loadEventFired", map[string]any{"timestamp": 12})

	got := []string{<-methods, <-methods, <-methods}
	if joined := strings.Join(got, ","); joined != "Runtime.executionContextCreated,Runtime.bindingCalled,Page.loadEventFired" {
		t.Fatalf("unexpected forwarded methods %q", joined)
	}
}

func TestClientEventHandlersAreIsolated(t *testing.T) {
	client := NewClient(time.Second)
	client.startEventDispatch()
	defer client.Close()
	params := map[string]any{
		"value":  "original",
		"nested": map[string]any{"value": "original"},
		"items":  []any{map[string]any{"value": "original"}},
	}
	calls := make(chan string, 4)
	client.OnEvent(func(_ string, event map[string]any) {
		calls <- "first"
		event["value"] = "mutated"
		event["nested"].(map[string]any)["value"] = "mutated"
		event["items"].([]any)[0].(map[string]any)["value"] = "mutated"
	})
	client.OnEvent(func(string, map[string]any) {
		calls <- "panic"
		panic("handler failure")
	})
	client.OnEvent(func(_ string, event map[string]any) {
		if event["nested"].(map[string]any)["value"] != "original" || event["items"].([]any)[0].(map[string]any)["value"] != "original" {
			calls <- "nested-mutated"
			return
		}
		calls <- event["value"].(string)
	})
	unsubscribe := client.OnEvent(func(string, map[string]any) { calls <- "unsubscribed" })
	unsubscribe()
	unsubscribe()

	client.handleEvent("Runtime.bindingCalled", params)

	got := []string{<-calls, <-calls, <-calls}
	if joined := strings.Join(got, ","); joined != "first,panic,original" {
		t.Fatalf("unexpected handler calls %q", joined)
	}
	if params["value"] != "original" || params["nested"].(map[string]any)["value"] != "original" {
		t.Fatalf("handler mutated source params: %#v", params)
	}
}

func TestClientEventHandlerCanMakeReentrantCommand(t *testing.T) {
	url := startCDPTestServer(t, func(conn *websocket.Conn) {
		defer conn.Close()
		enable := readCDPMessage(t, conn)
		writeCDPResult(t, conn, enable["id"], map[string]any{})
		if err := conn.WriteJSON(map[string]any{
			"method": "Runtime.bindingCalled",
			"params": map[string]any{"name": "runtime-event", "payload": `{}`},
		}); err != nil {
			t.Fatalf("write binding event: %v", err)
		}

		reentrant := readCDPMessage(t, conn)
		if reentrant["method"] != "Runtime.reentrant" {
			t.Fatalf("unexpected reentrant command %#v", reentrant)
		}
		writeCDPResult(t, conn, reentrant["id"], map[string]any{"ok": true})
	})

	client := NewClient(time.Second)
	result := make(chan error, 1)
	client.OnEvent(func(method string, _ map[string]any) {
		if method != "Runtime.bindingCalled" {
			return
		}
		value, err := client.Send(context.Background(), "Runtime.reentrant", map[string]any{}, 500*time.Millisecond)
		if err == nil && value["ok"] != true {
			err = fmt.Errorf("unexpected reentrant result %#v", value)
		}
		result <- err
	})
	if err := client.Connect(context.Background(), url); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("reentrant command failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("reentrant event handler did not finish")
	}
}

func TestClientEventHandlerCanCloseClient(t *testing.T) {
	url := startCDPTestServer(t, func(conn *websocket.Conn) {
		defer conn.Close()
		enable := readCDPMessage(t, conn)
		writeCDPResult(t, conn, enable["id"], map[string]any{})
		if err := conn.WriteJSON(map[string]any{
			"method": "Runtime.bindingCalled",
			"params": map[string]any{"name": "runtime-event", "payload": `{}`},
		}); err != nil {
			t.Fatalf("write binding event: %v", err)
		}
		<-time.After(100 * time.Millisecond)
	})

	client := NewClient(time.Second)
	closed := make(chan struct{})
	client.OnEvent(func(method string, _ map[string]any) {
		if method == "Runtime.bindingCalled" {
			client.Close()
			close(closed)
		}
	})
	if err := client.Connect(context.Background(), url); err != nil {
		t.Fatalf("connect: %v", err)
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Client.Close deadlocked inside event callback")
	}
	client.CloseAndWait()
}

func TestClientOwnerCloseAndWaitDropsQueuedEvents(t *testing.T) {
	client := NewClient(time.Second)
	client.startEventDispatch()
	entered := make(chan struct{})
	release := make(chan struct{})
	delivered := make(chan string, 2)
	client.OnEvent(func(method string, _ map[string]any) {
		if method == "active" {
			close(entered)
			<-release
		}
		delivered <- method
	})
	client.handleEvent("active", map[string]any{})
	<-entered
	client.handleEvent("queued", map[string]any{})
	closed := make(chan struct{})
	go func() {
		client.CloseAndWait()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("owner close returned before active callback")
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	<-closed
	if got := <-delivered; got != "active" {
		t.Fatalf("unexpected active event %q", got)
	}
	select {
	case got := <-delivered:
		t.Fatalf("queued event ran after owner close: %q", got)
	case <-time.After(25 * time.Millisecond):
	}
}

func TestClientEventPriorityClassification(t *testing.T) {
	for _, method := range []string{
		"Runtime.bindingCalled",
		"Runtime.executionContextCreated",
		"Runtime.executionContextDestroyed",
		"Runtime.executionContextsCleared",
	} {
		if clientEventPriority(clientEvent{method: method}) <= farmruntime.EventPriorityOrdinary {
			t.Fatalf("expected priority CDP event %q", method)
		}
	}
	if clientEventPriority(clientEvent{method: "Runtime.consoleAPICalled"}) != farmruntime.EventPriorityOrdinary {
		t.Fatal("ordinary console event was classified as priority")
	}
}
