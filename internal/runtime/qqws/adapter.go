package qqws

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
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
	writeMu     sync.Mutex
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
	runtimeEvents  *farmruntime.OrderedDispatcher[map[string]any]
	logHandler     func(level, message string, data map[string]any)
}

func (a *Adapter) OnRuntimeEvent(handler func(map[string]any)) func() {
	return a.runtimeEvents.Subscribe(handler)
}

func (a *Adapter) DroppedRuntimeEvents() uint64 {
	return a.runtimeEvents.Dropped()
}

func New(cfg config.QQWSConfig, manager *farmruntime.Manager) *Adapter {
	return &Adapter{
		cfg:     cfg,
		manager: manager,
		clients: make(map[string]*clientSession),
		pending: make(map[string]*pendingCall),
		runtimeEvents: farmruntime.NewOrderedDispatcherWithOptions("qqws", farmruntime.CloneJSONMap, func(event map[string]any) string {
			return stringFromPayload(event, "name", "unknown")
		}, farmruntime.OrderedDispatcherOptions[map[string]any]{
			Capacity: 256,
			Priority: runtimeEventPriority,
		}),
	}
}

func runtimeEventPriority(event map[string]any) int {
	name := stringFromPayload(event, "name", "")
	kind := stringFromPayload(event, "kind", "")
	if strings.HasPrefix(name, "guardian.") || strings.HasPrefix(kind, "guardian") ||
		name == "gameCtlReadyChanged" ||
		name == "network_reconnect" || name == "other_place_login_reconnect" {
		return farmruntime.EventPriorityGuardian
	}
	return farmruntime.EventPriorityOrdinary
}

func (a *Adapter) Start(ctx context.Context) error {
	a.startRuntimeEventDispatch()
	defer a.closeRuntimeEventDispatch()

	mux := http.NewServeMux()
	mux.HandleFunc(a.cfg.Path, a.handleWebSocket)

	a.server = &http.Server{
		Addr:              a.cfg.Host + ":" + portString(a.cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	a.manager.SetStatus(farmruntime.Status{
		Target: string(farmruntime.RuntimeTargetQQWS),
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
		Target:    string(farmruntime.RuntimeTargetQQWS),
		Phase:     farmruntime.PhaseError,
		LastError: err.Error(),
	})
	return err
}

func (a *Adapter) Call(ctx context.Context, path string, args []any, timeout time.Duration) (any, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	client := a.activeClient()
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

	if err := client.writeJSON(packet); err != nil {
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
		Target:     string(farmruntime.RuntimeTargetQQWS),
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
				Target:    string(farmruntime.RuntimeTargetQQWS),
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
		a.handleLog(client, packet)
	case MessagePing:
		_ = client.writeJSON(Packet{
			ID:      packet.ID,
			Type:    MessagePong,
			TS:      time.Now().UnixMilli(),
			Payload: map[string]any{},
		})
	case MessageResult:
		a.handleResult(packet)
	case MessageError:
		a.manager.SetStatus(farmruntime.Status{
			Target:    string(farmruntime.RuntimeTargetQQWS),
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
		_ = client.writeJSON(Packet{
			ID:   packet.ID,
			Type: MessageHelloAck,
			TS:   time.Now().UnixMilli(),
			Payload: map[string]any{
				"ok":    false,
				"error": errText,
			},
		})
		a.manager.SetStatus(farmruntime.Status{
			Target:    string(farmruntime.RuntimeTargetQQWS),
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
		Target:      string(farmruntime.RuntimeTargetQQWS),
		Phase:       farmruntime.PhaseReady,
		Connected:   true,
		Ready:       ready,
		InstanceID:  client.id,
		HostVersion: version,
		LastSeenAt:  time.Now().Format(time.RFC3339),
	})

	_ = client.writeJSON(Packet{
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

// OnLog registers a callback for log packets sent by the JS host script.
// Returns a deregister function. Safe to call before Start.
func (a *Adapter) OnLog(handler func(level, message string, data map[string]any)) func() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.logHandler = handler
	return func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.logHandler = nil
	}
}

func (a *Adapter) handleLog(client *clientSession, packet Packet) {
	a.touchClient(client)
	a.mu.Lock()
	handler := a.logHandler
	a.mu.Unlock()
	if handler == nil || packet.Payload == nil {
		return
	}
	message := stringFromPayload(packet.Payload, "message", "")
	if message == "" {
		return
	}
	level := stringFromPayload(packet.Payload, "level", "info")
	var data map[string]any
	if extra := packet.Payload["extra"]; extra != nil {
		if m, ok := extra.(map[string]any); ok {
			data = m
		}
	}
	handler(level, message, data)
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
	} else {
		a.touchClient(client)
	}

	// Ready changes remain observable as runtime events after status bookkeeping.
	a.forwardRuntimeEvent(packet.Payload)
}

func (a *Adapter) forwardRuntimeEvent(event map[string]any) {
	a.runtimeEvents.Dispatch(event)
}

func (a *Adapter) startRuntimeEventDispatch() {
	a.runtimeEvents.Start()
}

func (a *Adapter) closeRuntimeEventDispatch() {
	a.runtimeEvents.CloseAndWait()
}

func (a *Adapter) RequestStop() {
	a.runtimeEvents.RequestStop()
}

func (a *Adapter) CloseAndWait() {
	a.runtimeEvents.CloseAndWait()
}

func (a *Adapter) CloseAndWaitContext(ctx context.Context) error {
	return a.runtimeEvents.CloseAndWaitContext(ctx)
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

func (c *clientSession) writeJSON(packet Packet) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteJSON(packet)
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
			Target:    string(farmruntime.RuntimeTargetQQWS),
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

func (a *Adapter) activeClient() *clientSession {
	a.mu.Lock()
	defer a.mu.Unlock()

	if client := a.clients[a.activeClientID]; client != nil {
		return client
	}
	for id, client := range a.clients {
		a.activeClientID = id
		return client
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
