package farm

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestGameConfigResourcesArePresent(t *testing.T) {
	root := filepath.Join("..", "..", "resources", "gameConfig")
	for _, name := range []string{"Plant.json", "ItemInfo.json", "RoleLevel.json"} {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("expected %s in resources/gameConfig: %v", name, err)
		}
		if info.IsDir() || info.Size() == 0 {
			t.Fatalf("expected %s to be a non-empty file", name)
		}
	}
}

func TestBundledGameConfigRecognizesAllActivitySeeds(t *testing.T) {
	archive, err := zip.OpenReader(filepath.Join("..", "..", "resources", "gameConfig.bundle.zip"))
	if err != nil {
		t.Fatalf("open bundled game config: %v", err)
	}
	t.Cleanup(func() { archive.Close() })

	root := t.TempDir()
	for _, name := range []string{"Plant.json", "ItemInfo.json"} {
		copyBundledGameConfigFile(t, archive, root, name)
	}
	SetGameConfigRoot(root)
	t.Cleanup(func() { SetGameConfigRoot("") })

	wantBySeedID := map[int]string{
		20329: "发财红包", 20264: "帝王血", 20108: "铃兰", 26032: "月见草",
		21251: "紫茉莉", 21050: "萱草", 21404: "月光花", 21037: "银星海棠",
		21353: "紫薇", 21380: "梧桐", 20129: "勿忘我", 20375: "木槿",
		29003: "星语铃花",
	}
	seedList := make([]any, 0, len(wantBySeedID))
	for seedID := range wantBySeedID {
		seedList = append(seedList, map[string]any{"itemId": seedID, "count": 1})
	}
	payload := BuildBackpackSeedOptions(seedList, BackpackSeedOptionsRequest{})
	if !payload.OK || len(payload.List) != len(wantBySeedID) {
		t.Fatalf("bundled activity seed options = %#v", payload)
	}
	for _, option := range payload.List {
		wantName, ok := wantBySeedID[option.SeedID]
		if !ok || option.Name != wantName || option.BackpackCount != 1 {
			t.Fatalf("bundled activity seed option = %#v, want name %q", option, wantName)
		}
	}
}

func copyBundledGameConfigFile(t *testing.T, archive *zip.ReadCloser, root string, name string) {
	t.Helper()
	for _, entry := range archive.File {
		if entry.Name != name {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatalf("open bundled %s: %v", name, err)
		}
		defer reader.Close()
		output, err := os.Create(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("create extracted %s: %v", name, err)
		}
		if _, err := io.Copy(output, reader); err != nil {
			output.Close()
			t.Fatalf("extract %s: %v", name, err)
		}
		if err := output.Close(); err != nil {
			t.Fatalf("close extracted %s: %v", name, err)
		}
		return
	}
	t.Fatalf("bundled game config is missing %s", name)
}
