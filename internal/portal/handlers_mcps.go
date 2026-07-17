package portal

import (
	"encoding/json"
	"fmt"
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
		// Single catalog source:
		//   { "content": "<unified JSON>" }
		//   { "mcpServers": {...}, "disabled": ["a"] }
		//   { "mcpServers": {...}, "disabledServers": {...} }
		var contentBody struct {
			Content         string         `json:"content"`
			MCPServers      map[string]any `json:"mcpServers"`
			Disabled        []string       `json:"disabled"`
			DisabledServers map[string]any `json:"disabledServers"`
		}
		if err := json.Unmarshal(body, &contentBody); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(contentBody.Content) != "" {
			if err := s.store.WriteMCPsContent([]byte(contentBody.Content)); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		} else if contentBody.DisabledServers != nil || len(contentBody.Disabled) > 0 {
			// Rebuild unified content from structured fields so one write path
			// applies full-catalog replace semantics.
			raw, err := formatUnifiedMCPContent(
				contentBody.MCPServers,
				contentBody.DisabledServers,
				nil,
			)
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			// If only disabled name list (not map), inject names after format
			// via content document with disabled array.
			if contentBody.DisabledServers == nil && len(contentBody.Disabled) > 0 {
				doc := map[string]any{
					"mcpServers": contentBody.MCPServers,
					"disabled":   contentBody.Disabled,
				}
				raw, err = json.Marshal(doc)
				if err != nil {
					writeError(w, http.StatusBadRequest, err)
					return
				}
			}
			if err := s.store.WriteMCPsContent(raw); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		} else {
			if err := s.store.WriteMCPs(&MCPServers{MCPServers: contentBody.MCPServers}); err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
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

// handleMCPServer routes /api/mcps/{name}/enabled, DELETE /api/mcps/{name}, and reserved subpaths.
func (s *portalServer) handleMCPServer(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/mcps/")
	if path == "" || path == "preset" {
		// preset is handled by a dedicated route; empty is invalid.
		if path == "preset" {
			s.handleMCPPreset(w, r)
			return
		}
		writeError(w, http.StatusBadRequest, errMissingID)
		return
	}
	if strings.HasSuffix(path, "/enabled") {
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
		if err := s.store.SetMCPEnabled(name, req.Enabled); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		manifest, err := s.store.ReadMCPs()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, manifest)
		return
	}
	// DELETE /api/mcps/{name} — hard-remove from enabled + disabled overlay.
	name := strings.Trim(path, "/")
	if name == "" || strings.Contains(name, "/") {
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown mcp path %q", path))
		return
	}
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	if err := s.store.DeleteMCP(name); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	manifest, err := s.store.ReadMCPs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, manifest)
}

func isMCPPath(path string) bool {
	return path == "/api/mcps" || strings.HasPrefix(path, "/api/mcps/")
}
