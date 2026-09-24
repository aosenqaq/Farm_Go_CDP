package messagepush

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPushPlusRenderedPayloadUsesTxtAndMarkdownTemplates(t *testing.T) {
	cases := []struct {
		name         string
		mode         TemplateMode
		channelFmt   string
		wantTemplate string
	}{
		{name: "text_mode", mode: TemplateModeText, wantTemplate: "txt"},
		{name: "markdown_mode", mode: TemplateModeMarkdown, wantTemplate: "markdown"},
		{name: "legacy_channel_format_markdown", mode: TemplateModeText, channelFmt: "markdown", wantTemplate: "markdown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/send" {
					t.Fatalf("path=%s", r.URL.Path)
				}
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Fatal(err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"code":200,"msg":"请求成功","data":"ok"}`))
			}))
			defer server.Close()

			req, err := buildRenderedRequest(context.Background(), Config{
				ChannelFormats: map[string]string{"pushplus": tc.channelFmt},
				Channels:       Channels{PushPlusToken: "token-abc"},
			}, "pushplus", RenderedPayload{
				Kind: "abnormal",
				Mode: tc.mode,
				Text: "content-body",
				Context: TemplateContext{Values: map[string]any{
					"payload": map[string]any{"title": "农场测试推送"},
				}},
			})
			if err != nil {
				t.Fatal(err)
			}
			// Rewrite host to the local mock while keeping official path semantics.
			req.URL.Scheme = "http"
			req.URL.Host = strings.TrimPrefix(server.URL, "http://")
			req.RequestURI = ""

			resp, err := server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if err := validateChannelResponse("pushplus", resp.Body); err != nil {
				t.Fatal(err)
			}
			if body["token"] != "token-abc" || body["title"] != "农场测试推送" || body["content"] != "content-body" || body["template"] != tc.wantTemplate {
				t.Fatalf("body=%#v wantTemplate=%s", body, tc.wantTemplate)
			}
		})
	}
}

func TestPushPlusRejectsBusinessErrorEvenOnHTTP200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":500,"msg":"token invalid"}`))
	}))
	defer server.Close()

	// Directly exercise response validation first.
	err := validateChannelResponse("pushplus", strings.NewReader(`{"code":500,"msg":"token invalid"}`))
	if err == nil || !strings.Contains(err.Error(), "token invalid") {
		t.Fatalf("validate err=%v", err)
	}

	// And full send path against mocked host by overriding request builder target via transport rewrite.
	client := &http.Client{Transport: rewriteHostRoundTripper{base: server.Client().Transport, host: strings.TrimPrefix(server.URL, "http://")}}
	err = sendRenderedByChannel(context.Background(), client, Config{
		Channels: Channels{PushPlusToken: "bad-token"},
	}, "pushplus", RenderedPayload{Mode: TemplateModeText, Text: "hello", Kind: "test"})
	if err == nil || !strings.Contains(err.Error(), "token invalid") {
		t.Fatalf("send err=%v", err)
	}
}

func TestPushPlusTemplateHelper(t *testing.T) {
	if got := pushPlusTemplate(TemplateModeText, ""); got != "txt" {
		t.Fatalf("got=%q", got)
	}
	if got := pushPlusTemplate(TemplateModeMarkdown, "text"); got != "markdown" {
		t.Fatalf("got=%q", got)
	}
	if got := pushPlusTemplate(TemplateModeText, "markdown"); got != "markdown" {
		t.Fatalf("got=%q", got)
	}
}

type rewriteHostRoundTripper struct {
	base http.RoundTripper
	host string
}

func (r rewriteHostRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = r.host
	clone.Host = r.host
	base := r.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
}
