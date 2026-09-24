package guard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	qqFarmLaunchProtocol = "tencent://ntqq-open/?&subCmd=miniapp&action=openQQMiniApp&actionParams=%7B%22sourceType%22%3A%22open%22%2C%22appId%22%3A%221112386029%22%2C%22hostScene%22%3A%221246700100%22%7D"
	wxFarmLaunchProtocol = "weixin://launchapplet/?app_id=wx5306c5978fdb76e4"
	yybTargetDisplayName = "AndrowsLauncher.exe"
	yybLaunchParameters  = "--from \"2\" --launch-pkg-name \"wx5306c5978fdb76e4\" --launch-proc-name \"AndrowsStore.exe\" --pseudo-protocol \"androws://client/launchWithShortcut?pkgname=wx5306c5978fdb76e4&gameType=2&tray=1&silent=0\""
)

type LaunchRequest struct {
	Mode              string   `json:"mode"`
	Protocol          string   `json:"protocol,omitempty"`
	TargetDisplayName string   `json:"targetDisplayName,omitempty"`
	WorkingDirectory  string   `json:"workingDirectory,omitempty"`
	Parameters        string   `json:"parameters,omitempty"`
	ExecutableHints   []string `json:"executableHints,omitempty"`
}

func LaunchRequestForPlatform(platform string) LaunchRequest {
	request, _ := launchRequestForPlatform(platform, nil)
	return request
}

func ResolveLaunchRequestForPlatform(platform string, snapshots []HostProcessSnapshot) (LaunchRequest, error) {
	return resolveLaunchRequestForPlatform(platform, snapshots, os.Getenv, regularFileExists)
}

func resolveLaunchRequestForPlatform(
	platform string,
	snapshots []HostProcessSnapshot,
	getenv func(string) string,
	fileExists func(string) bool,
) (LaunchRequest, error) {
	if normalizePlatform(platform) != "yyb" {
		return launchRequestForPlatform(platform, nil)
	}
	for _, directory := range yybApplicationDirectoryCandidates(snapshots, getenv) {
		launcher := filepath.Join(directory, yybTargetDisplayName)
		if !fileExists(launcher) {
			continue
		}
		return LaunchRequest{
			Mode:              "yyb_shortcut",
			TargetDisplayName: launcher,
			WorkingDirectory:  directory,
			Parameters:        yybLaunchParameters,
		}, nil
	}
	return LaunchRequest{}, errors.New("未找到应用宝安装目录：请确认 Tencent\\Androws\\Application 中存在 AndrowsLauncher.exe")
}

func launchRequestForPlatform(platform string, binding *HostBinding) (LaunchRequest, error) {
	switch normalizePlatform(platform) {
	case "qq":
		return LaunchRequest{Mode: "protocol", Protocol: qqFarmLaunchProtocol}, nil
	case "wx":
		return LaunchRequest{Mode: "protocol", Protocol: wxFarmLaunchProtocol}, nil
	case "yyb":
		request := LaunchRequest{
			Mode:              "yyb_shortcut",
			TargetDisplayName: yybTargetDisplayName,
			WorkingDirectory:  yybApplicationDirFromWMPFPath(""),
			Parameters:        yybLaunchParameters,
		}
		if binding != nil {
			request.WorkingDirectory = yybApplicationDirFromWMPFPath(binding.ExecutablePath)
			if binding.ExecutablePath != "" {
				request.ExecutableHints = []string{binding.ExecutablePath}
			}
		}
		return request, nil
	default:
		return LaunchRequest{}, fmt.Errorf("unsupported_platform: %s", platform)
	}
}

func yybApplicationDirFromWMPFPath(path string) string {
	if directory, ok := inferYybApplicationDirFromWMPFPath(path); ok {
		return directory
	}
	return `D:\Program Files\Tencent\Androws\Application`
}

func yybApplicationDirectoryCandidates(snapshots []HostProcessSnapshot, getenv func(string) string) []string {
	var candidates []string
	for _, snapshot := range snapshots {
		if directory, ok := inferYybApplicationDirFromExecutablePath(snapshot.ExecutablePath); ok {
			candidates = append(candidates, directory)
		}
	}
	for _, name := range []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA", "ProgramData"} {
		if root := strings.TrimSpace(getenv(name)); root != "" {
			candidates = append(candidates, filepath.Join(root, "Tencent", "Androws", "Application"))
		}
	}
	candidates = append(candidates,
		`C:\Program Files\Tencent\Androws\Application`,
		`C:\Program Files (x86)\Tencent\Androws\Application`,
		`D:\Program Files\Tencent\Androws\Application`,
		`D:\Program Files (x86)\Tencent\Androws\Application`,
		`C:\Androws\Application`,
		`D:\Androws\Application`,
	)
	return uniqueYybApplicationDirectories(candidates)
}

func inferYybApplicationDirFromExecutablePath(path string) (string, bool) {
	if directory, ok := inferYybApplicationDirFromWMPFPath(path); ok {
		return directory, true
	}
	cleaned := filepath.Clean(strings.TrimSpace(path))
	directory := filepath.Dir(cleaned)
	if strings.EqualFold(filepath.Base(directory), "Application") &&
		strings.Contains(strings.ToLower(filepath.ToSlash(directory)), "/androws/application") {
		return directory, true
	}
	return "", false
}

func inferYybApplicationDirFromWMPFPath(path string) (string, bool) {
	normalized := strings.ReplaceAll(path, "\\", "/")
	lower := strings.ToLower(normalized)
	for _, marker := range []string{"/tencent/androws/wmpfruntime/", "/androws/wmpfruntime/"} {
		if at := strings.Index(lower, marker); at >= 0 {
			rootLen := strings.Index(strings.ToLower(marker), "/wmpfruntime/")
			if rootLen > 0 {
				return filepath.Clean(normalized[:at+rootLen] + "/Application"), true
			}
		}
	}
	return "", false
}

func uniqueYybApplicationDirectories(candidates []string) []string {
	seen := make(map[string]struct{}, len(candidates))
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		cleaned := filepath.Clean(strings.TrimSpace(candidate))
		if cleaned == "." || cleaned == "" {
			continue
		}
		key := strings.ToLower(cleaned)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, cleaned)
	}
	return result
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func normalizePlatform(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "qq", "qq_ws":
		return "qq"
	case "wx", "wechat", "wechat_cdp", "cdp":
		return "wx"
	case "yyb", "yyb_cdp":
		return "yyb"
	default:
		return strings.ToLower(strings.TrimSpace(platform))
	}
}
