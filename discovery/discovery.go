package discovery

import (
	"context"
	"fmt"
	"log"
	"runtime"

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

type RegisterHandle struct {
	cancel context.CancelFunc
}

func (h *RegisterHandle) Shutdown() {
	if h.cancel != nil {
		h.cancel()
	}
}

func Register(ctx context.Context, instance, uuid, host string, port int) (*RegisterHandle, error) {
	log.Printf("Registering mDNS service: %s (%s) on %s:%d", instance, uuid, host, port)
	if runtime.GOOS == "darwin" {
		return registerDarwin(ctx, instance, uuid, port)
	}
	return registerZeroconf(ctx, instance, uuid, host, port)
}

func registerZeroconf(ctx context.Context, instance, uuid, host string, port int) (*RegisterHandle, error) {
	txt := []string{"uuid=" + uuid}
	srv, err := zeroconf.Register(instance, "_clipboardsync._tcp", "local.", port, txt, nil)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-ctx.Done()
		srv.Shutdown()
	}()
	return &RegisterHandle{cancel: cancel}, nil
}

func Discover(ctx context.Context, handler Handler) error {
	if runtime.GOOS == "darwin" {
		return discoverDarwin(ctx, handler)
	}
	return discoverZeroconf(ctx, handler)
}

func discoverZeroconf(ctx context.Context, handler Handler) error {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return err
	}
	log.Printf("mDNS resolver created, browsing for _clipboardsync._tcp...")

	entries := make(chan *zeroconf.ServiceEntry)

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

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
