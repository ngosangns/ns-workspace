package preview

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

//go:embed preview_ui/*
var previewUIFS embed.FS

type previewOptions struct {
	projectRoot string
	docsDir     string
	addr        string
	openBrowser bool
	noReload    bool
}

type previewServer struct {
	opt previewOptions
	srv *http.Server
}

func Run(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	opt := previewOptions{projectRoot: cwd, docsDir: "docs", addr: "127.0.0.1:8787"}
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	fs.StringVar(&opt.projectRoot, "project", opt.projectRoot, "project root to inspect")
	fs.StringVar(&opt.docsDir, "docs-dir", opt.docsDir, "docs directory relative to project root, or absolute path")
	fs.StringVar(&opt.addr, "addr", opt.addr, "local server address")
	fs.BoolVar(&opt.openBrowser, "open", false, "open browser after the server starts")
	fs.BoolVar(&opt.noReload, "no-reload", false, "disable source hot reload supervisor")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if !opt.noReload {
		if root, ok := previewModuleRoot(cwd); ok {
			return runHotReloadSupervisor(root, args)
		}
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
	fmt.Printf("docs preview: %s\n", displayURL)
	fmt.Printf("project: %s\n", opt.projectRoot)
	fmt.Printf("docs: %s\n", docsRoot(opt.projectRoot, opt.docsDir))
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

func previewModuleRoot(start string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		if fileExists(filepath.Join(dir, "go.mod")) && fileExists(filepath.Join(dir, "main.go")) && fileExists(filepath.Join(dir, "internal", "preview", "preview.go")) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func runHotReloadSupervisor(moduleRoot string, args []string) error {
	fmt.Println("docs preview hot reload: watching Go backend and frontend sources")
	token := previewSourceToken(moduleRoot)
	childArgs := stripPreviewSupervisorFlags(args)
	cmd, done, err := startPreviewChild(moduleRoot, childArgs)
	if err != nil {
		return err
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	ticker := time.NewTicker(700 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-signals:
			stopPreviewChild(cmd)
			<-done
			return nil
		case result := <-done:
			if result.err != nil {
				return result.err
			}
			return nil
		case <-ticker.C:
			nextToken := previewSourceToken(moduleRoot)
			if nextToken == token {
				continue
			}
			token = nextToken
			fmt.Println("docs preview hot reload: source changed, restarting")
			stopPreviewChild(cmd)
			<-done
			childArgs = stripPreviewOpenFlag(childArgs)
			cmd, done, err = startPreviewChild(moduleRoot, childArgs)
			if err != nil {
				return err
			}
		}
	}
}

type previewChildResult struct {
	err error
}

func startPreviewChild(moduleRoot string, args []string) (*exec.Cmd, <-chan previewChildResult, error) {
	goArgs := append([]string{"run", ".", "preview", "--no-reload"}, args...)
	cmd := exec.Command("go", goArgs...)
	cmd.Dir = moduleRoot
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	done := make(chan previewChildResult, 1)
	go func() {
		done <- previewChildResult{err: cmd.Wait()}
	}()
	return cmd, done, nil
}

func stopPreviewChild(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if runtime.GOOS != "windows" {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		return
	}
	_ = cmd.Process.Kill()
}

func stripPreviewSupervisorFlags(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--no-reload" {
			continue
		}
		if strings.HasPrefix(arg, "--no-reload=") {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func stripPreviewOpenFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--open" {
			continue
		}
		if arg == "-open" {
			continue
		}
		if strings.HasPrefix(arg, "--open=") || strings.HasPrefix(arg, "-open=") {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func previewSourceToken(moduleRoot string) string {
	var newest int64
	var count int
	walkPreviewSource(moduleRoot, func(path string, info fs.FileInfo) {
		count++
		if mod := info.ModTime().UnixNano(); mod > newest {
			newest = mod
		}
	})
	return fmt.Sprintf("%d:%d", newest, count)
}

func walkPreviewSource(moduleRoot string, visit func(string, fs.FileInfo)) {
	uiRoot := filepath.Join(moduleRoot, "internal", "preview", "preview_ui")
	_ = filepath.WalkDir(moduleRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path == moduleRoot || path == uiRoot {
				return nil
			}
			name := d.Name()
			switch name {
			case ".git", ".agents", ".codex", "docs", "graphify-out", "node_modules", "tmp", "vendor":
				return filepath.SkipDir
			}
			if strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			if strings.HasPrefix(path, uiRoot+string(os.PathSeparator)) {
				return nil
			}
			return nil
		}
		rel, err := filepath.Rel(moduleRoot, path)
		if err != nil {
			return nil
		}
		if rel != "go.mod" && rel != "go.sum" && filepath.Ext(path) != ".go" && !strings.HasPrefix(path, uiRoot+string(os.PathSeparator)) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		visit(path, info)
		return nil
	})
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func newPreviewServer(opt previewOptions) *previewServer {
	ps := &previewServer{opt: opt}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/project", ps.handleProject)
	mux.HandleFunc("/api/docs", ps.handleSpecs)
	mux.HandleFunc("/api/docs/", ps.handleSpec)
	mux.HandleFunc("/api/files", ps.handleFile)
	mux.HandleFunc("/api/graph", ps.handleGraph)
	mux.HandleFunc("/api/search", ps.handleSearch)
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
	return scanSpecProject(ps.opt.projectRoot, ps.opt.docsDir)
}

func (ps *previewServer) changeToken() string {
	docRoot := docsRoot(ps.opt.projectRoot, ps.opt.docsDir)
	specToken := newestModToken(docRoot)
	staticToken := newestEmbeddedModToken()
	return startToken + "|" + specToken + "|" + staticToken
}

var startToken = fmt.Sprintf("%d", time.Now().UnixNano())

func expandPath(path string) string {
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
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
