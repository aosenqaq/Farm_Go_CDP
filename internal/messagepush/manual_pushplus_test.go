package messagepush

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestManualPushPlusSend(t *testing.T) {
	token := os.Getenv("PUSHPLUS_TOKEN")
	if token == "" {
		t.Skip("set PUSHPLUS_TOKEN to send a live PushPlus test")
	}

	service := NewService(ServiceOptions{
		Store:      NewMemoryStore(),
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
	})
	config := NormalizeConfig(Config{
		Enabled:          true,
		SelectedChannels: []string{"pushplus"},
		Channels:         Channels{PushPlusToken: token},
		Templates: map[MessageType]map[string]Template{
			MessageTypeDaily: {
				"pushplus": {
					Enabled: true,
					Mode:    TemplateModeMarkdown,
					Content: "**Farm_Go PushPlus live**\n\nchannel=pushplus\ntime={{event.time}}",
				},
			},
		},
	})

	result, err := service.SendTest(context.Background(), config)
	if err == nil && result.OK {
		return
	}
	t.Fatalf("send failed: result=%#v err=%v", result, err)
}
