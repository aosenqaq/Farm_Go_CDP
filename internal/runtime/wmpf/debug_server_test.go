package wmpf

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	farmruntime "Farm_Go/internal/runtime"

	"github.com/gorilla/websocket"
)

func TestDebugBridgeTracksMiniappConnectionLifecycle(t *testing.T) {
	bridge := NewDebugBridge(DebugBridgeOptions{})
	if err := bridge.Start(context.Background()); err != nil {
		t.Fatalf("start bridge: %v", err)
	}
	defer bridge.Close()

	if bridge.State().MiniappConnected {
		t.Fatal("miniapp should not be connected initially")
	}

	conn, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}
	waitForState(t, func() bool { return bridge.State().MiniappConnected })

	_ = conn.Close()
	waitForState(t, func() bool { return !bridge.State().MiniappConnected })
}

func TestDebugBridgeStartsLegacyMiniappAlias(t *testing.T) {
	legacyPort := freeTCPPort(t)
	bridge := NewDebugBridge(DebugBridgeOptions{LegacyDebugPort: legacyPort})
	if err := bridge.Start(context.Background()); err != nil {
		t.Fatalf("start bridge: %v", err)
	}
	defer bridge.Close()

	conn, _, err := websocket.DefaultDialer.Dial(bridge.LegacyMiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial legacy miniapp: %v", err)
	}
	defer conn.Close()

	waitForState(t, func() bool { return bridge.State().MiniappConnected })
}

func TestDebugBridgeForwardsCDPProxyMessages(t *testing.T) {
	bridge := NewDebugBridge(DebugBridgeOptions{})
	if err := bridge.Start(context.Background()); err != nil {
		t.Fatalf("start bridge: %v", err)
	}
	defer bridge.Close()

	miniapp, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}
	defer miniapp.Close()
	_ = miniapp.SetReadDeadline(time.Now().Add(2 * time.Second))

	cdp, _, err := websocket.DefaultDialer.Dial(bridge.CDPURL(), nil)
	if err != nil {
		t.Fatalf("dial cdp: %v", err)
	}
	defer cdp.Close()
	_ = cdp.SetReadDeadline(time.Now().Add(2 * time.Second))
	waitForState(t, func() bool { return bridge.State().CDPConnected })

	request := `{"id":1,"method":"Runtime.enable","params":{}}`
	if err := cdp.WriteMessage(websocket.TextMessage, []byte(request)); err != nil {
		t.Fatalf("write cdp request: %v", err)
	}

	_, rawToMiniapp, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read miniapp proxy message: %v", err)
	}
	message, err := DecodeDebugMessage(rawToMiniapp)
	if err != nil {
		t.Fatalf("decode proxy message: %v", err)
	}
	if message.Category != "chromeDevtools" || message.Payload != request {
		t.Fatalf("unexpected miniapp message %#v", message)
	}

	response := `{"id":1,"result":{}}`
	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category: "chromeDevtoolsResult",
		Payload:  response,
	})); err != nil {
		t.Fatalf("write miniapp result: %v", err)
	}

	_, rawToCDP, err := cdp.ReadMessage()
	if err != nil {
		t.Fatalf("read cdp response: %v", err)
	}
	if string(rawToCDP) != response {
		t.Fatalf("unexpected cdp response %q", rawToCDP)
	}
}

func TestDebugBridgeUsesIncreasingMiniappMessageSeq(t *testing.T) {
	bridge := NewDebugBridge(DebugBridgeOptions{})
	if err := bridge.Start(context.Background()); err != nil {
		t.Fatalf("start bridge: %v", err)
	}
	defer bridge.Close()

	miniapp, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}
	defer miniapp.Close()
	_ = miniapp.SetReadDeadline(time.Now().Add(2 * time.Second))
	waitForState(t, func() bool { return bridge.State().MiniappConnected })

	if err := bridge.ConnectJSContext("ctx-1"); err != nil {
		t.Fatalf("connect jscontext: %v", err)
	}
	_, rawConnect, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read connect jscontext: %v", err)
	}
	connect, err := DecodeDebugMessage(rawConnect)
	if err != nil {
		t.Fatalf("decode connect jscontext: %v", err)
	}

	cdp, _, err := websocket.DefaultDialer.Dial(bridge.CDPURL(), nil)
	if err != nil {
		t.Fatalf("dial cdp: %v", err)
	}
	defer cdp.Close()
	waitForState(t, func() bool { return bridge.State().CDPConnected })

	if err := cdp.WriteMessage(websocket.TextMessage, []byte(`{"id":1,"method":"Runtime.enable","params":{}}`)); err != nil {
		t.Fatalf("write Runtime.enable: %v", err)
	}
	_, rawEnable, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read Runtime.enable: %v", err)
	}
	enable, err := DecodeDebugMessage(rawEnable)
	if err != nil {
		t.Fatalf("decode Runtime.enable: %v", err)
	}

	if connect.Seq == 0 || enable.Seq == 0 || enable.Seq <= connect.Seq {
		t.Fatalf("expected increasing miniapp seq, connect=%d enable=%d", connect.Seq, enable.Seq)
	}
}

func TestDebugBridgeTracksAndConnectsMiniappJSContext(t *testing.T) {
	bridge := NewDebugBridge(DebugBridgeOptions{})
	if err := bridge.Start(context.Background()); err != nil {
		t.Fatalf("start bridge: %v", err)
	}
	defer bridge.Close()

	miniapp, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}
	defer miniapp.Close()

	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category:      "addJsContext",
		JSContextID:   "ctx-1",
		JSContextName: "gameContext",
	})); err != nil {
		t.Fatalf("write jscontext: %v", err)
	}

	context, err := bridge.WaitJSContext(context.Background(), "gameContext", time.Second)
	if err != nil {
		t.Fatalf("wait jscontext: %v", err)
	}
	if context.ID != "ctx-1" || context.Name != "gameContext" {
		t.Fatalf("unexpected jscontext %#v", context)
	}

	if err := bridge.ConnectJSContext("ctx-1"); err != nil {
		t.Fatalf("connect jscontext: %v", err)
	}

	_, raw, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read connect jscontext: %v", err)
	}
	message, err := DecodeDebugMessage(raw)
	if err != nil {
		t.Fatalf("decode connect jscontext: %v", err)
	}
	if message.Category != "connectJsContext" || message.JSContextID != "ctx-1" {
		t.Fatalf("unexpected connect message %#v", message)
	}

	state := bridge.State()
	if len(state.RecentMiniappEvents) == 0 || state.RecentMiniappEvents[len(state.RecentMiniappEvents)-1].Category != "addJsContext" {
		t.Fatalf("expected addJsContext in recent events, got %#v", state.RecentMiniappEvents)
	}
}

func TestDebugBridgeTracksMiniappDecodeErrors(t *testing.T) {
	bridge := NewDebugBridge(DebugBridgeOptions{})
	if err := bridge.Start(context.Background()); err != nil {
		t.Fatalf("start bridge: %v", err)
	}
	defer bridge.Close()

	miniapp, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}
	defer miniapp.Close()

	if err := miniapp.WriteMessage(websocket.BinaryMessage, []byte{0xff, 0x00}); err != nil {
		t.Fatalf("write bad packet: %v", err)
	}

	waitForState(t, func() bool { return bridge.State().LastMiniappDecodeErr != "" })
	state := bridge.State()
	if state.LastMiniappRawBytes != 2 {
		t.Fatalf("expected raw byte count, got %#v", state)
	}
}

func TestDebugBridgeClearsJSContextsAfterMiniappDisconnect(t *testing.T) {
	bridge := NewDebugBridge(DebugBridgeOptions{})
	if err := bridge.Start(context.Background()); err != nil {
		t.Fatalf("start bridge: %v", err)
	}
	defer bridge.Close()

	miniapp, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}

	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category:      "addJsContext",
		JSContextID:   "ctx-1",
		JSContextName: "gameContext",
	})); err != nil {
		t.Fatalf("write jscontext: %v", err)
	}
	if _, err := bridge.WaitJSContext(context.Background(), "gameContext", time.Second); err != nil {
		t.Fatalf("wait jscontext: %v", err)
	}

	_ = miniapp.Close()
	waitForState(t, func() bool { return !bridge.State().MiniappConnected })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if context, err := bridge.WaitJSContext(ctx, "gameContext", 20*time.Millisecond); err == nil {
		t.Fatalf("expected stale jscontext to be cleared, got %#v", context)
	}
}

func TestDebugBridgeForwardsCDPWithConnectedJSContext(t *testing.T) {
	bridge := NewDebugBridge(DebugBridgeOptions{})
	if err := bridge.Start(context.Background()); err != nil {
		t.Fatalf("start bridge: %v", err)
	}
	defer bridge.Close()

	miniapp, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}
	defer miniapp.Close()
	_ = miniapp.SetReadDeadline(time.Now().Add(2 * time.Second))
	waitForState(t, func() bool { return bridge.State().MiniappConnected })

	if err := bridge.ConnectJSContext("ctx-1"); err != nil {
		t.Fatalf("connect jscontext: %v", err)
	}
	if _, _, err := miniapp.ReadMessage(); err != nil {
		t.Fatalf("read initial connect jscontext: %v", err)
	}

	cdp, _, err := websocket.DefaultDialer.Dial(bridge.CDPURL(), nil)
	if err != nil {
		t.Fatalf("dial cdp: %v", err)
	}
	defer cdp.Close()
	waitForState(t, func() bool { return bridge.State().CDPConnected })

	request := `{"id":1,"method":"Runtime.enable","params":{}}`
	if err := cdp.WriteMessage(websocket.TextMessage, []byte(request)); err != nil {
		t.Fatalf("write cdp request: %v", err)
	}

	_, raw, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read miniapp proxy message: %v", err)
	}
	message, err := DecodeDebugMessage(raw)
	if err != nil {
		t.Fatalf("decode proxy message: %v", err)
	}
	if message.Category != "chromeDevtools" || message.JSContextID != "ctx-1" {
		t.Fatalf("unexpected cdp proxy message %#v", message)
	}
}

func TestCDPLinkAutoConnectsMiniappJSContextAndBecomesReady(t *testing.T) {
	manager := farmruntime.NewManager()
	bridge := NewDebugBridge(DebugBridgeOptions{})
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge: bridge,
	})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("start link: %v", err)
	}
	defer link.Stop(context.Background())

	miniapp, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}
	defer miniapp.Close()

	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category:      "addJsContext",
		JSContextID:   "ctx-1",
		JSContextName: "gameContext",
	})); err != nil {
		t.Fatalf("write jscontext: %v", err)
	}

	_, rawConnect, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read connect jscontext: %v", err)
	}
	connect, err := DecodeDebugMessage(rawConnect)
	if err != nil {
		t.Fatalf("decode connect jscontext: %v", err)
	}
	if connect.Category != "connectJsContext" || connect.JSContextID != "ctx-1" {
		t.Fatalf("unexpected connect message %#v", connect)
	}

	_, rawEnable, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read Runtime.enable: %v", err)
	}
	enable, err := DecodeDebugMessage(rawEnable)
	if err != nil {
		t.Fatalf("decode Runtime.enable: %v", err)
	}
	if enable.Category != "chromeDevtools" || enable.JSContextID != "ctx-1" {
		t.Fatalf("unexpected Runtime.enable message %#v", enable)
	}
	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category:    "chromeDevtoolsResult",
		JSContextID: "ctx-1",
		Payload:     `{"id":1,"result":{}}`,
	})); err != nil {
		t.Fatalf("write Runtime.enable result: %v", err)
	}
	ackRuntimeEventBinding(t, miniapp)
	ackFetchInterception(t, miniapp)
	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category:    "chromeDevtoolsResult",
		JSContextID: "ctx-1",
		Payload:     `{"method":"Runtime.executionContextCreated","params":{"context":{"id":7,"name":"gameContext","origin":"https://servicewechat.com"}}}`,
	})); err != nil {
		t.Fatalf("write execution context: %v", err)
	}
	_, rawEvaluate, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read Runtime.evaluate: %v", err)
	}
	evaluate, err := DecodeDebugMessage(rawEvaluate)
	if err != nil {
		t.Fatalf("decode Runtime.evaluate: %v", err)
	}
	if evaluate.Category != "chromeDevtools" || evaluate.JSContextID != "ctx-1" {
		t.Fatalf("unexpected Runtime.evaluate message %#v", evaluate)
	}
	writeDebugCDPEvaluateResult(t, miniapp, evaluate, map[string]any{
		"hasGameCtl": true, "methodCount": 3, "scene": "Farm", "farmRoot": "root",
	})
	ackRuntimeEventWrapper(t, miniapp)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := manager.Status()
		if status.Phase == farmruntime.PhaseReady && status.InstanceID == "gameContext" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("link did not become ready: %#v", manager.Status())
}

func TestCDPLinkWaitsForJSContextBeforeRuntimeEnable(t *testing.T) {
	manager := farmruntime.NewManager()
	bridge := NewDebugBridge(DebugBridgeOptions{})
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge: bridge,
	})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("start link: %v", err)
	}
	defer link.Stop(context.Background())

	miniapp, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}
	defer miniapp.Close()

	time.Sleep(1800 * time.Millisecond)
	if bridge.State().CDPConnected {
		t.Fatal("CDP client connected before addJsContext")
	}
	_ = miniapp.SetReadDeadline(time.Now().Add(2 * time.Second))

	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category:      "addJsContext",
		JSContextID:   "ctx-delayed",
		JSContextName: "gameContext",
	})); err != nil {
		t.Fatalf("write add js context: %v", err)
	}

	_, rawConnect, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read connect js context: %v", err)
	}
	connect, err := DecodeDebugMessage(rawConnect)
	if err != nil {
		t.Fatalf("decode connect js context: %v", err)
	}
	if connect.Category != "connectJsContext" || connect.JSContextID != "ctx-delayed" {
		t.Fatalf("unexpected connect message %#v", connect)
	}

	_, rawEnable, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read Runtime.enable: %v", err)
	}
	enable, err := DecodeDebugMessage(rawEnable)
	if err != nil {
		t.Fatalf("decode Runtime.enable: %v", err)
	}
	if enable.Category != "chromeDevtools" || enable.JSContextID != "ctx-delayed" {
		t.Fatalf("unexpected Runtime.enable message %#v", enable)
	}
	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category:    "chromeDevtoolsResult",
		JSContextID: "ctx-delayed",
		Payload:     `{"id":1,"result":{}}`,
	})); err != nil {
		t.Fatalf("write Runtime.enable result: %v", err)
	}
	ackRuntimeEventBinding(t, miniapp)
	ackFetchInterception(t, miniapp)
	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category:    "chromeDevtoolsResult",
		JSContextID: "ctx-delayed",
		Payload:     `{"method":"Runtime.executionContextCreated","params":{"context":{"id":9,"name":"gameContext","origin":"https://servicewechat.com"}}}`,
	})); err != nil {
		t.Fatalf("write execution context: %v", err)
	}
	_, rawEvaluate, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read fallback Runtime.evaluate: %v", err)
	}
	evaluate, err := DecodeDebugMessage(rawEvaluate)
	if err != nil {
		t.Fatalf("decode Runtime.evaluate: %v", err)
	}
	if evaluate.Category != "chromeDevtools" || evaluate.JSContextID != "ctx-delayed" {
		t.Fatalf("unexpected Runtime.evaluate message %#v", evaluate)
	}
	writeDebugCDPEvaluateResult(t, miniapp, evaluate, map[string]any{
		"hasGameCtl": true, "methodCount": 3, "scene": "Farm", "farmRoot": "root",
	})
	ackRuntimeEventWrapper(t, miniapp)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := manager.Status()
		if status.Phase == farmruntime.PhaseReady && status.InstanceID == "gameContext" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("link did not become ready after delayed js context: %#v", manager.Status())
}

func TestCDPLinkUsesDirectRuntimeEnableAfterSetupContext(t *testing.T) {
	manager := farmruntime.NewManager()
	bridge := NewDebugBridge(DebugBridgeOptions{})
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge: bridge,
	})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("start link: %v", err)
	}
	defer link.Stop(context.Background())

	miniapp, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}
	defer miniapp.Close()
	_ = miniapp.SetReadDeadline(time.Now().Add(2 * time.Second))

	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category: "setupContext",
		Payload:  "configure_js",
	})); err != nil {
		t.Fatalf("write setupContext: %v", err)
	}

	_, rawEnable, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read Runtime.enable: %v", err)
	}
	enable, err := DecodeDebugMessage(rawEnable)
	if err != nil {
		t.Fatalf("decode Runtime.enable: %v", err)
	}
	if enable.Category != "chromeDevtools" || enable.JSContextID != "" {
		t.Fatalf("unexpected direct Runtime.enable message %#v", enable)
	}
	status := manager.Status()
	if status.Phase != farmruntime.PhaseHandshaking || !status.Connected || status.LastError != "" || status.ProgressDetail != "miniapp setupContext received; using direct CDP Runtime.enable fallback" {
		t.Fatalf("unexpected direct fallback status %#v", status)
	}
	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category: "chromeDevtoolsResult",
		Payload:  `{"id":1,"result":{}}`,
	})); err != nil {
		t.Fatalf("write Runtime.enable result: %v", err)
	}
	ackRuntimeEventBinding(t, miniapp)
	ackFetchInterception(t, miniapp)
	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category: "chromeDevtoolsResult",
		Payload:  `{"method":"Runtime.executionContextCreated","params":{"context":{"id":13,"name":"gameContext","origin":"https://servicewechat.com"}}}`,
	})); err != nil {
		t.Fatalf("write execution context: %v", err)
	}
	_, rawEvaluate, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read Runtime.evaluate: %v", err)
	}
	evaluate, err := DecodeDebugMessage(rawEvaluate)
	if err != nil {
		t.Fatalf("decode Runtime.evaluate: %v", err)
	}
	if evaluate.Category != "chromeDevtools" || evaluate.JSContextID != "" {
		t.Fatalf("unexpected direct Runtime.evaluate message %#v", evaluate)
	}
	writeDebugCDPEvaluateResult(t, miniapp, evaluate, map[string]any{
		"hasGameCtl": true, "methodCount": 3, "scene": "Farm", "farmRoot": "root",
	})
	ackRuntimeEventWrapper(t, miniapp)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := manager.Status()
		if status.Phase == farmruntime.PhaseReady && status.InstanceID == "gameContext" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("link did not become ready through setupContext fallback: %#v", manager.Status())
}

func TestCDPLinkSelectsSyntheticRuntimeContextBeforeGameCtlInjection(t *testing.T) {
	manager := farmruntime.NewManager()
	bridge := NewDebugBridge(DebugBridgeOptions{})
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge: bridge,
	})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("start link: %v", err)
	}
	defer link.Stop(context.Background())

	miniapp, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}
	defer miniapp.Close()
	_ = miniapp.SetReadDeadline(time.Now().Add(3 * time.Second))

	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{Category: "setupContext"})); err != nil {
		t.Fatalf("write setupContext: %v", err)
	}
	_, rawEnable, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read Runtime.enable: %v", err)
	}
	enable, err := DecodeDebugMessage(rawEnable)
	if err != nil {
		t.Fatalf("decode Runtime.enable: %v", err)
	}
	if enable.Category != "chromeDevtools" || enable.JSContextID != "" {
		t.Fatalf("unexpected direct Runtime.enable message %#v", enable)
	}
	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category: "chromeDevtoolsResult",
		Payload:  `{"id":1,"result":{}}`,
	})); err != nil {
		t.Fatalf("write Runtime.enable result: %v", err)
	}
	ackRuntimeEventBinding(t, miniapp)
	ackFetchInterception(t, miniapp)
	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category: "chromeDevtoolsResult",
		Payload:  `{"method":"Runtime.consoleAPICalled","params":{"type":"trace","executionContextId":2,"args":[{"type":"string","value":"getFixedWindowSize"}]}}`,
	})); err != nil {
		t.Fatalf("write console context event: %v", err)
	}

	_, rawEvaluate, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read Runtime.evaluate: %v", err)
	}
	evaluate, err := DecodeDebugMessage(rawEvaluate)
	if err != nil {
		t.Fatalf("decode Runtime.evaluate: %v", err)
	}
	if evaluate.Category != "chromeDevtools" || evaluate.JSContextID != "" {
		t.Fatalf("unexpected direct Runtime.evaluate message %#v", evaluate)
	}
	if got := cdpPayloadContextID(t, evaluate.Payload); got != 2 {
		t.Fatalf("expected synthetic context 2, got %d", got)
	}
	writeDebugCDPEvaluateResult(t, miniapp, evaluate, map[string]any{"hasCc": true})
	ackRuntimeEventWrapper(t, miniapp)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := manager.Status()
		if status.Phase == farmruntime.PhaseReady && status.InstanceID == "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("link did not select synthetic runtime context: %#v", manager.Status())
}

func TestCDPLinkFallsBackToDefaultRuntimeEvaluateWhenNoContextEventsArrive(t *testing.T) {
	manager := farmruntime.NewManager()
	bridge := NewDebugBridge(DebugBridgeOptions{})
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge: bridge,
	})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("start link: %v", err)
	}
	defer link.Stop(context.Background())

	miniapp, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}
	defer miniapp.Close()
	_ = miniapp.SetReadDeadline(time.Now().Add(3 * time.Second))

	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{Category: "setupContext"})); err != nil {
		t.Fatalf("write setupContext: %v", err)
	}
	_, rawEnable, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read Runtime.enable: %v", err)
	}
	enable, err := DecodeDebugMessage(rawEnable)
	if err != nil {
		t.Fatalf("decode Runtime.enable: %v", err)
	}
	if enable.Category != "chromeDevtools" || enable.JSContextID != "" {
		t.Fatalf("unexpected direct Runtime.enable message %#v", enable)
	}
	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category: "chromeDevtoolsResult",
		Payload:  `{"id":1,"result":{}}`,
	})); err != nil {
		t.Fatalf("write Runtime.enable result: %v", err)
	}
	ackRuntimeEventBinding(t, miniapp)
	ackFetchInterception(t, miniapp)

	_, rawEvaluate, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read default Runtime.evaluate: %v", err)
	}
	evaluate, err := DecodeDebugMessage(rawEvaluate)
	if err != nil {
		t.Fatalf("decode Runtime.evaluate: %v", err)
	}
	if evaluate.Category != "chromeDevtools" || evaluate.JSContextID != "" {
		t.Fatalf("unexpected default Runtime.evaluate message %#v", evaluate)
	}
	if cdpPayloadHasContextID(t, evaluate.Payload) {
		t.Fatalf("default Runtime.evaluate should omit contextId: %s", evaluate.Payload)
	}
	writeDebugCDPEvaluateResult(t, miniapp, evaluate, map[string]any{"hasCc": true, "hasGameGlobal": true})
	ackRuntimeEventWrapper(t, miniapp)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := manager.Status()
		if status.Phase == farmruntime.PhaseReady && status.InstanceID == "default" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("link did not select default runtime context: %#v", manager.Status())
}

func TestCDPLinkStartsDirectRuntimeEnableFromRuntimeEventTraffic(t *testing.T) {
	manager := farmruntime.NewManager()
	bridge := NewDebugBridge(DebugBridgeOptions{})
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge: bridge,
	})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("start link: %v", err)
	}
	defer link.Stop(context.Background())

	miniapp, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}
	defer miniapp.Close()
	_ = miniapp.SetReadDeadline(time.Now().Add(3 * time.Second))

	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category: "chromeDevtoolsResult",
		Payload:  `{"method":"Runtime.consoleAPICalled","params":{"type":"trace","executionContextId":2}}`,
	})); err != nil {
		t.Fatalf("write runtime event: %v", err)
	}

	_, rawEnable, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read direct Runtime.enable: %v", err)
	}
	enable, err := DecodeDebugMessage(rawEnable)
	if err != nil {
		t.Fatalf("decode Runtime.enable: %v", err)
	}
	if enable.Category != "chromeDevtools" || enable.JSContextID != "" {
		t.Fatalf("unexpected direct Runtime.enable message %#v", enable)
	}
	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category: "chromeDevtoolsResult",
		Payload:  `{"id":1,"result":{}}`,
	})); err != nil {
		t.Fatalf("write Runtime.enable result: %v", err)
	}
	ackRuntimeEventBinding(t, miniapp)
	ackFetchInterception(t, miniapp)

	_, rawEvaluate, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read Runtime.evaluate: %v", err)
	}
	evaluate, err := DecodeDebugMessage(rawEvaluate)
	if err != nil {
		t.Fatalf("decode Runtime.evaluate: %v", err)
	}
	if got := cdpPayloadContextID(t, evaluate.Payload); got != 2 {
		t.Fatalf("expected replayed runtime event to seed context 2, got %d", got)
	}
}

func TestCDPLinkProbesExecutionContextsUntilGameCtlReady(t *testing.T) {
	manager := farmruntime.NewManager()
	bridge := NewDebugBridge(DebugBridgeOptions{})
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge: bridge,
	})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("start link: %v", err)
	}
	defer link.Stop(context.Background())

	miniapp, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}
	defer miniapp.Close()
	_ = miniapp.SetReadDeadline(time.Now().Add(3 * time.Second))

	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{Category: "setupContext"})); err != nil {
		t.Fatalf("write setupContext: %v", err)
	}
	_, rawEnable, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read Runtime.enable: %v", err)
	}
	enable, err := DecodeDebugMessage(rawEnable)
	if err != nil {
		t.Fatalf("decode Runtime.enable: %v", err)
	}
	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category: "chromeDevtoolsResult",
		Payload:  `{"id":1,"result":{}}`,
	})); err != nil {
		t.Fatalf("write Runtime.enable result: %v", err)
	}
	ackRuntimeEventBinding(t, miniapp)
	ackFetchInterception(t, miniapp)
	for _, payload := range []string{
		`{"method":"Runtime.executionContextCreated","params":{"context":{"id":1,"name":"default","origin":"https://servicewechat.com"}}}`,
		`{"method":"Runtime.executionContextCreated","params":{"context":{"id":9,"name":"worker","origin":"https://servicewechat.com"}}}`,
	} {
		if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
			Category: "chromeDevtoolsResult",
			Payload:  payload,
		})); err != nil {
			t.Fatalf("write execution context: %v", err)
		}
	}

	_, rawFirstEvaluate, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read first Runtime.evaluate: %v", err)
	}
	firstEvaluate, err := DecodeDebugMessage(rawFirstEvaluate)
	if err != nil {
		t.Fatalf("decode first Runtime.evaluate: %v", err)
	}
	if got := cdpPayloadContextID(t, firstEvaluate.Payload); got != 1 {
		t.Fatalf("expected first probe to check context 1, got %d; enable=%#v", got, enable)
	}
	writeDebugCDPEvaluateResult(t, miniapp, firstEvaluate, map[string]any{"hasGameCtl": false, "methodCount": 0})

	_, rawSecondEvaluate, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read second Runtime.evaluate: %v", err)
	}
	secondEvaluate, err := DecodeDebugMessage(rawSecondEvaluate)
	if err != nil {
		t.Fatalf("decode second Runtime.evaluate: %v", err)
	}
	if got := cdpPayloadContextID(t, secondEvaluate.Payload); got != 9 {
		t.Fatalf("expected second probe to check context 9, got %d", got)
	}
	writeDebugCDPEvaluateResult(t, miniapp, secondEvaluate, map[string]any{
		"hasGameCtl": true, "methodCount": 3, "scene": "Farm", "farmRoot": "root",
	})
	ackRuntimeEventWrapper(t, miniapp)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := manager.Status()
		if status.Phase == farmruntime.PhaseReady && status.InstanceID == "worker" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("link did not select gameCtl context: %#v", manager.Status())
}

func TestCDPLinkWaitsForDelayedExecutionContext(t *testing.T) {
	manager := farmruntime.NewManager()
	bridge := NewDebugBridge(DebugBridgeOptions{})
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge: bridge,
	})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("start link: %v", err)
	}
	defer link.Stop(context.Background())

	miniapp, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}
	defer miniapp.Close()
	_ = miniapp.SetReadDeadline(time.Now().Add(7 * time.Second))

	time.Sleep(3500 * time.Millisecond)
	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category:      "addJsContext",
		JSContextID:   "ctx-late",
		JSContextName: "gameContext",
	})); err != nil {
		t.Fatalf("write delayed add js context: %v", err)
	}

	_, rawConnect, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read delayed connect js context: %v", err)
	}
	connect, err := DecodeDebugMessage(rawConnect)
	if err != nil {
		t.Fatalf("decode delayed connect js context: %v", err)
	}
	if connect.Category != "connectJsContext" || connect.JSContextID != "ctx-late" {
		t.Fatalf("unexpected delayed connect message %#v", connect)
	}

	_, rawEnable, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read Runtime.enable: %v", err)
	}
	enable, err := DecodeDebugMessage(rawEnable)
	if err != nil {
		t.Fatalf("decode Runtime.enable: %v", err)
	}
	if enable.Category != "chromeDevtools" {
		t.Fatalf("unexpected Runtime.enable message %#v", enable)
	}
	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category:    "chromeDevtoolsResult",
		JSContextID: "ctx-late",
		Payload:     `{"id":1,"result":{}}`,
	})); err != nil {
		t.Fatalf("write Runtime.enable result: %v", err)
	}
	ackRuntimeEventBinding(t, miniapp)
	ackFetchInterception(t, miniapp)

	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category:    "chromeDevtoolsResult",
		JSContextID: "ctx-late",
		Payload:     `{"method":"Runtime.executionContextCreated","params":{"context":{"id":11,"name":"gameContext","origin":"https://servicewechat.com"}}}`,
	})); err != nil {
		t.Fatalf("write delayed execution context: %v", err)
	}

	_, rawEvaluate, err := miniapp.ReadMessage()
	if err != nil {
		t.Fatalf("read Runtime.evaluate after delayed context: %v", err)
	}
	evaluate, err := DecodeDebugMessage(rawEvaluate)
	if err != nil {
		t.Fatalf("decode Runtime.evaluate: %v", err)
	}
	if evaluate.Category != "chromeDevtools" {
		t.Fatalf("unexpected Runtime.evaluate message %#v", evaluate)
	}
	writeDebugCDPEvaluateResult(t, miniapp, evaluate, map[string]any{
		"hasGameCtl": true, "methodCount": 3, "scene": "Farm", "farmRoot": "root",
	})
	ackRuntimeEventWrapper(t, miniapp)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := manager.Status()
		if status.Phase == farmruntime.PhaseReady && status.InstanceID == "gameContext" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("link did not become ready after delayed context: %#v", manager.Status())
}

func TestCDPLinkRecreatesRuntimeAfterSelectedContextDestroyed(t *testing.T) {
	manager := farmruntime.NewManager()
	bridge := NewDebugBridge(DebugBridgeOptions{})
	link := NewCDPLinkWithDeps(ProfileForTarget(farmruntime.RuntimeTargetWeChatCDP), CDPLinkConfig{}, manager, CDPLinkDeps{
		Bridge: bridge,
	})
	if err := link.Start(context.Background()); err != nil {
		t.Fatalf("start link: %v", err)
	}
	defer link.Stop(context.Background())

	miniapp, _, err := websocket.DefaultDialer.Dial(bridge.MiniappURL(), nil)
	if err != nil {
		t.Fatalf("dial miniapp: %v", err)
	}
	defer miniapp.Close()
	_ = miniapp.SetReadDeadline(time.Now().Add(5 * time.Second))

	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{Category: "setupContext"})); err != nil {
		t.Fatalf("write setupContext: %v", err)
	}
	completeRuntimeContextSetup(t, miniapp, 7)
	waitForSelectedContext(t, link, 7)

	if err := miniapp.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category: "chromeDevtoolsResult",
		Payload:  `{"method":"Runtime.executionContextDestroyed","params":{"executionContextId":7}}`,
	})); err != nil {
		t.Fatalf("write destroyed context event: %v", err)
	}

	completeRuntimeContextSetup(t, miniapp, 9)
	waitForSelectedContext(t, link, 9)
}

func waitForState(t *testing.T, ready func() bool) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("state did not become ready")
}

func ackRuntimeEventBinding(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	request, payload := readDebugCDPRequest(t, conn)
	if payload["method"] != "Runtime.addBinding" {
		t.Fatalf("expected Runtime.addBinding, got %s", request.Payload)
	}
	params, _ := payload["params"].(map[string]any)
	if params["name"] != runtimeEventBindingName {
		t.Fatalf("unexpected binding params %#v", params)
	}
	writeDebugCDPResult(t, conn, request, payload["id"])
}

func ackFetchInterception(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	request, payload := readDebugCDPRequest(t, conn)
	if payload["method"] != "Fetch.enable" {
		t.Fatalf("expected Fetch.enable, got %s", request.Payload)
	}
	params, _ := payload["params"].(map[string]any)
	patterns, _ := params["patterns"].([]any)
	if len(patterns) != len(tsdkHosts) {
		t.Fatalf("Fetch.enable patterns = %d, want %d: %s", len(patterns), len(tsdkHosts), request.Payload)
	}
	writeDebugCDPResult(t, conn, request, payload["id"])
}

func completeRuntimeContextSetup(t *testing.T, conn *websocket.Conn, contextID int) {
	t.Helper()
	request, payload := readDebugCDPRequest(t, conn)
	if payload["method"] != "Runtime.enable" {
		t.Fatalf("expected Runtime.enable, got %s", request.Payload)
	}
	writeDebugCDPResult(t, conn, request, payload["id"])
	ackRuntimeEventBinding(t, conn)
	ackFetchInterception(t, conn)

	contextEvent, err := json.Marshal(map[string]any{
		"method": "Runtime.executionContextCreated",
		"params": map[string]any{
			"context": map[string]any{"id": contextID, "name": "gameContext", "origin": "https://servicewechat.com"},
		},
	})
	if err != nil {
		t.Fatalf("encode execution context: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category: "chromeDevtoolsResult",
		Payload:  string(contextEvent),
	})); err != nil {
		t.Fatalf("write execution context: %v", err)
	}

	for {
		probeRequest, probePayload := readDebugCDPRequest(t, conn)
		if probePayload["method"] != "Runtime.evaluate" {
			t.Fatalf("unexpected context probe %s", probeRequest.Payload)
		}
		probedContextID := cdpPayloadContextID(t, probeRequest.Payload)
		ready := probedContextID == contextID
		writeDebugCDPResultWithResult(t, conn, probeRequest, probePayload["id"], map[string]any{
			"result": map[string]any{"type": "object", "value": map[string]any{"hasCc": ready}},
		})
		if ready {
			break
		}
	}
	ackRuntimeEventWrapperInContext(t, conn, contextID)
}

func waitForSelectedContext(t *testing.T, link *CDPLink, contextID int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		link.mu.Lock()
		selectedID := link.context.ID
		link.mu.Unlock()
		if selectedID == contextID && link.Status().Ready {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("context %d was not selected: status=%#v", contextID, link.Status())
}

func ackRuntimeEventWrapper(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	ackRuntimeEventWrapperInContext(t, conn, -1)
}

func ackRuntimeEventWrapperInContext(t *testing.T, conn *websocket.Conn, expectedContextID int) {
	t.Helper()
	for {
		request, payload := readDebugCDPRequest(t, conn)
		if payload["method"] != "Runtime.evaluate" {
			t.Fatalf("expected runtime event wrapper evaluation, got %s", request.Payload)
		}
		params, _ := payload["params"].(map[string]any)
		expression, _ := params["expression"].(string)
		if strings.Contains(expression, "globalThis.__qqFarmRuntimeEventBridge") {
			if expectedContextID >= 0 && cdpPayloadContextID(t, request.Payload) != expectedContextID {
				t.Fatalf("wrapper used wrong context: %s", request.Payload)
			}
			writeDebugCDPResult(t, conn, request, payload["id"])
			return
		}
		if expectedContextID < 0 {
			t.Fatalf("unexpected wrapper expression %q", expression)
		}
		probedContextID := cdpPayloadContextID(t, request.Payload)
		writeDebugCDPResultWithResult(t, conn, request, payload["id"], map[string]any{
			"result": map[string]any{"type": "object", "value": map[string]any{"hasCc": probedContextID == expectedContextID}},
		})
	}
}

func readDebugCDPRequest(t *testing.T, conn *websocket.Conn) (DebugMessage, map[string]any) {
	t.Helper()
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read CDP request: %v", err)
	}
	request, err := DecodeDebugMessage(raw)
	if err != nil {
		t.Fatalf("decode CDP request: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(request.Payload), &payload); err != nil {
		t.Fatalf("decode CDP payload: %v", err)
	}
	return request, payload
}

func writeDebugCDPResult(t *testing.T, conn *websocket.Conn, request DebugMessage, id any) {
	t.Helper()
	writeDebugCDPResultWithResult(t, conn, request, id, map[string]any{})
}

func writeDebugCDPResultWithResult(t *testing.T, conn *websocket.Conn, request DebugMessage, id any, result map[string]any) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"id": id, "result": result})
	if err != nil {
		t.Fatalf("encode CDP result: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, EncodeDebugMessage(DebugMessage{
		Category:    "chromeDevtoolsResult",
		JSContextID: request.JSContextID,
		Payload:     string(payload),
	})); err != nil {
		t.Fatalf("write CDP result: %v", err)
	}
}

func writeDebugCDPEvaluateResult(t *testing.T, conn *websocket.Conn, request DebugMessage, value map[string]any) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(request.Payload), &payload); err != nil {
		t.Fatalf("decode Runtime.evaluate request: %v", err)
	}
	writeDebugCDPResultWithResult(t, conn, request, payload["id"], map[string]any{
		"result": map[string]any{"type": "object", "value": value},
	})
}

func freeTCPPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func cdpPayloadContextID(t *testing.T, payload string) int {
	t.Helper()
	var message struct {
		Params struct {
			ContextID int `json:"contextId"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(payload), &message); err != nil {
		t.Fatalf("decode cdp payload %q: %v", payload, err)
	}
	return message.Params.ContextID
}

func cdpPayloadHasContextID(t *testing.T, payload string) bool {
	t.Helper()
	var message struct {
		Params map[string]any `json:"params"`
	}
	if err := json.Unmarshal([]byte(payload), &message); err != nil {
		t.Fatalf("decode cdp payload %q: %v", payload, err)
	}
	_, ok := message.Params["contextId"]
	return ok
}
