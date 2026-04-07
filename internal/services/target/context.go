package target

import "context"

// ChatContext carries per-prompt metadata through context.Context so that
// session-aware clients can resolve the correct session for each chat
// without changing the Client interface.
type ChatContext struct {
	ChatID string
	Seq    int
}

type chatContextKeyType struct{}

var chatContextKey chatContextKeyType

// WithChatContext attaches chat metadata to a context.
func WithChatContext(ctx context.Context, cc ChatContext) context.Context {
	return context.WithValue(ctx, chatContextKey, cc)
}

func chatContextFrom(ctx context.Context) (ChatContext, bool) {
	cc, ok := ctx.Value(chatContextKey).(ChatContext)
	return cc, ok
}
