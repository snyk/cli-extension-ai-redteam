package target

import "context"

type sessionIDKeyType struct{}

var sessionIDKey sessionIDKeyType

// WithSessionID attaches a session ID to a context so the HTTP client
// can inject it into request templates and headers.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDKey, id)
}

func sessionIDFrom(ctx context.Context) string {
	id, ok := ctx.Value(sessionIDKey).(string)
	if !ok {
		return ""
	}
	return id
}
