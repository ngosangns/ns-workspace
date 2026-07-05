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

func TestEnsureContentIndexCreatesFallback(t *testing.T) {
	dir := t.TempDir()
	if err := ensureContentIndex(dir); err != nil {
		t.Fatalf("ensureContentIndex failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatalf("fallback index.md not created: %v", err)
	}
	if string(data) == "" {
		t.Error("fallback index.md is empty")
	}
}

func TestEnsureContentIndexLeavesExistingHomepage(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"index.md", "_index.md"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("# Home\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if err := ensureContentIndex(dir); err != nil {
			t.Fatalf("ensureContentIndex failed for %s: %v", name, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(data) != "# Home\n" {
			t.Errorf("existing %s was overwritten", name)
		}
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove %s: %v", name, err)
		}
	}
}

func TestPrepareContentDirLinksEntries(t *testing.T) {
	docs := t.TempDir()
	if err := os.WriteFile(filepath.Join(docs, "a.md"), []byte("# A\n"), 0o644); err != nil {
		t.Fatalf("write a.md: %v", err)
	}
	if err := os.Mkdir(filepath.Join(docs, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docs, "sub", "b.md"), []byte("# B\n"), 0o644); err != nil {
		t.Fatalf("write sub/b.md: %v", err)
	}

	content := t.TempDir()
	if err := prepareContentDir(docs, content); err != nil {
		t.Fatalf("prepareContentDir failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(content, "a.md")); err != nil {
		t.Errorf("a.md not linked into content: %v", err)
	}
	if _, err := os.Stat(filepath.Join(content, "sub", "b.md")); err != nil {
		t.Errorf("sub/b.md not linked into content: %v", err)
	}
	if _, err := os.Stat(filepath.Join(content, "index.md")); err != nil {
		t.Errorf("fallback index.md not created: %v", err)
	}
}
