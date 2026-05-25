package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"clipboard-sync/clipboard"
	"clipboard-sync/dedup"
	"clipboard-sync/discovery"
	"clipboard-sync/sync"
	"clipboard-sync/trust"

	"github.com/google/uuid"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	if len(os.Args) > 1 && os.Args[1] == "uuid" {
		id := loadOrCreateUUID()
		os.Stdout.WriteString(id + "\n")
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "trust" {
		store, err := trust.New()
		if err != nil {
			log.Fatalf("Failed to load trust store: %v", err)
		}
		if len(os.Args) > 2 {
			switch os.Args[2] {
			case "list":
				trust.RunList(store)
			case "add":
				if len(os.Args) < 4 {
					log.Fatal("Usage: clipboardsync trust add <uuid>")
				}
				store.Add(os.Args[3], os.Args[3])
				log.Printf("Added %s to trusted devices", os.Args[3])
			case "remove":
				if len(os.Args) < 4 {
					log.Fatal("Usage: clipboardsync trust remove <uuid>")
				}
				store.Remove(os.Args[3])
				log.Printf("Removed %s from trusted devices", os.Args[3])
			default:
				log.Fatalf("Unknown trust subcommand: %s", os.Args[2])
			}
		} else {
			trust.RunTUI(store)
		}
		return
	}

	log.Println("Clipboard Sync starting...")

	deviceUUID := loadOrCreateUUID()
	log.Printf("Device UUID: %s", deviceUUID)

	trustStore, err := trust.New()
	if err != nil {
		log.Fatalf("Failed to load trust store: %v", err)
	}
	log.Printf("Trust store loaded")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := trustStore.ReloadIfChanged(); err != nil && !os.IsNotExist(err) {
					log.Printf("Trust store reload error: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	deduper := dedup.New(DedupTTL)
	defer deduper.Stop()

	pm := sync.NewPeerManager(deviceUUID, func(msg sync.Message) {
		if !trustStore.IsTrusted(msg.Sender) {
			log.Printf("Skipped clipboard from untrusted device: %s", truncate(msg.Sender, 16))
			return
		}
		if deduper.Seen(msg.Hash) {
			return
		}
		deduper.Mark(msg.Hash)
		log.Printf("Received clipboard from %s: %s", msg.Sender, truncate(msg.Content, 50))
		if err := clipboard.Write(msg.Content); err != nil {
			log.Printf("Failed to write clipboard: %v", err)
		}
	}, 0)

	go func() {
		w := clipboard.New(func(text string) {
			hash := deduper.Hash(text)
			if deduper.Seen(hash) {
				return
			}
			deduper.Mark(hash)
			log.Printf("Local clipboard changed, broadcasting...")
			pm.Broadcast(sync.Message{
				ID:        uuid.New().String(),
				Hash:      hash,
				Type:      "clipboard",
				Content:   text,
				Timestamp: time.Now().UnixMilli(),
				Sender:    deviceUUID,
			})
		})
		if err := w.Start(ctx); err != nil {
			log.Fatalf("Clipboard watcher failed: %v", err)
		}
	}()

	go func() {
		handler := discovery.Handler{
			OnJoin: func(info discovery.PeerInfo) {
				if info.UUID == "" || info.UUID == deviceUUID || info.Addr == "" {
					return
				}
				if pm.Has(info.UUID) {
					return
				}
				// Lower UUID waits for higher UUID to connect.
				// Fall back after 5s if the remote didn't discover us.
				if deviceUUID < info.UUID {
					log.Printf("Discovered %s (%s), waiting for inbound connection", info.Hostname, info.UUID)
					go func(fi discovery.PeerInfo) {
						time.Sleep(5 * time.Second)
						if pm.Has(fi.UUID) {
							return
						}
						log.Printf("Fallback: dialing %s (%s)", fi.Hostname, fi.UUID)
						conn, err := net.DialTimeout("tcp", net.JoinHostPort(fi.Addr, itoa(fi.Port)), 5*time.Second)
						if err != nil {
							log.Printf("Fallback connect to %s failed: %v", fi.Hostname, err)
							return
						}
						greeting := sync.Message{
							ID:     uuid.New().String(),
							Type:   "hello",
							Sender: deviceUUID,
						}
						data, _ := sync.Encode(greeting)
						conn.Write(data)
						pm.Add(&sync.Peer{
							UUID:     fi.UUID,
							Hostname: fi.Hostname,
							Conn:     conn,
						})
					}(info)
					return
				}
				log.Printf("Discovered peer: %s (%s) at %s:%d", info.Hostname, info.UUID, info.Addr, info.Port)

				conn, err := net.DialTimeout("tcp", net.JoinHostPort(info.Addr, itoa(info.Port)), 5*time.Second)
				if err != nil {
					log.Printf("Failed to connect to %s: %v", info.Hostname, err)
					return
				}
				// Send greeting so the receiving side can identify this peer
				greeting := sync.Message{
					ID:     uuid.New().String(),
					Type:   "hello",
					Sender: deviceUUID,
				}
				data, err := sync.Encode(greeting)
				if err == nil {
					conn.Write(data)
				}
				pm.Add(&sync.Peer{
					UUID:     info.UUID,
					Hostname: info.Hostname,
					Conn:     conn,
				})
			},
			OnLeave: func(info discovery.PeerInfo) {
				log.Printf("Peer left: %s (%s)", info.Hostname, info.UUID)
				pm.Remove(info.UUID)
			},
		}

		if err := discovery.Discover(ctx, handler); err != nil {
			log.Printf("Discovery error: %v", err)
		}
	}()

	go func() {
		listener, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", itoa(ServicePort)))
		if err != nil {
			log.Fatalf("Failed to listen: %v", err)
		}
		defer listener.Close()
		log.Printf("TCP server listening on :%d", ServicePort)

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Accept error: %v", err)
				continue
			}
			go func(c net.Conn) {
				msg, err := sync.Decode(c)
				if err != nil {
					log.Printf("Failed to decode greeting: %v", err)
					c.Close()
					return
				}
				if msg.Sender == deviceUUID {
					c.Close()
					return
				}
				if pm.Has(msg.Sender) {
					c.Close()
					return
				}
				pm.Add(&sync.Peer{
					UUID:     msg.Sender,
					Hostname: msg.Sender,
					Conn:     c,
				})
			}(conn)
		}
	}()

	go func() {
		hostname, _ := os.Hostname()
		server, err := discovery.Register(ctx, hostname, deviceUUID, hostname, ServicePort)
		if err != nil {
			log.Fatalf("Failed to register mDNS service: %v", err)
		}
		defer server.Shutdown()
		log.Printf("mDNS service registered as %s", hostname)
		<-ctx.Done()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutting down...")
	cancel()
	time.Sleep(500 * time.Millisecond)
}

func loadOrCreateUUID() string {
	uuidFile := filepath.Join(os.TempDir(), "clipboard-sync-uuid")
	data, err := os.ReadFile(uuidFile)
	if err == nil && len(data) > 0 {
		return strings.TrimSpace(string(data))
	}
	id := uuid.New().String()
	os.WriteFile(uuidFile, []byte(id), 0644)
	return id
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ReplaceAll(s[:n], "\n", " ") + "..."
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
