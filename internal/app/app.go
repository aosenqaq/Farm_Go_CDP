package app

import (
	"context"
	"fmt"
	"strings"

	"Farm_Go/internal/config"
	"Farm_Go/internal/diagnostics"
	farmruntime "Farm_Go/internal/runtime"
)

type ConnectionInfo struct {
	URL                 string `json:"url"`
	ExpectedHostVersion string `json:"expectedHostVersion"`
	TokenPreview        string `json:"tokenPreview"`
}

type Service struct {
	runtime     *farmruntime.Manager
	diagnostics *diagnostics.Service
	cfg         config.Config
}

func NewService(runtime *farmruntime.Manager, diagnostics *diagnostics.Service, cfg config.Config) *Service {
	return &Service{
		runtime:     runtime,
		diagnostics: diagnostics,
		cfg:         cfg,
	}
}

func (s *Service) RuntimeStatus(ctx context.Context) farmruntime.Status {
	return s.runtime.Status()
}

func (s *Service) RunDiagnostic(ctx context.Context, method string, params map[string]any) diagnostics.Result {
	return s.diagnostics.Call(ctx, method, params)
}

func (s *Service) ConnectionInfo(ctx context.Context) ConnectionInfo {
	qqws := s.cfg.QQWS
	return ConnectionInfo{
		URL:                 fmt.Sprintf("ws://%s:%d%s", qqws.Host, qqws.Port, qqws.Path),
		ExpectedHostVersion: qqws.ExpectedHostVersion,
		TokenPreview:        tokenPreview(qqws.HostToken),
	}
}

func tokenPreview(token string) string {
	if token == "" {
		return "未生成"
	}
	if len(token) <= 4 {
		return strings.Repeat("*", len(token))
	}
	return strings.Repeat("*", len(token)-4) + token[len(token)-4:]
}
