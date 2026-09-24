package runtime

import "testing"

func TestInitialStatusIsIdleQQWS(t *testing.T) {
	status := InitialStatus()

	if status.Target != "qq_ws" {
		t.Fatalf("expected qq_ws target, got %q", status.Target)
	}
	if status.Phase != PhaseIdle {
		t.Fatalf("expected idle phase, got %q", status.Phase)
	}
	if status.Connected {
		t.Fatal("initial status should not be connected")
	}
	if status.Ready {
		t.Fatal("initial status should not be ready")
	}
}

func TestManagerStoresStatus(t *testing.T) {
	manager := NewManager()
	manager.SetStatus(Status{
		Target:    "qq_ws",
		Phase:     PhaseReady,
		Connected: true,
		Ready:     true,
	})

	status := manager.Status()
	if status.Phase != PhaseReady || !status.Connected || !status.Ready {
		t.Fatalf("manager returned unexpected status: %+v", status)
	}
}
