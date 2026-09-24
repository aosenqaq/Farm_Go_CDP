package eventbus

import "time"

type Level string

const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

type Event struct {
	ID        int64          `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Level     Level          `json:"level"`
	Source    string         `json:"source"`
	Type      string         `json:"type"`
	Message   string         `json:"message"`
	Data      map[string]any `json:"data,omitempty"`
}
