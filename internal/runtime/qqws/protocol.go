package qqws

type Packet struct {
	ID      string         `json:"id,omitempty"`
	Type    string         `json:"type"`
	TS      int64          `json:"ts,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

type Message = Packet

type RPCError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	MessageHello    = "hello"
	MessageHelloAck = "helloAck"
	MessageCall     = "call"
	MessageResult   = "result"
	MessageEvent    = "event"
	MessageLog      = "log"
	MessageError    = "error"
	MessagePing     = "ping"
	MessagePong     = "pong"
)
