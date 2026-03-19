package redteam

import (
	"errors"
	"net/http"

	cli_errors "github.com/snyk/error-catalog-golang-public/cli"
	snyk_common_errors "github.com/snyk/error-catalog-golang-public/snyk"
	"github.com/snyk/error-catalog-golang-public/snyk_errors"
)

// ErrorFromHTTPStatus returns the appropriate RedTeamError for an HTTP status code.
// Callers should pass a detail string (e.g. "tenants API returned status 401: ...").
func ErrorFromHTTPStatus(statusCode int, detail string) *RedTeamError {
	switch {
	case statusCode == http.StatusUnauthorized:
		return NewUnauthorizedError(detail)
	case statusCode == http.StatusForbidden:
		return NewForbiddenError(detail)
	case statusCode == http.StatusNotFound:
		return NewNotFoundError(detail)
	case statusCode >= 500:
		return NewServerError(detail)
	default:
		return NewHTTPClientError(detail)
	}
}

//nolint:revive // RedTeamError is the canonical name; renaming would break the public API
type RedTeamError struct {
	err     error
	userMsg string
}

func (xerr RedTeamError) Error() string {
	return xerr.userMsg
}

func (xerr RedTeamError) Unwrap() error {
	return xerr.err
}

// newRedTeamError requires a snyk_errors.Error so the compiler enforces that
// every RedTeamError wraps a catalog error. Without this the CLI falls back
// to "Unspecified Error (SNYK-CLI-0000)".
//
//nolint:gocritic // hugeParam: value type is intentional — enforces compile-time catalog error requirement
func newRedTeamError(catalogErr snyk_errors.Error, userMsg string) *RedTeamError {
	return &RedTeamError{
		err:     catalogErr,
		userMsg: userMsg,
	}
}

func NewBadRequestError(msg string) *RedTeamError {
	return newRedTeamError(snyk_common_errors.NewBadRequestError(msg), msg)
}

func NewServerError(msg string) *RedTeamError {
	return newRedTeamError(snyk_common_errors.NewServerError(msg), msg)
}

func NewForbiddenError(msg string) *RedTeamError {
	return newRedTeamError(snyk_common_errors.NewUnauthorisedError(msg), msg)
}

func NewUnauthorizedError(msg string) *RedTeamError {
	return newRedTeamError(snyk_common_errors.NewUnauthorisedError(msg), msg)
}

func NewNotFoundError(msg string) *RedTeamError {
	return newRedTeamError(cli_errors.NewGeneralCLIFailureError(msg), msg)
}

func NewHTTPClientError(msg string) *RedTeamError {
	return newRedTeamError(cli_errors.NewGeneralCLIFailureError(msg), msg)
}

func NewPollingTimeoutError() *RedTeamError {
	msg := "We couldn't get the scan results in a reasonable time. Please try again or contact support."
	return newRedTeamError(snyk_common_errors.NewTimeoutError(msg), msg)
}

func NewConfigValidationError(msg string) *RedTeamError {
	return newRedTeamError(cli_errors.NewCommandArgsError(msg), msg)
}

func NewInternalError(msg string) *RedTeamError {
	return newRedTeamError(cli_errors.NewGeneralCLIFailureError(msg), msg)
}

func NewNetworkError(msg string) *RedTeamError {
	return newRedTeamError(cli_errors.NewConnectionTimeoutError(msg), msg)
}

// NewGenericRedTeamError wraps an arbitrary error with a user-facing message.
// If the wrapped error already contains a catalog error, it is preserved in the
// chain. Otherwise a NewGeneralCLIFailureError is created as a fallback.
func NewGenericRedTeamError(msg string, err error) *RedTeamError {
	var snykErr snyk_errors.Error
	if errors.As(err, &snykErr) {
		// Preserve the original catalog error in the chain.
		return &RedTeamError{err: err, userMsg: msg}
	}
	return newRedTeamError(cli_errors.NewGeneralCLIFailureError(msg), msg)
}
