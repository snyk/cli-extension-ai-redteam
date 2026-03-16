package utils

import "github.com/spf13/pflag"

const (
	FlagExperimental     = "experimental"
	FlagHTML             = "html"
	FlagConfig           = "config"
	FlagHTMLFileOutput   = "html-file-output"
	FlagScanID           = "id"
	FlagTenantID         = "tenant-id"
	FlagTargetURL        = "target-url"
	FlagRequestBodyTmpl  = "request-body-template"
	FlagResponseSelector = "response-selector"
	FlagHeaders          = "headers"
	FlagListGoals        = "list-goals"
	FlagListStrategies   = "list-strategies"
	FlagPurpose          = "purpose"
	FlagSystemPrompt     = "system-prompt"
	FlagTools            = "tools"
)

// AddTargetFlags registers the common target-related flags shared by commands
// that interact with a target (redteam, ping, etc.).
func AddTargetFlags(fs *pflag.FlagSet) {
	fs.Bool(FlagExperimental, false, "This is an experimental feature that will contain breaking changes in future revisions")
	fs.String(FlagConfig, "", "Path to the red team configuration file (default: redteam.yaml)")
	fs.String(FlagTargetURL, "", "URL of the target to scan (overrides config file)")
	fs.String(FlagRequestBodyTmpl, "", `Request body template with {{prompt}} placeholder (e.g. '{"message": "{{prompt}}"}')`)
	fs.String(FlagResponseSelector, "", "Dot-notation path to extract response from target JSON (e.g. response)")
	fs.StringArray(FlagHeaders, nil, `Request headers in "Key: Value" format (repeatable)`)
}
