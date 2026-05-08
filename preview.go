package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	opt             previewOptions
	srv             *http.Server
	mermaidRenderer mermaidRenderer
}

type mermaidRenderer func(context.Context, string, string) (string, error)

type mermaidRenderRequest struct {
	Source string `json:"source"`
	Theme  string `json:"theme,omitempty"`
}

type mermaidRenderResponse struct {
	SVG string `json:"svg"`
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
	ps := &previewServer{opt: opt, mermaidRenderer: renderMermaidSVG}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/project", ps.handleProject)
	mux.HandleFunc("/api/specs", ps.handleSpecs)
	mux.HandleFunc("/api/specs/", ps.handleSpec)
	mux.HandleFunc("/api/graph", ps.handleGraph)
	mux.HandleFunc("/api/render/mermaid", ps.handleRenderMermaid)
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

func (ps *previewServer) handleRenderMermaid(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req mermaidRenderRequest
	r.Body = http.MaxBytesReader(w, r.Body, 512*1024)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Source) == "" {
		http.Error(w, "mermaid source is required", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	svg, err := ps.mermaidRenderer(ctx, req.Source, req.Theme)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, mermaidRenderResponse{SVG: svg})
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

func renderMermaidSVG(ctx context.Context, source, theme string) (string, error) {
	theme = mermaidTheme(theme)
	if _, err := exec.LookPath("mmdc"); err == nil {
		return renderMermaidSVGWithMMDC(ctx, source, theme)
	}
	return renderMermaidSVGWithInk(ctx, source, theme)
}

func renderMermaidSVGWithMMDC(ctx context.Context, source, theme string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "ns-workspace-mermaid-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "diagram.mmd")
	outputPath := filepath.Join(tmpDir, "diagram.svg")
	if err := os.WriteFile(inputPath, []byte(source), 0o600); err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, "mmdc", "-i", inputPath, "-o", outputPath, "-b", "transparent", "-t", theme)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("mermaid render failed: %v\n%s", err, strings.TrimSpace(output.String()))
	}

	svg, err := os.ReadFile(outputPath)
	if err != nil {
		return "", err
	}
	return string(svg), nil
}

func renderMermaidSVGWithInk(ctx context.Context, source, theme string) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"code": source,
		"mermaid": map[string]string{
			"theme": theme,
		},
	})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://mermaid.ink/svg/base64:"+encoded+"?bgColor=transparent", nil)
	if err != nil {
		return "", err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("mermaid render failed: install mmdc or check Mermaid Ink connectivity: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4*1024*1024))
	if err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("mermaid render failed: Mermaid Ink returned %s\n%s", res.Status, strings.TrimSpace(string(body)))
	}
	return string(body), nil
}

func mermaidTheme(theme string) string {
	if strings.EqualFold(theme, "dark") {
		return "dark"
	}
	return "default"
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
