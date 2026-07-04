package portal

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ngosangns/ns-workspace/internal/agentsync"
)

func (s *portalServer) handleAdapters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	adapters := s.buildAdapters()
	writeJSON(w, adapters)
}

func (s *portalServer) handleAdapter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/adapters/")
	if id == "" {
		writeError(w, http.StatusBadRequest, errMissingID)
		return
	}

	adapters := s.buildAdapters()
	for _, a := range adapters {
		if a.ID == id {
			writeJSON(w, a)
			return
		}
	}
	writeError(w, http.StatusNotFound, errAdapterNotFound)
}

func (s *portalServer) buildAdapters() []Adapter {
	home, _ := os.UserHomeDir()
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	kiro := os.Getenv("KIRO_HOME")
	if kiro == "" {
		kiro = filepath.Join(home, ".kiro")
	}

	registry := agentsync.NewAdapterRegistry(agentsync.RegistryOptions{
		Home:          home,
		XDGConfigHome: xdg,
		KiroHome:      kiro,
	})

	var out []Adapter
	for _, a := range registry.All() {
		caps := a.Capabilities()
		arts := make([]string, len(caps.Artifacts))
		for i, art := range caps.Artifacts {
			arts[i] = string(art)
		}
		out = append(out, Adapter{
			ID:        a.Name(),
			Name:      a.Name(),
			Tier:      string(caps.Tier),
			Docs:      caps.DocsURL,
			Artifacts: arts,
			Notes:     caps.Notes,
		})
	}
	return out
}
