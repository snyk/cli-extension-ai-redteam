package utils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snyk/go-application-framework/pkg/configuration"

	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

func TestRequireAuth_FailsWhenNotAuthenticated(t *testing.T) {
	cfg := configuration.NewWithOpts()
	err := utils.RequireAuth(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Authentication error")
}

func TestRequireAuth_PassesWhenAuthenticated(t *testing.T) {
	cfg := configuration.NewWithOpts()
	cfg.Set(configuration.ORGANIZATION, "some-org-id")
	err := utils.RequireAuth(cfg)
	assert.NoError(t, err)
}
