package preview

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
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

const defaultPreviewAddr = "127.0.0.1:0"

type previewOptions struct {
	projectRoot string
	docsDir     string
	addr        string
	openBrowser bool
	noReload    bool
	quartzDir   string
}

// Run starts the docs preview as a Quartz dev server. It builds the docs and
// serves them locally, blocking until the server process exits.
func Run(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	opt := previewOptions{projectRoot: cwd, docsDir: "docs", addr: defaultPreviewAddr}
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	fs.StringVar(&opt.projectRoot, "project", opt.projectRoot, "project root to inspect")
	fs.StringVar(&opt.docsDir, "docs-dir", opt.docsDir, "docs directory relative to project root, or absolute path")
	fs.StringVar(&opt.addr, "addr", opt.addr, "local server address (host is ignored; Quartz binds to 127.0.0.1)")
	fs.BoolVar(&opt.openBrowser, "open", false, "open browser after the server starts")
	fs.BoolVar(&opt.noReload, "no-reload", false, "deprecated: Quartz dev server has its own hot reload")
	fs.StringVar(&opt.quartzDir, "quartz-dir", opt.quartzDir, "path to a local Quartz checkout (with package.json); if unset, Quartz is cloned to the user cache")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	opt.projectRoot = normalizePreviewProjectRoot(opt.projectRoot)

	port, err := previewPort(opt.addr)
	if err != nil {
		return err
	}
	wsPort, err := pickPreviewAddrForTest()
	if err != nil {
		return fmt.Errorf("allocate websocket port: %w", err)
	}

	repoDir, err := resolveQuartzRepoForTest(opt.quartzDir)
	if err != nil {
		return err
	}

	workspace, cleanup, err := prepareQuartzWorkspaceForTest(opt.projectRoot, opt.docsDir)
	if err != nil {
		return err
	}
	defer cleanup()

	docsAbs := docsRoot(opt.projectRoot, opt.docsDir)
	displayURL := "http://localhost:" + port
	fmt.Printf("docs preview (Quartz): %s\n", displayURL)
	fmt.Printf("project: %s\n", opt.projectRoot)
	fmt.Printf("docs: %s\n", docsAbs)

	if opt.openBrowser {
		// Open after a short delay so Quartz has time to start serving.
		go func() {
			time.Sleep(800 * time.Millisecond)
			if err := openURLForTest(displayURL); err != nil {
				fmt.Printf("open browser failed: %v\n", err)
			}
		}()
	}

	return runQuartzServeForTest(repoDir, workspace, port, portOf(wsPort), os.Stdout, os.Stderr)
}

// previewPort extracts or allocates a TCP port from the address string.
// Quartz always binds to 127.0.0.1, so only the port is used.
func previewPort(addr string) (string, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid preview address %q: %w", addr, err)
	}
	if port == "0" {
		return pickPreviewAddrForTest()
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

// runHotReloadSupervisor was used to watch preview frontend sources and
// rebuild/restart the server. The standalone preview UI has been removed, so
// this is now a no-op stub.
func runHotReloadSupervisor(moduleRoot string, args []string, projectRoot string) error {
	_ = moduleRoot
	_ = args
	_ = projectRoot
	return nil
}

type previewChildResult struct {
	err error
}

// buildPreviewFrontend was used to build the standalone preview frontend. It
// is now a no-op stub kept for compatibility with tests that exercise the
// function directly.
func buildPreviewFrontend(moduleRoot string) error {
	_ = moduleRoot
	return nil
}

// buildPreviewFrontendForTest lets tests stub the frontend build. It is now a
// no-op stub because the standalone preview frontend no longer exists.
var buildPreviewFrontendForTest = func(moduleRoot string) error {
	return buildPreviewFrontend(moduleRoot)
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

// startPreviewChildForTest lets tests replace the real `go run` child with a stub so
// runHotReloadSupervisor can be exercised without spawning Go subprocesses.
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

// stopPreviewChildForTest lets tests stub the child process killer so
// runHotReloadSupervisor's interrupt branch can be exercised without
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

// newPreviewServer creates a reusable docs preview API server. It is used by
// the search and graph commands; the interactive preview command now uses
// Quartz instead.
func newPreviewServer(opt previewOptions) *previewServer {
	ps := &previewServer{opt: opt}
	ps.handler = NewPreviewHandler(opt.projectRoot, opt.docsDir, nil)
	mux := http.NewServeMux()
	ps.handler.Register(mux, "/api/")
	ps.addr = opt.addr
	ps.srv = &http.Server{
		Addr:              opt.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return ps
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

// prepareQuartzWorkspaceForTest lets tests stub workspace preparation.
var prepareQuartzWorkspaceForTest = prepareQuartzWorkspace

// runQuartzServeForTest lets tests stub the Quartz serve command.
var runQuartzServeForTest = runQuartzServe

// ioDiscard is a typed alias for io.Discard so tests can reference it.
var ioDiscard = io.Discard
