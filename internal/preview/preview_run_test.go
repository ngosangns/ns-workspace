package preview

import (
	"io"
	"net"
	"net/http"
	"os"
	"strings"
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

func TestRunServesSPAAndAPI(t *testing.T) {
	origServe := servePreviewForTest
	origOpen := openURLForTest
	origWait := waitForServerForTest
	defer func() {
		servePreviewForTest = origServe
		openURLForTest = origOpen
		waitForServerForTest = origWait
	}()

	serveCalled := false
	openCalled := false

	waitForServerForTest = func(string, time.Duration) error { return nil }
	servePreviewForTest = func(srv *http.Server, ln net.Listener) error {
		serveCalled = true
		// Serve briefly so SPA + API can be exercised, then shut down.
		errCh := make(chan error, 1)
		go func() { errCh <- srv.Serve(ln) }()

		addr := ln.Addr().String()
		base := "http://" + addr
		resp, err := http.Get(base + "/")
		if err != nil {
			t.Errorf("GET /: %v", err)
		} else {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if !strings.Contains(string(body), "app") {
				snippet := string(body)
				if len(snippet) > 200 {
					snippet = snippet[:200]
				}
				t.Errorf("SPA shell missing app mount: %q", snippet)
			}
		}
		resp2, err := http.Get(base + "/api/project")
		if err != nil {
			t.Errorf("GET /api/project: %v", err)
		} else {
			_ = resp2.Body.Close()
			if resp2.StatusCode != http.StatusOK {
				t.Errorf("api/project status = %d, want 200", resp2.StatusCode)
			}
		}
		_ = srv.Close()
		<-errCh
		return http.ErrServerClosed
	}
	openURLForTest = func(target string) error {
		openCalled = true
		return nil
	}

	root := t.TempDir()
	if err := os.MkdirAll(root+"/docs", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root+"/docs/_index.md", []byte("# Index\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Run([]string{"--no-reload", "--addr", "127.0.0.1:0", "--open", "--project", root}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !serveCalled {
		t.Errorf("servePreviewForTest was not called")
	}
	if !openCalled {
		t.Errorf("openURLForTest was not called")
	}
}

func TestRunListenError(t *testing.T) {
	// Bind a port then ask Run to use the same address so Listen fails.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	root := t.TempDir()
	err = Run([]string{"--addr", addr, "--project", root})
	if err == nil {
		t.Errorf("Run with busy addr = nil, want error")
	}
}

func TestRunQuartzDirDeprecatedIgnored(t *testing.T) {
	origServe := servePreviewForTest
	defer func() { servePreviewForTest = origServe }()

	servePreviewForTest = func(srv *http.Server, ln net.Listener) error {
		_ = ln.Close()
		return http.ErrServerClosed
	}

	root := t.TempDir()
	_ = os.MkdirAll(root+"/docs", 0o755)

	// Capture stderr so we prove the shipped Run path still accepts the flag
	// and emits the deprecation warning (Quartz helpers are gone; flag kept).
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStderr := os.Stderr
	os.Stderr = w
	runErr := Run([]string{"--addr", "127.0.0.1:0", "--quartz-dir", "/custom/quartz", "--project", root})
	_ = w.Close()
	os.Stderr = origStderr
	stderrBytes, _ := io.ReadAll(r)
	_ = r.Close()
	if runErr != nil {
		t.Fatalf("Run with deprecated --quartz-dir: %v", runErr)
	}
	stderr := string(stderrBytes)
	if !strings.Contains(stderr, "--quartz-dir is deprecated") {
		t.Fatalf("expected deprecation warning on stderr, got %q", stderr)
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

	port, err = previewPort("127.0.0.1:0")
	if err != nil {
		t.Fatalf("previewPort 0: %v", err)
	}
	if port == "" || port == "0" {
		t.Errorf("allocated port = %q, want non-zero", port)
	}
}

func TestPreviewSpaFileServerIndexFallback(t *testing.T) {
	// Ensure embedded preview_ui is present after frontend build.
	data, err := previewUIFS.ReadFile("preview_ui/index.html")
	if err != nil {
		t.Fatalf("embedded preview_ui/index.html: %v", err)
	}
	if !strings.Contains(string(data), "app") {
		t.Errorf("embedded index.html missing app root")
	}
}

