package preview

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleProjectReturnsSummary(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "docs/example.md", "# Example\n\nBody\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/project", nil)
	server.handleProject(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleProject code = %d, want 200", rec.Code)
	}
	var summary projectSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if summary.Name == "" {
		t.Errorf("handleProject summary.Name is empty")
	}
}

func TestHandleProjectMethodNotAllowed(t *testing.T) {
	root := t.TempDir()
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/project", nil)
	server.handleProject(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("handleProject POST code = %d, want 405", rec.Code)
	}
}

func TestHandleProjectInvalidProjectRoot(t *testing.T) {
	server := newPreviewServer(previewOptions{projectRoot: "/nonexistent-root-x", docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/project", nil)
	server.handleProject(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("handleProject invalid code = %d, want 500", rec.Code)
	}
}

func TestHandleSpecsMethodNotAllowed(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Index\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/docs", nil)
	server.handleSpecs(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("handleSpecs PUT code = %d, want 405", rec.Code)
	}
}

func TestHandleSpecNotFound(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Index\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/docs/nonexistent.md", nil)
	server.handleSpec(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("handleSpec nonexistent code = %d, want 404", rec.Code)
	}
}

func TestHandleSpecInvalidPath(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Index\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	// Empty id (after trimming prefix) → 400.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/docs/", nil)
	server.handleSpec(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("handleSpec empty code = %d, want 400", rec.Code)
	}
}

func TestHandleSpecMethodNotAllowed(t *testing.T) {
	root := t.TempDir()
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/docs/x.md", nil)
	server.handleSpec(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("handleSpec POST code = %d, want 405", rec.Code)
	}
}

func TestHandleFileMethodNotAllowed(t *testing.T) {
	root := t.TempDir()
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/files", nil)
	server.handleFile(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("handleFile POST code = %d, want 405", rec.Code)
	}
}

func TestHandleFileInvalidPath(t *testing.T) {
	root := t.TempDir()
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	cases := []string{"", ".", "/abs/path", "../escape", "..", "foo/../../../escape"}
	for _, p := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/files?path="+url.QueryEscape(p), nil)
		server.handleFile(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("handleFile path %q code = %d, want 400", p, rec.Code)
		}
	}
}

func TestHandleFileNotFound(t *testing.T) {
	root := t.TempDir()
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=missing.txt", nil)
	server.handleFile(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("handleFile missing code = %d, want 404", rec.Code)
	}
}

func TestHandleFileReturnsPreviewable(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/example.md", "# Example\n\nHello docs.\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=docs/example.md", nil)
	server.handleFile(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleFile code = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp previewFileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.Contains(resp.Raw, "Hello docs.") {
		t.Errorf("handleFile body missing content: %q", resp.Raw)
	}
}

func TestHandleFileReturnsFolder(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/inner.md", "# Inner\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=docs", nil)
	server.handleFile(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleFile folder code = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp previewFolderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Type != "folder" || len(resp.Entries) == 0 {
		t.Errorf("handleFile folder response = %+v", resp)
	}
}

func TestHandleFileFolderSkipsDotfiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/.hidden", "secret\n")
	writeTestFile(t, root, "docs/visible.md", "# Visible\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=docs", nil)
	server.handleFile(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleFile folder code = %d", rec.Code)
	}
	var resp previewFolderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	for _, e := range resp.Entries {
		if strings.HasPrefix(e.Name, ".") {
			t.Errorf("handleFile folder should skip dotfiles, got %v", resp.Entries)
		}
	}
}

func TestHandleFileRejectsOversizeFile(t *testing.T) {
	root := t.TempDir()
	big := strings.Repeat("x", maxSearchFileBytes+1)
	writeTestFile(t, root, "docs/big.txt", big)

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=docs/big.txt", nil)
	server.handleFile(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("handleFile oversize code = %d, want 400", rec.Code)
	}
}

func TestHandleFileRejectsInvalidUTF8(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/binary.txt", "\xff\xfe\xfd\xfc\xfb")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=docs/binary.txt", nil)
	server.handleFile(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("handleFile invalid utf8 code = %d, want 400", rec.Code)
	}
}

func TestHandleFileRejectsNonPreviewableExtension(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "foo.bin", "binary") // Outside docs, so isPathInside returns false.

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=foo.bin", nil)
	server.handleFile(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("handleFile non-previewable code = %d, want 400", rec.Code)
	}
}

func TestHandleGraphMethodNotAllowed(t *testing.T) {
	root := t.TempDir()
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/graph", nil)
	server.handleGraph(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("handleGraph POST code = %d, want 405", rec.Code)
	}
}

func TestHandleGraphReturnsGraph(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", `# Spec Index

## Modules

| Module | Spec File | Status | Version | Compliance | Priority |
| ------ | --------- | ------ | ------- | ---------- | -------- |
| Overview | [overview.md](./overview.md) | Finalized | v1.0 | - | - |

## Dependency Graph

`+"```"+`
overview → editor.core
`+"```"+`
`)
	writeTestFile(t, root, "docs/overview.md", "# Overview\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/graph", nil)
	server.handleGraph(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("handleGraph code = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var graph specGraph
	if err := json.Unmarshal(rec.Body.Bytes(), &graph); err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) == 0 {
		t.Errorf("handleGraph expected nodes, got 0")
	}
}

func TestHandleEventsMethodNotAllowed(t *testing.T) {
	root := t.TempDir()
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/events", nil)
	server.handleEvents(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("handleEvents POST code = %d, want 405", rec.Code)
	}
}

func TestHandleEventsStreamsInitialEvent(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Index\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	server.handleEvents(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "event: ready") {
		t.Errorf("handleEvents should emit 'event: ready', got %q", body)
	}
}

func TestIsPathInsideVariants(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		path, base string
		want       bool
	}{
		{"", "", false},
		{root, "", false},
		{"", root, false},
		{root, root, true},
		{filepath.Join(root, "docs"), root, true},
	}
	for _, c := range cases {
		if got := isPathInside(c.path, c.base); got != c.want {
			t.Errorf("isPathInside(%q, %q) = %v, want %v", c.path, c.base, got, c.want)
		}
	}

	// A completely separate path must not be inside the root.
	outside := "/this/path/does/not/exist/at/all"
	if isPathInside(outside, root) {
		t.Errorf("isPathInside(%q, %q) returned true, want false", outside, root)
	}

	// Sibling temp directory (separately created, therefore outside).
	sibling := t.TempDir()
	if isPathInside(sibling, root) {
		t.Errorf("isPathInside(%q, %q) returned true, want false", sibling, root)
	}
}

func TestIsPathInsideEmptyP(t *testing.T) {
	if isPathInside("", "/foo") {
		t.Error("expected false for empty path")
	}
	if isPathInside("/foo", "") {
		t.Error("expected false for empty root")
	}
}

func TestHandleFileAllBranchesP(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A"), 0o644); err != nil {
		t.Fatal(err)
	}
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "b.md"), []byte("# B"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "c.txt"), []byte("plain"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "d.txt"), []byte("plain"), 0o644); err != nil {
		t.Fatal(err)
	}
	bigFile := filepath.Join(docsDir, "big.md")
	bigData := make([]byte, maxSearchFileBytes+1)
	for i := range bigData {
		bigData[i] = 'a'
	}
	if err := os.WriteFile(bigFile, bigData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "bad.md"), []byte{0xff, 0xfe, 0xfd}, 0o644); err != nil {
		t.Fatal(err)
	}

	ps := &previewServer{opt: previewOptions{projectRoot: dir, docsDir: "docs"}}

	cases := []struct {
		name      string
		path      string
		wantCode  int
	}{
		{"invalid_empty", "", 400},
		{"invalid_abs", "/abs/path", 400},
		{"invalid_dotdot", "../escape", 400},
		{"not_found", "missing.md", 404},
		{"valid_md_in_root", "a.md", 200},
		{"valid_md_in_docs", "docs/b.md", 200},
		{"txt_in_docs", "docs/c.txt", 200},
		{"txt_in_root", "d.txt", 400},
		{"too_big", "docs/big.md", 400},
		{"invalid_utf8", "docs/bad.md", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/file?path="+tc.path, nil)
			w := httptest.NewRecorder()
			ps.handleFile(w, r)
			if w.Code != tc.wantCode {
				t.Errorf("path=%q got code %d want %d body=%s", tc.path, w.Code, tc.wantCode, w.Body.String())
			}
		})
	}
}

func TestHandleFileDirectoryListingP(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, ".hidden"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ps := &previewServer{opt: previewOptions{projectRoot: dir, docsDir: "docs"}}
	r := httptest.NewRequest(http.MethodGet, "/api/file?path=sub", nil)
	w := httptest.NewRecorder()
	ps.handleFile(w, r)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "a.md") {
		t.Errorf("expected a.md in listing, got %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), ".hidden") {
		t.Errorf("hidden file should be excluded")
	}
}

func TestHandleFileMethodNotAllowedP(t *testing.T) {
	ps := &previewServer{opt: previewOptions{projectRoot: t.TempDir(), docsDir: "docs"}}
	r := httptest.NewRequest(http.MethodPost, "/api/file?path=x", nil)
	w := httptest.NewRecorder()
	ps.handleFile(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}


func TestHandleSpecsAllBranchesP(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "docs/example.md", "# Example\n\nBody\n")
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})

	// Wrong method.
	r := httptest.NewRequest(http.MethodPost, "/api/docs", nil)
	w := httptest.NewRecorder()
	server.handleSpecs(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}

	// Correct method.
	r = httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	w = httptest.NewRecorder()
	server.handleSpecs(w, r)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandleSpecAllBranchesP(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "docs/example.md", "# Example\n\nBody\n")
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})

	// Wrong method.
	r := httptest.NewRequest(http.MethodPost, "/api/docs/example", nil)
	w := httptest.NewRecorder()
	server.handleSpec(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}

	// Missing ID.
	r = httptest.NewRequest(http.MethodGet, "/api/docs/", nil)
	w = httptest.NewRecorder()
	server.handleSpec(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	// ID not found.
	r = httptest.NewRequest(http.MethodGet, "/api/docs/missing", nil)
	w = httptest.NewRecorder()
	server.handleSpec(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	// Valid ID.
	project, err := server.load()
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Documents) == 0 {
		t.Fatal("no documents loaded")
	}
	id := project.Documents[0].ID
	r = httptest.NewRequest(http.MethodGet, "/api/docs/"+id, nil)
	w = httptest.NewRecorder()
	server.handleSpec(w, r)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}


func TestHandleFileEscapesRootP(t *testing.T) {
	// Setup two temp dirs. Set projectRoot to one, request a path that joins
	// to escape it. Use a relative path with "../" to escape.
	dir := t.TempDir()
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "secret.md"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	ps := &previewServer{opt: previewOptions{projectRoot: dir, docsDir: "docs"}}
	rel, _ := filepath.Rel(dir, filepath.Join(other, "secret.md"))
	r := httptest.NewRequest(http.MethodGet, "/api/file?path="+rel, nil)
	w := httptest.NewRecorder()
	ps.handleFile(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for escaping path, got %d body=%s", w.Code, w.Body.String())
	}
}


func TestIsPathInside(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	if isPathInside("", root) {
		t.Error("empty path should not be inside")
	}
	if isPathInside(root, "") {
		t.Error("empty root should not match")
	}
	if !isPathInside(root, root) {
		t.Error("equal paths should be inside")
	}
	if !isPathInside(sub, root) {
		t.Error("sub should be inside root")
	}
	if isPathInside(root, sub) {
		t.Error("root should not be inside sub")
	}
	other := filepath.Join(dir, "other")
	if isPathInside(other, root) {
		t.Error("other should not be inside root")
	}
	sibling := filepath.Join(dir, "root2")
	if isPathInside(sibling, root) {
		t.Error("/root2 should not be inside /root")
	}
}

func TestHandleSpecsLoadError(t *testing.T) {
	// Point server at a project root that has no docs/ dir, forcing
	// scanSpecProject to return an error and exercise the writeAPIError branch.
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create docs as a file so scanSpecProject fails when reading it.
	if err := os.RemoveAll(docs); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/specs", nil)
	server.handleSpecs(rec, req)
	if rec.Code == http.StatusOK {
		t.Errorf("expected error status, got %d", rec.Code)
	}
}

func TestHandleSpecLoadError(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.WriteFile(docs, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/docs/foo", nil)
	server.handleSpec(rec, req)
	if rec.Code == http.StatusOK {
		t.Errorf("expected error status, got %d", rec.Code)
	}
}

func TestHandleFileLoadError(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.WriteFile(docs, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=docs/foo.md", nil)
	server.handleFile(rec, req)
	if rec.Code == http.StatusOK {
		t.Errorf("expected error status, got %d", rec.Code)
	}
}


func TestIsPathInsideAbsError(t *testing.T) {
	// Empty arguments and nil-ish inputs to cover guard branches.
	if isPathInside("", "") {
		t.Error("both empty should be false")
	}
	// Trigger filepath.Abs error by using a path with NUL byte (only on POSIX
	// systems; this is a no-op on Windows but still exercises the path).
	_ = isPathInside("\x00bad", "/tmp")
}

func TestHandleFileReturnsFolderListing(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "a.md"), []byte("# A"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, ".hidden"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=docs", nil)
	server.handleFile(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("handleFile folder code = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "a.md") {
		t.Errorf("expected listing to include a.md, got %s", body)
	}
	if strings.Contains(body, ".hidden") {
		t.Errorf("listing should skip dotfiles, got %s", body)
	}
}

func TestHandleFileReturns404ForMissing(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=docs/missing.md", nil)
	server.handleFile(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleFileReturns400ForTraversal(t *testing.T) {
	root := t.TempDir()
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=../etc/passwd", nil)
	server.handleFile(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleFileReturns400ForAbsolutePath(t *testing.T) {
	root := t.TempDir()
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=/etc/passwd", nil)
	server.handleFile(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleFileReturns400ForEmptyPath(t *testing.T) {
	root := t.TempDir()
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=", nil)
	server.handleFile(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleFileMethodNotAllowedExtra(t *testing.T) {
	root := t.TempDir()
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/files?path=docs/x.md", nil)
	server.handleFile(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleFileEscapesProjectRoot(t *testing.T) {
	root := t.TempDir()
	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	// Path that's "valid" syntactically but escapes the root via ../.
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=foo/../../etc/passwd", nil)
	server.handleFile(rec, req)
	// Should be rejected as invalid path.
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for path traversal, got %d", rec.Code)
	}
}

func TestHandleFileReadDirError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "a.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(docs, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(docs, 0o755) })

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	defer func() { _ = server.shutdown(context.Background()) }()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/files?path=docs", nil)
	server.handleFile(rec, req)
	// ReadDir fails → 500.
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for unreadable dir, got %d", rec.Code)
	}
}
