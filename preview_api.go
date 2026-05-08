package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (ps *previewServer) handleProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	project, err := ps.load()
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, project.Summary)
}

func (ps *previewServer) handleSpecs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	project, err := ps.load()
	if err != nil {
		writeAPIError(w, err)
		return
	}
	list := make([]specDocument, len(project.Documents))
	for i, doc := range project.Documents {
		doc.Raw = ""
		doc.HTML = ""
		list[i] = doc
	}
	writeJSON(w, list)
}

func (ps *previewServer) handleSpec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/api/specs/"))
	if err != nil || id == "" {
		http.Error(w, "invalid spec id", http.StatusBadRequest)
		return
	}
	project, err := ps.load()
	if err != nil {
		writeAPIError(w, err)
		return
	}
	for _, doc := range project.Documents {
		if doc.ID == id {
			writeJSON(w, doc)
			return
		}
	}
	http.Error(w, "spec not found", http.StatusNotFound)
}

func (ps *previewServer) handleGraph(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	project, err := ps.load()
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, project.Graph)
}

func (ps *previewServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	last := ps.changeToken()
	fmt.Fprintf(w, "event: ready\ndata: %s\n\n", strconv.Quote(last))
	flusher.Flush()

	ticker := time.NewTicker(900 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			next := ps.changeToken()
			if next == last {
				fmt.Fprint(w, ": heartbeat\n\n")
				flusher.Flush()
				continue
			}
			last = next
			fmt.Fprintf(w, "event: change\ndata: %s\n\n", strconv.Quote(next))
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(value)
}

func writeAPIError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
