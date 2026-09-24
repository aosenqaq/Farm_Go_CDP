package cdp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	farmruntime "Farm_Go/internal/runtime"
	"github.com/gorilla/websocket"
)

type clientEvent struct {
	method string
	params map[string]any
}

type Client struct {
	timeout time.Duration

	mu            sync.Mutex
	conn          *websocket.Conn
	nextID        int
	pending       map[int]chan commandResult
	contexts      map[int]ExecutionContext
	eventDispatch *farmruntime.OrderedDispatcher[clientEvent]

	writeMu sync.Mutex
	closeCh chan struct{}
}

func (c *Client) OnEvent(handler func(method string, params map[string]any)) func() {
	if handler == nil {
		return func() {}
	}
	return c.eventDispatch.Subscribe(func(event clientEvent) {
		handler(event.method, event.params)
	})
}

type commandResult struct {
	result map[string]any
	err    error
}

func NewClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return &Client{
		timeout:  timeout,
		nextID:   1,
		pending:  map[int]chan commandResult{},
		contexts: map[int]ExecutionContext{},
		closeCh:  make(chan struct{}),
		eventDispatch: farmruntime.NewOrderedDispatcherWithOptions("cdp", func(event clientEvent) clientEvent {
			event.params = farmruntime.CloneJSONMap(event.params)
			return event
		}, func(event clientEvent) string {
			return event.method
		}, farmruntime.OrderedDispatcherOptions[clientEvent]{
			Capacity: 256,
			Priority: clientEventPriority,
		}),
	}
}

func clientEventPriority(event clientEvent) int {
	switch event.method {
	case "Runtime.executionContextCreated",
		"Runtime.executionContextDestroyed",
		"Runtime.executionContextsCleared":
		return farmruntime.EventPriorityLifecycle
	case "Runtime.bindingCalled":
		return farmruntime.EventPriorityGuardian
	default:
		return farmruntime.EventPriorityOrdinary
	}
}

func (c *Client) Connect(ctx context.Context, url string) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	c.startEventDispatch()
	go c.readLoop()
	_, err = c.Send(ctx, "Runtime.enable", map[string]any{}, c.timeout)
	if err != nil {
		c.CloseAndWait()
	}
	return err
}

func (c *Client) DroppedEvents() uint64 {
	return c.eventDispatch.Dropped()
}

func (c *Client) Send(ctx context.Context, method string, params map[string]any, timeout time.Duration) (map[string]any, error) {
	if timeout <= 0 {
		timeout = c.timeout
	}

	c.mu.Lock()
	conn := c.conn
	if conn == nil {
		c.mu.Unlock()
		return nil, errors.New("CDP not connected")
	}
	id := c.nextID
	c.nextID++
	done := make(chan commandResult, 1)
	c.pending[id] = done
	c.mu.Unlock()

	message := map[string]any{
		"id":     id,
		"method": method,
		"params": params,
	}
	c.writeMu.Lock()
	err := conn.WriteJSON(message)
	c.writeMu.Unlock()
	if err != nil {
		c.removePending(id)
		return nil, err
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case outcome := <-done:
		return outcome.result, outcome.err
	case <-timer.C:
		c.removePending(id)
		return nil, fmt.Errorf("CDP timeout: %s (%dms)", method, timeout.Milliseconds())
	case <-ctx.Done():
		c.removePending(id)
		return nil, ctx.Err()
	}
}

func (c *Client) Evaluate(ctx context.Context, expression string, contextID int, timeout time.Duration) (any, error) {
	params := map[string]any{
		"expression":            expression,
		"returnByValue":         true,
		"userGesture":           true,
		"awaitPromise":          true,
		"includeCommandLineAPI": true,
	}
	if contextID > 0 {
		params["contextId"] = contextID
	}

	result, err := c.Send(ctx, "Runtime.evaluate", params, timeout)
	if err != nil {
		return nil, err
	}
	if details, ok := result["exceptionDetails"]; ok && details != nil {
		return nil, fmt.Errorf("Runtime.evaluate failed: %v", details)
	}
	remote, ok := result["result"].(map[string]any)
	if !ok {
		return nil, nil
	}
	if value, ok := remote["value"]; ok {
		return value, nil
	}
	if value, ok := remote["unserializableValue"]; ok {
		return value, nil
	}
	if remote["type"] == "undefined" {
		return nil, nil
	}
	return remote, nil
}

func (c *Client) Contexts() []ExecutionContext {
	c.mu.Lock()
	defer c.mu.Unlock()

	contexts := make([]ExecutionContext, 0, len(c.contexts))
	for _, context := range c.contexts {
		contexts = append(contexts, context)
	}
	return contexts
}

func (c *Client) Close() {
	c.closeTransport()
	c.eventDispatch.RequestStop()
}

func (c *Client) CloseAndWait() {
	_ = c.CloseAndWaitContext(context.Background())
}

func (c *Client) CloseAndWaitContext(ctx context.Context) error {
	c.closeTransport()
	return c.eventDispatch.CloseAndWaitContext(ctx)
}

func (c *Client) closeTransport() {
	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	for id, done := range c.pending {
		delete(c.pending, id)
		done <- commandResult{err: errors.New("CDP WebSocket closed")}
	}
	c.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

func (c *Client) readLoop() {
	for {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			return
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			c.CloseAndWait()
			return
		}

		var message map[string]any
		if err := json.Unmarshal(data, &message); err != nil {
			continue
		}
		if rawID, ok := message["id"]; ok {
			id := intFromAny(rawID)
			if id != 0 {
				c.finishPending(id, message)
				continue
			}
		}
		if method, ok := message["method"].(string); ok {
			c.handleEvent(method, message["params"])
		}
	}
}

func (c *Client) finishPending(id int, message map[string]any) {
	c.mu.Lock()
	done := c.pending[id]
	if done != nil {
		delete(c.pending, id)
	}
	c.mu.Unlock()

	if done == nil {
		return
	}
	if rawErr, ok := message["error"]; ok && rawErr != nil {
		done <- commandResult{err: fmt.Errorf("CDP error: %v", rawErr)}
		return
	}
	result, _ := message["result"].(map[string]any)
	if result == nil {
		result = map[string]any{}
	}
	done <- commandResult{result: result}
}

func (c *Client) removePending(id int) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

func (c *Client) handleEvent(method string, rawParams any) {
	params, _ := rawParams.(map[string]any)
	switch method {
	case "Runtime.executionContextCreated":
		rawContext, _ := params["context"].(map[string]any)
		context := executionContextFromMap(rawContext)
		if context.ID != 0 {
			c.mu.Lock()
			c.contexts[context.ID] = context
			c.mu.Unlock()
		}
	case "Runtime.executionContextDestroyed":
		id := intFromAny(params["executionContextId"])
		c.mu.Lock()
		delete(c.contexts, id)
		c.mu.Unlock()
	case "Runtime.executionContextsCleared":
		c.mu.Lock()
		c.contexts = map[int]ExecutionContext{}
		c.mu.Unlock()
	}
	if strings.HasPrefix(method, "Runtime.") {
		id := intFromAny(params["executionContextId"])
		if id != 0 {
			c.mu.Lock()
			if _, ok := c.contexts[id]; !ok {
				c.contexts[id] = ExecutionContext{ID: id}
			}
			c.mu.Unlock()
		}
	}
	c.eventDispatch.Dispatch(clientEvent{method: method, params: params})
}

func (c *Client) startEventDispatch() {
	c.eventDispatch.Start()
}

func executionContextFromMap(raw map[string]any) ExecutionContext {
	if raw == nil {
		return ExecutionContext{}
	}
	return ExecutionContext{
		ID:     intFromAny(raw["id"]),
		Name:   stringFromAny(raw["name"]),
		Origin: stringFromAny(raw["origin"]),
	}
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		id, _ := typed.Int64()
		return int(id)
	default:
		return 0
	}
}

func stringFromAny(value any) string {
	text, _ := value.(string)
	return text
}
