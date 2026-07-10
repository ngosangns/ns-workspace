package portal

import (
	"encoding/json"
	"io"
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
	path := strings.TrimPrefix(r.URL.Path, "/api/adapters/")
	if path == "" {
		writeError(w, http.StatusBadRequest, errMissingID)
		return
	}

	// POST /api/adapters/{id}/enabled
	if strings.HasSuffix(path, "/enabled") {
		id := strings.TrimSuffix(path, "/enabled")
		id = strings.TrimSuffix(id, "/")
		if id == "" {
			writeError(w, http.StatusBadRequest, errMissingID)
			return
		}
		if r.Method != http.MethodPost && r.Method != http.MethodPut {
			writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		var req EnableRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.store.SetProviderEnabled(id, req.Enabled); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		adapters := s.buildAdapters()
		for _, a := range adapters {
			if a.ID == id || strings.EqualFold(a.ID, id) {
				writeJSON(w, a)
				return
			}
		}
		writeJSON(w, Adapter{ID: id, Name: id, Enabled: req.Enabled})
		return
	}

	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	id := path
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
			Enabled:   s.store.ProviderEnabled(a.Name()),
			Docs:      caps.DocsURL,
			Artifacts: arts,
			Notes:     caps.Notes,
		})
	}
	return out
}
