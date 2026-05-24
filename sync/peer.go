package sync

import (
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type Peer struct {
	UUID     string
	Hostname string
	Conn     net.Conn
}

type PeerManager struct {
	mu          sync.RWMutex
	peers       map[string]*Peer
	localUUID   string
	onMessage   func(Message)
	readTimeout time.Duration
}

func NewPeerManager(localUUID string, onMessage func(Message), readTimeout time.Duration) *PeerManager {
	return &PeerManager{
		peers:       make(map[string]*Peer),
		localUUID:   localUUID,
		onMessage:   onMessage,
		readTimeout: readTimeout,
	}
}

func (pm *PeerManager) Add(peer *Peer) {
	if tcpConn, ok := peer.Conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	pm.mu.Lock()
	if existing, ok := pm.peers[peer.UUID]; ok {
		existing.Conn.Close()
	}
	pm.peers[peer.UUID] = peer
	// Release lock before starting goroutine to prevent deadlock
	// if readLoop returns immediately (e.g. connection already closed)
	pm.mu.Unlock()

	go pm.readLoop(peer)
	log.Printf("Peer connected: %s (%s)", peer.Hostname, peer.UUID)
}

func (pm *PeerManager) Remove(uuid string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if p, ok := pm.peers[uuid]; ok {
		p.Conn.Close()
		delete(pm.peers, uuid)
		log.Printf("Peer disconnected: %s", p.Hostname)
	}
}

func (pm *PeerManager) Has(uuid string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	_, ok := pm.peers[uuid]
	return ok
}

func (pm *PeerManager) Broadcast(msg Message) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	data, err := Encode(msg)
	if err != nil {
		log.Printf("Encode broadcast failed: %v", err)
		return
	}
	for _, p := range pm.peers {
		if err := p.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			log.Printf("SetWriteDeadline to %s failed: %v", p.Hostname, err)
			continue
		}
		if _, err := p.Conn.Write(data); err != nil {
			log.Printf("Write to %s failed: %v", p.Hostname, err)
		}
	}
}

func (pm *PeerManager) readLoop(peer *Peer) {
	defer pm.disconnectPeer(peer)
	for {
		if pm.readTimeout > 0 {
			if err := peer.Conn.SetReadDeadline(time.Now().Add(pm.readTimeout)); err != nil {
				log.Printf("SetReadDeadline for %s failed: %v", peer.Hostname, err)
				return
			}
		}
		msg, err := Decode(peer.Conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("Read from %s failed: %v", peer.Hostname, err)
			}
			return
		}
		if msg.Sender == pm.localUUID {
			continue
		}
		pm.onMessage(msg)
	}
}

// disconnectPeer removes the peer from the map only if it matches the given pointer.
// This prevents a stale readLoop goroutine from removing a newer peer with the same UUID.
func (pm *PeerManager) disconnectPeer(peer *Peer) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if p, ok := pm.peers[peer.UUID]; ok && p == peer {
		p.Conn.Close()
		delete(pm.peers, peer.UUID)
		log.Printf("Peer disconnected: %s", peer.Hostname)
	}
}

func (pm *PeerManager) List() []*Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	list := make([]*Peer, 0, len(pm.peers))
	for _, p := range pm.peers {
		list = append(list, p)
	}
	return list
}
