//go:build !darwin

package discovery

import "context"

func registerDarwin(ctx context.Context, instance, uuid string, port int, trustStore interface{ GetDeviceAlias() string }) (*RegisterHandle, error) {
	return multicastRegister(ctx, instance, uuid, port, trustStore)
}

func discoverDarwin(ctx context.Context, handler Handler) error {
	return multicastDiscover(ctx, handler)
}
