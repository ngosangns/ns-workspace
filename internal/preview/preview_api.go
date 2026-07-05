package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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

// PreviewHandler exposes the docs preview HTTP API as a reusable handler set.
// It can be registered into any http.ServeMux (for example the portal server).
type PreviewHandler struct {
	projectRoot string
	docsDir     string
	codeGraph   PreviewCodeGraphProvider

	projectMu    sync.RWMutex
	projectToken string
	project      specProject
	projectErr   error

	searchMu    sync.RWMutex
	searchToken string
	search      previewSearchSnapshot
}

// NewPreviewHandler creates a docs preview API handler for the given project.
// If codeGraph is nil, a default LSP-based code graph provider is created.
func NewPreviewHandler(projectRoot, docsDir string, codeGraph PreviewCodeGraphProvider) *PreviewHandler {
	if codeGraph == nil {
		codeGraph = newPreviewLSPCodeGraphProvider(projectRoot, docsRoot(projectRoot, docsDir))
	}
	return &PreviewHandler{
		projectRoot: projectRoot,
		docsDir:     docsDir,
		codeGraph:   codeGraph,
	}
}

// Register adds the preview API routes to mux under the given prefix.
// The prefix should include a trailing slash when routes are nested, e.g.
// "/api/" for the standalone preview server or "/api/preview/" for the portal.
func (h *PreviewHandler) Register(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"project", h.HandleProject)
	mux.HandleFunc(prefix+"docs", h.HandleSpecs)
	mux.HandleFunc(prefix+"docs/", h.HandleSpec)
	mux.HandleFunc(prefix+"files", h.HandleFile)
	mux.HandleFunc(prefix+"graph", h.HandleGraph)
	mux.HandleFunc(prefix+"search", h.HandleSearch)
	mux.HandleFunc(prefix+"events", h.HandleEvents)
}

// Close releases resources held by the handler, such as LSP processes.
func (h *PreviewHandler) Close(ctx context.Context) error {
	if h.codeGraph != nil {
		return h.codeGraph.Close(ctx)
	}
	return nil
}

func (h *PreviewHandler) HandleProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	project, err := h.load()
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, project.Summary)
}

func (h *PreviewHandler) HandleSpecs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	project, err := h.load()
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

func (h *PreviewHandler) HandleSpec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	const specPathPrefix = "/docs/"
	idx := strings.Index(r.URL.Path, specPathPrefix)
	if idx < 0 {
		http.Error(w, "invalid spec id", http.StatusBadRequest)
		return
	}
	id, err := url.PathUnescape(r.URL.Path[idx+len(specPathPrefix):])
	if err != nil || id == "" {
		http.Error(w, "invalid spec id", http.StatusBadRequest)
		return
	}
	project, err := h.load()
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

func (h *PreviewHandler) HandleFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rel := filepath.Clean(filepath.FromSlash(strings.TrimSpace(r.URL.Query().Get("path"))))
	if rel == "." || rel == "" || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		http.Error(w, "invalid file path", http.StatusBadRequest)
		return
	}
	path := filepath.Join(h.projectRoot, rel)
	absRoot, err := filepath.Abs(h.projectRoot)
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
	if !isPreviewableFilePath(absPath) && !isPathInside(absPath, docsRoot(h.projectRoot, h.docsDir)) {
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

func (h *PreviewHandler) HandleGraph(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	project, err := h.load()
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, project.Graph)
}

func (h *PreviewHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	project, err := h.load()
	warnings := []string{}
	if err != nil {
		project = emptySearchProject(h.projectRoot, h.docsDir)
		warnings = append(warnings, "Docs directory is unavailable; searching code and LSP code graph only: "+err.Error())
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	mode := "hybrid"
	keywordOperator := parseSearchKeywordOperator(r.URL.Query().Get("keywordOp"))
	limit := parseSearchLimit(r.URL.Query().Get("limit"))
	snapshot := h.searchSnapshot(project)
	response := buildPreviewSearchResponseFromCorpus(r.Context(), project, h.codeGraph, query, mode, keywordOperator, limit, snapshot.Docs, snapshot.Code, snapshot.Warnings)
	response.Warnings = append(warnings, response.Warnings...)
	writeJSON(w, response)
}

func (h *PreviewHandler) HandleEvents(w http.ResponseWriter, r *http.Request) {
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

	last := h.changeToken()
	fmt.Fprintf(w, "event: ready\ndata: %s\n\n", strconv.Quote(last))
	flusher.Flush()

	ticker := time.NewTicker(900 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			next := h.changeToken()
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

func (h *PreviewHandler) load() (specProject, error) {
	token := h.changeToken()
	h.projectMu.RLock()
	if token == h.projectToken {
		project := h.project
		err := h.projectErr
		h.projectMu.RUnlock()
		return project, err
	}
	h.projectMu.RUnlock()

	project, err := scanSpecProject(h.projectRoot, h.docsDir)
	h.projectMu.Lock()
	h.projectToken = token
	h.project = project
	h.projectErr = err
	h.projectMu.Unlock()
	return project, err
}

func (h *PreviewHandler) changeToken() string {
	docRoot := docsRoot(h.projectRoot, h.docsDir)
	specToken := newestModToken(docRoot)
	return startToken + "|" + specToken
}

func (h *PreviewHandler) searchSnapshot(project specProject) previewSearchSnapshot {
	token := searchSnapshotToken(h.projectRoot, project.Summary.DocsRoot)
	h.searchMu.RLock()
	if token != "" && token == h.searchToken {
		snapshot := h.search
		h.searchMu.RUnlock()
		return snapshot
	}
	h.searchMu.RUnlock()

	docsDocs, docsWarnings := scanDocsSearchDocs(h.projectRoot, project.Summary.DocsRoot, project.Documents)
	codeDocs, codeWarnings := scanCodeSearchDocs(h.projectRoot, project.Summary.DocsRoot)
	snapshot := previewSearchSnapshot{
		Docs:     docsDocs,
		Code:     codeDocs,
		Warnings: append(docsWarnings, codeWarnings...),
	}
	h.searchMu.Lock()
	h.searchToken = token
	h.search = snapshot
	h.searchMu.Unlock()
	return snapshot
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
