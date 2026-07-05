package preview

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveQuartzRepoUsesLocalDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	got, err := resolveQuartzRepo(dir)
	if err != nil {
		t.Fatalf("resolveQuartzRepo error: %v", err)
	}
	abs, _ := filepath.Abs(dir)
	if got != abs {
		t.Errorf("resolveQuartzRepo = %q, want %q", got, abs)
	}
}

func TestResolveQuartzRepoRejectsMissingPackageJSON(t *testing.T) {
	dir := t.TempDir()
	_, err := resolveQuartzRepo(dir)
	if err == nil {
		t.Fatal("resolveQuartzRepo expected error for missing package.json")
	}
}

func TestResolveQuartzRepoFallsBackToEnsure(t *testing.T) {
	origEnsure := ensureQuartzRepoForTest
	defer func() { ensureQuartzRepoForTest = origEnsure }()

	ensureCalled := false
	ensureQuartzRepoForTest = func() (string, error) {
		ensureCalled = true
		return "/cached/quartz", nil
	}

	got, err := resolveQuartzRepo("")
	if err != nil {
		t.Fatalf("resolveQuartzRepo error: %v", err)
	}
	if got != "/cached/quartz" {
		t.Errorf("resolveQuartzRepo = %q, want /cached/quartz", got)
	}
	if !ensureCalled {
		t.Error("ensureQuartzRepo was not called")
	}
}
