package qqws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"Farm_Go/internal/config"
	farmruntime "Farm_Go/internal/runtime"
	"github.com/gorilla/websocket"
)

func startQQWSTestServer(t *testing.T, adapter *Adapter) string {
	t.Helper()
	adapter.startRuntimeEventDispatch()
	t.Cleanup(adapter.closeRuntimeEventDispatch)

	mux := http.NewServeMux()
	mux.HandleFunc("/runtime/qqws", adapter.handleWebSocket)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return "ws" + strings.TrimPrefix(server.URL, "http") + "/runtime/qqws"
}

func dialQQWSTestClient(t *testing.T, url string) *websocket.Conn {
	t.Helper()

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial qq ws: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})
	return conn
}

func sendReadyHello(t *testing.T, conn *websocket.Conn) Packet {
	t.Helper()

	err := conn.WriteJSON(Packet{
		ID:   "hello-1",
		Type: MessageHello,
		TS:   time.Now().UnixMilli(),
		Payload: map[string]any{
			"client":           "qq-miniapp",
			"app":              "qq-farm",
			"version":          "farm-go-host-1",
			"gameCtlReady":     true,
			"availableMethods": []any{"host.describe", "gameCtl.probe"},
			"transportKind":    "websocket",
			"appPlatform":      "qq",
			"scriptHash":       "farm-go-debug-link",
		},
	})
	if err != nil {
		t.Fatalf("write hello: %v", err)
	}

	var ack Packet
	if err := conn.ReadJSON(&ack); err != nil {
		t.Fatalf("read hello ack: %v", err)
	}
	return ack
}

func sendHelloWithGameCtlReady(t *testing.T, conn *websocket.Conn, ready bool) Packet {
	t.Helper()

	err := conn.WriteJSON(Packet{
		ID:   "hello-1",
		Type: MessageHello,
		TS:   time.Now().UnixMilli(),
		Payload: map[string]any{
			"client":           "qq-miniapp",
			"app":              "qq-farm",
			"version":          "farm-go-host-1",
			"gameCtlReady":     ready,
			"availableMethods": []any{"host.describe"},
			"transportKind":    "websocket",
			"appPlatform":      "qq",
			"scriptHash":       "farm-go-debug-link",
		},
	})
	if err != nil {
		t.Fatalf("write hello: %v", err)
	}

	var ack Packet
	if err := conn.ReadJSON(&ack); err != nil {
		t.Fatalf("read hello ack: %v", err)
	}
	return ack
}

func TestHelloReceivesAckAndMarksRuntimeReady(t *testing.T) {
	manager := farmruntime.NewManager()
	adapter := New(config.Default().QQWS, manager)
	url := startQQWSTestServer(t, adapter)
	conn := dialQQWSTestClient(t, url)

	ack := sendReadyHello(t, conn)

	if ack.Type != MessageHelloAck {
		t.Fatalf("expected helloAck, got %q", ack.Type)
	}
	if ack.ID != "hello-1" {
		t.Fatalf("expected ack to reuse hello id, got %q", ack.ID)
	}
	if ack.Payload["ok"] != true {
		t.Fatalf("expected ok ack payload, got %#v", ack.Payload)
	}
	if ack.Payload["clientId"] == "" {
		t.Fatalf("expected assigned client id, got %#v", ack.Payload)
	}

	status := manager.Status()
	if status.Phase != farmruntime.PhaseReady {
		t.Fatalf("expected ready phase, got %#v", status)
	}
	if !status.Connected || !status.Ready {
		t.Fatalf("expected connected ready status, got %#v", status)
	}
	if status.HostVersion != "farm-go-host-1" {
		t.Fatalf("expected host version from hello, got %q", status.HostVersion)
	}
	if status.InstanceID == "" {
		t.Fatalf("expected active client id in status, got %#v", status)
	}
}

func TestRuntimeEventsAreForwardedToIsolatedHandlers(t *testing.T) {
	manager := farmruntime.NewManager()
	adapter := New(config.Default().QQWS, manager)
	client := &clientSession{id: "client-1", ready: true}
	adapter.startRuntimeEventDispatch()
	defer adapter.closeRuntimeEventDispatch()
	payload := map[string]any{
		"name":   "guardian.result",
		"value":  "original",
		"nested": map[string]any{"value": "original"},
		"items":  []any{map[string]any{"value": "original"}},
	}

	calls := make(chan string, 4)
	adapter.OnRuntimeEvent(func(event map[string]any) {
		calls <- "first"
		event["value"] = "mutated"
		event["nested"].(map[string]any)["value"] = "mutated"
		event["items"].([]any)[0].(map[string]any)["value"] = "mutated"
	})
	adapter.OnRuntimeEvent(func(event map[string]any) {
		calls <- "panic"
		panic("handler failure")
	})
	adapter.OnRuntimeEvent(func(event map[string]any) {
		if event["nested"].(map[string]any)["value"] != "original" || event["items"].([]any)[0].(map[string]any)["value"] != "original" {
			calls <- "nested-mutated"
			return
		}
		calls <- event["value"].(string)
	})
	unsubscribe := adapter.OnRuntimeEvent(func(map[string]any) { calls <- "unsubscribed" })
	unsubscribe()
	unsubscribe()

	adapter.handlePacket(client, Packet{Type: MessageEvent, Payload: payload})

	got := []string{<-calls, <-calls, <-calls}
	if joined := strings.Join(got, ","); joined != "first,panic,original" {
		t.Fatalf("unexpected handler calls %q", joined)
	}
	if payload["value"] != "original" || payload["nested"].(map[string]any)["value"] != "original" {
		t.Fatalf("handler mutated packet payload: %#v", payload)
	}
}

func TestReadyChangedUpdatesStatusAndForwardsRuntimeEvent(t *testing.T) {
	manager := farmruntime.NewManager()
	adapter := New(config.Default().QQWS, manager)
	adapter.startRuntimeEventDispatch()
	defer adapter.closeRuntimeEventDispatch()
	client := &clientSession{id: "client-1"}
	adapter.clients[client.id] = client

	received := make(chan map[string]any, 1)
	adapter.OnRuntimeEvent(func(event map[string]any) {
		received <- event
	})
	adapter.handlePacket(client, Packet{
		Type: MessageEvent,
		Payload: map[string]any{
			"name":  "gameCtlReadyChanged",
			"ready": true,
		},
	})

	var event map[string]any
	select {
	case event = <-received:
	case <-time.After(time.Second):
		t.Fatal("ready event was not forwarded")
	}
	status := manager.Status()
	if !client.ready || !status.Ready || status.Phase != farmruntime.PhaseReady {
		t.Fatalf("ready status was not updated: client=%#v status=%#v", client, status)
	}
	if event["name"] != "gameCtlReadyChanged" || event["ready"] != true {
		t.Fatalf("ready event was not forwarded: %#v", event)
	}
}

func TestBlockedRuntimeEventHandlerDoesNotBlockSocketProcessing(t *testing.T) {
	manager := farmruntime.NewManager()
	adapter := New(config.Default().QQWS, manager)
	url := startQQWSTestServer(t, adapter)
	conn := dialQQWSTestClient(t, url)
	_ = sendReadyHello(t, conn)

	entered := make(chan struct{})
	release := make(chan struct{})
	adapter.OnRuntimeEvent(func(event map[string]any) {
		if event["name"] == "guardian.blocked" {
			close(entered)
			<-release
		}
	})
	if err := conn.WriteJSON(Packet{Type: MessageEvent, Payload: map[string]any{"name": "guardian.blocked"}}); err != nil {
		t.Fatalf("write blocked event: %v", err)
	}
	<-entered

	if err := conn.WriteJSON(Packet{ID: "ping-1", Type: MessagePing, Payload: map[string]any{}}); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
	var pong Packet
	if err := conn.ReadJSON(&pong); err != nil {
		close(release)
		t.Fatalf("read pong while handler blocked: %v", err)
	}
	close(release)
	if pong.Type != MessagePong || pong.ID != "ping-1" {
		t.Fatalf("unexpected pong %#v", pong)
	}
}

func TestRuntimeEventPriorityClassification(t *testing.T) {
	for _, name := range []string{"guardian.result", "gameCtlReadyChanged", "network_reconnect", "other_place_login_reconnect"} {
		if runtimeEventPriority(map[string]any{"name": name}) <= farmruntime.EventPriorityOrdinary {
			t.Fatalf("expected priority runtime event %q", name)
		}
	}
	if runtimeEventPriority(map[string]any{"kind": "guardian"}) <= farmruntime.EventPriorityOrdinary {
		t.Fatal("guardian kind event was not classified as priority")
	}
	if runtimeEventPriority(map[string]any{"name": "telemetry.sample"}) != farmruntime.EventPriorityOrdinary {
		t.Fatal("ordinary telemetry event was classified as priority")
	}
}

func TestHostDescribeCallWorksBeforeGameCtlReady(t *testing.T) {
	manager := farmruntime.NewManager()
	adapter := New(config.Default().QQWS, manager)
	url := startQQWSTestServer(t, adapter)
	conn := dialQQWSTestClient(t, url)
	_ = sendHelloWithGameCtlReady(t, conn, false)

	type callResult struct {
		value any
		err   error
	}
	resultCh := make(chan callResult, 1)
	go func() {
		value, err := adapter.Call(context.Background(), "host.describe", []any{}, 2*time.Second)
		resultCh <- callResult{value: value, err: err}
	}()

	var call Packet
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if err := conn.ReadJSON(&call); err != nil {
		t.Fatalf("read call packet: %v", err)
	}
	_ = conn.SetReadDeadline(time.Time{})
	if call.Type != MessageCall {
		t.Fatalf("expected call packet, got %#v", call)
	}
	if call.Payload["path"] != "host.describe" {
		t.Fatalf("expected host.describe path, got %#v", call.Payload)
	}

	err := conn.WriteJSON(Packet{
		ID:   call.ID,
		Type: MessageResult,
		TS:   time.Now().UnixMilli(),
		Payload: map[string]any{
			"ok":   true,
			"path": "host.describe",
			"data": map[string]any{"gameCtlReady": false},
		},
	})
	if err != nil {
		t.Fatalf("write result: %v", err)
	}

	select {
	case got := <-resultCh:
		if got.err != nil {
			t.Fatalf("call returned error: %v", got.err)
		}
		data, ok := got.value.(map[string]any)
		if !ok || data["gameCtlReady"] != false {
			t.Fatalf("unexpected result data %#v", got.value)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for adapter call result")
	}
}

func TestCallSendsPacketAndResolvesMatchingResult(t *testing.T) {
	manager := farmruntime.NewManager()
	adapter := New(config.Default().QQWS, manager)
	url := startQQWSTestServer(t, adapter)
	conn := dialQQWSTestClient(t, url)
	_ = sendReadyHello(t, conn)

	type callResult struct {
		value any
		err   error
	}
	resultCh := make(chan callResult, 1)
	go func() {
		value, err := adapter.Call(context.Background(), "host.describe", []any{}, 2*time.Second)
		resultCh <- callResult{value: value, err: err}
	}()

	var call Packet
	if err := conn.ReadJSON(&call); err != nil {
		t.Fatalf("read call packet: %v", err)
	}
	if call.Type != MessageCall {
		t.Fatalf("expected call packet, got %#v", call)
	}
	if call.ID == "" {
		t.Fatalf("expected call id, got %#v", call)
	}
	if call.Payload["path"] != "host.describe" {
		t.Fatalf("expected host.describe path, got %#v", call.Payload)
	}

	err := conn.WriteJSON(Packet{
		ID:   call.ID,
		Type: MessageResult,
		TS:   time.Now().UnixMilli(),
		Payload: map[string]any{
			"ok":   true,
			"path": "host.describe",
			"data": map[string]any{
				"gameCtlReady": true,
				"appPlatform":  "qq",
			},
		},
	})
	if err != nil {
		t.Fatalf("write result: %v", err)
	}

	select {
	case got := <-resultCh:
		if got.err != nil {
			t.Fatalf("call returned error: %v", got.err)
		}
		data, ok := got.value.(map[string]any)
		if !ok {
			t.Fatalf("expected map result, got %#v", got.value)
		}
		if data["appPlatform"] != "qq" {
			t.Fatalf("unexpected result data %#v", data)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for adapter call result")
	}
}

func TestConcurrentCallsSerializeWritesToClient(t *testing.T) {
	manager := farmruntime.NewManager()
	adapter := New(config.Default().QQWS, manager)
	url := startQQWSTestServer(t, adapter)
	conn := dialQQWSTestClient(t, url)
	_ = sendReadyHello(t, conn)

	const callCount = 32
	largeArg := strings.Repeat("x", 64*1024)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := make(chan struct{})
	errCh := make(chan error, callCount)
	var wg sync.WaitGroup
	for i := 0; i < callCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := adapter.Call(ctx, "gameCtl.probe", []any{map[string]any{"payload": largeArg}}, 5*time.Second)
			errCh <- err
		}()
	}

	close(start)
	for i := 0; i < callCount; i++ {
		var call Packet
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		if err := conn.ReadJSON(&call); err != nil {
			t.Fatalf("read concurrent call packet %d/%d: %v", i+1, callCount, err)
		}
		if call.Type != MessageCall {
			t.Fatalf("expected call packet, got %#v", call)
		}
		if err := conn.WriteJSON(Packet{
			ID:   call.ID,
			Type: MessageResult,
			TS:   time.Now().UnixMilli(),
			Payload: map[string]any{
				"ok":   true,
				"path": call.Payload["path"],
				"data": map[string]any{"ok": true},
			},
		}); err != nil {
			t.Fatalf("write concurrent result: %v", err)
		}
	}
	_ = conn.SetReadDeadline(time.Time{})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatal("timed out waiting for concurrent adapter calls")
	}
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent call returned error: %v", err)
		}
	}
}

func TestPendingCallFailsWhenClientDisconnects(t *testing.T) {
	manager := farmruntime.NewManager()
	adapter := New(config.Default().QQWS, manager)
	url := startQQWSTestServer(t, adapter)
	conn := dialQQWSTestClient(t, url)
	_ = sendReadyHello(t, conn)

	resultCh := make(chan error, 1)
	go func() {
		_, err := adapter.Call(context.Background(), "host.describe", []any{}, 5*time.Second)
		resultCh <- err
	}()

	var call Packet
	if err := conn.ReadJSON(&call); err != nil {
		t.Fatalf("read call packet: %v", err)
	}
	if call.Type != MessageCall {
		t.Fatalf("expected call packet, got %#v", call)
	}

	_ = conn.Close()

	select {
	case err := <-resultCh:
		if err == nil {
			t.Fatal("expected pending call to fail after disconnect")
		}
		if !strings.Contains(err.Error(), "disconnected") {
			t.Fatalf("expected disconnected error, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for disconnect failure")
	}
}
