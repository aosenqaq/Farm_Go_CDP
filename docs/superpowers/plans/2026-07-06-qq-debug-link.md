# QQ Debug Link Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Farm_Go accept the QQ miniapp host script protocol and run `host.describe` / `gameCtl.probe` diagnostics through the connected QQ host.

**Architecture:** Keep Farm_Go as the local Wails/Go desktop app. Upgrade `internal/runtime/qqws` from a handshake-only WebSocket handler into a small QQ host session manager compatible with the reference `qq-host.js` protocol, then wire diagnostics through that session. Copy only the QQ host script asset from `E:\desktop\farm-tauri` and adapt its build placeholders inside Farm_Go.

**Tech Stack:** Go 1.25, Wails v2, `github.com/gorilla/websocket`, React/Vite frontend, copied QQ host JavaScript asset.

---

## Scope Check

This plan covers one subsystem: the QQ WebSocket debug link. It does not migrate WMPF, Frida, CDP, Tauri sidecars, Node gateway code, browser WebUI behavior, or real farm automation tasks.

`E:\desktop\Farm_Go` is not currently a git repository. Do not include commit commands in execution. Verify changed files with filesystem commands instead.

## File Structure

- Modify: `internal/runtime/qqws/protocol.go`
  - Define the QQ host packet shape: `id`, `type`, `ts`, and `payload`.
  - Define protocol constants compatible with `qq-host.js`: `hello`, `helloAck`, `call`, `result`, `event`, `log`, `ping`, `pong`, and `error`.
- Modify: `internal/runtime/qqws/adapter.go`
  - Track connected clients, the active ready client, and pending calls.
  - Accept `hello`, emit `helloAck`, handle events/logs/ping/result/error, and expose `Call`.
- Create: `internal/runtime/qqws/adapter_test.go`
  - Test compatible handshake, call/result flow, and disconnect failure.
- Modify: `internal/diagnostics/service.go`
  - Depend on a runtime caller interface.
  - Forward supported diagnostics to the QQ WS adapter.
- Modify: `internal/diagnostics/service_test.go`
  - Test method validation, disconnected behavior, argument conversion, and caller forwarding.
- Modify: `app.go`
  - Construct the adapter first and inject it into diagnostics.
- Create: `resources/qq/qq-host.js`
  - Copy from `E:\desktop\farm-tauri\core\qq-host.js`.
  - Replace only Farm_Go-specific placeholders.
- Create: `internal/runtime/qqws/host_asset_test.go`
  - Verify the copied host asset no longer contains unresolved placeholders and points to the Farm_Go local WS URL.

---

### Task 1: Add QQ WS Protocol Red Tests

**Files:**
- Create: `internal/runtime/qqws/adapter_test.go`

- [ ] **Step 1: Write the failing adapter tests**

Create `internal/runtime/qqws/adapter_test.go`:

```go
package qqws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Farm_Go/internal/config"
	farmruntime "Farm_Go/internal/runtime"
	"github.com/gorilla/websocket"
)

func startQQWSTestServer(t *testing.T, adapter *Adapter) string {
	t.Helper()

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
```

- [ ] **Step 2: Run the red test**

Run:

```powershell
go test ./internal/runtime/qqws
```

Expected: FAIL because `Packet`, `MessageHelloAck`, `MessageCall`, `MessageResult`, and `Adapter.Call` are not defined yet.

---

### Task 2: Implement QQ WS Protocol Types

**Files:**
- Modify: `internal/runtime/qqws/protocol.go`

- [ ] **Step 1: Replace protocol definitions**

Replace `internal/runtime/qqws/protocol.go` with:

```go
package qqws

type Packet struct {
	ID      string         `json:"id,omitempty"`
	Type    string         `json:"type"`
	TS      int64          `json:"ts,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

type Message = Packet

type RPCError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	MessageHello   = "hello"
	MessageHelloAck = "helloAck"
	MessageCall    = "call"
	MessageResult  = "result"
	MessageEvent   = "event"
	MessageLog     = "log"
	MessageError   = "error"
	MessagePing    = "ping"
	MessagePong    = "pong"
)
```

- [ ] **Step 2: Run the focused test**

Run:

```powershell
go test ./internal/runtime/qqws
```

Expected: FAIL only because `Adapter.Call` is not implemented and existing `adapter.go` still references removed fields such as `RequestID`.

---

### Task 3: Implement QQ WS Session Manager

**Files:**
- Modify: `internal/runtime/qqws/adapter.go`

- [ ] **Step 1: Replace the adapter implementation**

Replace `internal/runtime/qqws/adapter.go` with:

```go
package qqws

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"Farm_Go/internal/config"
	farmruntime "Farm_Go/internal/runtime"
	"github.com/gorilla/websocket"
)

var ErrNotConnected = errors.New("runtime is not connected")

type clientSession struct {
	id          string
	conn        *websocket.Conn
	ready       bool
	connectedAt time.Time
}

type pendingCall struct {
	clientID string
	timer    *time.Timer
	done     chan callOutcome
}

type callOutcome struct {
	value any
	err   error
}

type Adapter struct {
	cfg     config.QQWSConfig
	manager *farmruntime.Manager
	server  *http.Server

	mu             sync.Mutex
	clientSeq      int
	callSeq        int
	activeClientID string
	clients        map[string]*clientSession
	pending        map[string]*pendingCall
}

func New(cfg config.QQWSConfig, manager *farmruntime.Manager) *Adapter {
	return &Adapter{
		cfg:     cfg,
		manager: manager,
		clients: make(map[string]*clientSession),
		pending: make(map[string]*pendingCall),
	}
}

func (a *Adapter) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc(a.cfg.Path, a.handleWebSocket)

	a.server = &http.Server{
		Addr:              a.cfg.Host + ":" + portString(a.cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	a.manager.SetStatus(farmruntime.Status{
		Target: "qqws",
		Phase:  farmruntime.PhaseListening,
	})

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = a.server.Shutdown(shutdownCtx)
	}()

	err := a.server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	a.manager.SetStatus(farmruntime.Status{
		Target:    "qqws",
		Phase:     farmruntime.PhaseError,
		LastError: err.Error(),
	})
	return err
}

func (a *Adapter) Call(ctx context.Context, path string, args []any, timeout time.Duration) (any, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	client := a.activeReadyClient()
	if client == nil {
		return nil, ErrNotConnected
	}

	reqID := a.nextCallID()
	done := make(chan callOutcome, 1)
	timer := time.AfterFunc(timeout, func() {
		a.finishPending(reqID, callOutcome{
			err: fmt.Errorf("qq ws call timed out: %s (%dms)", path, timeout.Milliseconds()),
		})
	})

	a.mu.Lock()
	a.pending[reqID] = &pendingCall{
		clientID: client.id,
		timer:    timer,
		done:     done,
	}
	a.mu.Unlock()

	packet := Packet{
		ID:   reqID,
		Type: MessageCall,
		TS:   time.Now().UnixMilli(),
		Payload: map[string]any{
			"path":      path,
			"args":      args,
			"timeoutMs": timeout.Milliseconds(),
		},
	}

	if err := client.conn.WriteJSON(packet); err != nil {
		a.finishPending(reqID, callOutcome{err: err})
	}

	select {
	case outcome := <-done:
		return outcome.value, outcome.err
	case <-ctx.Done():
		a.finishPending(reqID, callOutcome{err: ctx.Err()})
		return nil, ctx.Err()
	}
}

func (a *Adapter) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if a.cfg.HostToken != "" && r.URL.Query().Get("token") != a.cfg.HostToken {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := a.addClient(conn)
	a.manager.SetStatus(farmruntime.Status{
		Target:     "qqws",
		Phase:      farmruntime.PhaseHandshaking,
		Connected:  true,
		LastSeenAt: time.Now().Format(time.RFC3339),
	})

	defer func() {
		a.removeClient(client.id, errors.New("qq ws client disconnected"))
		_ = conn.Close()
	}()

	for {
		var packet Packet
		if err := conn.ReadJSON(&packet); err != nil {
			a.manager.SetStatus(farmruntime.Status{
				Target:    "qqws",
				Phase:     farmruntime.PhaseDisconnected,
				LastError: err.Error(),
			})
			return
		}
		a.handlePacket(client, packet)
	}
}

func (a *Adapter) handlePacket(client *clientSession, packet Packet) {
	switch packet.Type {
	case MessageHello:
		a.handleHello(client, packet)
	case MessageEvent:
		a.handleEvent(client, packet)
	case MessageLog:
		a.touchClient(client)
	case MessagePing:
		_ = client.conn.WriteJSON(Packet{
			ID:      packet.ID,
			Type:    MessagePong,
			TS:      time.Now().UnixMilli(),
			Payload: map[string]any{},
		})
	case MessageResult:
		a.handleResult(packet)
	case MessageError:
		a.manager.SetStatus(farmruntime.Status{
			Target:    "qqws",
			Phase:     farmruntime.PhaseError,
			Connected: true,
			Ready:     client.ready,
			LastError: stringFromPayload(packet.Payload, "error", "client_error"),
		})
	}
}

func (a *Adapter) handleHello(client *clientSession, packet Packet) {
	version := stringFromPayload(packet.Payload, "version", "")
	if version == "" {
		version = stringFromPayload(packet.Payload, "hostVersion", "")
	}
	if a.cfg.ExpectedHostVersion != "" && version != "" && version != a.cfg.ExpectedHostVersion {
		errText := fmt.Sprintf("host_version_mismatch: expected=%s actual=%s", a.cfg.ExpectedHostVersion, version)
		_ = client.conn.WriteJSON(Packet{
			ID:   packet.ID,
			Type: MessageHelloAck,
			TS:   time.Now().UnixMilli(),
			Payload: map[string]any{
				"ok":    false,
				"error": errText,
			},
		})
		a.manager.SetStatus(farmruntime.Status{
			Target:    "qqws",
			Phase:     farmruntime.PhaseError,
			LastError: errText,
		})
		_ = client.conn.Close()
		return
	}

	ready := boolFromPayload(packet.Payload, "gameCtlReady")
	a.mu.Lock()
	client.ready = ready
	a.activeClientID = client.id
	a.mu.Unlock()

	a.manager.SetStatus(farmruntime.Status{
		Target:      "qqws",
		Phase:       farmruntime.PhaseReady,
		Connected:   true,
		Ready:       ready,
		InstanceID:  client.id,
		HostVersion: version,
		LastSeenAt:  time.Now().Format(time.RFC3339),
	})

	_ = client.conn.WriteJSON(Packet{
		ID:   packet.ID,
		Type: MessageHelloAck,
		TS:   time.Now().UnixMilli(),
		Payload: map[string]any{
			"ok":         true,
			"clientId":   client.id,
			"serverTime": time.Now().Format(time.RFC3339),
		},
	})
}

func (a *Adapter) handleEvent(client *clientSession, packet Packet) {
	if packet.Payload != nil && packet.Payload["name"] == "gameCtlReadyChanged" {
		ready := boolFromPayload(packet.Payload, "ready")
		a.mu.Lock()
		client.ready = ready
		if ready {
			a.activeClientID = client.id
		}
		a.mu.Unlock()
		status := a.manager.Status()
		status.Ready = ready
		status.Connected = true
		status.LastSeenAt = time.Now().Format(time.RFC3339)
		if ready {
			status.Phase = farmruntime.PhaseReady
			status.InstanceID = client.id
		}
		a.manager.SetStatus(status)
		return
	}
	a.touchClient(client)
}

func (a *Adapter) handleResult(packet Packet) {
	if packet.ID == "" {
		return
	}
	payload := packet.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	if payload["ok"] == false {
		a.finishPending(packet.ID, callOutcome{
			err: errors.New(stringFromPayload(payload, "error", "qq ws call failed")),
		})
		return
	}
	a.finishPending(packet.ID, callOutcome{value: payload["data"]})
}

func (a *Adapter) touchClient(client *clientSession) {
	status := a.manager.Status()
	status.Connected = true
	status.Ready = client.ready
	status.LastSeenAt = time.Now().Format(time.RFC3339)
	if client.ready {
		status.Phase = farmruntime.PhaseReady
	}
	a.manager.SetStatus(status)
}

func (a *Adapter) addClient(conn *websocket.Conn) *clientSession {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.clientSeq++
	id := fmt.Sprintf("qqws-client-%d", a.clientSeq)
	client := &clientSession{
		id:          id,
		conn:        conn,
		connectedAt: time.Now(),
	}
	a.clients[id] = client
	if a.activeClientID == "" {
		a.activeClientID = id
	}
	return client
}

func (a *Adapter) removeClient(clientID string, err error) {
	a.mu.Lock()
	delete(a.clients, clientID)
	if a.activeClientID == clientID {
		a.activeClientID = ""
		for id, client := range a.clients {
			if client.ready {
				a.activeClientID = id
				break
			}
		}
	}
	for id, pending := range a.pending {
		if pending.clientID != clientID {
			continue
		}
		delete(a.pending, id)
		pending.timer.Stop()
		pending.done <- callOutcome{err: err}
	}
	connected := len(a.clients) > 0
	a.mu.Unlock()

	if !connected {
		a.manager.SetStatus(farmruntime.Status{
			Target:    "qqws",
			Phase:     farmruntime.PhaseDisconnected,
			LastError: err.Error(),
		})
	}
}

func (a *Adapter) activeReadyClient() *clientSession {
	a.mu.Lock()
	defer a.mu.Unlock()

	if client := a.clients[a.activeClientID]; client != nil && client.ready {
		return client
	}
	for id, client := range a.clients {
		if client.ready {
			a.activeClientID = id
			return client
		}
	}
	return nil
}

func (a *Adapter) nextCallID() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.callSeq++
	return fmt.Sprintf("qqcall-%d", a.callSeq)
}

func (a *Adapter) finishPending(id string, outcome callOutcome) {
	a.mu.Lock()
	pending := a.pending[id]
	if pending != nil {
		delete(a.pending, id)
		pending.timer.Stop()
	}
	a.mu.Unlock()

	if pending != nil {
		pending.done <- outcome
	}
}

func boolFromPayload(payload map[string]any, key string) bool {
	if payload == nil {
		return false
	}
	value, ok := payload[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "true"
	default:
		return false
	}
}

func stringFromPayload(payload map[string]any, key string, fallback string) string {
	if payload == nil {
		return fallback
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return fallback
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func portString(port int) string {
	return fmt.Sprintf("%d", port)
}
```

- [ ] **Step 2: Run the adapter tests**

Run:

```powershell
go test ./internal/runtime/qqws
```

Expected: PASS for `internal/runtime/qqws`.

---

### Task 4: Add Diagnostics Red Tests

**Files:**
- Modify: `internal/diagnostics/service_test.go`

- [ ] **Step 1: Replace diagnostics tests with caller-aware tests**

Replace `internal/diagnostics/service_test.go` with:

```go
package diagnostics

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeCaller struct {
	result  any
	err     error
	method  string
	args    []any
	timeout time.Duration
	calls   int
}

func (f *fakeCaller) Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error) {
	f.calls++
	f.method = method
	f.args = args
	f.timeout = timeout
	return f.result, f.err
}

func TestCallRequiresMethod(t *testing.T) {
	service := NewService(nil)

	result := service.Call(context.Background(), "", map[string]any{})

	if result.OK {
		t.Fatal("empty method should fail")
	}
	if result.Error != "method is required" {
		t.Fatalf("unexpected error %q", result.Error)
	}
}

func TestCallRejectsUnsupportedMethod(t *testing.T) {
	caller := &fakeCaller{}
	service := NewService(caller)

	result := service.Call(context.Background(), "runtime.inspect", map[string]any{})

	if result.OK {
		t.Fatal("unsupported method should fail")
	}
	if !strings.Contains(result.Error, "unsupported") {
		t.Fatalf("expected unsupported error, got %q", result.Error)
	}
	if caller.calls != 0 {
		t.Fatalf("unsupported method should not touch runtime, calls=%d", caller.calls)
	}
}

func TestCallReportsRuntimeNotConnectedForSupportedMethod(t *testing.T) {
	service := NewService(nil)

	result := service.Call(context.Background(), "host.describe", map[string]any{})

	if result.OK {
		t.Fatal("diagnostic should fail before runtime is connected")
	}
	if result.Error != "runtime is not connected" {
		t.Fatalf("unexpected error %q", result.Error)
	}
	if result.Method != "host.describe" {
		t.Fatalf("unexpected method %q", result.Method)
	}
}

func TestCallForwardsSupportedMethodToRuntimeCaller(t *testing.T) {
	caller := &fakeCaller{
		result: map[string]any{"appPlatform": "qq"},
	}
	service := NewService(caller)

	result := service.Call(context.Background(), "host.describe", map[string]any{})

	if !result.OK {
		t.Fatalf("expected ok result, got %#v", result)
	}
	if caller.calls != 1 {
		t.Fatalf("expected one runtime call, got %d", caller.calls)
	}
	if caller.method != "host.describe" {
		t.Fatalf("unexpected method %q", caller.method)
	}
	if len(caller.args) != 0 {
		t.Fatalf("expected empty args, got %#v", caller.args)
	}
	if caller.timeout != 15*time.Second {
		t.Fatalf("unexpected timeout %s", caller.timeout)
	}
}

func TestCallUsesArgsArrayWhenProvided(t *testing.T) {
	caller := &fakeCaller{result: map[string]any{"ok": true}}
	service := NewService(caller)

	result := service.Call(context.Background(), "gameCtl.probe", map[string]any{
		"args": []any{"quick", map[string]any{"level": float64(1)}},
	})

	if !result.OK {
		t.Fatalf("expected ok result, got %#v", result)
	}
	if len(caller.args) != 2 {
		t.Fatalf("expected two args, got %#v", caller.args)
	}
	if caller.args[0] != "quick" {
		t.Fatalf("unexpected args %#v", caller.args)
	}
}

func TestCallWrapsNonArgsParamsAsSingleArgument(t *testing.T) {
	caller := &fakeCaller{result: map[string]any{"ok": true}}
	service := NewService(caller)

	result := service.Call(context.Background(), "gameCtl.probe", map[string]any{
		"mode": "quick",
	})

	if !result.OK {
		t.Fatalf("expected ok result, got %#v", result)
	}
	if len(caller.args) != 1 {
		t.Fatalf("expected one arg, got %#v", caller.args)
	}
	arg, ok := caller.args[0].(map[string]any)
	if !ok {
		t.Fatalf("expected params map argument, got %#v", caller.args[0])
	}
	if arg["mode"] != "quick" {
		t.Fatalf("unexpected arg %#v", arg)
	}
}

func TestCallReturnsRuntimeError(t *testing.T) {
	caller := &fakeCaller{err: errors.New("gameCtl_not_ready")}
	service := NewService(caller)

	result := service.Call(context.Background(), "gameCtl.probe", map[string]any{})

	if result.OK {
		t.Fatal("expected runtime error result")
	}
	if result.Error != "gameCtl_not_ready" {
		t.Fatalf("unexpected error %q", result.Error)
	}
}
```

- [ ] **Step 2: Run the red diagnostics tests**

Run:

```powershell
go test ./internal/diagnostics
```

Expected: FAIL because `NewService` does not accept a caller and diagnostics never forwards runtime calls yet.

---

### Task 5: Implement Diagnostics Runtime Forwarding

**Files:**
- Modify: `internal/diagnostics/service.go`

- [ ] **Step 1: Replace diagnostics service**

Replace `internal/diagnostics/service.go` with:

```go
package diagnostics

import (
	"context"
	"errors"
	"time"
)

type RuntimeCaller interface {
	Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error)
}

type Result struct {
	Method     string `json:"method"`
	OK         bool   `json:"ok"`
	DurationMS int64  `json:"durationMs"`
	Result     any    `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
}

type Service struct {
	caller RuntimeCaller
}

func NewService(caller RuntimeCaller) *Service {
	return &Service{caller: caller}
}

func (s *Service) Call(ctx context.Context, method string, params map[string]any) Result {
	start := time.Now()
	if method == "" {
		return Result{
			Method:     method,
			OK:         false,
			DurationMS: time.Since(start).Milliseconds(),
			Error:      "method is required",
		}
	}

	if method != "host.describe" && method != "gameCtl.probe" {
		err := errors.New("unsupported diagnostic method")
		return Result{
			Method:     method,
			OK:         false,
			DurationMS: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		}
	}

	if s.caller == nil {
		return Result{
			Method:     method,
			OK:         false,
			DurationMS: time.Since(start).Milliseconds(),
			Error:      "runtime is not connected",
		}
	}

	value, err := s.caller.Call(ctx, method, argsFromParams(params), 15*time.Second)
	if err != nil {
		return Result{
			Method:     method,
			OK:         false,
			DurationMS: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		}
	}

	return Result{
		Method:     method,
		OK:         true,
		DurationMS: time.Since(start).Milliseconds(),
		Result:     value,
	}
}

func argsFromParams(params map[string]any) []any {
	if len(params) == 0 {
		return []any{}
	}
	if rawArgs, ok := params["args"]; ok {
		if args, ok := rawArgs.([]any); ok {
			return args
		}
	}
	return []any{params}
}
```

- [ ] **Step 2: Update app tests that construct diagnostics**

Modify `internal/app/app_test.go` so both `NewService` calls pass `nil`:

```go
service := NewService(farmruntime.NewManager(), diagnostics.NewService(nil), cfg)
```

- [ ] **Step 3: Update root app construction**

Modify `NewApp` in `app.go` to construct the adapter before the app service:

```go
func NewApp() *App {
	cfg := config.Default()
	manager := farmruntime.NewManager()
	adapter := qqws.New(cfg.QQWS, manager)
	return &App{
		cfg:     cfg,
		manager: manager,
		service: farmapp.NewService(manager, diagnostics.NewService(adapter), cfg),
		adapter: adapter,
		dataDir: defaultDataDir(),
	}
}
```

- [ ] **Step 4: Run diagnostics and app tests**

Run:

```powershell
go test ./internal/diagnostics ./internal/app
```

Expected: PASS.

---

### Task 6: Add Host Script Asset Red Test

**Files:**
- Create: `internal/runtime/qqws/host_asset_test.go`

- [ ] **Step 1: Write the failing asset test**

Create `internal/runtime/qqws/host_asset_test.go`:

```go
package qqws

import (
	"os"
	"strings"
	"testing"
)

func TestQQHostAssetIsAdaptedForFarmGo(t *testing.T) {
	content, err := os.ReadFile("../../../resources/qq/qq-host.js")
	if err != nil {
		t.Fatalf("read qq host asset: %v", err)
	}
	text := string(content)

	for _, placeholder := range []string{
		"__QQ_FARM_HOST_RPC_PATHS__",
		"__QQ_FARM_HOST_WS_URL__",
		"__QQ_FARM_BUNDLE_HASH__",
		"__QQ_FARM_HOST_VERSION__",
	} {
		if strings.Contains(text, placeholder) {
			t.Fatalf("asset still contains placeholder %s", placeholder)
		}
	}

	if !strings.Contains(text, `url: "ws://127.0.0.1:8787/runtime/qqws"`) {
		t.Fatal("asset should point to Farm_Go local qqws URL")
	}
	if !strings.Contains(text, `var hostPaths = ["host.ping", "host.describe", "gameCtl.probe"];`) {
		t.Fatal("asset should limit first-stage RPC paths")
	}
	if !strings.Contains(text, `hostVersion: "farm-go-host-1"`) {
		t.Fatal("asset should report Farm_Go host version")
	}
}
```

- [ ] **Step 2: Run the red asset test**

Run:

```powershell
go test ./internal/runtime/qqws -run TestQQHostAssetIsAdaptedForFarmGo
```

Expected: FAIL because `resources/qq/qq-host.js` does not exist yet.

---

### Task 7: Copy and Adapt QQ Host Script Asset

**Files:**
- Create: `resources/qq/qq-host.js`

- [ ] **Step 1: Copy only the necessary host script**

Run:

```powershell
New-Item -ItemType Directory -Force -Path 'resources\qq'
Copy-Item -LiteralPath 'E:\desktop\farm-tauri\core\qq-host.js' -Destination 'resources\qq\qq-host.js'
```

Expected: `resources\qq\qq-host.js` exists in Farm_Go. Do not run any write command inside `E:\desktop\farm-tauri`.

- [ ] **Step 2: Adapt placeholders inside the copied file**

Modify `resources/qq/qq-host.js`:

```diff
-  var hostPaths = __QQ_FARM_HOST_RPC_PATHS__;
+  var hostPaths = ["host.ping", "host.describe", "gameCtl.probe"];
```

```diff
-    url: "__QQ_FARM_HOST_WS_URL__",
+    url: "ws://127.0.0.1:8787/runtime/qqws",
```

```diff
-    return "__QQ_FARM_BUNDLE_HASH__";
+    return "farm-go-debug-link";
```

```diff
-      hostVersion: "__QQ_FARM_HOST_VERSION__"
+      hostVersion: "farm-go-host-1"
```

```diff
-      version: "__QQ_FARM_HOST_VERSION__",
+      version: "farm-go-host-1",
```

```diff
-    __hostVersion: "__QQ_FARM_HOST_VERSION__",
-    __bundleHash: "__QQ_FARM_BUNDLE_HASH__",
+    __hostVersion: "farm-go-host-1",
+    __bundleHash: "farm-go-debug-link",
```

- [ ] **Step 3: Run the asset test**

Run:

```powershell
go test ./internal/runtime/qqws -run TestQQHostAssetIsAdaptedForFarmGo
```

Expected: PASS.

---

### Task 8: Verify Full Go Runtime

**Files:**
- No new files.

- [ ] **Step 1: Run all Go tests**

Run:

```powershell
go test ./...
```

Expected: PASS for all Go packages.

- [ ] **Step 2: Fix compile errors with narrow edits**

If the command reports constructor mismatches for `diagnostics.NewService`, update call sites to pass either `nil` in tests or the QQ WS adapter in runtime construction. Re-run:

```powershell
go test ./...
```

Expected: PASS for all Go packages.

---

### Task 9: Verify Frontend Still Builds

**Files:**
- No planned frontend file edits.

- [ ] **Step 1: Run frontend tests**

Run:

```powershell
cd frontend
npm test
```

Expected: PASS.

- [ ] **Step 2: Run frontend build**

Run:

```powershell
cd frontend
npm run build
```

Expected: PASS and `frontend/dist` is regenerated.

---

### Task 10: Final Source Boundary Check

**Files:**
- No new files.

- [ ] **Step 1: Confirm the reference project was not modified**

Run:

```powershell
git -C 'E:\desktop\farm-tauri' status --short
```

Expected: Output may show pre-existing modified files, but no command in this plan writes to `E:\desktop\farm-tauri`. Compare the output with the initial status if needed; this task must not run any restore or checkout command.

- [ ] **Step 2: Confirm only one reference asset was copied**

Run:

```powershell
Get-ChildItem -Recurse -File 'resources' | Select-Object -ExpandProperty FullName
```

Expected: The copied QQ host asset path appears as `E:\desktop\Farm_Go\resources\qq\qq-host.js`. No WMPF, Frida, CDP, Tauri, Node gateway, or game config directories should appear under `resources`.

- [ ] **Step 3: Run final verification**

Run:

```powershell
go test ./...
cd frontend
npm test
npm run build
```

Expected: All commands exit with code 0.

## Self-Review

- Spec coverage:
  - Compatible `hello` / `helloAck` handshake: Task 1 and Task 3.
  - `call` / `result` diagnostics path: Task 1, Task 3, Task 4, and Task 5.
  - Structured disconnected error: Task 4 and Task 5.
  - Copy only required QQ host script: Task 6, Task 7, and Task 10.
  - Do not modify reference project: Task 7 and Task 10.
- Placeholder scan:
  - The only placeholder strings in this plan are the exact source placeholders that Task 6 tests against and Task 7 replaces in the copied asset.
- Type consistency:
  - The adapter exposes `Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error)`.
  - Diagnostics depends on the same method through `RuntimeCaller`.
  - Tests use `Packet` with `ID`, `Type`, `TS`, and `Payload`, matching the protocol implementation.
