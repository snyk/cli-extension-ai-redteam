package ping

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/snyk/go-application-framework/pkg/configuration"
	"github.com/snyk/go-application-framework/pkg/workflow"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/target"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

var PingWorkflowID = workflow.NewWorkflowIdentifier("redteam.ping")

func RegisterPingWorkflow(e workflow.Engine) error {
	flagset := pflag.NewFlagSet("snyk-cli-extension-ai-redteam-ping", pflag.ExitOnError)
	flagset.Bool(utils.FlagExperimental, false, "This is an experimental feature that will contain breaking changes in future revisions")
	flagset.String(utils.FlagConfig, "", "Path to the red team configuration file (default: redteam.yaml)")
	flagset.String(utils.FlagTargetURL, "", "URL of the target to ping")
	flagset.String(utils.FlagRequestBodyTmpl, "", `Request body template with {{prompt}} placeholder`)
	flagset.String(utils.FlagResponseSelector, "", "JMESPath expression to extract response from target JSON")
	flagset.StringArray(utils.FlagHeaders, nil, `Request headers in "Key: Value" format (repeatable)`)

	cfg := workflow.ConfigurationOptionsFromFlagset(flagset)
	if _, err := e.Register(PingWorkflowID, cfg, pingWorkflow); err != nil {
		return fmt.Errorf("error while registering ping workflow: %w", err)
	}
	return nil
}

func pingWorkflow(invocationCtx workflow.InvocationContext, _ []workflow.Data) ([]workflow.Data, error) {
	config := invocationCtx.GetConfiguration()

	if !config.GetBool(utils.FlagExperimental) {
		return nil, fmt.Errorf("set the `--experimental` flag to acknowledge that this feature may contain breaking changes")
	}

	settings, err := resolveTargetSettings(config)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	result := target.Ping(ctx, settings.URL, settings.headersMap(), settings.RequestBodyTemplate, settings.ResponseSelector)

	var output string
	if result.Success {
		output = fmt.Sprintf("SUCCESS: %s\nResponse: %s", result.Suggestion, result.Response)
	} else {
		output = fmt.Sprintf("FAILED: %s\nError: %s", result.Suggestion, result.Error)
		if result.RawBody != "" {
			output += fmt.Sprintf("\nRaw body: %s", result.RawBody)
		}
	}

	return []workflow.Data{
		workflow.NewData(
			workflow.NewTypeIdentifier(PingWorkflowID, "ping"),
			"text/plain",
			[]byte(output),
		),
	}, nil
}

type pingTargetSettings struct {
	URL                 string
	Headers             []redteam.ConfigHeader
	RequestBodyTemplate string
	ResponseSelector    string
}

func (s *pingTargetSettings) headersMap() map[string]string {
	headers := make(map[string]string)
	for _, h := range s.Headers {
		headers[h.Name] = h.Value
	}
	return headers
}

func resolveTargetSettings(config configuration.Configuration) (*pingTargetSettings, error) {
	settings := &pingTargetSettings{}

	configPath := config.GetString(utils.FlagConfig)
	if configPath == "" {
		if _, err := os.Stat("redteam.yaml"); err == nil {
			configPath = "redteam.yaml"
		}
	}

	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		var rtConfig redteam.Config
		if err := yaml.Unmarshal(data, &rtConfig); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
		settings.URL = rtConfig.Target.Settings.URL
		settings.Headers = rtConfig.Target.Settings.Headers
		settings.RequestBodyTemplate = rtConfig.Target.Settings.RequestBodyTemplate
		settings.ResponseSelector = rtConfig.Target.Settings.ResponseSelector
	}

	if v := config.GetString(utils.FlagTargetURL); v != "" {
		settings.URL = v
	}
	if v := config.GetString(utils.FlagRequestBodyTmpl); v != "" {
		settings.RequestBodyTemplate = v
	}
	if v := config.GetString(utils.FlagResponseSelector); v != "" {
		settings.ResponseSelector = v
	}
	if raw := config.Get(utils.FlagHeaders); raw != nil {
		if vals, ok := raw.([]string); ok && len(vals) > 0 {
			for _, h := range vals {
				name, value, found := strings.Cut(h, ":")
				if !found {
					continue
				}
				settings.Headers = append(settings.Headers, redteam.ConfigHeader{
					Name:  strings.TrimSpace(name),
					Value: strings.TrimSpace(value),
				})
			}
		}
	}

	if settings.ResponseSelector == "" {
		settings.ResponseSelector = "response"
	}
	if settings.RequestBodyTemplate == "" {
		settings.RequestBodyTemplate = `{"message": "{{prompt}}"}`
	}

	if settings.URL == "" {
		return nil, fmt.Errorf("target URL is required (set in config file or pass --target-url)")
	}

	return settings, nil
}
