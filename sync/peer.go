package sync

import (
	"log"
	"sync"
)

type PeerInfo struct {
	UUID     string
	Hostname string
	Addr     string
	Port     int
}

type PeerManager struct {
	mu    sync.RWMutex
	peers map[string]PeerInfo
}

func NewPeerManager() *PeerManager {
	return &PeerManager{
		peers: make(map[string]PeerInfo),
	}
}

func (pm *PeerManager) Add(info PeerInfo) {
	pm.mu.Lock()
	pm.peers[info.UUID] = info
	pm.mu.Unlock()
	log.Printf("Peer tracked: %s (%s) at %s:%d", info.Hostname, info.UUID, info.Addr, info.Port)
}

func (pm *PeerManager) Remove(uuid string) {
	pm.mu.Lock()
	if p, ok := pm.peers[uuid]; ok {
		delete(pm.peers, uuid)
		log.Printf("Peer removed: %s (%s)", p.Hostname, p.UUID)
	}
	pm.mu.Unlock()
}

func (pm *PeerManager) Has(uuid string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	_, ok := pm.peers[uuid]
	return ok
}

func (pm *PeerManager) Get(uuid string) (PeerInfo, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	p, ok := pm.peers[uuid]
	return p, ok
}

func (pm *PeerManager) All() []PeerInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	list := make([]PeerInfo, 0, len(pm.peers))
	for _, p := range pm.peers {
		list = append(list, p)
	}
	return list
}

func (pm *PeerManager) Len() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.peers)
}
