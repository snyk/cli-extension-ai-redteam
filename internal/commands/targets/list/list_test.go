package list_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/targets/list"
	"github.com/snyk/cli-extension-ai-redteam/internal/commands/targets/testutil"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	controlservermock "github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver/mock"
	"github.com/snyk/cli-extension-ai-redteam/mocks/frameworkmock"
)

func TestList_MissingExperimental(t *testing.T) {
	ictx := frameworkmock.NewMockInvocationContext(t)
	ictx.GetConfiguration().Set("tenant-id", testutil.TenantID)

	_, err := list.RunListWorkflow(ictx, testutil.MockCSFactory(&controlservermock.MockClient{}))
	require.Error(t, err)
}

func TestList_Happy(t *testing.T) {
	ictx := testutil.BaseCtx(t)

	mock := &controlservermock.MockClient{
		Targets: []controlserver.TargetListItem{
			{ID: testutil.TargetID, Name: testutil.TargetName, CreatedAt: "2026-04-01T10:00:00Z", UpdatedAt: "2026-04-01T10:00:00Z"},
			{ID: testutil.TargetID2, Name: "prod-bot", CreatedAt: "2026-03-30T14:22:00Z", UpdatedAt: "2026-03-30T14:22:00Z"},
		},
	}

	results, err := list.RunListWorkflow(ictx, testutil.MockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	payload, ok := results[0].GetPayload().([]byte)
	require.True(t, ok)
	output := string(payload)
	assert.Contains(t, output, testutil.TargetName)
	assert.Contains(t, output, "prod-bot")
	assert.Contains(t, output, testutil.TargetID)
}

func TestList_Empty(t *testing.T) {
	ictx := testutil.BaseCtx(t)

	mock := &controlservermock.MockClient{Targets: []controlserver.TargetListItem{}}

	results, err := list.RunListWorkflow(ictx, testutil.MockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	payload, ok := results[0].GetPayload().([]byte)
	require.True(t, ok)
	assert.Contains(t, string(payload), "No saved targets")
}

func TestList_ServerError(t *testing.T) {
	ictx := testutil.BaseCtx(t)

	mock := &controlservermock.MockClient{TargetsErr: fmt.Errorf("server error")}

	_, err := list.RunListWorkflow(ictx, testutil.MockCSFactory(mock))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server error")
}
