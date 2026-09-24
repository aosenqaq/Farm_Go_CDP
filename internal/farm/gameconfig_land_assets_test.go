package farm

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildRuntimeLandDetailsResolvesLocalPlantStageImage(t *testing.T) {
	root := writeLandAssetConfig(t)
	writeTinyPNG(t, filepath.Join(root, "plant_images", "stages", "作物", "白萝卜", "白萝卜_02_发芽.png"))

	payload := BuildRuntimeLandDetailsForRoot(map[string]any{
		"grids": []any{
			map[string]any{
				"landId":       float64(1),
				"plantName":    "白萝卜",
				"seedId":       float64(20002),
				"stageKind":    "growing",
				"currentStage": float64(2),
				"phaseName":    "发芽",
				"imageUrl":     "data:image/png;base64,blocked-on-lan",
			},
		},
	}, root)

	if len(payload.Lands) != 1 {
		t.Fatalf("expected one land, got %#v", payload.Lands)
	}
	assertLocalLandImage(t, payload.Lands[0].ImageURL)
}

func TestBuildRuntimeLandDetailsUsesImportedActivityCropFallbacks(t *testing.T) {
	root := writeLandAssetConfig(t)
	genericSeedPath := filepath.Join(root, "plant_images", "stages", "作物", "白萝卜", "白萝卜_01_种子.png")
	activityMainPath := filepath.Join(root, "plant_images", "stages", "活动果实", "星语铃花", "星语铃花_00_主图.png")
	writeTinyPNG(t, genericSeedPath)
	writeTinyPNG(t, activityMainPath)
	if err := os.MkdirAll(filepath.Join(root, "plant_images", "stages", "_mappings"), 0o755); err != nil {
		t.Fatalf("mkdir activity mapping: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "plant_images", "stages", "_mappings", "activity_crops.json"), []byte(`{"items":[{"crop_id":1029003,"seed_id":29003,"fruit_id":49003,"name":"星语铃花","image_category":"活动果实"}]}`), 0o644); err != nil {
		t.Fatalf("write activity mapping: %v", err)
	}

	firstPhase := BuildRuntimeLandDetailsForRoot(map[string]any{
		"grids": []any{map[string]any{
			"landId": float64(9), "plantName": "星语铃花", "seedId": float64(29003),
			"stageKind": "growing", "currentStage": float64(1), "phaseName": "种子",
		}},
	}, root)
	if len(firstPhase.Lands) != 1 {
		t.Fatalf("expected first phase land, got %#v", firstPhase.Lands)
	}
	if got, want := firstPhase.Lands[0].ImageURL, gameConfigLocalImageURL(root, genericSeedPath); got != want {
		t.Fatalf("first phase image = %q, want generic seed image %q", got, want)
	}

	missingPhase := BuildRuntimeLandDetailsForRoot(map[string]any{
		"grids": []any{map[string]any{
			"landId": float64(10), "plantName": "星语铃花", "seedId": float64(29003),
			"stageKind": "growing", "currentStage": float64(4), "phaseName": "大叶子",
		}},
	}, root)
	if len(missingPhase.Lands) != 1 {
		t.Fatalf("expected missing phase land, got %#v", missingPhase.Lands)
	}
	if got, want := missingPhase.Lands[0].ImageURL, gameConfigLocalImageURL(root, activityMainPath); got != want {
		t.Fatalf("missing phase image = %q, want activity main image %q", got, want)
	}
}

func TestBuildRuntimeLandDetailsUsesDisplayNameForActivityCropStage(t *testing.T) {
	root := writeLandAssetConfig(t)
	stagePath := filepath.Join(root, "plant_images", "stages", "活动果实", "紫薇", "紫薇_06_成熟.png")
	writeTinyPNG(t, stagePath)
	if err := os.MkdirAll(filepath.Join(root, "plant_images", "stages", "_mappings"), 0o755); err != nil {
		t.Fatalf("mkdir activity mapping: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "plant_images", "stages", "_mappings", "activity_crops.json"), []byte(`{"items":[{"crop_id":1021353,"seed_id":21353,"fruit_id":41353,"name":"紫薇","image_category":"活动果实"}]}`), 0o644); err != nil {
		t.Fatalf("write activity mapping: %v", err)
	}

	payload := BuildRuntimeLandDetailsForRoot(map[string]any{
		"grids": []any{map[string]any{
			"landId":           float64(11),
			"displayPlantName": "紫薇",
			"stageKind":        "mature",
			"currentStage":     float64(6),
			"phaseName":        "成熟",
		}},
	}, root)

	if len(payload.Lands) != 1 {
		t.Fatalf("expected one activity land, got %#v", payload.Lands)
	}
	if got, want := payload.Lands[0].ImageURL, gameConfigLocalImageURL(root, stagePath); got != want {
		t.Fatalf("activity stage image = %q, want %q", got, want)
	}
}

func TestBuildRuntimeLandDetailsResolvesDreamButterflyMutation(t *testing.T) {
	root := writeLandAssetConfig(t)
	writeTinyPNG(t, filepath.Join(root, "plant_images", "stages", "变异", "变异宝典", "黄金", "黄金_00_变异图标.png"))
	writeTinyPNG(t, filepath.Join(root, "plant_images", "stages", "变异", "变异宝典", "梦蝶", "梦蝶_00_变异图标.png"))
	writeTinyPNG(t, filepath.Join(root, "plant_images", "stages", "活动果实", "蝶梦星铃", "黄金·蝶梦星铃_00_主图.png"))
	if err := os.MkdirAll(filepath.Join(root, "plant_images", "stages", "_mappings"), 0o755); err != nil {
		t.Fatalf("mkdir activity mapping: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "plant_images", "stages", "_mappings", "activity_crops.json"), []byte(`{"items":[{"crop_id":1028003,"fruit_id":204006,"name":"蝶梦星铃","image_category":"活动果实"}]}`), 0o644); err != nil {
		t.Fatalf("write activity mapping: %v", err)
	}

	payload := BuildRuntimeLandDetailsForRoot(map[string]any{
		"grids": []any{map[string]any{
			"landId":       float64(18),
			"plantName":    "蝶梦星铃",
			"stageKind":    "growing",
			"currentStage": float64(3),
			"mutation": map[string]any{
				"hasMutation":       true,
				"activeMutantTypes": []any{float64(5), float64(11)},
			},
		}},
	}, root)

	land := payload.Lands[0]
	if land.MutationLabel != "黄金、梦蝶" {
		t.Fatalf("unexpected mutation label %q", land.MutationLabel)
	}
	if land.DisplayPlantName != "黄金·蝶梦星铃" {
		t.Fatalf("unexpected mutation output %q", land.DisplayPlantName)
	}
	assertLocalLandImage(t, land.MutationIconURL)
	assertLocalLandImage(t, land.MutationImageURL)
	if land.ImageURL != land.MutationImageURL {
		t.Fatalf("expected land image to prefer dream butterfly mutation image, got %#v", land)
	}
}

func TestBuildRuntimeLandDetailsExposesMutationTypeAndImage(t *testing.T) {
	root := writeLandAssetConfig(t)
	writeTinyPNG(t, filepath.Join(root, "plant_images", "stages", "作物", "白萝卜", "白萝卜_03_成熟.png"))
	writeTinyPNG(t, filepath.Join(root, "plant_images", "stages", "变异", "变异宝典", "黄金", "黄金_00_变异图标.png"))
	writeTinyPNG(t, filepath.Join(root, "plant_images", "stages", "变异", "超变图鉴", "黄金果实", "黄金·白萝卜", "黄金·白萝卜_00_主图.png"))

	payload := BuildRuntimeLandDetailsForRoot(map[string]any{
		"grids": []any{
			map[string]any{
				"landId":       float64(15),
				"plantName":    "白萝卜",
				"seedId":       float64(20002),
				"stageKind":    "growing",
				"currentStage": float64(3),
				"mutation": map[string]any{
					"hasMutation":       true,
					"activeMutantTypes": []any{float64(5)},
					"typeItems": []any{
						map[string]any{"name": "黄金", "iconUrl": "data:image/png;base64,blocked-on-lan"},
					},
				},
			},
		},
	}, root)

	land := payload.Lands[0]
	if !land.HasMutation || land.MutationLabel != "黄金" {
		t.Fatalf("expected golden mutation fields, got %#v", land)
	}
	assertLocalLandImage(t, land.MutationIconURL)
	assertLocalLandImage(t, land.MutationImageURL)
	if land.ImageURL != land.MutationImageURL {
		t.Fatalf("expected card image to prefer mutation image, got image=%q mutation=%q", land.ImageURL, land.MutationImageURL)
	}
}

func TestBuildRuntimeLandDetailsUsesActivityMutationPlantFromRuntimeEffectMap(t *testing.T) {
	root := writeLandAssetConfig(t)
	writeTinyPNG(t, filepath.Join(root, "plant_images", "stages", "作物", "白萝卜", "白萝卜_03_成熟.png"))
	writeTinyPNG(t, filepath.Join(root, "plant_images", "stages", "变异", "变异宝典", "绵绵", "miamian.png"))
	writeTinyPNG(t, filepath.Join(root, "plant_images", "stages", "变异", "超变图鉴", "活动果实", "绵绵糖果", "绵绵糖果_00_主图.png"))

	payload := BuildRuntimeLandDetailsForRoot(map[string]any{
		"grids": []any{
			map[string]any{
				"landId":       float64(16),
				"plantName":    "白萝卜",
				"seedId":       float64(20002),
				"stageKind":    "growing",
				"currentStage": float64(3),
				"mutation": map[string]any{
					"hasMutation":       true,
					"activeMutantTypes": []any{float64(10)},
				},
			},
		},
	}, root)

	land := payload.Lands[0]
	if !land.HasMutation || land.MutationLabel != "绵绵" {
		t.Fatalf("expected activity mutation fields, got %#v", land)
	}
	if land.DisplayPlantName != "绵绵糖果" {
		t.Fatalf("expected activity mutation display name, got %q", land.DisplayPlantName)
	}
	assertLocalLandImage(t, land.MutationImageURL)
	if land.ImageURL != land.MutationImageURL {
		t.Fatalf("expected card image to prefer activity mutation image, got image=%q mutation=%q", land.ImageURL, land.MutationImageURL)
	}
}

func TestBuildRuntimeLandDetailsUsesActivityMutationPlantFromTypeOutput(t *testing.T) {
	root := writeLandAssetConfig(t)
	writeTinyPNG(t, filepath.Join(root, "plant_images", "stages", "作物", "白萝卜", "白萝卜_03_成熟.png"))
	writeTinyPNG(t, filepath.Join(root, "plant_images", "stages", "变异", "超变图鉴", "活动果实", "荷花", "荷花_00_主图.png"))

	payload := BuildRuntimeLandDetailsForRoot(map[string]any{
		"grids": []any{
			map[string]any{
				"landId":       float64(17),
				"plantName":    "白萝卜",
				"seedId":       float64(20002),
				"stageKind":    "growing",
				"currentStage": float64(3),
				"mutation": map[string]any{
					"hasMutation":       true,
					"activeMutantTypes": []any{float64(8)},
				},
			},
		},
	}, root)

	land := payload.Lands[0]
	if land.DisplayPlantName != "荷花" {
		t.Fatalf("expected mutation type output name, got %q", land.DisplayPlantName)
	}
	assertLocalLandImage(t, land.MutationImageURL)
}

func writeLandAssetConfig(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	plantJSON := `[{"id":1020002,"name":"白萝卜","fruit":{"id":40002,"count":5},"seed_id":20002,"land_level_need":1,"seasons":1,"grow_phases":"种子:10;发芽:10;成熟:0;","exp":1,"size":1,"mutant_effect_plant":"10:1020804;"},{"id":1029003,"name":"星语铃花","fruit":{"id":49003,"count":192},"seed_id":29003,"land_level_need":1,"seasons":1,"grow_phases":"种子:10;发芽:10;小叶子:10;大叶子:10;开花:10;成熟:0;","exp":7680,"size":2},{"id":1028003,"name":"蝶梦星铃","fruit":{"id":204006,"count":1},"land_level_need":1,"seasons":1,"grow_phases":"种子:10;发芽:10;小叶子:10;大叶子:10;开花:10;成熟:0;","size":2,"mutant_effect_plant":"5_11:1128003:1"}]`
	if err := os.WriteFile(filepath.Join(root, "Plant.json"), []byte(plantJSON), 0o644); err != nil {
		t.Fatalf("write Plant.json: %v", err)
	}
	return root
}

func writeTinyPNG(t *testing.T, target string) {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir image dir: %v", err)
	}
	if err := os.WriteFile(target, raw, 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}
}

func assertLocalLandImage(t *testing.T, url string) {
	t.Helper()
	if !strings.HasPrefix(url, localImageAssetPrefix) || strings.Contains(url, "data:image/") {
		t.Fatalf("expected local image URL, got %q", url)
	}
	response := httptest.NewRecorder()
	if !ServeLocalGameConfigImage(response, httptest.NewRequest(http.MethodGet, url, nil)) {
		t.Fatalf("expected local image route to handle %q", url)
	}
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" || response.Body.Len() == 0 {
		t.Fatalf("unexpected local image response: code=%d type=%q bytes=%d", response.Code, response.Header().Get("Content-Type"), response.Body.Len())
	}
}
