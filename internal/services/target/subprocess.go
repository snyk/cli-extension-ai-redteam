package target

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// SubprocessClient implements Client by spawning an external command per prompt.
// The command receives JSON on stdin and writes JSON on stdout.
//
// stdin:  {"prompt": "..."}
// stdout: {"response": "...", "tool_calls": [...]}
type SubprocessClient struct {
	binary              string
	args                []string
	cwd                 string
	consecutiveFailures int
	lastErr             error
}

var _ Client = (*SubprocessClient)(nil)

func NewSubprocessClient(binary string, args []string, cwd string) *SubprocessClient {
	return &SubprocessClient{
		binary: binary,
		args:   args,
		cwd:    cwd,
	}
}

type subprocessRequest struct {
	Prompt string `json:"prompt"`
}

type subprocessResponse struct {
	Response string `json:"response"`
}

func (c *SubprocessClient) SendPrompt(ctx context.Context, prompt string) (string, error) {
	if c.consecutiveFailures >= ConsecutiveFailureMax {
		return "", fmt.Errorf("%w: last error: %w", ErrCircuitOpen, c.lastErr)
	}

	reqJSON, err := json.Marshal(subprocessRequest{Prompt: prompt})
	if err != nil {
		return "", fmt.Errorf("marshal prompt: %w", err)
	}

	cmd := exec.CommandContext(ctx, c.binary, c.args...)
	if c.cwd != "" {
		cmd.Dir = c.cwd
	}
	cmd.Stdin = bytes.NewReader(reqJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	// Tee stderr to both the buffer (for error messages) and os.Stderr (for live logging).
	cmd.Stderr = io.MultiWriter(&stderr, os.Stderr)

	if err := cmd.Run(); err != nil {
		c.consecutiveFailures++
		c.lastErr = fmt.Errorf("target command failed: %w\nstderr: %s", err, strings.TrimSpace(stderr.String()))
		return "", c.lastErr
	}

	c.consecutiveFailures = 0

	var resp subprocessResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return "", fmt.Errorf("parse target response: %w\nstdout: %s", err, strings.TrimSpace(stdout.String()))
	}

	return resp.Response, nil
}

func (c *SubprocessClient) Ping(ctx context.Context) PingResult {
	resp, err := c.SendPrompt(ctx, PingMessage)
	if err != nil {
		return PingResult{
			Success:    false,
			Error:      err.Error(),
			Suggestion: "Check that the target command is correct and the binary is installed",
		}
	}
	return PingResult{
		Success:  true,
		Response: resp,
	}
}
