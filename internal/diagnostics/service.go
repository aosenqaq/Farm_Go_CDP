package diagnostics

import (
	"context"
	"errors"
	"strings"
	"time"
)

type RuntimeCaller interface {
	Call(ctx context.Context, method string, args []any, timeout time.Duration) (any, error)
}

type Result struct {
	Method     string `json:"method"`
	OK         bool   `json:"ok"`
	DurationMS int64  `json:"durationMs"`
	Result     any    `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
}

type Service struct {
	caller RuntimeCaller
}

func NewService(caller RuntimeCaller) *Service {
	return &Service{caller: caller}
}

func (s *Service) Call(ctx context.Context, method string, params map[string]any) Result {
	start := time.Now()
	if method == "" {
		return Result{
			Method:     method,
			OK:         false,
			DurationMS: time.Since(start).Milliseconds(),
			Error:      "method is required",
		}
	}

	if method != "host.describe" && method != "gameCtl.probe" && !strings.HasPrefix(method, "gameCtl.") {
		err := errors.New("unsupported diagnostic method")
		return Result{
			Method:     method,
			OK:         false,
			DurationMS: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		}
	}

	if s.caller == nil {
		return Result{
			Method:     method,
			OK:         false,
			DurationMS: time.Since(start).Milliseconds(),
			Error:      "runtime is not connected",
		}
	}

	value, err := s.caller.Call(ctx, method, argsFromParams(params), 15*time.Second)
	if err != nil {
		return Result{
			Method:     method,
			OK:         false,
			DurationMS: time.Since(start).Milliseconds(),
			Error:      err.Error(),
		}
	}

	return Result{
		Method:     method,
		OK:         true,
		DurationMS: time.Since(start).Milliseconds(),
		Result:     value,
	}
}

func argsFromParams(params map[string]any) []any {
	if len(params) == 0 {
		return []any{}
	}
	if rawArgs, ok := params["args"]; ok {
		if args, ok := rawArgs.([]any); ok {
			return args
		}
	}
	return []any{params}
}
