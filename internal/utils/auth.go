package utils

import (
	snyk_common_errors "github.com/snyk/error-catalog-golang-public/snyk"
	"github.com/snyk/go-application-framework/pkg/configuration"
)

// RequireAuth checks if an orgID is set to ensure that the user is logged in.
// This may not be the canonical way of doing this and should be considered
// a temporary workaround.
func RequireAuth(config configuration.Configuration) error {
	orgID := config.GetString(configuration.ORGANIZATION)
	if orgID == "" {
		return snyk_common_errors.NewUnauthorisedError("")
	}
	return nil
}
