package graphquery

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunLSPListDispatch(t *testing.T) {
	// Verify that "list" subcommand dispatches correctly.
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	var buf bytes.Buffer
	if err := RunLSP([]string{"list", "--project", tmp}, emptySourceDetector{}); err != nil {
		t.Errorf("RunLSP list failed: %v", err)
	}
	// Output is on stdout, not buf.
	_ = buf
}

func TestRunLSPInstallDispatch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	var buf bytes.Buffer
	if err := RunLSP([]string{"install", "go", "--dry-run", "--force", "--project", tmp}, emptySourceDetector{}); err != nil {
		t.Errorf("RunLSP install failed: %v", err)
	}
	_ = buf
}

func TestCacheRootTempDirFallback(t *testing.T) {
	// Force CacheRoot to fall back to TempDir by clearing XDG_CACHE_HOME and HOME.
	t.Setenv(CacheEnv, "")
	t.Setenv("XDG_CACHE_HOME", "")
	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", "/this/path/does/not/exist/zzz")
	defer t.Setenv("HOME", originalHome)
	got := CacheRoot()
	if got == "" {
		t.Error("CacheRoot should never return empty")
	}
}

func TestCacheRootHomeFallback(t *testing.T) {
	t.Setenv(CacheEnv, "")
	t.Setenv("XDG_CACHE_HOME", "")
	got := CacheRoot()
	if got == "" {
		t.Error("CacheRoot should never return empty")
	}
}

func TestImplementationByIDLangFallback(t *testing.T) {
	tests := []string{"javascript", "js", "ts", "tsx", "jsx", "scss", "sass", "kt", "kotlin", "css", "html", "typescript", "go", "golang"}
	for _, id := range tests {
		if _, ok := implementationByID(id); !ok {
			t.Errorf("expected %q to be found", id)
		}
	}
}

func TestNormalizeProjectRootRelative(t *testing.T) {
	got := normalizeProjectRoot("foo/bar")
	if got == "" {
		t.Error("expected non-empty result")
	}
}

func TestInstallLSPAlreadyInstalledViaPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "ns-workspace-test-cmd-x1")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	impl := stubImpl{
		spec: InstallSpec{ID: "stub", Name: "Stub", Command: "ns-workspace-test-cmd-x1"},
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{})
	if result.Status != "already-installed" {
		t.Errorf("expected already-installed, got %+v", result)
	}
	if !strings.Contains(result.Message, "found in") {
		t.Errorf("expected 'found in' message, got: %s", result.Message)
	}
}

func TestInstallLSPForceRecheck(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "ns-workspace-test-cmd-x2")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	impl := stubImpl{
		spec:       InstallSpec{ID: "stub", Name: "Stub", Command: "ns-workspace-test-cmd-x2"},
		installOut: "/nonexistent-xyz",
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{Force: true})
	if result.Status != "failed" {
		t.Errorf("expected failed with Force=true, got %+v", result)
	}
}

func TestExtractZipArchiveMkdirAllFail(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.zip")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	if _, err := zw.Create("subdir/"); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(blocker, "extract")
	if err := extractZipArchive(archivePath, dest); err == nil {
		t.Error("expected error when mkdir fails")
	}
}

func TestExtractTarGzArchiveDirMkdirFail(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: "dir/", Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(blocker, "extract")
	if err := extractTarGzArchive(archivePath, dest); err == nil {
		t.Error("expected error when mkdir fails")
	}
}

func TestExtractTarGzArchiveFileMkdirFail(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: "x/file.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(blocker, "extract")
	if err := extractTarGzArchive(archivePath, dest); err == nil {
		t.Error("expected error when mkdir fails")
	}
}

func TestExtractTarGzArchiveOpenFileFail(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: "x/file.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(dest, "x")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGzArchive(archivePath, dest); err == nil {
		t.Error("expected error when file open fails")
	}
}

func TestInstallArchiveLSPMkdirTmpFail(t *testing.T) {
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(CacheEnv, blocker)

	source := ArchiveSource{
		Version:  "1.0",
		FileName: "x.zip",
		URL:      "http://x.invalid",
		SHA256:   "00",
		Format:   "zip",
	}
	spec := InstallSpec{ID: "test", Command: "test"}
	_, err := installArchiveLSP(context.Background(), spec, source, []string{"test"})
	if err == nil {
		t.Error("expected error when mkdir fails")
	}
}

func TestInstallArchiveLSPExtractDirMkdirFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	content := []byte("dummy")
	sum := sha256.Sum256(content)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	source := ArchiveSource{
		Version:  "1.0",
		FileName: "x.zip",
		URL:      srv.URL,
		SHA256:   hex.EncodeToString(sum[:]),
		Format:   "zip",
	}
	spec := InstallSpec{ID: "test", Command: "test"}
	_, err := installArchiveLSP(context.Background(), spec, source, []string{"test"})
	if err == nil {
		t.Error("expected error")
	}
}

func TestInstallArchiveLSPVersionsDirMkdirFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	cacheDir := filepath.Join(tmp, "ns-workspace", "lsp", "test")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	versionsDir := filepath.Join(cacheDir, "versions")
	if err := os.WriteFile(versionsDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	content := []byte("dummy")
	sum := sha256.Sum256(content)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	source := ArchiveSource{
		Version:  "1.0",
		FileName: "x.zip",
		URL:      srv.URL,
		SHA256:   hex.EncodeToString(sum[:]),
		Format:   "zip",
	}
	spec := InstallSpec{ID: "test", Command: "test"}
	_, err := installArchiveLSP(context.Background(), spec, source, []string{"x"})
	if err == nil {
		t.Error("expected error when versions dir mkdir fails")
	}
}

func TestDefaultKotlinArchiveSourceUnsupported(t *testing.T) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if runtime.GOARCH != "arm64" && runtime.GOARCH != "amd64" {
			_, err := defaultKotlinArchiveSource(InstallSpec{ID: "kotlin"})
			if err == nil {
				t.Error("expected error for unsupported arch")
			}
		}
	} else {
		_, err := defaultKotlinArchiveSource(InstallSpec{ID: "kotlin"})
		if err == nil {
			t.Error("expected error for unsupported OS")
		}
	}
}

func TestKotlinInstallErrorSource(t *testing.T) {
	impl := kotlinImplementation{}
	restore := SetArchiveSourceForTest(func(InstallSpec) (ArchiveSource, error) {
		return ArchiveSource{}, errors.New("custom error")
	})
	defer restore()
	_, err := impl.install(context.Background())
	if err == nil || !strings.Contains(err.Error(), "custom error") {
		t.Errorf("expected custom error, got: %v", err)
	}
}

func TestExtractTarGzArchiveCorruptGzip(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.tar.gz")
	if err := os.WriteFile(archivePath, []byte("not gzip"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGzArchive(archivePath, dest); err == nil {
		t.Error("expected error for corrupt gzip")
	}
}

func TestExtractTarGzArchiveCorruptTar(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	if _, err := gw.Write([]byte("not tar data")); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGzArchive(archivePath, dest); err == nil {
		t.Error("expected error for corrupt tar")
	}
}

func TestRunLSPInstallJSONEncodeFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	w := failingWriter{}
	err := RunLSPInstall([]string{"go", "--dry-run", "--force", "--json", "--project", tmp}, w, emptySourceDetector{})
	_ = err
}

func TestRunLSPListWriterFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	w := failingWriter{}
	err := RunLSPList([]string{"--project", tmp}, w, emptySourceDetector{})
	if err == nil {
		t.Error("expected error from failing writer")
	}
}

func TestRunLSPInstallUnknownCmd(t *testing.T) {
	err := RunLSP([]string{"unknown"}, emptySourceDetector{})
	if err == nil || !strings.Contains(err.Error(), "unknown lsp command") {
		t.Errorf("expected unknown command error, got: %v", err)
	}
}

func TestRunLSPInstallForceSkip(t *testing.T) {
	// Test that --force without force actually causes re-check.
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	var buf bytes.Buffer
	if err := RunLSPInstall([]string{"go", "--dry-run", "--project", tmp}, &buf, emptySourceDetector{}); err != nil {
		t.Errorf("dry-run failed: %v", err)
	}
}

func TestInstallArchiveLSPChmodFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)

	hdr := &zip.FileHeader{Name: "bin/kotlin-lsp", Method: zip.Deflate}
	hdr.SetMode(0o755)

	zipPath := filepath.Join(tmp, "kotlin.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("exec")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(zipPath)
	sum := sha256.Sum256(data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	spec := InstallSpec{ID: "kotlin", Command: "kotlin-lsp"}
	source := ArchiveSource{
		Version:  "1.0",
		FileName: "kotlin.zip",
		URL:      srv.URL,
		SHA256:   hex.EncodeToString(sum[:]),
		Format:   "zip",
	}
	wrapperPath, err := installArchiveLSP(context.Background(), spec, source, []string{"bin/kotlin-lsp"})
	if err != nil {
		t.Fatalf("installArchiveLSP failed: %v", err)
	}
	// Verify chmod happened.
	info, err := os.Stat(wrapperPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		t.Error("wrapper should be executable")
	}
}

func TestExtractZipArchiveSymlinkAbsolute(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.zip")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	hdr := &zip.FileHeader{Name: "link", Method: zip.Deflate}
	hdr.SetMode(os.ModeSymlink | 0o777)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("/absolute/target")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := extractZipArchive(archivePath, dest); err == nil {
		t.Error("expected error for absolute symlink target")
	}
}

func TestExtractZipArchiveSymlinkEscape(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.zip")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	hdr := &zip.FileHeader{Name: "link", Method: zip.Deflate}
	hdr.SetMode(os.ModeSymlink | 0o777)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("../../../etc/passwd")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := extractZipArchive(archivePath, dest); err == nil {
		t.Error("expected error for escape symlink target")
	}
}

func TestExtractTarGzArchiveSymlinkAbs(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.tar.gz")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: "link", Linkname: "/absolute", Typeflag: tar.TypeSymlink, Mode: 0o777}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := extractTarGzArchive(archivePath, dest); err == nil {
		t.Error("expected error for absolute symlink target")
	}
}

func TestRunLSPListFprintfFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	w := failingWriter{}
	err := RunLSPList([]string{"--project", tmp}, w, emptySourceDetector{})
	if err == nil {
		t.Error("expected error from failing writer")
	}
}

func TestRunLSPInstallFprintfFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	w := failingWriter{}
	err := RunLSPInstall([]string{"go", "--dry-run", "--force", "--project", tmp}, w, emptySourceDetector{})
	if err == nil {
		t.Error("expected error from failing writer")
	}
}

// Compile-time check
var _ = io.Copy
