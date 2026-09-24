package automation

const (
	fertilizerDelayedSubmitEnabledKey    = "autoFarmFertilizerDelayedSubmitEnabled"
	fertilizerDelayedSubmitScopesKey     = "autoFarmFertilizerDelayedSubmitScopes"
	fertilizerDelayedSubmitIntervalMSKey = "autoFarmFertilizerDelayedSubmitIntervalMs"

	defaultFertilizerDelayedSubmitIntervalMS = 500
)

// FertilizerSubmissionRuntimeArgs resolves the saved submission policy for one fertilizer source.
func FertilizerSubmissionRuntimeArgs(config map[string]any, scope string) map[string]any {
	if !boolFromAny(config[fertilizerDelayedSubmitEnabledKey]) || !fertilizerDelayedScopeSelected(config, scope) {
		return map[string]any{
			"fertilizerSubmissionMode": "batch",
			"betweenLandWait":          0,
		}
	}

	intervalMS := defaultFertilizerDelayedSubmitIntervalMS
	if value, ok := config[fertilizerDelayedSubmitIntervalMSKey]; ok {
		intervalMS = intFromAny(value)
		if intervalMS < 0 {
			intervalMS = 0
		}
	}
	return map[string]any{
		"fertilizerSubmissionMode": "serial",
		"betweenLandWait":          intervalMS,
	}
}

// ApplyFertilizerSubmissionRuntimeArgs adds serial-only options to a runtime payload.
// Batch mode is the WMPF default, so it deliberately leaves existing payloads unchanged.
func ApplyFertilizerSubmissionRuntimeArgs(payload map[string]any, config map[string]any, scope string) map[string]any {
	policy := FertilizerSubmissionRuntimeArgs(config, scope)
	if policy["fertilizerSubmissionMode"] != "serial" {
		return payload
	}
	for key, value := range policy {
		payload[key] = value
	}
	return payload
}

func fertilizerDelayedScopeSelected(config map[string]any, scope string) bool {
	if scope != "planting" && scope != "rush" && scope != "manual" {
		return false
	}
	rawScopes, configured := config[fertilizerDelayedSubmitScopesKey]
	if !configured {
		return true
	}
	for _, item := range sliceFromAny(rawScopes) {
		if text, ok := item.(string); ok && text == scope {
			return true
		}
	}
	if scopes, ok := rawScopes.([]string); ok {
		for _, item := range scopes {
			if item == scope {
				return true
			}
		}
	}
	return false
}
