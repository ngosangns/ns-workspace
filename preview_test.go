package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestPreviewHTTPHandlers(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "AGENTS.md", "# Agents\n")
	writeTestFile(t, root, "specs/_index.md", `# Spec Index

## Modules

| Module | Spec File | Status | Version | Compliance | Priority |
| ------ | --------- | ------ | ------- | ---------- | -------- |
| Overview | [overview.md](./overview.md) | Finalized | v1.0 | - | - |

## Dependency Graph

`+"```"+`
overview → editor.core
`+"```"+`
`)
	writeTestFile(t, root, "specs/overview.md", "# Overview\n\nHello **specs**.\n")

	server := newPreviewServer(previewOptions{projectRoot: root, specsDir: "specs", addr: "127.0.0.1:0"})
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/specs")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var docs []specDocument
	if err := json.NewDecoder(res.Body).Decode(&docs); err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected _index and overview docs, got %d", len(docs))
	}

	res, err = http.Get(ts.URL + "/api/specs/overview.md")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var doc specDocument
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.HTML, "<strong>specs</strong>") {
		t.Fatalf("markdown was not rendered: %s", doc.HTML)
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
}

func TestPreviewLikeC4RenderHandler(t *testing.T) {
	server := newPreviewServer(previewOptions{projectRoot: t.TempDir(), specsDir: "specs", addr: "127.0.0.1:0"})
	server.likeC4Renderer = func(_ context.Context, source string) ([]likeC4Diagram, error) {
		if !strings.Contains(source, "workspace") {
			t.Fatalf("renderer did not receive source: %s", source)
		}
		return []likeC4Diagram{{Name: "index", Mermaid: "graph TB\n  A-->B\n"}}, nil
	}
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Post(ts.URL+"/api/render/likec4", "application/json", strings.NewReader(`{"source":"workspace { }"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", res.Status)
	}
	var body likeC4RenderResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Diagrams) != 1 || body.Diagrams[0].Name != "index" || !strings.Contains(body.Diagrams[0].Mermaid, "A-->B") {
		t.Fatalf("unexpected LikeC4 response: %+v", body)
	}
}

func TestPreviewLikeC4ModelsFallsBackToSpecGraph(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "AGENTS.md", "# Agents\n")
	writeTestFile(t, root, "specs/_index.md", `# Spec Index

## Modules

| Module | Spec File | Status | Version | Compliance | Priority |
| ------ | --------- | ------ | ------- | ---------- | -------- |
| API | [modules/api.md](./modules/api.md) | Draft | v1.0 | - | P1 |
| Web | [modules/web.md](./modules/web.md) | Draft | v1.0 | - | P1 |

## Dependency Graph

`+"```"+`
web → api
`+"```"+`
`)
	writeTestFile(t, root, "specs/modules/api.md", "# API\n")
	writeTestFile(t, root, "specs/modules/web.md", "# Web\n")

	server := newPreviewServer(previewOptions{projectRoot: root, specsDir: "specs", addr: "127.0.0.1:0"})
	server.likeC4Renderer = func(_ context.Context, source string) ([]likeC4Diagram, error) {
		if !strings.Contains(source, "specs.web -[depends]-> specs.api") {
			t.Fatalf("generated LikeC4 source missing spec edge:\n%s", source)
		}
		return []likeC4Diagram{{Name: "specs-overview", Mermaid: "graph TB\n  web-->api\n"}}, nil
	}
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Get(ts.URL + "/api/likec4")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body likeC4ModelsResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Projects) != 1 || !body.Projects[0].Generated || !strings.Contains(body.Projects[0].Source, "view specs-overview") {
		t.Fatalf("unexpected LikeC4 models response: %+v", body)
	}
}

func TestPreviewHelpIsAccepted(t *testing.T) {
	if err := run([]string{"preview", "--help"}); err != nil {
		t.Fatalf("preview help failed: %v", err)
	}
}

func TestPreviewUIUsesProjectSummaryResponse(t *testing.T) {
	data, err := os.ReadFile("preview_ui/app.js")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "project.summary") {
		t.Fatalf("preview UI should use /api/project summary response directly")
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
	htmlText := string(html)
	appText := string(app)
	for _, want := range []string{"cdn.tailwindcss.com", "daisyui", "lucide", "markdown-it", "DOMPurify", "mermaid", "cytoscape"} {
		if !strings.Contains(htmlText, want) && !strings.Contains(appText, want) {
			t.Fatalf("preview UI missing %s integration", want)
		}
	}
}

func TestPreviewUIPrefersServerRenderedMarkdownHTML(t *testing.T) {
	app, err := os.ReadFile("preview_ui/app.js")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	if !strings.Contains(text, "if (fallbackHTML)") || !strings.Contains(text, "DOMPurify.sanitize(fallbackHTML)") {
		t.Fatalf("preview UI should prefer server-rendered Markdown HTML so GFM tables stay consistent")
	}
}

func TestPreviewMarkdownTablesWrapLongCellContent(t *testing.T) {
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(css)
	for _, want := range []string{".markdown td code", "overflow-wrap: anywhere", "word-break: break-word", "overflow-x: auto"} {
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
	app, err := os.ReadFile("preview_ui/app.js")
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
	app, err := os.ReadFile("preview_ui/app.js")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"new EventSource(\"/api/events\")", "reloadPreviewData", "addEventListener(\"change\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI hot reload missing %s", want)
		}
	}
}

func TestPreviewUISupportsLikeC4Blocks(t *testing.T) {
	app, err := os.ReadFile("preview_ui/app.js")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"renderLikeC4Blocks", "/api/render/likec4", "language-likec4", "language-c4", "workflow"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI LikeC4 support missing %s", want)
		}
	}
}

func TestPreviewUIHasLikeC4ModelsTab(t *testing.T) {
	html, err := os.ReadFile("preview_ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile("preview_ui/app.js")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(app)
	for _, want := range []string{"data-tab=\"models\"", "id=\"likec4Models\"", "/api/likec4", "renderLikeC4Models", "Generated from specs"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI LikeC4 models tab missing %s", want)
		}
	}
}

func TestPreviewDiagramSanitizerKeepsMermaidLabels(t *testing.T) {
	app, err := os.ReadFile("preview_ui/app.js")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"USE_PROFILES", "foreignObject", "\"div\"", "\"span\"", "\"style\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview diagram sanitizer missing %s, Mermaid/LikeC4 labels may be stripped", want)
		}
	}
}

func TestPreviewUISupportsDarkMode(t *testing.T) {
	html, err := os.ReadFile("preview_ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile("preview_ui/app.js")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(app)
	for _, want := range []string{"spec-preview-theme", "prefers-color-scheme: dark", "id=\"themeToggle\"", "applyTheme", "graphPalette", "theme === \"dark\" ? \"dark\" : \"default\""} {
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
