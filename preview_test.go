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
	if !strings.Contains(doc.Raw, "Hello **specs**.") || doc.HTML != "" {
		t.Fatalf("spec endpoint should return raw Markdown for client-side rendering: raw=%q html=%q", doc.Raw, doc.HTML)
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
	for _, want := range []string{"cdn.tailwindcss.com", "daisyui", "lucide", "markdown-it", "DOMPurify", "mermaid.min.js", "mermaid.render", "svg-pan-zoom", "d3.min.js"} {
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

func TestPreviewUIRendersDocsGraphWithD3(t *testing.T) {
	html, err := os.ReadFile("preview_ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	app, err := os.ReadFile("preview_ui/app.js")
	if err != nil {
		t.Fatal(err)
	}
	graphJS, err := os.ReadFile("preview_ui/js/graph.js")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(app) + "\n" + string(graphJS) + "\n" + string(css)
	for _, want := range []string{"data-tab=\"graph\"", "type=\"module\" src=\"/app.js\"", "id=\"graphCanvas\"", "fetchJSON(\"/api/graph\")", "createDocsGraph", "d3.forceSimulation", "normalizedGraphData", "graphSelectedId", "graph-details"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview docs graph UI missing %s", want)
		}
	}
}

func TestPreviewGraphLabelsUseDarkModeContrast(t *testing.T) {
	css, err := os.ReadFile("preview_ui/style.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(css)
	for _, want := range []string{`[data-theme="dark"] .graph-node text`, "fill: #f8fafc", "stroke: #020617"} {
		if !strings.Contains(text, want) {
			t.Fatalf("preview graph label dark mode contrast missing %s", want)
		}
	}
}

func TestPreviewUIRendersMarkdownClientSide(t *testing.T) {
	app, err := os.ReadFile("preview_ui/app.js")
	if err != nil {
		t.Fatal(err)
	}
	text := string(app)
	if strings.Contains(text, "fallbackHTML") || !strings.Contains(text, "markdownRenderer.render(raw)") {
		t.Fatalf("preview UI should render Markdown from raw content on the client")
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
	for _, want := range []string{"spec-preview-theme", "prefers-color-scheme: dark", "id=\"themeToggle\"", "applyTheme", "theme: state.theme"} {
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
	app, err := os.ReadFile("preview_ui/app.js")
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

func TestPreviewDiagramUsesSvgPanZoomAPI(t *testing.T) {
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
	for _, want := range []string{"data-diagram-action=\"zoom-in\"", "data-diagram-action=\"zoom-out\"", "data-diagram-action=\"fit\"", "diagram-zoom-level", "viewportSelector: \".svg-pan-zoom_viewport\"", "zoomEnabled: true", "panEnabled: true", "mouseWheelZoomEnabled: true", "zoomScaleSensitivity: 0.4", "instance.zoomIn()", "instance.zoomOut()", "instance.fit()", "instance.center()", "instance.resetZoom()", "instance.resetPan()"} {
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
	app, err := os.ReadFile("preview_ui/app.js")
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
	app, err := os.ReadFile("preview_ui/app.js")
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
	app, err := os.ReadFile("preview_ui/app.js")
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
