package automation

import (
	"fmt"
	"strings"
	"time"
)

const (
	friendQuietHoursEnabledConfigKey = "autoFarmFriendQuietHoursEnabled"
	friendQuietHoursStartConfigKey   = "autoFarmFriendQuietHoursStart"
	friendQuietHoursEndConfigKey     = "autoFarmFriendQuietHoursEnd"
	friendQuietHoursModeConfigKey    = "autoFarmFriendQuietHoursMode"
	friendQuietHoursScopesConfigKey  = "autoFarmFriendQuietHoursScopes"
	friendQuietHoursModeSleep        = "sleep"
	friendQuietHoursModeWork         = "work"
)

var friendQuietHoursScopeByTaskID = map[string]string{
	"friend_steal":    "steal",
	"friend_help":     "help",
	"friend_mischief": "mischief",
}

type quietHoursTaskDecision struct {
	Allowed       bool
	NextAllowedAt time.Time
}

func friendQuietHoursDecision(config map[string]any, taskID string, now time.Time) quietHoursTaskDecision {
	scope := friendQuietHoursScopeByTaskID[taskID]
	if scope == "" || !boolConfig(config[friendQuietHoursEnabledConfigKey], false) {
		return quietHoursTaskDecision{Allowed: true}
	}
	if !normalizedQuietHoursScopes(config)[scope] {
		return quietHoursTaskDecision{Allowed: true}
	}

	mode := strings.ToLower(strings.TrimSpace(fmt.Sprint(config[friendQuietHoursModeConfigKey])))
	if mode != friendQuietHoursModeWork {
		mode = friendQuietHoursModeSleep
	}
	startMinute := quietHoursMinute(config[friendQuietHoursStartConfigKey], 23*60)
	endMinute := quietHoursMinute(config[friendQuietHoursEndConfigKey], 7*60)
	if startMinute == endMinute {
		return quietHoursTaskDecision{Allowed: mode == friendQuietHoursModeWork}
	}

	active, nextStart, nextEnd := quietHoursWindow(now, startMinute, endMinute)
	if mode == friendQuietHoursModeSleep {
		if active {
			return quietHoursTaskDecision{Allowed: false, NextAllowedAt: nextEnd}
		}
		return quietHoursTaskDecision{Allowed: true}
	}
	if active {
		return quietHoursTaskDecision{Allowed: true}
	}
	return quietHoursTaskDecision{Allowed: false, NextAllowedAt: nextStart}
}

func normalizedQuietHoursScopes(config map[string]any) map[string]bool {
	value, exists := config[friendQuietHoursScopesConfigKey]
	if !exists {
		value = []string{"steal", "help"}
	}
	result := map[string]bool{}
	appendScope := func(item any) {
		scope := strings.ToLower(strings.TrimSpace(fmt.Sprint(item)))
		if scope == "steal" || scope == "help" || scope == "mischief" {
			result[scope] = true
		}
	}
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			appendScope(item)
		}
	case []any:
		for _, item := range typed {
			appendScope(item)
		}
	}
	return result
}

func quietHoursMinute(value any, fallback int) int {
	parsed, err := time.Parse("15:04", strings.TrimSpace(fmt.Sprint(value)))
	if err != nil {
		return fallback
	}
	return parsed.Hour()*60 + parsed.Minute()
}

func quietHoursWindow(now time.Time, startMinute, endMinute int) (bool, time.Time, time.Time) {
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startToday := dayStart.Add(time.Duration(startMinute) * time.Minute)
	endToday := dayStart.Add(time.Duration(endMinute) * time.Minute)
	if startMinute < endMinute {
		if now.Before(startToday) {
			return false, startToday, endToday
		}
		if now.Before(endToday) {
			return true, startToday, endToday
		}
		return false, startToday.AddDate(0, 0, 1), endToday.AddDate(0, 0, 1)
	}
	if !now.Before(startToday) {
		return true, startToday, endToday.AddDate(0, 0, 1)
	}
	if now.Before(endToday) {
		return true, startToday.AddDate(0, 0, -1), endToday
	}
	return false, startToday, endToday.AddDate(0, 0, 1)
}
