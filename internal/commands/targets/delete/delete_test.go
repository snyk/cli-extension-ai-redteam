package delete_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	targetsdelete "github.com/snyk/cli-extension-ai-redteam/internal/commands/targets/delete"
	"github.com/snyk/cli-extension-ai-redteam/internal/commands/targets/testutil"
	controlservermock "github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver/mock"
)

func TestDelete_Happy(t *testing.T) {
	defer testutil.SetArgs("snyk", "redteam", "targets", "delete", testutil.TargetName)()
	ictx := testutil.BaseCtx(t)

	mock := &controlservermock.MockClient{}

	results, err := targetsdelete.RunDeleteWorkflow(ictx, testutil.MockCSFactory(mock))
	require.NoError(t, err)
	require.Len(t, results, 1)

	payload, ok := results[0].GetPayload().([]byte)
	require.True(t, ok)
	assert.Contains(t, string(payload), "deleted")
	assert.Equal(t, testutil.TargetName, mock.DeletedTargetName)
}

func TestDelete_MissingArg(t *testing.T) {
	defer testutil.SetArgs("snyk", "redteam", "targets", "delete")()
	ictx := testutil.BaseCtx(t)

	_, err := targetsdelete.RunDeleteWorkflow(ictx, testutil.MockCSFactory(&controlservermock.MockClient{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), testutil.ErrUsage)
}

func TestDelete_NotFound(t *testing.T) {
	defer testutil.SetArgs("snyk", "redteam", "targets", "delete", "nonexistent")()
	ictx := testutil.BaseCtx(t)

	mock := &controlservermock.MockClient{
		DeleteTgtErr: errors.New(testutil.ErrNotFound),
	}

	_, err := targetsdelete.RunDeleteWorkflow(ictx, testutil.MockCSFactory(mock))
	require.Error(t, err)
	assert.Contains(t, err.Error(), testutil.ErrNotFound)
}
