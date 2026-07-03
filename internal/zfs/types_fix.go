package zfs

import "context"

// contextKey is unexported to avoid collisions.
type contextKey struct{}

// ensure Collector interface uses context (import used)
var _ = context.Background
