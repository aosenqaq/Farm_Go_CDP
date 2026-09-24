package qqpatch

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	MarkerStart       = "// >>> FARM_GO_QQ_DEBUG START >>>"
	MarkerEnd         = "// <<< FARM_GO_QQ_DEBUG END <<<"
	legacyMarkerStart = "// >>> QQ_FARM_AUTOMATION START >>>"
	legacyMarkerEnd   = "// <<< QQ_FARM_AUTOMATION END <<<"
	cocosAssetsGameJS = "cocos-js/assets/game.js"
)

var managedPatchBlockPattern = regexp.MustCompile(`(?ms)^// >>> [A-Z0-9_]+_QQ_DEBUG START >>>\r?\n.*?^// <<< [A-Z0-9_]+_QQ_DEBUG END <<<`)

type Options struct {
	TargetPath     string
	MiniappSrcRoot string
	ButtonScript   string
	HostScript     string
	HostVersion    string
	NoBackup       bool
}

type Result struct {
	OK              bool     `json:"ok"`
	Action          string   `json:"action"`
	TargetPath      string   `json:"targetPath,omitempty"`
	TargetPaths     []string `json:"targetPaths,omitempty"`
	BackupPath      string   `json:"backupPath,omitempty"`
	BackupPaths     []string `json:"backupPaths,omitempty"`
	ScriptHash      string   `json:"scriptHash,omitempty"`
	HostVersion     string   `json:"hostVersion,omitempty"`
	CandidatePaths  []string `json:"candidatePaths,omitempty"`
	RestartRequired bool     `json:"restartRequired"`
	Error           string   `json:"error,omitempty"`
}

type candidate struct {
	targetPath string
	mtime      time.Time
}

func Install(opts Options) Result {
	hostScript := strings.TrimSpace(opts.HostScript)
	if hostScript == "" {
		return Result{OK: false, Action: "failed", Error: "qq host script is required"}
	}
	hostVersion := strings.TrimSpace(opts.HostVersion)
	if hostVersion == "" {
		hostVersion = "farm-go-host-1"
	}

	targetPath, targetPaths, candidates, err := resolveTargetPaths(opts)
	if err != nil {
		return Result{
			OK:             false,
			Action:         "failed",
			CandidatePaths: candidates,
			Error:          err.Error(),
			HostVersion:    hostVersion,
		}
	}

	bundle, scriptHash := buildBundle(hostScript, hostVersion, opts.ButtonScript)
	needsPatch := pendingPatchTargets(targetPaths, scriptHash)
	if len(needsPatch) == 0 {
		return Result{
			OK:              true,
			Action:          "already_latest",
			TargetPath:      targetPath,
			TargetPaths:     targetPaths,
			ScriptHash:      scriptHash,
			HostVersion:     hostVersion,
			CandidatePaths:  candidates,
			RestartRequired: false,
		}
	}
	backupPaths, replaced, err := patchTargets(needsPatch, bundle, opts.NoBackup)
	if err != nil {
		return Result{
			OK:             false,
			Action:         "failed",
			TargetPath:     targetPath,
			TargetPaths:    targetPaths,
			CandidatePaths: candidates,
			Error:          err.Error(),
			HostVersion:    hostVersion,
			ScriptHash:     scriptHash,
		}
	}

	action := "patched"
	if replaced {
		action = "replaced"
	}
	return Result{
		OK:              true,
		Action:          action,
		TargetPath:      targetPath,
		TargetPaths:     targetPaths,
		BackupPath:      firstString(backupPaths),
		BackupPaths:     backupPaths,
		ScriptHash:      scriptHash,
		HostVersion:     hostVersion,
		CandidatePaths:  candidates,
		RestartRequired: true,
	}
}

func resolveTargetPath(opts Options) (string, []string, error) {
	targetPath, _, candidates, err := resolveTargetPaths(opts)
	return targetPath, candidates, err
}

func resolveTargetPaths(opts Options) (string, []string, []string, error) {
	if strings.TrimSpace(opts.TargetPath) != "" {
		targetPath := filepath.Clean(opts.TargetPath)
		stat, err := os.Stat(targetPath)
		if err != nil {
			return "", nil, nil, fmt.Errorf("目标路径不存在: %s", targetPath)
		}
		if stat.IsDir() {
			targetPath = filepath.Join(targetPath, "game.js")
		}
		if _, err := os.Stat(targetPath); err != nil {
			return "", nil, nil, fmt.Errorf("目标 game.js 不存在: %s", targetPath)
		}
		patchTarget := preferredPatchTarget(targetPath)
		return patchTarget, []string{patchTarget}, []string{patchTarget}, nil
	}

	roots := defaultSrcRoots()
	if strings.TrimSpace(opts.MiniappSrcRoot) != "" {
		roots = []string{filepath.Clean(opts.MiniappSrcRoot)}
	}
	candidates := discoverCandidates(roots)
	candidatePaths := make([]string, 0, len(candidates))
	for _, item := range candidates {
		candidatePaths = append(candidatePaths, item.targetPath)
	}
	if len(candidates) == 0 {
		return "", nil, candidatePaths, fmt.Errorf("未找到 QQ miniapp_src 下的 game.js")
	}
	return candidates[0].targetPath, candidatePaths, candidatePaths, nil
}

func preferredPatchTarget(targetPath string) string {
	targetPath = filepath.Clean(targetPath)
	root := filepath.Dir(targetPath)
	if filepath.Base(filepath.Dir(targetPath)) == "assets" && filepath.Base(filepath.Dir(filepath.Dir(targetPath))) == "cocos-js" {
		root = filepath.Dir(filepath.Dir(filepath.Dir(targetPath)))
	}
	cocosAssetsPath := filepath.Join(root, filepath.FromSlash(cocosAssetsGameJS))
	if info, err := os.Stat(cocosAssetsPath); err == nil && !info.IsDir() {
		return cocosAssetsPath
	}
	return filepath.Join(root, "game.js")
}

func defaultSrcRoots() []string {
	roots := make([]string, 0, 4)
	for _, base := range []string{os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA")} {
		if strings.TrimSpace(base) == "" {
			continue
		}
		roots = append(roots,
			filepath.Join(base, "QQ", "miniapp", "temps", "miniapp_src"),
			filepath.Join(base, "QQEX", "miniapp", "temps", "miniapp_src"),
		)
		roots = append(roots, findMiniappSrcRootsUnder(base, 5)...)
	}
	return dedupeStrings(roots)
}

func discoverCandidates(roots []string) []candidate {
	out := make([]candidate, 0)
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || !strings.Contains(entry.Name(), "_") {
				continue
			}
			dirPath := filepath.Join(root, entry.Name())
			gameJSPath := filepath.Join(dirPath, "game.js")
			gameJSONPath := filepath.Join(dirPath, "game.json")
			gameJSInfo, err := os.Stat(gameJSPath)
			if err != nil {
				continue
			}
			if _, err := os.Stat(gameJSONPath); err != nil {
				continue
			}
			dirInfo, _ := os.Stat(dirPath)
			mtime := gameJSInfo.ModTime()
			if dirInfo != nil && dirInfo.ModTime().After(mtime) {
				mtime = dirInfo.ModTime()
			}
			out = append(out, candidate{targetPath: preferredPatchTarget(gameJSPath), mtime: mtime})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].mtime.After(out[j].mtime)
	})
	return out
}

func buildBundle(hostScript string, hostVersion string, buttonScript string) (string, string) {
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	hash := sha1.Sum([]byte(hostVersion + "\n" + buttonScript + "\n" + hostScript))
	scriptHash := hex.EncodeToString(hash[:])[:16]
	lines := []string{
		MarkerStart,
		"// generatedAt=" + generatedAt,
		"// hostVersion=" + hostVersion,
		"// scriptHash=" + scriptHash,
		";(function () {",
		"  var root = typeof globalThis !== \"undefined\" ? globalThis : Function(\"return this\")();",
		"  root.__farmGoQqDebugMeta = { hostVersion: " + quoteJS(hostVersion) + ", scriptHash: " + quoteJS(scriptHash) + ", generatedAt: " + quoteJS(generatedAt) + " };",
		"  root.__qqFarmBundleMeta = root.__farmGoQqDebugMeta;",
		"})();",
		MarkerEnd,
		"",
	}
	if strings.TrimSpace(buttonScript) != "" {
		lines = append(lines[:len(lines)-2], buildButtonLayerBundle(buttonScript), hostScript, MarkerEnd, "")
	} else {
		lines = append(lines[:len(lines)-2], hostScript, MarkerEnd, "")
	}
	return strings.Join(lines, "\n"), scriptHash
}

func buildButtonLayerBundle(buttonScript string) string {
	indented := indentScript(strings.TrimRight(buttonScript, "\r\n"), "      ")
	return strings.Join([]string{
		";(function () {",
		"  var root = typeof globalThis !== \"undefined\" ? globalThis : Function(\"return this\")();",
		"  var meta = root.__farmGoQqDebugMeta || root.__qqFarmBundleMeta || {};",
		"  function getGameCtl() {",
		"    return root.gameCtl || (root.GameGlobal && root.GameGlobal.gameCtl);",
		"  }",
		"  function attachScriptHash() {",
		"    var ctl = getGameCtl();",
		"    if (!ctl || typeof ctl !== \"object\") return false;",
		"    ctl.__scriptHash = meta.scriptHash || \"farm-go-debug-link\";",
		"    ctl.__sourceRelPath = \"button.js\";",
		"    return true;",
		"  }",
		"  function isCurrentButtonLayerInstalled() {",
		"    var ctl = getGameCtl();",
		"    return !!(ctl && typeof ctl === \"object\" && ctl.__scriptHash === meta.scriptHash);",
		"  }",
		"  function installButtonLayer() {",
		"    if (isCurrentButtonLayerInstalled()) {",
		"      attachScriptHash();",
		"      return true;",
		"    }",
		"    try {",
		indented,
		"      attachScriptHash();",
		"      return true;",
		"    } catch (error) {",
		"      try {",
		"        console.log(\"[farm-go][qq-bundle][warn] button.js install deferred\", error && error.message ? error.message : String(error));",
		"      } catch (_) {}",
		"      return false;",
		"    }",
		"  }",
		"  function ensureButtonLayer() {",
		"    var attempts = 0;",
		"    var maxAttempts = 120;",
		"    function tick() {",
		"      if (installButtonLayer()) return;",
		"      attempts += 1;",
		"      if (attempts >= maxAttempts) return;",
		"      setTimeout(tick, 500);",
		"    }",
		"    tick();",
		"  }",
		"  ensureButtonLayer();",
		"})();",
	}, "\n")
}

func indentScript(script string, prefix string) string {
	lines := strings.Split(script, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func hasLatestPatch(targetPath string, scriptHash string) bool {
	content, err := os.ReadFile(targetPath)
	if err != nil {
		return false
	}
	blocks := managedPatchBlocks(string(content))
	return len(blocks) == 1 && strings.Contains(blocks[0], "// scriptHash="+scriptHash)
}

func pendingPatchTargets(targetPaths []string, scriptHash string) []string {
	pending := make([]string, 0, len(targetPaths))
	for _, targetPath := range dedupeStrings(targetPaths) {
		if !hasLatestPatch(targetPath, scriptHash) {
			pending = append(pending, targetPath)
		}
	}
	return pending
}

func patchTargets(targetPaths []string, bundle string, noBackup bool) ([]string, bool, error) {
	backupPaths := make([]string, 0, len(targetPaths))
	replacedAny := false
	for _, targetPath := range dedupeStrings(targetPaths) {
		backupPath, replaced, err := patchGameJS(targetPath, bundle, noBackup)
		if err != nil {
			return backupPaths, replacedAny, err
		}
		if backupPath != "" {
			backupPaths = append(backupPaths, backupPath)
		}
		replacedAny = replacedAny || replaced
	}
	return backupPaths, replacedAny, nil
}

func patchGameJS(targetPath string, bundle string, noBackup bool) (string, bool, error) {
	originalBytes, err := os.ReadFile(targetPath)
	if err != nil {
		return "", false, err
	}
	original := string(originalBytes)
	original, _ = stripPatchBlocks(original, legacyMarkerStart, legacyMarkerEnd)
	original, replaced := stripManagedPatchBlocks(original)
	next := strings.TrimRight(original, "\r\n") + "\n\n" + strings.TrimRight(bundle, "\r\n") + "\n"

	backupPath := ""
	if !noBackup {
		backupPath = targetPath + ".farm-go.bak"
		if _, err := os.Stat(backupPath); err != nil {
			if err := os.WriteFile(backupPath, originalBytes, 0o644); err != nil {
				return "", replaced, err
			}
		}
	}
	if err := os.WriteFile(targetPath, []byte(next), 0o644); err != nil {
		return backupPath, replaced, err
	}
	return backupPath, replaced, nil
}

func stripPatchBlocks(text string, markerStart string, markerEnd string) (string, bool) {
	changed := false
	for {
		start := strings.Index(text, markerStart)
		if start < 0 {
			return text, changed
		}
		endRelative := strings.Index(text[start:], markerEnd)
		if endRelative < 0 {
			return text, changed
		}
		end := start + endRelative + len(markerEnd)
		left := strings.TrimRight(text[:start], "\r\n")
		right := strings.TrimLeft(text[end:], "\r\n")
		if left != "" && right != "" {
			text = left + "\n\n" + right
		} else {
			text = left + right
		}
		changed = true
	}
}

func managedPatchBlocks(text string) []string {
	return managedPatchBlockPattern.FindAllString(text, -1)
}

func stripManagedPatchBlocks(text string) (string, bool) {
	if !managedPatchBlockPattern.MatchString(text) {
		return text, false
	}
	return managedPatchBlockPattern.ReplaceAllString(text, ""), true
}

func quoteJS(value string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`) + `"`
}

func findMiniappSrcRootsUnder(root string, maxDepth int) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	type item struct {
		path  string
		depth int
	}
	stack := []item{{path: root}}
	out := make([]string, 0)
	seen := map[string]bool{}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		clean := filepath.Clean(current.path)
		key := strings.ToLower(clean)
		if seen[key] {
			continue
		}
		seen[key] = true
		entries, err := os.ReadDir(clean)
		if err != nil {
			continue
		}
		if filepath.Base(clean) == "miniapp_src" && len(discoverCandidates([]string{clean})) > 0 {
			out = append(out, clean)
			continue
		}
		if current.depth >= maxDepth {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := strings.ToLower(entry.Name())
			child := filepath.Join(clean, entry.Name())
			if current.depth > 0 || strings.Contains(name, "qq") || strings.Contains(name, "tencent") || name == "miniapp" || name == "temps" {
				stack = append(stack, item{path: child, depth: current.depth + 1})
			}
		}
	}
	return out
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		clean := filepath.Clean(strings.TrimSpace(value))
		if clean == "." || clean == "" {
			continue
		}
		key := strings.ToLower(clean)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, clean)
	}
	return out
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
