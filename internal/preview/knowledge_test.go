package preview

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildKnowledgeProject creates a temporary project with one markdown doc so
// OpenKnowledge + the Knowledge façade have something to surface.
func buildKnowledgeProject(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	docs := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\n" +
		"title: Hello\n" +
		"type: module\n" +
		"tags: [a, b]\n" +
		"---\n" +
		"# Hello\nBody content."
	if err := os.WriteFile(filepath.Join(docs, "hello.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, func() { _ = os.RemoveAll(dir) }
}

func TestOpenKnowledgeAndFaçade(t *testing.T) {
	dir, cleanup := buildKnowledgeProject(t)
	defer cleanup()

	k, err := OpenKnowledge(dir, "docs")
	if err != nil {
		t.Fatalf("OpenKnowledge: %v", err)
	}
	if k.ProjectRoot() == "" {
		t.Error("ProjectRoot should not be empty")
	}
	if k.DocsRoot() == "" {
		t.Error("DocsRoot should not be empty")
	}
	if k.Name() == "" {
		t.Error("Name should not be empty (default)")
	}
	docs := k.Documents()
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if docs[0].Title != "Hello" {
		t.Errorf("unexpected title: %q", docs[0].Title)
	}
	doc, ok := k.Document("hello.md")
	if !ok {
		t.Error("Document lookup should succeed")
	}
	if doc.Title != "Hello" {
		t.Errorf("unexpected document title: %q", doc.Title)
	}
	// Missing doc.
	if _, ok := k.Document("missing.md"); ok {
		t.Error("Document lookup should fail for missing doc")
	}
	// Search runs the search pipeline.
	resp := k.Search(context.Background(), "hello", "docs", "AND", 10)
	// Just ensure it doesn't panic; result may or may not contain matches.
	if resp.Query != "hello" {
		t.Errorf("unexpected query in response: %q", resp.Query)
	}
}

func TestOpenKnowledgeMissingDocs(t *testing.T) {
	dir := t.TempDir()
	if _, err := OpenKnowledge(dir, "nope"); err == nil {
		t.Error("OpenKnowledge should error when docs dir is missing")
	}
}

func TestToKnowledgeDocumentCopiesAllFields(t *testing.T) {
	src := specDocument{
		ID:          "x",
		Title:       "T",
		Path:        "p",
		Language:    "go",
		Format:      "markdown",
		Category:    "c",
		Status:      "s",
		Version:     "v",
		Compliance:  "yes",
		Priority:    "high",
		Description: "d",
		Type:        "module",
		Tags:        []string{"a"},
		Timestamp:   "ts",
		Resource:    "r",
		Raw:         "raw",
	}
	got := toKnowledgeDocument(src)
	if got.ID != "x" || got.Title != "T" || got.Path != "p" || got.Language != "go" ||
		got.Format != "markdown" || got.Category != "c" || got.Status != "s" ||
		got.Version != "v" || got.Compliance != "yes" || got.Priority != "high" ||
		got.Description != "d" || got.Type != "module" || len(got.Tags) != 1 ||
		got.Timestamp != "ts" || got.Resource != "r" || got.Raw != "raw" {
		t.Errorf("toKnowledgeDocument did not copy all fields: %+v", got)
	}
}

func TestKnowledgeEmptyProject(t *testing.T) {
	// Empty docs dir should yield a knowledge base with zero documents but
	// valid metadata.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	k, err := OpenKnowledge(dir, "docs")
	if err != nil {
		t.Fatalf("OpenKnowledge: %v", err)
	}
	if got := k.Documents(); len(got) != 0 {
		t.Errorf("expected empty docs, got %d", len(got))
	}
}

func TestKnowledgeProjectRootNormalization(t *testing.T) {
	dir, cleanup := buildKnowledgeProject(t)
	defer cleanup()
	k, err := OpenKnowledge(dir, "docs")
	if err != nil {
		t.Fatalf("OpenKnowledge: %v", err)
	}
	// ProjectRoot should be normalized to an absolute path.
	if !filepath.IsAbs(k.ProjectRoot()) {
		t.Errorf("ProjectRoot should be absolute, got %q", k.ProjectRoot())
	}
	// DocsRoot should also be a non-empty path-like string.
	if !strings.Contains(k.DocsRoot(), "docs") {
		t.Errorf("DocsRoot should mention docs, got %q", k.DocsRoot())
	}
}