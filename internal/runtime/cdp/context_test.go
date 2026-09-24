package cdp

import "testing"

func TestSelectBestProbePrefersGameRuntimeContext(t *testing.T) {
	best, ok := SelectBestProbe([]ContextProbe{
		{ID: 1, Name: "ordinary", HasDocument: true, HasWx: true, Score: ScoreContextProbe(ContextProbe{HasDocument: true, HasWx: true})},
		{ID: 2, Name: "gameContext", HasCc: true, HasGameGlobal: true, HasCanvas: true, Scene: "FarmScene"},
		{ID: 3, Name: "worker", HasGameGlobal: true},
	})

	if !ok {
		t.Fatal("expected a selectable context")
	}
	if best.ID != 2 {
		t.Fatalf("expected game context, got %#v", best)
	}
	if best.Score <= 100 {
		t.Fatalf("expected strong score, got %#v", best)
	}
}

func TestSelectBestProbeRejectsWeakContexts(t *testing.T) {
	_, ok := SelectBestProbe([]ContextProbe{
		{ID: 1, HasDocument: true, HasWx: true},
		{ID: 2, HasGameGlobal: true},
	})

	if ok {
		t.Fatal("weak browser contexts should not be selected")
	}
}

func TestSelectBestProbeAcceptsRuntimeWithOnlyCc(t *testing.T) {
	best, ok := SelectBestProbe([]ContextProbe{
		{ID: 2, HasCc: true},
	})

	if !ok {
		t.Fatal("context with cc should meet the runtime threshold")
	}
	if best.ID != 2 || best.Score != 100 {
		t.Fatalf("unexpected best probe %#v", best)
	}
}
