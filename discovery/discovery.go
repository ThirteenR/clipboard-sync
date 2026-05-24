package discovery

import (
	"context"
	"log"

	"github.com/grandcat/zeroconf"
)

type PeerInfo struct {
	UUID     string
	Hostname string
	Addr     string
	Port     int
}

type Handler struct {
	OnJoin  func(PeerInfo)
	OnLeave func(PeerInfo)
}

func Register(ctx context.Context, instance, uuid, host string, port int) (*zeroconf.Server, error) {
	txt := []string{"uuid=" + uuid}
	log.Printf("Registering mDNS service: %s (%s) on %s:%d", instance, uuid, host, port)
	return zeroconf.Register(instance, "_clipboardsync._tcp", "local.", port, txt, nil)
}

// Discover continuously browses for _clipboardsync._tcp services.
// Blocks until ctx is cancelled. Calls handler.OnJoin as peers appear.
func Discover(ctx context.Context, handler Handler) error {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return err
	}
	log.Printf("mDNS resolver created, browsing for _clipboardsync._tcp...")

	entries := make(chan *zeroconf.ServiceEntry)
	defer close(entries)

	go func() {
		for entry := range entries {
			if entry.Instance == "" {
				log.Printf("mDNS entry skipped (empty instance)")
				continue
			}
			log.Printf("mDNS entry received: instance=%s host=%s addrs=%v port=%d txt=%v",
				entry.Instance, entry.HostName, entry.AddrIPv4, entry.Port, entry.Text)
			uuid := ""
			for _, t := range entry.Text {
				if len(t) > 5 && t[:5] == "uuid=" {
					uuid = t[5:]
				}
			}
			addr := ""
			if len(entry.AddrIPv4) > 0 {
				addr = entry.AddrIPv4[0].String()
			}
			info := PeerInfo{
				UUID:     uuid,
				Hostname: entry.HostName,
				Addr:     addr,
				Port:     entry.Port,
			}

			handler.OnJoin(info)
		}
	}()

	err = resolver.Browse(ctx, "_clipboardsync._tcp", "local.", entries)
	if err != nil {
		return err
	}
	log.Printf("mDNS Browse started successfully")

	<-ctx.Done()
	return nil
}
