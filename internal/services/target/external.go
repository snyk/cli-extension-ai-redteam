package target

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

// ChatContext carries per-prompt metadata through context.Context so that
// ExternalClient can include chat_id and seq in the binary's stdin without
// changing the Client interface.
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

// externalRequest is the JSON sent to the binary's stdin.
type externalRequest struct {
	ChatID string `json:"chat_id,omitempty"`
	Seq    int    `json:"seq,omitempty"`
	Prompt string `json:"prompt"`
}

// externalResponse is the JSON expected on the binary's stdout.
type externalResponse struct {
	Response string `json:"response"`
}

// ExternalClient implements Client by shelling out to an external binary
// for the entire send/receive cycle.
type ExternalClient struct {
	cmd *utils.ExternalCommand
}

// NewExternalClient creates a client that delegates to an external binary.
func NewExternalClient(cmd *utils.ExternalCommand) *ExternalClient {
	return &ExternalClient{cmd: cmd}
}

var _ Client = (*ExternalClient)(nil)

func (c *ExternalClient) SendPrompt(ctx context.Context, prompt string) (string, error) {
	req := externalRequest{Prompt: prompt}
	if cc, ok := chatContextFrom(ctx); ok {
		req.ChatID = cc.ChatID
		req.Seq = cc.Seq
	}

	stdinBytes, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("target_command: failed to marshal request: %w", err)
	}

	stdout, err := c.cmd.Run(ctx, string(stdinBytes))
	if err != nil {
		return "", fmt.Errorf("target_command: %w", err)
	}

	var resp externalResponse
	if json.Unmarshal([]byte(stdout), &resp) == nil && resp.Response != "" {
		return resp.Response, nil
	}
	// Output is not JSON or has no "response" field — use it as raw text.
	return stdout, nil
}

func (c *ExternalClient) Ping(ctx context.Context) PingResult {
	_, err := c.SendPrompt(ctx, "ping")
	if err != nil {
		return PingResult{
			Success:    false,
			Suggestion: fmt.Sprintf("target_command failed: %s", err),
		}
	}
	return PingResult{
		Success:    true,
		Suggestion: "target_command is responding.",
	}
}
