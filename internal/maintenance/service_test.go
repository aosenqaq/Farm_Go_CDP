package maintenance

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreviewWeChatIncludesOnlyRuntimeCacheTargets(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "radium", "cache"))
	mustMkdirAll(t, filepath.Join(root, "WeChat Files", "msg"))

	summary, err := previewWeChat(root, "")
	if err != nil {
		t.Fatalf("preview WeChat: %v", err)
	}
	if summary.TargetCount != 1 {
		t.Fatalf("target count = %d, want 1: %#v", summary.TargetCount, summary.Targets)
	}
	if got := summary.Targets[0].RelativePath; got != filepath.Join("radium", "cache") {
		t.Fatalf("relative path = %q", got)
	}
	if summary.BackupDir == "" {
		t.Fatal("preview should show the planned backup directory")
	}
}

func TestMoveTargetsPreservesBackupAndRejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "QQ", "miniapp", "temps", "miniapp_src")
	mustMkdirAll(t, target)
	backup := filepath.Join(t.TempDir(), "backup")

	moved, skipped, err := moveTargets(root, backup, []Target{{Path: target, RelativePath: filepath.Join("QQ", "miniapp", "temps", "miniapp_src")}})
	if err != nil {
		t.Fatalf("move target: %v", err)
	}
	if moved != 1 || skipped != 0 {
		t.Fatalf("moved=%d skipped=%d, want 1,0", moved, skipped)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backup, "QQ", "miniapp", "temps", "miniapp_src")); err != nil {
		t.Fatalf("backup target missing: %v", err)
	}

	outside := filepath.Join(t.TempDir(), "outside")
	mustMkdirAll(t, outside)
	_, _, err = moveTargets(root, backup, []Target{{Path: outside, RelativePath: "outside"}})
	if err == nil {
		t.Fatal("expected outside-root target error")
	}
}

func TestQQCandidatesExcludeLegacyTauriData(t *testing.T) {
	candidates := qqCandidates(`C:\Users\me\AppData\Roaming`, `C:\Users\me\AppData\Local`)
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(candidate.Path), "site.dank1ng.farm") {
			t.Fatalf("legacy path leaked into candidate list: %s", candidate.Path)
		}
	}
}

func TestIsAndrowsRootRejectsUnrelatedDirectory(t *testing.T) {
	if isAndrowsRoot(`C:\Users\me\AppData\Roaming\Tencent\xwechat`) {
		t.Fatal("xwechat must not qualify as an Androws root")
	}
	if !isAndrowsRoot(`C:\Program Files\Tencent\Androws`) {
		t.Fatal("Tencent Androws root should qualify")
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}
