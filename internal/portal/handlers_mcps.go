package portal

import (
	"encoding/json"
	"io"
	"net/http"
)

func (s *portalServer) handleMCPs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		servers, err := s.store.ReadMCPs()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, servers)
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
		writeJSON(w, servers)
	default:
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
	}
}
