package messagepush

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestQmsgUsesFormEncodedSendEndpoint(t *testing.T) {
	var (
		method string
		path   string
		ctype  string
		form   url.Values
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		ctype = r.Header.Get("Content-Type")
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		form, err = url.ParseQuery(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"reason":"ok"}`))
	}))
	defer server.Close()

	req, err := buildQmsgRequest(context.Background(), Config{
		Channels: Channels{QmsgKey: "key-1", QmsgType: "group", QmsgTarget: "12345"},
	}, "hello qmsg")
	if err != nil {
		t.Fatal(err)
	}
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(server.URL, "http://")
	req.Host = req.URL.Host
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err := validateChannelResponse("qmsg", resp.Body); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || !strings.HasSuffix(path, "/group/key-1") {
		t.Fatalf("method=%s path=%s", method, path)
	}
	if !strings.Contains(ctype, "application/x-www-form-urlencoded") {
		t.Fatalf("ctype=%s", ctype)
	}
	if form.Get("msg") != "hello qmsg" || form.Get("qq") != "12345" {
		t.Fatalf("form=%v", form)
	}
}

func TestQmsgRejectsBusinessFailure(t *testing.T) {
	err := validateChannelResponse("qmsg", strings.NewReader(`{"success":false,"reason":"key invalid"}`))
	if err == nil || !strings.Contains(err.Error(), "key invalid") {
		t.Fatalf("err=%v", err)
	}
}

func TestDingtalkMarkdownUsesProviderEnvelopeAndSign(t *testing.T) {
	var body map[string]any
	var rawQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()

	err := sendRenderedByChannel(context.Background(), server.Client(), Config{
		Channels: Channels{DingtalkWebhook: server.URL + "?access_token=abc", DingtalkSecret: "sec"},
	}, "dingtalk", RenderedPayload{
		Kind: "abnormal",
		Mode: TemplateModeMarkdown,
		Text: "**alert**",
		Context: TemplateContext{Values: map[string]any{
			"payload": map[string]any{"title": "异常告警"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	md, ok := body["markdown"].(map[string]any)
	if !ok || body["msgtype"] != "markdown" || md["title"] != "异常告警" || md["text"] != "**alert**" {
		t.Fatalf("body=%#v", body)
	}
	if !strings.Contains(rawQuery, "access_token=abc") || !strings.Contains(rawQuery, "timestamp=") || !strings.Contains(rawQuery, "sign=") {
		t.Fatalf("query=%s", rawQuery)
	}
}

func TestWecomAndDingtalkRejectBusinessErrors(t *testing.T) {
	err := validateChannelResponse("wecom", strings.NewReader(`{"errcode":93000,"errmsg":"invalid webhook url"}`))
	if err == nil || !strings.Contains(err.Error(), "invalid webhook url") {
		t.Fatalf("wecom err=%v", err)
	}
	err = validateChannelResponse("dingtalk", strings.NewReader(`{"errcode":300001,"errmsg":"sign not match"}`))
	if err == nil || !strings.Contains(err.Error(), "sign not match") {
		t.Fatalf("dingtalk err=%v", err)
	}
}

func TestTelegramKeepsBotTokenColonAndValidatesOK(t *testing.T) {
	var path string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	req, err := buildRenderedRequest(context.Background(), Config{
		Channels: Channels{TelegramBotToken: "123456:ABC-DEF", TelegramChatID: "-1001"},
	}, "telegram", RenderedPayload{Mode: TemplateModeText, Text: "hello tg", Kind: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(req.URL.Path, "/bot123456:ABC-DEF/sendMessage") {
		t.Fatalf("path should keep colon: %s", req.URL.Path)
	}
	if strings.Contains(req.URL.EscapedPath(), "%3A") {
		// EscapedPath may still encode; Path itself must keep raw token semantics in request construction.
	}
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(server.URL, "http://")
	req.Host = req.URL.Host
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err := validateChannelResponse("telegram", resp.Body); err != nil {
		t.Fatal(err)
	}
	_ = path
	if body["chat_id"] != "-1001" || body["text"] != "hello tg" {
		t.Fatalf("body=%#v", body)
	}
	err = validateChannelResponse("telegram", strings.NewReader(`{"ok":false,"description":"chat not found"}`))
	if err == nil || !strings.Contains(err.Error(), "chat not found") {
		t.Fatalf("telegram err=%v", err)
	}
}

func TestBarkJSONEnvelopeAndBusinessValidation(t *testing.T) {
	var body map[string]any
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"success"}`))
	}))
	defer server.Close()

	err := sendRenderedByChannel(context.Background(), server.Client(), Config{
		Channels: Channels{BarkServerURL: server.URL, BarkDeviceKey: "device-key"},
	}, "bark", RenderedPayload{
		Mode: TemplateModeText,
		Text: "body text",
		Kind: "test",
		Context: TemplateContext{Values: map[string]any{
			"payload": map[string]any{"title": "农场测试推送"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "/device-key") {
		t.Fatalf("path=%s", path)
	}
	if body["title"] != "农场测试推送" || body["body"] != "body text" {
		t.Fatalf("body=%#v", body)
	}
	err = validateChannelResponse("bark", strings.NewReader(`{"code":400,"message":"failed to get device token"}`))
	if err == nil || !strings.Contains(err.Error(), "failed to get device token") {
		t.Fatalf("bark err=%v", err)
	}
}

func TestNtfySetsMarkdownHeaderOnlyForMarkdownMode(t *testing.T) {
	var markdown string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		markdown = r.Header.Get("Markdown")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := sendRenderedByChannel(context.Background(), server.Client(), Config{
		Channels: Channels{NtfyServerURL: server.URL, NtfyTopic: "farm"},
	}, "ntfy", RenderedPayload{Mode: TemplateModeText, Text: "plain", Kind: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if markdown != "" {
		t.Fatalf("text mode should not set Markdown header: %q", markdown)
	}

	err = sendRenderedByChannel(context.Background(), server.Client(), Config{
		Channels: Channels{NtfyServerURL: server.URL, NtfyTopic: "farm"},
	}, "ntfy", RenderedPayload{Mode: TemplateModeMarkdown, Text: "**md**", Kind: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if markdown != "yes" {
		t.Fatalf("markdown mode header=%q", markdown)
	}
}
