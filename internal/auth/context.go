package auth

import "context"

type contextKey string

const userContextKey contextKey = "plomvix_authenticated_user"

func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) *User {
	user, _ := ctx.Value(userContextKey).(*User)
	return user
}

func RequireUser(ctx context.Context) *User {
	user := UserFromContext(ctx)
	if user == nil {
		panic("plomvix: RequireUser called on unprotected route — add auth.Middleware")
	}
	return user
}
