package web

import (
	"encoding/json"
	"net/http"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/target"
)

type validationResponse struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

type generateResponse struct {
	Yaml     string `json:"yaml"`
	Filename string `json:"filename"`
}

type initialConfigResponse struct {
	ConfigPath string        `json:"config_path,omitempty"`
	Config     *redteam.Config `json:"config,omitempty"`
}

func handleGetInitialConfig(cfg *redteam.Config, configPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg == nil {
			writeJSON(w, http.StatusOK, initialConfigResponse{})
			return
		}
		writeJSON(w, http.StatusOK, initialConfigResponse{
			ConfigPath: configPath,
			Config:     cfg,
		})
	}
}

func handleGenerateConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var cfg redteam.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, validationResponse{
				Valid:  false,
				Errors: []string{"invalid JSON: " + err.Error()},
			})
			return
		}

		data, err := yaml.Marshal(&cfg)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, validationResponse{
				Valid:  false,
				Errors: []string{"failed to marshal config: " + err.Error()},
			})
			return
		}

		writeJSON(w, http.StatusOK, generateResponse{
			Yaml:     string(data),
			Filename: "redteam.yaml",
		})
	}
}

type pingRequest struct {
	URL                 string              `json:"url"`
	Headers             []redteam.ConfigHeader `json:"headers"`
	RequestBodyTemplate string              `json:"request_body_template"`
	ResponseSelector    string              `json:"response_selector"`
}

func handlePing() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req pingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
			return
		}

		headers := make(map[string]string)
		for _, h := range req.Headers {
			if h.Name != "" {
				headers[h.Name] = h.Value
			}
		}

		client := target.NewHTTPClient(nil, req.URL, headers, req.RequestBodyTemplate, req.ResponseSelector)
		result := client.Ping(r.Context())
		writeJSON(w, http.StatusOK, result)
	}
}

func handleListGoals(client controlserver.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := client.ListGoals(r.Context())
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].DisplayOrder < entries[j].DisplayOrder })
		writeJSON(w, http.StatusOK, entries)
	}
}

func handleListStrategies(client controlserver.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := client.ListStrategies(r.Context())
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].DisplayOrder < entries[j].DisplayOrder })
		writeJSON(w, http.StatusOK, entries)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
