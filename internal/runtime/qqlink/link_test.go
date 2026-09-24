package qqlink

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"Farm_Go/internal/config"
	farmruntime "Farm_Go/internal/runtime"
	"Farm_Go/internal/runtime/qqws"
	"github.com/gorilla/websocket"
)

func TestLinkReportsQQWSTarget(t *testing.T) {
	link := New(config.Default().QQWS, farmruntime.NewManager())

	if link.Target() != farmruntime.RuntimeTargetQQWS {
		t.Fatalf("unexpected target %q", link.Target())
	}
	if link.Status().Target != "qq_ws" {
		t.Fatalf("unexpected status %#v", link.Status())
	}
}

func TestRuntimeEventHandlerCanStopLink(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	cfg := config.Default().QQWS
	cfg.Port = port
	link := New(cfg, farmruntime.NewManager())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := link.Start(ctx); err != nil {
		t.Fatalf("start link: %v", err)
	}
	defer link.StopAndWait(context.Background())

	stopped := make(chan struct{})
	link.OnRuntimeEvent(func(map[string]any) {
		if err := link.Stop(context.Background()); err != nil {
			t.Errorf("stop link: %v", err)
		}
		close(stopped)
	})

	url := fmt.Sprintf("ws://127.0.0.1:%d%s", port, cfg.Path)
	var conn *websocket.Conn
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		conn, _, err = websocket.DefaultDialer.Dial(url, nil)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("dial link: %v", err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(qqws.Packet{
		Type:    qqws.MessageEvent,
		Payload: map[string]any{"name": "guardian.stop"},
	}); err != nil {
		t.Fatalf("write runtime event: %v", err)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Link.Stop deadlocked inside runtime event callback")
	}
	if err := link.StopAndWait(context.Background()); err != nil {
		t.Fatalf("owner StopAndWait: %v", err)
	}
}

func TestLinkForwardsCallToAdapter(t *testing.T) {
	link := New(config.Default().QQWS, farmruntime.NewManager())

	_, err := link.Call(context.Background(), "host.describe", []any{}, time.Second)

	if !errors.Is(err, qqws.ErrNotConnected) {
		t.Fatalf("expected adapter not connected error, got %v", err)
	}
}

func TestLinkExposesRuntimeEventSubscription(t *testing.T) {
	link := New(config.Default().QQWS, farmruntime.NewManager())

	var subscriber interface {
		OnRuntimeEvent(func(map[string]any)) func()
	} = link
	unsubscribe := subscriber.OnRuntimeEvent(func(map[string]any) {})
	unsubscribe()
	unsubscribe()
}
