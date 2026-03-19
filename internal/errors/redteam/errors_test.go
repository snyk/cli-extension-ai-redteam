package redteam

import (
	"errors"
	"fmt"
	"testing"

	"github.com/snyk/error-catalog-golang-public/snyk_errors"
	"github.com/stretchr/testify/assert"
)

// newRedTeamError enforces snyk_errors.Error at compile time, so all typed
// constructors (NewBadRequestError, etc.) are guaranteed to wrap a catalog
// error. Only NewGenericRedTeamError accepts an arbitrary error and needs
// runtime tests.

func TestNewGenericRedTeamError_PreservesExistingCatalogError(t *testing.T) {
	original := NewBadRequestError("original detail")
	wrapped := NewGenericRedTeamError("wrapped msg", original)

	var snykErr snyk_errors.Error
	assert.True(t, errors.As(wrapped, &snykErr))
	assert.Equal(t, "wrapped msg", wrapped.Error())
}

func TestNewGenericRedTeamError_CreatesCatalogErrorForPlainErrors(t *testing.T) {
	plain := fmt.Errorf("plain network error")
	wrapped := NewGenericRedTeamError("request failed", plain)

	var snykErr snyk_errors.Error
	assert.True(t, errors.As(wrapped, &snykErr),
		"NewGenericRedTeamError must create a catalog error for plain errors")
}
