package utils_test

import (
	"os"
	"testing"

	"github.com/snyk/error-catalog-golang-public/snyk_errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snyk/go-application-framework/pkg/configuration"

	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

func withArgs(t *testing.T, args []string) {
	t.Helper()
	orig := os.Args
	os.Args = args
	t.Cleanup(func() { os.Args = orig })
}

func authedConfig() configuration.Configuration { //nolint:ireturn // test helper
	cfg := configuration.NewWithOpts()
	cfg.Set(configuration.ORGANIZATION, "some-org-id")
	return cfg
}

func errorDetail(t *testing.T, err error) string {
	t.Helper()
	var snykErr snyk_errors.Error
	require.ErrorAs(t, err, &snykErr)
	return snykErr.Detail
}

func TestRequireAuth_RejectsOrgFlag(t *testing.T) {
	withArgs(t, []string{"snyk", "redteam", "--experimental", "--org", "my-org"})
	err := utils.RequireAuth(authedConfig())
	require.Error(t, err)
	assert.Contains(t, errorDetail(t, err), "--org flag is not supported")
}

func TestRequireAuth_RejectsOrgFlagWithEquals(t *testing.T) {
	withArgs(t, []string{"snyk", "redteam", "--org=my-org"})
	err := utils.RequireAuth(authedConfig())
	require.Error(t, err)
	assert.Contains(t, errorDetail(t, err), "--org flag is not supported")
}

func TestRequireAuth_FailsWhenNotAuthenticated(t *testing.T) {
	withArgs(t, []string{"snyk", "redteam"})
	cfg := configuration.NewWithOpts()
	err := utils.RequireAuth(cfg)
	require.Error(t, err)
	assert.NotContains(t, errorDetail(t, err), "--org")
}

func TestRequireAuth_PassesWhenAuthenticated(t *testing.T) {
	withArgs(t, []string{"snyk", "redteam"})
	err := utils.RequireAuth(authedConfig())
	assert.NoError(t, err)
}
