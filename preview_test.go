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
}

func TestPreviewMermaidRenderHandler(t *testing.T) {
	server := newPreviewServer(previewOptions{projectRoot: t.TempDir(), specsDir: "specs", addr: "127.0.0.1:0"})
	server.mermaidRenderer = func(_ context.Context, source, theme string) (string, error) {
		if !strings.Contains(source, "A-->B") {
			t.Fatalf("renderer did not receive source: %s", source)
		}
		if theme != "dark" {
			t.Fatalf("renderer did not receive theme: %s", theme)
		}
		return `<svg viewBox="0 0 10 10"><text>A</text></svg>`, nil
	}
	ts := httptest.NewServer(server.srv.Handler)
	defer ts.Close()
	defer func() { _ = server.shutdown(context.Background()) }()

	res, err := http.Post(ts.URL+"/api/render/mermaid", "application/json", strings.NewReader(`{"source":"graph TB\nA-->B","theme":"dark"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", res.Status)
	}
	var body mermaidRenderResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.SVG, "<svg") || !strings.Contains(body.SVG, "<text>A</text>") {
		t.Fatalf("unexpected Mermaid response: %+v", body)
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
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	htmlText := string(html)
	appText := string(app) + "\n" + string(css)
	for _, want := range []string{"cdn.tailwindcss.com", "daisyui", "lucide", "markdown-it", "DOMPurify", "/api/render/mermaid", "cytoscape"} {
		if !strings.Contains(htmlText, want) && !strings.Contains(appText, want) {
			t.Fatalf("preview UI missing %s integration", want)
		}
	}
	if strings.Contains(htmlText, "mermaid.min.js") || strings.Contains(appText, "mermaid.render") || strings.Contains(appText, "mermaid.initialize") {
		t.Fatalf("preview UI should render Mermaid on the server side")
	}
	for _, forbidden := range []string{"data-ui-kit=\"treact\"", "Treact-style component primitives"} {
		if strings.Contains(htmlText, forbidden) || strings.Contains(appText, forbidden) {
			t.Fatalf("preview UI should use full DaisyUI instead of %s", forbidden)
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

func TestPreviewUIUpdatesURLForFocusedTabs(t *testing.T) {
	app, err := os.ReadFile("preview_ui/app.js")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"routeFromLocation", "updateRouteURL", "window.history.pushState", "window.location.pathname", "popstate", "encodeSpecPath", "join(\"/\")"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview UI route handling missing %s", want)
		}
	}
	if strings.Contains(text, "hashchange") || strings.Contains(text, "window.location.hash") {
		t.Fatalf("preview UI should use path routing without hash fragments")
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
			t.Fatalf("preview diagram sanitizer missing %s, Mermaid labels may be stripped", want)
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
	for _, want := range []string{"spec-preview-theme", "prefers-color-scheme: dark", "id=\"themeToggle\"", "applyTheme", "graphPalette", "theme: state.theme"} {
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

func TestPreviewUISupportsDiagramLightbox(t *testing.T) {
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
	text := string(html) + "\n" + string(app) + "\n" + string(css)
	for _, want := range []string{"id=\"diagramLightbox\"", "openDiagramLightbox", "decorateDiagram", "showModal()", "diagram-surface", "diagram-zoom", "diagram-lightbox__content"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview diagram lightbox missing %s", want)
		}
	}
}

func TestPreviewDiagramLightboxSupportsZoomPan(t *testing.T) {
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
	text := string(html) + "\n" + string(app) + "\n" + string(css)
	for _, want := range []string{"id=\"diagramZoomIn\"", "id=\"diagramZoomOut\"", "id=\"diagramZoomFit\"", "diagramZoomLevel", "zoomLightbox", "fitLightboxDiagram", "pointerdown", "wheel", "diagram-lightbox__stage", "is-panning"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview diagram lightbox zoom/pan missing %s", want)
		}
	}
}

func TestPreviewDiagramLightboxZoomKeepsSvgSharp(t *testing.T) {
	app, err := os.ReadFile("preview_ui/app.js")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"dataset.baseWidth", "dataset.baseHeight", "renderWidth", "renderHeight", "stage.style.transform = `translate(${state.lightbox.x}px, ${state.lightbox.y}px)`"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview diagram lightbox sharp SVG zoom missing %s", want)
		}
	}
	if strings.Contains(text, "scale(${state.lightbox.scale})") {
		t.Fatalf("preview diagram lightbox should resize SVG instead of CSS-scaling the stage")
	}
}

func TestPreviewDiagramLightboxPreservesSvgSize(t *testing.T) {
	app, err := os.ReadFile("preview_ui/app.js")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	for _, want := range []string{"svgDiagramSize", "clone.setAttribute(\"width\"", "clone.setAttribute(\"height\"", "clone.style.width", "clone.style.height"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview diagram lightbox SVG sizing missing %s", want)
		}
	}
	if strings.Contains(text, "clone.removeAttribute(\"width\")") || strings.Contains(text, "clone.removeAttribute(\"height\")") {
		t.Fatalf("preview diagram lightbox should preserve explicit SVG size")
	}
}

func TestPreviewDiagramLightboxUsesHiddenOverflowAndBackground(t *testing.T) {
	app, err := os.ReadFile("preview_ui/app.js")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app) + "\n" + string(css)
	for _, want := range []string{"overflow: hidden", "--diagram-lightbox-bg: #ffffff", "html[data-theme=\"dark\"]", "background-color: var(--diagram-lightbox-bg)", "--diagram-canvas-bg: #f3f4f6", "touch-action: none"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview diagram lightbox hidden overflow/background missing %s", want)
		}
	}
	if strings.Contains(text, ".diagram-lightbox__stage") && strings.Contains(text, "box-shadow: 0 12px 36px") {
		t.Fatalf("preview diagram lightbox stage should not have a diagram shadow")
	}
	for _, forbidden := range []string{"injectSvgBackground", "diagram-lightbox__svg-bg", "clone.style.background"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("preview diagram lightbox should not add background to diagram SVG: %s", forbidden)
		}
	}
	if strings.Contains(text, ".diagram-lightbox::backdrop {\n  background: rgb(0 0 0 / 0.62);\n  backdrop-filter") {
		t.Fatalf("preview diagram lightbox backdrop should not use blur")
	}
	if strings.Contains(text, "scrollbar-gutter: stable both-edges") {
		t.Fatalf("preview diagram lightbox viewport should not reserve scrollbar gutter")
	}
}

func TestPreviewDiagramLightboxCentersDiagram(t *testing.T) {
	app, err := os.ReadFile("preview_ui/app.js")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app) + "\n" + string(css)
	for _, want := range []string{"const stage = els.diagramLightboxContent.querySelector", "stageWidth", "stageHeight", "margin: 0 auto"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview diagram lightbox centering missing %s", want)
		}
	}
}

func TestPreviewDiagramLightboxHasCanvasBackground(t *testing.T) {
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(css)
	for _, want := range []string{"--diagram-canvas-bg", "--diagram-grid-line", "background-image:", "background-size: 24px 24px", "--diagram-stage-bg"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview diagram lightbox background missing %s", want)
		}
	}
}
