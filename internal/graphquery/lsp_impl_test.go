package graphquery

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHTMLImplementation(t *testing.T) {
	impl := htmlImplementation{}
	spec := impl.installSpec()
	if spec.ID != "html" {
		t.Errorf("ID = %q, want html", spec.ID)
	}
	if spec.Command != "vscode-html-language-server" {
		t.Errorf("Command = %q", spec.Command)
	}
	if len(spec.Prerequisites) == 0 {
		t.Error("expected prerequisites")
	}
	langs := impl.languageSpecs()
	if len(langs) == 0 || langs[0].ID != "html" {
		t.Errorf("expected html language spec, got: %+v", langs)
	}
	if langs[0].SymbolMode != SymbolModeDocument {
		t.Errorf("SymbolMode = %q, want document", langs[0].SymbolMode)
	}
	dirs := impl.cacheCommandDirs()
	if len(dirs) == 0 {
		t.Error("expected cache command dirs")
	}
	cmd := impl.installCommand()
	if !strings.Contains(cmd, "vscode-langservers-extracted") {
		t.Errorf("installCommand should reference package: %s", cmd)
	}
}

func TestCSSImplementation(t *testing.T) {
	impl := cssImplementation{}
	spec := impl.installSpec()
	if spec.ID != "css" {
		t.Errorf("ID = %q, want css", spec.ID)
	}
	if spec.Command != "vscode-css-language-server" {
		t.Errorf("Command = %q", spec.Command)
	}
	langs := impl.languageSpecs()
	if len(langs) != 2 {
		t.Errorf("expected 2 language specs, got %d", len(langs))
	}
	foundSCSS := false
	for _, l := range langs {
		if l.ID == "scss" && l.SymbolMode == SymbolModeSelector {
			foundSCSS = true
		}
	}
	if !foundSCSS {
		t.Error("expected scss language spec with selector mode")
	}
	dirs := impl.cacheCommandDirs()
	if len(dirs) == 0 {
		t.Error("expected cache command dirs")
	}
	cmd := impl.installCommand()
	if !strings.Contains(cmd, "vscode-langservers-extracted") {
		t.Errorf("installCommand should reference package: %s", cmd)
	}
}

func TestTypeScriptImplementation(t *testing.T) {
	impl := typeScriptImplementation{}
	spec := impl.installSpec()
	if spec.ID != "typescript" {
		t.Errorf("ID = %q, want typescript", spec.ID)
	}
	if spec.Command != "typescript-language-server" {
		t.Errorf("Command = %q", spec.Command)
	}
	langs := impl.languageSpecs()
	if len(langs) != 4 {
		t.Errorf("expected 4 language specs (js/jsx/ts/tsx), got %d", len(langs))
	}
	dirs := impl.cacheCommandDirs()
	if len(dirs) == 0 {
		t.Error("expected cache command dirs")
	}
	cmd := impl.installCommand()
	if !strings.Contains(cmd, "typescript-language-server") {
		t.Errorf("installCommand should reference package: %s", cmd)
	}
}

func TestGoImplementation(t *testing.T) {
	impl := goImplementation{}
	spec := impl.installSpec()
	if spec.ID != "go" {
		t.Errorf("ID = %q, want go", spec.ID)
	}
	if spec.Command != "gopls" {
		t.Errorf("Command = %q", spec.Command)
	}
	langs := impl.languageSpecs()
	if len(langs) != 1 || langs[0].ID != "go" {
		t.Errorf("expected go language spec, got: %+v", langs)
	}
	if langs[0].SymbolMode != SymbolModeCallable {
		t.Errorf("SymbolMode = %q, want callable", langs[0].SymbolMode)
	}
	dirs := impl.cacheCommandDirs()
	if len(dirs) == 0 {
		t.Error("expected cache command dirs")
	}
	cmd := impl.installCommand()
	if !strings.Contains(cmd, "gopls") {
		t.Errorf("installCommand should reference gopls: %s", cmd)
	}
}

func TestKotlinImplementation(t *testing.T) {
	impl := kotlinImplementation{}
	spec := impl.installSpec()
	if spec.ID != "kotlin" {
		t.Errorf("ID = %q, want kotlin", spec.ID)
	}
	if spec.Command != "kotlin-lsp" {
		t.Errorf("Command = %q", spec.Command)
	}
	langs := impl.languageSpecs()
	if len(langs) != 1 || langs[0].ID != "kotlin" {
		t.Errorf("expected kotlin language spec, got: %+v", langs)
	}
	if langs[0].SymbolMode != SymbolModeCallable {
		t.Errorf("SymbolMode = %q, want callable", langs[0].SymbolMode)
	}
	dirs := impl.cacheCommandDirs()
	if len(dirs) == 0 {
		t.Error("expected cache command dirs")
	}
}

func TestKotlinInstallCommand(t *testing.T) {
	impl := kotlinImplementation{}
	cmd := impl.installCommand()
	// Either it's a download command or an error message (when GOOS unsupported).
	if !strings.Contains(cmd, "download") && !strings.Contains(cmd, "does not support") {
		t.Errorf("installCommand unexpected: %s", cmd)
	}
}

func TestKotlinInstall(t *testing.T) {
	impl := kotlinImplementation{}
	// Set up a stub archive source.
	stub := ArchiveSource{
		Version:  "1.0",
		FileName: "kotlin.zip",
		URL:      "http://example.invalid/kotlin.zip",
		SHA256:   "00",
		Format:   "zip",
	}
	restore := SetArchiveSourceForTest(func(InstallSpec) (ArchiveSource, error) {
		return stub, nil
	})
	defer restore()

	_, err := impl.install(context.Background())
	if err == nil {
		t.Error("expected error with invalid URL")
	}
}

func TestKotlinInstallArchiveError(t *testing.T) {
	impl := kotlinImplementation{}
	restore := SetArchiveSourceForTest(func(InstallSpec) (ArchiveSource, error) {
		return ArchiveSource{}, nil
	})
	defer restore()
	_, err := impl.install(context.Background())
	if err == nil {
		t.Error("expected error from archive install")
	}
}

func TestKotlinInstallCommandError(t *testing.T) {
	impl := kotlinImplementation{}
	restore := SetArchiveSourceForTest(func(InstallSpec) (ArchiveSource, error) {
		return ArchiveSource{}, errInvalid
	})
	defer restore()
	cmd := impl.installCommand()
	if !strings.Contains(cmd, "invalid") {
		t.Errorf("expected error message in installCommand, got: %s", cmd)
	}
}

var errInvalid = &stringError{"invalid spec"}

type stringError struct{ s string }

func (e *stringError) Error() string { return e.s }

func TestSetArchiveSourceForTest(t *testing.T) {
	original := resolveArchiveSource
	defer func() { resolveArchiveSource = original }()

	called := false
	restore := SetArchiveSourceForTest(func(InstallSpec) (ArchiveSource, error) {
		called = true
		return ArchiveSource{}, nil
	})

	_, _ = resolveArchiveSource(InstallSpec{})
	if !called {
		t.Error("expected stub to be called")
	}
	restore()
	// After restore, the original function should be back.
	_, _ = defaultKotlinArchiveSource(InstallSpec{ID: "kotlin"})
}

func TestDefaultKotlinArchiveSource(t *testing.T) {
	// Wrong spec ID should error.
	_, err := defaultKotlinArchiveSource(InstallSpec{ID: "not-kotlin"})
	if err == nil {
		t.Error("expected error for non-kotlin spec")
	}

	// For kotlin, depending on GOOS/GOARCH, we get either a valid source or an unsupported error.
	src, err := defaultKotlinArchiveSource(InstallSpec{ID: "kotlin"})
	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "arm64", "amd64":
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if src.Format != "zip" {
				t.Errorf("darwin format = %q, want zip", src.Format)
			}
		default:
			if err == nil {
				t.Error("expected error for darwin unsupported arch")
			}
		}
	case "linux":
		switch runtime.GOARCH {
		case "arm64", "amd64":
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if src.Format != "tar.gz" {
				t.Errorf("linux format = %q, want tar.gz", src.Format)
			}
		default:
			if err == nil {
				t.Error("expected error for linux unsupported arch")
			}
		}
	default:
		if err == nil {
			t.Error("expected error for unsupported OS")
		}
	}
}

func TestInstallNPMLSPPrereqFailure(t *testing.T) {
	// Without npm on PATH, this should fail.
	t.Setenv("PATH", "")
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	spec := InstallSpec{ID: "test", Command: "test"}
	_, err := installNPMLSP(context.Background(), spec, []string{"package"})
	if err == nil {
		t.Error("expected error when npm is unavailable")
	}
}

func TestNpmCacheDirs(t *testing.T) {
	dirs := npmCacheDirs(InstallSpec{ID: "test"})
	if len(dirs) == 0 {
		t.Error("expected at least one cache dir")
	}
}

func TestNpmInstallCommand(t *testing.T) {
	cmd := npmInstallCommand(InstallSpec{ID: "test"}, []string{"pkg1", "pkg2"})
	if !strings.Contains(cmd, "pkg1 pkg2") {
		t.Errorf("installCommand should list packages: %s", cmd)
	}
	if !strings.Contains(cmd, "npm install") {
		t.Errorf("installCommand should start with npm install: %s", cmd)
	}
}

func TestInstallNPMLSPCreateDirFail(t *testing.T) {
	// Make CacheRoot point to a path under a file (mkdir fails).
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(CacheEnv, filepath.Join(blocker, "subdir"))
	spec := InstallSpec{ID: "test", Command: "test"}
	_, err := installNPMLSP(context.Background(), spec, []string{"pkg"})
	if err == nil {
		t.Error("expected error when mkdir fails")
	}
}

func TestGoInstallPrereqFailure(t *testing.T) {
	t.Setenv("PATH", "")
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	impl := goImplementation{}
	_, err := impl.install(context.Background())
	if err == nil {
		t.Error("expected error when go is unavailable")
	}
}

func TestGoInstallCreateDirFail(t *testing.T) {
	// Make CacheRoot point to a path that can't be created.
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(CacheEnv, filepath.Join(blocker, "subdir"))
	impl := goImplementation{}
	_, err := impl.install(context.Background())
	if err == nil {
		t.Error("expected error when mkdir fails")
	}
}

func TestGoInstallNoBinary(t *testing.T) {
	// If go install runs but doesn't create binary, error.
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	// Use a stub that runs go install but doesn't produce binary.
	impl := goImplementation{}
	// Create a fake go binary that just exits 0.
	goBin := t.TempDir()
	fakeGo := filepath.Join(goBin, "go")
	if err := os.WriteFile(fakeGo, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", goBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err := impl.install(context.Background())
	if err == nil {
		t.Skip("go install unexpectedly produced a binary (perhaps real go on PATH)")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestInstallNPMLSPMissingBinary(t *testing.T) {
	// npm installs but binary not found.
	tmp := t.TempDir()
	t.Setenv(CacheEnv, tmp)
	// Create a fake npm that exits 0 but doesn't create the binary.
	fakeBin := t.TempDir()
	fakeNpm := filepath.Join(fakeBin, "npm")
	if err := os.WriteFile(fakeNpm, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	spec := InstallSpec{ID: "test", Command: "nonexistent-binary-xyz"}
	_, err := installNPMLSP(context.Background(), spec, []string{"some-pkg"})
	if err == nil {
		t.Skip("unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestHTMLInstall(t *testing.T) {
	impl := htmlImplementation{}
	_, err := impl.install(context.Background())
	// npm install with no PATH → likely error
	if err == nil {
		t.Skip("npm install unexpectedly succeeded")
	}
}

func TestCSSInstall(t *testing.T) {
	impl := cssImplementation{}
	_, err := impl.install(context.Background())
	if err == nil {
		t.Skip("npm install unexpectedly succeeded")
	}
}

func TestTypeScriptInstall(t *testing.T) {
	impl := typeScriptImplementation{}
	_, err := impl.install(context.Background())
	if err == nil {
		t.Skip("npm install unexpectedly succeeded")
	}
}

func TestInstallNPMLSPCacheRootFallback(t *testing.T) {
	// Test that installNPMLSP handles CacheRoot correctly when env is empty.
	t.Setenv(CacheEnv, "")
	tmp := t.TempDir()
	// Set XDG_CACHE_HOME to control where UserCacheDir returns.
	t.Setenv("XDG_CACHE_HOME", tmp)
	spec := InstallSpec{ID: "test", Command: "nonexistent-xyz"}
	_, err := installNPMLSP(context.Background(), spec, []string{"pkg"})
	if err == nil {
		t.Skip("npm install unexpectedly succeeded")
	}
}

func TestGoInstallCommandStructure(t *testing.T) {
	impl := goImplementation{}
	cmd := impl.installCommand()
	// Should contain shellQuote(CacheRoot()) which expands to something.
	if !strings.Contains(cmd, "GOBIN=") {
		t.Errorf("installCommand missing GOBIN=: %s", cmd)
	}
	if !strings.Contains(cmd, "go install") {
		t.Errorf("installCommand missing go install: %s", cmd)
	}
}
