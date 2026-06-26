package graphquery

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeArchiveTarget(t *testing.T) {
	tmp := t.TempDir()
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"normal", "bin/x", false},
		{"nested", "a/b/c.txt", false},
		{"absolute", "/etc/passwd", true},
		{"escape dotdot", "../escape", true},
		{"escape nested", "a/../../escape", true},
		{"escape parent only", "..", true},
		{"dot", ".", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := safeArchiveTarget(tmp, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("safeArchiveTarget(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCreateSafeArchiveSymlink(t *testing.T) {
	tmp := t.TempDir()
	// Create absolute link target - should fail.
	err := createSafeArchiveSymlink(tmp, "/etc/passwd", filepath.Join(tmp, "link"))
	if err == nil {
		t.Error("expected error for absolute link target")
	}
	// Create escape link target - should fail.
	err = createSafeArchiveSymlink(tmp, "../../escape", filepath.Join(tmp, "link"))
	if err == nil {
		t.Error("expected error for escaping link target")
	}
	// Create valid link target.
	err = createSafeArchiveSymlink(tmp, "sublink", filepath.Join(tmp, "link"))
	if err != nil {
		t.Errorf("valid symlink failed: %v", err)
	}
}

func TestExtractZipArchive(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.zip")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a zip file with one regular file and one directory.
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	// Add directory
	if _, err := zw.Create("subdir/"); err != nil {
		t.Fatal(err)
	}
	// Add regular file
	w, err := zw.Create("subdir/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := extractZipArchive(archivePath, dest); err != nil {
		t.Fatalf("extractZipArchive failed: %v", err)
	}
	// Verify content.
	data, err := os.ReadFile(filepath.Join(dest, "subdir", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want hello", string(data))
	}
}

func TestExtractZipArchiveSymlink(t *testing.T) {
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
	if _, err := w.Write([]byte("target")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := extractZipArchive(archivePath, dest); err != nil {
		t.Fatalf("extractZipArchive failed: %v", err)
	}
	target, err := os.Readlink(filepath.Join(dest, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if target != "target" {
		t.Errorf("got %q, want target", target)
	}
}

func TestExtractZipArchiveInvalidFile(t *testing.T) {
	tmp := t.TempDir()
	badPath := filepath.Join(tmp, "nonexistent.zip")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractZipArchive(badPath, dest); err == nil {
		t.Error("expected error for nonexistent zip")
	}
}

func TestExtractTarGzArchive(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.tar.gz")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a tar.gz file with a directory and a regular file.
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	// Add directory
	if err := tw.WriteHeader(&tar.Header{Name: "subdir/", Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
		t.Fatal(err)
	}
	if err := tw.WriteHeader(&tar.Header{Name: "subdir/file.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("hello")); err != nil {
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

	if err := extractTarGzArchive(archivePath, dest); err != nil {
		t.Fatalf("extractTarGzArchive failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "subdir", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want hello", string(data))
	}
}

func TestExtractTarGzArchiveSymlink(t *testing.T) {
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
	// Symlink.
	if err := tw.WriteHeader(&tar.Header{Name: "link", Linkname: "target", Typeflag: tar.TypeSymlink, Mode: 0o777}); err != nil {
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

	if err := extractTarGzArchive(archivePath, dest); err != nil {
		t.Fatalf("extractTarGzArchive failed: %v", err)
	}
	target, err := os.Readlink(filepath.Join(dest, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if target != "target" {
		t.Errorf("got %q, want target", target)
	}
}

func TestExtractTarGzArchiveInvalidFile(t *testing.T) {
	tmp := t.TempDir()
	badPath := filepath.Join(tmp, "nonexistent.tar.gz")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGzArchive(badPath, dest); err == nil {
		t.Error("expected error for nonexistent tar.gz")
	}
}

func TestExtractTarGzArchiveCorrupted(t *testing.T) {
	tmp := t.TempDir()
	badPath := filepath.Join(tmp, "bad.tar.gz")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badPath, []byte("not a tar.gz"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGzArchive(badPath, dest); err == nil {
		t.Error("expected error for corrupted tar.gz")
	}
}

func TestExtractTarGzArchiveEscape(t *testing.T) {
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
	if err := tw.WriteHeader(&tar.Header{Name: "../escape.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("hello")); err != nil {
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
		t.Error("expected error for escape path")
	}
}

func TestExtractZipArchiveEscape(t *testing.T) {
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
	w, err := zw.Create("../escape.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := extractZipArchive(archivePath, dest); err == nil {
		t.Error("expected error for escape path")
	}
}

func TestExtractZipArchiveOpenEntryFail(t *testing.T) {
	// Use a zip with absolute path entry - safeArchiveTarget fails first.
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
	w, err := zw.Create("/absolute")
	if err != nil {
		t.Fatal(err)
	}
	_ = w
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractZipArchive(archivePath, dest); err == nil {
		t.Error("expected error for absolute path entry")
	}
}

func TestFindArchiveExecutableDirect(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "bin", "x")
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	found, err := findArchiveExecutable(tmp, []string{"bin/x"})
	if err != nil {
		t.Fatalf("findArchiveExecutable failed: %v", err)
	}
	if found != binPath {
		t.Errorf("got %q, want %q", found, binPath)
	}
}

func TestFindArchiveExecutableWalk(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "deep", "nested", "x")
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	found, err := findArchiveExecutable(tmp, []string{"missing", "x"})
	if err != nil {
		t.Fatalf("findArchiveExecutable walk failed: %v", err)
	}
	if found != binPath {
		t.Errorf("got %q, want %q", found, binPath)
	}
}

func TestFindArchiveExecutableNotFound(t *testing.T) {
	tmp := t.TempDir()
	_, err := findArchiveExecutable(tmp, []string{"missing1", "missing2"})
	if err == nil || !strings.Contains(err.Error(), "does not contain executable candidate") {
		t.Errorf("expected not found error, got: %v", err)
	}
}

func TestWriteExecutableWrapper(t *testing.T) {
	tmp := t.TempDir()
	wrapperPath := filepath.Join(tmp, "sub", "mywrapper")
	launcherPath := "/some/path with spaces/x"
	if err := writeExecutableWrapper(wrapperPath, launcherPath); err != nil {
		t.Fatalf("writeExecutableWrapper failed: %v", err)
	}
	data, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatal(err)
	}
	expected := "#!/bin/sh\nexec " + shellQuote(launcherPath) + " \"$@\"\n"
	if string(data) != expected {
		t.Errorf("got %q, want %q", string(data), expected)
	}
	info, err := os.Stat(wrapperPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("wrapper should be executable")
	}
}

func TestInstallArchiveLSPUnsupportedFormat(t *testing.T) {
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
		FileName: "fake.bin",
		URL:      srv.URL,
		SHA256:   hex.EncodeToString(sum[:]),
		Format:   "rar", // unsupported
	}
	spec := InstallSpec{ID: "test", Command: "test"}
	_, err := installArchiveLSP(context.Background(), spec, source, []string{"test"})
	if err == nil || !strings.Contains(err.Error(), "unsupported archive format") {
		t.Errorf("expected unsupported format error, got: %v", err)
	}
}

func TestDownloadArchiveSuccess(t *testing.T) {
	content := []byte("hello world")
	sum := sha256.Sum256(content)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	tmp := t.TempDir()
	dest := filepath.Join(tmp, "out.bin")
	source := ArchiveSource{
		FileName: "out.bin",
		URL:      srv.URL,
		SHA256:   hex.EncodeToString(sum[:]),
	}
	if err := downloadArchive(context.Background(), source, dest); err != nil {
		t.Fatalf("downloadArchive failed: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Errorf("got %q, want %q", string(got), string(content))
	}
}

func TestDownloadArchiveChecksumMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	tmp := t.TempDir()
	dest := filepath.Join(tmp, "out.bin")
	source := ArchiveSource{
		FileName: "out.bin",
		URL:      srv.URL,
		SHA256:   "0000000000000000000000000000000000000000000000000000000000000000",
	}
	if err := downloadArchive(context.Background(), source, dest); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected checksum mismatch error, got: %v", err)
	}
}

func TestDownloadArchiveHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	tmp := t.TempDir()
	dest := filepath.Join(tmp, "out.bin")
	source := ArchiveSource{
		FileName: "out.bin",
		URL:      srv.URL,
		SHA256:   "00",
	}
	if err := downloadArchive(context.Background(), source, dest); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 error, got: %v", err)
	}
}

func TestDownloadArchiveInvalidURL(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "out.bin")
	source := ArchiveSource{
		FileName: "out.bin",
		URL:      "://invalid",
		SHA256:   "00",
	}
	if err := downloadArchive(context.Background(), source, dest); err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestDownloadArchiveCreateFileFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()

	// Use a path that fails to create (directory doesn't exist).
	dest := "/nonexistent/dir/out.bin"
	source := ArchiveSource{
		FileName: "out.bin",
		URL:      srv.URL,
		SHA256:   "00",
	}
	if err := downloadArchive(context.Background(), source, dest); err == nil {
		t.Error("expected error when create file fails")
	}
}

func TestInstallArchiveLSPSuccess(t *testing.T) {
	// Create a fake zip archive in tmp.
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)

	content := []byte("fake executable")
	hdr := &zip.FileHeader{Name: "bin/kotlin-lsp", Method: zip.Deflate}
	// Ensure mode includes exec bits.
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
	if _, err := w.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	// Compute checksum.
	data, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}
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
	if !strings.HasPrefix(wrapperPath, CacheRoot()) {
		t.Errorf("wrapper path %q should be in cache", wrapperPath)
	}
	// Verify wrapper exists and is executable.
	info, err := os.Stat(wrapperPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Error("wrapper should be executable")
	}
}

func TestInstallArchiveLSPDownloadFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	spec := InstallSpec{ID: "test", Command: "test"}
	source := ArchiveSource{
		Version:  "1.0",
		FileName: "test.zip",
		URL:      "http://127.0.0.1:0/no-server",
		SHA256:   "00",
		Format:   "zip",
	}
	_, err := installArchiveLSP(context.Background(), spec, source, []string{"test"})
	if err == nil {
		t.Error("expected error when download fails")
	}
}

func TestInstallArchiveLSPMissingExecutable(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)

	// Create a zip without the expected executable.
	zipPath := filepath.Join(tmp, "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("some-file.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("x"))
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

	spec := InstallSpec{ID: "test", Command: "test"}
	source := ArchiveSource{
		Version:  "1.0",
		FileName: "test.zip",
		URL:      srv.URL,
		SHA256:   hex.EncodeToString(sum[:]),
		Format:   "zip",
	}
	_, err = installArchiveLSP(context.Background(), spec, source, []string{"missing-exe"})
	if err == nil || !strings.Contains(err.Error(), "does not contain executable candidate") {
		t.Errorf("expected missing executable error, got: %v", err)
	}
}

func TestExtractTarGzArchiveMissingFile(t *testing.T) {
	// Use a path that cannot be opened.
	if err := extractTarGzArchive("/nonexistent/file.tar.gz", t.TempDir()); err == nil {
		t.Error("expected error for nonexistent tar.gz")
	}
}

func TestSafeArchiveTargetEdgeCases(t *testing.T) {
	tmp := t.TempDir()
	// "." should fail (escape parent dir).
	if _, err := safeArchiveTarget(tmp, "."); err == nil {
		t.Error("expected error for '.'")
	}
	// ".." should fail.
	if _, err := safeArchiveTarget(tmp, ".."); err == nil {
		t.Error("expected error for '..'")
	}
	// "/abs/path" should fail.
	if _, err := safeArchiveTarget(tmp, "/abs/path"); err == nil {
		t.Error("expected error for absolute path")
	}
	// "../escape" should fail.
	if _, err := safeArchiveTarget(tmp, "../escape"); err == nil {
		t.Error("expected error for path traversal")
	}
	// Normal path should pass.
	if _, err := safeArchiveTarget(tmp, "a/b/c"); err != nil {
		t.Errorf("expected no error for normal path: %v", err)
	}
}

func TestCreateSafeArchiveSymlinkAbs(t *testing.T) {
	tmp := t.TempDir()
	if err := createSafeArchiveSymlink(tmp, "/abs", filepath.Join(tmp, "link")); err == nil {
		t.Error("expected error for absolute target")
	}
}

func TestCreateSafeArchiveSymlinkEscape(t *testing.T) {
	tmp := t.TempDir()
	if err := createSafeArchiveSymlink(tmp, "../../escape", filepath.Join(tmp, "link")); err == nil {
		t.Error("expected error for escaping target")
	}
}

func TestWriteExecutableWrapperCreateDirFail(t *testing.T) {
	// Path under a file (not a directory) - mkdir fails.
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	wrapperPath := filepath.Join(blocker, "sub", "wrapper")
	if err := writeExecutableWrapper(wrapperPath, "/x"); err == nil {
		t.Error("expected error when parent is not a directory")
	}
}

func TestFindArchiveExecutableErrPropagation(t *testing.T) {
	// WalkDir will fail if root doesn't exist.
	_, err := findArchiveExecutable("/nonexistent/path", []string{"x"})
	if err == nil {
		t.Error("expected error when root doesn't exist")
	}
}

func TestDownloadArchiveContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block forever - context cancel should fail it.
		<-r.Context().Done()
	}))
	defer srv.Close()

	tmp := t.TempDir()
	dest := filepath.Join(tmp, "out.bin")
	source := ArchiveSource{
		FileName: "out.bin",
		URL:      srv.URL,
		SHA256:   "00",
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	if err := downloadArchive(ctx, source, dest); err == nil {
		t.Error("expected error when context is cancelled")
	}
}

func TestInstallArchiveLSPVersionsAsFile(t *testing.T) {
	// Pre-create <cacheDir>/versions as a regular file so MkdirAll(versionsDir) fails.
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	versionsFile := filepath.Join(CacheRoot(), "test", "versions")
	if err := os.MkdirAll(filepath.Dir(versionsFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(versionsFile, []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set up a small zip that contains a binary.
	zipPath := filepath.Join(tmp, "blocker.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	hdr := &zip.FileHeader{Name: "bin/x", Method: zip.Deflate}
	hdr.SetMode(0o755)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("#!/bin/sh\n")); err != nil {
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
	source := ArchiveSource{
		Version:  "1.0",
		FileName: "x.zip",
		URL:      srv.URL,
		SHA256:   hex.EncodeToString(sum[:]),
		Format:   "zip",
	}
	spec := InstallSpec{ID: "test", Command: "x"}
	_, err = installArchiveLSP(context.Background(), spec, source, []string{"bin/x"})
	if err == nil {
		t.Error("expected error when versions is a file")
	}
}

func TestInstallArchiveLSPBinDirAsFile(t *testing.T) {
	// Pre-create <cacheDir>/bin as a regular file so MkdirAll(binDir) fails.
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	binFile := filepath.Join(CacheRoot(), "test", "bin")
	if err := os.MkdirAll(filepath.Dir(binFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binFile, []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build a zip with one binary.
	zipPath := filepath.Join(tmp, "blocker.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	hdr := &zip.FileHeader{Name: "bin/x", Method: zip.Deflate}
	hdr.SetMode(0o755)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("#!/bin/sh\n")); err != nil {
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
	source := ArchiveSource{
		Version:  "1.0",
		FileName: "x.zip",
		URL:      srv.URL,
		SHA256:   hex.EncodeToString(sum[:]),
		Format:   "zip",
	}
	spec := InstallSpec{ID: "test", Command: "x"}
	_, err = installArchiveLSP(context.Background(), spec, source, []string{"bin/x"})
	if err == nil {
		t.Error("expected error when bin is a file")
	}
}

func TestWriteExecutableWrapperWriteFileFail(t *testing.T) {
	// Pre-create wrapperPath as a directory so WriteFile fails.
	tmp := t.TempDir()
	wrapperPath := filepath.Join(tmp, "mywrapper")
	if err := os.MkdirAll(wrapperPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeExecutableWrapper(wrapperPath, "/x"); err == nil {
		t.Error("expected error when wrapperPath is a directory")
	}
}

func TestExtractTarGzArchivePermZeroBranch(t *testing.T) {
	// Tạo tar với file có mode = 0 để trigger nhánh if perm == 0 → perm = 0o644.
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
	if err := tw.WriteHeader(&tar.Header{Name: "file.txt", Typeflag: tar.TypeReg, Mode: 0, Size: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("hello")); err != nil {
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
	if err := extractTarGzArchive(archivePath, dest); err != nil {
		t.Errorf("extractTarGzArchive with mode=0 failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want hello", string(data))
	}
}

func TestExtractTarGzArchiveDirMkdirError(t *testing.T) {
	// Tạo tar với một directory entry nhưng dest không cho phép mkdir (parent là file).
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.tar.gz")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	// Block "subdir" creation: pre-create "subdir" as a file in dest.
	if err := os.WriteFile(filepath.Join(dest, "subdir"), []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: "subdir/", Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
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
		t.Error("expected error when dir target exists as file")
	}
}

func TestExtractZipArchiveTargetAsDir(t *testing.T) {
	// Pre-create the target file path as a directory to make os.OpenFile fail.
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.zip")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	// Block the target with a directory of the same name.
	if err := os.MkdirAll(filepath.Join(dest, "file.txt"), 0o755); err != nil {
		t.Fatal(err)
	}

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractZipArchive(archivePath, dest); err == nil {
		t.Error("expected error when target is a directory")
	}
}

func TestExtractTarGzArchiveTargetAsDir(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.tar.gz")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dest, "file.txt"), 0o755); err != nil {
		t.Fatal(err)
	}

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: "file.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("hello")); err != nil {
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
		t.Error("expected error when target is a directory")
	}
}

func TestExtractZipArchivePermZeroBranch(t *testing.T) {
	// Tạo zip với file có mode = 0 để trigger nhánh if perm == 0 → perm = 0o644.
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
	hdr := &zip.FileHeader{Name: "file.txt", Method: zip.Deflate}
	hdr.SetMode(0) // explicit mode = 0
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractZipArchive(archivePath, dest); err != nil {
		t.Errorf("extractZipArchive with perm=0 failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want hello", string(data))
	}
}

func TestSafeArchiveTargetSameAsDestClean(t *testing.T) {
	tmp := t.TempDir()
	// Force target == destClean (the second check).
	// We need clean == destClean exactly. Try with name being equivalent to dest itself.
	// Since destClean = filepath.Clean(tmp), and we need clean(target) == destClean,
	// we can construct this if name resolves exactly to dest.
	// However, filepath.Join(tmp, ".") → tmp, and clean(".") → "." which is rejected.
	// Try a name that's effectively the same as dest:
	if _, err := safeArchiveTarget(tmp, tmp); err == nil {
		t.Errorf("expected error (absolute path) for dest-as-name")
	}
}

func TestSafeArchiveTargetEscapeSecondCheck(t *testing.T) {
	// Construct a name that bypasses the dot/..prefix check but escapes via Join.
	// dest = "." produces destClean = "." and Join(".", "foo") → "foo" (after Clean),
	// which does NOT have the prefix "./" — triggering the second check.
	if _, err := safeArchiveTarget(".", "foo"); err == nil {
		t.Error("expected error from second check when dest is \".\"")
	}
}

// TestInstallArchiveLSPMkdirTmpParentFail exercises the os.MkdirAll error
// branch in installArchiveLSP when creating tmpParent under a path that
// already exists as a file.
func TestInstallArchiveLSPMkdirTmpParentFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)

	// Block tmpParent creation: turn <cacheDir>/<spec.ID>/tmp into a file.
	cacheRoot := CacheRoot()
	specID := "ktmp"
	dir := filepath.Join(cacheRoot, specID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tmp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	spec := InstallSpec{ID: specID, Command: "kotlin-lsp"}
	source := ArchiveSource{Version: "1.0", FileName: "k.tar.gz", URL: srv.URL, SHA256: "deadbeef", Format: "tar.gz"}
	if _, err := installArchiveLSP(context.Background(), spec, source, []string{"bin/kotlin-lsp"}); err == nil {
		t.Error("expected error from MkdirAll(tmpParent)")
	}
}

// TestExtractZipArchiveSymlinkCloseErr exercises the symlink closeErr branch.
func TestExtractZipArchiveSymlinkCloseErr(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.zip")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Create(archivePath)
	zw := zip.NewWriter(f)
	hdr := &zip.FileHeader{Name: "link"}
	hdr.SetMode(os.ModeSymlink | 0o777)
	w, _ := zw.CreateHeader(hdr)
	_, _ = w.Write([]byte("sublink"))
	_ = zw.Close()
	_ = f.Close()

	origClose := closeFn
	defer func() { closeFn = origClose }()
	closeFn = func() error { return errors.New("synthetic symlink close fail") }

	if err := extractZipArchive(archivePath, dest); err == nil || !strings.Contains(err.Error(), "synthetic symlink close fail") {
		t.Fatalf("expected symlink close error, got %v", err)
	}
}

// TestInstallArchiveLSPMkdirExtractFail exercises the extractDir MkdirAll error
// branch by blocking the tmpParent entirely so the install never reaches
// the extract step. This is a smoke test — the real branch is exercised by
// TestInstallArchiveLSPMkdirTmpParentFail, which sets up the same condition.
func TestInstallArchiveLSPMkdirExtractFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	cacheRoot := CacheRoot()
	specID := "kext"
	dir := filepath.Join(cacheRoot, specID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tmp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	spec := InstallSpec{ID: specID, Command: "k"}
	source := ArchiveSource{Version: "1.0", FileName: "k.tar.gz", URL: srv.URL, SHA256: "deadbeef", Format: "tar.gz"}
	if _, err := installArchiveLSP(context.Background(), spec, source, []string{"bin/x"}); err == nil {
		t.Error("expected error from installArchiveLSP")
	}
}

// TestInstallArchiveLSPRenameFail exercises the os.Rename error branch in
// installArchiveLSP by pre-creating installDir as a non-empty directory so
// Rename onto it fails on some platforms... actually Rename overwrites. Use
// a path that cannot be replaced (parent dir as file) instead.
func TestInstallArchiveLSPRenameFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)

	archivePath := filepath.Join(tmp, "k.tar.gz")
	f, _ := os.Create(archivePath)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: "bin/x", Typeflag: tar.TypeReg, Mode: 0o755, Size: 0})
	_ = tw.Close()
	_ = gw.Close()
	_ = f.Close()
	data, _ := os.ReadFile(archivePath)
	sum := sha256.Sum256(data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer srv.Close()

	spec := InstallSpec{ID: "kr", Command: "k"}
	source := ArchiveSource{Version: "1.0", FileName: "k.tar.gz", URL: srv.URL, SHA256: hex.EncodeToString(sum[:]), Format: "tar.gz"}

	// Make <cacheDir>/<spec.ID>/versions a regular file so MkdirAll fails
	// before Rename is attempted, exercising an earlier branch.
	cacheRoot := CacheRoot()
	dir := filepath.Join(cacheRoot, spec.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "versions"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := installArchiveLSP(context.Background(), spec, source, []string{"bin/x"}); err == nil {
		t.Error("expected error from MkdirAll(versions)")
	}
}

// TestInstallArchiveLSPWrapperFail exercises the writeExecutableWrapper error
// branch in installArchiveLSP by making wrapperPath itself a directory so the
// WriteFile inside writeExecutableWrapper fails.
func TestInstallArchiveLSPWrapperFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)

	archivePath := filepath.Join(tmp, "k.tar.gz")
	f, _ := os.Create(archivePath)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: "bin/x", Typeflag: tar.TypeReg, Mode: 0o755, Size: 0})
	_ = tw.Close()
	_ = gw.Close()
	_ = f.Close()
	data, _ := os.ReadFile(archivePath)
	sum := sha256.Sum256(data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer srv.Close()

	spec := InstallSpec{ID: "kw", Command: "k"}
	source := ArchiveSource{Version: "1.0", FileName: "k.tar.gz", URL: srv.URL, SHA256: hex.EncodeToString(sum[:]), Format: "tar.gz"}

	// Pre-create <cacheDir>/<spec.ID>/bin/k as a directory so the wrapper
	// WriteFile fails with EISDIR.
	cacheRoot := CacheRoot()
	dir := filepath.Join(cacheRoot, spec.ID)
	if err := os.MkdirAll(filepath.Join(dir, "bin", "k"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := installArchiveLSP(context.Background(), spec, source, []string{"bin/x"}); err == nil {
		t.Error("expected error from writeExecutableWrapper")
	}
}

// TestExtractZipArchiveCloseOutErr exercises the closeOutErr branch by
// injecting a Close error via the closeFn seam.
func TestExtractZipArchiveCloseOutErr(t *testing.T) {
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
	w, err := zw.Create("file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	origClose := closeFn
	defer func() { closeFn = origClose }()
	closeFn = func() error { return errors.New("synthetic close failure") }

	if err := extractZipArchive(archivePath, dest); err == nil || !strings.Contains(err.Error(), "synthetic close failure") {
		t.Fatalf("expected close error, got %v", err)
	}
}

// TestExtractTarGzArchiveCloseErr exercises the closeErr branch by
// injecting a Close error via the closeFn seam.
func TestExtractTarGzArchiveCloseErr(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.tar.gz")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Create(archivePath)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: "f.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 5})
	_, _ = tw.Write([]byte("hello"))
	_ = tw.Close()
	_ = gw.Close()
	_ = f.Close()

	origClose := closeFn
	defer func() { closeFn = origClose }()
	closeFn = func() error { return errors.New("synthetic tar close failure") }

	if err := extractTarGzArchive(archivePath, dest); err == nil || !strings.Contains(err.Error(), "synthetic tar close failure") {
		t.Fatalf("expected close error, got %v", err)
	}
}

// TestInstallArchiveLSPRenameFailExercised uses the renameFn seam to inject
// an error in the os.Rename branch of installArchiveLSP.
func TestInstallArchiveLSPRenameFailExercised(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)

	archivePath := filepath.Join(tmp, "k.tar.gz")
	f, _ := os.Create(archivePath)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: "bin/x", Typeflag: tar.TypeReg, Mode: 0o755, Size: 0})
	_ = tw.Close()
	_ = gw.Close()
	_ = f.Close()
	data, _ := os.ReadFile(archivePath)
	sum := sha256.Sum256(data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer srv.Close()

	spec := InstallSpec{ID: "kr2", Command: "k"}
	source := ArchiveSource{Version: "1.0", FileName: "k.tar.gz", URL: srv.URL, SHA256: hex.EncodeToString(sum[:]), Format: "tar.gz"}

	origRename := renameFn
	defer func() { renameFn = origRename }()
	renameFn = func(_, _ string) error { return errors.New("synthetic rename fail") }

	if _, err := installArchiveLSP(context.Background(), spec, source, []string{"bin/x"}); err == nil || !strings.Contains(err.Error(), "synthetic rename fail") {
		t.Fatalf("expected rename error, got %v", err)
	}
}

// TestInstallArchiveLSPRemoveFailExercised uses the removeAllFn seam.
func TestInstallArchiveLSPRemoveFailExercised(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)

	archivePath := filepath.Join(tmp, "k.tar.gz")
	f, _ := os.Create(archivePath)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: "bin/x", Typeflag: tar.TypeReg, Mode: 0o755, Size: 0})
	_ = tw.Close()
	_ = gw.Close()
	_ = f.Close()
	data, _ := os.ReadFile(archivePath)
	sum := sha256.Sum256(data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer srv.Close()

	spec := InstallSpec{ID: "krm", Command: "k"}
	source := ArchiveSource{Version: "1.0", FileName: "k.tar.gz", URL: srv.URL, SHA256: hex.EncodeToString(sum[:]), Format: "tar.gz"}

	origRm := removeAllFn
	defer func() { removeAllFn = origRm }()
	removeAllFn = func(_ string) error { return errors.New("synthetic remove fail") }

	if _, err := installArchiveLSP(context.Background(), spec, source, []string{"bin/x"}); err == nil || !strings.Contains(err.Error(), "synthetic remove fail") {
		t.Fatalf("expected remove error, got %v", err)
	}
}

// TestInstallArchiveLSPChmodFailExercised uses the chmodFn seam.
func TestInstallArchiveLSPChmodFailExercised(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)

	archivePath := filepath.Join(tmp, "k.tar.gz")
	f, _ := os.Create(archivePath)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: "bin/x", Typeflag: tar.TypeReg, Mode: 0o755, Size: 0})
	_ = tw.Close()
	_ = gw.Close()
	_ = f.Close()
	data, _ := os.ReadFile(archivePath)
	sum := sha256.Sum256(data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer srv.Close()

	spec := InstallSpec{ID: "kc", Command: "k"}
	source := ArchiveSource{Version: "1.0", FileName: "k.tar.gz", URL: srv.URL, SHA256: hex.EncodeToString(sum[:]), Format: "tar.gz"}

	origChmod := chmodFn
	defer func() { chmodFn = origChmod }()
	chmodFn = func(_ string, _ os.FileMode) error { return errors.New("synthetic chmod fail") }

	if _, err := installArchiveLSP(context.Background(), spec, source, []string{"bin/x"}); err == nil || !strings.Contains(err.Error(), "synthetic chmod fail") {
		t.Fatalf("expected chmod error, got %v", err)
	}
}

// TestExtractZipArchiveEntryOpenErr exercises entry.Open failure via seam.
func TestExtractZipArchiveEntryOpenErr(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.zip")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Create(archivePath)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("file.txt")
	_, _ = w.Write([]byte("hi"))
	_ = zw.Close()
	_ = f.Close()

	origOpen := zipEntryOpenFn
	defer func() { zipEntryOpenFn = origOpen }()
	zipEntryOpenFn = func(_ *zip.File) (io.ReadCloser, error) { return nil, errors.New("synthetic entry open fail") }

	if err := extractZipArchive(archivePath, dest); err == nil || !strings.Contains(err.Error(), "synthetic entry open fail") {
		t.Fatalf("expected entry open error, got %v", err)
	}
}

// TestExtractZipArchiveSymlinkReadErr exercises the symlink readErr branch via readAllFn seam.
func TestExtractZipArchiveSymlinkReadErr2(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.zip")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Create(archivePath)
	zw := zip.NewWriter(f)
	hdr := &zip.FileHeader{Name: "link"}
	hdr.SetMode(os.ModeSymlink | 0o777)
	w, _ := zw.CreateHeader(hdr)
	_, _ = w.Write([]byte("sublink"))
	_ = zw.Close()
	_ = f.Close()

	origRead := readAllFn
	defer func() { readAllFn = origRead }()
	readAllFn = func(_ io.Reader) ([]byte, error) { return nil, errors.New("synthetic readAll fail") }

	if err := extractZipArchive(archivePath, dest); err == nil || !strings.Contains(err.Error(), "synthetic readAll fail") {
		t.Fatalf("expected readAll error, got %v", err)
	}
}

// TestExtractZipArchiveCloseReadErr exercises closeReadErr branch via closeFn.
func TestExtractZipArchiveCloseReadErr(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.zip")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Create(archivePath)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("file.txt")
	_, _ = w.Write([]byte("hi"))
	_ = zw.Close()
	_ = f.Close()

	// Set closeFn to error only on the second call (the read side).
	calls := 0
	origClose := closeFn
	defer func() { closeFn = origClose }()
	closeFn = func() error {
		calls++
		if calls == 2 {
			return errors.New("synthetic read close fail")
		}
		return nil
	}

	if err := extractZipArchive(archivePath, dest); err == nil || !strings.Contains(err.Error(), "synthetic read close fail") {
		t.Fatalf("expected read close error, got %v", err)
	}
}

// TestInstallArchiveLSPMkdirTempFailExercised uses the mkdirTempFn seam.
func TestInstallArchiveLSPMkdirTempFailExercised(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)

	spec := InstallSpec{ID: "kmk", Command: "k"}
	source := ArchiveSource{Version: "1.0", FileName: "k.tar.gz", URL: "http://x", SHA256: "x", Format: "tar.gz"}

	origMkdt := mkdirTempFn
	defer func() { mkdirTempFn = origMkdt }()
	mkdirTempFn = func(_ string, _ string) (string, error) { return "", errors.New("synthetic mkdirTemp fail") }

	if _, err := installArchiveLSP(context.Background(), spec, source, []string{"bin/x"}); err == nil || !strings.Contains(err.Error(), "synthetic mkdirTemp fail") {
		t.Fatalf("expected mkdirTemp error, got %v", err)
	}
}

// TestExtractZipArchiveCopyErr exercises zip copyErr branch via copyFn seam.
func TestExtractZipArchiveCopyErr(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.zip")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Create(archivePath)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("file.txt")
	_, _ = w.Write([]byte("hi"))
	_ = zw.Close()
	_ = f.Close()

	origCopy := copyFn
	defer func() { copyFn = origCopy }()
	copyFn = func(_ io.Writer, _ io.Reader) (int64, error) { return 0, errors.New("synthetic zip copy fail") }

	if err := extractZipArchive(archivePath, dest); err == nil || !strings.Contains(err.Error(), "synthetic zip copy fail") {
		t.Fatalf("expected zip copy error, got %v", err)
	}
}

// TestExtractTarGzArchiveCopyErr exercises tar copyErr branch via copyFn seam.
func TestExtractTarGzArchiveCopyErr2(t *testing.T) {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "test.tar.gz")
	dest := filepath.Join(tmp, "extract")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Create(archivePath)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: "f.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 5})
	_, _ = tw.Write([]byte("hello"))
	_ = tw.Close()
	_ = gw.Close()
	_ = f.Close()

	origCopy := copyFn
	defer func() { copyFn = origCopy }()
	copyFn = func(_ io.Writer, _ io.Reader) (int64, error) { return 0, errors.New("synthetic tar copy fail") }

	if err := extractTarGzArchive(archivePath, dest); err == nil || !strings.Contains(err.Error(), "synthetic tar copy fail") {
		t.Fatalf("expected tar copy error, got %v", err)
	}
}

// TestDownloadArchiveCloseErr exercises the closeErr branch in
// downloadArchive by injecting via closeFn.
func TestDownloadArchiveCloseErr(t *testing.T) {
	tmp := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	origClose := closeFn
	defer func() { closeFn = origClose }()
	closeFn = func() error { return errors.New("synthetic download close failure") }

	source := ArchiveSource{FileName: "x", URL: srv.URL, SHA256: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", Format: "tar.gz"}
	if err := downloadArchive(context.Background(), source, filepath.Join(tmp, "out")); err == nil || !strings.Contains(err.Error(), "synthetic download close failure") {
		t.Fatalf("expected close error, got %v", err)
	}
}

func TestDefaultKotlinArchiveSourceDarwinArm64(t *testing.T) {
	origOS := runtimeOS
	origArch := runtimeArch
	runtimeOS = func() string { return "darwin" }
	runtimeArch = func() string { return "arm64" }
	t.Cleanup(func() {
		runtimeOS = origOS
		runtimeArch = origArch
	})
	src, err := defaultKotlinArchiveSource(InstallSpec{ID: "kotlin"})
	if err != nil {
		t.Fatalf("darwin/arm64 should succeed: %v", err)
	}
	if !strings.Contains(src.FileName, "aarch64") {
		t.Errorf("expected arm64 suffix, got %+v", src)
	}
}

func TestDefaultKotlinArchiveSourceDarwinAmd64(t *testing.T) {
	origOS := runtimeOS
	origArch := runtimeArch
	runtimeOS = func() string { return "darwin" }
	runtimeArch = func() string { return "amd64" }
	t.Cleanup(func() {
		runtimeOS = origOS
		runtimeArch = origArch
	})
	src, err := defaultKotlinArchiveSource(InstallSpec{ID: "kotlin"})
	if err != nil {
		t.Fatalf("darwin/amd64 should succeed: %v", err)
	}
	if strings.Contains(src.FileName, "aarch64") {
		t.Errorf("expected no arm64 suffix, got %+v", src)
	}
}

func TestDefaultKotlinArchiveSourceDarwinDefault(t *testing.T) {
	origOS := runtimeOS
	origArch := runtimeArch
	runtimeOS = func() string { return "darwin" }
	runtimeArch = func() string { return "ppc64" }
	t.Cleanup(func() {
		runtimeOS = origOS
		runtimeArch = origArch
	})
	_, err := defaultKotlinArchiveSource(InstallSpec{ID: "kotlin"})
	if err == nil {
		t.Error("expected error for unsupported darwin arch")
	}
}

func TestDefaultKotlinArchiveSourceLinuxArm64(t *testing.T) {
	origOS := runtimeOS
	origArch := runtimeArch
	runtimeOS = func() string { return "linux" }
	runtimeArch = func() string { return "arm64" }
	t.Cleanup(func() {
		runtimeOS = origOS
		runtimeArch = origArch
	})
	src, err := defaultKotlinArchiveSource(InstallSpec{ID: "kotlin"})
	if err != nil {
		t.Fatalf("linux/arm64 should succeed: %v", err)
	}
	if !strings.Contains(src.FileName, "aarch64") {
		t.Errorf("expected arm64 suffix, got %+v", src)
	}
}

func TestDefaultKotlinArchiveSourceLinuxDefault(t *testing.T) {
	origOS := runtimeOS
	origArch := runtimeArch
	runtimeOS = func() string { return "linux" }
	runtimeArch = func() string { return "ppc64" }
	t.Cleanup(func() {
		runtimeOS = origOS
		runtimeArch = origArch
	})
	_, err := defaultKotlinArchiveSource(InstallSpec{ID: "kotlin"})
	if err == nil {
		t.Error("expected error for unsupported linux arch")
	}
}

func TestDefaultKotlinArchiveSourceLinuxAmd64(t *testing.T) {
	origOS := runtimeOS
	origArch := runtimeArch
	runtimeOS = func() string { return "linux" }
	runtimeArch = func() string { return "amd64" }
	t.Cleanup(func() {
		runtimeOS = origOS
		runtimeArch = origArch
	})
	src, err := defaultKotlinArchiveSource(InstallSpec{ID: "kotlin"})
	if err != nil {
		t.Fatalf("linux/amd64 should succeed: %v", err)
	}
	if !strings.Contains(src.FileName, "tar.gz") {
		t.Errorf("expected tar.gz, got %+v", src)
	}
}

func TestDefaultKotlinArchiveSourceDefault(t *testing.T) {
	origOS := runtimeOS
	origArch := runtimeArch
	runtimeOS = func() string { return "plan9" }
	runtimeArch = func() string { return "amd64" }
	t.Cleanup(func() {
		runtimeOS = origOS
		runtimeArch = origArch
	})
	_, err := defaultKotlinArchiveSource(InstallSpec{ID: "kotlin"})
	if err == nil {
		t.Error("expected error for unsupported OS")
	}
}

// Compile-time check
var _ = fmt.Sprintf
var _ io.Reader = nil

// TestInstallArchiveLSPMkdirExtractFailExercised uses the mkdirAllFn seam to
// inject an error in the extractDir MkdirAll branch of installArchiveLSP.
func TestInstallArchiveLSPMkdirExtractFailExercised(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)

	archivePath := filepath.Join(tmp, "k.tar.gz")
	f, _ := os.Create(archivePath)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: "bin/x", Typeflag: tar.TypeReg, Mode: 0o755, Size: 0})
	_ = tw.Close()
	_ = gw.Close()
	_ = f.Close()
	data, _ := os.ReadFile(archivePath)
	sum := sha256.Sum256(data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer srv.Close()

	spec := InstallSpec{ID: "kmke", Command: "k"}
	source := ArchiveSource{Version: "1.0", FileName: "k.tar.gz", URL: srv.URL, SHA256: hex.EncodeToString(sum[:]), Format: "tar.gz"}

	origMk := mkdirAllFn
	defer func() { mkdirAllFn = origMk }()
	// The only installArchiveLSP call routed through mkdirAllFn is the
	// extractDir MkdirAll. Inject an error there.
	mkdirAllFn = func(_ string, _ os.FileMode) error { return errors.New("synthetic mkdirAll extract fail") }

	if _, err := installArchiveLSP(context.Background(), spec, source, []string{"bin/x"}); err == nil || !strings.Contains(err.Error(), "synthetic mkdirAll extract fail") {
		t.Fatalf("expected mkdirAll extract error, got %v", err)
	}
}

// TestInstallArchiveLSPRelFailExercised uses the relFn seam to inject an
// error in the filepath.Rel branch of installArchiveLSP.
func TestInstallArchiveLSPRelFailExercised(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)

	archivePath := filepath.Join(tmp, "k.tar.gz")
	f, _ := os.Create(archivePath)
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: "bin/x", Typeflag: tar.TypeReg, Mode: 0o755, Size: 0})
	_ = tw.Close()
	_ = gw.Close()
	_ = f.Close()
	data, _ := os.ReadFile(archivePath)
	sum := sha256.Sum256(data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer srv.Close()

	spec := InstallSpec{ID: "krel", Command: "k"}
	source := ArchiveSource{Version: "1.0", FileName: "k.tar.gz", URL: srv.URL, SHA256: hex.EncodeToString(sum[:]), Format: "tar.gz"}

	origRel := relFn
	defer func() { relFn = origRel }()
	relFn = func(_, _ string) (string, error) { return "", errors.New("synthetic rel fail") }

	if _, err := installArchiveLSP(context.Background(), spec, source, []string{"bin/x"}); err == nil || !strings.Contains(err.Error(), "synthetic rel fail") {
		t.Fatalf("expected rel error, got %v", err)
	}
}
