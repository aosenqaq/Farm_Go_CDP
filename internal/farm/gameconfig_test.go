package farm

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCropAnalyticsComputesConfigBackedMetrics(t *testing.T) {
	root := filepath.Join("..", "..", "resources", "gameConfig")

	payload, err := LoadCropAnalytics(root)
	if err != nil {
		t.Fatalf("load crop analytics: %v", err)
	}

	if payload.Source != "resources/gameConfig" {
		t.Fatalf("unexpected source %q", payload.Source)
	}
	if len(payload.Items) == 0 {
		t.Fatal("expected crop analytics items")
	}

	item := findCropAnalyticsItem(payload.Items, "白萝卜")
	if item == nil {
		t.Fatalf("expected 白萝卜 in analytics list")
	}
	if item.ID != 1020002 || item.SeedID != 20002 || item.FruitID != 40002 {
		t.Fatalf("unexpected ids %#v", item)
	}
	if item.GrowTime != 60 || item.GrowTimeText != "1分" {
		t.Fatalf("unexpected grow time %#v", item)
	}
	if item.HarvestExp != 1 || item.Income != 10 || item.NetProfit != 9 {
		t.Fatalf("unexpected economics %#v", item)
	}
	if item.ExpPerHour != 60 || item.ProfitPerHour != 540 {
		t.Fatalf("unexpected hourly metrics %#v", item)
	}
	if item.NormalFertilizerExpPerHour != 120 || item.NormalFertilizerProfitPerHour != 1080 {
		t.Fatalf("unexpected fertilizer metrics %#v", item)
	}
	if !item.ShopEligible {
		t.Fatalf("expected shop eligible item %#v", item)
	}
}

func TestSmartFertilizerCropClassFromGrowPhases(t *testing.T) {
	testCases := []struct {
		name   string
		phases string
		want   SmartFertilizerCropClass
	}{
		{
			name:   "large leaf tied for longest",
			phases: "种子:4800;发芽:4800;小叶子:4800;大叶子:7200;花蕾:7200;盛开:0;",
			want:   SmartFertilizerCropSmart,
		},
		{
			name:   "large leaf shorter than longest",
			phases: "种子:4800;大叶子:7200;花蕾:9600;成熟:0;",
			want:   SmartFertilizerCropFallback,
		},
		{
			name:   "no large leaf phase",
			phases: "种子:30;发芽:30;成熟:0;",
			want:   SmartFertilizerCropFallback,
		},
		{
			name:   "invalid and zero durations",
			phases: "种子:abc;大叶子:0;成熟:0;",
			want:   SmartFertilizerCropFallback,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := smartFertilizerCropClassFromGrowPhases(testCase.phases); got != testCase.want {
				t.Fatalf("class(%q) = %v, want %v", testCase.phases, got, testCase.want)
			}
		})
	}
}

func TestLoadSmartFertilizerCropIndexUsesPlantAndSeedIDs(t *testing.T) {
	index, err := LoadSmartFertilizerCropIndex(filepath.Join("..", "..", "resources", "gameConfig"))
	if err != nil {
		t.Fatalf("load smart fertilizer crop index: %v", err)
	}
	if got := index.Classify(1020003, 0); got != SmartFertilizerCropSmart {
		t.Fatalf("carrot class = %v, want smart", got)
	}
	if got := index.Classify(0, 20002); got != SmartFertilizerCropFallback {
		t.Fatalf("white radish class = %v, want fallback", got)
	}
	if got := index.Classify(999999, 0); got != SmartFertilizerCropUnknown {
		t.Fatalf("unknown class = %v, want unknown", got)
	}
}

func TestBuildStealCropOptionsFromResources(t *testing.T) {
	payload, err := BuildStealCropOptions(filepath.Join("..", "..", "resources", "gameConfig"))
	if err != nil {
		t.Fatalf("BuildStealCropOptions: %v", err)
	}
	if !payload.OK {
		t.Fatalf("payload should be ok: %#v", payload)
	}
	if len(payload.List) == 0 {
		t.Fatal("expected steal crop options")
	}
	first := payload.List[0]
	if first.PlantID <= 0 || first.SeedID <= 0 || first.Name == "" || first.Level <= 0 {
		t.Fatalf("expected populated first option, got %#v", first)
	}
	if first.ImageURL == "" {
		t.Fatalf("expected first option image")
	}
	for _, expected := range []struct {
		name    string
		plantID int
		seedID  int
	}{
		{name: "红云飞片", plantID: 1020193, seedID: 20193},
		{name: "艾草", plantID: 1021135, seedID: 21135},
	} {
		index := stealCropOptionIndex(payload.List, expected.name)
		if index < 0 {
			t.Fatalf("expected %s option in list", expected.name)
		}
		option := payload.List[index]
		if option.PlantID != expected.plantID || option.SeedID != expected.seedID || option.ImageURL == "" {
			t.Fatalf("unexpected %s option %#v", expected.name, option)
		}
	}
	for _, expected := range []struct {
		name    string
		plantID int
		seedID  int
	}{
		{name: "发财红包", plantID: 1020329, seedID: 20329},
		{name: "星语铃花", plantID: 1029003, seedID: 29003},
		{name: "蝶梦星铃", plantID: 1028003, seedID: 0},
	} {
		index := stealCropOptionIndex(payload.List, expected.name)
		if index < 0 {
			t.Fatalf("expected %s option in list", expected.name)
		}
		option := payload.List[index]
		if option.PlantID != expected.plantID || option.SeedID != expected.seedID || option.ImageURL == "" {
			t.Fatalf("unexpected %s option %#v", expected.name, option)
		}
		if option.SortGroup != 2 {
			t.Fatalf("expected activity crop %s to sort last, got %#v", expected.name, option)
		}
	}
}

func TestNewCropWarehouseNameMapping(t *testing.T) {
	itemMap, err := LoadItemInfoMap(filepath.Join("..", "..", "resources", "gameConfig"))
	if err != nil {
		t.Fatalf("LoadItemInfoMap: %v", err)
	}
	for id, want := range map[int]string{
		20329:   "发财红包种子",
		40329:   "发财红包",
		1049003: "黄金·星语铃花",
		204006:  "蝶梦星铃",
	} {
		if got := itemMap[id].Name; got != want {
			t.Fatalf("item %d name = %q, want %q", id, got, want)
		}
	}
}

func TestBuildStealCropOptionsUsesMappedCropLevels(t *testing.T) {
	payload, err := BuildStealCropOptions(filepath.Join("..", "..", "resources", "gameConfig"))
	if err != nil {
		t.Fatalf("BuildStealCropOptions: %v", err)
	}

	for _, expected := range []struct {
		name  string
		level int
	}{
		{name: "蓝铃花", level: 162},
		{name: "宋梅", level: 164},
	} {
		index := stealCropOptionIndex(payload.List, expected.name)
		if index < 0 {
			t.Fatalf("expected %s option in list", expected.name)
		}
		if got := payload.List[index].Level; got != expected.level {
			t.Fatalf("%s level = %d, want %d", expected.name, got, expected.level)
		}
	}
}

func TestBuildStealCropOptionsSortsLevelCropsBeforeActivityCrops(t *testing.T) {
	payload, err := BuildStealCropOptions(filepath.Join("..", "..", "resources", "gameConfig"))
	if err != nil {
		t.Fatalf("BuildStealCropOptions: %v", err)
	}

	whiteRadish := stealCropOptionIndex(payload.List, "白萝卜")
	carrot := stealCropOptionIndex(payload.List, "胡萝卜")
	chineseCabbage := stealCropOptionIndex(payload.List, "大白菜")
	lilac := stealCropOptionIndex(payload.List, "丁香花")
	impatiens := stealCropOptionIndex(payload.List, "凤仙花")
	if whiteRadish < 0 || carrot < 0 || chineseCabbage < 0 || lilac < 0 || impatiens < 0 {
		t.Fatalf("expected representative crops in list, got indexes whiteRadish=%d carrot=%d chineseCabbage=%d lilac=%d impatiens=%d", whiteRadish, carrot, chineseCabbage, lilac, impatiens)
	}
	if !(whiteRadish < carrot && carrot < chineseCabbage) {
		t.Fatalf("expected level crops ordered by level, got whiteRadish=%d carrot=%d chineseCabbage=%d", whiteRadish, carrot, chineseCabbage)
	}
	if !(chineseCabbage < lilac && chineseCabbage < impatiens) {
		t.Fatalf("expected activity crops after level crops, got chineseCabbage=%d lilac=%d impatiens=%d", chineseCabbage, lilac, impatiens)
	}
}

func stealCropOptionIndex(options []StealCropOption, name string) int {
	for index, option := range options {
		if option.Name == name {
			return index
		}
	}
	return -1
}

func TestCropAnalyticsSortsByExpPerHour(t *testing.T) {
	payload, err := LoadCropAnalytics(filepath.Join("..", "..", "resources", "gameConfig"))
	if err != nil {
		t.Fatalf("load crop analytics: %v", err)
	}
	if len(payload.Items) < 2 {
		t.Fatalf("expected at least two analytics items")
	}
	for i := 1; i < len(payload.Items); i++ {
		if payload.Items[i-1].ExpPerHour < payload.Items[i].ExpPerHour {
			t.Fatalf("items not sorted by exp per hour at %d: %.2f < %.2f", i, payload.Items[i-1].ExpPerHour, payload.Items[i].ExpPerHour)
		}
	}
}

func TestCropAnalyticsExcludesNonCanonicalShopCropFromExperienceRanking(t *testing.T) {
	payload, err := LoadCropAnalyticsForLevel(filepath.Join("..", "..", "resources", "gameConfig"), CropAnalyticsOptions{
		RequestedMaxLevel: 139,
	})
	if err != nil {
		t.Fatalf("load crop analytics: %v", err)
	}
	if findCropAnalyticsItem(payload.Items, "菠萝蜜") != nil {
		t.Fatalf("expected 菠萝蜜 to be excluded from the canonical level-crop ranking")
	}
	if findCropAnalyticsItem(payload.Items, "晚香玉") == nil {
		t.Fatalf("expected 晚香玉 in the canonical level-crop ranking")
	}
	if len(payload.Items) == 0 || payload.Items[0].Name != "晚香玉" {
		t.Fatalf("expected 晚香玉 to lead the 139-level experience ranking, got %#v", payload.Items)
	}
}

func TestAtlasPreviewUsesStaticConfigAndDisablesRuntimeActions(t *testing.T) {
	payload, err := LoadAtlasPreview(filepath.Join("..", "..", "resources", "gameConfig"))
	if err != nil {
		t.Fatalf("load atlas preview: %v", err)
	}

	if payload.Status != "static_preview" {
		t.Fatalf("unexpected atlas status %q", payload.Status)
	}
	if payload.RefreshEnabled || payload.BuyEnabled {
		t.Fatalf("runtime actions should be disabled: %#v", payload)
	}
	if len(payload.Sections) == 0 {
		t.Fatalf("expected atlas sections")
	}

	cropSection := findAtlasSection(payload.Sections, "crop")
	if cropSection == nil {
		t.Fatalf("expected crop atlas section")
	}
	if cropSection.Label != "作物图鉴" {
		t.Fatalf("unexpected crop section label %q", cropSection.Label)
	}
	item := findAtlasItem(cropSection.Items, "白萝卜")
	if item == nil {
		t.Fatalf("expected 白萝卜 in atlas preview")
	}
	if item.SeedID != 20002 || item.FruitID != 40002 || item.Level != 1 {
		t.Fatalf("unexpected atlas item %#v", item)
	}
}

func TestRuntimeAtlasPreviewBuildsCropAndMutationSections(t *testing.T) {
	payload := BuildRuntimeAtlasPreview(map[string]any{
		"crop": map[string]any{
			"items": []any{
				map[string]any{"fruitId": 40002, "seedId": 20002, "name": "白萝卜", "locked": true, "sort": 2},
			},
		},
		"mutation": map[string]any{
			"items": []any{
				map[string]any{"fruitId": 49001, "name": "黄金·白萝卜", "unlocked": true, "progress": 40, "sort": 1},
			},
		},
	})

	if !payload.RefreshEnabled || !payload.BuyEnabled {
		t.Fatalf("runtime atlas actions should be enabled: %#v", payload)
	}
	crop := findAtlasSection(payload.Sections, "crop")
	mutation := findAtlasSection(payload.Sections, "mutation")
	if crop == nil || mutation == nil {
		t.Fatalf("expected crop and mutation sections, got %#v", payload.Sections)
	}
	cropItem := findAtlasItem(crop.Items, "白萝卜")
	if cropItem == nil || !cropItem.Locked || cropItem.AtlasType != "crop" || cropItem.Sort != 2 {
		t.Fatalf("unexpected crop item %#v", cropItem)
	}
	mutationItem := findAtlasItem(mutation.Items, "黄金·白萝卜")
	if mutationItem == nil || !mutationItem.Unlocked || mutationItem.AtlasType != "mutation" || mutationItem.Progress != 40 {
		t.Fatalf("unexpected mutation item %#v", mutationItem)
	}
}

func TestRuntimeAtlasPreviewUsesMutationMappingForNameImageAndSort(t *testing.T) {
	payload := BuildRuntimeAtlasPreview(map[string]any{
		"mutation": map[string]any{
			"items": []any{
				map[string]any{"fruitId": 1120167, "locked": true},
				map[string]any{"fruitId": 1120112, "locked": false},
				map[string]any{"fruitId": 204002, "locked": false},
			},
		},
	})

	mutation := findAtlasSection(payload.Sections, "mutation")
	if mutation == nil {
		t.Fatalf("expected mutation section")
	}
	if len(mutation.Items) != 3 {
		t.Fatalf("expected 3 mutation rows, got %#v", mutation.Items)
	}
	if mutation.Items[0].Name != "黄金·风信子" || mutation.Items[1].Name != "黄金·欢乐糖果" || mutation.Items[2].Name != "哈哈南瓜塔" {
		t.Fatalf("unexpected mutation order/names %#v", mutation.Items)
	}
	if mutation.Items[0].GroupName != "黄金果实" || mutation.Items[2].GroupName != "装扮果实" {
		t.Fatalf("unexpected mutation groups %#v", mutation.Items)
	}
	if mutation.Items[0].Progress != 40 {
		t.Fatalf("expected atlas point from mapping, got %#v", mutation.Items[0])
	}
	if mutation.Items[0].ImageURL == "" || mutation.Items[2].ImageURL == "" {
		t.Fatalf("expected mapped mutation images %#v", mutation.Items)
	}
	for _, item := range mutation.Items {
		if !strings.HasPrefix(item.ImageURL, localImageAssetPrefix) || strings.Contains(item.ImageURL, "data:image/") {
			t.Fatalf("expected local mutation atlas image for %q, got %q", item.Name, item.ImageURL)
		}
	}
}

func TestRuntimeAtlasPreviewUsesExtendedMutationMappingForQingmei(t *testing.T) {
	payload := BuildRuntimeAtlasPreview(map[string]any{
		"mutation": map[string]any{
			"items": []any{
				map[string]any{"fruitId": 1041221, "locked": false},
			},
		},
	})

	mutation := findAtlasSection(payload.Sections, "mutation")
	if mutation == nil {
		t.Fatalf("expected mutation section")
	}
	qingmei := findAtlasItem(mutation.Items, "黄金·青梅")
	if qingmei == nil || qingmei.ImageURL == "" {
		t.Fatalf("expected mapped 黄金·青梅 row for 1041221, got %#v", mutation.Items)
	}
}

func TestRuntimeAtlasPreviewUsesCropMappingForMissingNamesImagesAndSort(t *testing.T) {
	fruitIDs := []int{40167, 40258, 40256, 40257, 40261, 41135, 40304, 40193, 40184, 49002, 40132, 40172}
	rows := make([]any, 0, len(fruitIDs))
	for _, fruitID := range fruitIDs {
		rows = append(rows, map[string]any{"fruitId": fruitID})
	}
	payload := BuildRuntimeAtlasPreview(map[string]any{
		"crop": map[string]any{"items": rows},
	})

	crop := findAtlasSection(payload.Sections, "crop")
	if crop == nil {
		t.Fatalf("expected crop section")
	}
	if len(crop.Items) != len(fruitIDs) {
		t.Fatalf("expected %d crop rows, got %#v", len(fruitIDs), crop.Items)
	}
	expectedNames := []string{
		"欢乐糖果",
		"卡特兰",
		"红云飞片",
		"石竹花",
		"针垫花",
		"孔雀草",
		"欧石楠",
		"黄金果",
		"蓝铃花",
		"宋梅",
		"哈哈南瓜",
		"艾草",
	}
	for index, expectedName := range expectedNames {
		item := crop.Items[index]
		if item.Name != expectedName {
			t.Fatalf("unexpected crop order/name at %d: want %s got %#v", index, expectedName, item)
		}
		if strings.HasPrefix(item.Name, "图鉴 ") || item.ImageURL == "" || item.SeedID <= 0 || item.Level <= 0 {
			t.Fatalf("expected mapped crop metadata at %d: %#v", index, item)
		}
		if !strings.HasPrefix(item.ImageURL, localImageAssetPrefix) || strings.Contains(item.ImageURL, "data:image/") {
			t.Fatalf("expected local crop atlas image at %d, got %q", index, item.ImageURL)
		}
	}
}

func TestRuntimeAtlasPreviewKeepsExplicitCropSortBeforeMappingLevel(t *testing.T) {
	payload := BuildRuntimeAtlasPreview(map[string]any{
		"crop": map[string]any{
			"items": []any{
				map[string]any{"fruitId": 40003, "sort": 2},
				map[string]any{"fruitId": 40002, "sort": 1},
				map[string]any{"fruitId": 40059},
			},
		},
	})

	crop := findAtlasSection(payload.Sections, "crop")
	if crop == nil || len(crop.Items) != 3 {
		t.Fatalf("expected crop rows, got %#v", crop)
	}
	got := []string{crop.Items[0].Name, crop.Items[1].Name, crop.Items[2].Name}
	want := []string{"白萝卜", "胡萝卜", "大白菜"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected crop sort order: want %#v got %#v", want, got)
	}
}

func TestAtlasLockedCropPurchasePlanIgnoresMutationRows(t *testing.T) {
	plan := BuildAtlasLockedCropPurchasePlan(AtlasLockedCropPurchaseInput{
		AtlasItems: []AtlasItem{
			{AtlasType: "crop", Name: "白萝卜", SeedID: 20002, FruitID: 40002, Level: 1, Locked: true, Sort: 2},
			{AtlasType: "mutation", Name: "黄金·白萝卜", FruitID: 49001, Locked: true, Sort: 1},
			{AtlasType: "crop", Name: "已解锁作物", SeedID: 20003, FruitID: 40003, Locked: false, Sort: 3},
		},
		ShopList: []any{
			map[string]any{"itemId": 20002, "goodsId": 90002, "price": 7, "level": 1},
		},
		EffectiveLevel: 10,
		CountPerSeed:   2,
	})

	if len(plan.Purchases) != 1 {
		t.Fatalf("expected one purchasable crop, got %#v", plan)
	}
	purchase := plan.Purchases[0]
	if purchase.SeedID != 20002 || purchase.GoodsID != 90002 || purchase.Count != 2 {
		t.Fatalf("unexpected purchase %#v", purchase)
	}
	if plan.Summary.LockedCropCount != 1 || plan.Summary.Purchasable != 1 {
		t.Fatalf("unexpected summary %#v", plan.Summary)
	}
	if plan.Summary.SkipReasons["mutation_ignored"] != 1 || plan.Summary.SkipReasons["already_unlocked"] != 1 {
		t.Fatalf("unexpected skip reasons %#v", plan.Summary.SkipReasons)
	}
}

func TestLandDetailsRuntimeGateDoesNotFakeLandData(t *testing.T) {
	payload := LandDetailsGate()

	if payload.Status != "not_migrated" {
		t.Fatalf("unexpected land status %#v", payload)
	}
	if len(payload.Lands) != 0 {
		t.Fatalf("expected no fake land data, got %#v", payload.Lands)
	}
	for _, action := range payload.Actions {
		if action.Enabled {
			t.Fatalf("land action should be disabled: %#v", action)
		}
	}
	if len(payload.Actions) != 3 {
		t.Fatalf("expected three land action gates, got %#v", payload.Actions)
	}
	if payload.Actions[0].ID != "rush" || payload.Actions[0].Label != "一键催熟" {
		t.Fatalf("unexpected land action gate %#v", payload.Actions[0])
	}
	if payload.Actions[1].ID != "fertilize_normal" || payload.Actions[1].Label != "一键无机肥" {
		t.Fatalf("unexpected land action gate %#v", payload.Actions[1])
	}
	if payload.Actions[2].ID != "fertilize_organic" || payload.Actions[2].Label != "一键有机肥" {
		t.Fatalf("unexpected land action gate %#v", payload.Actions[2])
	}
}

func TestRuntimeLandDetailsExposesLandRushAndBulkFertilizerActions(t *testing.T) {
	payload := BuildRuntimeLandDetailsForRoot(map[string]any{
		"farmType": "own",
		"grids": []any{
			map[string]any{
				"landId":      1,
				"plantName":   "白萝卜",
				"status":      "growing",
				"matureInSec": 180,
			},
		},
	}, filepath.Join("..", "..", "resources", "gameConfig"))

	if len(payload.Actions) != 3 {
		t.Fatalf("expected three runtime land actions, got %#v", payload.Actions)
	}
	action := payload.Actions[0]
	if action.ID != "rush" || action.Label != "一键催熟" || !action.Enabled {
		t.Fatalf("unexpected runtime land action %#v", action)
	}
	if payload.Actions[1].ID != "fertilize_normal" || payload.Actions[1].Label != "一键无机肥" || !payload.Actions[1].Enabled {
		t.Fatalf("unexpected runtime land action %#v", payload.Actions[1])
	}
	if payload.Actions[2].ID != "fertilize_organic" || payload.Actions[2].Label != "一键有机肥" || !payload.Actions[2].Enabled {
		t.Fatalf("unexpected runtime land action %#v", payload.Actions[2])
	}
}

func TestRuntimeLandDetailsDoesNotExposeTotalStageCount(t *testing.T) {
	payload := BuildRuntimeLandDetailsForRoot(map[string]any{
		"farmType": "own",
		"grids": []any{map[string]any{
			"landId":       1,
			"plantName":    "白萝卜",
			"stageKind":    "growing",
			"currentStage": 2,
			"totalStages":  5,
		}},
	}, filepath.Join("..", "..", "resources", "gameConfig"))

	encoded, err := json.Marshal(payload.Lands[0])
	if err != nil {
		t.Fatalf("marshal land: %v", err)
	}
	if strings.Contains(string(encoded), `"totalStages"`) {
		t.Fatalf("land payload still exposes totalStages: %s", encoded)
	}
}

func TestRuntimeLandDetailsReadsSeasonFromRawPlantData(t *testing.T) {
	payload := BuildRuntimeLandDetailsForRoot(map[string]any{
		"farmType": "own",
		"grids": []any{
			map[string]any{
				"landId":    1,
				"plantName": "白萝卜",
				"status":    "growing",
				"raw": map[string]any{
					"plantData": map[string]any{
						"season": float64(2),
					},
					"config": map[string]any{
						"seasons": float64(3),
					},
				},
			},
		},
	}, filepath.Join("..", "..", "resources", "gameConfig"))

	if len(payload.Lands) != 1 {
		t.Fatalf("expected one land, got %#v", payload.Lands)
	}
	land := payload.Lands[0]
	if land.CurrentSeason != 2 || land.TotalSeason != 3 || !land.IsMultiSeason {
		t.Fatalf("expected raw season fallback, got %#v", land)
	}
}

func TestRuntimeLandDetailsDoesNotInferSeasonFromLocalPlantConfig(t *testing.T) {
	payload := BuildRuntimeLandDetailsForRoot(map[string]any{
		"farmType": "own",
		"grids": []any{
			map[string]any{
				"landId":    1,
				"seedId":    20087,
				"plantName": "百香果",
				"status":    "growing",
			},
		},
	}, filepath.Join("..", "..", "resources", "gameConfig"))

	if len(payload.Lands) != 1 {
		t.Fatalf("expected one land, got %#v", payload.Lands)
	}
	land := payload.Lands[0]
	if land.TotalSeason != 0 || land.IsMultiSeason {
		t.Fatalf("season count should only come from runtime land data, got %#v", land)
	}
}

func TestWarehouseRuntimeGateDoesNotFakeItems(t *testing.T) {
	payload := WarehouseGate()

	if payload.Status != "not_migrated" {
		t.Fatalf("unexpected warehouse status %#v", payload)
	}
	if len(payload.Items) != 0 {
		t.Fatalf("expected no fake warehouse items, got %#v", payload.Items)
	}
	for _, action := range payload.Actions {
		if action.Enabled {
			t.Fatalf("warehouse action should be disabled: %#v", action)
		}
	}
	if len(payload.Actions) < 2 {
		t.Fatalf("expected warehouse action gates, got %#v", payload.Actions)
	}
}

func TestRuntimeWarehouseUsesFixedCategoriesAndMappedImages(t *testing.T) {
	itemMap, err := LoadItemInfoMap(filepath.Join("..", "..", "resources", "gameConfig"))
	if err != nil {
		t.Fatalf("load item info: %v", err)
	}
	if category := warehouseMappedCategory(49001); category != "超变果实" {
		t.Fatalf("expected mapped mutation category, got %q from root %q", category, DefaultGameConfigRoot())
	}

	payload := BuildRuntimeWarehouse(map[string]any{
		"items": []any{
			map[string]any{"itemId": 20002, "count": 3, "price": 1},
			map[string]any{"itemId": 40002, "count": 5, "price": 2},
			map[string]any{"itemId": 49001, "count": 1, "price": 3},
			map[string]any{"itemId": 1016, "count": 7},
		},
	}, itemMap)

	categories := make(map[int]string)
	labels := make(map[int]string)
	images := make(map[int]string)
	for _, item := range payload.Items {
		categories[item.ItemID] = item.Category
		labels[item.ItemID] = item.CategoryLabel
		images[item.ItemID] = item.ImageURL
	}

	wantCategories := map[int]string{
		20002: "seed",
		40002: "fruit",
		49001: "mutation",
		1016:  "tool",
	}
	if !reflect.DeepEqual(categories, wantCategories) {
		t.Fatalf("unexpected categories %#v", categories)
	}
	if labels[49001] != "超变果实" || labels[1016] != "道具" {
		t.Fatalf("unexpected labels %#v", labels)
	}
	if images[20002] == "" || images[40002] == "" || images[49001] == "" || images[1016] == "" {
		t.Fatalf("expected mapped images, got %#v", images)
	}
	if images[20002] != images[40002] {
		t.Fatalf("seed image should use crop main image: seed=%q fruit=%q", images[20002], images[40002])
	}
	for itemID, imageURL := range images {
		if !strings.HasPrefix(imageURL, localImageAssetPrefix) || strings.Contains(imageURL, "data:image/") {
			t.Fatalf("expected local warehouse image for %d, got %q", itemID, imageURL)
		}
	}

	var gotOrder []string
	for _, category := range payload.Summary.CategoryList {
		gotOrder = append(gotOrder, category.Key)
	}
	wantOrder := []string{"fruit", "mutation", "seed", "tool"}
	if !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("unexpected category order %#v", gotOrder)
	}
}

func TestBuildRuntimeWarehouseKeepsProtocolWarehouseKey(t *testing.T) {
	itemMap := map[int]itemInfoConfigItem{
		40002: {Name: "白萝卜", Type: 6},
	}
	payload := BuildRuntimeWarehouse(map[string]any{
		"items": []any{
			map[string]any{
				"itemId":        float64(40002),
				"count":         float64(5),
				"name":          "白萝卜",
				"warehouseKey":  "40002:open:123456",
				"saleUnitPrice": float64(2),
				"uid":           float64(123456),
			},
		},
	}, itemMap)

	if len(payload.Items) != 1 {
		t.Fatalf("expected one item, got %#v", payload)
	}
	if payload.Items[0].ID != "40002:open:123456" {
		t.Fatalf("expected protocol warehouse key, got %#v", payload.Items[0])
	}
	if payload.Items[0].EstimatedSellPrice != 10 {
		t.Fatalf("expected estimate from protocol sale unit price, got %#v", payload.Items[0])
	}
}

func TestBuildRuntimeWarehouseTreatsRuntimeCanSellAsSellable(t *testing.T) {
	itemMap := map[int]itemInfoConfigItem{
		41221: {Name: "青梅", Type: 6},
	}
	payload := BuildRuntimeWarehouse(map[string]any{
		"items": []any{
			map[string]any{
				"itemId":       float64(41221),
				"count":        float64(3),
				"name":         "青梅",
				"warehouseKey": "41221:open:9001",
				"canSell":      true,
			},
		},
	}, itemMap)

	if len(payload.Items) != 1 {
		t.Fatalf("expected one item, got %#v", payload)
	}
	if !payload.Items[0].CanSell {
		t.Fatalf("runtime canSell=true item should remain sellable: %#v", payload.Items[0])
	}
	if payload.Summary.SellableDistinct != 1 || payload.Summary.SellableCount != 3 {
		t.Fatalf("summary should count runtime-sellable item: %#v", payload.Summary)
	}
}

func TestBuildRuntimeWarehouseKeepsExplicitRuntimeCanSellFalseEvenWithPrice(t *testing.T) {
	itemMap := map[int]itemInfoConfigItem{
		41221: {Name: "青梅", Type: 6},
	}
	payload := BuildRuntimeWarehouse(map[string]any{
		"items": []any{
			map[string]any{
				"itemId":        float64(41221),
				"count":         float64(3),
				"name":          "青梅",
				"warehouseKey":  "41221:open:9001",
				"canSell":       false,
				"saleUnitPrice": float64(240),
			},
		},
	}, itemMap)

	if len(payload.Items) != 1 {
		t.Fatalf("expected one item, got %#v", payload)
	}
	item := payload.Items[0]
	if item.CanSell {
		t.Fatalf("explicit runtime canSell=false must not be overridden by price: %#v", item)
	}
	if item.EstimatedSellPrice != 720 {
		t.Fatalf("priced item should still expose its estimate: %#v", item)
	}
}

func TestBuildBackpackSeedOptionsEnrichesAndSortsRuntimeSeeds(t *testing.T) {
	level21 := 21
	level200 := 200
	payload := BuildBackpackSeedOptions([]any{
		map[string]any{"seedId": float64(20133), "name": "凤仙花种子", "count": float64(5), "level": float64(21)},
		map[string]any{"seedId": float64(21032), "name": "琉璃宝荷种子", "count": float64(9), "level": float64(200)},
		map[string]any{"seedId": float64(20176), "name": "金银花种子", "count": float64(2), "level": float64(21)},
	}, BackpackSeedOptionsRequest{
		SelectedSeedIDs: []int{21032, 20133},
		DisabledSeedIDs: []int{20176},
		ForcePriority:   true,
	})

	if !payload.OK {
		t.Fatalf("payload not ok: %#v", payload)
	}
	if len(payload.List) != 3 {
		t.Fatalf("option count = %d, want 3: %#v", len(payload.List), payload.List)
	}
	if payload.List[0].SeedID != 21032 || payload.List[0].Name != "琉璃宝荷" || payload.List[0].Count != 9 || payload.List[0].Level == nil || *payload.List[0].Level != level200 {
		t.Fatalf("unexpected first option: %#v", payload.List[0])
	}
	if payload.List[1].SeedID != 20133 || payload.List[1].Level == nil || *payload.List[1].Level != level21 {
		t.Fatalf("unexpected second option: %#v", payload.List[1])
	}
	if payload.List[2].SeedID != 20176 || !payload.List[2].Disabled {
		t.Fatalf("disabled seed should remain visible and marked: %#v", payload.List[2])
	}
}

func TestBuildBackpackSeedOptionsPrefersProtocolSeedLevelOverMapping(t *testing.T) {
	protocolLevel := 41
	payload := BuildBackpackSeedOptions([]any{
		map[string]any{"seedId": float64(20133), "name": "凤仙花种子", "count": float64(5), "seedLevel": float64(protocolLevel)},
	}, BackpackSeedOptionsRequest{})

	if !payload.OK {
		t.Fatalf("payload not ok: %#v", payload)
	}
	if len(payload.List) != 1 {
		t.Fatalf("option count = %d, want 1: %#v", len(payload.List), payload.List)
	}
	if payload.List[0].Level == nil || *payload.List[0].Level != protocolLevel {
		t.Fatalf("expected protocol level %d, got %#v", protocolLevel, payload.List[0])
	}
}

func TestBuildBackpackSeedOptionsBlocksAliasFourGridSeedWhenDisabled(t *testing.T) {
	payload := BuildBackpackSeedOptions([]any{
		map[string]any{"seedId": float64(20416), "name": "哈哈南瓜种子", "count": float64(8), "level": float64(31)},
	}, BackpackSeedOptionsRequest{
		FourGridPlantEnabled: false,
	})

	if !payload.OK {
		t.Fatalf("payload not ok: %#v", payload)
	}
	if len(payload.List) != 1 {
		t.Fatalf("option count = %d, want 1: %#v", len(payload.List), payload.List)
	}
	option := payload.List[0]
	if option.SeedID != 20416 || option.Name != "哈哈南瓜" {
		t.Fatalf("unexpected option identity: %#v", option)
	}
	if option.Plantable || option.PlantableReason != "multi_tile_seed_not_supported" || option.PlantSize != 2 {
		t.Fatalf("alias four-grid seed should be blocked when four-grid planting is disabled: %#v", option)
	}
}

func TestBuildBackpackSeedOptionsDropsNonSeedBagItems(t *testing.T) {
	payload := BuildBackpackSeedOptions([]any{
		map[string]any{"seedId": float64(20133), "name": "凤仙花种子", "count": float64(5), "level": float64(21)},
		map[string]any{"seedId": float64(90004), "name": "1天狗粮", "count": float64(6)},
		map[string]any{"seedId": float64(100003), "name": "化肥礼包", "count": float64(117)},
		map[string]any{"seedId": float64(101351), "name": "同气连枝礼包", "count": float64(12)},
		map[string]any{"itemId": float64(99999901), "name": "足球", "count": float64(34)},
	}, BackpackSeedOptionsRequest{
		SelectedSeedIDs: []int{90004, 20133},
		ForcePriority:   true,
	})

	if !payload.OK {
		t.Fatalf("payload not ok: %#v", payload)
	}
	if len(payload.List) != 1 {
		t.Fatalf("option count = %d, want 1: %#v", len(payload.List), payload.List)
	}
	if payload.List[0].SeedID != 20133 || payload.List[0].Name != "凤仙花" {
		t.Fatalf("expected only real seed option, got %#v", payload.List[0])
	}
}

func TestBuildBackpackSeedOptionsAcceptsRuntimeVerifiedUnknownSeed(t *testing.T) {
	payload := BuildBackpackSeedOptions([]any{
		map[string]any{
			"itemId":          float64(99999123),
			"name":            "未来活动种子",
			"count":           float64(4),
			"type":            float64(5),
			"interactionType": "plant",
		},
	}, BackpackSeedOptionsRequest{})

	if !payload.OK {
		t.Fatalf("payload not ok: %#v", payload)
	}
	if len(payload.List) != 1 {
		t.Fatalf("option count = %d, want 1: %#v", len(payload.List), payload.List)
	}
	if option := payload.List[0]; option.SeedID != 99999123 || option.Name != "未来活动" || option.BackpackCount != 4 {
		t.Fatalf("unexpected runtime-verified activity seed: %#v", option)
	}
}

func findCropAnalyticsItem(items []CropAnalyticsItem, name string) *CropAnalyticsItem {
	for i := range items {
		if items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}

func findAtlasSection(sections []AtlasSection, id string) *AtlasSection {
	for i := range sections {
		if sections[i].ID == id {
			return &sections[i]
		}
	}
	return nil
}

func findAtlasItem(items []AtlasItem, name string) *AtlasItem {
	for i := range items {
		if items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}
