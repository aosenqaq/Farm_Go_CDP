package automation

import (
	"context"
	"testing"
)

func TestRuntimeFacadeLeavesUnmigratedTasksWithoutRuntimeCall(t *testing.T) {
	caller := &fakeRuntimeCaller{value: map[string]any{"ok": true}}
	facade := NewRuntimeFacade(caller)

	result := facade.RunTask(context.Background(), "unknown_task")

	if result.Status != StatusNotMigrated {
		t.Fatalf("status = %q, want %q", result.Status, StatusNotMigrated)
	}
	if len(caller.calls) != 0 {
		t.Fatalf("unmigrated task should not call runtime, got %#v", caller.calls)
	}
}

func TestRuntimeFacadeHandlesNilRuntimeCaller(t *testing.T) {
	facade := NewRuntimeFacade(nil)

	result := facade.RunTask(context.Background(), "own_base")

	if result.Status != StatusRuntimeNotReady {
		t.Fatalf("status = %q, want %q", result.Status, StatusRuntimeNotReady)
	}
}
