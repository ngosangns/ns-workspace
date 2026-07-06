package portal

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func (s *portalServer) handleClaudeSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := s.store.ReadClaudeSettings()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, settings)
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		var settings ClaudeSettings
		if err := json.Unmarshal(body, &settings); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.store.WriteClaudeSettings(&settings); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		updated, err := s.store.ReadClaudeSettings()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, updated)
	case http.MethodDelete:
		if err := s.store.ResetClaudeSettings(); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		settings, err := s.store.ReadClaudeSettings()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, settings)
	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (s *portalServer) handleClaudeSettingsPreset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	settings, err := s.store.ReadClaudeSettingsPreset()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, settings)
}

func isClaudeSettingsPath(path string) bool {
	return path == "/api/settings/claude" || strings.HasPrefix(path, "/api/settings/claude/")
}
