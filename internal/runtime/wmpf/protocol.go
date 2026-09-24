package wmpf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const debugCompressZlib = 1

type DebugMessage struct {
	Seq           uint64 `json:"seq,omitempty"`
	Category      string `json:"category"`
	Payload       string `json:"payload"`
	JSContextID   string `json:"jscontextId,omitempty"`
	JSContextName string `json:"jscontextName,omitempty"`
	OpID          uint64 `json:"opId,omitempty"`
}

func EncodeDebugMessage(message DebugMessage) []byte {
	inner := encodeDebugPayload(message)
	seq := message.Seq
	if seq == 0 {
		seq = 1
	}
	var out []byte
	out = appendVarintField(out, 1, seq)
	out = appendStringField(out, 3, message.Category)
	out = appendBytesField(out, 4, inner)
	out = appendVarintField(out, 5, 0)
	out = appendVarintField(out, 6, uint64(len(inner)))
	return out
}

func DecodeDebugMessage(data []byte) (DebugMessage, error) {
	if len(data) > 0 && data[0] == '{' {
		var message DebugMessage
		err := json.Unmarshal(data, &message)
		return message, err
	}

	fields, err := decodeProtoFields(data)
	if err != nil {
		return DebugMessage{}, err
	}
	category := string(fields[3].bytes)
	payload := fields[4].bytes
	if fields[5].number&debugCompressZlib != 0 {
		payload, err = inflateDebugPayload(payload)
		if err != nil {
			return DebugMessage{}, err
		}
	}
	inner, err := decodeProtoFields(payload)
	if err != nil {
		return DebugMessage{}, err
	}
	message := DebugMessage{Seq: fields[1].number, Category: category}
	switch category {
	case "addJsContext":
		message.JSContextID = string(inner[1].bytes)
		message.JSContextName = string(inner[2].bytes)
	case "removeJsContext", "connectJsContext":
		message.JSContextID = string(inner[1].bytes)
	default:
		message.OpID = inner[1].number
		message.Payload = string(inner[2].bytes)
		message.JSContextID = string(inner[3].bytes)
	}
	return message, nil
}

func encodeDebugPayload(message DebugMessage) []byte {
	switch message.Category {
	case "addJsContext":
		var out []byte
		out = appendStringField(out, 1, message.JSContextID)
		out = appendStringField(out, 2, message.JSContextName)
		return out
	case "removeJsContext", "connectJsContext":
		var out []byte
		out = appendStringField(out, 1, message.JSContextID)
		return out
	default:
		return encodeChromeDevtoolsPayload(message)
	}
}

func inflateDebugPayload(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("inflate debug payload: %w", err)
	}
	defer reader.Close()
	inflated, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("inflate debug payload: %w", err)
	}
	return inflated, nil
}

func encodeChromeDevtoolsPayload(message DebugMessage) []byte {
	var out []byte
	out = appendVarintField(out, 1, message.OpID)
	out = appendStringField(out, 2, message.Payload)
	out = appendStringField(out, 3, message.JSContextID)
	return out
}

type protoField struct {
	number uint64
	bytes  []byte
}

func decodeProtoFields(data []byte) (map[int]protoField, error) {
	fields := map[int]protoField{}
	for len(data) > 0 {
		tag, n := binary.Uvarint(data)
		if n <= 0 {
			return nil, fmt.Errorf("invalid protobuf tag")
		}
		data = data[n:]
		fieldNo := int(tag >> 3)
		wireType := tag & 7
		switch wireType {
		case 0:
			value, n := binary.Uvarint(data)
			if n <= 0 {
				return nil, fmt.Errorf("invalid protobuf varint")
			}
			data = data[n:]
			fields[fieldNo] = protoField{number: value}
		case 2:
			size, n := binary.Uvarint(data)
			if n <= 0 {
				return nil, fmt.Errorf("invalid protobuf length")
			}
			data = data[n:]
			if uint64(len(data)) < size {
				return nil, io.ErrUnexpectedEOF
			}
			fields[fieldNo] = protoField{bytes: data[:size]}
			data = data[size:]
		default:
			return nil, fmt.Errorf("unsupported protobuf wire type %d", wireType)
		}
	}
	return fields, nil
}

func appendVarintField(out []byte, field int, value uint64) []byte {
	out = binary.AppendUvarint(out, uint64(field<<3))
	return binary.AppendUvarint(out, value)
}

func appendStringField(out []byte, field int, value string) []byte {
	return appendBytesField(out, field, []byte(value))
}

func appendBytesField(out []byte, field int, value []byte) []byte {
	out = binary.AppendUvarint(out, uint64(field<<3|2))
	out = binary.AppendUvarint(out, uint64(len(value)))
	return append(out, value...)
}
