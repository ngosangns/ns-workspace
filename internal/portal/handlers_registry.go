package portal

import (
	"encoding/json"
	"io"
	"net/http"
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
		writeJSON(w, reg)
	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}
