package sync

import (
	"encoding/binary"
	"encoding/json"
	"io"
)

type Message struct {
	ID        string `json:"id"`
	Hash      string `json:"hash"`
	Type      string `json:"type"`
	Content   string `json:"content,omitempty"`
	Timestamp int64  `json:"timestamp"`
	Sender    string `json:"sender"`
}

func Encode(msg Message) ([]byte, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 4+len(body))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(body)))
	copy(buf[4:], body)
	return buf, nil
}

func Decode(r io.Reader) (Message, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return Message{}, err
	}
	bodyLen := binary.BigEndian.Uint32(lenBuf[:])
	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(r, body); err != nil {
		return Message{}, err
	}
	var msg Message
	err := json.Unmarshal(body, &msg)
	return msg, err
}
