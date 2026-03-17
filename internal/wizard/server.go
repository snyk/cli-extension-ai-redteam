package wizard

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/browser"

	"github.com/snyk/go-application-framework/pkg/ui"

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
)

// Server serves the setup wizard UI and exposes API endpoints for config validation and saving.
type Server struct {
	port          int
	configPath    string
	initialConfig *redteam.Config
	csClient      controlserver.Client
	ui            ui.UserInterface
	devMode       bool
	shutdown      chan struct{}
}

func NewServer(port int, configPath string, initialConfig *redteam.Config, csClient controlserver.Client, userInterface ui.UserInterface) *Server {
	return &Server{
		port:          port,
		configPath:    configPath,
		initialConfig: initialConfig,
		csClient:      csClient,
		ui:            userInterface,
		devMode:       os.Getenv("REDTEAM_DEV") == "1",
		shutdown:      make(chan struct{}),
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/config", handleGetInitialConfig(s.initialConfig, s.configPath))
	mux.HandleFunc("POST /api/ping", handlePing())
	mux.HandleFunc("GET /api/goals", handleListGoals(s.csClient))
	mux.HandleFunc("GET /api/strategies", handleListStrategies(s.csClient))
	mux.HandleFunc("GET /api/profiles", handleListProfiles(s.csClient))
	mux.HandleFunc("POST /api/download-complete", s.handleDownloadComplete())

	if s.devMode {
		// In dev mode, proxy all non-API requests to the Vite dev server for hot-reload.
		viteURL, err := url.Parse("http://localhost:5173")
		if err != nil {
			return fmt.Errorf("failed to parse vite dev URL: %w", err)
		}
		mux.Handle("/", httputil.NewSingleHostReverseProxy(viteURL))
	} else {
		sub, err := fs.Sub(DistFS, "dist")
		if err != nil {
			return fmt.Errorf("failed to create sub filesystem: %w", err)
		}
		mux.Handle("/", spaHandler(http.FS(sub)))
	}

	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	addr := listener.Addr().String()
	wizardURL := fmt.Sprintf("http://%s", addr)
	_ = s.ui.Output(fmt.Sprintf("Setup wizard running at %s\n", wizardURL))
	if !s.devMode {
		if err := browser.OpenURL(wizardURL); err != nil {
			_ = s.ui.Output(fmt.Sprintf("Could not open browser: %v\n", err))
		}
	}

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-s.shutdown
		time.Sleep(1 * time.Second)
		_ = server.Close()
	}()

	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func (s *Server) handleDownloadComplete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Filename string `json:"filename"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})

		configFile := req.Filename
		if configFile == "" {
			configFile = "redteam.yaml"
		}

		var sb strings.Builder
		sb.WriteString("\nConfiguration downloaded successfully!\n\n")
		sb.WriteString("Next steps:\n")
		sb.WriteString("  1. Close this wizard with Ctrl+C\n")
		sb.WriteString("  2. Run your red team scan:\n\n")
		fmt.Fprintf(&sb, "     snyk redteam --experimental --config %s\n\n", configFile)
		_ = s.ui.Output(sb.String())
	}
}

// spaHandler serves static files from the given filesystem, falling back to index.html for
// paths that don't match a file (SPA client-side routing).
func spaHandler(fsys http.FileSystem) http.Handler {
	fileServer := http.FileServer(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Try to open the file; if it doesn't exist and has no extension, serve index.html.
		f, err := fsys.Open(path)
		if err != nil {
			if !strings.Contains(path[strings.LastIndex(path, "/")+1:], ".") {
				r.URL.Path = "/"
				fileServer.ServeHTTP(w, r)
				return
			}
			http.NotFound(w, r)
			return
		}
		_ = f.Close()
		fileServer.ServeHTTP(w, r)
	})
}
