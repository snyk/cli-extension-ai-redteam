package utils

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ExternalCommand describes a user-supplied binary that can be executed to
// produce or transform data (e.g. resolve a URL, build a request body, or
// parse a response).
type ExternalCommand struct {
	Binary string   `yaml:"binary" json:"binary"`
	Args   []string `yaml:"args" json:"args"`
}

// Run executes the command, optionally piping stdin, and returns trimmed stdout.
func (c *ExternalCommand) Run(ctx context.Context, stdin string) (string, error) {
	cmd := exec.CommandContext(ctx, c.Binary, c.Args...) //nolint:gosec // user-configured binary
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("external command %q failed: %w\nstderr: %s",
			c.Binary, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Validate checks that the command has at least a binary specified.
func (c *ExternalCommand) Validate(field string) error {
	if c.Binary == "" {
		return fmt.Errorf("%s: binary is required", field)
	}
	return nil
}
