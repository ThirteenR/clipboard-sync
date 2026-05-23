package sync

import (
	"net"
	"testing"
	"time"
)

func TestPeerManagerAddRemove(t *testing.T) {
	pm := NewPeerManager("local-uuid", func(m Message) {})

	server, client := net.Pipe()
	defer server.Close()

	peer := &Peer{
		UUID:     "peer-uuid",
		Hostname: "test-peer",
		Conn:     client,
	}

	pm.Add(peer)
	if len(pm.List()) != 1 {
		t.Errorf("expected 1 peer, got %d", len(pm.List()))
	}

	pm.Remove("peer-uuid")
	if len(pm.List()) != 0 {
		t.Errorf("expected 0 peers, got %d", len(pm.List()))
	}
}

func TestPeerManagerBroadcast(t *testing.T) {
	received := make(chan Message, 1)
	pm := NewPeerManager("local-uuid", func(m Message) {
		received <- m
	})

	server, client := net.Pipe()
	defer server.Close()

	peer := &Peer{
		UUID:     "peer-uuid",
		Hostname: "test-peer",
		Conn:     client,
	}

	pm.Add(peer)

	// Give readLoop time to start
	time.Sleep(50 * time.Millisecond)

	msg := Message{
		ID:      "msg-1",
		Type:    "clipboard",
		Content: "test broadcast",
		Sender:  "local-uuid",
	}

	// Start reading from server side before broadcasting, because
	// net.Pipe uses unbuffered channels: client.Write blocks until
	// server.Read receives.
	type readResult struct {
		data []byte
		err  error
	}
	serverResult := make(chan readResult, 1)
	go func() {
		data := make([]byte, 4096)
		n, err := server.Read(data)
		serverResult <- readResult{data[:n], err}
	}()

	pm.Broadcast(msg)

	rr := <-serverResult
	if rr.err != nil {
		t.Fatalf("server read failed: %v", rr.err)
	}

	if len(rr.data) <= 4 {
		t.Fatalf("message too short: %d bytes", len(rr.data))
	}

	t.Logf("Broadcast message sent successfully, %d bytes", len(rr.data))
}

func TestPeerManagerIgnoresOwnMessage(t *testing.T) {
	callCh := make(chan struct{}, 10)
	pm := NewPeerManager("local-uuid", func(m Message) {
		callCh <- struct{}{}
	})

	server, client := net.Pipe()
	defer server.Close()

	peer := &Peer{
		UUID:     "peer-uuid",
		Hostname: "test-peer",
		Conn:     server,
	}

	pm.Add(peer)

	// Send a message FROM local-uuid — should be ignored
	msg := Message{
		ID:     "self-msg",
		Type:   "clipboard",
		Sender: "local-uuid",
	}
	data, _ := Encode(msg)
	client.Write(data)

	time.Sleep(50 * time.Millisecond)

	select {
	case <-callCh:
		t.Error("own message should be ignored, got a callback")
	default:
	}

	// Send a message FROM another peer — should trigger callback
	msg2 := Message{
		ID:     "remote-msg",
		Type:   "clipboard",
		Sender: "other-uuid",
	}
	data2, _ := Encode(msg2)
	client.Write(data2)

	select {
	case <-callCh:
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Error("expected callback for remote message, got none")
	}
}

func TestPeerManagerDuplicateUUID(t *testing.T) {
	pm := NewPeerManager("local-uuid", func(m Message) {})

	// Add first peer with UUID "A"
	s1, c1 := net.Pipe()
	defer s1.Close()
	pm.Add(&Peer{UUID: "A", Hostname: "peer1", Conn: c1})

	// Add second peer with same UUID "A"
	s2, c2 := net.Pipe()
	defer s2.Close()
	pm.Add(&Peer{UUID: "A", Hostname: "peer2", Conn: c2})

	time.Sleep(50 * time.Millisecond)

	peers := pm.List()
	if len(peers) != 1 {
		t.Errorf("expected 1 peer after duplicate, got %d: %+v", len(peers), peers)
	}
}
