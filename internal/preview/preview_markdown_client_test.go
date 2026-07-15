package preview

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPreviewAPIDocReturnsRawMarkdownNotHTML asserts the Go contract the SPA
// relies on: document detail ships raw Markdown with empty html so the client
// must render Markdown (see renderDocumentBody in preview_ui_src/lib/markdown.ts).
func TestPreviewAPIDocReturnsRawMarkdownNotHTML(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/overview.md", `# Overview

## Meta

- **Description**: Client markdown contract.

Hello **docs**.
`)
	h := NewPreviewHandler(root, "docs", nil)
	mux := http.NewServeMux()
	h.Register(mux, "/api/")
	ts := httptest.NewServer(mux)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/docs/overview.md")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var doc specDocument
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if doc.HTML != "" {
		t.Fatalf("API should leave HTML empty for client render, got html=%q", doc.HTML)
	}
	if !strings.Contains(doc.Raw, "Hello **docs**.") {
		t.Fatalf("API should return raw Markdown, got raw=%q", doc.Raw)
	}
}

// TestPreviewClientMarkdownRender drives the shipped SPA helper
// (preview_ui_src/lib/markdown.ts → renderDocumentBody) via the repo node
// script. Empty html + raw markdown must become HTML (not left as source).
func TestPreviewClientMarkdownRender(t *testing.T) {
	moduleRoot, ok := previewModuleRoot(".")
	if !ok {
		// Walk from this test file's package dir.
		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		moduleRoot, ok = previewModuleRoot(wd)
		if !ok {
			t.Skip("module root not found; skip client markdown script")
		}
	}
	script := filepath.Join(moduleRoot, "scripts", "test-preview-markdown.mjs")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("missing client markdown test script: %v", err)
	}
	cmd := exec.Command("node", script)
	cmd.Dir = moduleRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("client markdown render test failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "PASS") {
		t.Fatalf("expected PASS from client markdown script, got:\n%s", out)
	}

	// Built SPA must embed the markdown path (marked + helper usage) after build.
	src, err := os.ReadFile(filepath.Join(moduleRoot, "internal", "preview", "preview_ui_src", "lib", "markdown.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "renderDocumentBody") || !strings.Contains(string(src), "marked") {
		t.Fatalf("markdown.ts missing client render implementation")
	}
	docsView, err := os.ReadFile(filepath.Join(moduleRoot, "internal", "preview", "preview_ui_src", "views", "Docs.tsx"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(docsView), "renderDocumentBody") {
		t.Fatalf("Docs.tsx must call renderDocumentBody for raw markdown")
	}
}
