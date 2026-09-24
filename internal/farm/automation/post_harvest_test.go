package automation

import (
	"reflect"
	"testing"
)

func TestSuccessfulRuntimeLandIDsRequiresExplicitSuccess(t *testing.T) {
	got := SuccessfulRuntimeLandIDs(map[string]any{
		"results": []any{
			map[string]any{"ok": true, "landId": float64(8)},
			map[string]any{"ok": true, "land_id": "3"},
			map[string]any{"ok": false, "landId": float64(5)},
			map[string]any{"landId": float64(6)},
			map[string]any{"ok": true, "landId": float64(8)},
			map[string]any{"ok": true, "action": "skipped", "landId": float64(9)},
		},
	})

	if want := []int{3, 8}; !reflect.DeepEqual(got, want) {
		t.Fatalf("successful land IDs = %#v, want %#v", got, want)
	}
}

func TestMultiSeasonContinuationLandIDsUsesConfirmedAnchors(t *testing.T) {
	status := map[string]any{
		"grids": []any{
			map[string]any{"landId": float64(1), "occupancyAnchorLandId": float64(5), "hasPlant": true, "stageKind": "growing", "isMultiSeason": true, "currentSeason": float64(2), "totalSeason": float64(3)},
			map[string]any{"landId": float64(5), "occupancyAnchorLandId": float64(5), "hasPlant": true, "stageKind": "growing", "isMultiSeason": true, "currentSeason": float64(2), "totalSeason": float64(3)},
			map[string]any{"landId": float64(7), "hasPlant": true, "stageKind": "growing", "isMultiSeason": true, "currentSeason": float64(1), "totalSeason": float64(3)},
			map[string]any{"landId": float64(9), "hasPlant": true, "stageKind": "growing", "isMultiSeason": false, "currentSeason": float64(2), "totalSeason": float64(3)},
			map[string]any{"landId": float64(11), "hasPlant": true, "stageKind": "mature", "isMultiSeason": true, "currentSeason": float64(2), "totalSeason": float64(3)},
		},
	}

	if got, want := MultiSeasonContinuationLandIDs(status, []int{1, 7, 9, 11}), []int{5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("multi-season anchors = %#v, want %#v", got, want)
	}
}

func TestMultiSeasonFertilizerModeUsesPlantingStrategy(t *testing.T) {
	testCases := []struct {
		name   string
		config map[string]any
		want   string
	}{
		{
			name: "normal planting fertilizer",
			config: map[string]any{
				"autoFarmFertilizerEnabled":              true,
				"autoFarmFertilizerMultiSeason":          true,
				"autoFarmPlantFertilizerMode":            "normal",
				"autoFarmFeatureGroupEnabled.fertilizer": true,
			},
			want: "normal",
		},
		{
			name: "organic planting fertilizer",
			config: map[string]any{
				"autoFarmFertilizerEnabled":              true,
				"autoFarmFertilizerMultiSeason":          true,
				"autoFarmPlantFertilizerMode":            "organic",
				"autoFarmFeatureGroupEnabled.fertilizer": true,
			},
			want: "organic",
		},
		{
			name: "multi-season switch off",
			config: map[string]any{
				"autoFarmFertilizerEnabled":              true,
				"autoFarmFertilizerMultiSeason":          false,
				"autoFarmPlantFertilizerMode":            "normal",
				"autoFarmFeatureGroupEnabled.fertilizer": true,
			},
			want: "",
		},
		{
			name: "planting fertilizer off",
			config: map[string]any{
				"autoFarmFertilizerEnabled":              true,
				"autoFarmFertilizerMultiSeason":          true,
				"autoFarmPlantFertilizerMode":            "none",
				"autoFarmFeatureGroupEnabled.fertilizer": true,
			},
			want: "",
		},
		{
			name: "fertilizer master off",
			config: map[string]any{
				"autoFarmFertilizerEnabled":              false,
				"autoFarmFertilizerMultiSeason":          true,
				"autoFarmPlantFertilizerMode":            "normal",
				"autoFarmFeatureGroupEnabled.fertilizer": false,
			},
			want: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := MultiSeasonFertilizerMode(testCase.config); got != testCase.want {
				t.Fatalf("mode = %q, want %q", got, testCase.want)
			}
		})
	}
}
