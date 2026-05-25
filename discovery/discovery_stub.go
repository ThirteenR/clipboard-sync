//go:build !darwin

package discovery

import "context"

func registerDarwin(ctx context.Context, instance, uuid string, port int) (*RegisterHandle, error) {
	return registerZeroconf(ctx, instance, uuid, "", port)
}

func discoverDarwin(ctx context.Context, handler Handler) error {
	return discoverZeroconf(ctx, handler)
}
