package portal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (s *portalServer) handleRegistry(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		reg, err := s.store.ReadRegistry()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, reg)
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		var reg RegistrySkills
		if err := json.Unmarshal(body, &reg); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.store.WriteRegistry(&reg); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		out, err := s.store.ReadRegistry()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, out)
	case http.MethodDelete:
		if err := s.store.ResetRegistry(); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		out, err := s.store.ReadRegistry()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, out)
	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

// handleRegistrySkill routes /api/registry/{name}/enabled.
func (s *portalServer) handleRegistrySkill(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/registry/")
	if path == "" {
		writeError(w, http.StatusBadRequest, errMissingID)
		return
	}
	if !strings.HasSuffix(path, "/enabled") {
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown registry path %q", path))
		return
	}
	name := strings.TrimSuffix(path, "/enabled")
	name = strings.TrimSuffix(name, "/")
	if name == "" {
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
	if err := s.store.SetRegistrySkillEnabled(name, req.Enabled); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	out, err := s.store.ReadRegistry()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, out)
}
