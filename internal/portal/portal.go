package portal

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

//go:embed portal_ui
var portalUIFS embed.FS

const defaultPortalAddr = "127.0.0.1:0"

type portalOptions struct {
	addr        string
	openBrowser bool
	noReload    bool
	agentsDir   string
}

// Run starts the portal web UI using the provided embedded presets.
func Run(args []string, presets fs.FS) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	portalPresetsForTest = presets
	opt := portalOptions{addr: defaultPortalAddr}
	fs := flag.NewFlagSet("portal", flag.ContinueOnError)
	fs.StringVar(&opt.addr, "addr", opt.addr, "local server address")
	fs.BoolVar(&opt.openBrowser, "open", false, "open browser after the server starts")
	fs.BoolVar(&opt.noReload, "no-reload", false, "disable source hot reload supervisor")
	fs.StringVar(&opt.agentsDir, "agents-home", "", "shared agents home")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if !opt.noReload {
		if root, ok := portalModuleRoot(cwd); ok {
			return runPortalHotReloadSupervisor(root, args)
		}
	}

	server, err := newPortalServer(presets, opt.agentsDir)
	if err != nil {
		return err
	}
	return servePortal(server, opt)
}

func servePortal(server *portalServer, opt portalOptions) error {
	static, err := fs.Sub(portalUIFS, "portal_ui")
	if err != nil {
		return err
	}
	mux := server.router()
	mux.Handle("/", spaFileServer(static))

	srv := &http.Server{
		Addr:              opt.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	listener, err := net.Listen("tcp", opt.addr)
	if err != nil {
		return err
	}
	addr := listener.Addr().String()
	displayURL := "http://" + addr
	if strings.HasPrefix(addr, "127.0.0.1:") || strings.HasPrefix(addr, "[::1]:") {
		displayURL = "http://localhost:" + portOf(addr)
	}
	fmt.Printf("portal: %s\n", displayURL)
	fmt.Printf("agents home: %s\n", server.agentsDir)

	if opt.openBrowser {
		if err := openURL(displayURL); err != nil {
			fmt.Printf("open browser failed: %v\n", err)
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(listener)
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	select {
	case err := <-done:
		return err
	case <-signals:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}

func portalModuleRoot(start string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		if fileExists(filepath.Join(dir, "go.mod")) && fileExists(filepath.Join(dir, "main.go")) && fileExists(filepath.Join(dir, "internal", "portal", "portal.go")) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func runPortalHotReloadSupervisor(moduleRoot string, args []string) error {
	fmt.Println("portal hot reload: watching Go backend and frontend sources")
	if err := buildPortalFrontend(moduleRoot); err != nil {
		return err
	}
	tokens := portalSourceTokens(moduleRoot)
	childArgs, err := portalChildArgs(args)
	if err != nil {
		return err
	}
	cmd, done, err := startPortalChild(moduleRoot, childArgs)
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
			stopPortalChild(cmd)
			<-done
			return nil
		case result := <-done:
			return result.err
		case <-ticker.C:
			nextTokens := portalSourceTokens(moduleRoot)
			if nextTokens == tokens {
				continue
			}
			frontendChanged := nextTokens.frontend != tokens.frontend
			tokens = nextTokens
			if frontendChanged {
				fmt.Println("portal hot reload: frontend changed, building portal assets")
				if err := buildPortalFrontend(moduleRoot); err != nil {
					fmt.Printf("portal hot reload: frontend build failed: %v\n", err)
					continue
				}
			}
			fmt.Println("portal hot reload: source changed, restarting")
			stopPortalChild(cmd)
			<-done
			childArgs = stripPortalOpenFlag(childArgs)
			cmd, done, err = startPortalChild(moduleRoot, childArgs)
			if err != nil {
				return err
			}
		}
	}
}

type portalChildResult struct {
	err error
}

func buildPortalFrontend(moduleRoot string) error {
	if !fileExists(filepath.Join(moduleRoot, "package.json")) || !fileExists(filepath.Join(moduleRoot, "vite.portal.config.ts")) {
		return nil
	}
	cmd := exec.Command("npm", "run", "build:portal")
	cmd.Dir = moduleRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func portalChildArgs(args []string) ([]string, error) {
	childArgs := stripPortalSupervisorFlags(args)
	if portalArgsHaveAddrFlag(childArgs) {
		return childArgs, nil
	}
	listener, err := net.Listen("tcp", defaultPortalAddr)
	if err != nil {
		return nil, err
	}
	defer listener.Close()
	addr := listener.Addr().String()
	return append(childArgs, "--addr", addr), nil
}

func startPortalChild(moduleRoot string, args []string) (*exec.Cmd, <-chan portalChildResult, error) {
	goArgs := append([]string{"run", ".", "portal", "--no-reload"}, args...)
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
	done := make(chan portalChildResult, 1)
	go func() {
		done <- portalChildResult{err: cmd.Wait()}
	}()
	return cmd, done, nil
}

func stopPortalChild(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if runtime.GOOS != "windows" {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		return
	}
	_ = cmd.Process.Kill()
}

func stripPortalSupervisorFlags(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--no-reload" || strings.HasPrefix(arg, "--no-reload=") {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func stripPortalOpenFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--open" || arg == "-open" || strings.HasPrefix(arg, "--open=") || strings.HasPrefix(arg, "-open=") {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func portalArgsHaveAddrFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--addr" || arg == "-addr" || strings.HasPrefix(arg, "--addr=") || strings.HasPrefix(arg, "-addr=") {
			return true
		}
	}
	return false
}

type portalSourceTokensValue struct {
	backend  string
	frontend string
}

func portalSourceTokens(moduleRoot string) portalSourceTokensValue {
	var backendNewest int64
	var backendCount int
	var frontendNewest int64
	var frontendCount int

	uiRoot := filepath.Join(moduleRoot, "internal", "portal", "portal_ui")
	uiSourceRoot := filepath.Join(moduleRoot, "internal", "portal", "portal_ui_src")

	_ = filepath.WalkDir(moduleRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path == moduleRoot || path == uiSourceRoot {
				return nil
			}
			if path == uiRoot {
				return filepath.SkipDir
			}
			name := d.Name()
			switch name {
			case ".git", ".agents", ".codex", "docs", "node_modules", "tmp", "vendor":
				return filepath.SkipDir
			}
			if strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			if strings.HasPrefix(path, uiSourceRoot+string(os.PathSeparator)) {
				return nil
			}
			return nil
		}
		rel, err := filepath.Rel(moduleRoot, path)
		if err != nil {
			return nil
		}
		kind, ok := portalSourceFileKind(rel, path, uiSourceRoot)
		if !ok {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		switch kind {
		case portalSourceFrontend:
			frontendCount++
			if mod := info.ModTime().UnixNano(); mod > frontendNewest {
				frontendNewest = mod
			}
		default:
			backendCount++
			if mod := info.ModTime().UnixNano(); mod > backendNewest {
				backendNewest = mod
			}
		}
		return nil
	})
	return portalSourceTokensValue{
		backend:  fmt.Sprintf("%d:%d", backendNewest, backendCount),
		frontend: fmt.Sprintf("%d:%d", frontendNewest, frontendCount),
	}
}

type portalSourceKind int

const (
	portalSourceBackend portalSourceKind = iota
	portalSourceFrontend
)

func portalSourceFileKind(rel, path, uiSourceRoot string) (portalSourceKind, bool) {
	if rel == "go.mod" || rel == "go.sum" || filepath.Ext(path) == ".go" {
		return portalSourceBackend, true
	}
	switch rel {
	case "package.json", "package-lock.json", "biome.json", "tsconfig.portal.json", "vite.portal.config.ts", "eslint.config.mjs", ".prettierrc.json", ".prettierignore":
		return portalSourceFrontend, true
	}
	if strings.HasPrefix(path, uiSourceRoot+string(os.PathSeparator)) {
		return portalSourceFrontend, true
	}
	return portalSourceBackend, false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func portOf(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return port
}

func openURL(target string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{target}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", target}
	default:
		name, args = "xdg-open", []string{target}
	}
	return exec.Command(name, args...).Start()
}

// portalPresetsForTest is replaced by tests. Production code uses the presets
// FS passed to Run, which is currently embedded in main. Because portal is
// compiled into the same binary as main, we use the package-level variable set
// by Run before creating the server.
var portalPresetsForTest fs.FS
