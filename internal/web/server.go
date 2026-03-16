package web

import (
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

	"github.com/snyk/cli-extension-ai-redteam/internal/commands/redteam"
)

// Server serves the setup wizard UI and exposes API endpoints for config validation and saving.
type Server struct {
	port          int
	outputPath    string
	initialConfig *redteam.Config
	devMode       bool
	shutdown      chan struct{}
}

func NewServer(port int, outputPath string, initialConfig *redteam.Config) *Server {
	return &Server{
		port:          port,
		outputPath:    outputPath,
		initialConfig: initialConfig,
		devMode:       os.Getenv("REDTEAM_DEV") == "1",
		shutdown:      make(chan struct{}),
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/config", handleGetInitialConfig(s.initialConfig))
	mux.HandleFunc("POST /api/config", handleGenerateConfig())

	if s.devMode {
		viteURL, _ := url.Parse("http://localhost:5173")
		mux.Handle("/", httputil.NewSingleHostReverseProxy(viteURL))
	} else {
		sub, err := fs.Sub(DistFS, "dist")
		if err != nil {
			return fmt.Errorf("failed to create sub filesystem: %w", err)
		}
		mux.Handle("/", spaHandler(http.FS(sub)))
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	addr := listener.Addr().String()
	url := fmt.Sprintf("http://%s", addr)
	fmt.Printf("Setup wizard running at %s\n", url)
	if !s.devMode {
		_ = browser.OpenURL(url)
	}

	server := &http.Server{Handler: mux}

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
