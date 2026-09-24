package wmpf

import (
	"strings"

	farmruntime "Farm_Go/internal/runtime"
)

type Profile struct {
	Target farmruntime.RuntimeTarget
	Name   string
}

type ProcessMatch struct {
	Path    string
	Version string
}

func ProfileForTarget(target farmruntime.RuntimeTarget) Profile {
	switch target {
	case farmruntime.RuntimeTargetYYBCDP:
		return Profile{Target: target, Name: "yyb_cdp"}
	default:
		return Profile{Target: farmruntime.RuntimeTargetWeChatCDP, Name: "wechat_cdp"}
	}
}

func (p Profile) MatchProcessPath(path string) (ProcessMatch, bool) {
	normalized := normalizePath(path)
	switch p.Target {
	case farmruntime.RuntimeTargetYYBCDP:
		version, ok := extractYYBVersion(normalized)
		if !ok {
			return ProcessMatch{}, false
		}
		return ProcessMatch{Path: path, Version: version}, true
	case farmruntime.RuntimeTargetWeChatCDP:
		version, ok := extractWeChatVersion(normalized)
		if ok {
			return ProcessMatch{Path: path, Version: version}, true
		}
		return ProcessMatch{}, false
	default:
		return ProcessMatch{}, false
	}
}

func extractWeChatVersion(path string) (string, bool) {
	for _, marker := range []string{
		`/tencent/xwechat/xplugin/plugins/radiumwmpf/`,
		`/tencent/xwechat/plugin/plugins/radiumwmpf/`,
	} {
		if version, ok := extractWeChatVersionAfterMarker(path, marker); ok {
			return version, true
		}
	}
	return "", false
}

func extractWeChatVersionAfterMarker(path string, marker string) (string, bool) {
	index := strings.Index(path, marker)
	if index < 0 {
		return "", false
	}
	rest := path[index+len(marker):]
	parts := strings.Split(rest, "/")
	if len(parts) < 4 || parts[0] == "" || parts[1] != "extracted" || parts[2] != "runtime" {
		return "", false
	}
	return parts[0], true
}

func extractYYBVersion(path string) (string, bool) {
	for _, marker := range []string{
		`/tencent/androws/wmpfruntime/`,
		`/androws/wmpfruntime/`,
	} {
		if version, ok := extractYYBVersionAfterMarker(path, marker); ok {
			return version, true
		}
	}
	return "", false
}

func extractYYBVersionAfterMarker(path string, marker string) (string, bool) {
	index := strings.Index(path, marker)
	if index < 0 {
		return "", false
	}
	rest := path[index+len(marker):]
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] != "runtime" {
		return "", false
	}
	return parts[0], true
}

func normalizePath(path string) string {
	return strings.ToLower(strings.ReplaceAll(path, `\`, `/`))
}
