//go:build !darwin

package discovery

import "context"

func registerDarwin(ctx context.Context, instance, uuid string, port int) (*RegisterHandle, error) {
	return multicastRegister(ctx, instance, uuid, port)
}

func discoverDarwin(ctx context.Context, handler Handler) error {
	return multicastDiscover(ctx, handler)
}
