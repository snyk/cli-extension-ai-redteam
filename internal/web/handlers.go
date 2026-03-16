package web

import (
	"encoding/json"
	"net/http"

	"gopkg.in/yaml.v3"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
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

		if err := redteam.ValidateConfig(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, validationResponse{
				Valid:  false,
				Errors: []string{err.Error()},
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
