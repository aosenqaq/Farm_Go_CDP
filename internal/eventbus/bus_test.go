package eventbus

import (
	"testing"
	"time"
)

func TestPublishDeliversEventToSubscriber(t *testing.T) {
	bus := New()
	events, cancel := bus.Subscribe()
	defer cancel()

	bus.Publish(Event{
		Timestamp: time.Now(),
		Level:     LevelInfo,
		Source:    "runtime",
		Type:      "runtime.listening",
		Message:   "listening",
	})

	select {
	case event := <-events:
		if event.ID != 1 {
			t.Fatalf("expected event ID 1, got %d", event.ID)
		}
		if event.Type != "runtime.listening" {
			t.Fatalf("unexpected event type %q", event.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published event")
	}
}
