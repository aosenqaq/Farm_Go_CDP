package app

import (
	"context"
	"testing"

	"Farm_Go/internal/config"
	"Farm_Go/internal/diagnostics"
	farmruntime "Farm_Go/internal/runtime"
)

func TestConnectionInfoBuildsLocalURLAndMasksToken(t *testing.T) {
	cfg := config.Default()
	cfg.QQWS.HostToken = "secret-token"
	service := NewService(farmruntime.NewManager(), diagnostics.NewService(nil), cfg)

	info := service.ConnectionInfo(context.Background())

	if info.URL != "ws://127.0.0.1:8787/runtime/qqws" {
		t.Fatalf("unexpected URL %q", info.URL)
	}
	if info.TokenPreview != "********oken" {
		t.Fatalf("unexpected token preview %q", info.TokenPreview)
	}
	if info.ExpectedHostVersion != "farm-go-host-1" {
		t.Fatalf("unexpected host version %q", info.ExpectedHostVersion)
	}
}

func TestConnectionInfoUsesChineseEmptyTokenPreview(t *testing.T) {
	cfg := config.Default()
	service := NewService(farmruntime.NewManager(), diagnostics.NewService(nil), cfg)

	info := service.ConnectionInfo(context.Background())

	if info.TokenPreview != "未生成" {
		t.Fatalf("unexpected token preview %q", info.TokenPreview)
	}
}
