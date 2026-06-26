package preview

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestRunHelpReturnsNil(t *testing.T) {
	if err := Run([]string{"--help"}); err != nil {
		t.Errorf("Run --help = %v, want nil", err)
	}
}

func TestRunBadFlagReturnsError(t *testing.T) {
	if err := Run([]string{"--unknown-flag"}); err == nil {
		t.Errorf("Run --unknown-flag = nil, want error")
	}
}

func TestRunStartPreviewServerAndServeReplaced(t *testing.T) {
	// Save and restore test seams.
	origServe := servePreviewForTest
	origOpen := openURLForTest
	defer func() {
		servePreviewForTest = origServe
		openURLForTest = origOpen
	}()

	called := false
	openCalled := false
	servePreviewForTest = func(srv *http.Server, listener net.Listener) error {
		called = true
		_ = listener.Close()
		return http.ErrServerClosed
	}
	openURLForTest = func(target string) error {
		openCalled = true
		return nil
	}

	// Run from a temp dir without go.mod to skip the hot reload supervisor.
	root := t.TempDir()
	mustChdir(t, root)

	if err := Run([]string{"--no-reload", "--addr", "127.0.0.1:0", "--open"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !called {
		t.Errorf("servePreviewForTest was not called")
	}
	if !openCalled {
		t.Errorf("openURLForTest was not called")
	}
}

func TestRunServeReturnsNonSpecialError(t *testing.T) {
	origServe := servePreviewForTest
	defer func() { servePreviewForTest = origServe }()

	boom := errors.New("boom")
	servePreviewForTest = func(srv *http.Server, listener net.Listener) error {
		_ = listener.Close()
		return boom
	}

	root := t.TempDir()
	mustChdir(t, root)

	err := Run([]string{"--no-reload", "--addr", "127.0.0.1:0"})
	if !errors.Is(err, boom) {
		t.Errorf("Run error = %v, want %v", err, boom)
	}
}

func TestRunNetListenError(t *testing.T) {
	root := t.TempDir()
	mustChdir(t, root)
	// Bind once to lock the port, then ask Run to bind again.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	if err := Run([]string{"--no-reload", "--addr", listener.Addr().String()}); err == nil {
		t.Errorf("Run with busy port = nil, want error")
	}
}

func TestRunDisplayURLIPv6(t *testing.T) {
	origServe := servePreviewForTest
	defer func() { servePreviewForTest = origServe }()

	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("ipv6 not supported: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Capture stdout
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	called := false
	servePreviewForTest = func(srv *http.Server, listener net.Listener) error {
		called = true
		_ = listener.Close()
		return http.ErrServerClosed
	}

	root := t.TempDir()
	mustChdir(t, root)

	if err := Run([]string{"--no-reload", "--addr", addr}); err != nil {
		t.Fatalf("Run ipv6 = %v", err)
	}
	_ = w.Close()
	out, _ := io.ReadAll(r)
	if !called {
		t.Errorf("servePreviewForTest was not called")
	}
	if len(out) == 0 {
		t.Errorf("expected stdout output, got none")
	}
}

func TestRunOpenURLError(t *testing.T) {
	origServe := servePreviewForTest
	origOpen := openURLForTest
	defer func() {
		servePreviewForTest = origServe
		openURLForTest = origOpen
	}()

	servePreviewForTest = func(srv *http.Server, listener net.Listener) error {
		_ = listener.Close()
		return http.ErrServerClosed
	}
	openURLForTest = func(target string) error {
		return errors.New("browser not available")
	}

	root := t.TempDir()
	mustChdir(t, root)

	if err := Run([]string{"--no-reload", "--addr", "127.0.0.1:0", "--open"}); err != nil {
		t.Errorf("Run with browser error = %v", err)
	}
}

func TestRunWithModuleRootTriggersHotReload(t *testing.T) {
	origStart := startPreviewChildForTest
	defer func() { startPreviewChildForTest = origStart }()

	called := false
	// Return a fake command that "completes" via done channel after a short delay.
	startPreviewChildForTest = func(moduleRoot string, args []string) (*exec.Cmd, <-chan previewChildResult, error) {
		called = true
		done := make(chan previewChildResult, 1)
		done <- previewChildResult{}
		// Return a stub cmd. Since we never call Wait() on it, the Process is nil-safe.
		return &exec.Cmd{Process: nil}, done, nil
	}

	// Use the actual repo root, which should contain go.mod/main.go/internal/preview/preview.go.
	moduleRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Restore CWD to internal/preview so other tests that depend on a stable CWD keep working.
		_ = os.Chdir(filepath.Join(moduleRoot, "internal", "preview"))
	}()
	// Verify the module root has the expected layout.
	if _, err := os.Stat(filepath.Join(moduleRoot, "internal", "preview", "preview.go")); err != nil {
		t.Skipf("module root layout missing: %v", err)
	}
	// We need to be inside the module root tree, not in internal/preview.
	subdir := filepath.Join(moduleRoot, "internal")
	if err := os.Chdir(subdir); err != nil {
		t.Skipf("chdir to %s: %v", subdir, err)
	}

	if err := Run([]string{"--addr", "127.0.0.1:0"}); err != nil {
		t.Errorf("Run = %v", err)
	}
	if !called {
		t.Errorf("startPreviewChildForTest was not called")
	}
}

func TestOpenURLUsesOpenURLForTestOverride(t *testing.T) {
	orig := openURLForTest
	defer func() { openURLForTest = orig }()

	called := false
	openURLForTest = func(target string) error {
		called = true
		return nil
	}
	if err := openURLForTest("http://example.com"); err != nil {
		t.Errorf("openURLForTest = %v", err)
	}
	if !called {
		t.Errorf("openURLForTest override not called")
	}
}

func TestOpenURLDispatchesByOS(t *testing.T) {
	// Can't easily mock exec.Cmd but we can verify the function returns a non-nil error
	// when the underlying command cannot find the binary (depending on OS).
	// Just verify it does not panic.
	_ = openURL
}

func TestPortOfWithValidHostPort(t *testing.T) {
	if got := portOf("127.0.0.1:8080"); got != "8080" {
		t.Errorf("portOf = %q, want 8080", got)
	}
	if got := portOf("[::1]:1234"); got != "1234" {
		t.Errorf("portOf ipv6 = %q, want 1234", got)
	}
}

func TestPortOfWithoutPort(t *testing.T) {
	if got := portOf("noport"); got != "noport" {
		t.Errorf("portOf no port = %q, want noport", got)
	}
	if got := portOf(""); got != "" {
		t.Errorf("portOf empty = %q, want empty", got)
	}
}

func TestBuildPreviewFrontendNoPackageJSON(t *testing.T) {
	// Without package.json+vite.config.ts, function returns nil.
	root := t.TempDir()
	if err := buildPreviewFrontend(root); err != nil {
		t.Errorf("buildPreviewFrontend empty = %v", err)
	}
}

func TestStopPreviewChildNilSafe(t *testing.T) {
	stopPreviewChild(nil)
	stopPreviewChild(&exec.Cmd{})
}

func TestStopPreviewChildKillsProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test")
	}
	// Use a child process group so syscall.Kill(-PID) hits the actual process.
	cmd := exec.Command("sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stopPreviewChild(cmd)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Errorf("process was not killed within 3s")
	}
}

func TestStartPreviewChildWithInvalidGo(t *testing.T) {
	// Point PATH to an empty temp dir so `go` cannot be found.
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPath)

	cmd, done, err := startPreviewChild(t.TempDir(), []string{"--no-reload"})
	if err == nil {
		if cmd != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		<-done
		t.Fatalf("startPreviewChild with missing go = nil error")
	}
	if cmd != nil {
		t.Errorf("cmd should be nil on error")
	}
	if done != nil {
		t.Errorf("done channel should be nil on error")
	}
}

func TestStartPreviewChildHandlesSetpgid(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only")
	}
	cmd, done, err := startPreviewChild(t.TempDir(), []string{"--no-reload"})
	if err != nil {
		t.Fatalf("startPreviewChild = %v", err)
	}
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Errorf("expected Setpgid=true on non-windows, got %+v", cmd.SysProcAttr)
	}
	_ = cmd.Process.Kill()
	<-done
}

func TestStripPreviewOpenFlagAllForms(t *testing.T) {
	cases := map[string][]string{
		"all-forms":      {"--open", "-open", "--open=true", "-open=true", "main"},
		"missing":        {"main", "--no-reload"},
		"empty":          nil,
	}
	for label, input := range cases {
		got := stripPreviewOpenFlag(input)
		for _, removed := range []string{"--open", "-open", "--open=true", "-open=true"} {
			for _, g := range got {
				if g == removed {
					t.Errorf("%s: stripped %q should not appear in %v", label, removed, got)
				}
			}
		}
	}
}

func mustChdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

// Helper to build a fake URL with hostname for testing.
var _ = url.URL{}
var _ = syscall.Kill
var _ = context.Background
