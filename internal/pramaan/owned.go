package pramaan

import "context"

type ownedInboxKey struct{}

// Only the authenticated capability handler marks an address after its workspace registry check.
func WithOwnedInbox(ctx context.Context, address string) context.Context {
	return context.WithValue(ctx, ownedInboxKey{}, address)
}
func OwnedInbox(ctx context.Context, address string) bool {
	return address != "" && ctx.Value(ownedInboxKey{}) == address
}
