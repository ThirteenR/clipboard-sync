package discovery

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"time"
)

const (
	multicastAddr = "239.255.0.42:8921"
	heartbeatSec  = 5
	peerTTL       = 15 * time.Second
)

type heartbeatMsg struct {
	UUID     string `json:"uuid"`
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
}

type peerRecord struct {
	info    PeerInfo
	lastSeen time.Time
}

func multicastRegister(ctx context.Context, instance, uuid string, port int) (*RegisterHandle, error) {
	addr, err := net.ResolveUDPAddr("udp", multicastAddr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}

	rCtx, cancel := context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(heartbeatSec * time.Second)
		defer ticker.Stop()
		defer conn.Close()

		msg := heartbeatMsg{UUID: uuid, Hostname: instance, Port: port}
		data, _ := json.Marshal(msg)

		for {
			select {
			case <-ticker.C:
				conn.Write(data)
			case <-rCtx.Done():
				return
			}
		}
	}()

	log.Printf("multicast heartbeat started for %s on %s", instance, multicastAddr)
	return &RegisterHandle{cancel: cancel}, nil
}

func multicastDiscover(ctx context.Context, handler Handler) error {
	addr, err := net.ResolveUDPAddr("udp", multicastAddr)
	if err != nil {
		return err
	}

	conn, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		return err
	}

	peers := make(map[string]peerRecord)
	cleanup := time.NewTicker(peerTTL)
	defer cleanup.Stop()

	buf := make([]byte, 1024)

	for {
		select {
		case <-ctx.Done():
			conn.Close()
			return nil
		case <-cleanup.C:
			now := time.Now()
			for uuid, rec := range peers {
				if now.Sub(rec.lastSeen) > peerTTL {
					delete(peers, uuid)
					if handler.OnLeave != nil {
						handler.OnLeave(rec.info)
					}
				}
			}
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, sender, err := conn.ReadFromUDP(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				continue
			}

			var msg heartbeatMsg
			if err := json.Unmarshal(buf[:n], &msg); err != nil {
				continue
			}
			if msg.UUID == "" {
				continue
			}

			hostname := msg.Hostname
			if hostname == "" {
				hostname = msg.UUID
			}

			info := PeerInfo{
				UUID:     msg.UUID,
				Hostname: hostname,
				Addr:     sender.IP.String(),
				Port:     msg.Port,
			}

			if _, exists := peers[msg.UUID]; !exists {
				log.Printf("multicast discovered: %s (%s)", hostname, msg.UUID)
				peers[msg.UUID] = peerRecord{info: info, lastSeen: time.Now()}
				handler.OnJoin(info)
			} else {
				peers[msg.UUID] = peerRecord{info: info, lastSeen: time.Now()}
			}
		}
	}
}
