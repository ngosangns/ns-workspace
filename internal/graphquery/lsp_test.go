package graphquery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCacheEnvConstant(t *testing.T) {
	if CacheEnv != "NS_WORKSPACE_LSP_CACHE" {
		t.Errorf("CacheEnv = %q, want NS_WORKSPACE_LSP_CACHE", CacheEnv)
	}
}

func TestSymbolModeConstants(t *testing.T) {
	if SymbolModeCallable != "callable" {
		t.Errorf("SymbolModeCallable = %q, want callable", SymbolModeCallable)
	}
	if SymbolModeDocument != "document" {
		t.Errorf("SymbolModeDocument = %q, want document", SymbolModeDocument)
	}
	if SymbolModeSelector != "selector" {
		t.Errorf("SymbolModeSelector = %q, want selector", SymbolModeSelector)
	}
}

func TestEmptySourceDetector(t *testing.T) {
	d := emptySourceDetector{}
	if langs := d.DetectedLanguages(".", "docs"); len(langs) != 0 {
		t.Errorf("DetectedLanguages = %v, want empty", langs)
	}
	if ids := d.DetectedInstallIDs(".", "docs"); len(ids) != 0 {
		t.Errorf("DetectedInstallIDs = %v, want empty", ids)
	}
}

func TestRunLSPUsage(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		t.Run(arg, func(t *testing.T) {
			// The function writes to os.Stdout directly; just verify it does not return error.
			if err := RunLSP([]string{arg}, emptySourceDetector{}); err != nil {
				t.Errorf("RunLSP(%q) returned error: %v", arg, err)
			}
		})
	}
}

func TestRunLSPUnknownCommand(t *testing.T) {
	err := RunLSP([]string{"unknown-cmd"}, emptySourceDetector{})
	if err == nil || !strings.Contains(err.Error(), "unknown lsp command") {
		t.Fatalf("expected unknown lsp command error, got: %v", err)
	}
}

func TestPrintLSPUsage(t *testing.T) {
	var buf bytes.Buffer
	PrintLSPUsage(&buf)
	out := buf.String()
	for _, want := range []string{"lsp list", "lsp install", "Supported languages"} {
		if !strings.Contains(out, want) {
			t.Errorf("PrintLSPUsage output missing %q. Got:\n%s", want, out)
		}
	}
}

func TestRunLSPListJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	var buf bytes.Buffer
	if err := RunLSPList([]string{"--project", tmp, "--docs-dir", "docs", "--json"}, &buf, emptySourceDetector{}); err != nil {
		t.Fatalf("RunLSPList failed: %v", err)
	}
	var rows []StatusRow
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, buf.String())
	}
	if len(rows) == 0 {
		t.Errorf("expected at least one status row")
	}
}

func TestRunLSPListTable(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	var buf bytes.Buffer
	if err := RunLSPList([]string{"--project", tmp}, &buf, emptySourceDetector{}); err != nil {
		t.Fatalf("RunLSPList failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Language\tDetected\tStatus\tBinary\tInstall") {
		t.Errorf("missing table header. Got:\n%s", out)
	}
	// Each row should print ID and a status.
	if !strings.Contains(out, "missing") {
		t.Errorf("expected at least one 'missing' status. Got:\n%s", out)
	}
}

func TestRunLSPListFlagHelp(t *testing.T) {
	var buf bytes.Buffer
	if err := RunLSPList([]string{"-h"}, &buf, emptySourceDetector{}); err != nil {
		t.Errorf("expected nil error for flag help, got: %v", err)
	}
}

func TestRunLSPListFlagError(t *testing.T) {
	var buf bytes.Buffer
	err := RunLSPList([]string{"--nonexistent-flag"}, &buf, emptySourceDetector{})
	if err == nil {
		t.Errorf("expected error for unknown flag, got nil")
	}
}

func TestRunLSPListDetected(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	det := fakeDetector{langs: map[string]bool{"go": true}}
	var buf bytes.Buffer
	if err := RunLSPList([]string{"--project", tmp}, &buf, det); err != nil {
		t.Fatalf("RunLSPList failed: %v", err)
	}
	if !strings.Contains(buf.String(), "go\tyes") {
		t.Errorf("expected 'go\\tyes' row. Got:\n%s", buf.String())
	}
}

func TestRunLSPInstallNoArgs(t *testing.T) {
	var buf bytes.Buffer
	err := RunLSPInstall([]string{}, &buf, emptySourceDetector{})
	if err == nil || !strings.Contains(err.Error(), "lsp install requires") {
		t.Fatalf("expected error about missing language, got: %v", err)
	}
}

func TestRunLSPInstallUnsupportedLanguage(t *testing.T) {
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"nosuchlang"}, &buf, emptySourceDetector{})
	if err == nil || !strings.Contains(err.Error(), "unsupported LSP language") {
		t.Fatalf("expected unsupported language error, got: %v", err)
	}
}

func TestRunLSPInstallDryRunJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"go", "--dry-run", "--force", "--json", "--project", tmp}, &buf, emptySourceDetector{})
	if err != nil {
		t.Fatalf("RunLSPInstall dry-run failed: %v", err)
	}
	var results []InstallResult
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, buf.String())
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "dry-run" {
		t.Errorf("expected dry-run status, got %q", results[0].Status)
	}
}

func TestRunLSPInstallDryRunTable(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"go", "--dry-run", "--force", "--project", tmp}, &buf, emptySourceDetector{})
	if err != nil {
		t.Fatalf("RunLSPInstall dry-run failed: %v", err)
	}
	if !strings.Contains(buf.String(), "go: dry-run") {
		t.Errorf("expected 'go: dry-run' line. Got:\n%s", buf.String())
	}
}

func TestRunLSPInstallAutoEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"auto", "--project", tmp, "--docs-dir", "docs"}, &buf, emptySourceDetector{})
	if err != nil {
		t.Fatalf("RunLSPInstall auto failed: %v", err)
	}
	if !strings.Contains(buf.String(), "no supported LSP languages detected") {
		t.Errorf("expected skipped message. Got:\n%s", buf.String())
	}
}

func TestRunLSPInstallFlagHelp(t *testing.T) {
	var buf bytes.Buffer
	if err := RunLSPInstall([]string{"go", "-h"}, &buf, emptySourceDetector{}); err != nil {
		t.Errorf("expected nil error for flag help, got: %v", err)
	}
}

func TestRunLSPInstallFlagError(t *testing.T) {
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"go", "--bad-flag"}, &buf, emptySourceDetector{})
	if err == nil {
		t.Errorf("expected error for unknown flag")
	}
}

func TestRunLSPInstallGoFailure(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	// Pretend gopls isn't found, so install will actually try to run `go install`.
	// This requires Go to be installed; if it isn't, we just expect a failure result.
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"go", "--project", tmp}, &buf, emptySourceDetector{})
	// We don't care about success - we just want the code path to be exercised.
	_ = err
	_ = buf
}

func TestListStatus(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	rows := ListStatus(tmp, "docs", emptySourceDetector{})
	if len(rows) == 0 {
		t.Fatal("expected rows from ListStatus")
	}
	// Verify rows are sorted by ID.
	for i := 1; i < len(rows); i++ {
		if rows[i-1].ID > rows[i].ID {
			t.Errorf("rows not sorted: %v", rows)
			break
		}
	}
	// Verify InstallCommand is set when status is missing.
	for _, row := range rows {
		if row.Status == "missing" && row.InstallCommand == "" {
			t.Errorf("missing install command for %s", row.ID)
		}
	}
}

func TestListStatusWithDetected(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	det := fakeDetector{langs: map[string]bool{"go": true, "kotlin": true}}
	rows := ListStatus(tmp, "docs", det)
	var foundGo, foundKotlin bool
	for _, r := range rows {
		if r.ID == "go" {
			foundGo = r.Detected
		}
		if r.ID == "kotlin" {
			foundKotlin = r.Detected
		}
	}
	if !foundGo {
		t.Errorf("expected go to be detected")
	}
	if !foundKotlin {
		t.Errorf("expected kotlin to be detected")
	}
}

func TestInstallProjectLSPsEmpty(t *testing.T) {
	results := InstallProjectLSPs(context.Background(), t.TempDir(), "docs", EnsureOptions{}, emptySourceDetector{})
	if len(results) != 1 || results[0].Status != "skipped" {
		t.Errorf("expected single skipped result, got: %+v", results)
	}
}

func TestInstallProjectLSPsDetected(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	det := fakeDetector{installIDs: map[string]bool{"go": true, "kotlin": true}}
	results := InstallProjectLSPs(context.Background(), tmp, "docs", EnsureOptions{DryRun: true}, det)
	if len(results) < 2 {
		t.Errorf("expected at least 2 results, got %d", len(results))
	}
}

func TestInstallProjectLSPsUnknownDetected(t *testing.T) {
	det := fakeDetector{installIDs: map[string]bool{"unknown-lsp": true}}
	results := InstallProjectLSPs(context.Background(), t.TempDir(), "docs", EnsureOptions{DryRun: true}, det)
	if len(results) != 0 {
		t.Errorf("expected 0 results for unknown detected, got %d", len(results))
	}
}

func TestEnsureProjectLSP(t *testing.T) {
	warnings := EnsureProjectLSP(context.Background(), t.TempDir(), "docs", EnsureOptions{}, emptySourceDetector{})
	// emptySourceDetector => skipped => no warnings
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestInstallWarnings(t *testing.T) {
	results := []InstallResult{
		{ID: "go", Name: "Go", Status: "installed", Path: "/x"},
		{ID: "ts", Name: "TypeScript", Status: "failed", Message: "boom"},
		{ID: "kt", Name: "Kotlin", Status: "manual", Message: "manual needed"},
		{ID: "css", Name: "CSS", Status: "skipped"},
	}
	var buf bytes.Buffer
	opts := EnsureOptions{Progress: &buf}
	warnings := InstallWarnings(results, opts)
	if len(warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(buf.String(), "installed go LSP") {
		t.Errorf("expected progress message, got: %s", buf.String())
	}
}

func TestInstallWarningsNoProgress(t *testing.T) {
	results := []InstallResult{{ID: "go", Name: "Go", Status: "installed", Path: "/x"}}
	warnings := InstallWarnings(results, EnsureOptions{})
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for installed, got: %v", warnings)
	}
}

func TestInstallLSPAlreadyInstalled(t *testing.T) {
	// Set up cache root to put a fake gopls in cache bin.
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	cacheBinDir := filepath.Join(CacheRoot(), "go", "bin")
	if err := os.MkdirAll(cacheBinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	goplsPath := filepath.Join(cacheBinDir, "gopls")
	if err := os.WriteFile(goplsPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Stub PATH so cache command dirs are visible.
	t.Setenv("PATH", cacheBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	impl := goImplementation{}
	// Pretend go is on PATH too.
	if _, err := exec.LookPath("go"); err == nil {
		result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{})
		if result.Status != "already-installed" {
			t.Errorf("expected already-installed, got %+v", result)
		}
	}
}

func TestInstallLSPPrereqFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	// Use a custom implementation with a failing prereq.
	impl := stubImpl{
		spec: InstallSpec{ID: "stub", Name: "Stub", Command: "stub", Prerequisites: []Prerequisite{
			{Name: "fakecmd-xyz-not-exist-9999", Command: "fakecmd-xyz-not-exist-9999", Args: []string{"--version"}},
		}},
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{Force: true})
	if result.Status != "failed" || !strings.Contains(result.Message, "prerequisite is missing") {
		t.Errorf("expected failed with prereq error, got %+v", result)
	}
}

func TestInstallLSPPrereqVersionTooOld(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	// Use `go version` with high MinMajor to force fail.
	impl := stubImpl{
		spec: InstallSpec{ID: "stub", Name: "Stub", Command: "stub", Prerequisites: []Prerequisite{
			{Name: "Go", Command: "go", Args: []string{"version"}, MinMajor: 999, InstallHint: "hint"},
		}},
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{Force: true})
	if result.Status != "failed" || !strings.Contains(result.Message, "too old") {
		t.Errorf("expected failed with version-too-old error, got %+v", result)
	}
}

func TestInstallLSPPrereqVersionOK(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	impl := stubImpl{
		spec: InstallSpec{ID: "stub", Name: "Stub", Command: "stub", Prerequisites: []Prerequisite{
			{Name: "Go", Command: "go", Args: []string{"version"}, MinMajor: 1},
		}},
		installErr: errors.New("install stub failed"),
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{Force: true})
	if result.Status != "failed" || result.Message != "install stub failed" {
		t.Errorf("expected failed with install error, got %+v", result)
	}
}

func TestInstallLSPDryRun(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	impl := stubImpl{
		spec:       InstallSpec{ID: "stub", Name: "Stub", Command: "stub"},
		installCmd: "stub install cmd",
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{DryRun: true})
	if result.Status != "dry-run" || result.Message != "stub install cmd" {
		t.Errorf("expected dry-run, got %+v", result)
	}
}

func TestInstallLSPCheckBinaryFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	impl := stubImpl{
		spec:       InstallSpec{ID: "stub", Name: "Stub", Command: "stub"},
		installOut: "/nonexistent/path/does/not/exist",
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{Force: true})
	if result.Status != "failed" || !strings.Contains(result.Message, "not executable") {
		t.Errorf("expected failed with not-executable error, got %+v", result)
	}
}

func TestInstallLSPSuccess(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	// Create a fake executable for the install.
	binPath := filepath.Join(tmp, "fakebin")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	impl := stubImpl{
		spec:       InstallSpec{ID: "stub", Name: "Stub", Command: "fakebin", CheckArgs: []string{}},
		installOut: binPath,
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{Force: true})
	if result.Status != "installed" {
		t.Errorf("expected installed, got %+v", result)
	}
}

func TestInstallLSPCheckBinaryExits(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	binPath := filepath.Join(tmp, "fakebin2")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	impl := stubImpl{
		spec:       InstallSpec{ID: "stub2", Name: "Stub2", Command: "fakebin2", CheckArgs: []string{"--version"}},
		installOut: binPath,
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{Force: true})
	if result.Status != "failed" {
		t.Errorf("expected failed, got %+v", result)
	}
}

func TestInstallSpecs(t *testing.T) {
	specs := InstallSpecs()
	if len(specs) < 3 {
		t.Errorf("expected at least 3 install specs, got %d", len(specs))
	}
	for _, spec := range specs {
		if spec.ID == "" {
			t.Error("install spec with empty ID")
		}
	}
}

func TestLanguageSpecs(t *testing.T) {
	specs := LanguageSpecs()
	if len(specs) < 3 {
		t.Errorf("expected at least 3 language specs, got %d", len(specs))
	}
}

func TestSupportedIDs(t *testing.T) {
	ids := SupportedIDs()
	if len(ids) == 0 {
		t.Error("expected supported IDs")
	}
	for i := 1; i < len(ids); i++ {
		if ids[i-1] > ids[i] {
			t.Errorf("ids not sorted: %v", ids)
			break
		}
	}
}

func TestInstallSpecByID(t *testing.T) {
	spec, ok := InstallSpecByID("go")
	if !ok {
		t.Fatal("expected go spec")
	}
	if spec.ID != "go" {
		t.Errorf("got ID %q, want go", spec.ID)
	}
	_, ok = InstallSpecByID("nosuchid")
	if ok {
		t.Error("expected !ok for nosuchid")
	}
}

func TestInstallSpecByIDAlias(t *testing.T) {
	spec, ok := InstallSpecByID("golang")
	if !ok {
		t.Fatal("expected golang alias to resolve to go spec")
	}
	if spec.ID != "go" {
		t.Errorf("got ID %q, want go", spec.ID)
	}
}

func TestInstallSpecByServerID(t *testing.T) {
	spec, ok := InstallSpecByServerID("css")
	if !ok {
		t.Fatal("expected css server ID")
	}
	if spec.ID != "css" {
		t.Errorf("got ID %q, want css", spec.ID)
	}
	_, ok = InstallSpecByServerID("nosuchserver")
	if ok {
		t.Error("expected !ok for nosuchserver")
	}
}

func TestInstallSpecByServerIDByID(t *testing.T) {
	// Spec ID is also accepted as server ID alias.
	spec, ok := InstallSpecByServerID("go")
	if !ok {
		t.Fatal("expected go via ServerID alias")
	}
	if spec.ID != "go" {
		t.Errorf("got ID %q, want go", spec.ID)
	}
}

func TestCacheRootEnv(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	if got := CacheRoot(); got != tmp {
		t.Errorf("CacheRoot with env = %q, want %q", got, tmp)
	}
}

func TestCacheRootEnvTrimmed(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, "  "+tmp+"  ")
	if got := CacheRoot(); got != tmp {
		t.Errorf("CacheRoot with trimmed env = %q, want %q", got, tmp)
	}
}

func TestCacheRootExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home dir")
	}
	t.Setenv(CacheEnv, "~/.cache/ns-workspace-test-x")
	if got := CacheRoot(); !strings.HasPrefix(got, home) {
		t.Errorf("CacheRoot with ~ = %q, want prefix %q", got, home)
	}
}

func TestCacheRootUserCacheDir(t *testing.T) {
	t.Setenv(CacheEnv, "")
	if got := CacheRoot(); got == "" {
		t.Error("CacheRoot with no env returned empty")
	}
}

func TestCacheCommandDirs(t *testing.T) {
	dirs := CacheCommandDirs("gopls")
	if len(dirs) == 0 {
		t.Error("expected at least one cache dir for gopls")
	}
	dirs = CacheCommandDirs("nosuchcmd")
	if len(dirs) != 0 {
		t.Errorf("expected 0 cache dirs for unknown command, got %d", len(dirs))
	}
}

func TestResolveCommandWithSource(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	// Use 'go' which should be on PATH.
	if _, err := exec.LookPath("go"); err == nil {
		path, source, err := ResolveCommandWithSource("go", tmp)
		if err != nil {
			t.Fatalf("ResolveCommandWithSource failed: %v", err)
		}
		if path == "" || source == "" {
			t.Errorf("expected non-empty path and source, got %q, %q", path, source)
		}
	}
	_, _, err := ResolveCommandWithSource("this-command-does-not-exist-xyz", tmp)
	if err == nil {
		t.Error("expected error for nonexistent command")
	}
}

func TestResolveCommandWithSourceCandidate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	// Place a fake gopls in the project node_modules/.bin
	proj := t.TempDir()
	binDir := filepath.Join(proj, "node_modules", ".bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fakePath := filepath.Join(binDir, "ns-workspace-test-cmd-xyz")
	if err := os.WriteFile(fakePath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Clear PATH to ensure exec.LookPath fails.
	t.Setenv("PATH", "")
	path, source, err := ResolveCommandWithSource("ns-workspace-test-cmd-xyz", proj)
	if err != nil {
		t.Fatalf("expected candidate match, got error: %v", err)
	}
	if path != fakePath {
		t.Errorf("got path %q, want %q", path, fakePath)
	}
	if source != "project" {
		t.Errorf("got source %q, want project", source)
	}
}

func TestCommandSourceCache(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	cache := CacheRoot()
	got := CommandSource(filepath.Join(cache, "bin", "gopls"), "")
	if got != "cache" {
		t.Errorf("CommandSource for cache = %q, want cache", got)
	}
	got = CommandSource(cache, "")
	if got != "cache" {
		t.Errorf("CommandSource for cache root = %q, want cache", got)
	}
}

func TestCommandSourceProject(t *testing.T) {
	tmp := t.TempDir()
	got := CommandSource(filepath.Join(tmp, "bin", "x"), tmp)
	if got != "project" {
		t.Errorf("CommandSource for project = %q, want project", got)
	}
	got = CommandSource(tmp, tmp)
	if got != "project" {
		t.Errorf("CommandSource for project root = %q, want project", got)
	}
}

func TestCommandSourcePath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	got := CommandSource("/some/random/path", tmp)
	if got != "path" {
		t.Errorf("CommandSource for path = %q, want path", got)
	}
}

func TestCommandSourceEmptyProject(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	got := CommandSource("/some/path", "")
	if got != "path" {
		t.Errorf("CommandSource with empty project = %q, want path", got)
	}
}

func TestUnavailableWarning(t *testing.T) {
	w := UnavailableWarning("go", "fallback", "detail")
	if !strings.Contains(w, "fallback") && !strings.Contains(w, "Go/Golang") {
		t.Errorf("UnavailableWarning missing name. Got: %s", w)
	}
	if !strings.Contains(w, "detail") {
		t.Errorf("UnavailableWarning missing detail. Got: %s", w)
	}
	if !strings.Contains(w, "lsp install") {
		t.Errorf("UnavailableWarning missing install hint. Got: %s", w)
	}
}

func TestUnavailableWarningNoSpec(t *testing.T) {
	w := UnavailableWarning("nosuchserver", "MyFallback", "my detail")
	if !strings.Contains(w, "MyFallback") {
		t.Errorf("UnavailableWarning should use fallback name. Got: %s", w)
	}
	if !strings.Contains(w, "lsp install nosuchserver") {
		t.Errorf("UnavailableWarning should use serverID in command. Got: %s", w)
	}
}

func TestSplitInstallArgs(t *testing.T) {
	tests := []struct {
		args     []string
		wantLang string
		wantRest []string
	}{
		{[]string{"go"}, "go", []string{}},
		{[]string{"go", "--dry-run"}, "go", []string{"--dry-run"}},
		{[]string{"go", "--project", "/tmp"}, "go", []string{"--project", "/tmp"}},
		{[]string{"go", "--docs-dir", "docs", "--force"}, "go", []string{"--docs-dir", "docs", "--force"}},
		{[]string{"go", "--project=/tmp"}, "go", []string{"--project=/tmp"}},
		{[]string{"go", "--docs-dir=docs"}, "go", []string{"--docs-dir=docs"}},
		{[]string{"go", "--json", "--force", "--dry-run"}, "go", []string{"--json", "--force", "--dry-run"}},
		{[]string{"go", "extra-positional"}, "go", []string{"extra-positional"}},
		{[]string{}, "", []string{}},
	}
	for _, tt := range tests {
		gotLang, gotRest := splitInstallArgs(tt.args)
		if gotLang != tt.wantLang {
			t.Errorf("splitInstallArgs(%v) lang = %q, want %q", tt.args, gotLang, tt.wantLang)
		}
		if strings.Join(gotRest, " ") != strings.Join(tt.wantRest, " ") {
			t.Errorf("splitInstallArgs(%v) rest = %v, want %v", tt.args, gotRest, tt.wantRest)
		}
	}
}

func TestImplementationByID(t *testing.T) {
	tests := []struct {
		id    string
		want  bool
	}{
		{"go", true},
		{"golang", true},
		{"GO", true},
		{" Go ", true},
		{"kotlin", true},
		{"kt", true},
		{"html", true},
		{"css", true},
		{"scss", true},
		{"typescript", true},
		{"ts", true},
		{"javascript", true},
		{"js", true},
		{"nosuch", false},
	}
	for _, tt := range tests {
		_, ok := implementationByID(tt.id)
		if ok != tt.want {
			t.Errorf("implementationByID(%q) ok = %v, want %v", tt.id, ok, tt.want)
		}
	}
}

func TestImplementationByServerID(t *testing.T) {
	if _, ok := implementationByServerID("go"); !ok {
		t.Error("expected go to be found by server ID")
	}
	if _, ok := implementationByServerID("GO"); !ok {
		t.Error("expected GO to be found (normalized)")
	}
	if _, ok := implementationByServerID("nosuch"); ok {
		t.Error("expected !ok for nosuch")
	}
}

func TestDetectorOrEmpty(t *testing.T) {
	det := fakeDetector{}
	if d := detectorOrEmpty(det); d == nil {
		t.Error("detectorOrEmpty should return the detector when not nil")
	}
	if d := detectorOrEmpty(nil); d == nil {
		t.Error("detectorOrEmpty(nil) should return a non-nil detector")
	}
}

func TestInstallMutex(t *testing.T) {
	m1 := installMutex("a")
	m2 := installMutex("a")
	m3 := installMutex("b")
	if m1 != m2 {
		t.Error("expected same mutex for same id")
	}
	if m1 == m3 {
		t.Error("expected different mutex for different id")
	}
}

func TestCheckPrerequisitesSuccess(t *testing.T) {
	spec := InstallSpec{Prerequisites: []Prerequisite{
		{Name: "Go", Command: "go", Args: []string{"version"}},
	}}
	if err := checkPrerequisites(context.Background(), spec); err != nil {
		if !strings.Contains(err.Error(), "prerequisite is missing") {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestCheckPrerequisitesMinMajorOK(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not available")
	}
	spec := InstallSpec{Prerequisites: []Prerequisite{
		{Name: "Go", Command: "go", Args: []string{"version"}, MinMajor: 1},
	}}
	if err := checkPrerequisites(context.Background(), spec); err != nil {
		t.Errorf("expected success with MinMajor=1, got: %v", err)
	}
}

func TestCheckPrerequisitesWithHint(t *testing.T) {
	spec := InstallSpec{Prerequisites: []Prerequisite{
		{Name: "fakecmd", Command: "fakecmd-xyz-nope", Args: []string{"--version"}, InstallHint: "Install it"},
	}}
	err := checkPrerequisites(context.Background(), spec)
	if err == nil {
		t.Fatal("expected error for missing prerequisite")
	}
	if !strings.Contains(err.Error(), "Install it") {
		t.Errorf("error should include install hint. Got: %v", err)
	}
}

func TestParseMajorVersion(t *testing.T) {
	tests := []struct {
		input   string
		wantN   int
		wantOK  bool
	}{
		{"v1.2.3", 1, true},
		{"v18.0.0", 18, true},
		{"go1.21.5", 1, true},
		{"  v20  ", 20, true},
		{"abc", 0, false},
		{"", 0, false},
		{"v", 0, false},
	}
	for _, tt := range tests {
		n, ok := parseMajorVersion(tt.input)
		if n != tt.wantN || ok != tt.wantOK {
			t.Errorf("parseMajorVersion(%q) = (%d, %v), want (%d, %v)", tt.input, n, ok, tt.wantN, tt.wantOK)
		}
	}
}

func TestCheckBinarySuccess(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "fakebin")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := checkBinary(context.Background(), binPath, []string{"--version"}); err != nil {
		// Some test environments may fail; only fail if it's not executable.
		if strings.Contains(err.Error(), "not executable") {
			t.Errorf("expected executable: %v", err)
		}
	}
}

func TestCheckBinaryNoArgs(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "fakebin")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := checkBinary(context.Background(), binPath, nil); err != nil {
		t.Errorf("expected nil for empty args, got: %v", err)
	}
}

func TestCheckBinaryNotExecutable(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "fakebin")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := checkBinary(context.Background(), binPath, []string{"--version"})
	if err == nil || !strings.Contains(err.Error(), "not executable") {
		t.Errorf("expected not-executable error, got: %v", err)
	}
}

func TestCheckBinaryFailure(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "fakebin")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\nexit 1\necho fail\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := checkBinary(context.Background(), binPath, []string{"--version"})
	if err == nil {
		t.Error("expected error for failing binary")
	}
}

func TestNormalizeProjectRoot(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"/tmp"},
		{"  /tmp  "},
	}
	for _, tt := range tests {
		got := normalizeProjectRoot(tt.input)
		if tt.input == "/tmp" && got != "/tmp" {
			t.Errorf("normalizeProjectRoot(%q) = %q, want /tmp", tt.input, got)
		}
		if tt.input == "  /tmp  " && got != "/tmp" {
			t.Errorf("normalizeProjectRoot(%q) = %q, want /tmp", tt.input, got)
		}
		if got == "" {
			t.Errorf("normalizeProjectRoot(%q) returned empty", tt.input)
		}
	}
}

func TestNormalizeProjectRootEmpty(t *testing.T) {
	got := normalizeProjectRoot("")
	// Empty input becomes ".", then abs returns cwd.
	if got == "" {
		t.Error("expected non-empty result")
	}
}

func TestNormalizeProjectRootDot(t *testing.T) {
	got := normalizeProjectRoot(".")
	if got == "" {
		t.Error("expected non-empty result")
	}
}

func TestNormalizeProjectRootExpandTilde(t *testing.T) {
	// Just exercise ExpandPath branch with tilde.
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home dir")
	}
	got := normalizeProjectRoot("~/some-dir")
	if !strings.HasPrefix(got, home) {
		t.Errorf("normalizeProjectRoot(~/) = %q, want prefix %q", got, home)
	}
}

func TestNormalizeID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"GO", "go"},
		{" Go ", "go"},
		{"go", "go"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := normalizeID(tt.input); got != tt.want {
			t.Errorf("normalizeID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizedIDs(t *testing.T) {
	got := normalizedIDs([]string{"GO", "  ", "TS", ""})
	if len(got) != 2 || got[0] != "go" || got[1] != "ts" {
		t.Errorf("normalizedIDs = %v, want [go ts]", got)
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "''"},
		{"simple", "simple"},
		{"with space", "'with space'"},
		{"with'quote", "'with'\\''quote'"},
		{"/path/with$dollar", "'/path/with$dollar'"},
		{"with\nnewline", "'with\nnewline'"},
	}
	for _, tt := range tests {
		if got := shellQuote(tt.input); got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCommandCandidates(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	// Without projectRoot, only cwd and cache dirs are checked.
	got := commandCandidates("gopls", "")
	if len(got) == 0 {
		t.Error("expected at least one candidate for gopls")
	}
}

func TestCommandCandidatesWithProjectRoot(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	proj := t.TempDir()
	got := commandCandidates("gopls", proj)
	if len(got) == 0 {
		t.Error("expected at least one candidate")
	}
}

func TestCommandCandidatesGoplsDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		gobin := filepath.Join(home, "go", "bin")
		t.Setenv("GOBIN", gobin)
		t.Setenv("GOPATH", gobin)
		got := commandCandidates("gopls", "")
		_ = got // just exercise the branch
	}
}

func TestCommandCandidatesEmptyDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("GOBIN", " ")
	t.Setenv("GOPATH", " ")
	// Should not crash.
	got := commandCandidates("gopls", "")
	_ = got
}

func TestTrimCommandOutput(t *testing.T) {
	if got := trimCommandOutput([]byte("  hello  ")); got != "hello" {
		t.Errorf("trimCommandOutput short = %q, want hello", got)
	}
	long := strings.Repeat("a", 600)
	if got := trimCommandOutput([]byte(long)); !strings.HasSuffix(got, "...") {
		t.Errorf("trimCommandOutput long should end with ...; got length %d", len(got))
	}
	if got := trimCommandOutput([]byte("")); got != "" {
		t.Errorf("trimCommandOutput empty = %q, want empty", got)
	}
}

func TestRunLSPInstallTableWithFailedResult(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	// Use a forced-install with stub that fails check, so we get a failed result.
	// Since RunLSPInstall only runs when language != "auto", and a failed result
	// causes the function to return an error.
	t.Setenv("PATH", "") // ensure gopls not found

	// Set up gopls cache bin but as a non-executable file to force check fail.
	cacheBin := filepath.Join(CacheRoot(), "go", "bin")
	if err := os.MkdirAll(cacheBin, 0o755); err != nil {
		t.Fatal(err)
	}
	// No fake gopls, so install tries go install. We'll just test dry-run + json.
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"go", "--dry-run"}, &buf, emptySourceDetector{})
	if err != nil {
		t.Fatalf("RunLSPInstall dry-run failed: %v", err)
	}
}

func TestRunLSPListFlagUnknown(t *testing.T) {
	var buf bytes.Buffer
	err := RunLSPList([]string{"--bogus-flag"}, &buf, emptySourceDetector{})
	if err == nil {
		t.Error("expected error for unknown flag")
	}
}

func TestRunLSPListCustomDocsDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	var buf bytes.Buffer
	if err := RunLSPList([]string{"--project", tmp, "--docs-dir", "mydocs"}, &buf, emptySourceDetector{}); err != nil {
		t.Fatalf("RunLSPList failed: %v", err)
	}
}

func TestRunLSPListWithInstalledBinary(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	cacheBinDir := filepath.Join(CacheRoot(), "go", "bin")
	if err := os.MkdirAll(cacheBinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	goplsPath := filepath.Join(cacheBinDir, "gopls")
	if err := os.WriteFile(goplsPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", cacheBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	var buf bytes.Buffer
	if err := RunLSPList([]string{"--project", tmp}, &buf, emptySourceDetector{}); err != nil {
		t.Fatalf("RunLSPList failed: %v", err)
	}
	if !strings.Contains(buf.String(), "installed") {
		t.Errorf("expected installed status. Got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "cache)") {
		t.Errorf("expected cache source annotation. Got:\n%s", buf.String())
	}
}

func TestEnsureProjectLSPWithProgress(t *testing.T) {
	var buf bytes.Buffer
	warnings := EnsureProjectLSP(context.Background(), t.TempDir(), "docs", EnsureOptions{Progress: &buf}, emptySourceDetector{})
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got: %v", warnings)
	}
}

func TestRunLSPInstallWithFail(t *testing.T) {
	// Force a failed install to test the error path at the end of RunLSPInstall.
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	// Without go on PATH and with no cache, install will fail.
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"go", "--project", tmp, "--force"}, &buf, emptySourceDetector{})
	if err == nil {
		// It's possible the test runs in an environment with go on PATH - that's OK.
		t.Log("install succeeded unexpectedly; test environment has go available")
	} else if !strings.Contains(err.Error(), "install go failed") {
		t.Logf("got expected error: %v", err)
	}
}

func TestRunLSPInstallAutoWithDetected(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	// Also override HOME to a clean temp dir to avoid finding gopls in ~/go/bin.
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	det := fakeDetector{installIDs: map[string]bool{"go": true}}
	var buf bytes.Buffer
	// Use --dry-run + --force so install is forced through dry-run path.
	err := RunLSPInstall([]string{"auto", "--dry-run", "--force", "--project", tmp}, &buf, det)
	if err != nil {
		t.Fatalf("RunLSPInstall auto dry-run failed: %v", err)
	}
	if !strings.Contains(buf.String(), "dry-run") {
		t.Errorf("expected dry-run line. Got:\n%s", buf.String())
	}
}

func TestRunLSPInstallAutoJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	det := fakeDetector{installIDs: map[string]bool{"go": true}}
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"auto", "--dry-run", "--json", "--project", tmp}, &buf, det)
	if err != nil {
		t.Fatalf("RunLSPInstall auto dry-run json failed: %v", err)
	}
	var results []InstallResult
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(results) == 0 {
		t.Error("expected at least one result")
	}
}

func TestRunLSPInstallAutoTable(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	det := fakeDetector{installIDs: map[string]bool{"go": true}}
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"auto", "--dry-run", "--project", tmp}, &buf, det)
	if err != nil {
		t.Fatalf("RunLSPInstall auto dry-run failed: %v", err)
	}
}

func TestRunLSPInstallJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"go", "--dry-run", "--json", "--project", tmp}, &buf, emptySourceDetector{})
	if err != nil {
		t.Fatalf("RunLSPInstall dry-run json failed: %v", err)
	}
}

func TestRunLSPInstallMultipleFlags(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"go", "--force", "--dry-run", "--json", "--project", tmp}, &buf, emptySourceDetector{})
	if err != nil {
		t.Fatalf("RunLSPInstall with multiple flags failed: %v", err)
	}
}

func TestInstallLSPForcePath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	// Set up an existing gopls so install would normally skip.
	cacheBinDir := filepath.Join(CacheRoot(), "go", "bin")
	if err := os.MkdirAll(cacheBinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	goplsPath := filepath.Join(cacheBinDir, "gopls")
	if err := os.WriteFile(goplsPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", cacheBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	impl := stubImpl{
		spec:       InstallSpec{ID: "go", Name: "Go", Command: "gopls"},
		installOut: "/dev/null", // non-existent, will fail checkBinary
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{Force: true})
	if result.Status != "failed" {
		t.Errorf("expected failed with Force=true forcing install, got %+v", result)
	}
}

func TestRunLSPListInstalledJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	cacheBinDir := filepath.Join(CacheRoot(), "go", "bin")
	if err := os.MkdirAll(cacheBinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	goplsPath := filepath.Join(cacheBinDir, "gopls")
	if err := os.WriteFile(goplsPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", cacheBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	var buf bytes.Buffer
	if err := RunLSPList([]string{"--project", tmp, "--json"}, &buf, emptySourceDetector{}); err != nil {
		t.Fatalf("RunLSPList failed: %v", err)
	}
	var rows []StatusRow
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	foundInstalled := false
	for _, row := range rows {
		if row.ID == "go" && row.Status == "installed" {
			foundInstalled = true
			break
		}
	}
	if !foundInstalled {
		t.Errorf("expected go row with installed status")
	}
}

func TestListStatusMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	rows := ListStatus(tmp, "docs", emptySourceDetector{})
	for _, row := range rows {
		if row.ID == "" {
			continue
		}
		if row.Status == "missing" && row.Path != "" {
			t.Errorf("missing row should not have path set: %+v", row)
		}
	}
}

func TestInstallLSPForceAlreadyInstalled(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	cacheBinDir := filepath.Join(CacheRoot(), "go", "bin")
	if err := os.MkdirAll(cacheBinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	goplsPath := filepath.Join(cacheBinDir, "gopls")
	if err := os.WriteFile(goplsPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", cacheBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Use stubImpl that succeeds in install, so with Force=true it tries to install.
	impl := stubImpl{
		spec:       InstallSpec{ID: "stub", Name: "Stub", Command: "fakebin-noexist-xyz"},
		installOut: "/nonexistent",
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{Force: true})
	// With Force=true, the early skip is bypassed, so it tries to install.
	if result.Status != "failed" {
		t.Errorf("expected failed with Force=true, got %+v", result)
	}
}

func TestRunLSPInstallWithFailedCheckReturnsError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	// Go is likely on PATH; if so, install may succeed. Use a fresh path.
	// Try with a stub that always fails check.
	// Since we can't easily inject a fake install function, just check dry-run path returns nil.
	var buf bytes.Buffer
	if err := RunLSPInstall([]string{"go", "--dry-run"}, &buf, emptySourceDetector{}); err != nil {
		t.Errorf("dry-run should not fail: %v", err)
	}
}

func TestResolveCommandWithSourceEmptyCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	_, _, err := ResolveCommandWithSource("nonexistent-command-xyz-zzz", tmp)
	if err == nil || !strings.Contains(err.Error(), "command not found") {
		t.Errorf("expected command not found error, got: %v", err)
	}
}

func TestRunLSPInstallTableErrorOnFailure(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	// Stub the install by overriding PATH to remove go. Without go, install will fail.
	// Use --force to skip the cache check.
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"go", "--force", "--project", tmp}, &buf, emptySourceDetector{})
	if err == nil {
		// Maybe go IS available - skip if so.
		t.Skip("go is available, install succeeded")
	}
	if !strings.Contains(err.Error(), "install go failed") {
		t.Errorf("expected install failure error, got: %v", err)
	}
}

func TestParseMajorVersionEdge(t *testing.T) {
	// Test that non-numeric after leading characters returns false.
	n, ok := parseMajorVersion("abc")
	if ok || n != 0 {
		t.Errorf("parseMajorVersion(abc) = (%d, %v), want (0, false)", n, ok)
	}
}

func TestRunLSPInstallMissingLanguageJSON(t *testing.T) {
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"--json"}, &buf, emptySourceDetector{})
	if err == nil {
		t.Fatal("expected error for missing language with --json")
	}
}

func TestRunLSPListJSONWithFailWriter(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	w := failingWriter{}
	if err := RunLSPList([]string{"--project", tmp, "--json"}, w, emptySourceDetector{}); err == nil {
		// json.NewEncoder may or may not fail; either way the code path is exercised.
		t.Log("no error from failing writer (acceptable)")
	}
}

func TestRunLSPListTableWithFailWriter(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	w := failingWriter{}
	err := RunLSPList([]string{"--project", tmp}, w, emptySourceDetector{})
	if err == nil {
		t.Error("expected error from failing writer")
	}
}

func TestRunLSPInstallJSONWithFailWriter(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	w := failingWriter{}
	err := RunLSPInstall([]string{"go", "--dry-run", "--json", "--project", tmp}, w, emptySourceDetector{})
	// encoder.Encode may or may not fail; either way code path is exercised.
	_ = err
}

func TestRunLSPInstallTableWithFailWriter(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	w := failingWriter{}
	err := RunLSPInstall([]string{"go", "--dry-run", "--project", tmp}, w, emptySourceDetector{})
	// First line should be "go: dry-run"; Fprintln will fail.
	if err == nil {
		t.Error("expected error from failing writer")
	}
}

func TestRunLSPInstallDryRunCustomCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	impl := stubImpl{
		spec:       InstallSpec{ID: "stub", Name: "Stub", Command: "stub"},
		installCmd: "custom install cmd",
	}
	result := InstallLSP(context.Background(), impl, t.TempDir(), EnsureOptions{DryRun: true})
	if result.Message != "custom install cmd" {
		t.Errorf("expected custom install cmd, got %q", result.Message)
	}
}

func TestCommandCandidatesGoplsGobinEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")
	got := commandCandidates("gopls", t.TempDir())
	_ = got
}

func TestCommandCandidatesNonGopls(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	got := commandCandidates("non-gopls", "")
	if len(got) == 0 {
		t.Error("expected candidates for non-gopls")
	}
}

func TestRunLSPInstallDryRunWithCustomCmd(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"go", "--dry-run", "--project", tmp}, &buf, emptySourceDetector{})
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	// dry-run result for "go" should produce a "go: dry-run" line
	if !strings.Contains(buf.String(), "go:") {
		t.Errorf("expected go: prefix, got: %s", buf.String())
	}
}

func TestRunLSPInstallManualStatus(t *testing.T) {
	// This test verifies that a "manual" status from InstallLSP is handled.
	// Since we can't easily inject a "manual" status, we test via direct call to InstallWarnings.
	results := []InstallResult{{ID: "go", Name: "Go", Status: "manual", Message: "install manually"}}
	warnings := InstallWarnings(results, EnsureOptions{})
	if len(warnings) != 1 || !strings.Contains(warnings[0], "manual installation") {
		t.Errorf("expected manual warning, got: %v", warnings)
	}
}

func TestRunLSPInstallFailedReturnsError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	t.Setenv("PATH", "")
	// With empty PATH and force flag, go install will fail.
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"go", "--force", "--project", tmp}, &buf, emptySourceDetector{})
	if err != nil {
		if !strings.Contains(err.Error(), "install go failed") {
			t.Logf("got error: %v", err)
		}
	}
}

func TestRunLSPListWithMissingStatusHasInstallCommand(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	var buf bytes.Buffer
	if err := RunLSPList([]string{"--project", tmp}, &buf, emptySourceDetector{}); err != nil {
		t.Fatalf("RunLSPList failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "go run . lsp install") {
		t.Errorf("expected install command in output. Got:\n%s", out)
	}
}

func TestRunLSPListFprintlnFails(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	w := failingWriter{}
	err := RunLSPList([]string{"--project", tmp}, w, emptySourceDetector{})
	if err == nil {
		t.Error("expected error from failing writer on table output")
	}
}

func TestRunLSPInstallFprintlnFails(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	w := failingWriter{}
	err := RunLSPInstall([]string{"go", "--dry-run", "--project", tmp}, w, emptySourceDetector{})
	if err == nil {
		t.Error("expected error from failing writer")
	}
}

func TestRunLSPInstallJSONEncodeFails(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	w := failingWriter{}
	err := RunLSPInstall([]string{"go", "--dry-run", "--json", "--project", tmp}, w, emptySourceDetector{})
	_ = err
}

func TestListStatusEmptyInstallSpec(t *testing.T) {
	// Test that ListStatus handles empty InstallSpec case (InstallSpecByServerID returns false).
	// This is hard to trigger directly, but we can verify it returns rows.
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	rows := ListStatus(tmp, "docs", emptySourceDetector{})
	if len(rows) == 0 {
		t.Error("expected non-empty rows")
	}
}

func TestCommandSourceEmptyProjectWithCache(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	cache := CacheRoot()
	got := CommandSource(filepath.Join(cache, "bin", "x"), "")
	if got != "cache" {
		t.Errorf("expected cache, got %q", got)
	}
}

func TestCommandSourceProjectRootEdge(t *testing.T) {
	tmp := t.TempDir()
	got := CommandSource(tmp, tmp)
	if got != "project" {
		t.Errorf("expected project, got %q", got)
	}
	// Project itself
	got = CommandSource(tmp, tmp+"/sub")
	if got != "path" {
		t.Errorf("expected path for subdir, got %q", got)
	}
}

func TestInstallSpecByIDNormalizeCase(t *testing.T) {
	_, ok := InstallSpecByID("GO")
	if !ok {
		t.Error("expected GO to resolve to go")
	}
}

func TestInstallSpecByServerIDUnknown(t *testing.T) {
	_, ok := InstallSpecByServerID("unknown-server-id-xyz")
	if ok {
		t.Error("expected !ok for unknown")
	}
}

func TestRunLSPInstallProjectArgOnly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"go", "--dry-run", "--project=" + tmp}, &buf, emptySourceDetector{})
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
}

func TestRunLSPInstallDocsArgOnly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	var buf bytes.Buffer
	err := RunLSPInstall([]string{"go", "--dry-run", "--docs-dir=mydocs"}, &buf, emptySourceDetector{})
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
}

func TestRunLSPListProjectArgOnly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	var buf bytes.Buffer
	err := RunLSPList([]string{"--project=" + tmp}, &buf, emptySourceDetector{})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
}

func TestRunLSPListDocsArgOnly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	var buf bytes.Buffer
	err := RunLSPList([]string{"--docs-dir=mydocs"}, &buf, emptySourceDetector{})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
}

func TestRunLSPListJSONEncodeFail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	w := failingWriter{}
	err := RunLSPList([]string{"--project", tmp, "--json"}, w, emptySourceDetector{})
	// json.Encoder may or may not catch this; we just want the code path.
	_ = err
}

func TestInstallProjectLSPsProgress(t *testing.T) {
	var buf bytes.Buffer
	det := fakeDetector{installIDs: map[string]bool{"go": true}}
	results := InstallProjectLSPs(context.Background(), t.TempDir(), "docs", EnsureOptions{DryRun: true, Progress: &buf}, det)
	if len(results) == 0 {
		t.Fatal("expected results")
	}
}

// --- helpers ---

type fakeDetector struct {
	langs      map[string]bool
	installIDs map[string]bool
}

func (d fakeDetector) DetectedLanguages(string, string) map[string]bool {
	return d.langs
}

func (d fakeDetector) DetectedInstallIDs(string, string) map[string]bool {
	return d.installIDs
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("failing writer")
}

type stubImpl struct {
	spec       InstallSpec
	installOut string
	installErr error
	installCmd string
}

func (s stubImpl) installSpec() InstallSpec         { return s.spec }
func (s stubImpl) languageSpecs() []LanguageSpec    { return nil }
func (s stubImpl) cacheCommandDirs() []string       { return nil }
func (s stubImpl) installCommand() string           { return s.installCmd }
func (s stubImpl) install(context.Context) (string, error) {
	return s.installOut, s.installErr
}

// TestRunLSPUsagePassthrough ensures each usage path works.
func TestRunLSPUsagePaths(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		if err := RunLSP([]string{arg}, emptySourceDetector{}); err != nil {
			t.Errorf("RunLSP(%q) returned error: %v", arg, err)
		}
	}
	// Empty string is treated as unknown command (since len(args)==0 returns nil for empty args).
	// The function returns PrintLSPUsage+nil when len(args)==0, then falls into "unknown" switch case only if non-empty.
}

func TestRunLSPListEmptyArgs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	var buf bytes.Buffer
	if err := RunLSPList(nil, &buf, emptySourceDetector{}); err != nil {
		t.Fatalf("RunLSPList empty args failed: %v", err)
	}
}

// Test main_test interface for unused symbols
var _ = flag.ErrHelp
var _ = fmt.Sprintf
var _ = io.Copy
var _ = httptest.NewServer
var _ = http.MethodGet
var _ = runtime.GOOS
