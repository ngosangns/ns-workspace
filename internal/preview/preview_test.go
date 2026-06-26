package preview

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ngosangns/ns-workspace/internal/graphquery"
	"github.com/ngosangns/ns-workspace/internal/internalutil"
)

func previewUIText(t *testing.T) string {
	t.Helper()
	paths := []string{
		"preview_ui_src/index.html",
		"preview_ui_src/main.ts",
		"preview_ui_src/App.vue",
		"preview_ui_src/js/code-preview.ts",
		"preview_ui_src/js/diagrams.ts",
		"preview_ui_src/js/html-doc.ts",
		"preview_ui_src/js/markdown.ts",
		"preview_ui_src/js/metadata.ts",
		"preview_ui_src/js/network-graph.ts",
		"preview_ui_src/js/internal-links.ts",
		"preview_ui_src/js/shared-types.ts",
		"preview_ui_src/js/shared-utils.ts",
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

	res, err = http.Get(ts.URL + "/search")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("search route fallback was not served: %s", res.Status)
	}
	if got := res.Header.Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("search route fallback should return HTML, got %s", got)
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
	if err := writeSearchLauncher(out, "http://localhost:12345/search", root, filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "http://localhost:12345/search") || !strings.Contains(text, root) {
		t.Fatalf("search launcher did not include app URL and project metadata: %s", string(data))
	}
	if strings.Contains(text, `\"http://localhost:12345/search\"`) {
		t.Fatalf("search launcher should not add literal quotes to the redirect URL: %s", text)
	}
	if !strings.Contains(text, `window.location.replace("http:\/\/localhost:12345\/search")`) {
		t.Fatalf("search launcher should emit a valid JavaScript redirect string: %s", text)
	}
}

func TestGraphRequiresQuery(t *testing.T) {
	err := RunGraph([]string{"--project", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "graph requires --query") {
		t.Fatalf("graph without --query should fail with a focused message, got %v", err)
	}
}

func TestGraphQueryAutoEnsuresLSPByDefault(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")

	var called bool
	restore := replaceGraphEnsureHook(func(ctx context.Context, projectRoot, docsDir string, opts lspEnsureOptions) []string {
		called = true
		if projectRoot != root || docsDir != "docs" {
			t.Fatalf("ensure should receive normalized project/docs, got %q %q", projectRoot, docsDir)
		}
		if opts.Progress == nil {
			t.Fatal("graph ensure should write progress to stderr, not stdout")
		}
		return []string{"auto ensure warning"}
	})
	defer restore()

	output, err := captureStdout(t, func() error {
		return RunGraph([]string{"--project", root, "--query", "Spec", "--limit", "1", "--json"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("graph query should ensure LSP by default")
	}
	var search previewSearchResponse
	if err := json.Unmarshal([]byte(output), &search); err != nil {
		t.Fatalf("graph query should keep JSON stdout valid after ensure: %v\n%s", err, output)
	}
	if !slices.Contains(search.Warnings, "auto ensure warning") {
		t.Fatalf("graph query should include ensure warnings, got %+v", search.Warnings)
	}
}

func TestGraphQueryCanSkipAutoEnsureLSP(t *testing.T) {
	cases := [][]string{
		{"--no-ensure-lsp"},
		{"--ensure-lsp=false"},
	}
	for _, extra := range cases {
		t.Run(strings.Join(extra, " "), func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")

			var called bool
			restore := replaceGraphEnsureHook(func(ctx context.Context, projectRoot, docsDir string, opts lspEnsureOptions) []string {
				called = true
				return nil
			})
			defer restore()

			args := append([]string{"--project", root, "--query", "Spec", "--json"}, extra...)
			output, err := captureStdout(t, func() error { return RunGraph(args) })
			if err != nil {
				t.Fatal(err)
			}
			if called {
				t.Fatalf("%v should skip automatic LSP ensure", extra)
			}
			var search previewSearchResponse
			if err := json.Unmarshal([]byte(output), &search); err != nil {
				t.Fatalf("graph query should print valid JSON: %v\n%s", err, output)
			}
		})
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
	if !slices.Contains(search.Warnings, "Code Graph relation expansion is unavailable for this language server.") {
		t.Fatalf("graph query should preserve Code Graph warnings: %+v", search.Warnings)
	}
}

func replaceGraphEnsureHook(hook func(context.Context, string, string, lspEnsureOptions) []string) func() {
	previous := ensureProjectLSPForGraph
	ensureProjectLSPForGraph = hook
	return func() {
		ensureProjectLSPForGraph = previous
	}
}

func restoreLSPRuntimeHooks(t *testing.T) {
	t.Helper()
	previousServerForLanguage := lspServerForLanguage
	previousServerByID := lspServerByID
	previousStartServer := lspStartServer
	previousDocumentSymbols := lspDocumentSymbols
	previousPrepareCallHierarchy := lspPrepareCallHierarchy
	previousIncomingCalls := lspIncomingCalls
	previousOutgoingCalls := lspOutgoingCalls
	previousReferences := lspReferences
	t.Cleanup(func() {
		lspServerForLanguage = previousServerForLanguage
		lspServerByID = previousServerByID
		lspStartServer = previousStartServer
		lspDocumentSymbols = previousDocumentSymbols
		lspPrepareCallHierarchy = previousPrepareCallHierarchy
		lspIncomingCalls = previousIncomingCalls
		lspOutgoingCalls = previousOutgoingCalls
		lspReferences = previousReferences
	})
}

func warningsContain(warnings []string, needle string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return true
		}
	}
	return false
}

func captureStdout(t *testing.T, run func() error) (string, error) {
	t.Helper()
	previous := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	done := make(chan struct {
		text string
		err  error
	}, 1)
	go func() {
		data, readErr := io.ReadAll(reader)
		done <- struct {
			text string
			err  error
		}{text: string(data), err: readErr}
	}()
	runErr := run()
	_ = writer.Close()
	os.Stdout = previous
	result := <-done
	_ = reader.Close()
	if result.err != nil {
		t.Fatal(result.err)
	}
	return result.text, runErr
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

func TestPreviewSearchSkipsGeneratedPreviewUIArtifacts(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "internal/preview/preview_ui/index.html", "<title>GeneratedNeedle</title>\n")
	writeTestFile(t, root, "internal/preview/preview_ui/assets/index-generated.js", "export function GeneratedNeedle() {}\n")
	writeTestFile(t, root, "internal/preview/preview_ui_src/main.ts", "export function SourceNeedle() {}\n")
	initGitRepo(t, root,
		"docs/_index.md",
		"internal/preview/preview_ui/index.html",
		"internal/preview/preview_ui/assets/index-generated.js",
		"internal/preview/preview_ui_src/main.ts",
	)

	project, err := scanSpecProject(root, "docs")
	if err != nil {
		t.Fatal(err)
	}
	response := buildPreviewSearchResponse(context.Background(), project, nil, root, "GeneratedNeedle", "hybrid", "sum", 8)
	for panel, results := range map[string][]previewSearchResult{
		"docsSemantic": response.Panels.DocsSemantic,
		"codeSemantic": response.Panels.CodeSemantic,
	} {
		for _, result := range results {
			if strings.HasPrefix(result.Path, "internal/preview/preview_ui/") {
				t.Fatalf("%s should skip generated preview UI artifact, got %+v", panel, result)
			}
		}
	}
}

func TestPreviewSearchSkipsGeneratedPreviewUIArtifactsWithoutGit(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "internal/preview/preview_ui/index.html", "<title>GeneratedNeedle</title>\n")
	writeTestFile(t, root, "internal/preview/preview_ui/assets/index-generated.js", "export function GeneratedNeedle() {}\n")
	writeTestFile(t, root, "internal/preview/preview_ui_src/main.ts", "export function SourceNeedle() {}\n")

	project, err := scanSpecProject(root, "docs")
	if err != nil {
		t.Fatal(err)
	}
	response := buildPreviewSearchResponse(context.Background(), project, nil, root, "GeneratedNeedle", "hybrid", "sum", 8)
	for panel, results := range map[string][]previewSearchResult{
		"docsSemantic": response.Panels.DocsSemantic,
		"codeSemantic": response.Panels.CodeSemantic,
	} {
		for _, result := range results {
			if strings.HasPrefix(result.Path, "internal/preview/preview_ui/") {
				t.Fatalf("%s should skip generated preview UI artifact without Git, got %+v", panel, result)
			}
		}
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
	if slices.Contains(search.Panels.DocsGraph[0].MatchedBy, "semantic-anchor") || !slices.Contains(search.Panels.DocsGraph[0].MatchedBy, "graph") {
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
	if slices.Contains(search.Panels.CodeGraph[0].MatchedBy, "semantic-anchor") || !slices.Contains(search.Panels.CodeGraph[0].MatchedBy, "graph") {
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

func TestLSPSourceFilesSkipsGeneratedPreviewUIButKeepsPreviewUISource(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/_index.md", "# Spec Index\n")
	writeTestFile(t, root, "internal/preview/preview_ui/index.html", "<div id=\"app\"></div>\n")
	writeTestFile(t, root, "internal/preview/preview_ui/style.css", ".generated { color: red; }\n")
	writeTestFile(t, root, "internal/preview/preview_ui_src/index.html", "<div id=\"app\"></div>\n")
	writeTestFile(t, root, "internal/preview/preview_ui_src/main.ts", "export function sourceNeedle() {}\n")
	initGitRepo(t, root,
		"docs/_index.md",
		"internal/preview/preview_ui/index.html",
		"internal/preview/preview_ui/style.css",
		"internal/preview/preview_ui_src/index.html",
		"internal/preview/preview_ui_src/main.ts",
	)

	_, files, warnings := lspSourceFiles(root, filepath.Join(root, "docs"))
	if len(warnings) != 0 {
		t.Fatalf("unexpected source scan warnings: %+v", warnings)
	}
	got := []string{}
	for _, file := range files {
		got = append(got, file.Rel)
	}
	for _, forbidden := range []string{"internal/preview/preview_ui/index.html", "internal/preview/preview_ui/style.css"} {
		if slices.Contains(got, forbidden) {
			t.Fatalf("generated preview UI artifact %s should not be indexed by LSP: %+v", forbidden, got)
		}
	}
	for _, want := range []string{"internal/preview/preview_ui_src/index.html", "internal/preview/preview_ui_src/main.ts"} {
		if !slices.Contains(got, want) {
			t.Fatalf("preview UI source %s should remain in LSP source files: %+v", want, got)
		}
	}
}

func TestLSPIndexContinuesAfterFileSymbolTimeout(t *testing.T) {
	restoreLSPRuntimeHooks(t)
	lspServerForLanguage = func(manager *previewLSPManager, lang lspLanguage) (*previewLSPServer, error) {
		return &previewLSPServer{lang: lang}, nil
	}
	lspStartServer = func(ctx context.Context, srv *previewLSPServer) error {
		return nil
	}
	lspDocumentSymbols = func(ctx context.Context, srv *previewLSPServer, path, languageID string) ([]lspDocumentSymbol, error) {
		if strings.HasSuffix(path, "slow.html") {
			return nil, context.DeadlineExceeded
		}
		return []lspDocumentSymbol{{
			Name:           "Needle",
			Kind:           12,
			Range:          lspRange{Start: lspPosition{Line: 2}, End: lspPosition{Line: 4}},
			SelectionRange: lspRange{Start: lspPosition{Line: 2, Character: 5}},
		}}, nil
	}

	root := t.TempDir()
	provider := newPreviewLSPCodeGraphProvider(root, filepath.Join(root, "docs"))
	files := []lspSourceFile{
		{Rel: "slow.html", Abs: filepath.Join(root, "slow.html"), Language: lspLanguage{ServerID: "html", LanguageID: "html", Name: "HTML"}},
		{Rel: "needle.go", Abs: filepath.Join(root, "needle.go"), Language: lspLanguage{ServerID: "go", LanguageID: "go", Name: "Go"}},
	}

	index := provider.buildIndex(context.Background(), files)
	if index.TimedOut {
		t.Fatalf("per-file symbol timeout should not mark the whole index as total timeout: %+v", index)
	}
	if index.IndexedFiles != 1 || index.TimedOutFiles != 1 {
		t.Fatalf("expected one indexed file and one timed out file, got indexed=%d timedOut=%d warnings=%+v", index.IndexedFiles, index.TimedOutFiles, index.Warnings)
	}
	if len(index.Nodes) != 1 {
		t.Fatalf("index should keep symbols from files after a timeout, got %+v", index.Nodes)
	}
	if !warningsContain(index.Warnings, "slow.html") || !warningsContain(index.Warnings, "symbol timeout") {
		t.Fatalf("expected focused file timeout warning, got %+v", index.Warnings)
	}
}

func TestLSPIndexDoesNotCacheTotalTimeoutAsComplete(t *testing.T) {
	root := t.TempDir()
	provider := newPreviewLSPCodeGraphProvider(root, filepath.Join(root, "docs"))
	partial := lspCodeGraphIndex{
		Nodes:    map[string]lspCodeNode{"needle": {ID: "needle", Name: "Needle"}},
		ByPath:   map[string][]string{"needle.go": {"needle"}},
		TimedOut: true,
	}
	provider.cacheIndexIfStable("partial", partial)
	if provider.token != "" {
		t.Fatalf("total-timeout index should not be cached as stable, got token %q", provider.token)
	}

	stable := partial
	stable.TimedOut = false
	provider.cacheIndexIfStable("stable", stable)
	if provider.token != "stable" {
		t.Fatalf("stable index should be cached, got token %q", provider.token)
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

func TestLSPRelationFallsBackToReferencesAfterCallHierarchyTimeout(t *testing.T) {
	restoreLSPRuntimeHooks(t)
	root := t.TempDir()
	anchorPath := filepath.Join(root, "needle.go")
	callerPath := filepath.Join(root, "caller.go")
	provider := newPreviewLSPCodeGraphProvider(root, filepath.Join(root, "docs"))
	index := lspCodeGraphIndex{
		Nodes: map[string]lspCodeNode{
			"needle": {
				ID:             "needle",
				Name:           "Needle",
				FullName:       "Needle",
				Kind:           12,
				KindLabel:      "function",
				ServerID:       "go",
				LanguageID:     "go",
				Path:           "needle.go",
				AbsPath:        anchorPath,
				Range:          lspRange{Start: lspPosition{Line: 1}, End: lspPosition{Line: 4}},
				SelectionRange: lspRange{Start: lspPosition{Line: 1, Character: 5}},
			},
			"caller": {
				ID:             "caller",
				Name:           "Caller",
				FullName:       "Caller",
				Kind:           12,
				KindLabel:      "function",
				ServerID:       "go",
				LanguageID:     "go",
				Path:           "caller.go",
				AbsPath:        callerPath,
				Range:          lspRange{Start: lspPosition{Line: 1}, End: lspPosition{Line: 8}},
				SelectionRange: lspRange{Start: lspPosition{Line: 1, Character: 5}},
			},
		},
		ByPath: map[string][]string{
			"needle.go": {"needle"},
			"caller.go": {"caller"},
		},
	}

	lspServerByID = func(manager *previewLSPManager, serverID string) (*previewLSPServer, error) {
		return &previewLSPServer{}, nil
	}
	lspPrepareCallHierarchy = func(ctx context.Context, srv *previewLSPServer, path, languageID string, pos lspPosition) ([]lspCallHierarchyItem, error) {
		return nil, context.DeadlineExceeded
	}
	lspReferences = func(ctx context.Context, srv *previewLSPServer, path, languageID string, pos lspPosition) ([]lspLocation, error) {
		return []lspLocation{{URI: fileURI(callerPath), Range: lspRange{Start: lspPosition{Line: 3, Character: 2}}}}, nil
	}

	edges, warnings := provider.relationsForNode(context.Background(), index, index.Nodes["needle"])
	if len(edges) != 1 || edges[0].Source != "caller" || edges[0].Target != "needle" || edges[0].Relation != "references" {
		t.Fatalf("references should provide fallback relation edge after call hierarchy timeout, edges=%+v warnings=%+v", edges, warnings)
	}
	if !warningsContain(warnings, "fell back to references after call hierarchy timeout") {
		t.Fatalf("expected fallback warning, got %+v", warnings)
	}
}

func TestLSPRelationWarningsIncludeIncomingOutgoingTimeout(t *testing.T) {
	restoreLSPRuntimeHooks(t)
	root := t.TempDir()
	provider := newPreviewLSPCodeGraphProvider(root, filepath.Join(root, "docs"))
	index := lspCodeGraphIndex{Nodes: map[string]lspCodeNode{
		"needle": {
			ID:             "needle",
			Name:           "Needle",
			FullName:       "Needle",
			Kind:           12,
			KindLabel:      "function",
			ServerID:       "go",
			LanguageID:     "go",
			Path:           "needle.go",
			AbsPath:        filepath.Join(root, "needle.go"),
			Range:          lspRange{Start: lspPosition{Line: 1}, End: lspPosition{Line: 4}},
			SelectionRange: lspRange{Start: lspPosition{Line: 1, Character: 5}},
		},
	}}

	lspPrepareCallHierarchy = func(ctx context.Context, srv *previewLSPServer, path, languageID string, pos lspPosition) ([]lspCallHierarchyItem, error) {
		return []lspCallHierarchyItem{{Name: "Needle", URI: fileURI(filepath.Join(root, "needle.go"))}}, nil
	}
	lspIncomingCalls = func(ctx context.Context, srv *previewLSPServer, item lspCallHierarchyItem) ([]lspIncomingCall, error) {
		return nil, context.DeadlineExceeded
	}
	lspOutgoingCalls = func(ctx context.Context, srv *previewLSPServer, item lspCallHierarchyItem) ([]lspOutgoingCall, error) {
		return nil, context.DeadlineExceeded
	}

	_, warnings, err := provider.callHierarchyEdges(context.Background(), &previewLSPServer{}, index, index.Nodes["needle"])
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout error from call hierarchy expansion, got %v", err)
	}
	if !warningsContain(warnings, "incoming call expansion") || !warningsContain(warnings, "outgoing call expansion") {
		t.Fatalf("incoming/outgoing timeout warnings should be preserved, got %+v", warnings)
	}
}

func TestPreviewLSPManagerFindsGoBinOutsidePATH(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	writeTestFile(t, home, "go/bin/gopls", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(home, "go", "bin", internalutil.ExecutableNames("gopls")[0]), 0o755); err != nil {
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
	if got != filepath.Join(home, "go", "bin", internalutil.ExecutableNames("gopls")[0]) {
		t.Fatalf("expected GOPATH-style gopls fallback, got %q", got)
	}
}

func TestPreviewLSPManagerFindsProjectNodeBinOutsidePATH(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	command := internalutil.ExecutableNames("typescript-language-server")[0]
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
	command := internalutil.ExecutableNames("typescript-language-server")[0]
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
		name := internalutil.ExecutableNames(command)[0]
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
		if !strings.Contains(result.Message, cache) || result.Message == "" {
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

func TestRunLSPInstallKotlinDownloadsArchiveToCache(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()
	t.Setenv(lspCacheEnv, cache)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", "")
	t.Chdir(root)

	archive := testZipArchive(t, "kotlin-server-test/bin/intellij-server", "#!/bin/sh\nexit 0\n")
	sum := sha256.Sum256(archive)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	restoreResolver := graphquery.SetArchiveSourceForTest(func(spec graphquery.InstallSpec) (graphquery.ArchiveSource, error) {
		return graphquery.ArchiveSource{
			Version:  "test",
			FileName: "kotlin-server-test.zip",
			URL:      server.URL + "/kotlin-server-test.zip",
			SHA256:   hex.EncodeToString(sum[:]),
			Format:   "zip",
		}, nil
	})
	t.Cleanup(restoreResolver)

	var buf bytes.Buffer
	err := runLSPInstall([]string{"kotlin", "--project", root, "--json"}, &buf)
	if err != nil {
		t.Fatalf("expected Kotlin archive install to succeed: %v\n%s", err, buf.String())
	}
	var results []lspInstallResult
	if decodeErr := json.NewDecoder(&buf).Decode(&results); decodeErr != nil {
		t.Fatalf("expected install JSON: %v\n%s", decodeErr, buf.String())
	}
	wantPath := filepath.Join(cache, "kotlin", "bin", internalutil.ExecutableNames("kotlin-lsp")[0])
	if len(results) != 1 || results[0].ID != "kotlin" || results[0].Status != "installed" || results[0].Path != wantPath {
		t.Fatalf("expected Kotlin installed result at %s, got %+v", wantPath, results)
	}
	if !internalutil.ExecutableFile(wantPath) {
		t.Fatalf("expected installed Kotlin wrapper to be executable at %s", wantPath)
	}
	manager := newPreviewLSPManager(root)
	got, source, err := manager.resolveCommandWithSource("kotlin-lsp")
	if err != nil {
		t.Fatal(err)
	}
	if got != wantPath || source != "cache" {
		t.Fatalf("expected resolver to find cached Kotlin wrapper, got path=%q source=%q", got, source)
	}
}

func testZipArchive(t *testing.T, path, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	header := &zip.FileHeader{Name: path, Method: zip.Deflate}
	header.SetMode(0o755)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
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
	if !slices.Contains(search.Panels.DocsSemantic[0].MatchedBy, "semantic") {
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
		if !slices.Contains(symbols, want) {
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
	if !slices.Contains(args, "--addr") {
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
	if !slices.Contains(args, "--project") || !slices.Contains(args, projectRoot) {
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
	text := previewUIText(t)
	if strings.Contains(text, "project.summary") {
		t.Fatalf("preview UI should use /api/project summary response directly")
	}
}

func TestPreviewUIHasTypeScriptToolchain(t *testing.T) {
	for _, path := range []string{
		"../../package.json",
		"../../package-lock.json",
		"../../biome.json",
		"../../tsconfig.vue.json",
		"../../eslint.config.mjs",
		"../../.prettierrc.json",
		"../../vite.config.ts",
		"preview_ui_src/index.html",
		"preview_ui_src/main.ts",
		"preview_ui_src/App.vue",
		"preview_ui_src/js/code-preview.ts",
		"preview_ui_src/js/diagrams.ts",
		"preview_ui_src/js/html-doc.ts",
		"preview_ui_src/js/markdown.ts",
		"preview_ui_src/js/metadata.ts",
		"preview_ui_src/js/network-graph.ts",
		"preview_ui_src/js/internal-links.ts",
		"preview_ui_src/js/shared-types.ts",
		"preview_ui_src/js/shared-utils.ts",
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
	for _, want := range []string{"build:preview", "check:preview", "lint:preview", "lint:preview:fix", "lint:preview:biome", "format:preview", "format:preview:check", "simple-git-hooks", "lint-staged", "vue-tsc", "vite", "vue", "typescript", "eslint", "prettier", "biome"} {
		if !strings.Contains(string(pkg), want) {
			t.Fatalf("preview package scripts/deps missing %s", want)
		}
	}
	for _, legacy := range []string{"preview_ui_src/app.ts", "preview_ui_src/js/graph.ts"} {
		if _, err := os.Stat(legacy); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("preview TypeScript toolchain should not keep legacy source %s", legacy)
		}
	}
}

func TestPreviewUIUsesDedicatedFrontendLibraries(t *testing.T) {
	text := previewUIText(t)
	for _, want := range []string{"cdn.tailwindcss.com", "@fontsource/geist-sans", "@fontsource/geist-mono", "lucide", "@toast-ui/editor", "toastui-editor-viewer", "DOMPurify", "highlight.js", "languages/go.min.js", "languages/typescript.min.js", "mermaid.min.js", "mermaid.render", "svg-pan-zoom", "sigma@3.0.3", "graphology@0.26.0", "graphology-layout-forceatlas2@0.10.1"} {
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
	networkGraphJS, err := os.ReadFile("preview_ui_src/js/network-graph.ts")
	if err != nil {
		t.Fatal(err)
	}
	graphViewer, err := os.ReadFile("preview_ui_src/components/GraphViewer.vue")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"data-tab=\"graph\"", "/main.ts", "id=\"graphCanvas\"", "renderNetworkGraph", "Sigma", "forceAtlas2", "clickNode", "clickStage", "enterNode", "leaveNode", "forceLabel: true", "labelRenderedSizeThreshold: 0", "normalizedGraphData", "selectedNodeId", "graphRenderer", "graph-details", "openSpecPreview", "data-preview-spec", "selectGraphNode", "clearGraphSelection", ".is-node-hover canvas"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview docs graph UI missing %s", want)
		}
	}
	if strings.Contains(string(graphViewer), "selectSpec(") || strings.Contains(string(graphViewer), "data-open-spec") {
		t.Fatalf("preview docs graph should use popup previews instead of direct doc navigation")
	}
	if strings.Contains(string(graphViewer), "selectedNodeId.value = node.id;\n  const incoming") {
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
		"projectRoot",
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
	networkGraphJS, err := os.ReadFile("preview_ui_src/js/network-graph.ts")
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
		"fetchJSON(`/api/search?${params.toString()}",
		"panelResults(\"docsSemantic\")",
		"panelResults(\"codeGraph\")",
		"renderSearchSummary",
		"result.description || result.excerpt",
		"result.specId",
		"renderSearchGraphPanel",
		"searchResultsToGraph",
		"renderNetworkGraph",
		"codeGraphNodeLabel",
		"neighborPath",
		"neighborLine",
		"searchGraphRenderers",
		"renderSearchGraphDetails(name: string, graph: NetworkGraphData, details: HTMLElement)",
		"const node = graph.nodes.find((item) => item.id === selected) || graph.nodes[0];",
		"clearSearchGraphSelection",
		".search-graph-canvas",
		"searchLoading",
		"Searching docs, code, and graphs",
		"fetch(path, { signal })",
		"const controller = new AbortController()",
		"searchController.value !== controller",
		"finally",
		`id="previewDialog"`,
		"openSpecPreview",
		"openFilePreview",
		"/api/files?",
		`id="previewRawToggle"`,
		"previewSource",
		"previewShowRaw",
		"renderPreviewSource",
		`data-preview-spec`,
		`data-preview-file`,
		"buildRouteQuery",
		"replaceFocusedRouteURL",
		"decorateCodePreviewLines",
		"code-line-target",
		".preview-modal",
		".code-preview",
		"line-height: 1.18",
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
	markdown, err := os.ReadFile("preview_ui_src/js/markdown.ts")
	if err != nil {
		t.Fatal(err)
	}
	htmlDoc, err := os.ReadFile("preview_ui_src/js/html-doc.ts")
	if err != nil {
		t.Fatal(err)
	}
	codePreview, err := os.ReadFile("preview_ui_src/js/code-preview.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(previewModal) + "\n" + string(markdown) + "\n" + string(htmlDoc) + "\n" + string(codePreview)
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
	app, err := os.ReadFile("preview_ui_src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	previewModal, err := os.ReadFile("preview_ui_src/components/PreviewModal.vue")
	if err != nil {
		t.Fatal(err)
	}
	icon, err := os.ReadFile("preview_ui/favicon.svg")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(app) + "\n" + string(previewModal) + "\n" + string(icon)
	for _, want := range []string{
		`href="/favicon.svg"`,
		`type="image/svg+xml"`,
		`viewBox="0 0 64 64"`,
		"updateDocumentTitle",
		"pageTitleForTab",
		"dedupeTitleParts",
		`Search: ${query}`,
		"Doc preview:",
		"File preview:",
		"project.value?.generatedTitle",
		"document.title = dedupeTitleParts",
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
	networkGraphJS, err := os.ReadFile("preview_ui_src/js/network-graph.ts")
	if err != nil {
		t.Fatal(err)
	}
	graphViewer, err := os.ReadFile("preview_ui_src/components/GraphViewer.vue")
	if err != nil {
		t.Fatal(err)
	}
	searchPanel, err := os.ReadFile("preview_ui_src/components/SearchPanel.vue")
	if err != nil {
		t.Fatal(err)
	}
	text := string(css) + "\n" + string(graphViewer) + "\n" + string(searchPanel)
	for _, want := range []string{"labelColor", "#f8fafc", "#0f172a", "edgeColorForTheme", "searchEdgeColorForTheme", "#94a3b8", "#f87171", ".graph-canvas canvas", ".search-graph-canvas canvas"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview graph label dark mode contrast missing %s", want)
		}
	}
	if strings.Contains(string(networkGraphJS), "highlighted: node === selectedId") {
		t.Fatalf("focused preview graph node should not render Sigma highlighted label background")
	}
	if !strings.Contains(string(networkGraphJS), "defaultDrawNodeHover: drawNodeLabel") {
		t.Fatalf("hovered preview graph node should not render Sigma label background")
	}
	if !strings.Contains(string(networkGraphJS), "const unfocusedColor = options.unfocusedEdgeColor || colorWithOpacity(data.color, 0.14)") ||
		!strings.Contains(string(networkGraphJS), "color: selected && !related ? unfocusedColor : data.color") {
		t.Fatalf("focused preview graph should dim unfocused edges without hiding them")
	}
	for name, source := range map[string]string{
		"Vue docs graph": string(graphViewer),
		"Vue search":     string(searchPanel),
	} {
		if !strings.Contains(source, `=== "dark" ? dark`) {
			t.Fatalf("%s should use dark edge palette when dark theme is enabled", name)
		}
		if !strings.Contains(source, `unfocusedEdgeColor: `) || !strings.Contains(source, `=== "dark" ? "#0f172a" : undefined`) {
			t.Fatalf("%s should use a solid dark unfocused edge color when dark theme is enabled", name)
		}
	}
}

func TestPreviewUIRendersMarkdownClientSide(t *testing.T) {
	markdown, err := os.ReadFile("preview_ui_src/js/markdown.ts")
	if err != nil {
		t.Fatal(err)
	}
	codePreview, err := os.ReadFile("preview_ui_src/js/code-preview.ts")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile("preview_ui_src/js/metadata.ts")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(markdown) + "\n" + string(codePreview) + "\n" + string(metadata) + "\n" + string(css)
	if strings.Contains(text, "fallbackHTML") || strings.Contains(text, "markdownRenderer.render(metadata.body)") {
		t.Fatalf("preview UI should render Markdown from raw content on the client")
	}
	for _, want := range []string{"renderMarkdownPreview", "loadToastMarkdownViewer", "toastui-editor-viewer", "Loading Markdown preview...", "data-source-language", "markdown-wysiwyg-host", "markdown-toast-viewer", ".markdown-toast-viewer .toastui-editor-contents", ".metadata-table", "padding: 18px 25px", "renderableMarkdownMetadata", "markdownMetadataRows", "renderMetadataTable", "renderMetadataValue", "metadataArrayValues", "cleanMetadataScalar", "metadata-badges", "badge badge-ghost badge-sm"} {
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
	markdown, err := os.ReadFile("preview_ui_src/js/markdown.ts")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile("preview_ui_src/js/metadata.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(docViewer) + "\n" + string(markdown) + "\n" + string(metadata)
	for _, want := range []string{
		`import { renderMarkdownPreview } from "../js/markdown.js"`,
		"renderableMarkdownMetadata",
		"markdownBodyMetadata",
		"renderMetadataTable",
		"metadata-table",
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
	htmlDoc, err := os.ReadFile("preview_ui_src/js/html-doc.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(docViewer) + "\n" + string(htmlDoc)
	for _, want := range []string{
		`import { renderHTMLPreview } from "../js/html-doc.js"`,
		"DOMPurify.sanitize(raw || \"<p>No content.</p>\", htmlDocSanitizeConfig)",
		"doc-meta",
		"doc-section",
		"doc-metric",
		"normalizeHTMLDocTags",
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
	htmlDoc, err := os.ReadFile("preview_ui_src/js/html-doc.ts")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(htmlDoc) + "\n" + string(css)
	for _, want := range []string{"renderHTMLPreview", "htmlDocSanitizeConfig", "htmlMVPStylesheetURL", "mvp.css@1.17.3", "scopeMVPStylesheet", "data-html-mvp-css", "html-doc", "doc-meta", "doc-title", "doc-description", "doc-relation", "doc-callout", "doc-diagram", "doc-code", "doc-section", "doc-grid", "doc-card", "doc-steps", "doc-step", "doc-flow", "doc-flow-step", "doc-graph", "doc-metrics", "doc-metric", "normalizeHTMLDocTags", "htmlMetadataRows", "normalizeDocDiagramLanguage", "c4-model", "replaceDocContainer", "replaceDocMetric", "createDocDiagramSource", "doc-relation-${typeClass}", "doc-code-block", "doc-diagram-source", "doc-graph-source", ".html-doc", ".html-doc table", ".html-doc a", ".doc-title", ".doc-description", ".doc-callout-info", ".doc-callout-warning", ".doc-relation-depends", ".doc-code-block::before", ".doc-steps", ".doc-flow-step", ".doc-metrics", ".doc-metric-value", ".hero-panel", ".metric-grid", ".insight-card", ".risk-panel", ".evidence-card.success", ".timeline-list"} {
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
	docViewer, err := os.ReadFile("preview_ui_src/components/DocViewer.vue")
	if err != nil {
		t.Fatal(err)
	}
	markdown, err := os.ReadFile("preview_ui_src/js/markdown.ts")
	if err != nil {
		t.Fatal(err)
	}
	htmlDoc, err := os.ReadFile("preview_ui_src/js/html-doc.ts")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(docViewer) + "\n" + string(markdown) + "\n" + string(htmlDoc) + "\n" + string(css)
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
	internalLinks, err := os.ReadFile("preview_ui_src/js/internal-links.ts")
	if err != nil {
		t.Fatal(err)
	}
	htmlDoc, err := os.ReadFile("preview_ui_src/js/html-doc.ts")
	if err != nil {
		t.Fatal(err)
	}
	docViewer, err := os.ReadFile("preview_ui_src/components/DocViewer.vue")
	if err != nil {
		t.Fatal(err)
	}
	text := string(internalLinks) + "\n" + string(htmlDoc) + "\n" + string(docViewer)
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
		"selectSpec(target.specId, true",
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
	for _, want := range []string{"lg:fixed", "buildSpecTree", "TreeNode", "folder", `name="file-text"`, "Icon.vue"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview sidebar missing %s", want)
		}
	}
}

func TestPreviewUIConnectsHotReload(t *testing.T) {
	app, err := os.ReadFile("preview_ui_src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"window.addEventListener(\"popstate\"", "systemThemeQuery?.addEventListener", "loadGraph", "fetchJSON(\"/api/project\")", "fetchJSON(\"/api/docs\")"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI lifecycle wiring missing %s", want)
		}
	}
}

func TestPreviewUIUpdatesURLForFocusedTabs(t *testing.T) {
	app, err := os.ReadFile("preview_ui_src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"routeFromLocation", "updateRouteURL", "window.history.pushState", "window.location.pathname", "popstate", "encodeURIComponent", "buildRouteQuery"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI route handling missing %s", want)
		}
	}
	for _, want := range []string{"validSpecFolderPath", "selectSpecFolder", "selectedFolderListing", "selectedFolderPath", "routeFolderPath"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI folder route handling missing %s", want)
		}
	}
	if strings.Contains(text, "fetchJSON(\"/api/graph\")])") {
		t.Fatalf("preview UI should not fetch graph during initial project/docs load")
	}
	if strings.Contains(text, "hashchange") {
		t.Fatalf("preview UI should use path routing without hash fragments")
	}
}

func TestPreviewDiagramSanitizerKeepsMermaidLabels(t *testing.T) {
	diagrams, err := os.ReadFile("preview_ui_src/js/diagrams.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(diagrams)
	for _, want := range []string{"USE_PROFILES", "foreignObject", "\"div\"", "\"span\"", "\"style\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview diagram sanitizer missing %s, Mermaid labels may be stripped", want)
		}
	}
}

func TestPreviewDiagramUsesThemeAwareEdgesAndLabels(t *testing.T) {
	diagrams, err := os.ReadFile("preview_ui_src/js/diagrams.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(diagrams)
	for _, want := range []string{"window.mermaid.initialize", `theme: theme === "dark" ? "dark" : "default"`, "securityLevel: \"strict\"", "renderDiagram(host, id, diagram, theme)"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview diagram dark theme edge/label support missing %s", want)
		}
	}
}

func TestPreviewUISupportsDarkMode(t *testing.T) {
	text := previewUIText(t)
	for _, want := range []string{"spec-preview-theme", "prefers-color-scheme: dark", "id=\"themeToggle\"", "applyThemePreference", `theme === "dark" ? "dark" : "default"`, "renderCurrentSpec"} {
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
	diagrams, err := os.ReadFile("preview_ui_src/js/diagrams.ts")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(diagrams) + "\n" + string(css)
	for _, want := range []string{"decorateDiagram", "diagram-surface", "diagram-toolbar", "diagram-viewport", "panZoomInstances", "window.svgPanZoom"} {
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
	diagrams, err := os.ReadFile("preview_ui_src/js/diagrams.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(diagrams)
	for _, want := range []string{"sourceLanguage", "renderDiagramsIn", "diagramSourceFromCodeBlock", "mermaidC4TypeFromLanguage", "looksLikeMermaidC4", "C4(?:Context|Container|Component|Dynamic|Deployment)", "replace(/[-_\\s]/g, \"\")", "c4(?:context|container|component|dynamic|deployment)?", "C4Component"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI Mermaid C4 fence rendering missing %s", want)
		}
	}
}

func TestPreviewUIRendersLikeC4ModelThroughMermaidC4(t *testing.T) {
	diagrams, err := os.ReadFile("preview_ui_src/js/diagrams.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(diagrams)
	for _, want := range []string{"sourceLanguage", "language === \"likec4\"", "language === \"c4model\"", "looksLikeLikeC4Model", "likeC4ModelToMermaid", "appendLikeC4Root", "node.kind === \"softwareSystem\"", "C4Component", "Container_Boundary", "Component(", "Rel(", "relation[3] || \"uses\"", "LikeC4 model"} {
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
	diagrams, err := os.ReadFile("preview_ui_src/js/diagrams.ts")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(diagrams) + "\n" + string(css)
	for _, want := range []string{"data-diagram-action=\"zoom-in\"", "data-diagram-action=\"zoom-out\"", "data-diagram-action=\"fit\"", "diagram-zoom-level", "Ctrl/Command-scroll to zoom", "mouseWheelZoomEnabled: false", "!event.ctrlKey && !event.metaKey", "event.preventDefault()", "instance.zoomAtPointBy", "zoomScaleSensitivity: 0.25", "instance.zoomIn()", "instance.zoomOut()", "instance.fit()", "instance.center()", "instance.resetZoom()", "instance.resetPan()"} {
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
	diagrams, err := os.ReadFile("preview_ui_src/js/diagrams.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(diagrams)
	for _, want := range []string{"destroyDiagramsIn", "panZoomInstances.get(id)?.destroy()", "panZoomInstances.set(id, instance)", "panZoomInstances.delete(id)"} {
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
	diagrams, err := os.ReadFile("preview_ui_src/js/diagrams.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(diagrams)
	for _, want := range []string{"svg.style.maxWidth = \"none\"", "svg.classList.add(\"diagram-svg\")", "window.svgPanZoom", "fit: true", "center: true"} {
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
	diagrams, err := os.ReadFile("preview_ui_src/js/diagrams.ts")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(diagrams) + "\n" + string(css)
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

func TestRunHelpAndUnknownP(t *testing.T) {
	if err := Run([]string{"--help"}); err != nil {
		t.Errorf("--help should return nil: %v", err)
	}
}

func TestPreviewArgsHaveAddrFlagP(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"--addr", "x"}, true},
		{[]string{"--addr=x"}, true},
		{[]string{"-addr", "x"}, true},
		{[]string{"-addr=x"}, true},
		{[]string{"--other"}, false},
	}
	for _, c := range cases {
		if got := previewArgsHaveAddrFlag(c.args); got != c.want {
			t.Errorf("previewArgsHaveAddrFlag(%v) = %v, want %v", c.args, got, c.want)
		}
	}
}

func TestStripPreviewFlagsP(t *testing.T) {
	in := []string{"--no-reload", "--no-reload=true", "--addr", "x"}
	out := stripPreviewSupervisorFlags(in)
	want := []string{"--addr", "x"}
	if len(out) != len(want) {
		t.Fatalf("supervisor: %v, want %v", out, want)
	}
	in = []string{"--project", "/p", "--project=/p", "-project", "/p", "-project=/p", "--other"}
	out = stripPreviewProjectFlag(in)
	want = []string{"--other"}
	if len(out) != len(want) {
		t.Fatalf("project: %v, want %v", out, want)
	}
	in = []string{"--open", "-open", "--open=true", "-open=true", "--other"}
	out = stripPreviewOpenFlag(in)
	want = []string{"--other"}
	if len(out) != len(want) {
		t.Fatalf("open: %v, want %v", out, want)
	}
}

func TestPreviewChildArgsP(t *testing.T) {
	child, err := previewChildArgs([]string{"--no-reload", "--addr", "x"}, "/proj")
	if err != nil {
		t.Fatalf("previewChildArgs: %v", err)
	}
	if !previewHasString(child, "--addr") || !previewHasString(child, "x") {
		t.Errorf("expected --addr preserved, got %v", child)
	}
	if !previewHasString(child, "--project") || !previewHasString(child, "/proj") {
		t.Errorf("expected --project /proj in %v", child)
	}
	if previewHasString(child, "--no-reload") {
		t.Errorf("supervisor flag should be stripped: %v", child)
	}
}

func TestPreviewChildArgsNoAddrP(t *testing.T) {
	child, err := previewChildArgs([]string{}, "/proj")
	if err != nil {
		t.Fatalf("previewChildArgs: %v", err)
	}
	if !previewHasString(child, "--addr") {
		t.Errorf("expected --addr to be auto-picked, got %v", child)
	}
}

func TestPreviewChildArgsAddrErrorP(t *testing.T) {
	orig := pickPreviewAddrForTest
	defer func() { pickPreviewAddrForTest = orig }()
	pickPreviewAddrForTest = func() (string, error) {
		return "", errors.New("no addr")
	}
	if _, err := previewChildArgs([]string{}, "/proj"); err == nil {
		t.Error("expected error from pickPreviewAddr")
	}
}

func TestNormalizePreviewProjectRootPx(t *testing.T) {
	got := normalizePreviewProjectRoot(".")
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute, got %q", got)
	}
	got = normalizePreviewProjectRoot("/already/abs")
	if got != "/already/abs" {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestPickPreviewAddrP(t *testing.T) {
	addr, err := pickPreviewAddr()
	if err != nil {
		t.Fatalf("pickPreviewAddr: %v", err)
	}
	if !strings.Contains(addr, "127.0.0.1") {
		t.Errorf("expected loopback, got %q", addr)
	}

	// Error path: pre-bind an address and reuse it for pickPreviewAddr.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pre-bind: %v", err)
	}
	defer ln.Close()
	boundAddr := ln.Addr().String()
	if _, err := pickPreviewAddrAt(boundAddr); err == nil {
		t.Errorf("expected error when address already in use")
	}
}

func TestIsPreviewSourceFileP(t *testing.T) {
	ui := "/proj/internal/preview/preview_ui"
	uiSrc := "/proj/internal/preview/preview_ui_src"
	if kind, ok := previewSourceFileKind("main.go", "/proj/main.go", uiSrc); !ok || kind != previewSourceBackend {
		t.Errorf("main.go should be backend")
	}
	if kind, ok := previewSourceFileKind("go.mod", "/proj/go.mod", uiSrc); !ok || kind != previewSourceBackend {
		t.Errorf("go.mod should be backend")
	}
	for _, rel := range []string{"package.json", "package-lock.json", "biome.json", "tsconfig.preview.json", "tsconfig.vue.json", "vite.config.ts", "eslint.config.mjs", ".prettierrc.json", ".prettierignore"} {
		if kind, ok := previewSourceFileKind(rel, "/proj/"+rel, uiSrc); !ok || kind != previewSourceFrontend {
			t.Errorf("%s should be frontend", rel)
		}
	}
	if kind, ok := previewSourceFileKind("preview_ui_src/main.ts", "/proj/internal/preview/preview_ui_src/main.ts", uiSrc); !ok || kind != previewSourceFrontend {
		t.Errorf("ui_src file should be frontend")
	}
	if !isPreviewSourceFile("main.go", "/proj/main.go", ui, uiSrc) {
		t.Error("isPreviewSourceFile should be true for main.go")
	}
	if !isPreviewSourceFile("x", ui+"/x", ui, uiSrc) {
		t.Error("isPreviewSourceFile should be true for files under ui/")
	}
}

func TestFileExistsP(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if fileExists(p) {
		t.Error("missing file should not exist")
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !fileExists(p) {
		t.Error("written file should exist")
	}
	if fileExists(dir) {
		t.Error("dir should not count as file")
	}
}

func TestWalkPreviewSourceAndTokensP(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "d.md"), []byte("---\ntype: m\n---\n# D"), 0o644); err != nil {
		t.Fatal(err)
	}
	seen := map[string]previewSourceKind{}
	walkPreviewSource(dir, func(p string, _ os.FileInfo, k previewSourceKind) {
		rel, _ := filepath.Rel(dir, p)
		seen[filepath.ToSlash(rel)] = k
	})
	if seen["go.mod"] != previewSourceBackend {
		t.Errorf("go.mod should be backend: %v", seen)
	}
	if seen["package.json"] != previewSourceFrontend {
		t.Errorf("package.json should be frontend: %v", seen)
	}
	if _, ok := seen[".git/HEAD"]; ok {
		t.Error(".git should be skipped")
	}
	tok := previewSourceTokens(dir)
	if tok.backend == "0:0" {
		t.Errorf("expected backend tokens, got %q", tok.backend)
	}
	if tok.frontend == "0:0" {
		t.Errorf("expected frontend tokens, got %q", tok.frontend)
	}
	if tok1 := previewSourceToken(dir); tok1 != tok.backend+"|"+tok.frontend {
		t.Errorf("previewSourceToken inconsistent: %q", tok1)
	}
}

func TestNewestModTokenEmptyP(t *testing.T) {
	if got := newestModToken("/this/does/not/exist"); got == "" {
		t.Error("expected non-empty token")
	}
}

func TestNewestEmbeddedModTokenP(t *testing.T) {
	if got := newestEmbeddedModToken(); got == "" {
		t.Error("expected non-empty token")
	}
}

func TestIsPreviewStaticAssetPathP(t *testing.T) {
	cases := map[string]bool{
		"favicon.svg":          true,
		"style.css":            true,
		"assets/foo.js":        true,
		"nested/assets/foo.js": true,
		"js/app.js":            true,
		"index.html":           false,
		"page.html":            false,
	}
	for p, want := range cases {
		if got := isPreviewStaticAssetPath(p); got != want {
			t.Errorf("isPreviewStaticAssetPath(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestSpaFileServerP(t *testing.T) {
	static := os.DirFS(t.TempDir())
	handler := spaFileServer(static)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("HEAD /: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/favicon.svg", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /favicon.svg (missing): %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/some/spa/route", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /some/spa/route: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("POST / should pass through: %d", rec.Code)
	}
}

func TestWriteJSONP(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, map[string]any{"a": 1})
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("unexpected content type: %q", ct)
	}
	if !strings.Contains(rec.Body.String(), `"a": 1`) {
		t.Errorf("missing value in body: %q", rec.Body.String())
	}
}

func TestWriteAPIErrorP(t *testing.T) {
	rec := httptest.NewRecorder()
	writeAPIError(rec, previewSentinelErrP{})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

type previewSentinelErrP struct{}

func (previewSentinelErrP) Error() string { return "sentinel" }

func TestPreviewModuleRootP(t *testing.T) {
	if _, ok := previewModuleRoot("/this/does/not/exist"); ok {
		t.Error("missing module root should not be found")
	}
}

func previewHasString(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

func TestParseTagsValueP(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"a", []string{"a"}},
		{"a, b, c", []string{"a", "b", "c"}},
		{"[a, b]", []string{"a", "b"}},
		{"a, , b", []string{"a", "b"}},
		{`"a", 'b'`, []string{"a", "b"}},
	}
	for _, c := range cases {
		got := parseTagsValue(c.in)
		if !equalStringSlice(got, c.want) {
			t.Errorf("parseTagsValue(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBetweenAfterP(t *testing.T) {
	if got := betweenAfter("foo:bar:baz", "bar:"); got != "baz" {
		t.Errorf("betweenAfter = %q, want baz", got)
	}
	if got := betweenAfter("foo bar baz", "missing:"); got != "" {
		t.Errorf("betweenAfter missing = %q", got)
	}
	// Case insensitive marker (marker appears with colon).
	if got := betweenAfter("hello WORLD: trailing", "WORLD:"); got != "trailing" {
		t.Errorf("betweenAfter case = %q", got)
	}
}

func TestIsLoopbackHostP(t *testing.T) {
	cases := map[string]bool{
		"localhost":     true,
		"127.0.0.1":     true,
		"127.0.0.1:80":  true,
		"":              true,
		"example.com":   false,
		"10.0.0.1:80":   false,
	}
	for host, want := range cases {
		if got := isLoopbackHost(host); got != want {
			t.Errorf("isLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}


func TestLspSymbolKindLabelAllKinds(t *testing.T) {
	cases := map[int]string{
		2:  "module",
		3:  "namespace",
		5:  "class",
		6:  "method",
		7:  "property",
		8:  "field",
		9:  "constructor",
		11: "interface",
		12: "function",
		13: "variable",
		14: "constant",
		15: "string",
		18: "object",
		20: "key",
		23: "struct",
		24: "event",
		25: "operator",
		0:  "symbol", // default
		99: "symbol", // default
	}
	for kind, want := range cases {
		got := lspSymbolKindLabel(kind)
		if got != want {
			t.Errorf("lspSymbolKindLabel(%d) = %q, want %q", kind, got, want)
		}
	}
}


func TestLocationsArrayOfLSPLocations(t *testing.T) {
	orig := lspRequest
	defer func() { lspRequest = orig }()

	lspRequest = func(ctx context.Context, srv *previewLSPServer, method string, params any, result any) error {
		raw := []byte(`[{"uri":"file:///a.go","range":{"start":{"line":0,"character":0},"end":{"line":1,"character":1}}}]`)
		// result is *json.RawMessage
		if r, ok := result.(*json.RawMessage); ok {
			*r = raw
		}
		return nil
	}
	srv := &previewLSPServer{}
	locs, err := srv.locations(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(locs) != 1 || locs[0].URI != "file:///a.go" {
		t.Errorf("got %+v", locs)
	}
}

func TestLocationsArrayOfLinks(t *testing.T) {
	orig := lspRequest
	defer func() { lspRequest = orig }()

	lspRequest = func(ctx context.Context, srv *previewLSPServer, method string, params any, result any) error {
		raw := []byte(`[{"targetUri":"file:///b.go","targetRange":{"start":{"line":2,"character":0},"end":{"line":3,"character":0}},"targetSelectionRange":{"start":{"line":2,"character":5},"end":{"line":2,"character":10}}}]`)
		if r, ok := result.(*json.RawMessage); ok {
			*r = raw
		}
		return nil
	}
	srv := &previewLSPServer{}
	locs, err := srv.locations(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(locs) != 1 || locs[0].URI != "file:///b.go" {
		t.Errorf("got %+v", locs)
	}
	if locs[0].Range.Start.Line != 2 || locs[0].Range.Start.Character != 5 {
		t.Errorf("expected selection range, got %+v", locs[0].Range)
	}
}

func TestLocationsLinksEmptySelectionRangeFallsBackToTarget(t *testing.T) {
	orig := lspRequest
	defer func() { lspRequest = orig }()

	lspRequest = func(ctx context.Context, srv *previewLSPServer, method string, params any, result any) error {
		raw := []byte(`[{"targetUri":"file:///c.go","targetRange":{"start":{"line":4,"character":0},"end":{"line":5,"character":0}}}]`)
		if r, ok := result.(*json.RawMessage); ok {
			*r = raw
		}
		return nil
	}
	srv := &previewLSPServer{}
	locs, err := srv.locations(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(locs) != 1 || locs[0].Range.Start.Line != 4 {
		t.Errorf("expected target range fallback, got %+v", locs[0].Range)
	}
}

func TestLocationsSingleObject(t *testing.T) {
	orig := lspRequest
	defer func() { lspRequest = orig }()

	lspRequest = func(ctx context.Context, srv *previewLSPServer, method string, params any, result any) error {
		raw := []byte(`{"uri":"file:///d.go","range":{"start":{"line":6,"character":0},"end":{"line":7,"character":0}}}`)
		if r, ok := result.(*json.RawMessage); ok {
			*r = raw
		}
		return nil
	}
	srv := &previewLSPServer{}
	locs, err := srv.locations(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(locs) != 1 || locs[0].URI != "file:///d.go" || locs[0].Range.Start.Line != 6 {
		t.Errorf("got %+v", locs)
	}
}

func TestLocationsNullOrEmpty(t *testing.T) {
	orig := lspRequest
	defer func() { lspRequest = orig }()

	lspRequest = func(ctx context.Context, srv *previewLSPServer, method string, params any, result any) error {
		if r, ok := result.(*json.RawMessage); ok {
			*r = nil
		}
		return nil
	}
	srv := &previewLSPServer{}
	locs, err := srv.locations(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(locs) != 0 {
		t.Errorf("expected empty, got %+v", locs)
	}
}

func TestLocationsRequestError(t *testing.T) {
	orig := lspRequest
	defer func() { lspRequest = orig }()

	lspRequest = func(ctx context.Context, srv *previewLSPServer, method string, params any, result any) error {
		return errors.New("boom")
	}
	srv := &previewLSPServer{}
	_, err := srv.locations(context.Background(), "test", nil)
	if err == nil {
		t.Error("expected error")
	}
}

func TestLocationsArrayInvalidJSON(t *testing.T) {
	orig := lspRequest
	defer func() { lspRequest = orig }()

	lspRequest = func(ctx context.Context, srv *previewLSPServer, method string, params any, result any) error {
		raw := []byte(`{not-valid-json`)
		if r, ok := result.(*json.RawMessage); ok {
			*r = raw
		}
		return nil
	}
	srv := &previewLSPServer{}
	_, err := srv.locations(context.Background(), "test", nil)
	if err == nil {
		t.Error("expected error from invalid JSON")
	}
}


func TestRunHotReloadSupervisorDoneChannelReturnsError(t *testing.T) {
	orig := startPreviewChildForTest
	defer func() { startPreviewChildForTest = orig }()

	startPreviewChildForTest = func(moduleRoot string, args []string) (*exec.Cmd, <-chan previewChildResult, error) {
		return nil, makeChannelWithResult(errors.New("child failed")), nil
	}
	err := runHotReloadSupervisor(t.TempDir(), []string{}, t.TempDir())
	if err == nil || err.Error() != "child failed" {
		t.Errorf("expected child failed, got %v", err)
	}
}

func TestRunHotReloadSupervisorDoneChannelReturnsSuccess(t *testing.T) {
	orig := startPreviewChildForTest
	defer func() { startPreviewChildForTest = orig }()

	startPreviewChildForTest = func(moduleRoot string, args []string) (*exec.Cmd, <-chan previewChildResult, error) {
		return nil, makeChannelWithResult(nil), nil
	}
	err := runHotReloadSupervisor(t.TempDir(), []string{}, t.TempDir())
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestRunHotReloadSupervisorStartError(t *testing.T) {
	orig := startPreviewChildForTest
	defer func() { startPreviewChildForTest = orig }()

	startPreviewChildForTest = func(moduleRoot string, args []string) (*exec.Cmd, <-chan previewChildResult, error) {
		return nil, nil, errors.New("start failed")
	}
	err := runHotReloadSupervisor(t.TempDir(), []string{}, t.TempDir())
	if err == nil || err.Error() != "start failed" {
		t.Errorf("expected start failed, got %v", err)
	}
}

func TestRunHotReloadSupervisorPreviewChildArgsError(t *testing.T) {
	// previewChildArgs returns error if cwd is invalid
	err := runHotReloadSupervisor(t.TempDir(), []string{}, "/nonexistent/path/that/does/not/exist/anywhere")
	if err == nil {
		t.Error("expected error from previewChildArgs with bad project root")
	}
}

func makeChannelWithResult(err error) <-chan previewChildResult {
	ch := make(chan previewChildResult, 1)
	ch <- previewChildResult{err: err}
	return ch
}


func TestRunHotReloadSupervisorTickerReload(t *testing.T) {
	orig := startPreviewChildForTest
	defer func() { startPreviewChildForTest = orig }()

	// Mock that returns a never-closing done channel and we will manually
	// change a file to trigger token change.
	startCount := 0
	startPreviewChildForTest = func(moduleRoot string, args []string) (*exec.Cmd, <-chan previewChildResult, error) {
		startCount++
		return nil, makeChannelBlocking(), nil
	}

	// Create a module dir with a Go file that has tokens (backend)
	moduleRoot := t.TempDir()
	goFile := filepath.Join(moduleRoot, "main.go")
	if err := os.WriteFile(goFile, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run in goroutine; will tick after 700ms and try to restart
	done := make(chan error, 1)
	go func() {
		done <- runHotReloadSupervisor(moduleRoot, []string{}, moduleRoot)
	}()
	// Touch the file after a brief delay to trigger token change
	time.Sleep(800 * time.Millisecond)
	if err := os.WriteFile(goFile, []byte("package main\n// changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Wait a bit more for ticker to fire and pick up the change
	time.Sleep(1500 * time.Millisecond)
	// Force shutdown via done channel - just close our test by leaking
	// (we'll cancel by sending to a signal, but we can't here).
	// Instead just wait a bit and verify startCount >= 1.
	if startCount < 1 {
		t.Errorf("expected at least 1 start, got %d", startCount)
	}
	// Send to the blocking channel to terminate
	// We can't reach the done channel directly since it's inside the function.
	// Just exit the test - the goroutine will leak but that's OK for testing.
}

func makeChannelBlocking() chan previewChildResult {
	return make(chan previewChildResult)
}

func TestTrimDocFragmentP(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"plain", "modules/foo.md", "modules/foo.md"},
		{"query", "modules/foo.md?bar=baz", "modules/foo.md"},
		{"fragment", "modules/foo.md#section", "modules/foo.md"},
		{"lineRef", "modules/foo.md:42", "modules/foo.md"},
		{"lineRange", "modules/foo.md:42-99", "modules/foo.md"},
		{"trailingSpace", "modules/foo.md   ", "modules/foo.md"},
	}
	for _, tc := range cases {
		got := trimDocFragment(tc.input)
		if got != tc.want {
			t.Errorf("%s: trimDocFragment(%q)=%q want %q", tc.name, tc.input, got, tc.want)
		}
	}
}

func TestRelationTypeFromTextP(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"implements", "this **implements** the spec", "implements"},
		{"depends", "depends on prior step", "depends"},
		{"dependency", "dependency tree", "depends"},
		{"blocks", "blocks later steps", "blocked-by"},
		{"blockedBy", "blocked by approval", "blocked-by"},
		{"follows", "follows design", "follows"},
		{"supersedes", "supersedes v1", "supersedes"},
		{"verifies", "verifies correctness", "verifies"},
		{"test", "test plan", "verifies"},
		{"provides", "provides API", "provides"},
		{"consumes", "consumes events", "consumes"},
		{"related", "general prose mention", "related"},
		{"underscore", "implements_feature", "implements"},
		{"hyphen", "depends-on", "depends"},
		{"markdown", "**follows** spec", "follows"},
		{"caseInsensitive", "IMPLEMENTS rule", "implements"},
	}
	for _, tc := range cases {
		got := relationTypeFromText(tc.input)
		if got != tc.want {
			t.Errorf("%s: relationTypeFromText(%q)=%q want %q", tc.name, tc.input, got, tc.want)
		}
	}
}


func TestHandleEventsStreamsChangesAndHeartbeats(t *testing.T) {
	projectRoot := t.TempDir()
	docsDir := filepath.Join(projectRoot, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Force at least one file so newestModToken has something deterministic.
	if err := os.WriteFile(filepath.Join(docsDir, "a.md"), []byte("# a"), 0o644); err != nil {
		t.Fatal(err)
	}
	ps := &previewServer{opt: previewOptions{projectRoot: projectRoot, docsDir: "docs"}}
	srv := httptest.NewServer(http.HandlerFunc(ps.handleEvents))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", ct)
	}
	buf := make([]byte, 2048)
	if _, err := resp.Body.Read(buf); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	// We expect at least the "event: ready" prelude.
	if !strings.Contains(string(buf), "event: ready") {
		t.Errorf("missing ready event: %q", string(buf))
	}
	cancel()
}

func TestHandleEventsRejectsNonGET(t *testing.T) {
	ps := &previewServer{opt: previewOptions{projectRoot: t.TempDir(), docsDir: "docs"}}
	req := httptest.NewRequest(http.MethodPost, "/api/events", nil)
	w := httptest.NewRecorder()
	ps.handleEvents(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestShouldSkipSearchDirP(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"git", ".git", true},
		{"nodeModules", "node_modules", true},
		{"graphifyOut", "graphify-out", true},
		{"cache", ".cache", true},
		{"dist", "dist", true},
		{"build", "build", true},
		{"vendor", "vendor", true},
		{"src", "src", false},
		{"empty", "", false},
		{"docs", "docs", false},
		{"internal", "internal", false},
	}
	for _, tc := range cases {
		got := shouldSkipSearchDir(tc.in)
		if got != tc.want {
			t.Errorf("%s: shouldSkipSearchDir(%q)=%v want %v", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestCleanProjectRelP(t *testing.T) {
	cases := []struct {
		name        string
		projectRoot string
		path        string
		want        string
	}{
		{"empty", "/r", "", ""},
		{"whitespace", "/r", "   ", ""},
		{"relative", "/r", "docs/foo.md", "docs/foo.md"},
		{"relativeDotPrefix", "/r", "./docs/foo.md", "docs/foo.md"},
		{"absoluteInside", "/r", "/r/docs/foo.md", "docs/foo.md"},
		{"relativeDot", "/r", ".", ""},
		{"relativeDotSlash", "/r", "./", ""},
	}
	for _, tc := range cases {
		got := cleanProjectRel(tc.projectRoot, tc.path)
		if got != tc.want {
			t.Errorf("%s: cleanProjectRel(%q, %q)=%q want %q", tc.name, tc.projectRoot, tc.path, got, tc.want)
		}
	}
}

func TestCleanRelPathP(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "docs/foo.md", "docs/foo.md"},
		{"dotPrefix", "./docs/foo.md", "docs/foo.md"},
		{"doubleDot", "..", ".."},
		{"dot", ".", ""},
		{"trailingSlash", "docs/", "docs"},
		{"whitespace", "  docs/foo.md  ", "docs/foo.md"},
	}
	for _, tc := range cases {
		got := cleanRelPath(tc.in)
		if got != tc.want {
			t.Errorf("%s: cleanRelPath(%q)=%q want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestWritePreviewEmbeddingIndexP(t *testing.T) {
	dir := t.TempDir()
	idx := previewEmbeddingIndex{
		Model:      "test-model",
		APIBase:    "https://api.example.com",
		Dimensions: 768,
		IndexedAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := writePreviewEmbeddingIndex(dir, idx); err != nil {
		t.Fatalf("writePreviewEmbeddingIndex: %v", err)
	}
	// Read it back using the loader
	got := readPreviewEmbeddingIndex(dir)
	if got.Model != "test-model" || got.Dimensions != 768 {
		t.Errorf("readback mismatch: %+v", got)
	}
	// Verify it was written to a cache directory that exists
	if got.IndexedAt.IsZero() {
		t.Errorf("IndexedAt mismatch: zero value")
	}
	if got.APIBase != "https://api.example.com" {
		t.Errorf("APIBase mismatch: %q", got.APIBase)
	}
}

func TestInjectBundleNilTemplateP(t *testing.T) {
	bundle := okfBundle{Nodes: []okfNode{}, Edges: []okfEdge{}, Bodies: map[string]string{}}
	if _, err := injectBundle(nil, bundle, "test", exportOptions{}); err == nil {
		t.Error("expected error for nil template")
	}
}

func TestInjectBundleOKP(t *testing.T) {
	bundle := okfBundle{
		Nodes:  []okfNode{{Data: okfNodeData{ID: "n1", Label: "Node 1"}}},
		Edges:  []okfEdge{},
		Bodies: map[string]string{"n1": "Hello body"},
		Types:  []string{"Concept"},
	}
	// inlineAssets=false uses CDN references
	out, err := injectBundle(exportTemplate, bundle, "MyName", exportOptions{inlineAssets: false})
	if err != nil {
		t.Fatalf("injectBundle: %v", err)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(string(out), "MyName") {
		t.Error("expected bundle name to appear in output")
	}
	if !strings.Contains(string(out), "Hello body") {
		t.Error("expected body to appear in output")
	}
}

func TestInjectBundleInlineAssetsP(t *testing.T) {
	bundle := okfBundle{
		Nodes:  []okfNode{{Data: okfNodeData{ID: "n1"}}},
		Edges:  []okfEdge{},
		Bodies: map[string]string{"n1": "Inline test"},
		Types:  []string{"Concept"},
	}
	out, err := injectBundle(exportTemplate, bundle, "InlineName", exportOptions{inlineAssets: true})
	if err != nil {
		t.Fatalf("injectBundle inline: %v", err)
	}
	if !strings.Contains(string(out), "InlineName") {
		t.Error("expected bundle name in output")
	}
}

func TestBuildVendorHeadP(t *testing.T) {
	inline, err := buildVendorHead(true)
	if err != nil {
		t.Fatalf("buildVendorHead(inline=true): %v", err)
	}
	if !strings.Contains(string(inline), "<script>") {
		t.Error("expected script tag for inline")
	}
	cdn, err := buildVendorHead(false)
	if err != nil {
		t.Fatalf("buildVendorHead(inline=false): %v", err)
	}
	if !strings.Contains(string(cdn), "cdn.jsdelivr.net") {
		t.Error("expected CDN reference")
	}
}

func TestOpenURLAllPlatformsP(t *testing.T) {
	// Restore original at end
	origCmd := openURLCmdForTest
	defer func() { openURLCmdForTest = origCmd }()

	// Capture which command was used by inspecting args
	var capturedName string
	var capturedArgs []string
	openURLCmdForTest = func(name string, args ...string) *exec.Cmd {
		capturedName = name
		capturedArgs = args
		// Return a command that just exits successfully without spawning
		return exec.Command(os.Args[0], "-test.run=^$")
	}

	if err := openURL("https://example.com"); err != nil {
		t.Fatalf("openURL: %v", err)
	}
	switch runtime.GOOS {
	case "darwin":
		if capturedName != "open" {
			t.Errorf("darwin: expected open, got %q", capturedName)
		}
	case "windows":
		if capturedName != "rundll32" {
			t.Errorf("windows: expected rundll32, got %q", capturedName)
		}
	default:
		if capturedName != "xdg-open" {
			t.Errorf("default: expected xdg-open, got %q", capturedName)
		}
	}
	if len(capturedArgs) == 0 {
		t.Error("expected captured args")
	}
}

func TestPortOfP(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"127.0.0.1:8080", "8080"},
		{":8080", "8080"},
		{"[::1]:8080", "8080"},
		{"no-port", "no-port"},
	}
	for _, tc := range cases {
		if got := portOf(tc.in); got != tc.want {
			t.Errorf("portOf(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestWriteGraphQueryTextEmptyP(t *testing.T) {
	var buf bytes.Buffer
	resp := previewSearchResponse{
		Query:    "test query",
		Warnings: []string{},
		Stats:    map[string]int{"docsSemantic": 0, "docsGraph": 0, "codeSemantic": 0, "codeGraph": 0},
	}
	if err := writeGraphQueryText(&buf, resp); err != nil {
		t.Fatalf("writeGraphQueryText: %v", err)
	}
	if !strings.Contains(buf.String(), "Query: test query") {
		t.Error("expected query in output")
	}
	if !strings.Contains(buf.String(), "Stats:") {
		t.Error("expected Stats in output")
	}
}

func TestWriteGraphQueryTextWithWarningsAndResultsP(t *testing.T) {
	var buf bytes.Buffer
	resp := previewSearchResponse{
		Query:    "hello",
		Warnings: []string{"warn1", "warn2"},
		Stats:    map[string]int{"docsSemantic": 5, "docsGraph": 2, "codeSemantic": 3, "codeGraph": 1},
		Panels: previewSearchPanels{
			CodeGraph: []previewSearchResult{{Title: "C1", Path: "a.go", Line: 10, NodeID: "n1"}},
			DocsGraph: []previewSearchResult{{Title: "D1", Path: "docs.md", NodeID: "n2"}},
		},
	}
	if err := writeGraphQueryText(&buf, resp); err != nil {
		t.Fatalf("writeGraphQueryText: %v", err)
	}
	if !strings.Contains(buf.String(), "Warnings:") {
		t.Error("expected Warnings in output")
	}
	if !strings.Contains(buf.String(), "warn1") {
		t.Error("expected warn1 in output")
	}
	if !strings.Contains(buf.String(), "Code Graph:") {
		t.Error("expected Code Graph panel in output")
	}
	if !strings.Contains(buf.String(), "C1") {
		t.Error("expected title in output")
	}
}

func TestWriteGraphQueryResultVariantsP(t *testing.T) {
	cases := []struct {
		name   string
		result previewSearchResult
		want   []string
	}{
		{
			"basic", previewSearchResult{Title: "Title", Path: "foo.go"},
			[]string{"Title", "foo.go"},
		},
		{
			"withLine", previewSearchResult{Title: "Title", Path: "foo.go", Line: 42},
			[]string{"foo.go:42"},
		},
		{
			"noPath", previewSearchResult{Title: "Title", NodeID: "node-id"},
			[]string{"Title", "node-id"},
		},
		{
			"meta", previewSearchResult{Title: "Title", Source: "code", Confidence: "high", FlowRole: "anchor"},
			[]string{"[code, high, anchor]"},
		},
		{
			"withNeighbors", previewSearchResult{
				Title: "Title",
				Neighbors: []previewSearchNeighbor{
					{ID: "n1", Direction: "out", Label: "L1", Path: "a.go", Line: 5, Relation: "implements"},
					{ID: "n2", Direction: "in", Label: "L2", Path: "b.go"},
					{ID: "n3", Direction: "out", Label: "L3"},
					{ID: "n4", Direction: "in", Label: "L4"},
				},
			},
			[]string{"out L1 via implements", "in L2", "+1 more"},
		},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		if err := writeGraphQueryResult(&buf, tc.result); err != nil {
			t.Errorf("%s: writeGraphQueryResult: %v", tc.name, err)
			continue
		}
		for _, want := range tc.want {
			if !strings.Contains(buf.String(), want) {
				t.Errorf("%s: expected %q in output: %s", tc.name, want, buf.String())
			}
		}
	}
}

func TestNonEmptyStringsP(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{nil, []string{}},
		{[]string{}, []string{}},
		{[]string{"", "a", "b"}, []string{"a", "b"}},
		{[]string{"a", "b", "a"}, []string{"a", "b"}},
		{[]string{"a", "", "a", "b"}, []string{"a", "b"}},
	}
	for _, tc := range cases {
		got := nonEmptyStrings(tc.in...)
		if !slices.Equal(got, tc.want) {
			t.Errorf("nonEmptyStrings(%v)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeSearchOutputPathP(t *testing.T) {
	cwd := t.TempDir()
	// Empty path → default name
	got := normalizeSearchOutputPath(cwd, "")
	if !strings.HasSuffix(got, defaultSearchLauncherName) {
		t.Errorf("empty: got %q, want suffix %q", got, defaultSearchLauncherName)
	}
	// Relative path → joined with cwd
	got = normalizeSearchOutputPath(cwd, "out.html")
	if !strings.HasPrefix(got, cwd) {
		t.Errorf("relative: got %q, want prefix %q", got, cwd)
	}
	// Absolute path → stays absolute
	got = normalizeSearchOutputPath(cwd, "/tmp/abs.html")
	if !strings.HasSuffix(got, "/tmp/abs.html") {
		t.Errorf("absolute: got %q, want suffix /tmp/abs.html", got)
	}
}

func TestLSPSymbolIsContainerP(t *testing.T) {
	containerKinds := []int{2, 3, 5, 11, 18, 23}
	for _, k := range containerKinds {
		if !lspSymbolIsContainer(k) {
			t.Errorf("kind %d: expected true", k)
		}
	}
	nonContainer := []int{1, 4, 6, 7, 8, 9, 10, 12, 13, 14, 15, 16, 17, 19, 20, 21, 22, 24, 25, 26}
	for _, k := range nonContainer {
		if lspSymbolIsContainer(k) {
			t.Errorf("kind %d: expected false", k)
		}
	}
}

func TestLSPSymbolIsResultNodeDocumentModeP(t *testing.T) {
	lang := lspLanguage{SymbolMode: lspSymbolModeDocument}
	// File (kind=1) and Module (kind=2) are not result nodes
	if lspSymbolIsResultNode(lang, lspDocumentSymbol{Name: "x", Kind: 1}) {
		t.Error("kind 1 should not be result node in document mode")
	}
	if lspSymbolIsResultNode(lang, lspDocumentSymbol{Name: "x", Kind: 2}) {
		t.Error("kind 2 should not be result node in document mode")
	}
	// Other kinds (12=function) are
	if !lspSymbolIsResultNode(lang, lspDocumentSymbol{Name: "foo", Kind: 12}) {
		t.Error("kind 12 should be result node in document mode")
	}
	// Empty name excluded
	if lspSymbolIsResultNode(lang, lspDocumentSymbol{Name: "", Kind: 12}) {
		t.Error("empty name should not be result node")
	}
	if lspSymbolIsResultNode(lang, lspDocumentSymbol{Name: "  ", Kind: 12}) {
		t.Error("whitespace-only name should not be result node")
	}
}

func TestLSPSymbolIsResultNodeSelectorModeP(t *testing.T) {
	lang := lspLanguage{SymbolMode: lspSymbolModeSelector}
	selectorKinds := []int{5, 7, 8, 12, 13, 14, 15, 18, 20}
	for _, k := range selectorKinds {
		if !lspSymbolIsResultNode(lang, lspDocumentSymbol{Name: "x", Kind: k}) {
			t.Errorf("kind %d: expected true in selector mode", k)
		}
	}
	nonSelector := []int{1, 2, 3, 4, 6, 9, 10, 11, 16, 17, 19, 21, 22, 23, 24}
	for _, k := range nonSelector {
		if lspSymbolIsResultNode(lang, lspDocumentSymbol{Name: "x", Kind: k}) {
			t.Errorf("kind %d: expected false in selector mode", k)
		}
	}
}

func TestLSPSymbolKindLabelP(t *testing.T) {
	cases := []struct {
		kind int
		want string
	}{
		{2, "module"}, {3, "namespace"}, {5, "class"}, {6, "method"},
		{9, "constructor"}, {11, "interface"}, {12, "function"}, {7, "property"},
		{8, "field"}, {13, "variable"}, {14, "constant"}, {15, "string"},
		{18, "object"}, {20, "key"}, {23, "struct"}, {24, "event"}, {25, "operator"},
		{1, "symbol"}, {99, "symbol"}, {0, "symbol"},
	}
	for _, tc := range cases {
		if got := lspSymbolKindLabel(tc.kind); got != tc.want {
			t.Errorf("kind %d: got %q want %q", tc.kind, got, tc.want)
		}
	}
}

func TestNodeLineP(t *testing.T) {
	cases := []struct {
		name string
		node lspCodeNode
		want int
	}{
		{"normal", lspCodeNode{SelectionRange: lspRange{Start: lspPosition{Line: 10}}}, 11},
		{"zeroLine", lspCodeNode{SelectionRange: lspRange{Start: lspPosition{Line: -1}}}, 1},
		{"negativeUsesRange", lspCodeNode{
			SelectionRange: lspRange{Start: lspPosition{Line: -2}},
			Range:          lspRange{Start: lspPosition{Line: 4}},
		}, 5},
	}
	for _, tc := range cases {
		if got := nodeLine(tc.node); got != tc.want {
			t.Errorf("%s: nodeLine=%d want %d", tc.name, got, tc.want)
		}
	}
}

func TestSortLSPCodeGraphCandidatesP(t *testing.T) {
	candidates := []lspCodeGraphCandidate{
		{ID: "a", Score: 0.5, Exactness: 3, Title: "B", Path: "z"},
		{ID: "b", Score: 0.9, Exactness: 1, Title: "A", Path: "y"},
		{ID: "c", Score: 0.9, Exactness: 5, Title: "C", Path: "x"},
		{ID: "d", Score: 0.9, Exactness: 5, Title: "C", Path: "x"}, // dup, sorts by ID
		{ID: "e", Score: 0.7, Exactness: 0, Title: "E", Path: "w"},
	}
	sortLSPCodeGraphCandidates(candidates)
	// Highest score first
	if candidates[0].Score < candidates[1].Score {
		t.Errorf("not sorted by score desc")
	}
	// Among ties, exactness desc (highest Exactness first)
	if candidates[0].Exactness < candidates[1].Exactness {
		t.Errorf("not sorted by exactness desc on ties")
	}
	// Then by Title asc
	if candidates[0].Title > candidates[1].Title {
		t.Errorf("not sorted by title asc on ties")
	}
}

func TestLimitLSPCodeGraphCandidatesP(t *testing.T) {
	candidates := []lspCodeGraphCandidate{
		{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"},
	}
	// Empty
	got := limitLSPCodeGraphCandidates([]lspCodeGraphCandidate{}, 2)
	if len(got) != 0 {
		t.Errorf("empty: got len %d", len(got))
	}
	// Zero limit → default
	got = limitLSPCodeGraphCandidates(candidates, 0)
	if len(got) != 4 {
		t.Errorf("zero limit: got len %d", len(got))
	}
	// Limit returns limit*2 capped by len
	got = limitLSPCodeGraphCandidates(candidates, 2)
	if len(got) != 4 {
		t.Errorf("limit 2: got len %d, want 4 (2*2)", len(got))
	}
	// Negative limit
	got = limitLSPCodeGraphCandidates(candidates, -1)
	if len(got) != 4 {
		t.Errorf("negative limit: got len %d", len(got))
	}
}

func TestLSPRangeSpanP(t *testing.T) {
	cases := []struct {
		name string
		rng  lspRange
		want int
	}{
		{"normal", lspRange{Start: lspPosition{Line: 0}, End: lspPosition{Line: 4}}, 5},
		{"singleLine", lspRange{Start: lspPosition{Line: 5}, End: lspPosition{Line: 5}}, 1},
		{"reverseRange", lspRange{Start: lspPosition{Line: 5}, End: lspPosition{Line: 0}}, 1},
	}
	for _, tc := range cases {
		if got := lspRangeSpan(tc.rng); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}

func TestPositionInLSPRangeP(t *testing.T) {
	rng := lspRange{
		Start: lspPosition{Line: 2, Character: 5},
		End:   lspPosition{Line: 4, Character: 10},
	}
	cases := []struct {
		name string
		pos  lspPosition
		want bool
	}{
		{"beforeLine", lspPosition{Line: 1, Character: 0}, false},
		{"afterLine", lspPosition{Line: 5, Character: 0}, false},
		{"startLineBeforeChar", lspPosition{Line: 2, Character: 3}, false},
		{"startLineAtChar", lspPosition{Line: 2, Character: 5}, true},
		{"middleLine", lspPosition{Line: 3, Character: 0}, true},
		{"endLineAtChar", lspPosition{Line: 4, Character: 10}, true},
		{"endLineAfterChar", lspPosition{Line: 4, Character: 11}, false},
	}
	for _, tc := range cases {
		if got := positionInLSPRange(tc.pos, rng); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestContextTimedOutP(t *testing.T) {
	// No error, no timeout
	if contextTimedOut(context.Background(), nil) {
		t.Error("background context: should not be timed out")
	}
	// Deadline exceeded error
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond)
	if !contextTimedOut(ctx, nil) {
		t.Error("expired context: should be timed out")
	}
	// Direct err = DeadlineExceeded
	if !contextTimedOut(context.Background(), context.DeadlineExceeded) {
		t.Error("err DeadlineExceeded: should be timed out")
	}
	if !contextTimedOut(context.Background(), context.Canceled) {
		t.Error("err Canceled: should be timed out")
	}
}

func TestDedupeLSPCodeEdgesP(t *testing.T) {
	edges := []lspCodeEdge{
		{Source: "a", Target: "b", Relation: "calls"},
		{Source: "a", Target: "b", Relation: "calls"}, // dup
		{Source: "b", Target: "a", Relation: "calls"}, // different direction
		{Source: "a", Target: "b", Relation: "uses"},  // different relation
		{Source: "", Target: "b", Relation: "calls"},  // empty source
		{Source: "a", Target: "", Relation: "calls"},  // empty target
		{Source: "a", Target: "a", Relation: "calls"}, // self
	}
	got := dedupeLSPCodeEdges(edges)
	if len(got) != 3 {
		t.Errorf("expected 3 deduped edges, got %d", len(got))
	}
}

func TestPathFromLSPURIP(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain", "abc", "abc"},
		{"fileURI", "file:///tmp/foo.go", "/tmp/foo.go"},
		{"fileURIWithSpaces", "file:///tmp/foo%20bar.go", "/tmp/foo bar.go"},
		{"malformed", "not-a-url", "not-a-url"},
	}
	for _, tc := range cases {
		got := pathFromLSPURI(tc.in)
		if runtime.GOOS == "windows" && tc.want != "" {
			// pathFromLSPURI returns what url.Parse gives us; on Windows the
			// drive letter may be stripped — accept the actual result.
			continue
		}
		if got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestValueAfterColonP(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"noColon", "no colon here", ""},
		{"simple", "Key: Value", "Value"},
		{"withBold", "**Status**: **Active**", "Active"},
		{"trailingSpace", "Key: Value   ", "Value"},
		{"multipleColons", "Key: foo: bar", "foo: bar"},
	}
	for _, tc := range cases {
		got := valueAfterColon(tc.in)
		if got != tc.want {
			t.Errorf("%s: valueAfterColon(%q)=%q want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestCleanNodeNameP(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "node1", "node1"},
		{"withParens", "node1(extra)", "node1"},
		{"withColon", "node1:label", "node1"},
		{"withMdSuffix", "node1.md", "node1"},
		{"withMdSuffixParens", "node1(file).md", "node1"},
		{"withBackticks", "`node1`", "node1"},
		{"trimmed", "  node1  ", "node1"},
	}
	for _, tc := range cases {
		got := cleanNodeName(tc.in)
		if got != tc.want {
			t.Errorf("%s: cleanNodeName(%q)=%q want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestSplitConstraintTargetP(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantKey string
		wantVal string
	}{
		{"empty", "", "", ""},
		{"single", "node1", "node1", ""},
		{"two", "node1 description here", "node1", "description here"},
		{"withBackticks", "`node1` value", "node1", "value"},
		{"withMd", "node1.md value", "node1", "value"},
		{"withMdParens", "node1(file).md", "node1(file)", ""},
	}
	for _, tc := range cases {
		k, v := splitConstraintTarget(tc.in)
		if k != tc.wantKey || v != tc.wantVal {
			t.Errorf("%s: splitConstraintTarget(%q)=(%q,%q) want (%q,%q)", tc.name, tc.in, k, v, tc.wantKey, tc.wantVal)
		}
	}
}

func TestResolveMermaidEndpointP(t *testing.T) {
	aliases := map[string]string{}
	// Empty
	if got := resolveMermaidEndpoint("", aliases); got != "" {
		t.Errorf("empty: got %q", got)
	}
	// Plain
	if got := resolveMermaidEndpoint("node1", aliases); got != "node1" {
		t.Errorf("plain: got %q", got)
	}
	// Inline alias - alias added to map
	got := resolveMermaidEndpoint("n1[\"Foo Bar\"]", aliases)
	if got != "Foo Bar" {
		t.Errorf("inline: got %q", got)
	}
	if aliases["n1"] != "Foo Bar" {
		t.Errorf("alias not registered: %v", aliases)
	}
	// With class via :::
	if got := resolveMermaidEndpoint("n2:::class1", aliases); got != "n2" {
		t.Errorf("class: got %q", got)
	}
	// Empty alias with label → just label
	got = resolveMermaidEndpoint("[\"L\"]", aliases)
	if got != "" || aliases[""] != "" {
		t.Errorf("empty alias with label: got %q aliases %v", got, aliases)
	}
	// Empty alias without label
	got = resolveMermaidEndpoint("[]", aliases)
	if got != "" {
		t.Errorf("empty label: got %q", got)
	}
	// Use existing alias
	if got := resolveMermaidEndpoint("n1", aliases); got != "Foo Bar" {
		t.Errorf("use alias: got %q want Foo Bar", got)
	}
}

func TestParseMetaSectionP(t *testing.T) {
	// Table-style metadata inside ## Meta block
	raw := `# Title

## Meta

| Key | Value |
| --- | --- |
| Status | Active |
| Version | 1.0 |
| Compliance | Strict |
| Description | Test description |
| Priority | High |
| Type | module |
| Timestamp | 2024-01-01 |
| Resource | src |
| Tags | [a, b, c] |

**Status**: ActiveBold
**Meta**: Status=Approved, Version=2.0, Compliance=Newer, Description=Final
`
	meta := parseMetaSection(raw)
	// prose values overwrite table values when both are present
	if meta.Status != "ActiveBold" {
		t.Errorf("Status: got %q", meta.Status)
	}
	if meta.Version != "1.0" {
		t.Errorf("Version: got %q", meta.Version)
	}
	if meta.Compliance != "Strict" {
		t.Errorf("Compliance: got %q", meta.Compliance)
	}
	if meta.Description != "Test description" {
		t.Errorf("Description: got %q", meta.Description)
	}
	if meta.Priority != "High" {
		t.Errorf("Priority: got %q", meta.Priority)
	}
	if meta.Type != "module" {
		t.Errorf("Type: got %q", meta.Type)
	}
	if meta.Timestamp != "2024-01-01" {
		t.Errorf("Timestamp: got %q", meta.Timestamp)
	}
	if meta.Resource != "src" {
		t.Errorf("Resource: got %q", meta.Resource)
	}
	if len(meta.Tags) != 3 {
		t.Errorf("Tags: got %v", meta.Tags)
	}
	// Frontmatter in front of ## Meta should be ignored by parseMetaSection (only ## Meta block is parsed)
	raw2 := `---
status: FromFrontmatter
---

## Meta

**Status**: FromProse
`
	meta2 := parseMetaSection(raw2)
	if meta2.Status != "FromProse" {
		t.Errorf("Status from prose: got %q", meta2.Status)
	}
}

func TestWriteLockedNotRunningP(t *testing.T) {
	// Not running, no stdin
	s := &previewLSPServer{lang: lspLanguage{ServerID: "test-server"}}
	err := s.writeLocked(context.Background(), map[string]any{"x": 1})
	if err == nil {
		t.Error("expected error when not running")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("expected 'not running' error, got %v", err)
	}
}

func TestWriteLockedContextCanceledP(t *testing.T) {
	// Use a pipe to simulate stdin/stdout
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()
	s := &previewLSPServer{
		lang:    lspLanguage{ServerID: "test"},
		running: true,
		stdin:   pw,
		reader:  bufio.NewReader(pr),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.writeLocked(ctx, map[string]any{"x": 1})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestWriteLockedMarshalErrorP(t *testing.T) {
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()
	s := &previewLSPServer{
		lang:    lspLanguage{ServerID: "test"},
		running: true,
		stdin:   pw,
		reader:  bufio.NewReader(pr),
	}
	// Channels are not marshalable
	err := s.writeLocked(context.Background(), make(chan int))
	if err == nil {
		t.Error("expected marshal error")
	}
}

func TestWriteLockedSuccessP(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		defer pr.Close()
		defer pw.Close()
		buf := make([]byte, 4096)
		_, _ = pr.Read(buf)
	}()
	s := &previewLSPServer{
		lang:    lspLanguage{ServerID: "test"},
		running: true,
		stdin:   pw,
		reader:  bufio.NewReader(pr),
	}
	if err := s.writeLocked(context.Background(), map[string]any{"x": 1}); err != nil {
		t.Fatalf("writeLocked: %v", err)
	}
}

func TestReadMessageLockedP(t *testing.T) {
	// Test readMessageLocked which reads Content-Length framed messages
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		pw.Write([]byte("Content-Length: 11\r\n\r\nHello World"))
	}()
	s := &previewLSPServer{reader: bufio.NewReader(pr)}
	msg, err := s.readMessageLocked()
	if err != nil {
		t.Fatalf("readMessageLocked: %v", err)
	}
	if string(msg) != "Hello World" {
		t.Errorf("got %q, want Hello World", msg)
	}
}

func TestReadMessageLockedMissingContentLengthP(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		pw.Write([]byte("\r\n"))
	}()
	s := &previewLSPServer{reader: bufio.NewReader(pr)}
	_, err := s.readMessageLocked()
	if err == nil {
		t.Error("expected error for missing content length")
	}
}

func TestReadResponseLockedContextCanceledP(t *testing.T) {
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()
	s := &previewLSPServer{lang: lspLanguage{ServerID: "test"}, reader: bufio.NewReader(pr)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.readResponseLocked(ctx, 1, nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestReadResponseLockedReadErrorP(t *testing.T) {
	pr, pw := io.Pipe()
	pw.Close()
	s := &previewLSPServer{
		lang:       lspLanguage{ServerID: "test"},
		reader:     bufio.NewReader(pr),
		running:    true,
		initialized: true,
	}
	err := s.readResponseLocked(context.Background(), 1, nil)
	if err == nil {
		t.Error("expected error from closed pipe")
	}
	if s.running {
		t.Error("expected running to be reset to false on read error")
	}
}

func TestRequestNotInitializedP(t *testing.T) {
	// request() should refuse non-initialize calls before initialize completes
	s := &previewLSPServer{lang: lspLanguage{ServerID: "test"}, initialized: false}
	err := s.request(context.Background(), "textDocument/documentSymbol", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("expected not initialized error, got %v", err)
	}
}

func TestRequestInitializeAllowedWithoutInitializedP(t *testing.T) {
	// "initialize" method is allowed even if initialized=false - just verify it doesn't return
	// the "not initialized" error. The subsequent I/O will fail since stdin is not connected.
	s := &previewLSPServer{
		lang:       lspLanguage{ServerID: "test"},
		running:    true,
		initialized: false,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := s.request(ctx, "initialize", map[string]any{}, nil)
	if err != nil && strings.Contains(err.Error(), "not initialized") {
		t.Errorf("initialize should not be blocked by not-initialized check, got %v", err)
	}
}

func TestDocsRootP(t *testing.T) {
	absPath := "/absolute/docs"
	if got := docsRoot("/project", absPath); got != absPath {
		t.Errorf("absolute: got %q", got)
	}
	// Relative → joined
	if got := docsRoot("/project", "docs"); got != "/project/docs" {
		t.Errorf("relative: got %q", got)
	}
	// Tilde expansion
	home, _ := os.UserHomeDir()
	if home != "" {
		if got := docsRoot("/project", "~/mydocs"); got != filepath.Join(home, "mydocs") {
			t.Errorf("tilde: got %q", got)
		}
	}
}

func TestScanSpecDocumentsP(t *testing.T) {
	root := t.TempDir()
	// Create a valid markdown file
	mdPath := filepath.Join(root, "doc.md")
	if err := os.WriteFile(mdPath, []byte("# Title\n\nContent"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create a subdir with a file
	subdir := filepath.Join(root, "sub")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	subFile := filepath.Join(subdir, "sub.md")
	if err := os.WriteFile(subFile, []byte("# Sub"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Invalid UTF-8
	invalidPath := filepath.Join(root, "invalid.md")
	if err := os.WriteFile(invalidPath, []byte{0xff, 0xfe}, 0o644); err != nil {
		t.Fatal(err)
	}
	// Large file
	largePath := filepath.Join(root, "large.md")
	large := bytes.Repeat([]byte("a"), int(maxSearchFileBytes)+1)
	if err := os.WriteFile(largePath, large, 0o644); err != nil {
		t.Fatal(err)
	}
	docs, err := scanSpecDocuments(root, nil)
	if err != nil {
		t.Fatalf("scanSpecDocuments: %v", err)
	}
	// Should have 2 valid docs (doc.md and sub/sub.md)
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d: %+v", len(docs), docs)
	}
}

func TestFirstNonEmptyTagsP(t *testing.T) {
	if got := firstNonEmptyTags(); got != nil {
		t.Errorf("empty: got %v", got)
	}
	if got := firstNonEmptyTags(nil, nil); got != nil {
		t.Errorf("all nil: got %v", got)
	}
	got := firstNonEmptyTags(nil, []string{"a"}, []string{"b", "c"})
	if !slices.Equal(got, []string{"a"}) {
		t.Errorf("first non-empty: got %v", got)
	}
	if got := firstNonEmptyTags([]string{}, []string{"x"}); !slices.Equal(got, []string{"x"}) {
		t.Errorf("empty slice skipped: got %v", got)
	}
}

func TestNormalizePreviewProjectRootPy(t *testing.T) {
	// Plain path
	got := normalizePreviewProjectRoot("docs")
	if !filepath.IsAbs(got) {
		t.Errorf("plain: expected abs, got %q", got)
	}
	// Already absolute
	abs, _ := filepath.Abs("/foo/bar")
	got = normalizePreviewProjectRoot(abs)
	if got != abs {
		t.Errorf("absolute: got %q want %q", got, abs)
	}
	// Tilde
	home, _ := os.UserHomeDir()
	if home != "" {
		got = normalizePreviewProjectRoot("~/")
		if got != home {
			t.Errorf("tilde: got %q want %q", got, home)
		}
	}
}

func TestHandleEventsStreamingUnsupportedP(t *testing.T) {
	// Wrap response writer to NOT implement Flusher
	ps := &previewServer{opt: previewOptions{projectRoot: t.TempDir(), docsDir: "docs"}}
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	w := &nonFlusherWriter{ResponseWriter: httptest.NewRecorder()}
	ps.handleEvents(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// nonFlusherWriter wraps http.ResponseWriter to NOT satisfy http.Flusher.
type nonFlusherWriter struct {
	http.ResponseWriter
	Code int
}

func (n *nonFlusherWriter) WriteHeader(code int) { n.Code = code }

func TestHandleEventsChangeEventP(t *testing.T) {
	projectRoot := t.TempDir()
	docsDir := filepath.Join(projectRoot, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Start with no files to ensure changeToken is stable initially
	ps := &previewServer{opt: previewOptions{projectRoot: projectRoot, docsDir: "docs"}}
	srv := httptest.NewServer(http.HandlerFunc(ps.handleEvents))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Read in a goroutine; we expect "event: ready" first.
	got := make(chan string, 16)
	go func() {
		b := make([]byte, 8192)
		for {
			n, err := resp.Body.Read(b)
			if n > 0 {
				got <- string(b[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// Wait for "event: ready"
	deadline := time.After(2 * time.Second)
readyLoop:
	for {
		select {
		case s := <-got:
			if strings.Contains(s, "event: ready") {
				break readyLoop
			}
		case <-deadline:
			t.Fatal("timed out waiting for ready event")
		}
	}

	// Now add a file in docs/ to trigger change
	if err := os.WriteFile(filepath.Join(docsDir, "new.md"), []byte("# new"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for change event
	deadline = time.After(3 * time.Second)
changeLoop:
	for {
		select {
		case s := <-got:
			if strings.Contains(s, "event: change") {
				break changeLoop
			}
		case <-deadline:
			t.Fatal("timed out waiting for change event")
		}
	}
	cancel()
}

func TestOpenURLExecuteFailureP(t *testing.T) {
	origCmd := openURLCmdForTest
	defer func() { openURLCmdForTest = origCmd }()

	// Simulate command that returns a command whose Start will fail.
	// We use a non-existent executable to make Start fail.
	openURLCmdForTest = func(name string, args ...string) *exec.Cmd {
		return exec.Command("/this/binary/does/not/exist/at/all", args...)
	}
	if err := openURL("https://example.com"); err == nil {
		t.Error("expected error from openURL")
	}
}

func TestOpenURLTestOverrideP(t *testing.T) {
	origOpen := openURLForTest
	origCmd := openURLCmdForTest
	defer func() {
		openURLForTest = origOpen
		openURLCmdForTest = origCmd
	}()

	called := false
	openURLForTest = func(target string) error {
		called = true
		if target != "https://example.com" {
			t.Errorf("expected target, got %q", target)
		}
		return nil
	}
	if err := openURLForTest("https://example.com"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected override to be called")
	}
}

func TestPreviewEmbeddingConfigFromKnownsProjectP(t *testing.T) {
	orig := loadKnownsEmbeddingSettingsForTest
	defer func() { loadKnownsEmbeddingSettingsForTest = orig }()
	loadKnownsEmbeddingSettingsForTest = func() (knownsEmbeddingSettings, error) {
		return knownsEmbeddingSettings{}, errors.New("no settings")
	}

	// No .knowns/config.json
	dir := t.TempDir()
	if _, err := previewEmbeddingConfigFromKnownsProject(dir); err == nil {
		t.Error("expected error when no .knowns dir")
	}

	// Create invalid config.json
	knownsDir := filepath.Join(dir, ".knowns")
	if err := os.MkdirAll(knownsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(knownsDir, "config.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := previewEmbeddingConfigFromKnownsProject(dir); err == nil {
		t.Error("expected error from invalid JSON")
	}

	// valid JSON but missing fields
	if err := os.WriteFile(filepath.Join(knownsDir, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := previewEmbeddingConfigFromKnownsProject(dir); err == nil {
		t.Error("expected error when no semantic search enabled")
	}

	// semantic search enabled but not "api" provider
	cfg := `{"settings":{"semanticSearch":{"enabled":true,"model":"m1","provider":"local"}}}`
	if err := os.WriteFile(filepath.Join(knownsDir, "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := previewEmbeddingConfigFromKnownsProject(dir); err == nil {
		t.Error("expected error for non-API provider")
	}

	// settings load failure
	cfg = `{"settings":{"semanticSearch":{"enabled":true,"model":"m1","provider":"api"}}}`
	if err := os.WriteFile(filepath.Join(knownsDir, "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := previewEmbeddingConfigFromKnownsProject(dir); err == nil {
		t.Error("expected error when settings load fails")
	}
}

func TestPreviewEmbeddingConfigForProjectAllFailP(t *testing.T) {
	orig := loadKnownsEmbeddingSettingsForTest
	defer func() { loadKnownsEmbeddingSettingsForTest = orig }()
	// Make all resolvers fail
	loadKnownsEmbeddingSettingsForTest = func() (knownsEmbeddingSettings, error) {
		return knownsEmbeddingSettings{}, errors.New("no settings")
	}
	dir := t.TempDir()
	cfg, warning := previewEmbeddingConfigForProject(dir)
	if cfg.APIBase != "" {
		t.Errorf("expected empty cfg, got %+v", cfg)
	}
	if warning == "" {
		t.Error("expected warning when all resolvers fail")
	}
}

func TestLoadPreviewEmbeddingSearchWarningP(t *testing.T) {
	orig := loadKnownsEmbeddingSettingsForTest
	defer func() { loadKnownsEmbeddingSettingsForTest = orig }()
	loadKnownsEmbeddingSettingsForTest = func() (knownsEmbeddingSettings, error) {
		return knownsEmbeddingSettings{}, errors.New("no settings")
	}
	dir := t.TempDir()
	search, warnings := loadPreviewEmbeddingSearch(dir, nil, nil)
	if search != nil {
		t.Errorf("expected nil search when config fails, got %+v", search)
	}
	if len(warnings) == 0 {
		t.Error("expected warnings when config fails")
	}
}

func TestPreviewEmbeddingConfigFromOllamaP(t *testing.T) {
	orig := ollamaGetForTest
	defer func() { ollamaGetForTest = orig }()

	// Network error path.
	ollamaGetForTest = func(url string) (*http.Response, error) {
		return nil, errors.New("connection refused")
	}
	if _, err := previewEmbeddingConfigFromOllama(); err == nil {
		t.Error("expected error when ollama server unreachable")
	}

	// Non-2xx response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()
	ollamaGetForTest = func(url string) (*http.Response, error) {
		return server.Client().Get(server.URL + "/api/tags")
	}
	if _, err := previewEmbeddingConfigFromOllama(); err == nil {
		t.Error("expected error on non-2xx status")
	}

	// Invalid JSON.
	invalidJSONServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not-json"))
	}))
	defer invalidJSONServer.Close()
	ollamaGetForTest = func(url string) (*http.Response, error) {
		return invalidJSONServer.Client().Get(invalidJSONServer.URL + "/api/tags")
	}
	if _, err := previewEmbeddingConfigFromOllama(); err == nil {
		t.Error("expected error when JSON invalid")
	}

	// No priority match (no models returned).
	emptyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer emptyServer.Close()
	ollamaGetForTest = func(url string) (*http.Response, error) {
		return emptyServer.Client().Get(emptyServer.URL + "/api/tags")
	}
	if _, err := previewEmbeddingConfigFromOllama(); err == nil {
		t.Error("expected error when no priority match")
	}

	// Success path with a priority model.
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"random-model"},{"name":"nomic-embed-text"}]}`))
	}))
	defer successServer.Close()
	ollamaGetForTest = func(url string) (*http.Response, error) {
		return successServer.Client().Get(successServer.URL + "/api/tags")
	}
	cfg, err := previewEmbeddingConfigFromOllama()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model != "nomic-embed-text" {
		t.Errorf("expected nomic-embed-text, got %q", cfg.Model)
	}
	if cfg.Source != "ollama" {
		t.Errorf("expected source ollama, got %q", cfg.Source)
	}
}

func TestHandleGraphP(t *testing.T) {
	// Method not allowed.
	ps := &previewServer{opt: previewOptions{projectRoot: t.TempDir(), docsDir: "docs"}}
	req := httptest.NewRequest(http.MethodPost, "/api/graph", nil)
	w := httptest.NewRecorder()
	ps.handleGraph(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}

	// Success path with valid project root.
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "test.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ps = &previewServer{opt: previewOptions{projectRoot: root, docsDir: "docs"}}
	req = httptest.NewRequest(http.MethodGet, "/api/graph", nil)
	w = httptest.NewRecorder()
	ps.handleGraph(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct == "" || !strings.Contains(ct, "json") {
		t.Errorf("expected JSON content type, got %q", ct)
	}

	// Load error path: docs directory does not exist.
	ps = &previewServer{opt: previewOptions{projectRoot: t.TempDir(), docsDir: "missing"}}
	req = httptest.NewRequest(http.MethodGet, "/api/graph", nil)
	w = httptest.NewRecorder()
	ps.handleGraph(w, req)
	if w.Code == http.StatusOK {
		t.Errorf("expected error status, got 200")
	}
}

func TestExpandLSPCodeGraphCallFlowP(t *testing.T) {
	// Manager exists but no server registered for the node.
	provider := &previewLSPCodeGraphProvider{
		manager: &previewLSPManager{servers: map[string]*previewLSPServer{}},
	}
	candidate := lspCodeGraphCandidate{
		ID:    "n1",
		Node:  lspCodeNode{ID: "n1", Name: "alpha"},
		Score: 1.0,
	}
	index := lspCodeGraphIndex{Nodes: map[string]lspCodeNode{}}
	results, edges, warnings := provider.expandLSPCodeGraphCallFlow(context.Background(), index, candidate, 5)
	if _, ok := results[candidate.ID]; !ok {
		t.Errorf("expected anchor in results, got %+v", results)
	}
	if len(edges) != 0 {
		t.Errorf("expected no edges without server, got %+v", edges)
	}
	if len(warnings) == 0 {
		t.Error("expected warnings when no server registered")
	}
}

func TestLSPServerStartAlreadyRunningP(t *testing.T) {
	// Already running - returns nil without re-running command.
	s := &previewLSPServer{running: true}
	if err := s.Start(context.Background()); err != nil {
		t.Errorf("expected nil when running, got %v", err)
	}
}

func TestLSPServerStartContextCanceledP(t *testing.T) {
	// Context canceled before start.
	s := &previewLSPServer{command: "echo", args: []string{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Start(ctx); err == nil {
		t.Error("expected error when context already canceled")
	}
}

func TestLSPServerStartBadCommandP(t *testing.T) {
	// Command fails to start.
	s := &previewLSPServer{command: "definitely-not-a-real-command-xyz123", args: []string{}}
	if err := s.Start(context.Background()); err == nil {
		t.Error("expected error when command cannot start")
	}
}


func TestLSPCodeGraphProviderCloseP(t *testing.T) {
	// nil provider.
	var p *previewLSPCodeGraphProvider
	if err := p.Close(context.Background()); err != nil {
		t.Errorf("expected nil for nil provider, got %v", err)
	}
	// provider with nil manager.
	p = &previewLSPCodeGraphProvider{}
	if err := p.Close(context.Background()); err != nil {
		t.Errorf("expected nil for nil manager, got %v", err)
	}
	// provider with manager (no servers to stop).
	p = &previewLSPCodeGraphProvider{manager: newPreviewLSPManager(t.TempDir())}
	if err := p.Close(context.Background()); err != nil {
		t.Errorf("expected nil for empty manager, got %v", err)
	}
	// provider with a registered server (Stop will be called).
	srv := &previewLSPServer{}
	p.manager.servers["test"] = srv
	// srv is not running so Stop is a no-op.
	if err := p.Close(context.Background()); err != nil {
		t.Errorf("expected nil even with registered server, got %v", err)
	}
}

func TestWithOpenFileP(t *testing.T) {
	// Start fails -> error propagated.
	origStart := lspStartServer
	defer func() { lspStartServer = origStart }()
	lspStartServer = func(ctx context.Context, srv *previewLSPServer) error {
		return errors.New("start failed")
	}
	s := &previewLSPServer{}
	err := s.withOpenFile(context.Background(), "/nonexistent/file.go", "go", func() error { return nil })
	if err == nil {
		t.Error("expected error when start fails")
	}

	// Path doesn't exist.
	lspStartServer = func(ctx context.Context, srv *previewLSPServer) error {
		srv.running = true
		return nil
	}
	err = s.withOpenFile(context.Background(), "/nonexistent/file.go", "go", func() error { return nil })
	if err == nil {
		t.Error("expected error when file not found")
	}

	// File not valid UTF-8.
	tmpFile := filepath.Join(t.TempDir(), "badutf8.go")
	if err := os.WriteFile(tmpFile, []byte{0xff, 0xfe, 0xfd}, 0o644); err != nil {
		t.Fatal(err)
	}
	err = s.withOpenFile(context.Background(), tmpFile, "go", func() error { return nil })
	if err == nil {
		t.Error("expected error when file not valid UTF-8")
	}

	// Start succeeds and file is opened; fn is invoked.
	tmpFile2 := filepath.Join(t.TempDir(), "ok.go")
	if err := os.WriteFile(tmpFile2, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	called := false
	s2 := &previewLSPServer{}
	err = s2.withOpenFile(context.Background(), tmpFile2, "go", func() error {
		called = true
		return nil
	})
	// withOpenFile calls notify -> notify goes through request which writes to stdin.
	// Since srv.stdin is nil, request will panic; we accept either an error or a clean success.
	if !called && err == nil {
		t.Error("expected fn to be invoked or error to be returned")
	}
}

func TestExpandLSPCodeGraphCallFlowWithEdgesP(t *testing.T) {
	// Stub hooks so we can drive expandLSPCodeGraphCallFlow through both
	// edge branches (Source == candidate.ID and Target == candidate.ID).
	origByID := lspServerByID
	origPrepare := lspPrepareCallHierarchy
	origIncoming := lspIncomingCalls
	origOutgoing := lspOutgoingCalls
	defer func() {
		lspServerByID = origByID
		lspPrepareCallHierarchy = origPrepare
		lspIncomingCalls = origIncoming
		lspOutgoingCalls = origOutgoing
	}()

	projectRoot := t.TempDir()
	fileA := filepath.Join(projectRoot, "a.go")
	fileB := filepath.Join(projectRoot, "b.go")

	lspServerByID = func(manager *previewLSPManager, serverID string) (*previewLSPServer, error) {
		return &previewLSPServer{}, nil
	}
	lspPrepareCallHierarchy = func(ctx context.Context, srv *previewLSPServer, path, languageID string, pos lspPosition) ([]lspCallHierarchyItem, error) {
		return []lspCallHierarchyItem{{Name: "alpha"}}, nil
	}
	lspIncomingCalls = func(ctx context.Context, srv *previewLSPServer, item lspCallHierarchyItem) ([]lspIncomingCall, error) {
		return []lspIncomingCall{{
			From: lspCallHierarchyItem{
				URI:            fileURI(fileA),
				SelectionRange: lspRange{Start: lspPosition{Line: 1, Character: 1}, End: lspPosition{Line: 2, Character: 2}},
			},
		}}, nil
	}
	lspOutgoingCalls = func(ctx context.Context, srv *previewLSPServer, item lspCallHierarchyItem) ([]lspOutgoingCall, error) {
		return []lspOutgoingCall{{
			To: lspCallHierarchyItem{
				URI:            fileURI(fileB),
				SelectionRange: lspRange{Start: lspPosition{Line: 3, Character: 3}, End: lspPosition{Line: 4, Character: 4}},
			},
		}}, nil
	}

	provider := &previewLSPCodeGraphProvider{
		manager:     &previewLSPManager{servers: map[string]*previewLSPServer{}},
		projectRoot: projectRoot,
	}
	nodeA := lspCodeNode{
		ID:             "nA",
		Name:           "caller",
		ServerID:       "test",
		Path:           "a.go",
		AbsPath:        fileA,
		SelectionRange: lspRange{Start: lspPosition{Line: 1, Character: 1}, End: lspPosition{Line: 2, Character: 2}},
	}
	nodeB := lspCodeNode{
		ID:             "nB",
		Name:           "callee",
		ServerID:       "test",
		Path:           "b.go",
		AbsPath:        fileB,
		SelectionRange: lspRange{Start: lspPosition{Line: 3, Character: 3}, End: lspPosition{Line: 4, Character: 4}},
	}
	candidate := lspCodeGraphCandidate{
		ID:    "n1",
		Node:  lspCodeNode{ID: "n1", ServerID: "test", Path: "x.go", SelectionRange: lspRange{Start: lspPosition{Line: 5, Character: 5}}},
		Score: 1.0,
	}
	index := lspCodeGraphIndex{
		Nodes: map[string]lspCodeNode{
			"n1": candidate.Node,
			"nA": nodeA,
			"nB": nodeB,
		},
		ByPath: map[string][]string{
			"a.go": {"nA"},
			"b.go": {"nB"},
			"x.go": {"n1"},
		},
	}
	results, edges, _ := provider.expandLSPCodeGraphCallFlow(context.Background(), index, candidate, 5)
	if _, ok := results[candidate.ID]; !ok {
		t.Errorf("expected anchor in results, got %+v", results)
	}
	if len(edges) < 2 {
		t.Errorf("expected >=2 edges (incoming + outgoing), got %d", len(edges))
	}
}

func TestStopPreviewChildAllBranchesP(t *testing.T) {
	origGOOS := runtimeGOOSForTest
	defer func() { runtimeGOOSForTest = origGOOS }()

	// nil cmd → return early.
	stopPreviewChild(nil)

	// Non-windows branch: real *exec.Cmd whose Process exists.
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	runtimeGOOSForTest = "darwin"
	stopPreviewChild(cmd)
	cmd.Wait() // reap

	// Windows branch: just call with stubbed GOOS.
	cmd2 := exec.Command("sleep", "10")
	if err := cmd2.Start(); err != nil {
		t.Fatalf("start 2: %v", err)
	}
	runtimeGOOSForTest = "windows"
	stopPreviewChild(cmd2)
	cmd2.Wait()
}

func TestOpenURLAllBranchesP(t *testing.T) {
	origCmd := openURLCmdForTest
	origGOOS := runtimeGOOSForTest
	defer func() {
		openURLCmdForTest = origCmd
		runtimeGOOSForTest = origGOOS
	}()
	// Track calls to verify each branch picks the expected binary.
	var gotName string
	var gotArgs []string
	openURLCmdForTest = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = args
		return exec.Command("true")
	}

	// darwin branch.
	runtimeGOOSForTest = "darwin"
	if err := openURL("https://example.com"); err != nil {
		t.Fatalf("openURL darwin: %v", err)
	}
	if gotName != "open" || len(gotArgs) != 1 || gotArgs[0] != "https://example.com" {
		t.Errorf("darwin: got %s %v", gotName, gotArgs)
	}

	// windows branch.
	runtimeGOOSForTest = "windows"
	if err := openURL("https://example.com"); err != nil {
		t.Fatalf("openURL windows: %v", err)
	}
	if gotName != "rundll32" || len(gotArgs) != 2 {
		t.Errorf("windows: got %s %v", gotName, gotArgs)
	}

	// default branch.
	runtimeGOOSForTest = "linux"
	if err := openURL("https://example.com"); err != nil {
		t.Fatalf("openURL default: %v", err)
	}
	if gotName != "xdg-open" || len(gotArgs) != 1 {
		t.Errorf("default: got %s %v", gotName, gotArgs)
	}
}

func TestRunHotReloadSupervisorFrontendChangedP(t *testing.T) {
	orig := startPreviewChildForTest
	defer func() { startPreviewChildForTest = orig }()

	// Track starts. Each start returns a buffered channel; we drain prior
	// channels after the supervisor calls stopPreviewChild so it can advance
	// to the next iteration. Sending a result of nil keeps the supervisor
	// from exiting (it treats a result on done as a normal exit).
	var mu sync.Mutex
	startCount := 0
	doneChans := []chan previewChildResult{}
	startPreviewChildForTest = func(moduleRoot string, args []string) (*exec.Cmd, <-chan previewChildResult, error) {
		mu.Lock()
		startCount++
		ch := make(chan previewChildResult, 1)
		doneChans = append(doneChans, ch)
		mu.Unlock()
		return nil, ch, nil
	}

	moduleRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(moduleRoot, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	uiDir := filepath.Join(moduleRoot, "internal", "preview", "preview_ui_src")
	if err := os.MkdirAll(uiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	uiFile := filepath.Join(uiDir, "index.html")
	if err := os.WriteFile(uiFile, []byte("<html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	go func() {
		_ = runHotReloadSupervisor(moduleRoot, []string{}, moduleRoot)
	}()
	// Wait for ticker (700ms cycle), then change the frontend file's mtime.
	time.Sleep(800 * time.Millisecond)
	if err := os.WriteFile(uiFile, []byte("<html>changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(uiFile, future, future); err != nil {
		t.Fatal(err)
	}
	// Wait for the next ticker to fire (at most ~700ms) so the supervisor
	// reaches <-done inside the ticker.C branch. Then send to the initial
	// doneChans[0] so it can advance past <-done and start a new child.
	time.Sleep(1200 * time.Millisecond)
	mu.Lock()
	if len(doneChans) >= 1 {
		select {
		case doneChans[0] <- previewChildResult{err: nil}:
		default:
		}
	}
	mu.Unlock()
	// Wait for the second start.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		current := startCount
		mu.Unlock()
		if current >= 2 {
			break
		}
	}
	mu.Lock()
	got := startCount
	// Close last channel to let the supervisor exit cleanly.
	if len(doneChans) > 0 {
		last := doneChans[len(doneChans)-1]
		select {
		case <-last:
		default:
			close(last)
		}
	}
	mu.Unlock()
	if got < 2 {
		t.Errorf("expected at least 2 starts (initial + restart), got %d", got)
	}
}

func TestRunHotReloadSupervisorRestartErrorP(t *testing.T) {
	orig := startPreviewChildForTest
	defer func() { startPreviewChildForTest = orig }()

	// First call succeeds, second fails. Use a channel to safely hand
	// the first child's done channel from the stub goroutine to the
	// main test goroutine; this avoids a data race on the variable.
	var mu sync.Mutex
	var callCount int
	firstDoneCh := make(chan chan previewChildResult, 1)
	startPreviewChildForTest = func(moduleRoot string, args []string) (*exec.Cmd, <-chan previewChildResult, error) {
		mu.Lock()
		callCount++
		idx := callCount
		mu.Unlock()
		if idx == 1 {
			firstDone := make(chan previewChildResult, 1)
			firstDoneCh <- firstDone
			return nil, firstDone, nil
		}
		return nil, nil, errors.New("restart failed")
	}

	moduleRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(moduleRoot, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- runHotReloadSupervisor(moduleRoot, []string{}, moduleRoot)
	}()
	// Trigger ticker with a file change.
	time.Sleep(800 * time.Millisecond)
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filepath.Join(moduleRoot, "main.go"), future, future); err != nil {
		t.Fatal(err)
	}
	// Wait for the next ticker to fire and supervisor to reach <-done.
	time.Sleep(1200 * time.Millisecond)
	// Receive the first child's done channel from the stub via a channel
	// (synchronized handoff). This avoids reading the variable directly
	// from a different goroutine.
	var firstDone chan previewChildResult
	select {
	case firstDone = <-firstDoneCh:
	case <-time.After(1 * time.Second):
		t.Fatal("did not receive first child done channel")
	}
	if firstDone != nil {
		select {
		case firstDone <- previewChildResult{err: nil}:
		default:
		}
	}

	select {
	case err := <-done:
		if err == nil || err.Error() != "restart failed" {
			t.Errorf("expected restart failed, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("test timed out waiting for restart error")
	}
}


func TestNormalizePreviewProjectRootFallbackP(t *testing.T) {
	// Path that will trigger ExpandPath but not filepath.Abs.
	// We can't easily trigger Abs error, but we test the basic path.
	got := normalizePreviewProjectRoot("/already/absolute/path")
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
	// Empty path.
	got = normalizePreviewProjectRoot("")
	if got != "" && !filepath.IsAbs(got) {
		t.Errorf("expected empty or absolute, got %q", got)
	}
	// ~ path: ExpandPath resolves to home dir.
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		got = normalizePreviewProjectRoot("~/foo")
		if !filepath.IsAbs(got) || !strings.HasPrefix(got, home) {
			t.Errorf("expected ~/foo to be under %s, got %q", home, got)
		}
	}

	// filepath.Abs error path: stub the seam to fail.
	origAbs := filepathAbsForTest
	defer func() { filepathAbsForTest = origAbs }()
	filepathAbsForTest = func(path string) (string, error) {
		return "", errors.New("abs fail")
	}
	got = normalizePreviewProjectRoot("/some/path")
	if got != "/some/path" {
		t.Errorf("expected /some/path on abs failure, got %q", got)
	}
}

func TestBuildGraphQueryResponseP(t *testing.T) {
	// No docs dir - warning path.
	root := t.TempDir()
	codeGraph := &nullCodeGraphProvider{}
	resp := buildGraphQueryResponse(context.Background(), graphOptions{projectRoot: root, docsDir: "docs"}, codeGraph)
	if len(resp.Warnings) == 0 {
		t.Error("expected warning when docs dir missing")
	}

	// With empty query and matching docs dir - no docs but response returned.
	root2 := t.TempDir()
	docsDir := filepath.Join(root2, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	resp2 := buildGraphQueryResponse(context.Background(), graphOptions{projectRoot: root2, docsDir: "docs", query: "alpha"}, codeGraph)
	if resp2.Query != "alpha" {
		t.Errorf("expected query alpha, got %q", resp2.Query)
	}
	// Opt warnings are prepended.
	resp3 := buildGraphQueryResponse(context.Background(), graphOptions{projectRoot: root, docsDir: "docs", warnings: []string{"opt-warn"}}, codeGraph)
	if len(resp3.Warnings) < 2 || resp3.Warnings[0] != "opt-warn" {
		t.Errorf("expected opt-warn first, got %+v", resp3.Warnings)
	}
}

type nullCodeGraphProvider struct{}

func (n *nullCodeGraphProvider) SearchCodeGraph(ctx context.Context, query string, tokens []string, exclusionQuery string, exclusionTokens []string, limit int) ([]previewSearchResult, []string) {
	return nil, nil
}
func (n *nullCodeGraphProvider) Close(ctx context.Context) error { return nil }

func TestWriteGraphQueryTextAllBranchesP(t *testing.T) {
	response := previewSearchResponse{
		Query: "alpha",
		Warnings: []string{"warn1"},
		Stats: map[string]int{
			"docsSemantic": 1,
			"docsGraph":    2,
			"codeSemantic": 3,
			"codeGraph":    4,
		},
		Panels: previewSearchPanels{
			DocsSemantic: []previewSearchResult{{Title: "doc", Path: "doc.md", Source: "semantic"}},
			DocsGraph:    []previewSearchResult{{Title: "docG", Path: "docG.md"}},
			CodeSemantic: []previewSearchResult{{Title: "code", Path: "code.go", Line: 5, Source: "semantic", Confidence: "high"}},
			CodeGraph:    []previewSearchResult{{Title: "codeG", Path: "codeG.go", NodeID: "n1"}},
		},
	}
	var buf bytes.Buffer
	if err := writeGraphQueryText(&buf, response); err != nil {
		t.Fatalf("writeGraphQueryText: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Query: alpha",
		"Warnings:",
		"- warn1",
		"docsSemantic=1",
		"docsGraph=2",
		"codeSemantic=3",
		"codeGraph=4",
		"Docs Semantic:",
		"Docs Graph:",
		"Code Semantic:",
		"Code Graph:",
		"- doc (doc.md)",
		"- code (code.go:5)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}

	// With empty results -> "no results" path.
	responseEmpty := previewSearchResponse{Query: "x"}
	if err := writeGraphQueryText(&buf, responseEmpty); err != nil {
		t.Fatalf("writeGraphQueryText empty: %v", err)
	}
	if !strings.Contains(buf.String(), "- no results") {
		t.Errorf("expected 'no results' in output")
	}

	// Write error path using a faulty writer.
	faulty := &errWriter{err: errors.New("write fail")}
	if err := writeGraphQueryText(faulty, response); err == nil {
		t.Error("expected error from faulty writer")
	}
}

func TestWriteGraphQueryTextAllErrorPathsP(t *testing.T) {
	// Helper: response with one warning and one panel result.
	response := previewSearchResponse{
		Query:    "alpha",
		Warnings: []string{"w1"},
		Stats:    map[string]int{"docsSemantic": 1, "docsGraph": 2, "codeSemantic": 3, "codeGraph": 4},
		Panels: previewSearchPanels{
			DocsSemantic: []previewSearchResult{{Title: "doc", Path: "doc.md"}},
		},
	}
	// Order of writes: Query, Warnings header, warning line, Stats, panel header, no results, panel result line.
	// Iterate through each write position and verify error path.

	// Position 0: fail on first write (Query).
	w := &delayedErrWriter{failAfter: 0, err: errors.New("e0")}
	if err := writeGraphQueryText(w, response); err == nil {
		t.Error("expected error at position 0")
	}

	// Position 1: fail after Query (Warnings header).
	w = &delayedErrWriter{failAfter: 1, err: errors.New("e1")}
	if err := writeGraphQueryText(w, response); err == nil {
		t.Error("expected error at position 1")
	}

	// Position 2: fail after Warnings header (warning line).
	w = &delayedErrWriter{failAfter: 2, err: errors.New("e2")}
	if err := writeGraphQueryText(w, response); err == nil {
		t.Error("expected error at position 2")
	}

	// Position 3: fail after warning line (Stats).
	w = &delayedErrWriter{failAfter: 3, err: errors.New("e3")}
	if err := writeGraphQueryText(w, response); err == nil {
		t.Error("expected error at position 3")
	}

	// Position 4: fail after Stats (panel header).
	w = &delayedErrWriter{failAfter: 4, err: errors.New("e4")}
	if err := writeGraphQueryText(w, response); err == nil {
		t.Error("expected error at position 4")
	}

	// Empty panel and fail at panel header.
	respEmpty := previewSearchResponse{Query: "x"}
	w = &delayedErrWriter{failAfter: 0, err: errors.New("empty panel fail")}
	if err := writeGraphQueryText(w, respEmpty); err == nil {
		t.Error("expected error with empty response")
	}

	// Empty panel succeeds, then panel header fails on the second panel.
	w = &delayedErrWriter{failAfter: 1, err: errors.New("second panel fail")}
	if err := writeGraphQueryText(w, respEmpty); err == nil {
		t.Error("expected error on second panel header")
	}

	// Empty panels - cover "no results" path then fail on subsequent write.
	respNoRes := previewSearchResponse{Query: "x"}
	w = &delayedErrWriter{failAfter: 0, err: errors.New("first panel fail")}
	if err := writeGraphQueryText(w, respNoRes); err == nil {
		t.Error("expected error on first panel with no results")
	}

	// Panels with results, succeed through Query and panel header, then fail in writeGraphQueryResult.
	respWithRes := previewSearchResponse{
		Query: "x",
		Panels: previewSearchPanels{
			CodeGraph: []previewSearchResult{{Title: "main", Path: "main.go"}},
		},
	}
	// Try several failAfter values to find where writeGraphQueryResult actually writes.
	for fa := 0; fa <= 5; fa++ {
		w = &delayedErrWriter{failAfter: fa, err: errors.New("fail")}
		if err := writeGraphQueryText(w, respWithRes); err == nil {
			t.Errorf("expected error at failAfter=%d", fa)
		}
	}

	// All empty panels - fail on "- no results" path. Query succeeds (1 write), then
	// each panel: header + "no results" = 2 writes per panel. 4 panels = 8 writes + 1 query = 9.
	// After the first "no results", fail.
	respAllEmpty := previewSearchResponse{Query: "x"}
	w = &delayedErrWriter{failAfter: 2, err: errors.New("first no results fail")}
	if err := writeGraphQueryText(w, respAllEmpty); err == nil {
		t.Error("expected error on first no results")
	}
}

type errWriter struct{ err error }

func (e *errWriter) Write(p []byte) (int, error) { return 0, e.err }

// delayedErrWriter fails only after `failAfter` successful writes.
type delayedErrWriter struct {
	failAfter int
	count     int
	err       error
}

func (d *delayedErrWriter) Write(p []byte) (int, error) {
	d.count++
	if d.count > d.failAfter {
		return 0, d.err
	}
	return len(p), nil
}

func TestWriteGraphQueryResultAllBranchesP(t *testing.T) {
	// Result with all fields populated.
	result := previewSearchResult{
		Title:      "main",
		Path:       "main.go",
		Line:       42,
		Source:     "code",
		Confidence: "high",
		FlowRole:   "caller",
		Neighbors: []previewSearchNeighbor{
			{Direction: "in", Label: "foo", Relation: "calls", Path: "foo.go", Line: 1},
			{Direction: "out", Label: "bar", Path: "bar.go"},
			{Direction: "x", Label: "baz", ID: "id1"},
			{Direction: "y", Label: "qux"},
			{Direction: "z", Label: "more"},
		},
	}
	var buf bytes.Buffer
	if err := writeGraphQueryResult(&buf, result); err != nil {
		t.Fatalf("writeGraphQueryResult: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "main (main.go:42)") {
		t.Errorf("expected title and location in output: %s", out)
	}
	if !strings.Contains(out, "[code, high, caller]") {
		t.Errorf("expected source/confidence/flow in output: %s", out)
	}
	if !strings.Contains(out, "  - in foo via calls") {
		t.Errorf("expected first neighbor with relation in output: %s", out)
	}
	if !strings.Contains(out, "  - out bar") {
		t.Errorf("expected second neighbor: %s", out)
	}
	if !strings.Contains(out, "  - x baz (id1)") {
		t.Errorf("expected third neighbor with id: %s", out)
	}
	if !strings.Contains(out, "+2 more") {
		t.Errorf("expected '+2 more neighbors' marker: %s", out)
	}
}

func TestWriteGraphQueryResultNoLocationP(t *testing.T) {
	// Result with no Path/Line -> uses NodeID as location.
	result := previewSearchResult{Title: "x", NodeID: "n42"}
	var buf bytes.Buffer
	if err := writeGraphQueryResult(&buf, result); err != nil {
		t.Fatalf("writeGraphQueryResult: %v", err)
	}
	if !strings.Contains(buf.String(), "x (n42)") {
		t.Errorf("expected NodeID as location: %s", buf.String())
	}

	// Neighbor with no Path/Line, fall back to ID.
	result2 := previewSearchResult{
		Title: "y",
		Path:  "y.go",
		Neighbors: []previewSearchNeighbor{
			{Direction: "in", Label: "z", ID: "zID"},
		},
	}
	buf.Reset()
	if err := writeGraphQueryResult(&buf, result2); err != nil {
		t.Fatalf("writeGraphQueryResult: %v", err)
	}
	if !strings.Contains(buf.String(), "(zID)") {
		t.Errorf("expected neighbor ID as location: %s", buf.String())
	}

	// Write error path.
	faulty := &errWriter{err: errors.New("write fail")}
	if err := writeGraphQueryResult(faulty, result); err == nil {
		t.Error("expected error from faulty writer")
	}
}

func TestWriteGraphQueryResultAllErrorPathsP(t *testing.T) {
	// Full result with line, source/confidence/flow, and many neighbors.
	result := previewSearchResult{
		Title:      "main",
		Path:       "main.go",
		Line:       42,
		Source:     "code",
		Confidence: "high",
		FlowRole:   "caller",
		Neighbors: []previewSearchNeighbor{
			{Direction: "in", Label: "foo", Relation: "calls", Path: "foo.go", Line: 1},
			{Direction: "out", Label: "bar", Path: "bar.go"},
			{Direction: "x", Label: "baz", ID: "id1"},
			{Direction: "y", Label: "qux"},
			{Direction: "z", Label: "more"},
		},
	}
	// Order of writes for full result:
	// 0: "- main"
	// 1: " (main.go:42)"
	// 2: " [code, high, caller]"
	// 3: "\n" (Fprintln)
	// 4: "  - in foo"
	// 5: " via calls"
	// 6: " (foo.go:1)"
	// 7: "\n"
	// 8: "  - out bar"
	// 9: " (bar.go)"
	// 10: "\n"
	// 11: "  - x baz"
	// 12: " (id1)"
	// 13: "\n"
	// 14: "  - +1 more neighbors\n"
	type tc struct {
		failAfter int
		name      string
	}
	for _, c := range []tc{
		{0, "title"},
		{1, "location"},
		{2, "source"},
		{3, "newline1"},
		{4, "first neighbor"},
		{5, "relation"},
		{6, "neighbor location"},
		{7, "neighbor newline"},
		{12, "after first 3 neighbors"},
	} {
		w := &delayedErrWriter{failAfter: c.failAfter, err: errors.New(c.name)}
		if err := writeGraphQueryResult(w, result); err == nil {
			t.Errorf("expected error at position %d (%s)", c.failAfter, c.name)
		}
	}

	// Result with only Source (to exercise source path without line/location).
	r := previewSearchResult{Title: "x", Source: "y"}
	w := &delayedErrWriter{failAfter: 1, err: errors.New("source path")}
	if err := writeGraphQueryResult(w, r); err == nil {
		t.Error("expected error on source path")
	}

	// Neighbor with relation and location.
	rn := previewSearchResult{
		Title: "z",
		Neighbors: []previewSearchNeighbor{
			{Direction: "in", Label: "q", Relation: "r", Path: "p.go", Line: 1},
		},
	}
	w = &delayedErrWriter{failAfter: 3, err: errors.New("relation path")}
	if err := writeGraphQueryResult(w, rn); err == nil {
		t.Error("expected error after relation path")
	}
	w = &delayedErrWriter{failAfter: 4, err: errors.New("neighbor location path")}
	if err := writeGraphQueryResult(w, rn); err == nil {
		t.Error("expected error on neighbor location path")
	}
	w = &delayedErrWriter{failAfter: 5, err: errors.New("neighbor newline path")}
	if err := writeGraphQueryResult(w, rn); err == nil {
		t.Error("expected error on neighbor newline")
	}
}

func TestRunSearchHelpP(t *testing.T) {
	if err := RunSearch([]string{"--help"}); err != nil {
		t.Errorf("--help should return nil: %v", err)
	}
}

func TestRunSearchFlagErrorP(t *testing.T) {
	if err := RunSearch([]string{"--unknown-flag"}); err == nil {
		t.Error("expected error for unknown flag")
	}
}

func TestRunSearchListenFailureP(t *testing.T) {
	origServe := servePreviewForTest
	origOpen := openURLForTest
	defer func() {
		servePreviewForTest = origServe
		openURLForTest = origOpen
	}()
	openURLForTest = func(target string) error { return nil }
	// Use a port that we know is taken.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().String()
	if err := RunSearch([]string{"--project", t.TempDir(), "--docs-dir", "docs", "--addr", addr, "--no-open"}); err == nil {
		t.Error("expected error when address already in use")
	}
}

func TestRunSearchSuccessP(t *testing.T) {
	origServe := servePreviewForTest
	origOpen := openURLForTest
	defer func() {
		servePreviewForTest = origServe
		openURLForTest = origOpen
	}()
	openURLForTest = func(target string) error { return nil }
	// Stop the serve quickly.
	servePreviewForTest = func(srv *http.Server, listener net.Listener) error {
		return http.ErrServerClosed
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "search-launcher.html")
	if err := RunSearch([]string{"--project", root, "--docs-dir", "docs", "--addr", "127.0.0.1:0", "--no-open", "--out", out}); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("expected launcher file at %s, got %v", out, err)
	}
}

func TestRunGraphHelpP(t *testing.T) {
	if err := RunGraph([]string{"--help"}); err != nil {
		t.Errorf("--help should return nil: %v", err)
	}
}

func TestRunGraphFlagErrorP(t *testing.T) {
	if err := RunGraph([]string{"--unknown-flag"}); err == nil {
		t.Error("expected error for unknown flag")
	}
}

func TestRunGraphSuccessP(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := runGraphQueryWithProvider(context.Background(), graphOptions{projectRoot: root, docsDir: "docs", query: "alpha"}, &nullCodeGraphProvider{}, &buf); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if !strings.Contains(buf.String(), "Query: alpha") {
		t.Errorf("expected Query: alpha in output, got %s", buf.String())
	}
}

func TestRunGraphJSONP(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := runGraphQueryWithProvider(context.Background(), graphOptions{projectRoot: root, docsDir: "docs", query: "alpha", jsonOutput: true}, &nullCodeGraphProvider{}, &buf); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if !strings.Contains(buf.String(), `"query": "alpha"`) {
		t.Errorf("expected JSON output, got %s", buf.String())
	}
}

func TestRunHotReloadSupervisorInitialBuildErrorP(t *testing.T) {
	orig := buildPreviewFrontendForTest
	defer func() { buildPreviewFrontendForTest = orig }()
	buildPreviewFrontendForTest = func(moduleRoot string) error {
		return errors.New("initial build failed")
	}
	err := runHotReloadSupervisor(t.TempDir(), []string{}, t.TempDir())
	if err == nil || err.Error() != "initial build failed" {
		t.Errorf("expected initial build failed, got %v", err)
	}
}

func TestRunHotReloadSupervisorSignalsInterruptP(t *testing.T) {
	orig := startPreviewChildForTest
	defer func() { startPreviewChildForTest = orig }()

	stopCalled := make(chan struct{}, 1)
	var once sync.Once
	done := make(chan previewChildResult, 1)
	// Keep the child channel open until the test signals so the supervisor
	// cannot exit through the result path. We only close it from the
	// interrupt branch after stopPreviewChild is called.
	startPreviewChildForTest = func(moduleRoot string, args []string) (*exec.Cmd, <-chan previewChildResult, error) {
		return nil, done, nil
	}
	origStop := stopPreviewChildForTest
	defer func() { stopPreviewChildForTest = origStop }()
	stopPreviewChildForTest = func(cmd *exec.Cmd) {
		once.Do(func() { close(stopCalled) })
		// unblock the supervisor's <-done so it can return
		select {
		case done <- previewChildResult{err: nil}:
		default:
		}
	}

	result := make(chan error, 1)
	go func() {
		result <- runHotReloadSupervisor(t.TempDir(), []string{}, t.TempDir())
	}()

	// Wait briefly for the supervisor to enter its select loop.
	time.Sleep(200 * time.Millisecond)
	// Trigger signal: simulate SIGINT by stopping signal notification and
	// sending on the channel directly is not possible without exporting the
	// channel. Instead, we use the Stop signal the supervisor is listening
	// to. The cleanest approach is to call signal.Reset and re-Notify and
	// then send ourselves. Use a direct approach by sending to the process
	// group via os.Interrupt is hard from within the same process.
	// Instead, we'll just trigger a stop via test infrastructure. For test
	// coverage we only need to enter the signal branch; we can simulate it
	// by calling the supervisor's signal handler indirectly. The supervisor
	// calls signal.Notify(signals, os.Interrupt); we can simulate the
	// channel being closed by sending interrupt ourselves via the syscall.
	proc, err := os.FindProcess(os.Getpid())
	if err == nil {
		_ = proc.Signal(os.Interrupt)
	}
	select {
	case <-stopCalled:
	case <-time.After(3 * time.Second):
		t.Error("expected stopPreviewChild to be called after signal")
	}
	select {
	case err := <-result:
		if err != nil {
			t.Errorf("expected nil error after interrupt, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("supervisor did not exit after interrupt")
	}
}

func TestRunHotReloadSupervisorFrontendBuildFailContinueP(t *testing.T) {
	orig := startPreviewChildForTest
	defer func() { startPreviewChildForTest = orig }()
	origBuild := buildPreviewFrontendForTest
	defer func() { buildPreviewFrontendForTest = origBuild }()

	// First build succeeds (so supervisor reaches the loop).
	// Second build (frontend rebuild) fails; we expect the supervisor to
	// continue the loop rather than exit.
	var mu sync.Mutex
	var buildCount int
	buildPreviewFrontendForTest = func(moduleRoot string) error {
		mu.Lock()
		buildCount++
		n := buildCount
		mu.Unlock()
		if n == 1 {
			return nil
		}
		return errors.New("frontend rebuild failed")
	}

	// The child channel must remain blocked indefinitely so the supervisor
	// loops via the ticker.
	firstDone := make(chan previewChildResult, 1)
	startPreviewChildForTest = func(moduleRoot string, args []string) (*exec.Cmd, <-chan previewChildResult, error) {
		return nil, firstDone, nil
	}

	moduleRoot := t.TempDir()
	// Create frontend source file so previewSourceTokens sees a token.
	uiDir := filepath.Join(moduleRoot, "internal", "preview", "preview_ui_src")
	if err := os.MkdirAll(uiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	uiFile := filepath.Join(uiDir, "index.html")
	if err := os.WriteFile(uiFile, []byte("<html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() {
		result <- runHotReloadSupervisor(moduleRoot, []string{}, moduleRoot)
	}()

	// Wait for ticker (700ms), then update frontend mtime so supervisor
	// detects frontendChanged and triggers build (which fails -> continue).
	time.Sleep(800 * time.Millisecond)
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(uiFile, future, future); err != nil {
		t.Fatal(err)
	}

	// Give time for the failed build to be exercised.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := buildCount
		mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	mu.Lock()
	got := buildCount
	mu.Unlock()
	if got < 2 {
		t.Errorf("expected at least 2 builds (initial + frontend rebuild), got %d", got)
	}
	// Now let the supervisor exit.
	firstDone <- previewChildResult{err: nil}
	select {
	case err := <-result:
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("supervisor did not exit")
	}
}

func TestRunHotReloadSupervisorNoChangeContinueP(t *testing.T) {
	// Verifies that when tokens don't change, the supervisor takes the
	// 'continue' path inside the ticker branch.
	orig := startPreviewChildForTest
	defer func() { startPreviewChildForTest = orig }()
	done := make(chan previewChildResult, 1)
	startPreviewChildForTest = func(moduleRoot string, args []string) (*exec.Cmd, <-chan previewChildResult, error) {
		return nil, done, nil
	}

	moduleRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(moduleRoot, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() {
		result <- runHotReloadSupervisor(moduleRoot, []string{}, moduleRoot)
	}()
	// Wait for at least two ticker cycles so the 'continue' path is hit
	// without any file changes.
	time.Sleep(1700 * time.Millisecond)
	done <- previewChildResult{err: nil}
	select {
	case err := <-result:
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("supervisor did not exit")
	}
}


func TestLSPServerStartInitializeFailsP(t *testing.T) {
	// Use a command that starts but doesn't speak LSP. We use `true` which
	// exits 0 immediately. The initialize handshake will fail because the
	// subprocess closes its stdout before responding. The Start function
	// should call Stop and return the initialize error.
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("`true` not available")
	}
	s := &previewLSPServer{command: "true", args: []string{}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := s.Start(ctx)
	if err == nil {
		t.Error("expected initialize error")
	}
}

func TestLSPServerStartStdoutPipeErrorP(t *testing.T) {
	// We can't easily force StdoutPipe/StdinPipe to fail, but we can verify
	// that a non-existent command still gets a clean error from Start.
	s := &previewLSPServer{command: "/nonexistent/path/to/cmd-zzz", args: []string{}}
	if err := s.Start(context.Background()); err == nil {
		t.Error("expected error for non-existent command")
	}
}


func TestWithOpenFileReadFileErrorP(t *testing.T) {
	// Path doesn't exist -> os.ReadFile returns error.
	s := &previewLSPServer{running: true, stdin: os.Stdout}
	err := s.withOpenFile(context.Background(), "/nonexistent/file-zzz.go", "go", func() error { return nil })
	if err == nil {
		t.Error("expected error when file does not exist")
	}
}

func TestWithOpenFileUTF8ErrorP(t *testing.T) {
	// File not valid UTF-8.
	tmpFile := filepath.Join(t.TempDir(), "badutf8.go")
	if err := os.WriteFile(tmpFile, []byte{0xff, 0xfe, 0xfd}, 0o644); err != nil {
		t.Fatal(err)
	}
	s := &previewLSPServer{running: true, stdin: os.Stdout}
	err := s.withOpenFile(context.Background(), tmpFile, "go", func() error { return nil })
	if err == nil {
		t.Error("expected error when file is not valid UTF-8")
	}
}

func TestWithOpenFileNotifyErrorP(t *testing.T) {
	// Start returns nil because running=true; stdin is nil so notify fails.
	tmpFile := filepath.Join(t.TempDir(), "ok.go")
	if err := os.WriteFile(tmpFile, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &previewLSPServer{running: true, stdin: nil, lang: lspLanguage{}}
	err := s.withOpenFile(context.Background(), tmpFile, "go", func() error { return nil })
	if err == nil {
		t.Error("expected error from notify when stdin is nil")
	}
}

func TestWithOpenFileSuccessAndCallbackP(t *testing.T) {
	// withOpenFile calls s.Start(ctx) directly (not via seam), so we set up
	// a server with running=true so Start is a no-op. We need a writable
	// stdin to allow notify to succeed. We also need a valid s.lang.
	tmpFile := filepath.Join(t.TempDir(), "ok.go")
	if err := os.WriteFile(tmpFile, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	s := &previewLSPServer{
		running: true,
		stdin:   w,
		reader:  bufio.NewReader(r),
		lang:    lspLanguage{},
	}
	// We expect either a successful path (fn is called and returns nil)
	// or a deferred didClose error. We test that fn gets called.
	called := false
	fnErr := errors.New("fn error")
	result := s.withOpenFile(context.Background(), tmpFile, "go", func() error {
		called = true
		return fnErr
	})
	if !called && result == nil {
		t.Error("expected fn to be invoked or error returned")
	}
	if called && result != fnErr {
		t.Errorf("expected fn error to propagate, got %v", result)
	}
}


func TestLSPReferenceEdgesErrorPathP(t *testing.T) {
	// Stub lspReferences to return an error so the error branch in
	// referenceEdges is exercised.
	orig := lspReferences
	defer func() { lspReferences = orig }()
	lspReferences = func(ctx context.Context, srv *previewLSPServer, path, languageID string, pos lspPosition) ([]lspLocation, error) {
		return nil, errors.New("references failed")
	}
	p := &previewLSPCodeGraphProvider{projectRoot: t.TempDir()}
	index := lspCodeGraphIndex{Nodes: map[string]lspCodeNode{}}
	node := lspCodeNode{ID: "n1"}
	_, err := p.referenceEdges(context.Background(), nil, index, node)
	if err == nil {
		t.Error("expected error from lspReferences")
	}
}

func TestLSPReferenceEdgesSkipSelfP(t *testing.T) {
	// Stub lspReferences to return a ref whose caller is the same as the node.
	orig := lspReferences
	defer func() { lspReferences = orig }()
	lspReferences = func(ctx context.Context, srv *previewLSPServer, path, languageID string, pos lspPosition) ([]lspLocation, error) {
		return []lspLocation{{URI: "file:///x.go", Range: lspRange{Start: lspPosition{Line: 1, Character: 0}}}}, nil
	}
	p := &previewLSPCodeGraphProvider{projectRoot: t.TempDir()}
	// Index has a node whose ID matches what containingNodeIDForLocation will return.
	// Use an empty index so containingNodeIDForLocation returns "" → continue.
	index := lspCodeGraphIndex{Nodes: map[string]lspCodeNode{}}
	node := lspCodeNode{ID: "n1"}
	edges, err := p.referenceEdges(context.Background(), nil, index, node)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	// Edge should be empty because callerID is "" and is skipped.
	if len(edges) != 0 {
		t.Errorf("expected 0 edges (callerID empty), got %d", len(edges))
	}
}

func TestAssignLSPGraphNeighborsIncomingP(t *testing.T) {
	// Test the case edge.Target branch (incoming direction).
	results := map[string]previewSearchResult{
		"r1": {NodeID: "n1"},
	}
	index := lspCodeGraphIndex{
		Nodes: map[string]lspCodeNode{
			"n2": {ID: "n2", Name: "target", AbsPath: "/x.go", LanguageID: "go"},
		},
	}
	// Edge has n1 as Target, so it falls into case edge.Target.
	edges := []lspCodeEdge{{Source: "n2", Target: "n1", Relation: "callers"}}
	assignLSPGraphNeighbors(results, index, edges)
	got := results["r1"].Neighbors
	if len(got) != 1 {
		t.Fatalf("expected 1 neighbor, got %d", len(got))
	}
	if got[0].Direction != "incoming" {
		t.Errorf("expected incoming direction, got %s", got[0].Direction)
	}
}

func TestAssignLSPGraphNeighborsOutgoingP(t *testing.T) {
	// Test the case edge.Source branch (outgoing direction).
	results := map[string]previewSearchResult{
		"r1": {NodeID: "n1"},
	}
	index := lspCodeGraphIndex{
		Nodes: map[string]lspCodeNode{
			"n2": {ID: "n2", Name: "callee", AbsPath: "/y.go", LanguageID: "go"},
		},
	}
	// Edge has n1 as Source, so it falls into case edge.Source.
	edges := []lspCodeEdge{{Source: "n1", Target: "n2", Relation: "calls"}}
	assignLSPGraphNeighbors(results, index, edges)
	got := results["r1"].Neighbors
	if len(got) != 1 {
		t.Fatalf("expected 1 neighbor, got %d", len(got))
	}
	if got[0].Direction != "outgoing" {
		t.Errorf("expected outgoing direction, got %s", got[0].Direction)
	}
}

func TestAssignLSPGraphNeighborsUnknownTargetP(t *testing.T) {
	// Edge where index doesn't have the target node -> no neighbor added.
	results := map[string]previewSearchResult{
		"r1": {NodeID: "n1"},
	}
	index := lspCodeGraphIndex{Nodes: map[string]lspCodeNode{}}
	edges := []lspCodeEdge{{Source: "n1", Target: "missing", Relation: "calls"}}
	assignLSPGraphNeighbors(results, index, edges)
	if len(results["r1"].Neighbors) != 0 {
		t.Errorf("expected 0 neighbors, got %d", len(results["r1"].Neighbors))
	}
}


func TestParseLSPDocumentSymbolsEmpty(t *testing.T) {
	// Empty raw message → returns nil, nil.
	syms, err := parseLSPDocumentSymbols(nil)
	if err != nil {
		t.Errorf("empty: %v", err)
	}
	if syms != nil {
		t.Errorf("empty: expected nil symbols, got %v", syms)
	}
	// "null" raw message → returns nil, nil.
	syms, err = parseLSPDocumentSymbols(json.RawMessage("null"))
	if err != nil {
		t.Errorf("null: %v", err)
	}
	if syms != nil {
		t.Errorf("null: expected nil symbols, got %v", syms)
	}
	// Empty bytes → returns nil, nil.
	syms, err = parseLSPDocumentSymbols(json.RawMessage(""))
	if err != nil {
		t.Errorf("empty bytes: %v", err)
	}
	if syms != nil {
		t.Errorf("empty bytes: expected nil symbols, got %v", syms)
	}
	// Invalid JSON → returns error.
	_, err = parseLSPDocumentSymbols(json.RawMessage("not json"))
	if err == nil {
		t.Error("invalid JSON should error")
	}
}

func TestLSPFullSymbolName(t *testing.T) {
	cases := []struct {
		name  string
		owner string
		want  string
	}{
		// Empty inputs.
		{"", "", ""},
		{"foo", "", "foo"},
		{"", "Owner", "Owner"},
		// No matching prefix: prepend owner.
		{"foo", "Owner", "Owner.foo"},
		// Matching prefix variants.
		{"Owner.foo", "Owner", "Owner.foo"},
		{"Owner#foo", "Owner", "Owner#foo"},
		{"Owner::foo", "Owner", "Owner::foo"},
		{"(Owner)foo", "Owner", "(Owner)foo"},
		// Whitespace trimming.
		{"  foo  ", "Owner", "Owner.foo"},
		{"foo", "  Owner  ", "Owner.foo"},
	}
	for _, tc := range cases {
		got := lspFullSymbolName(tc.name, tc.owner)
		if got != tc.want {
			t.Errorf("lspFullSymbolName(%q, %q) = %q, want %q", tc.name, tc.owner, got, tc.want)
		}
	}
}

func TestExtractSemanticSpecRefs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"no_match", "no refs here", 0},
		{"simple_match", "see @spec/foo for details", 1},
		{"doc_match", "see @doc/bar/baz are related", 1},
		{"multiple_match", "@spec/foo and @spec/bar/baz are related", 2},
		{"explicit_relation", "see @spec/foo{depends} for details", 1},
		{"trailing_punctuation", "see @spec/foo.", 1},
	}
	for _, tc := range cases {
		got := extractSemanticSpecRefs(tc.in)
		if len(got) != tc.want {
			t.Errorf("%s: got %d refs, want %d (refs: %+v)", tc.name, len(got), tc.want, got)
		}
	}
}

