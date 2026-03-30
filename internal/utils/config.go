package utils

import (
	"os"
	"strings"

	cli_errors "github.com/snyk/error-catalog-golang-public/cli"
	"github.com/spf13/pflag"
)

// RejectOrgFlag returns an error if the --org flag was passed on the command
// line. Red team commands use --tenant-id for scoping; --org is not supported.
func RejectOrgFlag() error {
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
	return nil
}

const (
	FlagExperimental     = "experimental"
	FlagHTML             = "html"
	FlagConfig           = "config"
	FlagHTMLFileOutput   = "html-file-output"
	FlagJSONFileOutput   = "json-file-output"
	FlagScanID           = "id"
	FlagTenantID         = "tenant-id"
	FlagTargetURL        = "target-url"
	FlagRequestBodyTmpl  = "request-body-template"
	FlagResponseSelector = "response-selector"
	FlagHeader           = "header"
	FlagJSON             = "json"
	FlagListGoals        = "list-goals"
	FlagListStrategies   = "list-strategies"
	FlagListProfiles     = "list-profiles"
	FlagGoals            = "goals"
	FlagProfile          = "profile"
	FlagPurpose          = "purpose"
	FlagSystemPrompt     = "system-prompt"
	FlagTools            = "tools"
)

// AddTargetFlags registers the common target-related flags shared by commands
// that interact with a target (redteam, ping, etc.).
func AddTargetFlags(fs *pflag.FlagSet) {
	fs.Bool(FlagExperimental, false,
		"This is an experimental feature that will contain breaking changes in future revisions")
	fs.String(FlagConfig, "", "Path to the red team configuration file (default: redteam.yaml)")
	fs.String(FlagTargetURL, "", "URL of the target to scan (overrides config file)")
	fs.String(FlagRequestBodyTmpl, "",
		`Request body template with {{prompt}} placeholder (e.g. '{"message": "{{prompt}}"}')`)
	fs.String(FlagResponseSelector, "",
		"Dot-notation path to extract response from target JSON (e.g. response)")
	fs.StringArray(FlagHeader, nil, `Request header in "Key: Value" format (repeatable)`)
}
