package helpers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snyk/cli-extension-ai-redteam/internal/helpers"
	"github.com/snyk/cli-extension-ai-redteam/mocks/frameworkmock"

	"github.com/snyk/go-application-framework/pkg/configuration"
)

func TestGetTenantID_ReturnsProvidedID(t *testing.T) {
	const expected = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	got, err := helpers.GetTenantID(nil, expected)
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestGetTenantID_SingleTenant(t *testing.T) {
	resp := map[string]interface{}{
		"data": []map[string]interface{}{
			{
				"id":         "11111111-2222-3333-4444-555555555555",
				"attributes": map[string]string{"name": "My Tenant"},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/tenants", r.URL.Path)
		assert.Contains(t, r.URL.RawQuery, "version=2025-11-05")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ictx := frameworkmock.NewMockInvocationContext(t)
	ictx.GetConfiguration().Set(configuration.API_URL, server.URL)

	got, err := helpers.GetTenantID(ictx, "")
	require.NoError(t, err)
	assert.Equal(t, "11111111-2222-3333-4444-555555555555", got)
}

func TestGetTenantID_ZeroTenants_ReturnsError(t *testing.T) {
	resp := map[string]interface{}{
		"data": []map[string]interface{}{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ictx := frameworkmock.NewMockInvocationContext(t)
	ictx.GetConfiguration().Set(configuration.API_URL, server.URL)

	_, err := helpers.GetTenantID(ictx, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tenants found")
}

func TestGetTenantID_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "server error"}`))
	}))
	defer server.Close()

	ictx := frameworkmock.NewMockInvocationContext(t)
	ictx.GetConfiguration().Set(configuration.API_URL, server.URL)

	_, err := helpers.GetTenantID(ictx, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
