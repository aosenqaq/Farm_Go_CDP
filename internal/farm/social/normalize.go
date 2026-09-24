package social

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var ruleListSeparator = regexp.MustCompile(`[\r\n,，;；、|]+`)

var protectedFriendGIDs = map[int]bool{
	1184649322: true,
	1142601927: true,
}

func NormalizeFriendRules(input FriendRules) FriendRules {
	blacklistScopes := NormalizeScopeList(input.BlacklistScopes)
	whitelistScopes := NormalizeScopeList(input.WhitelistScopes)

	maskedMaxLevel := input.MaskedMaxLevel
	if maskedMaxLevel < 1 {
		maskedMaxLevel = 1
	}

	return FriendRules{
		WhitelistEnabled: input.WhitelistEnabled,
		WhitelistScopes:  whitelistScopes,
		Whitelist:        NormalizeStringList(input.Whitelist),
		BlacklistEnabled: input.BlacklistEnabled,
		BlacklistScopes:  blacklistScopes,
		Blacklist:        NormalizeStringList(input.Blacklist),
		MaskedBlacklist:  input.MaskedBlacklist,
		MaskedMaxLevel:   maskedMaxLevel,
	}
}

func NormalizeScopeList(value any) []string {
	allowed := map[string]bool{"steal": true, "help": true, "mischief": true}
	items := normalizeTextItems(value)
	result := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		scope := strings.ToLower(item)
		if !allowed[scope] || seen[scope] {
			continue
		}
		seen[scope] = true
		result = append(result, scope)
	}
	return result
}

func NormalizeStringList(value any) []string {
	return normalizeTextItems(value)
}

func IsProtectedFriendGID(value any) bool {
	gid := PositiveInt(value)
	return gid > 0 && protectedFriendGIDs[gid]
}

func PositiveInt(value any) int {
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return typed
		}
	case int64:
		if typed > 0 {
			return int(typed)
		}
	case float64:
		if typed > 0 {
			return int(typed)
		}
	case float32:
		if typed > 0 {
			return int(typed)
		}
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0
		}
		parsed, err := strconv.Atoi(text)
		if err == nil && parsed > 0 {
			return parsed
		}
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" || text == "<nil>" {
			return 0
		}
		parsed, err := strconv.Atoi(text)
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
}

func normalizeTextItems(value any) []string {
	var source []string
	switch typed := value.(type) {
	case []string:
		source = typed
	case []any:
		source = make([]string, 0, len(typed))
		for _, item := range typed {
			source = append(source, fmt.Sprint(item))
		}
	case string:
		source = ruleListSeparator.Split(typed, -1)
	case nil:
		source = nil
	default:
		source = ruleListSeparator.Split(fmt.Sprint(typed), -1)
	}

	result := make([]string, 0, len(source))
	seen := map[string]bool{}
	for _, item := range source {
		text := strings.TrimSpace(item)
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		result = append(result, text)
	}
	return result
}
