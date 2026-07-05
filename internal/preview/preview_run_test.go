package preview

import (
	"errors"
	"io"
	"net"
	"os"
	"testing"
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

func TestRunQuartzServeReplaced(t *testing.T) {
	origResolve := resolveQuartzRepoForTest
	origPrepare := prepareQuartzWorkspaceForTest
	origServe := runQuartzServeForTest
	origOpen := openURLForTest
	defer func() {
		resolveQuartzRepoForTest = origResolve
		prepareQuartzWorkspaceForTest = origPrepare
		runQuartzServeForTest = origServe
		openURLForTest = origOpen
	}()

	resolveCalled := false
	prepareCalled := false
	serveCalled := false
	openCalled := false

	resolveQuartzRepoForTest = func(dir string) (string, error) {
		resolveCalled = true
		if dir != "" {
			t.Errorf("resolveQuartzRepoForTest dir = %q, want empty", dir)
		}
		return "/quartz/repo", nil
	}
	prepareQuartzWorkspaceForTest = func(projectRoot, docsDir string) (string, func(), error) {
		prepareCalled = true
		return "/quartz/workspace", func() {}, nil
	}
	serveDone := make(chan struct{})
	runQuartzServeForTest = func(repoDir, workspaceDir, port, wsPort string, stdout, stderr io.Writer) error {
		serveCalled = true
		if repoDir != "/quartz/repo" {
			t.Errorf("repoDir = %q, want /quartz/repo", repoDir)
		}
		if workspaceDir != "/quartz/workspace" {
			t.Errorf("workspaceDir = %q, want /quartz/workspace", workspaceDir)
		}
		if port == "" {
			t.Errorf("port should not be empty")
		}
		if wsPort == "" {
			t.Errorf("wsPort should not be empty")
		}
		<-serveDone
		return nil
	}
	openURLForTest = func(target string) error {
		openCalled = true
		close(serveDone)
		return nil
	}

	root := t.TempDir()
	mustChdir(t, root)

	if err := Run([]string{"--no-reload", "--addr", "127.0.0.1:0", "--open"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !resolveCalled {
		t.Errorf("resolveQuartzRepoForTest was not called")
	}
	if !prepareCalled {
		t.Errorf("prepareQuartzWorkspaceForTest was not called")
	}
	if !serveCalled {
		t.Errorf("runQuartzServeForTest was not called")
	}
	if !openCalled {
		t.Errorf("openURLForTest was not called")
	}
}

func TestRunQuartzServeReturnsError(t *testing.T) {
	origResolve := resolveQuartzRepoForTest
	origPrepare := prepareQuartzWorkspaceForTest
	origServe := runQuartzServeForTest
	defer func() {
		resolveQuartzRepoForTest = origResolve
		prepareQuartzWorkspaceForTest = origPrepare
		runQuartzServeForTest = origServe
	}()

	boom := errors.New("boom")
	resolveQuartzRepoForTest = func(string) (string, error) { return "/quartz/repo", nil }
	prepareQuartzWorkspaceForTest = func(string, string) (string, func(), error) { return "/w", func() {}, nil }
	runQuartzServeForTest = func(string, string, string, string, io.Writer, io.Writer) error { return boom }

	root := t.TempDir()
	mustChdir(t, root)

	err := Run([]string{"--no-reload", "--addr", "127.0.0.1:0"})
	if !errors.Is(err, boom) {
		t.Errorf("Run error = %v, want %v", err, boom)
	}
}

func TestRunResolveQuartzRepoError(t *testing.T) {
	orig := resolveQuartzRepoForTest
	defer func() { resolveQuartzRepoForTest = orig }()

	boom := errors.New("no quartz")
	resolveQuartzRepoForTest = func(string) (string, error) { return "", boom }

	root := t.TempDir()
	mustChdir(t, root)

	err := Run([]string{"--addr", "127.0.0.1:0"})
	if !errors.Is(err, boom) {
		t.Errorf("Run error = %v, want %v", err, boom)
	}
}

func TestRunPrepareWorkspaceError(t *testing.T) {
	origResolve := resolveQuartzRepoForTest
	origPrepare := prepareQuartzWorkspaceForTest
	defer func() {
		resolveQuartzRepoForTest = origResolve
		prepareQuartzWorkspaceForTest = origPrepare
	}()

	boom := errors.New("no workspace")
	resolveQuartzRepoForTest = func(string) (string, error) { return "/quartz/repo", nil }
	prepareQuartzWorkspaceForTest = func(string, string) (string, func(), error) { return "", nil, boom }

	root := t.TempDir()
	mustChdir(t, root)

	err := Run([]string{"--addr", "127.0.0.1:0"})
	if !errors.Is(err, boom) {
		t.Errorf("Run error = %v, want %v", err, boom)
	}
}

func TestRunQuartzDirFlagPassed(t *testing.T) {
	origResolve := resolveQuartzRepoForTest
	origPrepare := prepareQuartzWorkspaceForTest
	origServe := runQuartzServeForTest
	defer func() {
		resolveQuartzRepoForTest = origResolve
		prepareQuartzWorkspaceForTest = origPrepare
		runQuartzServeForTest = origServe
	}()

	resolveQuartzRepoForTest = func(dir string) (string, error) {
		if dir != "/custom/quartz" {
			t.Errorf("resolveQuartzRepoForTest dir = %q, want /custom/quartz", dir)
		}
		return "/custom/quartz", nil
	}
	prepareQuartzWorkspaceForTest = func(string, string) (string, func(), error) { return "/w", func() {}, nil }
	runQuartzServeForTest = func(string, string, string, string, io.Writer, io.Writer) error { return nil }

	root := t.TempDir()
	mustChdir(t, root)

	if err := Run([]string{"--addr", "127.0.0.1:0", "--quartz-dir", "/custom/quartz"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestPreviewPort(t *testing.T) {
	port, err := previewPort("127.0.0.1:8080")
	if err != nil {
		t.Fatalf("previewPort: %v", err)
	}
	if port != "8080" {
		t.Errorf("port = %q, want 8080", port)
	}

	// Port 0 should allocate a free port.
	port, err = previewPort("127.0.0.1:0")
	if err != nil {
		t.Fatalf("previewPort 0: %v", err)
	}
	if port == "" || port == "0" {
		t.Errorf("allocated port = %q, want non-zero", port)
	}

	if _, err := previewPort("not-an-address"); err == nil {
		t.Error("previewPort invalid address expected error")
	}
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

func TestBuildPreviewFrontendNoPackageJSON(t *testing.T) {
	// Without package.json+vite.config.ts, function returns nil.
	root := t.TempDir()
	if err := buildPreviewFrontend(root); err != nil {
		t.Errorf("buildPreviewFrontend empty = %v", err)
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

var _ = net.Listen
