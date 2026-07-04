package portal

import (
	"net/http"
	"os"
	"path/filepath"
)

func (s *portalServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	paths := []string{
		filepath.Join(s.agentsDir, "AGENTS.md"),
		filepath.Join(s.agentsDir, "agents"),
		filepath.Join(s.agentsDir, "registry", "skills.json"),
		filepath.Join(s.agentsDir, "skills"),
		filepath.Join(s.agentsDir, "settings.json"),
		filepath.Join(s.agentsDir, "mcp", "servers.json"),
	}

	var statuses []PathStatus
	for _, p := range paths {
		info, err := os.Stat(p)
		statuses = append(statuses, PathStatus{
			Path:   p,
			Exists: err == nil,
			IsDir:  err == nil && info.IsDir(),
		})
	}

	writeJSON(w, StatusSummary{AgentsDir: s.agentsDir, Paths: statuses})
}

func (s *portalServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	writeJSON(w, s.store.UserOverlay())
}
