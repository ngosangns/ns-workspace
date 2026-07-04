package portal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (s *portalServer) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	command := strings.TrimPrefix(r.URL.Path, "/api/sync/")
	if command == "" {
		writeError(w, http.StatusBadRequest, errMissingCommand)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req SyncRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	if req.Command == "" {
		req.Command = command
	}

	jobID, err := s.runner.Start(req.Command, req.DryRun, s.agentsDir)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, SyncJob{ID: jobID, Command: req.Command, DryRun: req.DryRun, Running: true})
}

func (s *portalServer) handleSyncStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}
	jobID := r.URL.Query().Get("jobId")
	if jobID == "" {
		writeError(w, http.StatusBadRequest, errMissingJobID)
		return
	}

	job := s.runner.Job(jobID)
	if job == nil {
		writeError(w, http.StatusNotFound, errJobNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errStreamingNotSupported)
		return
	}

	fmt.Fprintf(w, "event: start\ndata: %s\n\n", jobID)
	flusher.Flush()

	job.Subscribe(func(line string) {
		fmt.Fprintf(w, "data: %s\n\n", jsonEscape(line))
		flusher.Flush()
	})

	fmt.Fprint(w, "event: end\ndata: {}\n\n")
	flusher.Flush()
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
