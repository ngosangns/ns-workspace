package preview

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func previewUIText(t *testing.T) string {
	t.Helper()
	paths := []string{
		"preview_ui_src/index.html",
		"preview_ui_src/search.html",
		"preview_ui_src/main.ts",
		"preview_ui_src/search-main.ts",
		"preview_ui_src/App.vue",
		"preview_ui_src/SearchStandaloneApp.vue",
		"preview_ui_src/app.ts",
		"preview_ui_src/js/graph.ts",
		"preview_ui_src/js/network_graph.ts",
		"preview_ui_src/js/internal-links.ts",
		"preview_ui_src/types.d.ts",
		"preview_ui_src/components/DocViewer.vue",
		"preview_ui_src/components/GraphViewer.vue",
		"preview_ui_src/components/Icon.vue",
		"preview_ui_src/components/PreviewModal.vue",
		"preview_ui_src/components/SearchPanel.vue",
		"preview_ui_src/components/Sidebar.vue",
		"preview_ui_src/components/TreeNode.vue",
		"preview_ui_src/public/style.css",
		"preview_ui/index.html",
		"preview_ui/style.css",
	}
	var builder strings.Builder
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read preview UI source %s: %v", path, err)
		}
		builder.Write(data)
		builder.WriteByte('\n')
	}
	return builder.String()
}

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

	res, err = http.Get(ts.URL + "/search.html")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("standalone search app was not served: %s", res.Status)
	}

	res, err = http.Get(ts.URL + "/favicon.svg")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("preview static asset was not served: %s", res.Status)
	}

	// The published module must include the hashed Vite bundle referenced by index.html.
	indexHTML, err := previewUIFS.ReadFile("preview_ui/index.html")
	if err != nil {
		t.Fatalf("read embedded preview index: %v", err)
	}
	assetPath := regexp.MustCompile(`src="(/assets/[^"]+\.js)"`).FindStringSubmatch(string(indexHTML))
	if len(assetPath) != 2 {
		t.Fatalf("preview index should reference a hashed JS bundle: %s", indexHTML)
	}
	res, err = http.Get(ts.URL + assetPath[1])
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("preview JS bundle was not served: %s", res.Status)
	}
}

func TestSearchLauncherWritesRedirectHTML(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "search.html")
	if err := writeSearchLauncher(out, "http://localhost:12345/search.html", root, filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "http://localhost:12345/search.html") || !strings.Contains(text, root) {
		t.Fatalf("search launcher did not include app URL and project metadata: %s", string(data))
	}
	if strings.Contains(text, `\"http://localhost:12345/search.html\"`) {
		t.Fatalf("search launcher should not add literal quotes to the redirect URL: %s", text)
	}
	if !strings.Contains(text, `window.location.replace("http:\/\/localhost:12345\/search.html")`) {
		t.Fatalf("search launcher should emit a valid JavaScript redirect string: %s", text)
	}
}

func TestGraphRequiresQuery(t *testing.T) {
	err := RunGraph([]string{"--project", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "graph requires --query") {
		t.Fatalf("graph without --query should fail with a focused message, got %v", err)
	}
}

func TestGraphQueryJSONUsesSearchPipeline(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "docs/auth.md", "# Auth\n\nCredential vault documentation.\n")
	writeTestFile(t, root, "auth.go", `package demo

func credentialVault() {}
`)
	initGitRepo(t, root, "docs/_index.md", "docs/auth.md", "auth.go")

	provider := &staticCodeGraphProvider{
		warnings: []string{"Code Graph relation expansion is unavailable for this language server."},
		results: []previewSearchResult{{
			ID:         "code-lsp:credential_vault",
			Title:      "credentialVault()",
			Path:       "auth.go",
			Kind:       "function",
			Source:     "lsp",
			Line:       3,
			Score:      0.91,
			MatchedBy:  []string{"graph"},
			NodeID:     "credential_vault",
			Confidence: "lsp",
			Neighbors: []previewSearchNeighbor{{
				ID:        "caller",
				Label:     "caller()",
				Relation:  "calls",
				Direction: "incoming",
				Path:      "caller.go",
				Line:      9,
			}},
		}},
	}
	opt := graphOptions{projectRoot: root, docsDir: "docs", query: "credentialVault", limit: 3, keywordOp: "sum", jsonOutput: true}
	var buf bytes.Buffer
	if err := runGraphQueryWithProvider(context.Background(), opt, provider, &buf); err != nil {
		t.Fatal(err)
	}

	var search previewSearchResponse
	if err := json.NewDecoder(&buf).Decode(&search); err != nil {
		t.Fatalf("graph query should print valid JSON: %v\n%s", err, buf.String())
	}
	if search.Query != "credentialVault" || search.KeywordOperator != "sum" {
		t.Fatalf("graph query should preserve query metadata: %+v", search)
	}
	if len(search.Panels.CodeGraph) != 1 {
		t.Fatalf("graph query should include LSP Code Graph results: %+v", search.Panels.CodeGraph)
	}
	if search.Panels.CodeGraph[0].Path != "auth.go" || search.Panels.CodeGraph[0].Line != 3 {
		t.Fatalf("graph query should expose source location: %+v", search.Panels.CodeGraph[0])
	}
	if len(search.Panels.CodeGraph[0].Neighbors) != 1 || search.Panels.CodeGraph[0].Neighbors[0].Path != "caller.go" {
		t.Fatalf("graph query should expose neighbor preview targets: %+v", search.Panels.CodeGraph[0].Neighbors)
	}
	if !containsString(search.Warnings, "Code Graph relation expansion is unavailable for this language server.") {
		t.Fatalf("graph query should preserve Code Graph warnings: %+v", search.Warnings)
	}
}

func TestGraphQueryTextPrioritizesGraphContext(t *testing.T) {
	response := previewSearchResponse{
		Query: "credentialVault",
		Stats: map[string]int{
			"docsSemantic": 1,
			"docsGraph":    1,
			"codeSemantic": 1,
			"codeGraph":    1,
		},
		Warnings: []string{"gopls not found"},
		Panels: previewSearchPanels{
			CodeGraph: []previewSearchResult{{
				Title:      "credentialVault()",
				Path:       "auth.go",
				Line:       3,
				Source:     "lsp",
				Confidence: "lsp",
				FlowRole:   "match",
				Neighbors: []previewSearchNeighbor{{
					Label:     "caller()",
					Relation:  "calls",
					Direction: "incoming",
					Path:      "caller.go",
					Line:      9,
				}},
			}},
		},
	}
	var buf bytes.Buffer
	if err := writeGraphQueryText(&buf, response); err != nil {
		t.Fatal(err)
	}
	text := buf.String()
	if !strings.Contains(text, "Warnings:") || !strings.Contains(text, "gopls not found") {
		t.Fatalf("text output should include warnings: %s", text)
	}
	codeGraphIndex := strings.Index(text, "Code Graph:")
	docsGraphIndex := strings.Index(text, "Docs Graph:")
	if codeGraphIndex < 0 || docsGraphIndex < 0 || codeGraphIndex > docsGraphIndex {
		t.Fatalf("text output should show Code Graph before other panels: %s", text)
	}
	if !strings.Contains(text, "credentialVault() (auth.go:3)") || !strings.Contains(text, "incoming caller() via calls (caller.go:9)") {
		t.Fatalf("text output should include source and neighbor locations: %s", text)
	}
}

type staticCodeGraphProvider struct {
	results  []previewSearchResult
	warnings []string
	closed   bool
}

func (p *staticCodeGraphProvider) SearchCodeGraph(ctx context.Context, query string, tokens []string, exclusionQuery string, exclusionTokens []string, limit int) ([]previewSearchResult, []string) {
	filtered := []previewSearchResult{}
	for _, result := range p.results {
		haystack := strings.Join([]string{result.Title, result.Path, result.NodeID, result.Kind}, " ")
		if excludedByKeywordSearch(exclusionQuery, exclusionTokens, result.Title, result.Path, haystack) {
			continue
		}
		if graphScore(query, tokens, haystack) <= 0 {
			continue
		}
		filtered = append(filtered, result)
	}
	sortSearchResults(filtered)
	return limitResults(filtered, graphExpansionLimit(limit)), p.warnings
}

func (p *staticCodeGraphProvider) Close(ctx context.Context) error {
	p.closed = true
	return nil
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

	func validateAuthSession(raw string) bool {
		return parseAuthToken(raw) != ""
	}
	`)
	initGitRepo(t, root, "docs/_index.md", "docs/auth.md", "docs/session.md", "auth.go")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	server.codeGraph = &staticCodeGraphProvider{results: []previewSearchResult{{
		ID:        "code-lsp:parse_auth_token",
		Title:     "parseAuthToken()",
		Path:      "auth.go",
		Kind:      "function",
		Source:    "lsp",
		Line:      3,
		Score:     0.91,
		MatchedBy: []string{"graph"},
		NodeID:    "parse_auth_token",
		Neighbors: []previewSearchNeighbor{{
			ID:        "validate_auth_session",
			Label:     "validateAuthSession()",
			Relation:  "calls",
			Direction: "outgoing",
			SourceID:  "parse_auth_token",
			TargetID:  "validate_auth_session",
			Path:      "auth.go",
			Line:      7,
		}},
	}}}
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
	if search.Panels.CodeGraph[0].Neighbors[0].Path != "auth.go" || search.Panels.CodeGraph[0].Neighbors[0].Line != 7 {
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

func TestPreviewSearchScansMarkdownDocsUnderDocs(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "docs/specs/auth.md", "# Auth\n\nAuthentication validates session tokens.\n")
	writeTestFile(t, root, "docs/reference/settings.custom", "feature_flag: preview_index_all_docs_files\n")
	writeTestFile(t, root, "main.go", "package main\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/search?q=Authentication")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var search previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&search); err != nil {
		t.Fatal(err)
	}
	if len(search.Panels.DocsSemantic) == 0 {
		t.Fatalf("expected docs semantic search to include docs Markdown files: %+v", search.Panels)
	}
	got := search.Panels.DocsSemantic[0]
	if got.Path != "specs/auth.md" || got.SpecID != "specs/auth.md" || got.Kind != "doc" {
		t.Fatalf("expected docs-relative Markdown document result, got %+v", got)
	}
	if len(search.Panels.CodeSemantic) != 0 {
		t.Fatalf("Markdown docs should not be duplicated in code semantic results: %+v", search.Panels.CodeSemantic)
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

func TestPreviewSearchScansMarkdownAndHTMLAcrossRepo(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "README.md", "# Readme\n\nRoot repo markdown covers repo_wide_markdown_search.\n")
	writeTestFile(t, root, "guides/setup.html", "<h1>Setup</h1><p>repo_wide_html_search marker.</p>\n")
	writeTestFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	initGitRepo(t, root, "docs/_index.md", "README.md", "guides/setup.html", "main.go")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/search?q=repo_wide_markdown_search")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var markdownSearch previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&markdownSearch); err != nil {
		t.Fatal(err)
	}
	if len(markdownSearch.Panels.DocsSemantic) == 0 || markdownSearch.Panels.DocsSemantic[0].Path != "README.md" {
		t.Fatalf("expected repo root Markdown in docs semantic results: %+v", markdownSearch.Panels)
	}
	if markdownSearch.Panels.DocsSemantic[0].SpecID != "" {
		t.Fatalf("repo Markdown outside docs root should open as file, got spec id: %+v", markdownSearch.Panels.DocsSemantic[0])
	}
	if len(markdownSearch.Panels.CodeSemantic) != 0 {
		t.Fatalf("repo Markdown should not be duplicated in code semantic results: %+v", markdownSearch.Panels.CodeSemantic)
	}

	res, err = http.Get(ts.URL + "/api/search?q=repo_wide_html_search")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var htmlSearch previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&htmlSearch); err != nil {
		t.Fatal(err)
	}
	if len(htmlSearch.Panels.DocsSemantic) == 0 || htmlSearch.Panels.DocsSemantic[0].Path != "guides/setup.html" {
		t.Fatalf("expected repo HTML in docs semantic results: %+v", htmlSearch.Panels)
	}
	if len(htmlSearch.Panels.CodeSemantic) != 0 {
		t.Fatalf("repo HTML should not be duplicated in code semantic results: %+v", htmlSearch.Panels.CodeSemantic)
	}
}

func TestPreviewSearchDirectlySearchesDocsGraph(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", `# Spec Index

## Dependency Graph

`+"```"+`
policyEngine → accessRule
`+"```"+`
`)
	writeTestFile(t, root, "docs/guide.md", "# Guide\n\nHandles session credential lookup.\n")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/search?q=policyEngine")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var search previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&search); err != nil {
		t.Fatal(err)
	}
	if len(search.Panels.DocsSemantic) != 0 {
		t.Fatalf("docs graph search should not require docs semantic results, got %+v", search.Panels.DocsSemantic)
	}
	if len(search.Panels.DocsGraph) == 0 {
		t.Fatalf("expected docs graph direct query result: %+v", search.Panels)
	}
	if containsString(search.Panels.DocsGraph[0].MatchedBy, "semantic-anchor") || !containsString(search.Panels.DocsGraph[0].MatchedBy, "graph") {
		t.Fatalf("expected docs graph direct match, got %+v", search.Panels.DocsGraph[0])
	}
	if search.Panels.DocsGraph[0].NodeID != "policyEngine" {
		t.Fatalf("expected policyEngine graph node, got %+v", search.Panels.DocsGraph[0])
	}
	if len(search.Panels.DocsGraph[0].Neighbors) == 0 || search.Panels.DocsGraph[0].Neighbors[0].ID != "accessRule" {
		t.Fatalf("expected docs graph direct result to expose neighbors, got %+v", search.Panels.DocsGraph[0].Neighbors)
	}
}

func TestPreviewSearchDirectlySearchesLSPCodeGraph(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "auth.go", `package demo

func parseToken(raw string) string {
	return raw
}
`)
	writeTestFile(t, root, "store.go", `package demo

func hydrateStore() {}
`)
	initGitRepo(t, root, "docs/_index.md", "auth.go", "store.go")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	server.codeGraph = &staticCodeGraphProvider{results: []previewSearchResult{{
		ID:        "code-lsp:credential_vault",
		Title:     "credentialVault()",
		Path:      "store.go",
		Kind:      "function",
		Source:    "lsp",
		Line:      3,
		Score:     0.88,
		MatchedBy: []string{"graph"},
		NodeID:    "credential_vault",
		Neighbors: []previewSearchNeighbor{{
			ID:        "parse_token",
			Label:     "parseToken()",
			Relation:  "calls",
			Direction: "incoming",
			SourceID:  "parse_token",
			TargetID:  "credential_vault",
			Path:      "auth.go",
			Line:      3,
		}},
	}}}
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/search?q=credentialVault")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var search previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&search); err != nil {
		t.Fatal(err)
	}
	if len(search.Panels.CodeSemantic) != 0 {
		t.Fatalf("code graph search should not require code semantic results, got %+v", search.Panels.CodeSemantic)
	}
	if len(search.Panels.CodeGraph) == 0 {
		t.Fatalf("expected code graph direct query result: %+v", search.Panels)
	}
	if containsString(search.Panels.CodeGraph[0].MatchedBy, "semantic-anchor") || !containsString(search.Panels.CodeGraph[0].MatchedBy, "graph") {
		t.Fatalf("expected code graph direct match, got %+v", search.Panels.CodeGraph[0])
	}
	if search.Panels.CodeGraph[0].Path != "store.go" || search.Panels.CodeGraph[0].Line != 3 {
		t.Fatalf("expected code graph to expose matched LSP symbol source, got %+v", search.Panels.CodeGraph[0])
	}
	if len(search.Panels.CodeGraph[0].Neighbors) == 0 || search.Panels.CodeGraph[0].Neighbors[0].Path != "auth.go" {
		t.Fatalf("expected code graph direct result to expose neighbors, got %+v", search.Panels.CodeGraph[0].Neighbors)
	}
}

func TestLSPSourceFilesUseOnlyGitTrackedCode(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "tracked.go", "package demo\n\nfunc AllowedTrackedSymbol() {}\n")
	writeTestFile(t, root, "docs/doc_code.go", "package docs\n\nfunc IgnoredDocsCode() {}\n")
	writeTestFile(t, root, "untracked.go", "package demo\n\nfunc GhostOnlyNeedle() {}\n")
	initGitRepo(t, root, "docs/_index.md", "tracked.go", "docs/doc_code.go")

	_, files, warnings := lspSourceFiles(root, filepath.Join(root, "docs"))
	if len(warnings) != 0 {
		t.Fatalf("unexpected source scan warnings: %+v", warnings)
	}
	got := []string{}
	for _, file := range files {
		got = append(got, file.Rel)
	}
	if len(got) != 1 || got[0] != "tracked.go" {
		t.Fatalf("LSP code graph should use tracked source outside docs only, got %+v", got)
	}
}

func TestLSPCodeGraphSearchUsesSymbolOwnersAndDeterministicSorting(t *testing.T) {
	root := t.TempDir()
	provider := newPreviewLSPCodeGraphProvider(root, filepath.Join(root, "docs"))
	index := lspCodeGraphIndex{
		Nodes: map[string]lspCodeNode{
			"z_needle": {
				ID:             "z_needle",
				Name:           "ZNeedle",
				FullName:       "ZNeedle",
				Kind:           12,
				KindLabel:      "function",
				Path:           "zeta.go",
				SelectionRange: lspRange{Start: lspPosition{Line: 2}},
			},
			"a_needle": {
				ID:             "a_needle",
				Name:           "ANeedle",
				FullName:       "ANeedle",
				Kind:           12,
				KindLabel:      "function",
				Path:           "alpha.go",
				SelectionRange: lspRange{Start: lspPosition{Line: 2}},
			},
			"load_secret": {
				ID:             "load_secret",
				Name:           "LoadSecret",
				FullName:       "CredentialVault.LoadSecret",
				Owner:          "CredentialVault",
				Kind:           6,
				KindLabel:      "method",
				Path:           "vault.go",
				SelectionRange: lspRange{Start: lspPosition{Line: 4}},
			},
		},
		ByPath: map[string][]string{
			"alpha.go": {"a_needle"},
			"zeta.go":  {"z_needle"},
			"vault.go": {"load_secret"},
		},
	}

	results, _ := searchLSPCodeGraph(context.Background(), provider, index, "Needle", searchTokens("Needle"), "", nil, 8)
	if len(results) < 2 {
		t.Fatalf("expected both direct LSP code graph matches, got %+v", results)
	}
	if results[0].NodeID != "a_needle" || results[1].NodeID != "z_needle" {
		t.Fatalf("LSP code graph direct matches should sort deterministically, got %+v", results[:2])
	}

	results, _ = searchLSPCodeGraph(context.Background(), provider, index, "CredentialVault", searchTokens("CredentialVault"), "", nil, 8)
	if len(results) == 0 || results[0].NodeID != "load_secret" {
		t.Fatalf("expected owner-backed LSP method match, got %+v", results)
	}
	if results[0].Title != "CredentialVault.LoadSecret()" {
		t.Fatalf("method node title should include LSP owner, got %+v", results[0])
	}
}

func TestFlattenLSPSymbolsIndexesOnlyCallableNodesWithOwners(t *testing.T) {
	root := t.TempDir()
	file := lspSourceFile{
		Rel:      "service.ts",
		Abs:      filepath.Join(root, "service.ts"),
		Language: lspLanguage{ServerID: "typescript", LanguageID: "typescript"},
	}
	index := lspCodeGraphIndex{Nodes: map[string]lspCodeNode{}, ByPath: map[string][]string{}}

	flattenLSPSymbols(&index, file, []lspDocumentSymbol{{
		Name: "CredentialVault",
		Kind: 5,
		Range: lspRange{
			Start: lspPosition{Line: 1},
			End:   lspPosition{Line: 20},
		},
		SelectionRange: lspRange{Start: lspPosition{Line: 1}},
		Children: []lspDocumentSymbol{
			{
				Name: "loadSecret",
				Kind: 6,
				Range: lspRange{
					Start: lspPosition{Line: 4},
					End:   lspPosition{Line: 8},
				},
				SelectionRange: lspRange{Start: lspPosition{Line: 4, Character: 2}},
			},
			{
				Name:           "hydrate",
				Kind:           12,
				ContainerName:  "ExplicitContainer",
				Range:          lspRange{Start: lspPosition{Line: 10}, End: lspPosition{Line: 12}},
				SelectionRange: lspRange{Start: lspPosition{Line: 10, Character: 2}},
			},
			{
				Name:           "state",
				Kind:           13,
				Range:          lspRange{Start: lspPosition{Line: 14}, End: lspPosition{Line: 14}},
				SelectionRange: lspRange{Start: lspPosition{Line: 14, Character: 2}},
			},
		},
	}}, nil)

	if len(index.Nodes) != 2 {
		t.Fatalf("expected only callable child symbols to be indexed, got %+v", index.Nodes)
	}
	if got := len(index.ByPath["service.ts"]); got != 2 {
		t.Fatalf("expected ByPath to include both callables, got %d", got)
	}
	var loadSecret, hydrate lspCodeNode
	for _, node := range index.Nodes {
		switch node.Name {
		case "loadSecret":
			loadSecret = node
		case "hydrate":
			hydrate = node
		}
	}
	if loadSecret.FullName != "CredentialVault.loadSecret" || loadSecret.Owner != "CredentialVault" || loadSecret.KindLabel != "method" {
		t.Fatalf("expected nested method to inherit owner and kind label, got %+v", loadSecret)
	}
	if hydrate.FullName != "ExplicitContainer.hydrate" || hydrate.Owner != "ExplicitContainer" || hydrate.KindLabel != "function" {
		t.Fatalf("expected containerName to override parent owner, got %+v", hydrate)
	}
}

func TestParseLSPDocumentSymbolsSupportsFlatAndHierarchicalResults(t *testing.T) {
	flatRaw := json.RawMessage(`[
		{"name":"LoadSecret","kind":12,"containerName":"CredentialVault","location":{"uri":"file:///tmp/service.go","range":{"start":{"line":4,"character":1},"end":{"line":6,"character":1}}}}
	]`)
	flat, err := parseLSPDocumentSymbols(flatRaw)
	if err != nil {
		t.Fatal(err)
	}
	if len(flat) != 1 || flat[0].Name != "LoadSecret" || flat[0].ContainerName != "CredentialVault" || flat[0].SelectionRange.Start.Line != 4 {
		t.Fatalf("flat SymbolInformation results should map to document symbols, got %+v", flat)
	}

	hierRaw := json.RawMessage(`[
		{"name":"CredentialVault","kind":5,"range":{"start":{"line":1,"character":0},"end":{"line":9,"character":1}},"selectionRange":{"start":{"line":1,"character":6},"end":{"line":1,"character":21}},"children":[{"name":"LoadSecret","kind":6,"range":{"start":{"line":3,"character":2},"end":{"line":5,"character":3}},"selectionRange":{"start":{"line":3,"character":7},"end":{"line":3,"character":17}}}]}
	]`)
	hier, err := parseLSPDocumentSymbols(hierRaw)
	if err != nil {
		t.Fatal(err)
	}
	if len(hier) != 1 || len(hier[0].Children) != 1 || hier[0].Children[0].Name != "LoadSecret" {
		t.Fatalf("hierarchical DocumentSymbol results should preserve children, got %+v", hier)
	}
}

func TestLSPCodeGraphLocationMappingUsesSelectionRangeAndSmallestContainer(t *testing.T) {
	root := t.TempDir()
	authPath := filepath.Join(root, "internal", "auth.go")
	index := lspCodeGraphIndex{
		Nodes: map[string]lspCodeNode{
			"outer": {
				ID:   "outer",
				Path: "internal/auth.go",
				Range: lspRange{
					Start: lspPosition{Line: 1},
					End:   lspPosition{Line: 20},
				},
				SelectionRange: lspRange{Start: lspPosition{Line: 1, Character: 5}, End: lspPosition{Line: 1, Character: 10}},
			},
			"inner": {
				ID:   "inner",
				Path: "internal/auth.go",
				Range: lspRange{
					Start: lspPosition{Line: 6},
					End:   lspPosition{Line: 8},
				},
				SelectionRange: lspRange{Start: lspPosition{Line: 6, Character: 4}, End: lspPosition{Line: 6, Character: 9}},
			},
		},
		ByPath: map[string][]string{"internal/auth.go": {"outer", "inner"}},
	}

	if got := index.nodeIDForLocation(root, fileURI(authPath), lspPosition{Line: 6, Character: 6}); got != "inner" {
		t.Fatalf("selection range should map exact symbol location, got %q", got)
	}
	if got := index.containingNodeIDForLocation(root, fileURI(authPath), lspPosition{Line: 7, Character: 1}); got != "inner" {
		t.Fatalf("reference fallback should choose smallest containing node, got %q", got)
	}
	if got := index.containingNodeIDForLocation(root, fileURI(authPath), lspPosition{Line: 15, Character: 1}); got != "outer" {
		t.Fatalf("outer range should contain locations outside inner range, got %q", got)
	}
}

func TestAssignLSPGraphNeighborsDedupeAndLimit(t *testing.T) {
	index := lspCodeGraphIndex{Nodes: map[string]lspCodeNode{
		"anchor": {
			ID:             "anchor",
			Name:           "Anchor",
			FullName:       "Anchor",
			Kind:           12,
			KindLabel:      "function",
			Path:           "anchor.go",
			SelectionRange: lspRange{Start: lspPosition{Line: 2}},
		},
	}}
	results := map[string]previewSearchResult{
		"anchor": {Title: "Anchor()", NodeID: "anchor"},
	}
	edges := []lspCodeEdge{}
	for i := 0; i < maxGraphNeighborUI+3; i++ {
		id := fmt.Sprintf("neighbor_%02d", i)
		index.Nodes[id] = lspCodeNode{
			ID:             id,
			Name:           fmt.Sprintf("Neighbor%02d", i),
			FullName:       fmt.Sprintf("Neighbor%02d", i),
			Kind:           12,
			KindLabel:      "function",
			Path:           fmt.Sprintf("neighbor_%02d.go", i),
			SelectionRange: lspRange{Start: lspPosition{Line: i}},
		}
		edges = append(edges, lspCodeEdge{Source: "anchor", Target: id, Relation: "calls", SourceID: "anchor", TargetID: id})
	}
	edges = append(edges, edges[0])

	assignLSPGraphNeighbors(results, index, edges)
	neighbors := results["anchor"].Neighbors
	if len(neighbors) != maxGraphNeighborUI {
		t.Fatalf("neighbors should be deduped and capped to UI limit, got %d: %+v", len(neighbors), neighbors)
	}
	if neighbors[0].Direction != "outgoing" || neighbors[0].Relation != "calls" || neighbors[0].Path != "neighbor_00.go" || neighbors[0].Line != 1 {
		t.Fatalf("neighbor should include direction, relation and preview target, got %+v", neighbors[0])
	}
}

func TestSearchLSPCodeGraphExclusionAndRelationWarnings(t *testing.T) {
	root := t.TempDir()
	provider := newPreviewLSPCodeGraphProvider(root, filepath.Join(root, "docs"))
	index := lspCodeGraphIndex{
		Nodes: map[string]lspCodeNode{
			"needle": {
				ID:             "needle",
				Name:           "Needle",
				FullName:       "Needle",
				Kind:           12,
				KindLabel:      "function",
				ServerID:       "missing",
				Path:           "needle.go",
				SelectionRange: lspRange{Start: lspPosition{Line: 3}},
			},
		},
		ByPath: map[string][]string{"needle.go": {"needle"}},
	}

	results, warnings := searchLSPCodeGraph(context.Background(), provider, index, "Needle", searchTokens("Needle"), "needle.go", searchTokens("needle.go"), 8)
	if len(results) != 0 || len(warnings) != 0 {
		t.Fatalf("exclusion query should remove candidate before relation expansion, got results=%+v warnings=%+v", results, warnings)
	}

	results, warnings = searchLSPCodeGraph(context.Background(), provider, index, "Needle", searchTokens("Needle"), "", nil, 8)
	if len(results) == 0 || results[0].NodeID != "needle" || !results[0].Anchor || results[0].FlowRole != "match" {
		t.Fatalf("expected direct anchor result despite missing relation server, got %+v", results)
	}
	var warned bool
	for _, warning := range warnings {
		if strings.Contains(warning, "LSP server missing is not running") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("expected relation warning for missing server, got %+v", warnings)
	}
}

func TestPreviewLSPManagerFindsGoBinOutsidePATH(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	writeTestFile(t, home, "go/bin/gopls", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(home, "go", "bin", executableNames("gopls")[0]), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GOPATH", "")
	t.Setenv("GOBIN", "")
	t.Setenv("PATH", "")

	manager := newPreviewLSPManager(root)
	got, err := manager.resolveCommand(lspLanguage{Command: "gopls"})
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(home, "go", "bin", executableNames("gopls")[0]) {
		t.Fatalf("expected GOPATH-style gopls fallback, got %q", got)
	}
}

func TestPreviewLSPManagerFindsProjectNodeBinOutsidePATH(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	command := executableNames("typescript-language-server")[0]
	writeTestFile(t, root, filepath.Join("node_modules", ".bin", command), "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(root, "node_modules", ".bin", command), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")

	manager := newPreviewLSPManager(root)
	got, err := manager.resolveCommand(lspLanguage{Command: "typescript-language-server"})
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(root, "node_modules", ".bin", command) {
		t.Fatalf("expected project node_modules LSP fallback, got %q", got)
	}
}

func TestPreviewLSPManagerFindsCachedNodeBinOutsidePATH(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	cache := t.TempDir()
	command := executableNames("typescript-language-server")[0]
	writeTestFile(t, cache, filepath.Join("typescript", "node_modules", ".bin", command), "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(cache, "typescript", "node_modules", ".bin", command), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")
	t.Setenv(lspCacheEnv, cache)
	t.Chdir(root)

	manager := newPreviewLSPManager(root)
	got, source, err := manager.resolveCommandWithSource("typescript-language-server")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(cache, "typescript", "node_modules", ".bin", command) {
		t.Fatalf("expected cached TypeScript LSP fallback, got %q", got)
	}
	if source != "cache" {
		t.Fatalf("expected cache source, got %q", source)
	}
}

func TestPreviewLSPManagerFindsCachedWebAndKotlinBinsOutsidePATH(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	cache := t.TempDir()
	commands := map[string]string{
		"html":   "vscode-html-language-server",
		"css":    "vscode-css-language-server",
		"kotlin": "kotlin-lsp",
	}
	for id, command := range commands {
		name := executableNames(command)[0]
		dir := filepath.Join(id, "bin")
		if id == "html" || id == "css" {
			dir = filepath.Join(id, "node_modules", ".bin")
		}
		writeTestFile(t, cache, filepath.Join(dir, name), "#!/bin/sh\n")
		if err := os.Chmod(filepath.Join(cache, dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")
	t.Setenv(lspCacheEnv, cache)
	t.Chdir(root)

	manager := newPreviewLSPManager(root)
	for id, command := range commands {
		got, source, err := manager.resolveCommandWithSource(command)
		if err != nil {
			t.Fatalf("%s should resolve from cache: %v", command, err)
		}
		if !strings.Contains(got, filepath.Join(cache, id)) {
			t.Fatalf("expected %s cached under %s, got %q", command, id, got)
		}
		if source != "cache" {
			t.Fatalf("expected cache source for %s, got %q", command, source)
		}
	}
}

func TestLSPLanguageForPathSupportsRequestedLanguages(t *testing.T) {
	cases := map[string]struct {
		serverID   string
		languageID string
		mode       lspSymbolMode
	}{
		"index.html": {serverID: "html", languageID: "html", mode: lspSymbolModeDocument},
		"style.css":  {serverID: "css", languageID: "css", mode: lspSymbolModeSelector},
		"theme.scss": {serverID: "css", languageID: "scss", mode: lspSymbolModeSelector},
		"app.js":     {serverID: "typescript", languageID: "javascript", mode: lspSymbolModeCallable},
		"app.ts":     {serverID: "typescript", languageID: "typescript", mode: lspSymbolModeCallable},
		"main.go":    {serverID: "go", languageID: "go", mode: lspSymbolModeCallable},
		"Main.kt":    {serverID: "kotlin", languageID: "kotlin", mode: lspSymbolModeCallable},
	}
	for path, want := range cases {
		got, ok := lspLanguageForPath(path)
		if !ok {
			t.Fatalf("%s should be supported", path)
		}
		if got.ServerID != want.serverID || got.LanguageID != want.languageID || got.SymbolMode != want.mode {
			t.Fatalf("%s mapped to server=%q language=%q mode=%q, want %+v", path, got.ServerID, got.LanguageID, got.SymbolMode, want)
		}
	}
}

func TestRunLSPInstallAutoDryRunDetectsProjectLanguages(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()
	writeTestFile(t, root, "src/index.html", "<main id=\"app\"></main>\n")
	writeTestFile(t, root, "src/style.css", ".app { color: red; }\n")
	writeTestFile(t, root, "src/theme.scss", "$accent: red;\n")
	writeTestFile(t, root, "src/app.js", "export function installNeedleJs() {}\n")
	writeTestFile(t, root, "src/app.ts", "export function installNeedleTs() {}\n")
	writeTestFile(t, root, "cmd/app/main.go", "package main\n\nfunc installNeedleGo() {}\n")
	writeTestFile(t, root, "src/Main.kt", "fun installNeedleKotlin() {}\n")
	t.Setenv(lspCacheEnv, cache)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")
	t.Setenv("PATH", "")
	t.Chdir(root)

	var buf bytes.Buffer
	if err := runLSPInstall([]string{"auto", "--project", root, "--dry-run", "--json"}, &buf); err != nil {
		t.Fatal(err)
	}
	var results []lspInstallResult
	if err := json.NewDecoder(&buf).Decode(&results); err != nil {
		t.Fatalf("expected install dry-run JSON: %v\n%s", err, buf.String())
	}
	gotIDs := []string{}
	for _, result := range results {
		gotIDs = append(gotIDs, result.ID+":"+result.Status)
	}
	wantIDs := []string{"css:dry-run", "go:dry-run", "html:dry-run", "kotlin:dry-run", "typescript:dry-run"}
	if strings.Join(gotIDs, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("expected dry-run installs %v, got %+v", wantIDs, results)
	}
	for _, result := range results {
		if result.ID != "kotlin" && (!strings.Contains(result.Message, cache) || result.Message == "") {
			t.Fatalf("dry-run should include cache install command for %s, got %+v", result.ID, result)
		}
	}
}

func TestRunLSPInstallAliasesDryRun(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()
	t.Setenv(lspCacheEnv, cache)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")
	t.Setenv("PATH", "")
	t.Chdir(root)

	cases := map[string]string{
		"javascript": "typescript",
		"jsx":        "typescript",
		"js":         "typescript",
		"ts":         "typescript",
		"tsx":        "typescript",
		"golang":     "go",
		"scss":       "css",
		"sass":       "css",
		"kotlin":     "kotlin",
		"kt":         "kotlin",
	}
	for alias, wantID := range cases {
		var buf bytes.Buffer
		if err := runLSPInstall([]string{alias, "--project", root, "--dry-run", "--json"}, &buf); err != nil {
			t.Fatalf("%s alias should dry-run: %v\n%s", alias, err, buf.String())
		}
		var results []lspInstallResult
		if err := json.NewDecoder(&buf).Decode(&results); err != nil {
			t.Fatalf("expected JSON for %s: %v\n%s", alias, err, buf.String())
		}
		if len(results) != 1 || results[0].ID != wantID || results[0].Status != "dry-run" {
			t.Fatalf("%s alias resolved to %+v, want %s dry-run", alias, results, wantID)
		}
	}
}

func TestLSPSymbolModesAcceptMarkupAndStyleSymbols(t *testing.T) {
	html := lspLanguage{SymbolMode: lspSymbolModeDocument}
	css := lspLanguage{SymbolMode: lspSymbolModeSelector}
	goLang := lspLanguage{SymbolMode: lspSymbolModeCallable}

	if !lspSymbolIsResultNode(html, lspDocumentSymbol{Name: "main#app", Kind: 8}) {
		t.Fatal("html document mode should accept named document symbols")
	}
	if !lspSymbolIsResultNode(css, lspDocumentSymbol{Name: ".app", Kind: 5}) {
		t.Fatal("css selector mode should accept selector-like symbols")
	}
	if lspSymbolIsResultNode(goLang, lspDocumentSymbol{Name: "notCallable", Kind: 13}) {
		t.Fatal("callable mode should reject non-callable symbols")
	}
}

func TestRunLSPInstallExplicitJSONFailureReturnsError(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()
	t.Setenv(lspCacheEnv, cache)
	t.Setenv("PATH", "")
	t.Chdir(root)

	var buf bytes.Buffer
	err := runLSPInstall([]string{"typescript", "--project", root, "--json"}, &buf)
	if err == nil {
		t.Fatal("expected explicit install failure to return an error")
	}
	var results []lspInstallResult
	if decodeErr := json.NewDecoder(&buf).Decode(&results); decodeErr != nil {
		t.Fatalf("expected failure JSON to still be written: %v\n%s", decodeErr, buf.String())
	}
	if len(results) != 1 || results[0].ID != "typescript" || results[0].Status != "failed" {
		t.Fatalf("expected TypeScript failed install result, got %+v", results)
	}
}

func TestRunLSPInstallKotlinReturnsManualWhenMissing(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()
	t.Setenv(lspCacheEnv, cache)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", "")
	t.Chdir(root)

	var buf bytes.Buffer
	err := runLSPInstall([]string{"kotlin", "--project", root, "--json"}, &buf)
	if err == nil {
		t.Fatal("expected missing Kotlin LSP to require manual installation")
	}
	var results []lspInstallResult
	if decodeErr := json.NewDecoder(&buf).Decode(&results); decodeErr != nil {
		t.Fatalf("expected manual install JSON: %v\n%s", decodeErr, buf.String())
	}
	if len(results) != 1 || results[0].ID != "kotlin" || results[0].Status != "manual" || !strings.Contains(results[0].Message, "kotlin-lsp") {
		t.Fatalf("expected Kotlin manual install result, got %+v", results)
	}
}

func TestPreviewCodeGraphMissingLSPServerWarnsWithoutFailingSearch(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "main.go", "package demo\n\nfunc MissingServerNeedle() {}\n")
	t.Setenv("HOME", home)
	t.Setenv("GOPATH", "")
	t.Setenv("GOBIN", "")
	t.Setenv("PATH", "")

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/search?q=MissingServerNeedle")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("search should fail open when LSP is unavailable: %s", res.Status)
	}
	var search previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&search); err != nil {
		t.Fatal(err)
	}
	if len(search.Panels.CodeGraph) != 0 {
		t.Fatalf("missing LSP server should leave Code Graph empty, got %+v", search.Panels.CodeGraph)
	}
	var warned bool
	for _, warning := range search.Warnings {
		if strings.Contains(warning, "gopls not found") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("expected missing LSP warning, got %+v", search.Warnings)
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

func TestPreviewCodeSemanticEmbeddingRequiresKeywordEvidence(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "docs/search.md", "# Search\n\nSearch docs.\n")
	writeTestFile(t, root, "onboarding.go", `package demo

func StartOnboarding() {}
`)
	writeTestFile(t, root, "conference.go", `package demo

func ScheduleConference() {}
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
		for i := range req.Input {
			res.Data = append(res.Data, datum{Index: i, Embedding: []float32{1, 0, 0}})
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
    "noisy-code-test": {
      "provider": "preview-test",
      "model": "noisy-code-test",
      "dimensions": 3
    }
  },
  "defaultEmbeddingModel": "noisy-code-test"
}`, embedServer.URL+"/v1"))

	server := newPreviewServer(previewOptions{projectRoot: root, docsDir: "docs", addr: "127.0.0.1:0"})
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/search?q=onboarding")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var search previewSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&search); err != nil {
		t.Fatal(err)
	}
	if len(search.Panels.CodeSemantic) == 0 {
		t.Fatalf("expected code semantic keyword-backed result, got %+v", search.Panels.CodeSemantic)
	}
	for _, result := range search.Panels.CodeSemantic {
		if result.Path == "conference.go" {
			t.Fatalf("code semantic embedding result should require keyword evidence, got %+v", search.Panels.CodeSemantic)
		}
	}
}

func TestPreviewCodeSemanticFallbackRequiresKeywordEvidence(t *testing.T) {
	codeDocs := []codeSearchDoc{
		{
			ID:      "settings.go",
			Title:   "settings.go",
			Path:    "settings.go",
			Content: "package demo\n\nfunc ApplySettings() {}\n",
		},
		{
			ID:      "short.go",
			Title:   "short.go",
			Path:    "short.go",
			Content: "package demo\n\nfunc Set() {}\n",
		},
	}

	results := searchCodeSemantic(codeDocs, "settings", searchTokens("settings"), "hybrid", 8)
	if len(results) == 0 {
		t.Fatalf("expected keyword-backed code semantic result")
	}
	for _, result := range results {
		if result.Path == "short.go" {
			t.Fatalf("semantic fallback should not include fuzzy-only code results: %+v", results)
		}
	}
}

func TestPreviewCodeSymbolsCoverCommonLanguages(t *testing.T) {
	content := `
export const createSession = () => {}
class ProfileStore {
  refreshToken() {}
}
fun scheduleOnboarding() {}
public String loadCredential() { return ""; }
`
	symbols := codeSymbols(content)
	for _, want := range []string{"createSession", "ProfileStore", "refreshToken", "scheduleOnboarding", "loadCredential"} {
		if !containsString(symbols, want) {
			t.Fatalf("expected codeSymbols to include %s, got %+v", want, symbols)
		}
	}
}

func TestPreviewHelpIsAccepted(t *testing.T) {
	if err := Run([]string{"--help"}); err != nil {
		t.Fatalf("preview help failed: %v", err)
	}
}

func TestPreviewChildArgsPickAutoPortOnce(t *testing.T) {
	projectRoot := t.TempDir()
	args, err := previewChildArgs([]string{}, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(args[:2], " "); got != "--project "+projectRoot {
		t.Fatalf("preview child args should inject the caller project root, got %+v", args)
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
	projectRoot := t.TempDir()
	args, err := previewChildArgs([]string{"--project", ".", "--addr", "127.0.0.1:9999"}, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(args, " "); got != "--addr 127.0.0.1:9999 --project "+projectRoot {
		t.Fatalf("preview child args should preserve explicit addr, got %q", got)
	}
}

func TestPreviewChildArgsNormalizesExplicitProjectForSupervisor(t *testing.T) {
	projectRoot := t.TempDir()
	args, err := previewChildArgs([]string{"--project=.", "--open"}, projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(args, "--project") || !containsString(args, projectRoot) {
		t.Fatalf("preview child args should replace relative project with normalized caller project root: %+v", args)
	}
	if strings.Contains(strings.Join(args, " "), "--project=.") {
		t.Fatalf("preview child args should not preserve relative project for child running from module root: %+v", args)
	}
}

func TestPreviewSourceHotReloadTokenTracksBackendAndFrontend(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/preview\n")
	writeTestFile(t, root, "main.go", "package main\n")
	writeTestFile(t, root, "internal/preview/preview.go", "package preview\n")
	writeTestFile(t, root, "internal/preview/preview_ui/assets/index-old.js", "console.log('generated')\n")
	writeTestFile(t, root, "internal/preview/preview_ui_src/App.vue", "<script setup>const title = 'one'</script>\n")
	writeTestFile(t, root, "docs/guide.md", "# guide\n")

	nested := filepath.Join(root, "internal", "preview")
	if got, ok := previewModuleRoot(nested); !ok || got != root {
		t.Fatalf("previewModuleRoot(%q) = %q, %v; want %q, true", nested, got, ok, root)
	}
	initial := previewSourceToken(root)
	time.Sleep(time.Millisecond)
	writeTestFile(t, root, "internal/preview/preview_ui_src/App.vue", "<script setup>const title = 'two'</script>\n")
	if next := previewSourceToken(root); next == initial {
		t.Fatalf("frontend source change should update hot reload token")
	}
	frontendToken := previewSourceToken(root)
	time.Sleep(time.Millisecond)
	writeTestFile(t, root, "internal/preview/preview_ui/assets/index-new.js", "console.log('rebuilt')\n")
	if next := previewSourceToken(root); next != frontendToken {
		t.Fatalf("generated frontend assets should not trigger source restart: %q != %q", next, frontendToken)
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
		"../../tsconfig.vue.json",
		"../../eslint.config.mjs",
		"../../.prettierrc.json",
		"../../vite.config.ts",
		"preview_ui_src/index.html",
		"preview_ui_src/main.ts",
		"preview_ui_src/App.vue",
		"preview_ui_src/app.ts",
		"preview_ui_src/js/graph.ts",
		"preview_ui_src/js/internal-links.ts",
		"preview_ui_src/types.d.ts",
		"preview_ui/index.html",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("preview TypeScript toolchain missing %s: %v", path, err)
		}
	}
	pkg, err := os.ReadFile("../../package.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"build:preview", "check:preview", "lint:preview", "format:preview", "format:preview:check", "vue-tsc", "vite", "vue", "typescript", "eslint", "prettier"} {
		if !strings.Contains(string(pkg), want) {
			t.Fatalf("preview package scripts/deps missing %s", want)
		}
	}
}

func TestPreviewUIUsesDedicatedFrontendLibraries(t *testing.T) {
	text := previewUIText(t)
	for _, want := range []string{"cdn.tailwindcss.com", "daisyui", "lucide", "@toast-ui/editor", "toastui-editor-viewer", "DOMPurify", "highlight.js", "languages/go.min.js", "languages/typescript.min.js", "hljs.highlight", "mermaid.min.js", "mermaid.render", "svg-pan-zoom", "sigma@3.0.3", "graphology@0.26.0", "graphology-layout-forceatlas2@0.10.1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI missing %s integration", want)
		}
	}
	if strings.Contains(text, "/api/render/mermaid") {
		t.Fatalf("preview UI should render Mermaid client-side")
	}
	for _, forbidden := range []string{"data-ui-kit=\"treact\"", "Treact-style component primitives", "cytoscape"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("preview UI should not include %s", forbidden)
		}
	}
}

func TestPreviewUIRendersDocsGraphWithSigma(t *testing.T) {
	text := previewUIText(t)
	graphJS, err := os.ReadFile("preview_ui_src/js/graph.ts")
	if err != nil {
		t.Fatal(err)
	}
	networkGraphJS, err := os.ReadFile("preview_ui_src/js/network_graph.ts")
	if err != nil {
		t.Fatal(err)
	}
	graphViewer, err := os.ReadFile("preview_ui_src/components/GraphViewer.vue")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"data-tab=\"graph\"", "/main.ts", "id=\"graphCanvas\"", "fetchJSON(\"/api/graph\")", "createDocsGraph", "renderNetworkGraph", "Sigma", "forceAtlas2", "clickNode", "clickStage", "enterNode", "leaveNode", "forceLabel: true", "labelRenderedSizeThreshold: 0", "normalizedGraphData", "graphSelectedId", "graphRenderer", "graph-details", "openSpecPreview", "data-preview-spec", "openGraphNode", "clearGraphSelection", ".is-node-hover canvas"} {
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
	for _, want := range []string{"grid-template-columns: minmax(0, 1fr) minmax(18rem, 24rem)", "border-left: 1px solid hsl(var(--b3))", "max-height: 68vh"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview docs graph sidebar layout missing %s", want)
		}
	}
	for _, want := range []string{"renderNetworkGraph", "normalizedGraphData", "selectGraphNode", "clearGraphSelection", "edgeColorForTheme(props.theme)", "darkEdgeColor", "graphFullscreen", "toggleGraphFullscreen", `id="graphFullscreen"`, "is-fullscreen", "maximize", "minimize"} {
		if !strings.Contains(string(graphViewer), want) {
			t.Fatalf("Vue docs graph viewer missing %s", want)
		}
	}
	for _, forbidden := range []string{"x: 0,\n        y: 0", "await import(\"graphology\")", "await import(\"sigma\")", "data-preview-file", "openFilePreview"} {
		if strings.Contains(string(graphViewer), forbidden) {
			t.Fatalf("Vue docs graph viewer should use shared graph adapter instead of %s", forbidden)
		}
	}
}

func TestPreviewTopbarUsesIconOnlyTabs(t *testing.T) {
	text := previewUIText(t)
	for _, want := range []string{
		`aria-label="Preview sections"`,
		`data-tab="graph"`,
		`name="git-fork"`,
		`data-tab="search"`,
		`name="search"`,
		`id="themeToggle"`,
		`:data-theme-option="themePreference"`,
		"themeToggleIcon",
		"themeToggleLabel",
		"toggleTheme",
		`themePreference.value === "system" ? "dark" : themePreference.value === "dark" ? "light" : "system"`,
		"themePreference",
		"applyThemePreference",
		`localStorage.removeItem("spec-preview-theme")`,
		"project.projectRoot",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview topbar icon-only tabs missing %s", want)
		}
	}
	for _, forbidden := range []string{`data-tab="overview"`, `data-lucide="layout-dashboard"`, `data-tab="spec"`, "overviewTab", ">Overview</button>", ">Graph</button>", ">Search</button>", ">Doc</button>", "id=\"themeLabel\""} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("preview topbar should not render text label %s", forbidden)
		}
	}
	app, err := os.ReadFile("preview_ui_src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	themeToggleStart := strings.Index(string(app), `id="themeToggle"`)
	if themeToggleStart == -1 {
		t.Fatalf("preview topbar missing theme toggle")
	}
	themeToggleEnd := strings.Index(string(app)[themeToggleStart:], `</button>`)
	if themeToggleEnd == -1 {
		t.Fatalf("preview topbar theme toggle button is malformed")
	}
	themeToggleBlock := string(app)[themeToggleStart : themeToggleStart+themeToggleEnd]
	if strings.Contains(themeToggleBlock, "tab-active") {
		t.Fatalf("theme toggle should not render an active tab state")
	}
}

func TestPreviewUIRendersFourPanelSearchPage(t *testing.T) {
	text := previewUIText(t)
	searchPanel, err := os.ReadFile("preview_ui_src/components/SearchPanel.vue")
	if err != nil {
		t.Fatal(err)
	}
	networkGraphJS, err := os.ReadFile("preview_ui_src/js/network_graph.ts")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-tab="search"`,
		`id="searchTab"`,
		`data-search-panel="docsSemantic"`,
		`data-search-panel="docsGraph"`,
		`data-search-panel="codeSemantic"`,
		`data-search-panel="codeGraph"`,
		`data-search-domain-tab="docs"`,
		`data-search-domain-tab="code"`,
		`data-search-domain-panel="docs"`,
		`data-search-domain-panel="code"`,
		"activeSearchDomain",
		"docsResultCount",
		"codeResultCount",
		`aria-label="Keyword result operator"`,
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
		"fetch(path, { signal })",
		"const controller = new AbortController()",
		"searchController.value !== controller",
		"finally",
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
	for _, want := range []string{
		"renderSearchGraphPanel",
		"searchResultsToGraph",
		"renderNetworkGraph",
		"renderSearchGraphDetails",
		"selectSearchGraphNode",
		"clearSearchGraphSelection",
		`data-search-graph-shell="docsGraph"`,
		`data-search-graph-shell="codeGraph"`,
		`data-search-domain-tab="docs"`,
		`data-search-domain-tab="code"`,
		`data-search-domain-panel="docs"`,
		`data-search-domain-panel="code"`,
		"activeSearchDomain",
		"docsResultCount",
		"codeResultCount",
		`data-fullscreen-graph="docsGraph"`,
		`data-fullscreen-graph="codeGraph"`,
		"fullscreenSearchGraph",
		"toggleSearchGraphFullscreen",
		"is-fullscreen",
		"maximize",
		"minimize",
		`ref="docsGraphCanvas"`,
		`ref="codeGraphCanvas"`,
		"codeGraphNodeLabel",
		"neighborPath",
		"neighborLine",
		`result.path && panelName !== "codeGraph"`,
		`rootCallerBorderColor: theme.value === "dark" ? "#ffffff" : "#000000"`,
		"isRootCaller",
		"markSourceCallerNodes",
		"outgoing.has(node.id) && !incoming.has(node.id)",
		"codeGraphMemberLabel",
		"codeGraphNodeLabelText",
		"neighbor.label || neighbor.id || neighborID",
		"cleaned.lastIndexOf(separator)",
		"return `${owner}\\n${member}`",
		"data-preview-file",
		"data-preview-line",
	} {
		if !strings.Contains(string(searchPanel), want) {
			t.Fatalf("Vue preview search graph missing %s", want)
		}
	}
	for _, stale := range []string{"data-code-graph-view", "data-code-graph-flow", "codeGraphViewMode", "code-flow-shell"} {
		if strings.Contains(string(searchPanel), stale) {
			t.Fatalf("Vue Code Graph should render a single directed graph, found stale flow UI marker %s", stale)
		}
	}
	for _, want := range []string{"installRootCallerBorderOverlay", "network-graph-root-caller-border", "getNodeDisplayData", "framedGraphToViewport", "scaleSize", "context.strokeStyle = borderColor", "defaultDrawNodeLabel: drawNodeLabel", "multilineLabel(data.label)", ".split(/\\r?\\n/)"} {
		if !strings.Contains(string(networkGraphJS), want) {
			t.Fatalf("preview network graph missing expected rendering support %s", want)
		}
	}
	docsSemanticStart := strings.Index(string(searchPanel), `data-search-panel="docsSemantic"`)
	docsGraphStart := strings.Index(string(searchPanel), `data-search-panel="docsGraph"`)
	if docsSemanticStart == -1 || docsGraphStart == -1 {
		t.Fatalf("Vue preview search panel missing docs semantic or docs graph section")
	}
	docsSemanticBlock := string(searchPanel)[docsSemanticStart:docsGraphStart]
	if !strings.Contains(docsSemanticBlock, "openSpecPreview") || !strings.Contains(docsSemanticBlock, "openFilePreview") {
		t.Fatalf("Docs Semantic results should expose doc and repo Markdown/HTML file preview actions")
	}
	docsDomainStart := strings.Index(string(searchPanel), `data-search-domain-panel="docs"`)
	codeDomainStart := strings.Index(string(searchPanel), `data-search-domain-panel="code"`)
	if docsDomainStart == -1 || codeDomainStart == -1 || docsDomainStart > codeDomainStart {
		t.Fatalf("Vue preview search panel should render docs and code as separate ordered tabs")
	}
	docsDomainBlock := string(searchPanel)[docsDomainStart:codeDomainStart]
	if !strings.Contains(docsDomainBlock, `data-search-panel="docsSemantic"`) || !strings.Contains(docsDomainBlock, `data-search-panel="docsGraph"`) {
		t.Fatalf("Docs search tab should contain semantic and graph doc panels")
	}
	if strings.Contains(docsDomainBlock, `data-search-panel="codeSemantic"`) || strings.Contains(docsDomainBlock, `data-search-panel="codeGraph"`) {
		t.Fatalf("Docs search tab should not contain code panels")
	}
	codeDomainBlock := string(searchPanel)[codeDomainStart:]
	if !strings.Contains(codeDomainBlock, `data-search-panel="codeSemantic"`) || !strings.Contains(codeDomainBlock, `data-search-panel="codeGraph"`) {
		t.Fatalf("Code search tab should contain semantic and graph code panels")
	}
	graphDetailsStart := strings.Index(string(searchPanel), "function renderSearchGraphDetails")
	graphDetailsEnd := strings.Index(string(searchPanel), "function codeGraphNodeLabel")
	if graphDetailsStart == -1 || graphDetailsEnd == -1 {
		t.Fatalf("Vue preview search panel missing graph details renderer")
	}
	graphDetailsBlock := string(searchPanel)[graphDetailsStart:graphDetailsEnd]
	for _, want := range []string{"data-preview-file", "data-preview-line", "openFilePreview"} {
		if !strings.Contains(graphDetailsBlock, want) {
			t.Fatalf("Graph details should expose focused node file preview action %s", want)
		}
	}
	for _, want := range []string{"grid-template-columns: minmax(0, 1fr) minmax(16rem, 20rem)", "max-height: 22rem", ".graph-shell.is-fullscreen", ".search-graph-shell.is-fullscreen"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview search graph sidebar layout missing %s", want)
		}
	}
}

func TestPreviewVuePreviewModalRendersStyledDocs(t *testing.T) {
	previewModal, err := os.ReadFile("preview_ui_src/components/PreviewModal.vue")
	if err != nil {
		t.Fatal(err)
	}
	text := string(previewModal)
	for _, want := range []string{
		"renderPreviewSource",
		"renderPreviewContentCard",
		"renderPreviewMetadata",
		"preview-content-card",
		"data-preview-content",
		"preview-metadata",
		"renderMarkdownPreview",
		"loadToastMarkdownViewer",
		"renderHTMLPreview",
		"ensureHTMLMVPStylesheet",
		"scopeMVPStylesheet",
		`renderPreviewContentCard("", "html-doc")`,
		"markdown-wysiwyg-host markdown-toast-viewer",
		"DOMPurify.sanitize(raw || \"<p>No content.</p>\"",
		"decorateInternalDocNavigation(root, spec",
		"source.spec.description",
		"source.line ? String(source.line) : \"\"",
		"decorateCodePreviewLines(root)",
		"scrollPreviewToLine(root, source.line)",
		"data-line",
		"code-line-target",
		"scrollIntoView({ block: \"center\" })",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Vue preview modal styled doc rendering missing %s", want)
		}
	}
	if strings.Contains(text, `v-html="previewBody"`) || strings.Contains(text, "return props.source.raw") {
		t.Fatalf("Vue preview modal should not inject raw doc content as rendered preview")
	}
	css, err := os.ReadFile("preview_ui_src/public/style.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{".preview-content-card", "background: hsl(var(--b1))", ".preview-modal-body", "background: hsl(var(--b2) / 0.56)"} {
		if !strings.Contains(string(css), want) {
			t.Fatalf("preview modal content card style missing %s", want)
		}
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
	for _, want := range []string{"labelColor", "#f8fafc", "#0f172a", "edgeColorForTheme", "searchEdgeColorForTheme", "#94a3b8", "#f87171", ".graph-canvas canvas", ".search-graph-canvas canvas"} {
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
	if !strings.Contains(string(networkGraphJS), "const unfocusedColor = options.unfocusedEdgeColor || colorWithOpacity(data.color, 0.14)") ||
		!strings.Contains(string(networkGraphJS), "color: selected && !related ? unfocusedColor : data.color") {
		t.Fatalf("focused preview graph should dim unfocused edges without hiding them")
	}
	graphViewer, err := os.ReadFile("preview_ui_src/components/GraphViewer.vue")
	if err != nil {
		t.Fatal(err)
	}
	searchPanel, err := os.ReadFile("preview_ui_src/components/SearchPanel.vue")
	if err != nil {
		t.Fatal(err)
	}
	for name, source := range map[string]string{
		"legacy docs graph": string(graphJS),
		"Vue docs graph":    string(graphViewer),
		"legacy search":     string(app),
		"Vue search":        string(searchPanel),
	} {
		if !strings.Contains(source, `=== "dark" ? dark`) {
			t.Fatalf("%s should use dark edge palette when dark theme is enabled", name)
		}
		if !strings.Contains(source, `unfocusedEdgeColor: `) || !strings.Contains(source, `=== "dark" ? "#0f172a" : undefined`) {
			t.Fatalf("%s should use a solid dark unfocused edge color when dark theme is enabled", name)
		}
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
	for _, want := range []string{".markdown-wysiwyg-host .toastui-editor-contents", "TOAST UI ships fixed viewer colors", "color: hsl(var(--bc))", "blockquote p"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI Markdown theme contrast CSS missing %s", want)
		}
	}
}

func TestPreviewVueDocViewerRendersMetadataTables(t *testing.T) {
	docViewer, err := os.ReadFile("preview_ui_src/components/DocViewer.vue")
	if err != nil {
		t.Fatal(err)
	}
	text := string(docViewer)
	for _, want := range []string{
		"renderableMarkdownMetadata",
		"markdownBodyMetadata",
		"htmlMetadataRows",
		"renderMetadataTable",
		"metadata-table",
		"doc-meta",
		"metadata-link-badges",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview Vue doc viewer metadata rendering missing %s", want)
		}
	}
}

func TestPreviewVueDocViewerRendersHTMLDocMetadata(t *testing.T) {
	docViewer, err := os.ReadFile("preview_ui_src/components/DocViewer.vue")
	if err != nil {
		t.Fatal(err)
	}
	text := string(docViewer)
	for _, want := range []string{
		"DOMPurify.sanitize(raw || \"<p>No content.</p>\", htmlDocSanitizeConfig)",
		"ADD_TAGS: [\"doc-meta\", \"doc-title\", \"doc-description\", \"doc-relation\", \"doc-diagram\", \"doc-graph\"]",
		"ADD_ATTR: [\"status\", \"compliance\", \"priority\", \"version\", \"tone\", \"type\", \"target\", \"href\", \"language\"]",
		"const meta = root.querySelector(\"doc-meta\")",
		"meta.replaceWith(...metadata.childNodes)",
		"meta.querySelectorAll(\"a[href]\")",
		"meta.querySelectorAll(\"doc-relation[target]\")",
		"key: \"link\"",
		"key: `relation.${type}`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview Vue doc viewer HTML metadata rendering missing %s", want)
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
	for _, want := range []string{"renderHTMLPreview", "htmlDocSanitizeConfig", "htmlMVPStylesheetURL", "mvp.css@1.17.3", "scopeMVPStylesheet", "data-html-mvp-css", "html-doc", "doc-meta", "doc-title", "doc-description", "doc-relation", "doc-callout", "doc-diagram", "doc-code", "doc-section", "doc-grid", "doc-card", "doc-steps", "doc-step", "doc-flow", "doc-flow-step", "doc-graph", "doc-metrics", "doc-metric", "normalizeHTMLDocTags", "htmlMetadataRows", "normalizeDocDiagramLanguage", "language-c4-model", "replaceDocContainer", "replaceDocMetric", "createDocDiagramSource", "doc-relation-${typeClass}", "doc-code-block", "doc-diagram-source", "doc-graph-source", ".markdown-wysiwyg-shell", ".html-doc", ".html-doc table", ".html-doc a", ".doc-title", ".doc-description", ".doc-callout-info", ".doc-callout-warning", ".doc-relation-depends", ".doc-code-block::before", ".doc-steps", ".doc-flow-step", ".doc-metrics", ".doc-metric-value", ".hero-panel", ".metric-grid", ".insight-card", ".risk-panel", ".evidence-card.success", ".timeline-list"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI HTML doc rendering missing %s", want)
		}
	}
	if strings.Contains(text, "doc-link") {
		t.Fatalf("preview UI HTML doc link contract should use plain <a href> links only")
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
	for _, want := range []string{".markdown td code", ".markdown table th,\n.markdown table td", ".html-doc table th,\n.html-doc table td", ".metadata-table th,\n.metadata-table td", "text-align: left !important", "overflow-wrap: anywhere", "word-break: break-word", "overflow-x: auto"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview table CSS missing %s", want)
		}
	}
}

func TestPreviewSidebarIsFixedTreeWithIcons(t *testing.T) {
	text := previewUIText(t)
	for _, want := range []string{"lg:fixed", "buildSpecTree", "renderFolderNode", "folder-open", `name="file-text"`, "Icon.vue"} {
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
	for _, want := range []string{"validSpecFolderPath", "selectSpecFolder", "renderSpecFolderContent", "data-folder-path", "state.routeFolderPath"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI folder route handling missing %s", want)
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
	text := previewUIText(t)
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
	for _, want := range []string{"data-diagram-action=\"zoom-in\"", "data-diagram-action=\"zoom-out\"", "data-diagram-action=\"fit\"", "diagram-zoom-level", "Ctrl/Command-scroll to zoom", "viewportSelector: \".svg-pan-zoom_viewport\"", "zoomEnabled: true", "panEnabled: true", "mouseWheelZoomEnabled: false", "!event.ctrlKey && !event.metaKey", "event.preventDefault()", "instance.zoomAtPointBy", "zoomScaleSensitivity: 0.4", "instance.zoomIn()", "instance.zoomOut()", "instance.fit()", "instance.center()", "instance.resetZoom()", "instance.resetPan()"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview Mermaid svg-pan-zoom API missing %s", want)
		}
	}
	for _, forbidden := range []string{"beforeWheel", "zoomDiagramViewport", "fitDiagramViewport", "centerDiagramViewport", "pointerdown", "pointermove", "setPointerCapture", "view.stage.style.transform"} {
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
	diagrams, err := os.ReadFile("preview_ui_src/js/diagrams.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"svgDiagramSize", "svg.setAttribute(\"width\", String(size.width))", "svg.setAttribute(\"height\", String(size.height))", "svg.style.width = \"100%\"", "svg.style.height = \"100%\"", "svg.style.maxWidth = \"none\"", "svg.setAttribute(\"preserveAspectRatio\"", "svg.classList.add(\"diagram-svg\")", "prepareSvgPanZoomViewport", "svg-pan-zoom_viewport"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview Mermaid SVG library preparation missing %s", want)
		}
	}
	if !strings.Contains(string(diagrams), `svg.style.maxWidth = "none"`) {
		t.Fatalf("Vue preview diagram source should clear Mermaid inline max-width")
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
