package discovery

import (
	"context"

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
	return zeroconf.Register(instance, "_clipboardsync._tcp", "local.", port, txt, nil)
}

// Discover continuously browses for _clipboardsync._tcp services.
// Blocks until ctx is cancelled. Calls handler.OnJoin as peers appear.
func Discover(ctx context.Context, handler Handler) error {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for entry := range entries {
			if entry.Instance == "" {
				continue
			}
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
		close(entries)
		return err
	}

	<-ctx.Done()
	return nil
}
