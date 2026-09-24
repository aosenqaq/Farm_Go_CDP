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

func TestCallAllowsGameCtlRuntimeMethods(t *testing.T) {
	caller := &fakeCaller{result: map[string]any{"farmType": "own"}}
	service := NewService(caller)

	result := service.Call(context.Background(), "gameCtl.getFarmStatus", map[string]any{
		"args": []any{map[string]any{"includeGrids": true, "silent": true}},
	})

	if !result.OK {
		t.Fatalf("expected gameCtl method to be allowed, got %#v", result)
	}
	if caller.method != "gameCtl.getFarmStatus" {
		t.Fatalf("unexpected method %q", caller.method)
	}
	if len(caller.args) != 1 {
		t.Fatalf("expected one runtime arg, got %#v", caller.args)
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
