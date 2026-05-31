package discovery

import (
	"context"
	"encoding/json"
	"fmt"
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
	Alias    string `json:"alias,omitempty"`
}

type peerRecord struct {
	info    PeerInfo
	lastSeen time.Time
}

func multicastRegister(ctx context.Context, instance, uuid string, port int, trustStore interface{ GetDeviceAlias() string }) (*RegisterHandle, error) {
	rCtx, cancel := context.WithCancel(ctx)

	go func() {
		var conn *net.UDPConn
		addr, _ := net.ResolveUDPAddr("udp", multicastAddr)
		msg := heartbeatMsg{
			UUID:     uuid,
			Hostname: instance,
			Port:     port,
			Alias:    trustStore.GetDeviceAlias(),
		}
		data, _ := json.Marshal(msg)
		ticker := time.NewTicker(heartbeatSec * time.Second)
		defer ticker.Stop()
		defer func() {
			if conn != nil {
				conn.Close()
			}
		}()

		conn = newSenderConn(addr)
		if conn == nil {
			log.Printf("multicast heartbeat: failed to create initial socket")
		}

		for {
			select {
			case <-ticker.C:
				if conn == nil {
					conn = newSenderConn(addr)
					if conn == nil {
						continue
					}
				}
				if _, err := conn.Write(data); err != nil {
					log.Printf("multicast heartbeat send error: %v, recreating socket", err)
					conn.Close()
					conn = nil
				}
			case <-rCtx.Done():
				return
			}
		}
	}()

	log.Printf("multicast heartbeat started for %s on %s", instance, multicastAddr)
	return &RegisterHandle{cancel: cancel}, nil
}

func newSenderConn(addr *net.UDPAddr) *net.UDPConn {
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Printf("multicast sender socket creation failed: %v", err)
		return nil
	}
	return conn
}

func newReceiverConn(addr *net.UDPAddr) *net.UDPConn {
	conn, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		log.Printf("multicast receiver socket creation failed: %v", err)
		return nil
	}
	conn.SetReadBuffer(1024 * 1024)
	return conn
}

func multicastDiscover(ctx context.Context, handler Handler) error {
	addr, err := net.ResolveUDPAddr("udp", multicastAddr)
	if err != nil {
		return err
	}

	conn := newReceiverConn(addr)
	if conn == nil {
		return fmt.Errorf("failed to create initial multicast receiver socket")
	}

	peers := make(map[string]peerRecord)
	cleanup := time.NewTicker(peerTTL)
	defer cleanup.Stop()

	buf := make([]byte, 1024)
	lastReceived := time.Now()

	for {
		select {
		case <-ctx.Done():
			conn.Close()
			return nil
		case <-cleanup.C:
			// 清理过期 peer
			now := time.Now()
			for uuid, rec := range peers {
				if now.Sub(rec.lastSeen) > peerTTL {
					delete(peers, uuid)
					if handler.OnLeave != nil {
						handler.OnLeave(rec.info)
					}
				}
			}
			// 检查接收是否超时（socket 可能已死）
			if time.Since(lastReceived) > 15*time.Second {
				log.Printf("multicast receiver: no data for %v, recreating socket", time.Since(lastReceived))
				conn.Close()
				conn = newReceiverConn(addr)
				if conn == nil {
					log.Printf("multicast receiver: socket recreation failed")
				}
				lastReceived = time.Now() // 重置计时器，避免每秒重建
			}
		default:
			if conn == nil {
				time.Sleep(1 * time.Second)
				continue
			}
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, sender, err := conn.ReadFromUDP(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				// 非超时错误：socket 可能已死，重建
				log.Printf("multicast receiver read error: %v, recreating socket", err)
				conn.Close()
				conn = newReceiverConn(addr)
				if conn == nil {
					log.Printf("multicast receiver: socket recreation failed")
				}
				lastReceived = time.Now()
				continue
			}

			lastReceived = time.Now()

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
				displayName := hostname
				if msg.Alias != "" {
					displayName = msg.Alias
				}
				log.Printf("multicast discovered: %s (%s)", displayName, msg.UUID)
				peers[msg.UUID] = peerRecord{info: info, lastSeen: time.Now()}
				if msg.Alias != "" && handler.OnAliasUpdate != nil {
					handler.OnAliasUpdate(msg.UUID, msg.Alias)
				}
				handler.OnJoin(info)
			} else {
				peers[msg.UUID] = peerRecord{info: info, lastSeen: time.Now()}
				if msg.Alias != "" && handler.OnAliasUpdate != nil {
					handler.OnAliasUpdate(msg.UUID, msg.Alias)
				}
			}
		}
	}
}
