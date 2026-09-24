package qqpatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInstallPatchesCocosAssetsHostInsteadOfRootGameJS(t *testing.T) {
	root := t.TempDir()
	rootGameJS := filepath.Join(root, "game.js")
	hostGameJS := filepath.Join(root, "cocos-js", "assets", "game.js")
	if err := os.MkdirAll(filepath.Dir(hostGameJS), 0o755); err != nil {
		t.Fatalf("create Cocos assets directory: %v", err)
	}
	if err := os.WriteFile(rootGameJS, []byte("console.log('root game');\n"), 0o644); err != nil {
		t.Fatalf("write root game.js: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "game.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write game.json: %v", err)
	}
	if err := os.WriteFile(hostGameJS, []byte("console.log('wasm assets');\n"), 0o644); err != nil {
		t.Fatalf("write Cocos assets game.js: %v", err)
	}

	result := Install(Options{
		TargetPath:  rootGameJS,
		HostScript:  "root.__qqFarmHost = { status: function () {} };",
		HostVersion: "farm-go-host-1",
	})

	if !result.OK {
		t.Fatalf("expected install ok, got %#v", result)
	}
	if result.TargetPath != hostGameJS {
		t.Fatalf("expected Cocos assets host target %q, got %q", hostGameJS, result.TargetPath)
	}
	if result.BackupPath != hostGameJS+".farm-go.bak" {
		t.Fatalf("expected host backup path, got %q", result.BackupPath)
	}
	rootContent, err := os.ReadFile(rootGameJS)
	if err != nil {
		t.Fatalf("read root game.js: %v", err)
	}
	if strings.Contains(string(rootContent), MarkerStart) {
		t.Fatalf("root game.js must remain unpatched: %q", rootContent)
	}
	hostContent, err := os.ReadFile(hostGameJS)
	if err != nil {
		t.Fatalf("read Cocos assets game.js: %v", err)
	}
	if !strings.Contains(string(hostContent), MarkerStart) || !strings.Contains(string(hostContent), "root.__qqFarmHost") {
		t.Fatalf("expected host patch in Cocos assets game.js: %q", hostContent)
	}
}

func TestInstallPatchesExplicitGameJSWithHostScript(t *testing.T) {
	dir := t.TempDir()
	gameJS := filepath.Join(dir, "game.js")
	if err := os.WriteFile(gameJS, []byte("console.log('game');\n"), 0o644); err != nil {
		t.Fatalf("write game.js: %v", err)
	}

	result := Install(Options{
		TargetPath:  gameJS,
		HostScript:  "root.__qqFarmHost = { status: function () {} };",
		HostVersion: "farm-go-host-1",
	})

	if !result.OK {
		t.Fatalf("expected install ok, got %#v", result)
	}
	if result.TargetPath != gameJS {
		t.Fatalf("unexpected target path %q", result.TargetPath)
	}
	if result.ScriptHash == "" {
		t.Fatalf("expected script hash, got %#v", result)
	}
	if result.BackupPath == "" {
		t.Fatalf("expected backup path, got %#v", result)
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("expected backup file: %v", err)
	}

	content, err := os.ReadFile(gameJS)
	if err != nil {
		t.Fatalf("read patched game.js: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, MarkerStart) || !strings.Contains(text, MarkerEnd) {
		t.Fatalf("expected patch markers in %q", text)
	}
	if !strings.Contains(text, "farm-go-host-1") {
		t.Fatalf("expected host version in patch block: %q", text)
	}
	if !strings.Contains(text, "root.__qqFarmHost") {
		t.Fatalf("expected host script in patch block: %q", text)
	}
	if !result.RestartRequired {
		t.Fatalf("expected fresh patch to require miniapp restart, got %#v", result)
	}
}

func TestInstallPatchesButtonLayerBeforeHostScript(t *testing.T) {
	dir := t.TempDir()
	gameJS := filepath.Join(dir, "game.js")
	if err := os.WriteFile(gameJS, []byte("console.log('game');\n"), 0o644); err != nil {
		t.Fatalf("write game.js: %v", err)
	}

	result := Install(Options{
		TargetPath:   gameJS,
		ButtonScript: "G.gameCtl = { probe: function () { return { ok: true }; } };",
		HostScript:   "root.__qqFarmHost = { status: function () {} };",
		HostVersion:  "farm-go-host-1",
		NoBackup:     true,
	})

	if !result.OK {
		t.Fatalf("expected install ok, got %#v", result)
	}
	content, err := os.ReadFile(gameJS)
	if err != nil {
		t.Fatalf("read patched game.js: %v", err)
	}
	text := string(content)
	buttonIndex := strings.Index(text, "G.gameCtl =")
	hostIndex := strings.Index(text, "root.__qqFarmHost")
	if buttonIndex < 0 {
		t.Fatalf("expected button script in patch: %q", text)
	}
	if hostIndex < 0 {
		t.Fatalf("expected host script in patch: %q", text)
	}
	if buttonIndex > hostIndex {
		t.Fatalf("expected button layer before host script: %q", text)
	}
}

func TestInstallReplacesExistingFarmGoPatch(t *testing.T) {
	dir := t.TempDir()
	gameJS := filepath.Join(dir, "game.js")
	original := "console.log('game');\n\n" + MarkerStart + "\nold\n" + MarkerEnd + "\n"
	if err := os.WriteFile(gameJS, []byte(original), 0o644); err != nil {
		t.Fatalf("write game.js: %v", err)
	}

	result := Install(Options{
		TargetPath:  gameJS,
		HostScript:  "newHost();",
		HostVersion: "farm-go-host-2",
		NoBackup:    true,
	})

	if !result.OK {
		t.Fatalf("expected install ok, got %#v", result)
	}
	content, err := os.ReadFile(gameJS)
	if err != nil {
		t.Fatalf("read patched game.js: %v", err)
	}
	text := string(content)
	if strings.Contains(text, "\nold\n") {
		t.Fatalf("expected old patch to be replaced: %q", text)
	}
	if !strings.Contains(text, "newHost();") {
		t.Fatalf("expected new patch content: %q", text)
	}
}

func TestInstallRemovesLegacyReferencePatch(t *testing.T) {
	dir := t.TempDir()
	gameJS := filepath.Join(dir, "game.js")
	original := strings.Join([]string{
		"console.log('game');",
		"",
		legacyMarkerStart,
		"root.__qqFarmHost = { __installed: true };",
		legacyMarkerEnd,
		"",
	}, "\n")
	if err := os.WriteFile(gameJS, []byte(original), 0o644); err != nil {
		t.Fatalf("write game.js: %v", err)
	}

	result := Install(Options{
		TargetPath:  gameJS,
		HostScript:  "root.__farmGoQqHost = { status: function () {} };",
		HostVersion: "farm-go-host-1",
		NoBackup:    true,
	})

	if !result.OK {
		t.Fatalf("expected install ok, got %#v", result)
	}
	content, err := os.ReadFile(gameJS)
	if err != nil {
		t.Fatalf("read patched game.js: %v", err)
	}
	text := string(content)
	if strings.Contains(text, legacyMarkerStart) || strings.Contains(text, "root.__qqFarmHost = { __installed: true };") {
		t.Fatalf("expected legacy reference patch to be removed: %q", text)
	}
	if !strings.Contains(text, MarkerStart) || !strings.Contains(text, "root.__farmGoQqHost") {
		t.Fatalf("expected Farm_Go patch to be installed: %q", text)
	}
}

func TestInstallSkipsWhenPatchAlreadyLatest(t *testing.T) {
	dir := t.TempDir()
	gameJS := filepath.Join(dir, "game.js")
	if err := os.WriteFile(gameJS, []byte("console.log('game');\n"), 0o644); err != nil {
		t.Fatalf("write game.js: %v", err)
	}

	first := Install(Options{
		TargetPath:  gameJS,
		HostScript:  "sameHost();",
		HostVersion: "farm-go-host-1",
		NoBackup:    true,
	})
	if !first.OK {
		t.Fatalf("expected first install ok, got %#v", first)
	}

	second := Install(Options{
		TargetPath:  gameJS,
		HostScript:  "sameHost();",
		HostVersion: "farm-go-host-1",
		NoBackup:    true,
	})

	if !second.OK {
		t.Fatalf("expected second install ok, got %#v", second)
	}
	if second.Action != "already_latest" {
		t.Fatalf("expected already_latest, got %#v", second)
	}
	if second.RestartRequired {
		t.Fatalf("already latest patch should not require restart: %#v", second)
	}
	if second.ScriptHash != first.ScriptHash {
		t.Fatalf("expected same hash, got %q and %q", first.ScriptHash, second.ScriptHash)
	}
}

func TestInstallAutoDiscoversLatestGameJS(t *testing.T) {
	root := t.TempDir()
	oldDir := filepath.Join(root, "111_1_old")
	newDir := filepath.Join(root, "222_2_new")
	for _, dir := range []string{oldDir, newDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "game.js"), []byte("console.log('game');\n"), 0o644); err != nil {
			t.Fatalf("write game.js: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "game.json"), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write game.json: %v", err)
		}
	}
	if err := os.Chtimes(oldDir, oldTime(), oldTime()); err != nil {
		t.Fatalf("chtimes old dir: %v", err)
	}
	for _, name := range []string{"game.js", "game.json"} {
		if err := os.Chtimes(filepath.Join(oldDir, name), oldTime(), oldTime()); err != nil {
			t.Fatalf("chtimes old %s: %v", name, err)
		}
	}

	result := Install(Options{
		MiniappSrcRoot: root,
		HostScript:     "autoHost();",
		HostVersion:    "farm-go-host-1",
		NoBackup:       true,
	})

	if !result.OK {
		t.Fatalf("expected install ok, got %#v", result)
	}
	if result.TargetPath != filepath.Join(newDir, "game.js") {
		t.Fatalf("expected latest target, got %q", result.TargetPath)
	}
}

func TestInstallPatchesAllAutoDiscoveredCandidates(t *testing.T) {
	root := t.TempDir()
	firstDir := filepath.Join(root, "1112386029_1_first")
	secondDir := filepath.Join(root, "1112386029_2_second")
	for _, dir := range []string{firstDir, secondDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "game.js"), []byte("console.log('game');\n"), 0o644); err != nil {
			t.Fatalf("write game.js: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "game.json"), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write game.json: %v", err)
		}
	}

	result := Install(Options{
		MiniappSrcRoot: root,
		HostScript:     "autoHost();",
		HostVersion:    "farm-go-host-1",
		NoBackup:       true,
	})

	if !result.OK {
		t.Fatalf("expected install ok, got %#v", result)
	}
	for _, target := range []string{filepath.Join(firstDir, "game.js"), filepath.Join(secondDir, "game.js")} {
		content, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read %s: %v", target, err)
		}
		if !strings.Contains(string(content), MarkerStart) || !strings.Contains(string(content), "autoHost();") {
			t.Fatalf("expected %s to be patched, got %q", target, string(content))
		}
	}
	if len(result.TargetPaths) != 2 {
		t.Fatalf("expected both target paths in result, got %#v", result.TargetPaths)
	}
}

func TestInstallAutoDiscoveryPatchesCocosAssetsHosts(t *testing.T) {
	root := t.TempDir()
	firstDir := filepath.Join(root, "1112386029_1_first")
	secondDir := filepath.Join(root, "1112386029_2_second")
	hostPaths := make([]string, 0, 2)
	for _, dir := range []string{firstDir, secondDir} {
		if err := os.MkdirAll(filepath.Join(dir, "cocos-js", "assets"), 0o755); err != nil {
			t.Fatalf("create Cocos assets directory: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "game.js"), []byte("console.log('root game');\n"), 0o644); err != nil {
			t.Fatalf("write root game.js: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "game.json"), []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("write game.json: %v", err)
		}
		hostPath := filepath.Join(dir, "cocos-js", "assets", "game.js")
		if err := os.WriteFile(hostPath, []byte("console.log('wasm assets');\n"), 0o644); err != nil {
			t.Fatalf("write Cocos assets game.js: %v", err)
		}
		hostPaths = append(hostPaths, hostPath)
	}

	result := Install(Options{
		MiniappSrcRoot: root,
		HostScript:     "autoHost();",
		HostVersion:    "farm-go-host-1",
		NoBackup:       true,
	})

	if !result.OK {
		t.Fatalf("expected install ok, got %#v", result)
	}
	if len(result.TargetPaths) != len(hostPaths) {
		t.Fatalf("expected Cocos assets targets %#v, got %#v", hostPaths, result.TargetPaths)
	}
	for _, hostPath := range hostPaths {
		content, err := os.ReadFile(hostPath)
		if err != nil {
			t.Fatalf("read %s: %v", hostPath, err)
		}
		if !strings.Contains(string(content), MarkerStart) || !strings.Contains(string(content), "autoHost();") {
			t.Fatalf("expected %s to be patched, got %q", hostPath, string(content))
		}
	}
	for _, rootGamePath := range []string{filepath.Join(firstDir, "game.js"), filepath.Join(secondDir, "game.js")} {
		content, err := os.ReadFile(rootGamePath)
		if err != nil {
			t.Fatalf("read %s: %v", rootGamePath, err)
		}
		if strings.Contains(string(content), MarkerStart) {
			t.Fatalf("root game.js must remain unpatched: %q", content)
		}
	}
}

func TestInstallReplacesForeignBrandPatch(t *testing.T) {
	dir := t.TempDir()
	gameJS := filepath.Join(dir, "game.js")
	foreignBrand := "HQ_FARM"
	if strings.Contains(MarkerStart, "HQ_FARM") {
		foreignBrand = "FARM" + "_GO"
	}
	foreignStart := "// >>> " + foreignBrand + "_QQ_DEBUG START >>>"
	foreignEnd := "// <<< " + foreignBrand + "_QQ_DEBUG END <<<"
	original := strings.Join([]string{
		"console.log('game');", "", MarkerStart, "// scriptHash=old-farm", MarkerEnd, "",
		foreignStart, "// scriptHash=hq", "root.__hqFarmHost = {};", foreignEnd, "",
	}, "\n")
	if err := os.WriteFile(gameJS, []byte(original), 0o644); err != nil {
		t.Fatalf("write game.js: %v", err)
	}

	result := Install(Options{TargetPath: gameJS, HostScript: "normalHost();", HostVersion: "farm-go-host-1", NoBackup: true})
	if !result.OK || result.Action != "replaced" || !result.RestartRequired {
		t.Fatalf("expected foreign replacement, got %#v", result)
	}
	content, err := os.ReadFile(gameJS)
	if err != nil {
		t.Fatalf("read game.js: %v", err)
	}
	text := string(content)
	if strings.Contains(text, foreignStart) || strings.Contains(text, "root.__hqFarmHost") {
		t.Fatalf("foreign patch remained: %q", text)
	}
	if strings.Count(text, "_QQ_DEBUG START >>>") != 1 || !strings.Contains(text, "normalHost();") {
		t.Fatalf("expected exactly one normal patch, got %q", text)
	}
}

func TestInstallDoesNotTreatMixedBrandBlocksAsLatest(t *testing.T) {
	dir := t.TempDir()
	gameJS := filepath.Join(dir, "game.js")
	if err := os.WriteFile(gameJS, []byte("console.log('game');\n"), 0o644); err != nil {
		t.Fatalf("write game.js: %v", err)
	}
	options := Options{TargetPath: gameJS, HostScript: "sameHost();", HostVersion: "farm-go-host-1", NoBackup: true}
	if first := Install(options); !first.OK {
		t.Fatalf("first install: %#v", first)
	}
	file, err := os.OpenFile(gameJS, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open game.js: %v", err)
	}
	_, err = file.WriteString("\n// >>> HQ_FARM_QQ_DEBUG START >>>\n// scriptHash=hq\n// <<< HQ_FARM_QQ_DEBUG END <<<\n")
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("append foreign patch: %v", err)
	}

	second := Install(options)
	if !second.OK || second.Action == "already_latest" || !second.RestartRequired {
		t.Fatalf("mixed blocks must be replaced, got %#v", second)
	}
	content, err := os.ReadFile(gameJS)
	if err != nil {
		t.Fatalf("read game.js: %v", err)
	}
	if strings.Count(string(content), "_QQ_DEBUG START >>>") != 1 {
		t.Fatalf("expected one managed patch, got %q", content)
	}
}

func oldTime() time.Time {
	return time.Unix(1, 0)
}
