package preview

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/ngosangns/ns-workspace/internal/internalutil"
)

//go:embed preview_ui
var previewUIFS embed.FS

const defaultPreviewAddr = "127.0.0.1:0"

type previewOptions struct {
	projectRoot string
	docsDir     string
	addr        string
	openBrowser bool
	noReload    bool
	quartzDir   string // deprecated; accepted for CLI compatibility and ignored
}

// Run starts the docs preview SolidJS SPA with the Go PreviewHandler API.
// It binds a local HTTP server and blocks until the server exits.
func Run(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	opt := previewOptions{projectRoot: cwd, docsDir: "docs", addr: defaultPreviewAddr}
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	fs.StringVar(&opt.projectRoot, "project", opt.projectRoot, "project root to inspect")
	fs.StringVar(&opt.docsDir, "docs-dir", opt.docsDir, "docs directory relative to project root, or absolute path")
	fs.StringVar(&opt.addr, "addr", opt.addr, "local server address")
	fs.BoolVar(&opt.openBrowser, "open", false, "open browser after the server starts")
	fs.BoolVar(&opt.noReload, "no-reload", false, "disable source hot reload supervisor when running from a checkout")
	fs.StringVar(&opt.quartzDir, "quartz-dir", opt.quartzDir, "deprecated: Quartz is no longer used; flag is ignored")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	opt.projectRoot = normalizePreviewProjectRoot(opt.projectRoot)
	if strings.TrimSpace(opt.quartzDir) != "" {
		fmt.Fprintf(os.Stderr, "preview: --quartz-dir is deprecated and ignored (Solid SPA preview)\n")
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

	docsAbs := docsRoot(opt.projectRoot, opt.docsDir)
	fmt.Printf("docs preview: %s\n", displayURL)
	fmt.Printf("project: %s\n", opt.projectRoot)
	fmt.Printf("docs: %s\n", docsAbs)

	if opt.openBrowser {
		go func() {
			if err := waitForServerAndOpen(displayURL); err != nil {
				fmt.Printf("open browser failed: %v\n", err)
			}
		}()
	}

	if err := servePreviewForTest(server.srv, listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// waitForServerAndOpen polls the display URL until it responds, then opens it
// in the system browser. It gives up after a timeout so the preview command
// does not hang indefinitely if the server fails to start.
func waitForServerAndOpen(url string) error {
	if err := waitForServerForTest(url, 30*time.Second); err != nil {
		return err
	}
	return openURLForTest(url)
}

// waitForServerForTest lets tests stub the readiness poll.
var waitForServerForTest = func(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("server did not become ready at %s within %v", url, timeout)
}

// previewPort extracts or allocates a TCP port from the address string.
func previewPort(addr string) (string, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid preview address %q: %w", addr, err)
	}
	if port == "0" {
		picked, err := pickPreviewAddrForTest()
		if err != nil {
			return "", err
		}
		port = portOf(picked)
	}
	return port, nil
}

// previewModuleRoot detects the module root for the legacy hot-reload
// supervisor. It is kept for compatibility with existing tests.
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

type previewChildResult struct {
	err error
}

func previewChildArgs(args []string, projectRoot string) ([]string, error) {
	childArgs := stripPreviewSupervisorFlags(args)
	childArgs = stripPreviewProjectFlag(childArgs)
	childArgs = append(childArgs, "--project", projectRoot)
	if previewArgsHaveAddrFlag(childArgs) {
		return childArgs, nil
	}
	addr, err := pickPreviewAddrForTest()
	if err != nil {
		return nil, err
	}
	return append(childArgs, "--addr", addr), nil
}

// pickPreviewAddrForTest lets tests stub the address picker so the error branch
// in previewChildArgs can be exercised without exhausting real ports.
var pickPreviewAddrForTest = pickPreviewAddr

func normalizePreviewProjectRoot(path string) string {
	path = internalutil.ExpandPath(path)
	if abs, err := filepathAbsForTest(path); err == nil {
		return abs
	}
	return path
}

// filepathAbsForTest lets tests stub filepath.Abs so the normally-unreachable
// error branch in normalizePreviewProjectRoot can be exercised.
var filepathAbsForTest = func(path string) (string, error) {
	return filepath.Abs(path)
}

func previewArgsHaveAddrFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--addr" || arg == "-addr" || strings.HasPrefix(arg, "--addr=") || strings.HasPrefix(arg, "-addr=") {
			return true
		}
	}
	return false
}

func pickPreviewAddr() (string, error) {
	return pickPreviewAddrAt(defaultPreviewAddr)
}

// pickPreviewAddrAt is a testable variant that takes an explicit address so the
// error branch (already-in-use) can be exercised by binding the address first.
func pickPreviewAddrAt(addr string) (string, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}
	defer listener.Close()
	return listener.Addr().String(), nil
}

// startPreviewChildForTest lets tests replace the real `go run` child with a stub.
var startPreviewChildForTest = startPreviewChild

func startPreviewChild(moduleRoot string, args []string) (*exec.Cmd, <-chan previewChildResult, error) {
	goArgs := append([]string{"run", ".", "preview", "--no-reload"}, args...)
	cmd := exec.Command("go", goArgs...)
	cmd.Dir = moduleRoot
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if runtimeGOOSForTest != "windows" {
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
	stopPreviewChildForTest(cmd)
}

// stopPreviewChildForTest lets tests stub the child process killer without
// actually sending SIGKILL to the test process.
var stopPreviewChildForTest = func(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if runtimeGOOSForTest != "windows" {
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

func stripPreviewProjectFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--project" || arg == "-project" {
			i++
			continue
		}
		if strings.HasPrefix(arg, "--project=") || strings.HasPrefix(arg, "-project=") {
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
	tokens := previewSourceTokens(moduleRoot)
	return tokens.backend + "|" + tokens.frontend
}

func previewSourceTokens(moduleRoot string) previewSourceTokensValue {
	var backendNewest int64
	var backendCount int
	walkPreviewSource(moduleRoot, func(path string, info os.FileInfo, kind previewSourceKind) {
		switch kind {
		case previewSourceFrontend:
			// No standalone preview frontend to track anymore.
		default:
			backendCount++
			if mod := info.ModTime().UnixNano(); mod > backendNewest {
				backendNewest = mod
			}
		}
	})
	return previewSourceTokensValue{
		backend:  fmt.Sprintf("%d:%d", backendNewest, backendCount),
		frontend: "0:0",
	}
}

type previewSourceTokensValue struct {
	backend  string
	frontend string
}

type previewSourceKind int

const (
	previewSourceBackend previewSourceKind = iota
	previewSourceFrontend
)

func walkPreviewSource(moduleRoot string, visit func(string, os.FileInfo, previewSourceKind)) {
	_ = filepath.WalkDir(moduleRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path == moduleRoot {
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
			return nil
		}
		rel, err := filepath.Rel(moduleRoot, path)
		if err != nil {
			return nil
		}
		kind, ok := previewSourceFileKind(rel, path)
		if !ok {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		visit(path, info, kind)
		return nil
	})
}

func isPreviewSourceFile(rel, path string) bool {
	_, ok := previewSourceFileKind(rel, path)
	return ok
}

func previewSourceFileKind(rel, path string) (previewSourceKind, bool) {
	if rel == "go.mod" || rel == "go.sum" || filepath.Ext(path) == ".go" {
		return previewSourceBackend, true
	}
	switch rel {
	case "package.json", "package-lock.json", "biome.json", "tsconfig.portal.json", "vite.portal.config.ts", "eslint.config.mjs", ".prettierrc.json", ".prettierignore":
		return previewSourceBackend, true
	}
	return previewSourceBackend, false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// newPreviewServer creates a docs preview server that serves the embedded
// SolidJS SPA and the PreviewHandler REST/SSE API under /api/.
func newPreviewServer(opt previewOptions) *previewServer {
	ps := &previewServer{opt: opt}
	ps.handler = NewPreviewHandler(opt.projectRoot, opt.docsDir, nil)
	mux := http.NewServeMux()
	ps.handler.Register(mux, "/api/")
	static, err := fs.Sub(previewUIFS, "preview_ui")
	if err == nil {
		mux.Handle("/", previewSpaFileServer(static))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "preview UI not embedded", http.StatusInternalServerError)
		})
	}
	ps.addr = opt.addr
	ps.srv = &http.Server{
		Addr:              opt.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return ps
}

// previewSpaFileServer serves the embedded SPA: real files when present, otherwise index.html.
func previewSpaFileServer(static fs.FS) http.Handler {
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
		if path == "favicon.svg" || path == "style.css" || strings.HasPrefix(path, "assets/") {
			http.NotFound(w, r)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		files.ServeHTTP(w, r2)
	})
}

type previewServer struct {
	opt     previewOptions
	srv     *http.Server
	handler *PreviewHandler
	addr    string
}

func (ps *previewServer) shutdown(ctx context.Context) error {
	if ps.handler != nil {
		_ = ps.handler.Close(ctx)
	}
	return ps.srv.Shutdown(ctx)
}

var startToken = fmt.Sprintf("%d", time.Now().UnixNano())

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

// newestEmbeddedModToken previously walked the embedded preview UI assets. The
// standalone UI has been removed, so it now returns a stable token.
func newestEmbeddedModToken() string {
	return "portal"
}

// openURL opens the system browser. The variable openURLForTest lets tests
// override this without spawning a real browser process.
var openURLForTest = openURL

// servePreviewForTest lets tests substitute the blocking http.Serve call so the
// rest of Run can be exercised without hanging the test process.
var servePreviewForTest = func(srv *http.Server, listener net.Listener) error {
	return srv.Serve(listener)
}

func openURL(target string) error {
	var name string
	var args []string
	switch runtimeGOOSForTest {
	case "darwin":
		name, args = "open", []string{target}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", target}
	default:
		name, args = "xdg-open", []string{target}
	}
	return openURLCmdForTest(name, args...).Start()
}

// runtimeGOOSForTest lets tests stub runtime.GOOS so all branches of openURL
// can be exercised without build-tagged files.
var runtimeGOOSForTest = runtime.GOOS

// openURLCmdForTest lets tests substitute exec.Command construction.
var openURLCmdForTest = exec.Command

func portOf(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return port
}

// ioDiscard is a typed alias for io.Discard so tests can reference it.
var ioDiscard = io.Discard
