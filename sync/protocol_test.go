package sync

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeClipboard(t *testing.T) {
	orig := Message{
		ID:        "test-id",
		Hash:      "abcdef",
		Type:      "clipboard",
		Content:   "hello world",
		Timestamp: 1234567890,
		Sender:    "device-uuid",
	}

	data, err := Encode(orig)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.ID != orig.ID {
		t.Errorf("ID: got %s, want %s", decoded.ID, orig.ID)
	}
	if decoded.Content != orig.Content {
		t.Errorf("Content: got %s, want %s", decoded.Content, orig.Content)
	}
	if decoded.Sender != orig.Sender {
		t.Errorf("Sender: got %s, want %s", decoded.Sender, orig.Sender)
	}
}

func TestEncodeDecodePing(t *testing.T) {
	orig := Message{
		ID:     "ping-id",
		Type:   "ping",
		Sender: "device-uuid",
	}

	data, err := Encode(orig)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Type != "ping" {
		t.Errorf("Type: got %s, want ping", decoded.Type)
	}
}

func TestDecodeInvalidData(t *testing.T) {
	r := bytes.NewReader([]byte{0, 0, 0, 5, 0x01, 0x02, 0x03, 0x04, 0x05})
	_, err := Decode(r)
	if err == nil {
		t.Error("expected error decoding invalid JSON, got nil")
	}
}

func TestEncodeLengthPrefix(t *testing.T) {
	msg := Message{
		ID:      "test",
		Type:    "clipboard",
		Content: "hello",
		Sender:  "dev",
	}

	data, err := Encode(msg)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	if len(data) < 4 {
		t.Fatal("encoded data too short")
	}

	// First 4 bytes should be the body length = len(data) - 4
	bodyLen := int(data[0])<<24 | int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	if bodyLen != len(data)-4 {
		t.Errorf("length prefix: got %d, want %d", bodyLen, len(data)-4)
	}
}
