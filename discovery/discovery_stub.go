//go:build !darwin

package discovery

import "context"

func discoverDarwin(ctx context.Context, handler Handler) error {
	return discoverZeroconf(ctx, handler)
}
