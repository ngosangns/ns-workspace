package portal

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func (s *portalServer) handleMCPs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		manifest, err := s.store.ReadMCPs()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, manifest)
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		var servers MCPServers
		if err := json.Unmarshal(body, &servers); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.store.WriteMCPs(&servers); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		manifest, err := s.store.ReadMCPs()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, manifest)
	case http.MethodDelete:
		if err := s.store.ResetMCPs(); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		manifest, err := s.store.ReadMCPs()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, manifest)
	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (s *portalServer) handleMCPPreset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	servers, err := s.store.ReadMCPPreset()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, servers)
}

func isMCPPath(path string) bool {
	return path == "/api/mcps" || strings.HasPrefix(path, "/api/mcps/")
}
