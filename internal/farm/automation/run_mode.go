package automation

import (
	"fmt"
	"strings"
)

type RunMode string

const (
	RunModeSafe RunMode = "safe"
	RunModeGod  RunMode = "god"
)

func ParseRunMode(value string) (RunMode, error) {
	switch RunMode(strings.ToLower(strings.TrimSpace(value))) {
	case RunModeSafe:
		return RunModeSafe, nil
	case RunModeGod:
		return RunModeGod, nil
	default:
		return "", fmt.Errorf("不支持的运行模式：%s", strings.TrimSpace(value))
	}
}

func NormalizeRunMode(value RunMode) RunMode {
	if value == RunModeSafe || value == RunModeGod {
		return value
	}
	return RunModeGod
}

func (m RunMode) IsSafe() bool {
	return NormalizeRunMode(m) == RunModeSafe
}
