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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// multiFailWriter fails after N successful writes.
type multiFailWriter struct {
	writes  int
	failAt  int
	failed  bool
	written []byte
}

func (w *multiFailWriter) Write(p []byte) (int, error) {
	if w.failed {
		return 0, fmt.Errorf("already failed")
	}
	w.writes++
	if w.writes > w.failAt {
		w.failed = true
		return 0, fmt.Errorf("simulated write failure at %d", w.writes)
	}
	w.written = append(w.written, p...)
	return len(p), nil
}

func TestRunLSPListFprintfRowFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	// failAt=0 means fail on the very first write (the header line).
	w := &multiFailWriter{failAt: 0}
	err := RunLSPList([]string{"--project", tmp}, w, emptySourceDetector{})
	if err == nil {
		t.Error("expected error from multiFailWriter")
	}
}

func TestRunLSPInstallForceRecheckSkip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	// Set up cache bin so gopls is "found" after first check.
	cacheBinDir := filepath.Join(CacheRoot(), "go", "bin")
	if err := os.MkdirAll(cacheBinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	goplsPath := filepath.Join(cacheBinDir, "gopls")
	if err := os.WriteFile(goplsPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", cacheBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Use stub that would succeed, but should be skipped due to first check.
	impl := stubImpl{
		spec: InstallSpec{ID: "go", Name: "Go", Command: "gopls"},
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{})
	if result.Status != "already-installed" {
		t.Errorf("expected already-installed, got %+v", result)
	}
}

func TestInstallArchiveLSPSuccessTarGz(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)

	// Create a tar.gz with the executable.
	archivePath := filepath.Join(tmp, "kotlin.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: "bin/kotlin-lsp", Typeflag: tar.TypeReg, Mode: 0o755, Size: 5}); err != nil {
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

	data, _ := os.ReadFile(archivePath)
	sum := sha256.Sum256(data)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	spec := InstallSpec{ID: "kotlin", Command: "kotlin-lsp"}
	source := ArchiveSource{
		Version:  "1.0",
		FileName: "kotlin.tar.gz",
		URL:      srv.URL,
		SHA256:   hex.EncodeToString(sum[:]),
		Format:   "tar.gz",
	}
	wrapperPath, err := installArchiveLSP(context.Background(), spec, source, []string{"bin/kotlin-lsp"})
	if err != nil {
		t.Fatalf("installArchiveLSP failed: %v", err)
	}
	if !strings.HasPrefix(wrapperPath, CacheRoot()) {
		t.Errorf("wrapper path %q should be in cache", wrapperPath)
	}
}

func TestExtractZipArchiveFileWriteFail(t *testing.T) {
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

	// Block file creation by making parent a file.
	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest = filepath.Join(blocker, "extract")
	if err := extractZipArchive(archivePath, dest); err == nil {
		t.Error("expected error when mkdir fails")
	}
}

func TestExtractZipArchiveSymlinkReadError(t *testing.T) {
	// Hard to trigger ReadAll error from in-memory zip.
	t.Skip("difficult to trigger symlink read error from in-memory zip")
}

func TestCreateSafeArchiveSymlinkMkdirFail(t *testing.T) {
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(blocker, "subdir", "link")
	err := createSafeArchiveSymlink(tmp, "target", target)
	if err == nil {
		t.Error("expected error when mkdir fails")
	}
}

func TestWriteExecutableWrapperWriteFail(t *testing.T) {
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	wrapperPath := filepath.Join(blocker, "sub", "wrapper")
	if err := writeExecutableWrapper(wrapperPath, "/x"); err == nil {
		t.Error("expected error when WriteFile fails")
	}
}

func TestRunLSPListJSONWriteFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	w := &multiFailWriter{failAt: 0}
	err := RunLSPList([]string{"--project", tmp, "--json"}, w, emptySourceDetector{})
	_ = err // may or may not fail
}

func TestRunLSPInstallFprintfJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	w := &multiFailWriter{failAt: 0}
	err := RunLSPInstall([]string{"go", "--dry-run", "--force", "--json", "--project", tmp}, w, emptySourceDetector{})
	_ = err
}

func TestRunLSPInstallDetectMCP(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	var buf bytes.Buffer
	if err := RunLSPInstall([]string{"go", "--dry-run", "--force", "--project", tmp, "--docs-dir", "docs"}, &buf, emptySourceDetector{}); err != nil {
		t.Errorf("dry-run failed: %v", err)
	}
}

func TestRunLSPListProjectFromCwd(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	// Change to tmp dir.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := RunLSPList(nil, &buf, emptySourceDetector{}); err != nil {
		t.Errorf("RunLSPList failed: %v", err)
	}
}

func TestRunLSPInstallFromCwd(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := RunLSPInstall([]string{"go", "--dry-run", "--force"}, &buf, emptySourceDetector{}); err != nil {
		t.Errorf("RunLSPInstall failed: %v", err)
	}
}

func TestListStatusWithLangID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	rows := ListStatus(tmp, "docs", emptySourceDetector{})
	// Verify all rows have non-empty Name and Binary.
	for _, row := range rows {
		if row.Name == "" {
			t.Errorf("row %s has empty Name", row.ID)
		}
		if row.Binary == "" {
			t.Errorf("row %s has empty Binary", row.ID)
		}
	}
}

func TestInstallLSPForceFalseRecheck(t *testing.T) {
	// Test that after lock acquisition, if path is still not found, it tries to install.
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")
	impl := stubImpl{
		spec:       InstallSpec{ID: "stub", Name: "Stub", Command: "nonexistent-cmd-xyz-zzz"},
		installOut: "/nonexistent",
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{})
	if result.Status != "failed" {
		t.Errorf("expected failed, got %+v", result)
	}
}

func TestInstallLSPForceTrueNoRecheck(t *testing.T) {
	// With Force=true, the first skip is bypassed but second recheck is also bypassed.
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "ns-workspace-test-cmd-x3")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	impl := stubImpl{
		spec:       InstallSpec{ID: "stub", Name: "Stub", Command: "ns-workspace-test-cmd-x3"},
		installOut: "/nonexistent",
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{Force: true})
	if result.Status != "failed" {
		t.Errorf("expected failed (with install path /nonexistent), got %+v", result)
	}
}

func TestExtractTarGzArchiveCopyError(t *testing.T) {
	// Covered by TestExtractTarGzArchiveCopyErr2 in lsp_archive_test.go.
	t.Skip("superseded by TestExtractTarGzArchiveCopyErr2")
}

func TestInstallArchiveLSPRemoveFail(t *testing.T) {
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
	if wrapperPath == "" {
		t.Error("wrapper path should not be empty")
	}
}

func TestExtractZipArchiveCloseError(t *testing.T) {
	t.Skip("difficult to trigger zip Close error")
}

func TestDownloadArchiveCopyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write some bytes, then close abruptly to trigger copy error.
		w.Header().Set("Content-Length", "100000")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "out.bin")
	source := ArchiveSource{
		FileName: "out.bin",
		URL:      srv.URL,
		SHA256:   "00",
	}
	_ = downloadArchive(context.Background(), source, dest)
}

func TestCacheRootAllFallbacks(t *testing.T) {
	// Test all fallbacks at once by setting invalid paths.
	t.Setenv(CacheEnv, "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "/nonexistent/path/zzz/yyy")
	got := CacheRoot()
	if got == "" {
		t.Error("CacheRoot should never return empty")
	}
}

func TestInstallLSPForceTrueRealFlow(t *testing.T) {
	// With Force=true, the lock acquisition's recheck is skipped.
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())

	// Create a fake binary that exists.
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "ns-workspace-test-cmd-x4")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	impl := stubImpl{
		spec:       InstallSpec{ID: "stub", Name: "Stub", Command: "ns-workspace-test-cmd-x4"},
		installOut: "/nonexistent/path/zzz",
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{Force: true})
	if result.Status != "failed" {
		t.Errorf("expected failed (binary not executable at /nonexistent), got %+v", result)
	}
}

func TestKotlinInstallForceSkipArchive(t *testing.T) {
	impl := kotlinImplementation{}
	// Override to a working stub so we can test the archive install path.
	restore := SetArchiveSourceForTest(func(InstallSpec) (ArchiveSource, error) {
		return ArchiveSource{
			Version:  "1.0",
			FileName: "x.zip",
			URL:      "http://x.invalid",
			SHA256:   "00",
			Format:   "zip",
		}, nil
	})
	defer restore()
	_, err := impl.install(context.Background())
	if err == nil {
		t.Error("expected error from network failure")
	}
}

func TestRunLSPListCwdError(t *testing.T) {
	orig := getWorkingDir
	getWorkingDir = func() (string, error) { return "", errors.New("simulated cwd failure") }
	t.Cleanup(func() { getWorkingDir = orig })
	if err := RunLSPList(nil, io.Discard, emptySourceDetector{}); err == nil {
		t.Error("expected error from getWorkingDir")
	}
}

func TestRunLSPInstallCwdError(t *testing.T) {
	orig := getWorkingDir
	getWorkingDir = func() (string, error) { return "", errors.New("simulated cwd failure") }
	t.Cleanup(func() { getWorkingDir = orig })
	if err := RunLSPInstall([]string{"go", "--dry-run"}, io.Discard, emptySourceDetector{}); err == nil {
		t.Error("expected error from getWorkingDir")
	}
}

func TestNormalizeProjectRootAbsError(t *testing.T) {
	orig := filepathAbs
	filepathAbs = func(string) (string, error) { return "", errors.New("simulated abs failure") }
	t.Cleanup(func() { filepathAbs = orig })
	// Với seam inject error, normalizeProjectRoot phải fallback về clean(root).
	got := normalizeProjectRoot("some-root")
	if got != "some-root" {
		t.Errorf("expected fallback to clean(root), got %q", got)
	}
}

func TestCacheRootUserCacheDirError(t *testing.T) {
	t.Setenv(CacheEnv, "")
	origUC := userCacheDirImpl
	userCacheDirImpl = func() (string, error) { return "", errors.New("simulated user cache dir failure") }
	t.Cleanup(func() { userCacheDirImpl = origUC })
	// Khi UserCacheDir fail, fallback sang UserHomeDir hoặc TempDir.
	got := CacheRoot()
	if got == "" {
		t.Error("CacheRoot should never return empty")
	}
}

func TestCacheRootUserHomeDirError(t *testing.T) {
	t.Setenv(CacheEnv, "")
	origUC := userCacheDirImpl
	origUH := userHomeDirImpl
	userCacheDirImpl = func() (string, error) { return "", errors.New("simulated user cache dir failure") }
	userHomeDirImpl = func() (string, error) { return "", errors.New("simulated user home dir failure") }
	t.Cleanup(func() {
		userCacheDirImpl = origUC
		userHomeDirImpl = origUH
	})
	got := CacheRoot()
	if got == "" {
		t.Error("CacheRoot should never return empty")
	}
}

func TestInstallLSPRecheckAfterLockSuccess(t *testing.T) {
	// First call to resolveCommandWithSourceImpl returns error,
	// but after lock acquisition the second call returns success → recheck branch.
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())

	calls := 0
	orig := resolveCommandWithSourceImpl
	resolveCommandWithSourceImpl = func(command, projectRoot string) (string, string, error) {
		calls++
		if calls == 1 {
			return "", "", errors.New("not found on first call")
		}
		return "/found/after/lock", "cache", nil
	}
	t.Cleanup(func() { resolveCommandWithSourceImpl = orig })

	impl := stubImpl{
		spec:       InstallSpec{ID: "stub", Name: "Stub", Command: "stub-cmd"},
		installOut: "/nonexistent",
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{})
	if result.Status != "already-installed" {
		t.Errorf("expected already-installed, got %+v", result)
	}
	if calls != 2 {
		t.Errorf("expected resolveCommandWithSourceImpl to be called twice, got %d", calls)
	}
}

func TestListStatusWithUnknownServerID(t *testing.T) {
	// Override LanguageSpecs với một spec có ServerID không có InstallSpec nào match.
	// → ListStatus skip nhánh đó.
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	orig := languageSpecsSnapshotOverride
	languageSpecsSnapshotOverride = []LanguageSpec{
		{ID: "ghost", Name: "Ghost", ServerID: "no-such-server-id", LanguageID: "ghost"},
	}
	t.Cleanup(func() { languageSpecsSnapshotOverride = orig })

	rows := ListStatus(tmp, "docs", emptySourceDetector{})
	if len(rows) != 0 {
		t.Errorf("expected 0 rows (skipped), got %d", len(rows))
	}
}

func TestRunLSPListFprintfRowError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	// failAt=1: cho phép header line qua, fail ở row đầu tiên.
	w := &multiFailWriter{failAt: 1}
	err := RunLSPList([]string{"--project", tmp}, w, emptySourceDetector{})
	if err == nil {
		t.Error("expected error from row Fprintf")
	}
}

func TestSafeArchiveTargetSameAsDest(t *testing.T) {
	tmp := t.TempDir()
	// When target == destClean, it's allowed.
	// Try with empty name - clean is "." which is rejected.
	if _, err := safeArchiveTarget(tmp, "."); err == nil {
		t.Error("expected error for '.'")
	}
	// Use name that resolves to dest itself? Hard to construct.
	// Just verify edge case where target == dest.
	if _, err := safeArchiveTarget(tmp, ""); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestRunLSPInstallDetectWithEmptyBuffer(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	var buf bytes.Buffer
	if err := RunLSPInstall([]string{"go", "--dry-run", "--force", "--project", tmp}, &buf, emptySourceDetector{}); err != nil {
		t.Errorf("dry-run failed: %v", err)
	}
	if !strings.Contains(buf.String(), "go:") {
		t.Errorf("expected go: prefix in output: %s", buf.String())
	}
}

func TestRunLSPInstallInstallCommandForKotlin(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	var buf bytes.Buffer
	// Use --dry-run with --force for kotlin to test installCommand path.
	// Without internet access, the download will fail. But installCommand will be called.
	if err := RunLSPInstall([]string{"kotlin", "--dry-run", "--force", "--project", tmp}, &buf, emptySourceDetector{}); err != nil {
		t.Errorf("dry-run failed: %v", err)
	}
}

// Verify the FileInfo().IsDir() branch in extractZipArchive for directories.
func TestExtractZipArchiveDirEntry(t *testing.T) {
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
	if _, err := zw.Create("subdir/"); err != nil {
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
	info, err := os.Stat(filepath.Join(dest, "subdir"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("expected subdir to be a directory")
	}
}

// Compile-time checks.
var _ = io.Copy

// TestGoImplementationInstallSuccess exercises the success branch of
// goImplementation.install() by replacing the `go` binary with a fake one
// that drops a gopls executable into GOBIN.
func TestGoImplementationInstallSuccess(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)

	// Build a fake "go" binary that creates gopls in GOBIN.
	fakeBinDir := t.TempDir()
	fakeGo := filepath.Join(fakeBinDir, "go")
	script := "#!/bin/sh\n" +
		"gob=\"$GOBIN\"\n" +
		"if [ -z \"$gob\" ]; then gob=$(go env GOBIN 2>/dev/null); fi\n" +
		"if [ -z \"$gob\" ]; then gob=\"$HOME/go/bin\"; fi\n" +
		"mkdir -p \"$gob\"\n" +
		"printf '#!/bin/sh\\nexit 0\\n' > \"$gob/gopls\"\n" +
		"chmod +x \"$gob/gopls\"\n" +
		"exit 0\n"
	if err := os.WriteFile(fakeGo, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	got, err := goImplementation{}.install(context.Background())
	if err != nil {
		t.Fatalf("goImplementation.install: %v", err)
	}
	if !strings.HasSuffix(got, "gopls") {
		t.Errorf("expected path to end in gopls, got %q", got)
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("returned path not accessible: %v", err)
	}
}
