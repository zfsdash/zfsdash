package auth

import (
	"context"

	"github.com/zfsdash/zfsdash/internal/db"
)

type contextKey string

const sessionKey contextKey = "session"

// WithSession stores a session in the context.
func WithSession(ctx context.Context, session *db.Session) context.Context {
	return context.WithValue(ctx, sessionKey, session)
}

// SessionFromContext retrieves a session from the context.
func SessionFromContext(ctx context.Context) (*db.Session, bool) {
	session, ok := ctx.Value(sessionKey).(*db.Session)
	return session, ok
}
