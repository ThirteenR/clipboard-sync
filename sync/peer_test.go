package sync

import (
	"testing"
)

func TestPeerManagerAddRemove(t *testing.T) {
	pm := NewPeerManager()

	pm.Add(PeerInfo{UUID: "peer-uuid", Hostname: "test-peer", Addr: "192.168.1.1", Port: 8920})
	if pm.Len() != 1 {
		t.Errorf("expected 1 peer, got %d", pm.Len())
	}

	pm.Remove("peer-uuid")
	if pm.Len() != 0 {
		t.Errorf("expected 0 peers, got %d", pm.Len())
	}
}

func TestPeerManagerHas(t *testing.T) {
	pm := NewPeerManager()

	if pm.Has("nonexistent") {
		t.Error("expected Has to return false for nonexistent peer")
	}

	pm.Add(PeerInfo{UUID: "peer-uuid", Hostname: "test-peer", Addr: "192.168.1.1", Port: 8920})
	if !pm.Has("peer-uuid") {
		t.Error("expected Has to return true for existing peer")
	}
}

func TestPeerManagerGet(t *testing.T) {
	pm := NewPeerManager()

	pm.Add(PeerInfo{UUID: "peer-uuid", Hostname: "test-peer", Addr: "192.168.1.1", Port: 8920})
	info, ok := pm.Get("peer-uuid")
	if !ok {
		t.Fatal("expected Get to return true")
	}
	if info.Hostname != "test-peer" {
		t.Errorf("expected hostname 'test-peer', got '%s'", info.Hostname)
	}
	if info.Addr != "192.168.1.1" {
		t.Errorf("expected addr '192.168.1.1', got '%s'", info.Addr)
	}
}

func TestPeerManagerAll(t *testing.T) {
	pm := NewPeerManager()

	pm.Add(PeerInfo{UUID: "a", Hostname: "peer-a", Addr: "10.0.0.1", Port: 8920})
	pm.Add(PeerInfo{UUID: "b", Hostname: "peer-b", Addr: "10.0.0.2", Port: 8920})

	all := pm.All()
	if len(all) != 2 {
		t.Errorf("expected 2 peers, got %d", len(all))
	}
}

func TestPeerManagerDuplicateUUID(t *testing.T) {
	pm := NewPeerManager()

	pm.Add(PeerInfo{UUID: "A", Hostname: "peer1", Addr: "10.0.0.1", Port: 8920})
	pm.Add(PeerInfo{UUID: "A", Hostname: "peer2", Addr: "10.0.0.2", Port: 8920})

	if pm.Len() != 1 {
		t.Errorf("expected 1 peer after duplicate, got %d", pm.Len())
	}

	info, _ := pm.Get("A")
	if info.Hostname != "peer2" {
		t.Errorf("expected hostname 'peer2' (latest), got '%s'", info.Hostname)
	}
}
