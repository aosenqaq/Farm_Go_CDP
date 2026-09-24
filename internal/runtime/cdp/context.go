package cdp

const minUsableExecutionContextScore = 100

type ExecutionContext struct {
	ID     int
	Name   string
	Origin string
}

type ContextProbe struct {
	ID            int
	Name          string
	Origin        string
	HasCc         bool
	HasGameCtl    bool
	HasGameGlobal bool
	HasWx         bool
	HasDocument   bool
	HasCanvas     bool
	Scene         string
	Href          string
	Error         string
	Score         int
}

func ScoreContextProbe(probe ContextProbe) int {
	if probe.Error != "" {
		return -1000
	}
	score := 0
	if probe.HasCc {
		score += 100
	}
	if probe.HasGameCtl {
		score += 160
	}
	if probe.Scene != "" {
		score += 120
	}
	if probe.HasGameGlobal {
		score += 40
	}
	if probe.HasCanvas {
		score += 20
	}
	if probe.HasDocument {
		score += 10
	}
	if probe.HasWx {
		score += 10
	}
	if probe.Href != "" && contains(probe.Href, "servicewechat.com") {
		score += 5
	}
	return score
}

func SelectBestProbe(probes []ContextProbe) (ContextProbe, bool) {
	var best ContextProbe
	found := false
	for _, probe := range probes {
		probe.Score = ScoreContextProbe(probe)
		if !found || probe.Score > best.Score || (probe.Score == best.Score && probe.ID > best.ID) {
			best = probe
			found = true
		}
	}
	if !found || best.Score < minUsableExecutionContextScore {
		return ContextProbe{}, false
	}
	return best, true
}

func contains(value string, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(value) < len(needle) {
		return false
	}
	for i := 0; i <= len(value)-len(needle); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
