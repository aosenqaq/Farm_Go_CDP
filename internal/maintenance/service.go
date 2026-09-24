package maintenance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type Target struct {
	Path         string `json:"path"`
	RelativePath string `json:"relativePath"`
	Reason       string `json:"reason"`
}

type Summary struct {
	Platform        string   `json:"platform"`
	Preview         bool     `json:"preview"`
	TargetCount     int      `json:"targetCount"`
	MovedCount      int      `json:"movedCount"`
	SkippedCount    int      `json:"skippedCount"`
	ClosedProcesses int      `json:"closedProcesses"`
	BackupDir       string   `json:"backupDir"`
	Targets         []Target `json:"targets"`
}

type Service struct{}

func NewService() *Service { return &Service{} }

func (s *Service) PreviewWeChat() (Summary, error) {
	return previewWeChat(filepath.Join(os.Getenv("APPDATA"), "Tencent", "xwechat"), "")
}

func (s *Service) CleanWeChat() (Summary, error) {
	root := filepath.Join(os.Getenv("APPDATA"), "Tencent", "xwechat")
	summary, err := previewWeChat(root, "")
	if err != nil || summary.TargetCount == 0 {
		return summary, err
	}
	summary.Preview = false
	summary.ClosedProcesses = stopByImageNames([]string{"WeChat.exe", "WeChatAppEx.exe", "WeChatPlayer.exe", "Weixin.exe", "WeixinAppEx.exe"})
	summary.BackupDir, err = createBackupDirAt(summary.BackupDir)
	if err != nil {
		return summary, err
	}
	summary.MovedCount, summary.SkippedCount, err = moveTargets(root, summary.BackupDir, summary.Targets)
	return summary, err
}

func (s *Service) CleanQQ() (Summary, error) {
	groups := qqCandidateGroups(os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA"))
	summary := Summary{Platform: "qq", Targets: flattenGroups(groups)}
	summary.TargetCount = len(summary.Targets)
	if summary.TargetCount == 0 {
		return summary, nil
	}
	summary.ClosedProcesses = stopByImageNames([]string{"QQ.exe", "QQNT.exe", "QQMiniProgram.exe", "QQMiniApp.exe", "QQEX.exe"})
	backup, err := createBackupDir("qq")
	if err != nil {
		return summary, err
	}
	summary.BackupDir = backup
	for root, targets := range groups {
		moved, skipped, moveErr := moveTargets(root, backup, targets)
		summary.MovedCount += moved
		summary.SkippedCount += skipped
		if moveErr != nil {
			return summary, moveErr
		}
	}
	return summary, nil
}

func (s *Service) CleanYYB() (Summary, error) {
	groups := yybCandidateGroups(yybRoots())
	summary := Summary{Platform: "yyb", Targets: flattenGroups(groups)}
	summary.TargetCount = len(summary.Targets)
	if summary.TargetCount == 0 {
		return summary, nil
	}
	summary.ClosedProcesses = stopYYBProcesses()
	backup, err := createBackupDir("yyb")
	if err != nil {
		return summary, err
	}
	summary.BackupDir = backup
	for root, targets := range groups {
		moved, skipped, moveErr := moveTargets(root, backup, targets)
		summary.MovedCount += moved
		summary.SkippedCount += skipped
		if moveErr != nil {
			return summary, moveErr
		}
	}
	return summary, nil
}

func previewWeChat(root, backupDir string) (Summary, error) {
	targets, err := weChatCandidates(root)
	if err != nil {
		return Summary{}, err
	}
	if len(targets) > 0 && backupDir == "" {
		backupDir, err = plannedBackupDir("wechat")
		if err != nil {
			return Summary{}, err
		}
	}
	return Summary{
		Platform:    "wechat",
		Preview:     true,
		TargetCount: len(targets),
		BackupDir:   backupDir,
		Targets:     targets,
	}, nil
}

func weChatCandidates(root string) ([]Target, error) {
	if strings.TrimSpace(root) == "" {
		return nil, nil
	}
	targets := make([]Target, 0)
	for _, item := range []struct {
		relative string
		reason   string
	}{
		{filepath.Join("xplugin", "Plugins", "RadiumWMPF"), "plugin-runtime"},
		{filepath.Join("xplugin", "Plugins", "WMPFDrm"), "plugin-runtime"},
		{filepath.Join("radium", "cache"), "runtime-cache"},
		{filepath.Join("radium", "mmkv", "codecache_appid_dict"), "runtime-cache"},
		{filepath.Join("radium", "mmkv", "emu_pkg_appid_dict"), "runtime-cache"},
		{filepath.Join("radium", "mmkv", "xweb_config_storage"), "runtime-cache"},
		{filepath.Join("radium", "mmkv", "xweb_global_storage"), "runtime-cache"},
	} {
		if err := appendExistingTarget(&targets, root, filepath.Join(root, item.relative), item.reason); err != nil {
			return nil, err
		}
	}

	for _, userDir := range childDirectories(filepath.Join(root, "radium", "users")) {
		for _, name := range []string{"applet", "xworker"} {
			if err := appendExistingTarget(&targets, root, filepath.Join(userDir, name), "user-runtime"); err != nil {
				return nil, err
			}
		}
	}
	for _, profileDir := range childDirectories(filepath.Join(root, "radium", "web", "profiles")) {
		name := strings.ToLower(filepath.Base(profileDir))
		if name == "game" || strings.HasPrefix(name, "game_") || name == "webview" || strings.HasPrefix(name, "webview_") {
			if err := appendExistingTarget(&targets, root, profileDir, "web-profile"); err != nil {
				return nil, err
			}
		}
	}
	sortTargets(targets)
	return targets, nil
}

func qqCandidates(appData, localAppData string) []Target {
	groups := qqCandidateGroups(appData, localAppData)
	return flattenGroups(groups)
}

func qqCandidateGroups(appData, localAppData string) map[string][]Target {
	groups := make(map[string][]Target)
	for _, base := range uniquePaths([]string{appData, localAppData}) {
		if base == "" {
			continue
		}
		for _, appName := range []string{"QQ", "QQEX"} {
			for _, name := range []string{"miniapp_src", "miniapp_pkgs"} {
				path := filepath.Join(base, appName, "miniapp", "temps", name)
				if _, err := os.Stat(path); err == nil {
					groups[base] = append(groups[base], targetFor(base, path, "miniapp-cache"))
				}
			}
		}
	}
	return groups
}

func yybRoots() []string {
	bases := uniquePaths([]string{
		os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA"), os.Getenv("PROGRAMDATA"),
		os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"),
	})
	roots := make([]string, 0, len(bases)*2)
	for _, base := range bases {
		for _, relative := range []string{filepath.Join("Tencent", "Androws"), "Androws"} {
			candidate := filepath.Join(base, relative)
			if info, err := os.Stat(candidate); err == nil && info.IsDir() && isAndrowsRoot(candidate) {
				roots = append(roots, candidate)
			}
		}
	}
	return uniquePaths(roots)
}

func isAndrowsRoot(path string) bool {
	normalized := strings.Trim(strings.ReplaceAll(path, "/", "\\"), "\\")
	parts := strings.Split(strings.ToLower(normalized), "\\")
	return len(parts) >= 1 && parts[len(parts)-1] == "androws"
}

func yybCandidateGroups(roots []string) map[string][]Target {
	groups := make(map[string][]Target)
	for _, root := range roots {
		if !isAndrowsRoot(root) {
			continue
		}
		for _, directory := range append([]string{root}, childDirectories(root)...) {
			for _, relative := range []string{
				"WmpfRuntime", "Data", "User Data", "Cache", "Temp", "logs",
				filepath.Join("Application", "Cache"), filepath.Join("Application", "User Data"), filepath.Join("Application", "Temp"),
			} {
				path := filepath.Join(directory, relative)
				if _, err := os.Stat(path); err == nil {
					groups[root] = append(groups[root], targetFor(root, path, "androws-runtime"))
				}
			}
		}
	}
	return groups
}

func appendExistingTarget(targets *[]Target, root, path, reason string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("inspect cache path %s: %w", path, err)
	}
	if !isInsideRoot(root, path) || hasForbiddenWeChatSegment(path) {
		return nil
	}
	*targets = append(*targets, targetFor(root, path, reason))
	return nil
}

func targetFor(root, path, reason string) Target {
	relative, _ := filepath.Rel(root, path)
	return Target{Path: path, RelativePath: relative, Reason: reason}
}

func hasForbiddenWeChatSegment(path string) bool {
	forbidden := map[string]bool{"wechat files": true, "filestorage": true, "msg": true, "image": true, "video": true, "voice": true, "audio": true, "attachment": true, "backup": true, "backupfiles": true}
	for _, segment := range strings.FieldsFunc(strings.ToLower(path), func(r rune) bool { return r == '\\' || r == '/' }) {
		if forbidden[segment] {
			return true
		}
	}
	return false
}

func moveTargets(root, backupDir string, targets []Target) (int, int, error) {
	moved, skipped := 0, 0
	for _, target := range targets {
		if !isInsideRoot(root, target.Path) {
			return moved, skipped, fmt.Errorf("refusing cache path outside approved root: %s", target.Path)
		}
		if _, err := os.Stat(target.Path); err != nil {
			if os.IsNotExist(err) {
				skipped++
				continue
			}
			return moved, skipped, fmt.Errorf("inspect cache path %s: %w", target.Path, err)
		}
		relative, err := filepath.Rel(root, target.Path)
		if err != nil || relative == "." || strings.HasPrefix(relative, "..") || filepath.IsAbs(relative) {
			return moved, skipped, fmt.Errorf("refusing cache path outside approved root: %s", target.Path)
		}
		destination := uniqueDestination(filepath.Join(backupDir, relative))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return moved, skipped, fmt.Errorf("create backup directory: %w", err)
		}
		if err := os.Rename(target.Path, destination); err != nil {
			return moved, skipped, fmt.Errorf("move cache %s to backup: %w", target.Path, err)
		}
		moved++
	}
	return moved, skipped, nil
}

func isInsideRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != "." && !strings.HasPrefix(relative, "..") && !filepath.IsAbs(relative)
}

func createBackupDir(platform string) (string, error) {
	directory, err := plannedBackupDir(platform)
	if err != nil {
		return "", err
	}
	return createBackupDirAt(directory)
}

func plannedBackupDir(platform string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate user home: %w", err)
	}
	return filepath.Join(home, "Desktop", fmt.Sprintf("Farm_Go-%s-cache-backup-%s", platform, time.Now().Format("20060102-150405"))), nil
}

func createBackupDirAt(directory string) (string, error) {
	for index := 1; ; index++ {
		candidate := directory
		if index > 1 {
			candidate = fmt.Sprintf("%s-%d", directory, index)
		}
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			if err := os.MkdirAll(candidate, 0o755); err != nil {
				return "", fmt.Errorf("create backup directory %s: %w", candidate, err)
			}
			return candidate, nil
		}
	}
}

func stopByImageNames(names []string) int {
	if runtime.GOOS != "windows" {
		return 0
	}
	closed := 0
	for _, name := range names {
		if exec.Command("taskkill", "/IM", name, "/T", "/F").Run() == nil {
			closed++
		}
	}
	return closed
}

func stopYYBProcesses() int {
	if runtime.GOOS != "windows" {
		return 0
	}
	command := "$count=0; Get-Process -Name AndrowsLauncher,AndrowsStore,WeChatAppEx,QQMiniApp -ErrorAction SilentlyContinue | Where-Object { $_.Path -match '(?i)\\\\(?:Tencent\\\\)?Androws(?:\\\\|$)' } | ForEach-Object { try { Stop-Process -Id $_.Id -Force -ErrorAction Stop; $count++ } catch {} }; Write-Output $count"
	output, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", command).Output()
	if err != nil {
		return 0
	}
	count := 0
	_, _ = fmt.Sscan(strings.TrimSpace(string(output)), &count)
	return count
}

func childDirectories(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	directories := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, filepath.Join(root, entry.Name()))
		}
	}
	return directories
}

func flattenGroups(groups map[string][]Target) []Target {
	targets := make([]Target, 0)
	for _, values := range groups {
		targets = append(targets, values...)
	}
	sortTargets(targets)
	return targets
}

func sortTargets(targets []Target) {
	sort.Slice(targets, func(i, j int) bool { return targets[i].Path < targets[j].Path })
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]bool)
	unique := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		key := strings.ToLower(filepath.Clean(path))
		if !seen[key] {
			seen[key] = true
			unique = append(unique, path)
		}
	}
	return unique
}

func uniqueDestination(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	for index := 2; ; index++ {
		candidate := fmt.Sprintf("%s-%d", path, index)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
