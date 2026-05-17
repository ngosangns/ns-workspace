package preview

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type previewFileResponse struct {
	Type     string `json:"type"`
	Path     string `json:"path"`
	Title    string `json:"title"`
	Language string `json:"language"`
	Raw      string `json:"raw"`
}

type previewFolderEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

type previewFolderResponse struct {
	Type    string              `json:"type"`
	Path    string              `json:"path"`
	Title   string              `json:"title"`
	Entries []previewFolderEntry `json:"entries"`
}

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
	id, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/api/docs/"))
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

func (ps *previewServer) handleFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rel := filepath.Clean(filepath.FromSlash(strings.TrimSpace(r.URL.Query().Get("path"))))
	if rel == "." || rel == "" || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		http.Error(w, "invalid file path", http.StatusBadRequest)
		return
	}
	path := filepath.Join(ps.opt.projectRoot, rel)
	absRoot, err := filepath.Abs(ps.opt.projectRoot)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	rootPrefix := absRoot + string(filepath.Separator)
	if absPath != absRoot && !strings.HasPrefix(absPath, rootPrefix) {
		http.Error(w, "file path escapes project root", http.StatusBadRequest)
		return
	}
	info, err := os.Stat(absPath)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	if info.IsDir() {
		dirents, err := os.ReadDir(absPath)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		entries := make([]previewFolderEntry, 0, len(dirents))
		for _, d := range dirents {
			name := d.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			childRel := filepath.ToSlash(filepath.Join(rel, name))
			entries = append(entries, previewFolderEntry{
				Name:  name,
				Path:  childRel,
				IsDir: d.IsDir(),
			})
		}
		writeJSON(w, previewFolderResponse{
			Type:    "folder",
			Path:    filepath.ToSlash(rel),
			Title:   filepath.Base(absPath),
			Entries: entries,
		})
		return
	}
	if info.Size() > maxSearchFileBytes {
		http.Error(w, "file is not previewable", http.StatusBadRequest)
		return
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if !utf8.Valid(data) {
		http.Error(w, "file is not valid UTF-8", http.StatusBadRequest)
		return
	}
	if !isPreviewableFilePath(absPath) && !isPathInside(absPath, docsRoot(ps.opt.projectRoot, ps.opt.docsDir)) {
		http.Error(w, "file is not previewable", http.StatusBadRequest)
		return
	}
	writeJSON(w, previewFileResponse{
		Type:     "file",
		Path:     filepath.ToSlash(rel),
		Title:    filepath.Base(absPath),
		Language: languageForPath(absPath),
		Raw:      string(data),
	})
}

func isPathInside(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absPath = filepath.Clean(absPath)
	absRoot = filepath.Clean(absRoot)
	return absPath == absRoot || strings.HasPrefix(absPath, absRoot+string(filepath.Separator))
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
