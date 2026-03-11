package helpers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/snyk/go-application-framework/pkg/configuration"
	"github.com/snyk/go-application-framework/pkg/workflow"
)

type tenant struct {
	ID   string
	Name string
}

type tenantsAPIResponse struct {
	Data []tenantsAPIData `json:"data"`
}

type tenantsAPIData struct {
	ID         string               `json:"id"`
	Attributes tenantsAPIAttributes `json:"attributes"`
}

type tenantsAPIAttributes struct {
	Name string `json:"name"`
}

// GetTenantID resolves a tenant ID using the following priority:
//  1. Already provided (flag / env var)
//  2. Auto-discover via GET /rest/tenants — single tenant auto-selects, multiple prompts
func GetTenantID(ctx workflow.InvocationContext, tenantID string) (string, error) {
	if tenantID != "" {
		return tenantID, nil
	}

	logger := ctx.GetEnhancedLogger()
	ui := ctx.GetUserInterface()

	tenants, err := fetchTenants(ctx)
	if err != nil {
		logger.Debug().Err(err).Msg("failed to fetch tenants")
		return "", fmt.Errorf("failed to fetch tenants: %w", err)
	}

	if len(tenants) == 0 {
		return "", fmt.Errorf("no tenants found for your account")
	}

	if len(tenants) == 1 {
		return tenants[0].ID, nil
	}

	labels := make([]string, len(tenants))
	for i, t := range tenants {
		labels[i] = fmt.Sprintf("%s (%s)", t.Name, t.ID)
	}
	idx, _, selErr := ui.SelectOptions("Select tenant", labels)
	if selErr != nil {
		logger.Debug().Err(selErr).Msg("error selecting tenant")
		return "", fmt.Errorf("error selecting tenant: %w", selErr)
	}
	if idx >= 0 && idx < len(tenants) {
		return tenants[idx].ID, nil
	}

	return "", fmt.Errorf("invalid tenant selection (index %d)", idx)
}

func fetchTenants(ctx workflow.InvocationContext) ([]tenant, error) {
	config := ctx.GetConfiguration()
	httpClient := ctx.GetNetworkAccess().GetHttpClient()

	apiURL := strings.TrimSuffix(config.GetString(configuration.API_URL), "/")
	url := fmt.Sprintf("%s/rest/tenants?version=2025-11-05&limit=100", apiURL)

	req, err := http.NewRequestWithContext(ctx.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("build tenants request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tenants request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read tenants response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tenants API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp tenantsAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal tenants response: %w", err)
	}

	result := make([]tenant, 0, len(apiResp.Data))
	for _, d := range apiResp.Data {
		result = append(result, tenant{
			ID:   d.ID,
			Name: d.Attributes.Name,
		})
	}
	return result, nil
}
