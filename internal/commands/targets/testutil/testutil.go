package testutil

import (
	"net/http"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/snyk/go-application-framework/pkg/mocks"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/targets"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	controlservermock "github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver/mock"
	"github.com/snyk/cli-extension-ai-redteam/mocks/frameworkmock"
)

const (
	TenantID    = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	TargetID    = "11111111-2222-3333-4444-555555555555"
	TargetID2   = "66666666-7777-8888-9999-aaaaaaaaaaaa"
	TargetName  = "my-chatbot"
	ErrUsage    = "usage"
	ErrNotFound = "not found"
)

// MockCSFactory returns a ControlServerFactory that always yields the given mock.
func MockCSFactory(mock *controlservermock.MockClient) targets.ControlServerFactory {
	return func(_ *zerolog.Logger, _ *http.Client, _, _ string) controlserver.Client {
		return mock
	}
}

// SetArgs replaces os.Args for the duration of a test.
// Call the returned func in a defer to restore the original value.
func SetArgs(args ...string) func() {
	original := os.Args
	os.Args = args
	return func() { os.Args = original }
}

// BaseCtx creates a mock InvocationContext with experimental and tenant-id pre-set.
func BaseCtx(t *testing.T) *mocks.MockInvocationContext {
	t.Helper()
	ictx := frameworkmock.NewMockInvocationContext(t)
	ictx.GetConfiguration().Set("experimental", true)
	ictx.GetConfiguration().Set("tenant-id", TenantID)
	return ictx
}
