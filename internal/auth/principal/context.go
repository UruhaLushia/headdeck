package principal

import "context"

type contextKey struct{}

func WithContext(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

func FromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(contextKey{}).(*Principal)
	return p, ok
}
