package discovery

import (
	"context"
	"log"
	"runtime"
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
	log.Printf("Registering service: %s (%s) on %s:%d", instance, uuid, host, port)
	if runtime.GOOS == "darwin" {
		registerDarwin(ctx, instance, uuid, port)
	}
	return multicastRegister(ctx, instance, uuid, port)
}

func Discover(ctx context.Context, handler Handler) error {
	if runtime.GOOS == "darwin" {
		go discoverDarwin(ctx, handler)
	}
	return multicastDiscover(ctx, handler)
}
