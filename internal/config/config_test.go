package config

import "testing"

func TestDefaultUsesLocalQQWS(t *testing.T) {
	cfg := Default()

	if cfg.QQWS.Host != "127.0.0.1" {
		t.Fatalf("expected local host, got %q", cfg.QQWS.Host)
	}
	if cfg.QQWS.Port != 8787 {
		t.Fatalf("expected default port 8787, got %d", cfg.QQWS.Port)
	}
	if cfg.QQWS.Path != "/runtime/qqws" {
		t.Fatalf("expected qqws path, got %q", cfg.QQWS.Path)
	}
	if cfg.QQWS.ExpectedHostVersion != "farm-go-host-1" {
		t.Fatalf("unexpected host version %q", cfg.QQWS.ExpectedHostVersion)
	}
	if cfg.UI.Theme != "system" {
		t.Fatalf("expected system theme, got %q", cfg.UI.Theme)
	}
}

func TestDefaultRuntimeTargetIsQQWS(t *testing.T) {
	cfg := Default()

	if cfg.Runtime.DefaultTarget != "qq_ws" {
		t.Fatalf("expected qq_ws, got %q", cfg.Runtime.DefaultTarget)
	}
	if cfg.Runtime.CurrentTarget != "qq_ws" {
		t.Fatalf("expected current target qq_ws, got %q", cfg.Runtime.CurrentTarget)
	}
	if !cfg.Runtime.AutoStart {
		t.Fatalf("expected runtime autostart")
	}
	if cfg.CDP.Host != "127.0.0.1" {
		t.Fatalf("expected cdp host 127.0.0.1, got %q", cfg.CDP.Host)
	}
	if cfg.CDP.Port != 62000 {
		t.Fatalf("expected cdp port 62000, got %d", cfg.CDP.Port)
	}
	if cfg.CDP.TimeoutMS != 8000 {
		t.Fatalf("expected cdp timeout 8000, got %d", cfg.CDP.TimeoutMS)
	}
	if cfg.CDP.ContextName != "gameContext" {
		t.Fatalf("expected gameContext, got %q", cfg.CDP.ContextName)
	}
	if cfg.WMPF.DebugPort != 9420 {
		t.Fatalf("expected wmpf debug port 9420, got %d", cfg.WMPF.DebugPort)
	}
	if cfg.WMPF.LegacyDebugPort != 9421 {
		t.Fatalf("expected wmpf legacy debug port 9421, got %d", cfg.WMPF.LegacyDebugPort)
	}
	if !cfg.WMPF.FridaEnabled {
		t.Fatalf("expected frida hook enabled by default")
	}
}
