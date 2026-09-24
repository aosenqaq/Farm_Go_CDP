package wmpf

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type DebugBridgeOptions struct {
	Host            string
	DebugPort       int
	LegacyDebugPort int
	CDPPort         int
}

type DebugBridgeState struct {
	MiniappConnected     bool               `json:"miniappConnected"`
	MiniappClients       int                `json:"miniappClients"`
	CDPConnected         bool               `json:"cdpConnected"`
	CDPClients           int                `json:"cdpClients"`
	JSContextID          string             `json:"jsContextId,omitempty"`
	JSContextName        string             `json:"jsContextName,omitempty"`
	RecentMiniappEvents  []DebugBridgeEvent `json:"recentMiniappEvents,omitempty"`
	LastMiniappDecodeErr string             `json:"lastMiniappDecodeErr,omitempty"`
	LastMiniappRawBytes  int                `json:"lastMiniappRawBytes,omitempty"`
}

type JSContext struct {
	ID   string
	Name string
}

type DebugBridgeEvent struct {
	At            string `json:"at"`
	Category      string `json:"category"`
	JSContextID   string `json:"jsContextId,omitempty"`
	JSContextName string `json:"jsContextName,omitempty"`
	PayloadBytes  int    `json:"payloadBytes,omitempty"`
	Summary       string `json:"summary,omitempty"`
}

type DebugBridge struct {
	options DebugBridgeOptions

	mu         sync.Mutex
	state      DebugBridgeState
	miniapps   map[*websocket.Conn]bool
	cdp        map[*websocket.Conn]bool
	jsContexts map[string]JSContext
	cdpBacklog []string
	nextSeq    uint64

	miniappServer *http.Server
	legacyServer  *http.Server
	cdpServer     *http.Server
	miniappURL    string
	legacyURL     string
	cdpURL        string
}

func NewDebugBridge(options DebugBridgeOptions) *DebugBridge {
	if options.Host == "" {
		options.Host = "127.0.0.1"
	}
	return &DebugBridge{
		options:    options,
		miniapps:   map[*websocket.Conn]bool{},
		cdp:        map[*websocket.Conn]bool{},
		jsContexts: map[string]JSContext{},
	}
}

func (b *DebugBridge) Start(ctx context.Context) error {
	miniappURL, miniappServer, err := b.startServer(ctx, b.options.DebugPort, b.handleMiniapp)
	if err != nil {
		return err
	}
	if b.options.LegacyDebugPort > 0 && b.options.LegacyDebugPort != b.options.DebugPort {
		legacyURL, legacyServer, err := b.startServer(ctx, b.options.LegacyDebugPort, b.handleMiniapp)
		if err != nil {
			_ = miniappServer.Close()
			return err
		}
		b.legacyURL = legacyURL
		b.legacyServer = legacyServer
	}
	cdpURL, cdpServer, err := b.startServer(ctx, b.options.CDPPort, b.handleCDP)
	if err != nil {
		_ = miniappServer.Close()
		if b.legacyServer != nil {
			_ = b.legacyServer.Close()
		}
		return err
	}
	b.miniappURL = miniappURL
	b.cdpURL = cdpURL
	b.miniappServer = miniappServer
	b.cdpServer = cdpServer
	return nil
}

func (b *DebugBridge) MiniappURL() string {
	return b.miniappURL
}

func (b *DebugBridge) LegacyMiniappURL() string {
	return b.legacyURL
}

func (b *DebugBridge) CDPURL() string {
	return b.cdpURL
}

func (b *DebugBridge) State() DebugBridgeState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

func (b *DebugBridge) WaitJSContext(ctx context.Context, preferredName string, timeout time.Duration) (JSContext, error) {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if context, ok := b.findJSContext(preferredName); ok {
			return context, nil
		}
		select {
		case <-ctx.Done():
			return JSContext{}, ctx.Err()
		case <-deadline.C:
			return JSContext{}, fmt.Errorf("miniapp jscontext %q not found", preferredName)
		case <-ticker.C:
		}
	}
}

func (b *DebugBridge) ConnectJSContext(id string) error {
	if id == "" {
		return fmt.Errorf("jscontext id is required")
	}
	b.mu.Lock()
	context := b.jsContexts[id]
	if context.ID == "" {
		context = JSContext{ID: id}
	}
	b.state.JSContextID = context.ID
	b.state.JSContextName = context.Name
	b.mu.Unlock()

	b.broadcastMiniapp(EncodeDebugMessage(DebugMessage{
		Seq:         b.nextMiniappSeq(),
		Category:    "connectJsContext",
		JSContextID: id,
	}))
	return nil
}

func (b *DebugBridge) Close() error {
	b.mu.Lock()
	for conn := range b.miniapps {
		_ = conn.Close()
	}
	for conn := range b.cdp {
		_ = conn.Close()
	}
	b.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if b.miniappServer != nil {
		_ = b.miniappServer.Shutdown(ctx)
	}
	if b.legacyServer != nil {
		_ = b.legacyServer.Shutdown(ctx)
	}
	if b.cdpServer != nil {
		_ = b.cdpServer.Shutdown(ctx)
	}
	return nil
}

func (b *DebugBridge) startServer(ctx context.Context, port int, handler func(*websocket.Conn)) (string, *http.Server, error) {
	addr := fmt.Sprintf("%s:%d", b.options.Host, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return "", nil, err
	}

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		handler(conn)
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			_ = server.Close()
		}
	}()
	return "ws://" + listener.Addr().String(), server, nil
}

func (b *DebugBridge) handleMiniapp(conn *websocket.Conn) {
	b.mu.Lock()
	b.miniapps[conn] = true
	b.state.MiniappClients = len(b.miniapps)
	b.state.MiniappConnected = b.state.MiniappClients > 0
	b.mu.Unlock()

	go func() {
		defer func() {
			b.mu.Lock()
			delete(b.miniapps, conn)
			b.state.MiniappClients = len(b.miniapps)
			b.state.MiniappConnected = b.state.MiniappClients > 0
			if !b.state.MiniappConnected {
				b.jsContexts = map[string]JSContext{}
				b.state.JSContextID = ""
				b.state.JSContextName = ""
			}
			b.mu.Unlock()
			_ = conn.Close()
		}()

		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			message, err := DecodeDebugMessage(raw)
			if err != nil {
				b.recordMiniappDecodeError(err, len(raw))
				continue
			}
			b.recordMiniappEvent(message, len(raw))
			switch message.Category {
			case "addJsContext":
				b.recordJSContext(message)
			case "removeJsContext":
				b.removeJSContext(message.JSContextID)
			case "chromeDevtoolsResult":
				b.recordCDPBacklog(message.Payload)
				b.broadcastCDP([]byte(message.Payload))
			}
		}
	}()
}

func (b *DebugBridge) handleCDP(conn *websocket.Conn) {
	b.mu.Lock()
	backlog := append([]string(nil), b.cdpBacklog...)
	b.mu.Unlock()
	for _, payload := range backlog {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(payload))
	}

	b.mu.Lock()
	b.cdp[conn] = true
	b.state.CDPClients = len(b.cdp)
	b.state.CDPConnected = b.state.CDPClients > 0
	b.mu.Unlock()

	go func() {
		defer func() {
			b.mu.Lock()
			delete(b.cdp, conn)
			b.state.CDPClients = len(b.cdp)
			b.state.CDPConnected = b.state.CDPClients > 0
			b.mu.Unlock()
			_ = conn.Close()
		}()

		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			b.mu.Lock()
			jsContextID := b.state.JSContextID
			b.mu.Unlock()
			b.broadcastMiniapp(EncodeDebugMessage(DebugMessage{
				Seq:         b.nextMiniappSeq(),
				Category:    "chromeDevtools",
				Payload:     string(raw),
				JSContextID: jsContextID,
			}))
		}
	}()
}

func (b *DebugBridge) findJSContext(preferredName string) (JSContext, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var fallback JSContext
	for _, context := range b.jsContexts {
		if fallback.ID == "" {
			fallback = context
		}
		if preferredName != "" && context.Name == preferredName {
			return context, true
		}
	}
	if fallback.ID != "" {
		return fallback, true
	}
	return JSContext{}, false
}

func (b *DebugBridge) nextMiniappSeq() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextSeq++
	return b.nextSeq
}

func (b *DebugBridge) recordMiniappDecodeError(err error, rawBytes int) {
	b.mu.Lock()
	b.state.LastMiniappDecodeErr = err.Error()
	b.state.LastMiniappRawBytes = rawBytes
	b.mu.Unlock()
}

func (b *DebugBridge) recordMiniappEvent(message DebugMessage, rawBytes int) {
	b.mu.Lock()
	b.state.LastMiniappDecodeErr = ""
	b.state.LastMiniappRawBytes = rawBytes
	b.state.RecentMiniappEvents = append(b.state.RecentMiniappEvents, DebugBridgeEvent{
		At:            time.Now().Format(time.RFC3339),
		Category:      message.Category,
		JSContextID:   message.JSContextID,
		JSContextName: message.JSContextName,
		PayloadBytes:  len(message.Payload),
		Summary:       summarizeDebugPayload(message),
	})
	if len(b.state.RecentMiniappEvents) > 20 {
		b.state.RecentMiniappEvents = b.state.RecentMiniappEvents[len(b.state.RecentMiniappEvents)-20:]
	}
	b.mu.Unlock()
}

func summarizeDebugPayload(message DebugMessage) string {
	if message.Payload == "" {
		return ""
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(message.Payload), &raw); err != nil {
		if len(message.Payload) > 160 {
			return message.Payload[:160]
		}
		return message.Payload
	}
	if method, _ := raw["method"].(string); method != "" {
		return method
	}
	if id, ok := raw["id"]; ok {
		if rawErr, ok := raw["error"]; ok && rawErr != nil {
			return fmt.Sprintf("id=%v error=%v", id, rawErr)
		}
		if result, _ := raw["result"].(map[string]any); result != nil {
			if remote, _ := result["result"].(map[string]any); remote != nil {
				if value, ok := remote["value"]; ok {
					return fmt.Sprintf("id=%v value=%v", id, value)
				}
				if description, _ := remote["description"].(string); description != "" {
					return fmt.Sprintf("id=%v result=%s", id, description)
				}
			}
		}
		return fmt.Sprintf("id=%v", id)
	}
	return ""
}

func (b *DebugBridge) recordCDPBacklog(payload string) {
	if !isReplayableCDPEvent(payload) {
		return
	}
	b.mu.Lock()
	b.cdpBacklog = append(b.cdpBacklog, payload)
	if len(b.cdpBacklog) > 50 {
		b.cdpBacklog = b.cdpBacklog[len(b.cdpBacklog)-50:]
	}
	b.mu.Unlock()
}

func isReplayableCDPEvent(payload string) bool {
	var raw map[string]any
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return false
	}
	method, _ := raw["method"].(string)
	return strings.HasPrefix(method, "Runtime.")
}

func (b *DebugBridge) recordJSContext(message DebugMessage) {
	if message.JSContextID == "" {
		return
	}
	b.mu.Lock()
	b.jsContexts[message.JSContextID] = JSContext{ID: message.JSContextID, Name: message.JSContextName}
	b.mu.Unlock()
}

func (b *DebugBridge) removeJSContext(id string) {
	if id == "" {
		return
	}
	b.mu.Lock()
	delete(b.jsContexts, id)
	if b.state.JSContextID == id {
		b.state.JSContextID = ""
		b.state.JSContextName = ""
	}
	b.mu.Unlock()
}

func (b *DebugBridge) broadcastMiniapp(data []byte) {
	b.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(b.miniapps))
	for conn := range b.miniapps {
		conns = append(conns, conn)
	}
	b.mu.Unlock()
	for _, conn := range conns {
		_ = conn.WriteMessage(websocket.BinaryMessage, data)
	}
}

func (b *DebugBridge) broadcastCDP(data []byte) {
	b.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(b.cdp))
	for conn := range b.cdp {
		conns = append(conns, conn)
	}
	b.mu.Unlock()
	for _, conn := range conns {
		_ = conn.WriteMessage(websocket.TextMessage, data)
	}
}
