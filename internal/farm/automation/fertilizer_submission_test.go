package automation

import "testing"

func TestFertilizerSubmissionRuntimeArgs(t *testing.T) {
	testCases := []struct {
		name       string
		config     map[string]any
		scope      string
		wantMode   string
		wantWaitMS int
	}{
		{
			name:       "disabled",
			config:     map[string]any{},
			scope:      "planting",
			wantMode:   "batch",
			wantWaitMS: 0,
		},
		{
			name: "selected planting",
			config: map[string]any{
				"autoFarmFertilizerDelayedSubmitEnabled": true,
				"autoFarmFertilizerDelayedSubmitScopes":  []any{"planting"},
			},
			scope:      "planting",
			wantMode:   "serial",
			wantWaitMS: 500,
		},
		{
			name: "unselected rush",
			config: map[string]any{
				"autoFarmFertilizerDelayedSubmitEnabled": true,
				"autoFarmFertilizerDelayedSubmitScopes":  []any{"planting"},
			},
			scope:      "rush",
			wantMode:   "batch",
			wantWaitMS: 0,
		},
		{
			name: "legacy enabled defaults all scopes",
			config: map[string]any{
				"autoFarmFertilizerDelayedSubmitEnabled": true,
			},
			scope:      "manual",
			wantMode:   "serial",
			wantWaitMS: 500,
		},
		{
			name: "explicit empty scope disables all",
			config: map[string]any{
				"autoFarmFertilizerDelayedSubmitEnabled": true,
				"autoFarmFertilizerDelayedSubmitScopes":  []any{},
			},
			scope:      "manual",
			wantMode:   "batch",
			wantWaitMS: 0,
		},
		{
			name: "custom interval",
			config: map[string]any{
				"autoFarmFertilizerDelayedSubmitEnabled":    true,
				"autoFarmFertilizerDelayedSubmitScopes":     []any{"rush"},
				"autoFarmFertilizerDelayedSubmitIntervalMs": 850,
			},
			scope:      "rush",
			wantMode:   "serial",
			wantWaitMS: 850,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := FertilizerSubmissionRuntimeArgs(tc.config, tc.scope)
			if got["fertilizerSubmissionMode"] != tc.wantMode || intFromAny(got["betweenLandWait"]) != tc.wantWaitMS {
				t.Fatalf("runtime args = %#v, want mode=%q wait=%d", got, tc.wantMode, tc.wantWaitMS)
			}
		})
	}
}
