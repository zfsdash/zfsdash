package auth

import (
	"context"

	"github.com/zfsdash/zfsdash/internal/db"
)

type contextKey string

const (
	sessionKey contextKey = "session"
	userKey    contextKey = "user"
)

// WithAuth stores session and user in the context.
func WithAuth(ctx context.Context, sess *db.Session, user *db.User) context.Context {
	ctx = context.WithValue(ctx, sessionKey, sess)
	ctx = context.WithValue(ctx, userKey, user)
	return ctx
}

// SessionFromContext retrieves a session from the context.
func SessionFromContext(ctx context.Context) (*db.Session, bool) {
	sess, ok := ctx.Value(sessionKey).(*db.Session)
	return sess, ok
}

// UserFromContext retrieves a user from the context.
func UserFromContext(ctx context.Context) (*db.User, bool) {
	user, ok := ctx.Value(userKey).(*db.User)
	return user, ok
}
