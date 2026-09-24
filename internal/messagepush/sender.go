package messagepush

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func sendByChannel(ctx context.Context, client *http.Client, config Config, channelType string, payload Payload) error {
	config = NormalizeConfig(config)
	if client == nil {
		client = http.DefaultClient
	}
	req, err := buildRequest(ctx, config, channelType, payload)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%s 返回 HTTP %d", ChannelLabels[channelType], resp.StatusCode)
	}
	return validateChannelResponse(channelType, resp.Body)
}

func sendRenderedByChannel(ctx context.Context, client *http.Client, config Config, channelType string, payload RenderedPayload) error {
	config = NormalizeConfig(config)
	channelType = strings.ToLower(strings.TrimSpace(channelType))
	if !supportsTemplateMode(channelType, payload.Mode) {
		return fmt.Errorf("渠道 %s 不支持 %s 模板", channelType, payload.Mode)
	}
	if client == nil {
		client = http.DefaultClient
	}
	req, err := buildRenderedRequest(ctx, config, channelType, payload)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%s 返回 HTTP %d", ChannelLabels[channelType], resp.StatusCode)
	}
	return validateChannelResponse(channelType, resp.Body)
}

func validateChannelResponse(channelType string, body io.Reader) error {
	raw, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	switch channelType {
	case "feishu":
		var result struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if err := json.Unmarshal(raw, &result); err != nil || result.Code == 0 {
			return nil
		}
		if strings.TrimSpace(result.Msg) == "" {
			return fmt.Errorf("飞书返回业务错误 %d", result.Code)
		}
		return fmt.Errorf("飞书返回业务错误 %d: %s", result.Code, result.Msg)
	case "pushplus":
		var result struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil
		}
		if result.Code == 200 {
			return nil
		}
		if strings.TrimSpace(result.Msg) == "" {
			return fmt.Errorf("PushPlus 返回业务错误 %d", result.Code)
		}
		return fmt.Errorf("PushPlus 返回业务错误 %d: %s", result.Code, result.Msg)
	case "wecom", "dingtalk":
		var result struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil
		}
		if result.ErrCode == 0 {
			return nil
		}
		label := ChannelLabels[channelType]
		if label == "" {
			label = channelType
		}
		if strings.TrimSpace(result.ErrMsg) == "" {
			return fmt.Errorf("%s 返回业务错误 %d", label, result.ErrCode)
		}
		return fmt.Errorf("%s 返回业务错误 %d: %s", label, result.ErrCode, result.ErrMsg)
	case "qmsg":
		var result struct {
			Success bool   `json:"success"`
			Reason  string `json:"reason"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil
		}
		if result.Success {
			return nil
		}
		if strings.TrimSpace(result.Reason) == "" {
			return errors.New("Qmsg酱返回业务失败")
		}
		return fmt.Errorf("Qmsg酱返回业务失败: %s", result.Reason)
	case "telegram":
		var result struct {
			OK          bool   `json:"ok"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil
		}
		if result.OK {
			return nil
		}
		if strings.TrimSpace(result.Description) == "" {
			return errors.New("Telegram 返回业务失败")
		}
		return fmt.Errorf("Telegram 返回业务失败: %s", result.Description)
	case "bark":
		var result struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil
		}
		if result.Code == 200 {
			return nil
		}
		if strings.TrimSpace(result.Message) == "" {
			return fmt.Errorf("Bark 返回业务错误 %d", result.Code)
		}
		return fmt.Errorf("Bark 返回业务错误 %d: %s", result.Code, result.Message)
	default:
		return nil
	}
}

func buildRequest(ctx context.Context, config Config, channelType string, payload Payload) (*http.Request, error) {
	if channelType == "webhook" {
		return buildWebhookRequest(ctx, config, payload)
	}
	return buildRenderedRequest(ctx, config, channelType, legacyRenderedPayload(payload))
}

func buildRenderedRequest(ctx context.Context, config Config, channelType string, payload RenderedPayload) (*http.Request, error) {
	if payload.Mode == TemplateModeCard || payload.Mode == TemplateModeJSON {
		return buildStructuredRequest(ctx, config, channelType, payload)
	}
	text := payload.Text
	if title := renderedPayloadTitle(payload); title != "" && isLegacyRenderedPayload(payload) {
		text = title + "\n" + text
	}
	switch channelType {
	case "webhook":
		return buildWebhookRequest(ctx, config, Payload{Kind: payload.Kind, Lines: []string{text}})
	case "serverchan":
		return buildJSONRequest(ctx, http.MethodPost, appendPath("https://sctapi.ftqq.com", config.Channels.ServerChanSendKey+".send"), map[string]any{
			"title": renderedPayloadTitle(payload, payload.Kind),
			"desp":  text,
		})
	case "pushplus":
		return buildJSONRequest(ctx, http.MethodPost, "https://www.pushplus.plus/send", map[string]any{
			"token":    config.Channels.PushPlusToken,
			"title":    renderedPayloadTitle(payload, payload.Kind),
			"content":  text,
			"template": pushPlusTemplate(payload.Mode, config.ChannelFormats["pushplus"]),
		})
	case "qmsg":
		return buildQmsgRequest(ctx, config, text)
	case "wecom":
		if payload.Mode == TemplateModeMarkdown {
			return buildJSONRequest(ctx, http.MethodPost, config.Channels.WecomWebhook, map[string]any{
				"msgtype":  "markdown",
				"markdown": map[string]any{"content": text},
			})
		}
		return buildJSONRequest(ctx, http.MethodPost, config.Channels.WecomWebhook, map[string]any{
			"msgtype": "text",
			"text":    map[string]any{"content": text},
		})
	case "dingtalk":
		webhook := signedDingtalkWebhook(config.Channels.DingtalkWebhook, config.Channels.DingtalkSecret)
		if payload.Mode == TemplateModeMarkdown {
			return buildJSONRequest(ctx, http.MethodPost, webhook, map[string]any{
				"msgtype": "markdown",
				"markdown": map[string]any{
					"title": renderedPayloadTitle(payload, payload.Kind),
					"text":  text,
				},
			})
		}
		return buildJSONRequest(ctx, http.MethodPost, webhook, map[string]any{
			"msgtype": "text",
			"text":    map[string]any{"content": text},
		})
	case "feishu":
		return buildJSONRequest(ctx, http.MethodPost, config.Channels.FeishuWebhook, map[string]any{
			"msg_type": "text",
			"content":  map[string]any{"text": text},
		})
	case "telegram":
		// Bot tokens contain ":" and must stay unescaped in the path.
		return buildJSONRequest(ctx, http.MethodPost, "https://api.telegram.org/bot"+strings.TrimSpace(config.Channels.TelegramBotToken)+"/sendMessage", map[string]any{
			"chat_id": config.Channels.TelegramChatID,
			"text":    text,
		})
	case "bark":
		return buildJSONRequest(ctx, http.MethodPost, appendPath(config.Channels.BarkServerURL, config.Channels.BarkDeviceKey), map[string]any{
			"title": renderedPayloadTitle(payload, payload.Kind),
			"body":  text,
		})
	case "ntfy":
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, appendPath(config.Channels.NtfyServerURL, config.Channels.NtfyTopic), strings.NewReader(text))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Title", renderedPayloadTitle(payload, payload.Kind))
		if payload.Mode == TemplateModeMarkdown {
			req.Header.Set("Markdown", "yes")
		}
		return req, nil
	default:
		return nil, fmt.Errorf("不支持的推送渠道：%s", channelType)
	}
}

func buildStructuredRequest(ctx context.Context, config Config, channelType string, payload RenderedPayload) (*http.Request, error) {
	switch channelType {
	case "webhook":
		return buildWebhookJSONRequest(ctx, config, payload.JSON)
	case "wecom":
		return buildJSONRequest(ctx, http.MethodPost, config.Channels.WecomWebhook, payload.JSON)
	case "dingtalk":
		return buildJSONRequest(ctx, http.MethodPost, signedDingtalkWebhook(config.Channels.DingtalkWebhook, config.Channels.DingtalkSecret), payload.JSON)
	case "feishu":
		return buildJSONRequest(ctx, http.MethodPost, config.Channels.FeishuWebhook, payload.JSON)
	default:
		return nil, fmt.Errorf("渠道 %s 不支持 %s 模板", channelType, payload.Mode)
	}
}

func buildWebhookRequest(ctx context.Context, config Config, payload Payload) (*http.Request, error) {
	body := map[string]any{
		"kind":  payload.Kind,
		"title": payload.Title,
		"lines": payload.Lines,
		"text":  payloadText(payload),
		"meta":  payload.Meta,
	}
	return buildWebhookJSONRequest(ctx, config, body)
}

func buildWebhookJSONRequest(ctx context.Context, config Config, body map[string]any) (*http.Request, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, config.Channels.WebhookMethod, config.Channels.WebhookURL, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(config.Channels.WebhookHeaders) == "" {
		return req, nil
	}
	var headers map[string]any
	if err := json.Unmarshal([]byte(config.Channels.WebhookHeaders), &headers); err != nil {
		return nil, errors.New("请求头 JSON 无效")
	}
	for key, value := range headers {
		text, ok := value.(string)
		if !ok {
			return nil, errors.New("请求头 JSON 无效：值必须是字符串")
		}
		if strings.TrimSpace(key) != "" {
			req.Header.Set(key, text)
		}
	}
	return req, nil
}

func legacyRenderedPayload(payload Payload) RenderedPayload {
	return RenderedPayload{
		Kind: payload.Kind,
		Mode: TemplateModeText,
		Text: payloadText(payload),
		Context: TemplateContext{Values: map[string]any{
			"payload":       map[string]any{"title": payload.Title},
			"legacyPayload": true,
		}},
	}
}

func isLegacyRenderedPayload(payload RenderedPayload) bool {
	legacy, _ := payload.Context.Values["legacyPayload"].(bool)
	return legacy
}

func renderedPayloadTitle(payload RenderedPayload, fallback ...string) string {
	if values, ok := payload.Context.Values["payload"].(map[string]any); ok {
		if title, ok := values["title"].(string); ok {
			return title
		}
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ""
}

func pushPlusTemplate(mode TemplateMode, channelFormat string) string {
	if mode == TemplateModeMarkdown || strings.EqualFold(strings.TrimSpace(channelFormat), "markdown") {
		return "markdown"
	}
	return "txt"
}

func buildQmsgRequest(ctx context.Context, config Config, text string) (*http.Request, error) {
	targetType := strings.ToLower(strings.TrimSpace(config.Channels.QmsgType))
	switch targetType {
	case "", "send", "private":
		targetType = "send"
	case "group":
		// group endpoint
	default:
		targetType = "send"
	}
	form := url.Values{}
	form.Set("msg", text)
	if qq := strings.TrimSpace(config.Channels.QmsgTarget); qq != "" {
		form.Set("qq", qq)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, appendPath("https://qmsg.zendee.cn", targetType, config.Channels.QmsgKey), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func buildJSONRequest(ctx context.Context, method string, target string, body map[string]any) (*http.Request, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func signedDingtalkWebhook(webhook string, secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return webhook
	}
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "\n" + secret))
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	separator := "?"
	if strings.Contains(webhook, "?") {
		separator = "&"
	}
	return webhook + separator + "timestamp=" + timestamp + "&sign=" + sign
}

func payloadText(payload Payload) string {
	if len(payload.Lines) == 0 {
		return payload.Title
	}
	return strings.Join(payload.Lines, "\n")
}
