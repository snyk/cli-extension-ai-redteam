package utils

import (
	"os"
	"strings"

	cli_errors "github.com/snyk/error-catalog-golang-public/cli"
	snyk_common_errors "github.com/snyk/error-catalog-golang-public/snyk"
	"github.com/snyk/go-application-framework/pkg/configuration"
)

// RequireAuth rejects the unsupported --org flag and then checks that an orgID
// is present in the configuration (which signals the user is authenticated).
// This may not be the canonical way of doing this and should be considered
// a temporary workaround.
func RequireAuth(config configuration.Configuration) error {
	for _, arg := range os.Args[1:] {
		if arg == "--" {
			break
		}
		if arg == "--org" || strings.HasPrefix(arg, "--org=") {
			return cli_errors.NewCommandArgsError(
				"the --org flag is not supported by red team commands; use --tenant-id instead",
			)
		}
	}

	orgID := config.GetString(configuration.ORGANIZATION)
	if orgID == "" {
		return snyk_common_errors.NewUnauthorisedError("")
	}
	return nil
}
