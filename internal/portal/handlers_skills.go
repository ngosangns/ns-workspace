package portal

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func (s *portalServer) handleSkills(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		skills, err := s.store.ListSkills()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, skills)
	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}

func (s *portalServer) handleSkill(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/skills/")
	if path == "" {
		writeError(w, http.StatusBadRequest, errMissingID)
		return
	}

	// POST /api/skills/{id}/enabled
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
		if err := s.store.SetSkillEnabled(id, req.Enabled); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		skill, err := s.store.ReadSkill(id)
		if err != nil {
			// Skill may be listed but content missing; return enable state only.
			writeJSON(w, Skill{ID: id, Name: id, Enabled: req.Enabled})
			return
		}
		writeJSON(w, skill)
		return
	}

	id := path
	switch r.Method {
	case http.MethodGet:
		skill, err := s.store.ReadSkill(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, skill)
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		var update SkillUpdate
		if err := json.Unmarshal(body, &update); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.store.WriteSkill(id, []byte(update.Content)); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		skill, err := s.store.ReadSkill(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, skill)
	case http.MethodDelete:
		if err := s.store.ResetSkill(id); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}
