package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed preview_ui/*
var previewUIFS embed.FS

type previewOptions struct {
	projectRoot string
	specsDir    string
	addr        string
	openBrowser bool
}

type previewServer struct {
	opt            previewOptions
	srv            *http.Server
	likeC4Renderer likeC4Renderer
}

type likeC4Renderer func(context.Context, string) ([]likeC4Diagram, error)

type likeC4RenderRequest struct {
	Source string `json:"source"`
}

type likeC4RenderResponse struct {
	Diagrams []likeC4Diagram `json:"diagrams"`
}

type likeC4ModelsResponse struct {
	Projects []likeC4ModelProject `json:"projects"`
}

type likeC4ModelProject struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Root        string          `json:"root"`
	SourceFiles []string        `json:"sourceFiles"`
	Generated   bool            `json:"generated"`
	Source      string          `json:"source,omitempty"`
	Diagrams    []likeC4Diagram `json:"diagrams,omitempty"`
	Error       string          `json:"error,omitempty"`
}

type likeC4Diagram struct {
	Name    string `json:"name"`
	Mermaid string `json:"mermaid"`
}

func runPreview(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	opt := previewOptions{projectRoot: cwd, specsDir: "specs", addr: "127.0.0.1:8787"}
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	fs.StringVar(&opt.projectRoot, "project", opt.projectRoot, "project root to inspect")
	fs.StringVar(&opt.specsDir, "specs-dir", opt.specsDir, "specs directory relative to project root, or absolute path")
	fs.StringVar(&opt.addr, "addr", opt.addr, "local server address")
	fs.BoolVar(&opt.openBrowser, "open", false, "open browser after the server starts")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	opt.projectRoot = expandPath(opt.projectRoot)
	if abs, err := filepath.Abs(opt.projectRoot); err == nil {
		opt.projectRoot = abs
	}

	server := newPreviewServer(opt)
	listener, err := net.Listen("tcp", opt.addr)
	if err != nil {
		return err
	}
	addr := listener.Addr().String()
	displayURL := "http://" + addr
	if strings.HasPrefix(addr, "127.0.0.1:") || strings.HasPrefix(addr, "[::1]:") {
		displayURL = "http://localhost:" + portOf(addr)
	}
	fmt.Printf("spec preview: %s\n", displayURL)
	fmt.Printf("project: %s\n", opt.projectRoot)
	fmt.Printf("specs: %s\n", specsRoot(opt.projectRoot, opt.specsDir))
	if opt.openBrowser {
		if err := openURL(displayURL); err != nil {
			fmt.Printf("open browser failed: %v\n", err)
		}
	}
	if err := server.srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func newPreviewServer(opt previewOptions) *previewServer {
	ps := &previewServer{opt: opt, likeC4Renderer: renderLikeC4Mermaid}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/project", ps.handleProject)
	mux.HandleFunc("/api/specs", ps.handleSpecs)
	mux.HandleFunc("/api/specs/", ps.handleSpec)
	mux.HandleFunc("/api/graph", ps.handleGraph)
	mux.HandleFunc("/api/likec4", ps.handleLikeC4Models)
	mux.HandleFunc("/api/render/likec4", ps.handleRenderLikeC4)
	mux.HandleFunc("/api/events", ps.handleEvents)
	static, _ := fs.Sub(previewUIFS, "preview_ui")
	mux.Handle("/", spaFileServer(static))
	ps.srv = &http.Server{
		Addr:              opt.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return ps
}

func spaFileServer(static fs.FS) http.Handler {
	files := http.FileServer(http.FS(static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			files.ServeHTTP(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(static, path); err == nil {
			files.ServeHTTP(w, r)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		files.ServeHTTP(w, r2)
	})
}

func (ps *previewServer) shutdown(ctx context.Context) error {
	return ps.srv.Shutdown(ctx)
}

func (ps *previewServer) load() (specProject, error) {
	return scanSpecProject(ps.opt.projectRoot, ps.opt.specsDir)
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

func (ps *previewServer) handleRenderLikeC4(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req likeC4RenderRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Source) == "" {
		http.Error(w, "likec4 source is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	diagrams, err := ps.likeC4Renderer(ctx, req.Source)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, likeC4RenderResponse{Diagrams: diagrams})
}

func (ps *previewServer) handleLikeC4Models(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	project, err := ps.load()
	if err != nil {
		writeAPIError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	models := discoverLikeC4ModelProjects(ps.opt.projectRoot)
	if len(models) == 0 {
		if generated := buildSpecLikeC4ModelProject(project); generated.Source != "" {
			diagrams, err := ps.likeC4Renderer(ctx, generated.Source)
			if err != nil {
				generated.Error = err.Error()
			} else {
				generated.Diagrams = diagrams
			}
			models = append(models, generated)
		}
	} else {
		for i := range models {
			diagrams, err := renderLikeC4Workspace(ctx, models[i].Root)
			if err != nil {
				models[i].Error = err.Error()
				continue
			}
			models[i].Diagrams = diagrams
		}
	}
	writeJSON(w, likeC4ModelsResponse{Projects: models})
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

func (ps *previewServer) changeToken() string {
	specRoot := specsRoot(ps.opt.projectRoot, ps.opt.specsDir)
	specToken := newestModToken(specRoot)
	staticToken := newestEmbeddedModToken()
	return specToken + "|" + staticToken
}

func newestModToken(root string) string {
	var newest int64
	var count int
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		count++
		if mod := info.ModTime().UnixNano(); mod > newest {
			newest = mod
		}
		return nil
	})
	return fmt.Sprintf("%d:%d", newest, count)
}

func newestEmbeddedModToken() string {
	var newest int64
	var count int
	_ = fs.WalkDir(previewUIFS, "preview_ui", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		count++
		if mod := info.ModTime().UnixNano(); mod > newest {
			newest = mod
		}
		return nil
	})
	return fmt.Sprintf("%d:%d", newest, count)
}

func renderLikeC4Mermaid(ctx context.Context, source string) ([]likeC4Diagram, error) {
	tmpDir, err := os.MkdirTemp("", "ns-workspace-likec4-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	sourcePath := filepath.Join(tmpDir, "workspace.c4")
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		return nil, err
	}
	return renderLikeC4Workspace(ctx, tmpDir)
}

func renderLikeC4Workspace(ctx context.Context, workspaceDir string) ([]likeC4Diagram, error) {
	tmpDir, err := os.MkdirTemp("", "ns-workspace-likec4-out-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	outDir := filepath.Join(tmpDir, "out")
	cmd := exec.CommandContext(ctx, "npx", "--yes", "likec4", "gen", "mermaid", workspaceDir, "-o", outDir)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("likec4 gen mermaid failed: %v\n%s", err, strings.TrimSpace(output.String()))
	}

	var files []string
	if err := filepath.WalkDir(outDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".mmd" {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("likec4 did not generate any Mermaid diagrams")
	}

	diagrams := make([]likeC4Diagram, 0, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		name, err := filepath.Rel(outDir, path)
		if err != nil {
			name = filepath.Base(path)
		}
		name = strings.TrimSuffix(filepath.ToSlash(name), ".mmd")
		diagrams = append(diagrams, likeC4Diagram{Name: name, Mermaid: string(data)})
	}
	return diagrams, nil
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

func openURL(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}

func portOf(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return port
}
