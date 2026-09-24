package wmpf

import (
	"bytes"
	"compress/zlib"
	"testing"
)

func TestDecodeDebugMessageInflatesZlibData(t *testing.T) {
	inner := encodeDebugPayload(DebugMessage{
		Category:      "addJsContext",
		JSContextID:   "ctx-1",
		JSContextName: "gameContext",
	})

	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(inner); err != nil {
		t.Fatalf("compress inner debug payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zlib writer: %v", err)
	}

	var raw []byte
	raw = appendVarintField(raw, 1, 7)
	raw = appendStringField(raw, 3, "addJsContext")
	raw = appendBytesField(raw, 4, compressed.Bytes())
	raw = appendVarintField(raw, 5, 1)
	raw = appendVarintField(raw, 6, uint64(len(inner)))

	message, err := DecodeDebugMessage(raw)
	if err != nil {
		t.Fatalf("decode compressed debug message: %v", err)
	}
	if message.Seq != 7 || message.Category != "addJsContext" || message.JSContextID != "ctx-1" || message.JSContextName != "gameContext" {
		t.Fatalf("unexpected debug message %#v", message)
	}
}

func TestDecodeDebugMessageAcceptsJSONCamelCaseJSContextFields(t *testing.T) {
	message, err := DecodeDebugMessage([]byte(`{
		"seq": 9,
		"category": "addJsContext",
		"jsContextId": "ctx-json",
		"jsContextName": "gameContext"
	}`))
	if err != nil {
		t.Fatalf("decode json debug message: %v", err)
	}
	if message.JSContextID != "ctx-json" || message.JSContextName != "gameContext" {
		t.Fatalf("unexpected json debug message %#v", message)
	}
}
