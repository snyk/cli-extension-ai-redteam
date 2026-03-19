package utils

import (
	"errors"

	"github.com/snyk/error-catalog-golang-public/snyk_errors"

	redteam_errors "github.com/snyk/cli-extension-ai-redteam/internal/errors/redteam"
)

// ErrorFromHTTPClient wraps an error returned by the framework's HTTP client
// into a RedTeamError, preserving the original catalog error in the chain.
//
// The framework intercepts 4xx/5xx responses and returns a snyk_errors.Error
// with StatusCode, ErrorCode, Detail, and Classification already set. Rather
// than re-classifying (which loses the original error identity), we pass it
// through via NewGenericRedTeamError so the CLI can display the correct error
// code and detail.
func ErrorFromHTTPClient(endpoint string, err error) *redteam_errors.RedTeamError {
	var snykErr snyk_errors.Error
	if errors.As(err, &snykErr) {
		msg := snykErr.Detail
		if msg == "" {
			msg = snykErr.Title
		}
		if msg == "" {
			msg = err.Error()
		}
		return redteam_errors.NewGenericRedTeamError(msg, err)
	}

	// Fallback for non-catalog errors (e.g. network failures).
	return redteam_errors.NewGenericRedTeamError(
		endpoint+" request failed: "+err.Error(), err,
	)
}
