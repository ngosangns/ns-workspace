package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPreviewHTTPHandlers(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "AGENTS.md", "# Agents\n")
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
	writeTestFile(t, root, "docs/overview.md", `# Overview

## Meta

- **Description**: Overview metadata description.

Hello **docs**.
`)
	writeTestFile(t, root, "docs/reference/settings.custom", "feature_flag: preview_index_all_docs_files\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/docs")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var docs []specDocument
	if err := json.NewDecoder(res.Body).Decode(&docs); err != nil {
		t.Fatal(err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected _index, overview, and custom docs file, got %d", len(docs))
	}
	var custom *specDocument
	for i := range docs {
		if docs[i].ID == "reference/settings.custom" {
			custom = &docs[i]
			break
		}
	}
	if custom == nil || custom.Language != "plaintext" {
		t.Fatalf("expected non-Markdown docs file in docs list with plaintext language, got %+v", custom)
	}

	res, err = http.Get(ts.URL + "/api/docs/overview.md")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var doc specDocument
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.Raw, "Hello **docs**.") || doc.HTML != "" {
		t.Fatalf("doc endpoint should return raw Markdown for client-side rendering: raw=%q html=%q", doc.Raw, doc.HTML)
	}
	if doc.Description != "Overview metadata description." {
		t.Fatalf("doc endpoint should preserve document metadata description: %+v", doc)
	}

	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/docs/overview.md", strings.NewReader(`{"raw":"# Updated Overview\n\nSaved from preview.\n"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("doc endpoint should be read-only, got %s", res.Status)
	}
	if data, err := os.ReadFile(filepath.Join(root, "docs", "overview.md")); err != nil || strings.Contains(string(data), "Saved from preview.") {
		t.Fatalf("read-only doc endpoint should not persist content: %q, %v", string(data), err)
	}

	res, err = http.Get(ts.URL + "/api/docs/reference%2Fsettings.custom")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var customDoc specDocument
	if err := json.NewDecoder(res.Body).Decode(&customDoc); err != nil {
		t.Fatal(err)
	}
	if customDoc.ID != "reference/settings.custom" || customDoc.Language != "plaintext" || !strings.Contains(customDoc.Raw, "preview_index_all_docs_files") {
		t.Fatalf("nested docs file endpoint returned wrong content: %+v", customDoc)
	}

	req, err = http.NewRequest(http.MethodPut, ts.URL+"/api/docs/reference%2Fsettings.custom", strings.NewReader(`{"raw":"feature_flag: edited\n"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("docs file endpoint should be read-only, got %s", res.Status)
	}

	res, err = http.Get(ts.URL + "/api/files?path=docs/overview.md")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var file previewFileResponse
	if err := json.NewDecoder(res.Body).Decode(&file); err != nil {
		t.Fatal(err)
	}
	if file.Path != "docs/overview.md" || file.Language != "markdown" || !strings.Contains(file.Raw, "Hello **docs**.") {
		t.Fatalf("file endpoint should return previewable UTF-8 file content: %+v", file)
	}

	res, err = http.Get(ts.URL + "/api/graph")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var graph specGraph
	if err := json.NewDecoder(res.Body).Decode(&graph); err != nil {
		t.Fatal(err)
	}
	if !hasEdge(graph.Edges, "overview", "editor.core") {
		t.Fatalf("graph endpoint did not expose edge: %+v", graph.Edges)
	}

	res, err = http.Get(ts.URL + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if got := res.Header.Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("events endpoint did not use SSE content type: %s", got)
	}

	res, err = http.Get(ts.URL + "/spec/modules/example.md")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("preview app fallback failed: %s", res.Status)
	}
	if got := res.Header.Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("preview app fallback should return HTML, got %s", got)
	}

	res, err = http.Get(ts.URL + "/js/graph.js")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("preview graph module was not served: %s", res.Status)
	}
}

func TestPreviewSearchReturnsFourPanelsAcrossDocsAndCode(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", `# Spec Index

## Dependency Graph

`+"```"+`
auth → session
`+"```"+`
`)
	writeTestFile(t, root, "docs/auth.md", `# Auth

## Meta

- **Description**: Describes auth metadata for search result cards.

Authentication validates session tokens.
`)
	writeTestFile(t, root, "docs/session.md", "# Session\n\nSession token lifecycle.\n")
	writeTestFile(t, root, "auth.go", `package demo

func parseAuthToken(raw string) string {
	return raw
}
`)
	writeTestFile(t, root, "graphify-out/graph.json", `{
  "nodes": [
    {"id":"code_parse_auth_token","label":"parseAuthToken()","file_type":"code","source_file":"`+filepath.ToSlash(filepath.Join(root, "auth.go"))+`","source_location":"L3","community":1},
    {"id":"doc_auth","label":"Auth","file_type":"doc","source_file":"`+filepath.ToSlash(filepath.Join(root, "docs/auth.md"))+`","source_location":"L1","community":2}
  ],
  "links": [
    {"source":"code_parse_auth_token","target":"doc_auth","relation":"references","confidence":"EXTRACTED"}
  ]
}`)

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/search?q=auth")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var search previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&search); err != nil {
		t.Fatal(err)
	}
	if search.Mode != "hybrid" {
		t.Fatalf("expected hybrid mode, got %s", search.Mode)
	}
	if len(search.Panels.DocsSemantic) == 0 || len(search.Panels.DocsGraph) == 0 || len(search.Panels.CodeSemantic) == 0 || len(search.Panels.CodeGraph) == 0 {
		t.Fatalf("expected all four search panels to have results: %+v", search.Panels)
	}
	if search.Panels.DocsSemantic[0].SpecID == "" {
		t.Fatalf("docs semantic result should be openable as a doc: %+v", search.Panels.DocsSemantic[0])
	}
	if search.Panels.DocsSemantic[0].Description != "Describes auth metadata for search result cards." {
		t.Fatalf("docs semantic result should expose metadata description: %+v", search.Panels.DocsSemantic[0])
	}
	if search.Panels.CodeGraph[0].Line != 3 || len(search.Panels.CodeGraph[0].Neighbors) == 0 {
		t.Fatalf("code graph should expose source line and neighbors: %+v", search.Panels.CodeGraph[0])
	}
	if search.Panels.CodeGraph[0].Neighbors[0].Path != "docs/auth.md" || search.Panels.CodeGraph[0].Neighbors[0].Line != 1 {
		t.Fatalf("code graph neighbors should expose their own preview targets: %+v", search.Panels.CodeGraph[0].Neighbors[0])
	}

	res, err = http.Get(ts.URL + "/api/search?q=auth,session")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var commaSearch previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&commaSearch); err != nil {
		t.Fatal(err)
	}
	if len(commaSearch.Panels.DocsSemantic) < 2 {
		t.Fatalf("comma-separated keywords should match multiple document terms: %+v", commaSearch.Panels.DocsSemantic)
	}
}

func TestPreviewSearchKeywordDifferenceExcludesLaterKeywords(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "docs/alpha.md", "# Alpha\n\nAlpha only.\n")
	writeTestFile(t, root, "docs/beta.md", "# Beta\n\nAlpha beta overlap.\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/search?q=alpha,beta&keywordOp=difference")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var search previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&search); err != nil {
		t.Fatal(err)
	}
	if search.KeywordOperator != "difference" {
		t.Fatalf("expected difference keyword operator, got %s", search.KeywordOperator)
	}
	if len(search.Panels.DocsSemantic) == 0 {
		t.Fatalf("expected alpha result after keyword difference")
	}
	for _, result := range search.Panels.DocsSemantic {
		if strings.Contains(result.Path, "beta.md") {
			t.Fatalf("difference search should exclude later keyword matches: %+v", search.Panels.DocsSemantic)
		}
	}
}

func TestPreviewSearchScansAllTextFilesUnderDocs(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "docs/specs/auth.md", "# Auth\n\nAuthentication validates session tokens.\n")
	writeTestFile(t, root, "docs/reference/settings.custom", "feature_flag: preview_index_all_docs_files\n")
	writeTestFile(t, root, "main.go", "package main\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/search?q=preview_index_all_docs_files")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var search previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&search); err != nil {
		t.Fatal(err)
	}
	if len(search.Panels.DocsSemantic) == 0 {
		t.Fatalf("expected docs semantic search to include non-Markdown docs files: %+v", search.Panels)
	}
	got := search.Panels.DocsSemantic[0]
	if got.Path != "reference/settings.custom" || got.SpecID != "reference/settings.custom" || got.Kind != "doc" {
		t.Fatalf("expected docs-relative document result, got %+v", got)
	}
	if len(search.Panels.CodeSemantic) != 0 {
		t.Fatalf("docs files should not be duplicated in code semantic results: %+v", search.Panels.CodeSemantic)
	}

	res, err = http.Get(ts.URL + "/api/files?path=docs/reference/settings.custom")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("docs text file should be previewable: %s", res.Status)
	}
	var file previewFileResponse
	if err := json.NewDecoder(res.Body).Decode(&file); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(file.Raw, "preview_index_all_docs_files") {
		t.Fatalf("docs text file preview returned wrong content: %+v", file)
	}
}

func TestPreviewSearchExpandsDocsGraphFromSemanticResults(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", `# Spec Index

## Dependency Graph

`+"```"+`
guide → policy
`+"```"+`
`)
	writeTestFile(t, root, "docs/guide.md", "# Guide\n\nHandles session credential lookup.\n")
	writeTestFile(t, root, "docs/policy.md", "# Policy\n\nAccess policy details.\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/search?q=session%20credential")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var search previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&search); err != nil {
		t.Fatal(err)
	}
	if len(search.Panels.DocsSemantic) == 0 || search.Panels.DocsSemantic[0].SpecID != "guide.md" {
		t.Fatalf("expected semantic doc anchor for guide.md, got %+v", search.Panels.DocsSemantic)
	}
	if len(search.Panels.DocsGraph) == 0 {
		t.Fatalf("expected docs graph expansion from semantic result: %+v", search.Panels)
	}
	if !containsString(search.Panels.DocsGraph[0].MatchedBy, "semantic-anchor") {
		t.Fatalf("expected docs graph anchor to be marked semantic-anchor, got %+v", search.Panels.DocsGraph[0])
	}
	if len(search.Panels.DocsGraph[0].Neighbors) == 0 || search.Panels.DocsGraph[0].Neighbors[0].ID != "policy" {
		t.Fatalf("expected docs graph to expand to policy neighbor, got %+v", search.Panels.DocsGraph[0].Neighbors)
	}
}

func TestPreviewSearchExpandsCodeGraphFromSemanticResults(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "auth.go", `package demo

func parseToken(raw string) string {
	// Secure secret material is normalized before storage.
	return raw
}
`)
	writeTestFile(t, root, "store.go", `package demo

func hydrateStore() {}
`)
	writeTestFile(t, root, "graphify-out/graph.json", `{
  "nodes": [
    {"id":"code_lookup","label":"parseToken()","file_type":"code","source_file":"`+filepath.ToSlash(filepath.Join(root, "auth.go"))+`","source_location":"L3","community":1},
    {"id":"code_store","label":"hydrateStore()","file_type":"code","source_file":"`+filepath.ToSlash(filepath.Join(root, "store.go"))+`","source_location":"L3","community":1}
  ],
  "links": [
    {"source":"code_lookup","target":"code_store","relation":"calls","confidence":"EXTRACTED"}
  ]
}`)

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/search?q=secure%20secret")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var search previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&search); err != nil {
		t.Fatal(err)
	}
	if len(search.Panels.CodeSemantic) == 0 || search.Panels.CodeSemantic[0].Path != "auth.go" {
		t.Fatalf("expected semantic code anchor for auth.go, got %+v", search.Panels.CodeSemantic)
	}
	if len(search.Panels.CodeGraph) == 0 {
		t.Fatalf("expected code graph expansion from semantic result: %+v", search.Panels)
	}
	if !containsString(search.Panels.CodeGraph[0].MatchedBy, "semantic-anchor") || search.Panels.CodeGraph[0].Path != "auth.go" {
		t.Fatalf("expected code graph anchor to be marked semantic-anchor, got %+v", search.Panels.CodeGraph[0])
	}
	if len(search.Panels.CodeGraph[0].Neighbors) == 0 || search.Panels.CodeGraph[0].Neighbors[0].Path != "store.go" {
		t.Fatalf("expected code graph to expand to store.go neighbor, got %+v", search.Panels.CodeGraph[0].Neighbors)
	}
}

func TestPreviewSearchWorksWithoutDocsDirectory(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "auth.go", `package demo

func parseAuthToken(raw string) string {
	return raw
}
`)

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/search?q=parseAuthToken")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("search without docs should not fail: %s", res.Status)
	}
	var search previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&search); err != nil {
		t.Fatal(err)
	}
	if len(search.Panels.CodeSemantic) == 0 {
		t.Fatalf("expected code semantic results without docs folder: %+v", search)
	}
	if len(search.Panels.DocsSemantic) != 0 || len(search.Panels.DocsGraph) != 0 {
		t.Fatalf("docs panels should be empty without docs folder: %+v", search.Panels)
	}
	if len(search.Warnings) == 0 || !strings.Contains(search.Warnings[0], "Docs directory is unavailable") {
		t.Fatalf("expected missing docs warning, got %+v", search.Warnings)
	}
}

func TestPreviewSearchHidesEmbeddingFallbackWarning(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "docs/auth.md", "# Auth\n\nAuthentication validates session tokens.\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/search?q=auth")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var search previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&search); err != nil {
		t.Fatal(err)
	}
	for _, warning := range search.Warnings {
		if strings.Contains(warning, "Embedding search is not configured") {
			t.Fatalf("embedding fallback should not be exposed as a search warning: %+v", search.Warnings)
		}
	}
}

func TestPreviewSearchUsesKnownsEmbeddingSettings(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "docs/auth.md", "# Auth\n\nAuthentication validates session tokens.\n")
	writeTestFile(t, root, "docs/billing.md", "# Billing\n\nInvoices and payment records.\n")
	writeTestFile(t, root, "auth.go", `package demo

func parseAuthToken(raw string) string {
	return raw
}
`)
	embedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		type datum struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		}
		res := struct {
			Data []datum `json:"data"`
		}{}
		for i, input := range req.Input {
			lower := strings.ToLower(input)
			vec := []float32{0, 1, 0}
			if strings.Contains(lower, "auth") || strings.Contains(lower, "session") || strings.Contains(lower, "token") {
				vec = []float32{1, 0, 0}
			}
			res.Data = append(res.Data, datum{Index: i, Embedding: vec})
		}
		_ = json.NewEncoder(w).Encode(res)
	}))
	defer embedServer.Close()
	writeTestFile(t, home, ".knowns/settings.json", fmt.Sprintf(`{
  "embeddingProviders": {
    "preview-test": {
      "apiBase": %q,
      "batchSize": 2,
      "timeout": 5
    }
  },
  "embeddingModels": {
    "multilingual-e5-small": {
      "provider": "preview-test",
      "model": "multilingual-e5-small",
      "dimensions": 3
    }
  },
  "defaultEmbeddingModel": "multilingual-e5-small"
}`, embedServer.URL+"/v1"))

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/search?q=session%20credential")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var search previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&search); err != nil {
		t.Fatal(err)
	}
	if len(search.Panels.DocsSemantic) == 0 || search.Panels.DocsSemantic[0].Path != "auth.md" {
		t.Fatalf("expected embedding semantic doc result for auth.md, got %+v", search.Panels.DocsSemantic)
	}
	if !containsString(search.Panels.DocsSemantic[0].MatchedBy, "semantic") {
		t.Fatalf("expected semantic match method, got %+v", search.Panels.DocsSemantic[0].MatchedBy)
	}
	for _, warning := range search.Warnings {
		if strings.Contains(warning, "lexical fallback") {
			t.Fatalf("embedding-configured search should not use lexical fallback warning: %+v", search.Warnings)
		}
	}
}

func TestPreviewHelpIsAccepted(t *testing.T) {
	if err := Run([]string{"--help"}); err != nil {
		t.Fatalf("preview help failed: %v", err)
	}
}

func TestPreviewChildArgsPickAutoPortOnce(t *testing.T) {
	args, err := previewChildArgs([]string{"--project", "."})
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(args, "--addr") {
		t.Fatalf("preview child args should include an auto-picked address: %+v", args)
	}
	addr := args[len(args)-1]
	if strings.HasSuffix(addr, ":0") {
		t.Fatalf("preview child args should pin the selected port instead of passing :0: %+v", args)
	}
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Fatalf("preview child args should use loopback address, got %q", addr)
	}
}

func TestPreviewChildArgsPreserveExplicitAddr(t *testing.T) {
	args, err := previewChildArgs([]string{"--project", ".", "--addr", "127.0.0.1:9999"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(args, " "); got != "--project . --addr 127.0.0.1:9999" {
		t.Fatalf("preview child args should preserve explicit addr, got %q", got)
	}
}

func TestPreviewSourceHotReloadTokenTracksBackendAndFrontend(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/preview\n")
	writeTestFile(t, root, "main.go", "package main\n")
	writeTestFile(t, root, "internal/preview/preview.go", "package preview\n")
	writeTestFile(t, root, "internal/preview/preview_ui/app.js", "console.log('one')\n")
	writeTestFile(t, root, "docs/guide.md", "# guide\n")

	nested := filepath.Join(root, "internal", "preview")
	if got, ok := previewModuleRoot(nested); !ok || got != root {
		t.Fatalf("previewModuleRoot(%q) = %q, %v; want %q, true", nested, got, ok, root)
	}
	initial := previewSourceToken(root)
	time.Sleep(time.Millisecond)
	writeTestFile(t, root, "internal/preview/preview_ui/app.js", "console.log('two')\n")
	if next := previewSourceToken(root); next == initial {
		t.Fatalf("frontend source change should update hot reload token")
	}
	codeToken := previewSourceToken(root)
	time.Sleep(time.Millisecond)
	writeTestFile(t, root, "internal/preview/preview.go", "package preview\nconst changed = true\n")
	if next := previewSourceToken(root); next == codeToken {
		t.Fatalf("backend source change should update hot reload token")
	}
	docToken := previewSourceToken(root)
	time.Sleep(time.Millisecond)
	writeTestFile(t, root, "docs/guide.md", "# changed\n")
	if next := previewSourceToken(root); next != docToken {
		t.Fatalf("docs changes should be handled by data hot reload, not source restart: %q != %q", next, docToken)
	}
}

func TestPreviewUIUsesProjectSummaryResponse(t *testing.T) {
	data, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "project.summary") {
		t.Fatalf("preview UI should use /api/project summary response directly")
	}
}

func TestPreviewUIHasTypeScriptToolchain(t *testing.T) {
	for _, path := range []string{
		"../../package.json",
		"../../package-lock.json",
		"../../tsconfig.preview.json",
		"../../eslint.config.mjs",
		"../../.prettierrc.json",
		"preview_ui_src/app.ts",
		"preview_ui_src/js/graph.ts",
		"preview_ui_src/types.d.ts",
		"preview_ui/app.js",
		"preview_ui/js/graph.js",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("preview TypeScript toolchain missing %s: %v", path, err)
		}
	}
	pkg, err := os.ReadFile("../../package.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"build:preview", "check:preview", "lint:preview", "format:preview", "format:preview:check", "typescript", "eslint", "prettier"} {
		if !strings.Contains(string(pkg), want) {
			t.Fatalf("preview package scripts/deps missing %s", want)
		}
	}
}

func TestPreviewUIUsesDedicatedFrontendLibraries(t *testing.T) {
	html, err := os.ReadFile("preview_ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile("preview_ui/app.js")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	htmlText := string(html)
	appText := string(app) + "\n" + string(css)
	for _, want := range []string{"cdn.tailwindcss.com", "daisyui", "lucide", "@toast-ui/editor", "toastui-editor-viewer", "DOMPurify", "highlight.js", "languages/go.min.js", "languages/typescript.min.js", "hljs.highlight", "mermaid.min.js", "mermaid.render", "svg-pan-zoom", "sigma@3.0.3", "graphology@0.26.0", "graphology-layout-forceatlas2@0.10.1"} {
		if !strings.Contains(htmlText, want) && !strings.Contains(appText, want) {
			t.Fatalf("preview UI missing %s integration", want)
		}
	}
	if strings.Contains(htmlText, "/api/render/mermaid") || strings.Contains(appText, "/api/render/mermaid") {
		t.Fatalf("preview UI should render Mermaid client-side")
	}
	for _, forbidden := range []string{"data-ui-kit=\"treact\"", "Treact-style component primitives", "cytoscape"} {
		if strings.Contains(htmlText, forbidden) || strings.Contains(appText, forbidden) {
			t.Fatalf("preview UI should not include %s", forbidden)
		}
	}
}

func TestPreviewUIRendersDocsGraphWithSigma(t *testing.T) {
	html, err := os.ReadFile("preview_ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	graphJS, err := os.ReadFile("preview_ui_src/js/graph.ts")
	if err != nil {
		t.Fatal(err)
	}
	networkGraphJS, err := os.ReadFile("preview_ui_src/js/network_graph.ts")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(app) + "\n" + string(graphJS) + "\n" + string(networkGraphJS) + "\n" + string(css)
	for _, want := range []string{"data-tab=\"graph\"", "type=\"module\" src=\"/app.js\"", "id=\"graphCanvas\"", "fetchJSON(\"/api/graph\")", "createDocsGraph", "renderNetworkGraph", "Sigma", "forceAtlas2", "clickNode", "clickStage", "enterNode", "leaveNode", "forceLabel: true", "labelRenderedSizeThreshold: 0", "normalizedGraphData", "renderedGraph", "graphSelectedId", "graphRenderer", "graph-details", "openSpecPreview", "openFilePreview", "data-preview-spec", "data-preview-file", "openGraphNode", "clearGraphSelection", ".is-node-hover canvas"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview docs graph UI missing %s", want)
		}
	}
	if strings.Contains(string(graphJS), "selectSpec(") || strings.Contains(string(graphJS), "data-open-spec") {
		t.Fatalf("preview docs graph should use popup previews instead of direct doc navigation")
	}
	if strings.Contains(string(graphJS), "state.graphSelectedId = node.id;\n    const incoming") {
		t.Fatalf("preview docs graph should not focus the first node while rendering details")
	}
	if strings.Contains(string(networkGraphJS), ": nodes[0]?.id") {
		t.Fatalf("preview graph renderer should not default-focus the first node")
	}
	if strings.Contains(string(networkGraphJS), `label: dimmed ? "" : data.label`) || strings.Contains(string(networkGraphJS), "size: node === selectedId") {
		t.Fatalf("focused preview graph should dim nodes without hiding labels or resizing nodes")
	}
	if !strings.Contains(string(networkGraphJS), "labelColor: dimmed ? colorWithOpacity(data.labelColor, 0.22) : data.labelColor") {
		t.Fatalf("focused preview graph should dim unfocused node labels")
	}
	if strings.Contains(string(networkGraphJS), "softenColor") || !strings.Contains(string(networkGraphJS), "return color;") {
		t.Fatalf("focused preview graph should dim by opacity only, without changing original colors")
	}
}

func TestPreviewTopbarUsesIconOnlyTabs(t *testing.T) {
	html, err := os.ReadFile("preview_ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	htmlText := string(html)
	text := htmlText + "\n" + string(app)
	for _, want := range []string{
		`aria-label="Preview sections"`,
		`id="projectPath"`,
		`data-tab="graph"`,
		`data-lucide="git-fork"`,
		`data-tab="search"`,
		`data-lucide="search"`,
		"project.projectRoot",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview topbar icon-only tabs missing %s", want)
		}
	}
	for _, forbidden := range []string{`data-tab="overview"`, `data-lucide="layout-dashboard"`, `data-tab="spec"`, `data-lucide="file-text"`, "overviewTab", ">Overview</button>", ">Graph</button>", ">Search</button>", ">Doc</button>", "id=\"themeLabel\""} {
		if strings.Contains(htmlText, forbidden) {
			t.Fatalf("preview topbar should not render text label %s", forbidden)
		}
	}
}

func TestPreviewUIRendersFourPanelSearchPage(t *testing.T) {
	html, err := os.ReadFile("preview_ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(app) + "\n" + string(css)
	for _, want := range []string{
		`data-tab="search"`,
		`id="searchTab"`,
		`id="docsSemanticResults"`,
		`id="docsGraphResults"`,
		`id="codeSemanticResults"`,
		`id="codeGraphResults"`,
		`id="codeGraphReload"`,
		`id="searchKeywordOperator"`,
		`keywordOp`,
		`currentSearchKeywordOperator`,
		"fetch(`/api/search?${params.toString()}",
		`renderSearchPanel("docsSemantic"`,
		`renderSearchPanel("codeGraph"`,
		`renderSearchResult(result, name)`,
		"result.description || result.excerpt",
		`!result.specId`,
		"reloadCodeGraph",
		"updateCodeGraphReloadControl",
		"codeGraphLoading",
		"els.codeGraphReload?.addEventListener",
		"renderSearchGraphPanel",
		"searchResultsToGraph",
		"renderSearchResultGraph",
		"renderNetworkGraph",
		"codeGraphNodeLabel",
		"neighborPath",
		"neighborLine",
		"previewPath",
		"searchGraphRenderers",
		"renderSearchGraphDetails(name, graph, details)",
		"const selectedNode = graph.nodes.find((node) => node.id === selected);",
		"clearSearchGraphSelection",
		".search-graph-canvas",
		"searchLoading",
		"renderSearchLoading",
		"Searching docs, code, and graphs",
		`id="previewDialog"`,
		"openSpecPreview",
		"openFilePreview",
		"/api/files?",
		"highlightRenderedCode",
		"renderSpecDocumentContent",
		"renderCurrentSpecContent",
		`id="previewRawToggle"`,
		"previewSource",
		"previewShowRaw",
		"updatePreviewRawToggle",
		"renderPreviewSource",
		"selectionContextMenu",
		"selectionCopyButton",
		"updateSelectionContextMenu",
		"resolveSelectionCopyTarget",
		"navigator.clipboard.writeText",
		"data-source-line-start",
		"Copy filepath and line index",
		`data-preview-spec`,
		`data-preview-file`,
		"route.searchQuery",
		"buildRouteQuery",
		"updateSearchRouteURL",
		`params.set("preview", state.previewRoute.type)`,
		`params.set("path", state.previewRoute.path)`,
		"applyPreviewRoute",
		"decorateCodePreviewLines",
		"code-line-target",
		".preview-modal",
		".code-preview",
		"line-height: 1.18",
		".search-loading",
		`.search-grid`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview search UI missing %s", want)
		}
	}
	if strings.Contains(text, "state.searchGraphSelections.set(name, selectedNode.id)") {
		t.Fatalf("preview search graph should not default-focus the first rendered node")
	}
}

func TestPreviewUIHasFaviconAndRouteTitles(t *testing.T) {
	html, err := os.ReadFile("preview_ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	icon, err := os.ReadFile("preview_ui/favicon.svg")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(app) + "\n" + string(icon)
	for _, want := range []string{
		`href="/favicon.svg"`,
		`type="image/svg+xml"`,
		`viewBox="0 0 64 64"`,
		"setPageChromeForTab",
		"updateDocumentTitle",
		"pageTitleForTab",
		"dedupeTitleParts",
		`Search: ${query}`,
		"Doc preview:",
		"File preview:",
		"state.project?.generatedTitle",
		"document.title = \"Failed to load docs | Docs Preview\"",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI title/favicon support missing %s", want)
		}
	}
}

func TestPreviewGraphLabelsUseDarkModeContrast(t *testing.T) {
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	graphJS, err := os.ReadFile("preview_ui_src/js/graph.ts")
	if err != nil {
		t.Fatal(err)
	}
	networkGraphJS, err := os.ReadFile("preview_ui_src/js/network_graph.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(css) + "\n" + string(app) + "\n" + string(graphJS)
	for _, want := range []string{"labelColor", "#f8fafc", "#0f172a", "edgeColorForTheme", "searchEdgeColorForTheme", "#334155", "#991b1b", ".graph-canvas canvas", ".search-graph-canvas canvas"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview graph label dark mode contrast missing %s", want)
		}
	}
	if strings.Contains(string(networkGraphJS), "highlighted: node === selectedId") {
		t.Fatalf("focused preview graph node should not render Sigma highlighted label background")
	}
	if !strings.Contains(string(networkGraphJS), "defaultDrawNodeHover: drawNodeHoverLabelOnly") {
		t.Fatalf("hovered preview graph node should not render Sigma label background")
	}
	if !strings.Contains(string(networkGraphJS), "color: selected && !related ? colorWithOpacity(data.color, 0.14) : data.color") {
		t.Fatalf("focused preview graph should dim unfocused edges without hiding them")
	}
	themeRerender := string(app)
	rerenderIndex := strings.Index(themeRerender, "function rerenderForTheme()")
	if rerenderIndex == -1 || !strings.Contains(themeRerender[rerenderIndex:], "renderSearchPanels();") {
		t.Fatalf("preview graph label dark mode contrast should rerender search graph panels")
	}
}

func TestPreviewUIRendersMarkdownClientSide(t *testing.T) {
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app) + "\n" + string(css)
	if strings.Contains(text, "fallbackHTML") || strings.Contains(text, "markdownRenderer.render(metadata.body)") {
		t.Fatalf("preview UI should render Markdown from raw content on the client")
	}
	for _, want := range []string{"renderMarkdownPreview", "loadToastMarkdownViewer", "toastui-editor-viewer", "toastMarkdownCustomRenderer", "renderToastMarkdownLoading", "Loading Markdown preview...", "codeBlock", "data-source-language", "markdown-wysiwyg-host", "markdown-toast-viewer", ".markdown-toast-viewer .toastui-editor-contents", ".metadata-table", "padding: 18px 25px", "renderableMarkdownMetadata", "markdownMetadataRows", "renderMetadataTable", "renderMetadataValue", "metadataArrayValues", "cleanMetadataScalar", "metadata-badges", "badge badge-ghost badge-sm"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI Markdown rendering missing %s", want)
		}
	}
}

func TestPreviewUIRendersCompactHTMLDocsClientSide(t *testing.T) {
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app) + "\n" + string(css)
	for _, want := range []string{"renderHTMLPreview", "htmlDocSanitizeConfig", "html-doc", "doc-meta", "doc-title", "doc-description", "doc-link", "doc-relation", "doc-callout", "doc-diagram", "doc-code", "doc-section", "doc-grid", "doc-card", "doc-steps", "doc-step", "doc-flow", "doc-flow-step", "doc-graph", "doc-metrics", "doc-metric", "normalizeHTMLDocTags", "htmlMetadataRows", "normalizeDocDiagramLanguage", "language-c4-model", "replaceDocContainer", "replaceDocMetric", "createDocDiagramSource", "doc-relation-${typeClass}", "doc-code-block", "doc-diagram-source", "doc-graph-source", ".markdown-wysiwyg-shell", ".html-doc", ".html-doc table", ".doc-title", ".doc-description", ".doc-callout-info", ".doc-callout-warning", ".doc-relation-depends", ".doc-code-block::before", ".doc-steps", ".doc-flow-step", ".doc-metrics", ".doc-metric-value"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI HTML doc rendering missing %s", want)
		}
	}
	for _, forbidden := range []string{"data-reactroot", "onclick", "onload", "onerror"} {
		if !strings.Contains(text, forbidden) {
			t.Fatalf("preview UI HTML sanitizer should explicitly reject %s", forbidden)
		}
	}
}

func TestPreviewUIKeepsMarkdownDocumentsReadOnly(t *testing.T) {
	html, err := os.ReadFile("preview_ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(app) + "\n" + string(css)
	for _, want := range []string{
		".markdown-wysiwyg-host",
		".toast-markdown-loading",
		"renderMarkdownPreview",
		"renderHTMLPreview",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview read-only Markdown UI missing %s", want)
		}
	}
	for _, forbidden := range []string{
		`id="markdownEditToolbar"`,
		`id="markdownEditActions"`,
		`id="markdownEditButton"`,
		`id="markdownSaveButton"`,
		`id="markdownCancelButton"`,
		`id="rawMarkdownToggle"`,
		`data-markdown-command=`,
		"showRawMarkdown",
		"updateRawMarkdownToggle",
		"rawMarkdown:",
		"editingMarkdown",
		"markdownEditor",
		"saveMarkdownDraft",
		"applyMarkdownCommand",
		"mountMarkdownPreviewEditor",
		"destroyMarkdownPreviewEditor",
		"loadToastMarkdownEditor",
		"Loading Markdown editor...",
		`initialEditType: "wysiwyg"`,
		"hideModeSwitch: true",
		`toolbarItems: [["table"]]`,
		"clickToastTableToolbarItem",
		"toastui-editor.css",
		"PUT",
		"/api/docs/${encodeURIComponent(state.currentSpec.id)}",
		".markdown-edit-toolbar",
		".markdown-edit-actions",
		".markdown-edit-action-group",
		".metadata-edit-panel",
		"markdown-editing",
		"Editing Markdown",
		"markdownEditorStatus",
		"replaceSelectedLines",
		"wrapMarkdownSelection",
		"replaceMarkdownSelection",
		".markdown-editor-input",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("preview Markdown read-only UI should not keep editor helper %s", forbidden)
		}
	}
}

func TestPreviewUIResolvesInternalLinksAndMentionsThroughRouter(t *testing.T) {
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{
		"decorateInternalDocNavigation",
		"decorateInternalDocLinks",
		"decorateInternalDocMentions",
		"resolveSpecNavigationTarget",
		"internalSpecLink",
		"@doc",
		"@spec",
		"internalDocMentionPattern",
		`+\.(?:md|html?)`,
		"doc-relation-${typeClass}",
		"relation.href = target",
		"navigateToSpecTarget",
		"selectSpec(target.specId, true",
		"pushSpecRoute",
		"scrollToSpecFragment",
		`${pathNoExt}.html`,
		`${basenameNoExt}.html`,
		"candidates.add(`${key}.html`)",
		"candidates.add(`${key}/_overview.html`)",
		"NodeFilter.SHOW_TEXT",
		`closest("a, pre, code, script, style")`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview internal link router support missing %s", want)
		}
	}
	for _, forbidden := range []string{"window.location.href", "location.assign", "location.replace"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("preview internal docs navigation should not use raw redirect: %s", forbidden)
		}
	}
}

func TestPreviewMarkdownTablesWrapLongCellContent(t *testing.T) {
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(css)
	for _, want := range []string{".markdown td code", ".markdown th", "text-align: left", "overflow-wrap: anywhere", "word-break: break-word", "overflow-x: auto"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview table CSS missing %s", want)
		}
	}
}

func TestPreviewSidebarIsFixedTreeWithIcons(t *testing.T) {
	html, err := os.ReadFile("preview_ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(app)
	for _, want := range []string{"lg:fixed", "buildSpecTree", "renderFolderNode", "folder-open", "data-lucide=\"file-text", "lucide.createIcons"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview sidebar missing %s", want)
		}
	}
}

func TestPreviewUIConnectsHotReload(t *testing.T) {
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"new EventSource(\"/api/events\")", "reloadPreviewData", "addEventListener(\"change\"", "addEventListener(\"ready\"", "hotReloadToken", "parseEventToken", "window.location.reload"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI hot reload missing %s", want)
		}
	}
}

func TestPreviewUIUpdatesURLForFocusedTabs(t *testing.T) {
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"routeFromLocation", "updateRouteURL", "window.history.pushState", "window.location.pathname", "popstate", "encodeSpecPath", "join(\"/\")"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI route handling missing %s", want)
		}
	}
	if count := strings.Count(text, "selectSpec(state.selectedId, false, { updateURL: false })"); count < 2 {
		t.Fatalf("preview UI should not rewrite /graph or /search to a default spec during initial load/reload; found %d guarded selects", count)
	}
	if strings.Contains(text, "hashchange") {
		t.Fatalf("preview UI should use path routing without hash fragments")
	}
}

func TestPreviewDiagramSanitizerKeepsMermaidLabels(t *testing.T) {
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"USE_PROFILES", "foreignObject", "\"div\"", "\"span\"", "\"style\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview diagram sanitizer missing %s, Mermaid labels may be stripped", want)
		}
	}
}

func TestPreviewDiagramUsesThemeAwareEdgesAndLabels(t *testing.T) {
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"mermaidSourceForTheme", "mermaidC4ElementStyles", "UpdateElementStyle", "$fontColor=\"${fontColor}\"", "$borderColor=\"${borderColor}\"", "mermaidC4RelationStyles", "UpdateRelStyle", "$textColor=\"${textColor}\"", "$lineColor=\"${lineColor}\"", "mermaidThemeConfig", "applyDiagramThemeOverrides", "applyC4BoundaryThemeOverrides", "isC4BoundaryRect", "darkMode: true", "lineColor: \"#cbd5e1\"", "edgeLabelBackground: \"#111827\"", "relationColor: \"#cbd5e1\"", "relationLabelColor: \"#f8fafc\"", ":scope > rect", ":scope > text", "stroke-dasharray", ".relationship line", ".relationshipLabel *", ".edgeLabel *", "marker path"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview diagram dark theme edge/label support missing %s", want)
		}
	}
}

func TestPreviewUISupportsDarkMode(t *testing.T) {
	html, err := os.ReadFile("preview_ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(app) + "\n" + string(css)
	for _, want := range []string{"spec-preview-theme", "prefers-color-scheme: dark", "id=\"themeToggle\"", "applyTheme", "mermaidThemeConfig", `theme: state.theme === "dark" ? "dark" : "default"`, "renderCurrentSpecContent().catch"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI dark mode missing %s", want)
		}
	}
}

func TestPreviewUIUsesSafeScrollbars(t *testing.T) {
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(css)
	for _, want := range []string{"scrollbar-gutter: stable", "scrollbar-width: thin", "::-webkit-scrollbar-thumb", "--scrollbar-thumb", "background-clip: padding-box"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview safe scrollbar CSS missing %s", want)
		}
	}
}

func TestPreviewUIRendersMermaidWithSvgPanZoom(t *testing.T) {
	html, err := os.ReadFile("preview_ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(app) + "\n" + string(css)
	for _, want := range []string{"decorateDiagram", "diagram-surface", "diagram-toolbar", "diagram-viewport", "diagramPanZoomInstances", "diagramPanZoomTargets", "window.svgPanZoom"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview Mermaid svg-pan-zoom integration missing %s", want)
		}
	}
	for _, forbidden := range []string{"id=\"diagramLightbox\"", "openDiagramLightbox", "showModal()", "diagram-lightbox", "diagramViewports"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("preview Mermaid should not use old lightbox/custom viewport code: %s", forbidden)
		}
	}
}

func TestPreviewUIRendersMermaidC4Fences(t *testing.T) {
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"data-source-language", "renderDocumentDiagrams", "isMermaidDiagramBlock", "mermaidC4DiagramTypeFromBlock", "looksLikeMermaidC4Diagram", "C4(?:Context|Container|Component|Dynamic|Deployment)", "replace(/[-_]/g, \"\")", "c4(?:context|container|component|dynamic|deployment)?", "C4Component"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI Mermaid C4 fence rendering missing %s", want)
		}
	}
}

func TestPreviewUIRendersLikeC4ModelThroughMermaidC4(t *testing.T) {
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"data-source-language", "renderLikeC4Blocks", "isLikeC4ModelBlock", "language-likec4", "language-c4-model", "language === \"c4model\"", "looksLikeLikeC4Model", "likeC4ModelToMermaid", "appendLikeC4MermaidRoot", "node.kind === \"softwareSystem\"", "C4Component", "Container_Boundary", "Component(", "Rel(", "relation[3] || \"Uses\"", "LikeC4 model"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI LikeC4 model rendering missing %s", want)
		}
	}
}

func TestPreviewDiagramUsesSvgPanZoomAPI(t *testing.T) {
	html, err := os.ReadFile("preview_ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(app) + "\n" + string(css)
	for _, want := range []string{"data-diagram-action=\"zoom-in\"", "data-diagram-action=\"zoom-out\"", "data-diagram-action=\"fit\"", "diagram-zoom-level", "Command-scroll to zoom", "viewportSelector: \".svg-pan-zoom_viewport\"", "zoomEnabled: true", "panEnabled: true", "mouseWheelZoomEnabled: true", "beforeWheel", "event.ctrlKey || event.metaKey", "zoomScaleSensitivity: 0.4", "instance.zoomIn()", "instance.zoomOut()", "instance.fit()", "instance.center()", "instance.resetZoom()", "instance.resetPan()"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview Mermaid svg-pan-zoom API missing %s", want)
		}
	}
	for _, forbidden := range []string{"zoomDiagramViewport", "fitDiagramViewport", "centerDiagramViewport", "pointerdown", "pointermove", "setPointerCapture", "view.stage.style.transform"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("preview Mermaid should delegate zoom/pan to svg-pan-zoom: %s", forbidden)
		}
	}
}

func TestPreviewDiagramPanZoomLifecycleIsManaged(t *testing.T) {
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"destroyDiagramPanZoom", "destroyDiagramsIn", "instance.destroy()", "state.diagramPanZoomInstances.set", "state.diagramPanZoomInstances.delete", "state.diagramPanZoomTargets.delete"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview Mermaid svg-pan-zoom lifecycle missing %s", want)
		}
	}
	for _, forbidden := range []string{"dataset.baseWidth", "dataset.baseHeight", "renderWidth", "renderHeight"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("preview Mermaid should not keep custom SVG resize zoom code: %s", forbidden)
		}
	}
}

func TestPreviewDiagramSvgIsPreparedForLibraryViewport(t *testing.T) {
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"svgDiagramSize", "svg.setAttribute(\"width\", String(size.width))", "svg.setAttribute(\"height\", String(size.height))", "svg.style.width = \"100%\"", "svg.style.height = \"100%\"", "svg.style.maxWidth = \"none\"", "svg.setAttribute(\"preserveAspectRatio\"", "svg.classList.add(\"diagram-svg\")", "prepareSvgPanZoomViewport", "svg-pan-zoom_viewport"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview Mermaid SVG library preparation missing %s", want)
		}
	}
	if strings.Contains(text, "svg.removeAttribute(\"width\")") || strings.Contains(text, "svg.removeAttribute(\"height\")") {
		t.Fatalf("preview Mermaid should let svg-pan-zoom manage rendered SVG sizing")
	}
}

func TestPreviewDiagramViewportUsesHiddenOverflowWithoutCustomBackground(t *testing.T) {
	app, err := os.ReadFile("preview_ui_src/app.ts")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app) + "\n" + string(css)
	for _, want := range []string{"overflow: hidden", "touch-action: none"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview inline Mermaid viewport behavior missing %s", want)
		}
	}
	for _, forbidden := range []string{"injectSvgBackground", "diagram-lightbox__svg-bg", "clone.style.background", "--diagram-canvas-bg", "--diagram-grid-line"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("preview Mermaid should not add background to diagram SVG: %s", forbidden)
		}
	}
	if strings.Contains(text, "scrollbar-gutter: stable both-edges") {
		t.Fatalf("preview Mermaid viewport should not reserve scrollbar gutter")
	}
}
