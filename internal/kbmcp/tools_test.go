package kbmcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ngosangns/ns-workspace/internal/preview"
)

// writeFixtureDocs lays down a small docs/ knowledge base under a fresh temp
// project root and returns (projectRoot, docsDir). Some docs carry OKF
// frontmatter (type/tags) so the type/tag filters and metadata round-trip can
// be exercised. A stray markdown file is written OUTSIDE the docs root to prove
// list_docs never reaches beyond it (Property 16).
func writeFixtureDocs(t *testing.T) (projectRoot, docsDir string) {
	t.Helper()
	projectRoot = t.TempDir()
	docsDir = "docs"
	root := filepath.Join(projectRoot, docsDir)

	files := map[string]string{
		"alpha.md": "---\n" +
			"type: module\n" +
			"description: Alpha module doc about preview rendering.\n" +
			"tags: [preview, docs]\n" +
			"---\n\n# Alpha\n\nAlpha covers preview rendering and search.\n",
		"beta.md": "---\n" +
			"type: feature\n" +
			"description: Beta feature doc about search.\n" +
			"tags: search\n" + // single string tag → must normalize to []string
			"---\n\n# Beta\n\nBeta is a search feature.\n",
		"nested/gamma.md": "---\n" +
			"type: reference\n" +
			"description: Gamma reference for the graph.\n" +
			"tags: [graph]\n" +
			"---\n\n# Gamma\n\nGamma documents the graph contract.\n",
		"plain.md": "# Plain\n\nPlain doc with no frontmatter at all.\n",
	}
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	// A file outside the docs root that must never surface in list_docs.
	if err := os.WriteFile(filepath.Join(projectRoot, "outside.md"), []byte("# Outside\n"), 0o644); err != nil {
		t.Fatalf("write outside.md: %v", err)
	}
	return projectRoot, docsDir
}

func mustListDocs(t *testing.T, s *Server, args any) listDocsResult {
	t.Helper()
	raw := mustArgs(t, args)
	got, err := s.handleListDocs(raw)
	if err != nil {
		t.Fatalf("handleListDocs error: %v", err)
	}
	res, ok := got.(listDocsResult)
	if !ok {
		t.Fatalf("handleListDocs returned %T, want listDocsResult", got)
	}
	return res
}

func mustArgs(t *testing.T, args any) json.RawMessage {
	t.Helper()
	if args == nil {
		return nil
	}
	b, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return b
}

func TestHandleListDocs_ReturnsOnlyDocsRoot(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)

	res := mustListDocs(t, s, listDocsArgs{})

	// Property 16: only docs scanned from the docs root are returned; the stray
	// outside.md (in the project root, not docs/) must not appear.
	wantIDs := map[string]bool{"alpha.md": true, "beta.md": true, "nested/gamma.md": true, "plain.md": true}
	if res.Count != len(wantIDs) {
		t.Fatalf("Count = %d, want %d (docs: %+v)", res.Count, len(wantIDs), res.Docs)
	}
	for _, d := range res.Docs {
		if !wantIDs[d.ID] {
			t.Errorf("unexpected doc id %q in list_docs result", d.ID)
		}
		if strings.Contains(d.ID, "..") || filepath.IsAbs(d.ID) {
			t.Errorf("doc id %q escapes docs root", d.ID)
		}
	}
}

func TestHandleListDocs_TypeFilter(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)

	res := mustListDocs(t, s, listDocsArgs{Type: "module"})
	if res.Count != 1 || res.Docs[0].ID != "alpha.md" {
		t.Fatalf("type=module filter = %+v, want only alpha.md", res.Docs)
	}

	// Case-insensitive match.
	res = mustListDocs(t, s, listDocsArgs{Type: "FEATURE"})
	if res.Count != 1 || res.Docs[0].ID != "beta.md" {
		t.Fatalf("type=FEATURE filter = %+v, want only beta.md", res.Docs)
	}

	// Unknown type → no matches, no error (permissive).
	res = mustListDocs(t, s, listDocsArgs{Type: "does-not-exist"})
	if res.Count != 0 {
		t.Fatalf("unknown type filter = %+v, want empty", res.Docs)
	}
}

func TestHandleListDocs_TagFilter(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)

	// "search" was declared as a single string tag and must normalize so the
	// tag filter still matches beta.md.
	res := mustListDocs(t, s, listDocsArgs{Tag: "search"})
	if res.Count != 1 || res.Docs[0].ID != "beta.md" {
		t.Fatalf("tag=search filter = %+v, want only beta.md", res.Docs)
	}

	res = mustListDocs(t, s, listDocsArgs{Tag: "preview"})
	if res.Count != 1 || res.Docs[0].ID != "alpha.md" {
		t.Fatalf("tag=preview filter = %+v, want only alpha.md", res.Docs)
	}
}

func TestHandleLookupDoc_Found(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)

	got, err := s.handleLookupDoc(mustArgs(t, lookupArgs{ID: "alpha.md"}))
	if err != nil {
		t.Fatalf("lookup_doc(alpha.md) error: %v", err)
	}
	doc, ok := got.(preview.KnowledgeDocument)
	if !ok {
		t.Fatalf("lookup_doc returned %T, want preview.KnowledgeDocument", got)
	}
	if doc.ID != "alpha.md" {
		t.Errorf("doc.ID = %q, want alpha.md", doc.ID)
	}
	if doc.Type != "module" {
		t.Errorf("doc.Type = %q, want module", doc.Type)
	}
	if !strings.Contains(doc.Raw, "Alpha covers preview rendering") {
		t.Errorf("doc.Raw missing body content: %q", doc.Raw)
	}
}

func TestHandleLookupDoc_NotFound(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)

	// Property 16: unknown id returns a clear error, never panics.
	_, err := s.handleLookupDoc(mustArgs(t, lookupArgs{ID: "nope.md"}))
	if err == nil {
		t.Fatal("lookup_doc(nope.md) expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want a 'not found' message", err)
	}
}

func TestHandleLookupDoc_EmptyID(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)

	if _, err := s.handleLookupDoc(mustArgs(t, lookupArgs{ID: "  "})); err == nil {
		t.Fatal("lookup_doc with blank id expected error, got nil")
	}
}

func TestHandleSearchDocs_SingleContract(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)
	ctx := context.Background()
	const query = "preview search"

	got, err := s.handleSearchDocs(ctx, mustArgs(t, searchArgs{Query: query}))
	if err != nil {
		t.Fatalf("search_docs error: %v", err)
	}

	// Property 17: search_docs must match the shared knowledge-core contract
	// (OpenKnowledge.Search → buildPreviewSearchResponse) for the same query and
	// the same defaults the handler uses.
	knowledge, err := preview.OpenKnowledge(projectRoot, docsDir)
	if err != nil {
		t.Fatalf("OpenKnowledge: %v", err)
	}
	want := knowledge.Search(ctx, query, defaultSearchMode, defaultSearchKeywordOperator, defaultSearchLimit)

	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		t.Fatalf("search_docs result does not match buildPreviewSearchResponse contract\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

func TestHandleSearchDocs_EmptyQuery(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)

	if _, err := s.handleSearchDocs(context.Background(), mustArgs(t, searchArgs{Query: "  "})); err == nil {
		t.Fatal("search_docs with blank query expected error, got nil")
	}
}

// TestHandlers_InvalidArgs verifies malformed JSON arguments return an error
// rather than crashing the handler (Req 7.6).
func TestHandlers_InvalidArgs(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)
	bad := json.RawMessage(`{"id": 123}`) // id should be a string

	if _, err := s.handleListDocs(json.RawMessage(`[`)); err == nil {
		t.Error("handleListDocs with malformed JSON expected error")
	}
	if _, err := s.handleLookupDoc(bad); err == nil {
		t.Error("handleLookupDoc with wrong-typed id expected error")
	}
	if _, err := s.handleSearchDocs(context.Background(), json.RawMessage(`{`)); err == nil {
		t.Error("handleSearchDocs with malformed JSON expected error")
	}
	if _, err := s.handleModifyDoc(json.RawMessage(`{`)); err == nil {
		t.Error("handleModifyDoc with malformed JSON expected error")
	}
}

// TestDispatch_UnknownMethodAndTool ensures the server returns JSON-RPC errors
// (and keeps serving) for unknown methods and unknown tool names (Req 7.6).
func TestDispatch_UnknownMethodAndTool(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)

	resp := s.route(context.Background(), rpcRequest{Method: "no/such/method", ID: json.RawMessage(`1`)})
	if resp.Error == nil || resp.Error.Code != codeMethodNotFound {
		t.Fatalf("unknown method: got %+v, want method-not-found error", resp.Error)
	}

	params, _ := json.Marshal(toolCallParams{Name: "no_such_tool"})
	resp = s.handleToolCall(context.Background(), rpcRequest{Method: "tools/call", ID: json.RawMessage(`2`), Params: params})
	if resp.Error == nil || resp.Error.Code != codeInvalidParams {
		t.Fatalf("unknown tool: got %+v, want invalid-params error", resp.Error)
	}
}

// --- resolveDocPath traversal cases (Property 15, Req 8.2) ---

func TestResolveDocPath_RejectsTraversal(t *testing.T) {
	root := filepath.Clean(t.TempDir())

	escaping := []string{
		"../escape.md",
		"../../etc/passwd",
		"nested/../../escape.md",
		"a/b/../../../escape.md",
		"",
		"   ",
		"..",
		filepath.Join(root, "..", "escape.md"), // absolute escaping path
	}
	if os.PathSeparator == '/' {
		escaping = append(escaping, "/etc/passwd", "/tmp/escape.md")
	}

	for _, id := range escaping {
		if p, err := resolveDocPath(root, id); err == nil {
			t.Errorf("resolveDocPath(%q) = %q, want error", id, p)
		}
	}
}

func TestResolveDocPath_AcceptsInside(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	sep := string(os.PathSeparator)

	valid := []string{"alpha.md", "nested/gamma.md", "a/b/c/deep.md", "./relative.md"}
	for _, id := range valid {
		p, err := resolveDocPath(root, id)
		if err != nil {
			t.Errorf("resolveDocPath(%q) unexpected error: %v", id, err)
			continue
		}
		if !strings.HasPrefix(p, root+sep) {
			t.Errorf("resolveDocPath(%q) = %q, not inside root %q", id, p, root)
		}
	}
}

// TestHandleModifyDoc_WriteAndReject exercises the modify_doc happy path
// (writes inside docs root, creates parent dirs) and the traversal rejection.
func TestHandleModifyDoc_WriteAndReject(t *testing.T) {
	projectRoot, docsDir := writeFixtureDocs(t)
	s := NewServer(projectRoot, docsDir)

	got, err := s.handleModifyDoc(mustArgs(t, modifyArgs{ID: "references/new.md", Content: "# New\n"}))
	if err != nil {
		t.Fatalf("modify_doc error: %v", err)
	}
	res, ok := got.(modifyDocResult)
	if !ok || !res.OK || res.Path != "references/new.md" {
		t.Fatalf("modify_doc result = %+v, want {ok:true, path:references/new.md}", got)
	}
	written := filepath.Join(projectRoot, docsDir, "references", "new.md")
	if _, statErr := os.Stat(written); statErr != nil {
		t.Fatalf("expected file written at %s: %v", written, statErr)
	}

	// Path traversal must be rejected and must not write outside docs root.
	if _, err := s.handleModifyDoc(mustArgs(t, modifyArgs{ID: "../escaped.md", Content: "x"})); err == nil {
		t.Fatal("modify_doc with traversal id expected error, got nil")
	}
	if _, statErr := os.Stat(filepath.Join(projectRoot, "escaped.md")); statErr == nil {
		t.Fatal("traversal write escaped the docs root")
	}
}
