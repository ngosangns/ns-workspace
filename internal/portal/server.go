package portal

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"

	"github.com/ngosangns/ns-workspace/internal/agentsync"
)

var (
	errMethodNotAllowed    = errors.New("method not allowed")
	errMissingID           = errors.New("missing skill id")
	errMissingCommand      = errors.New("missing sync command")
	errMissingJobID        = errors.New("missing job id")
	errJobNotFound         = errors.New("sync job not found")
	errStreamingNotSupported = errors.New("streaming not supported")
	errAdapterNotFound     = errors.New("adapter not found")
)

type portalServer struct {
	presets   fs.FS
	store     *Store
	runner    *SyncRunner
	agentsDir string
}

func newPortalServer(presets fs.FS, agentsDir string) (*portalServer, error) {
	opt := agentsyncOptions(agentsDir)
	store, err := NewStore(presets, opt)
	if err != nil {
		return nil, err
	}
	return &portalServer{
		presets:   presets,
		store:     store,
		runner:    NewSyncRunner(presets),
		agentsDir: opt.AgentsDir,
	}, nil
}

func (s *portalServer) router() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/skills", s.handleSkills)
	mux.HandleFunc("/api/skills/", s.handleSkill)
	mux.HandleFunc("/api/mcps", s.handleMCPs)
	mux.HandleFunc("/api/mcps/preset", s.handleMCPPreset)
	mux.HandleFunc("/api/registry", s.handleRegistry)
	mux.HandleFunc("/api/adapters", s.handleAdapters)
	mux.HandleFunc("/api/adapters/", s.handleAdapter)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/sync/", s.handleSync)
	mux.HandleFunc("/api/sync/stream", s.handleSyncStream)
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
}

func spaFileServer(static fs.FS) http.Handler {
	files := http.FileServer(http.FS(static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			files.ServeHTTP(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(static, path); err == nil {
			files.ServeHTTP(w, r)
			return
		}
		if isPortalStaticAssetPath(path) {
			http.NotFound(w, r)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		files.ServeHTTP(w, r2)
	})
}

func isPortalStaticAssetPath(path string) bool {
	return path == "favicon.svg" ||
		path == "style.css" ||
		strings.HasPrefix(path, "assets/") ||
		strings.Contains(path, "/assets/") ||
		strings.HasPrefix(path, "js/")
}

// agentsyncOptions builds default agentsync.Options for the portal.
func agentsyncOptions(agentsDir string) agentsync.Options {
	opt := agentsync.Options{
		ToolFilter: map[string]bool{"all": true},
	}
	if agentsDir != "" {
		opt.AgentsDir = agentsDir
	} else {
		if home, err := agentsync.DefaultAgentsDir(); err == nil {
			opt.AgentsDir = home
		}
	}
	if path, err := agentsync.DefaultUserConfigPath(); err == nil {
		opt.ConfigPath = path
	}
	return opt
}
