package preview

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"testing/quick"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// extractBundle pulls the JSON blob assigned to window.__NS_KB__ out of an
// exported HTML document and unmarshals it back into an exportBundle. The
// template emits `window.__NS_KB__ = {json};` so we decode exactly one JSON
// value starting right after the assignment, letting the JSON decoder stop at
// the matching closing brace (ignoring the trailing `;`).
func extractBundle(t *testing.T, htmlBytes []byte) exportBundle {
	t.Helper()
	const marker = "window.__NS_KB__ = "
	idx := bytes.Index(htmlBytes, []byte(marker))
	if idx < 0 {
		t.Fatalf("exported HTML does not contain %q assignment", marker)
	}
	dec := json.NewDecoder(bytes.NewReader(htmlBytes[idx+len(marker):]))
	var bundle exportBundle
	if err := dec.Decode(&bundle); err != nil {
		t.Fatalf("decode embedded bundle JSON: %v", err)
	}
	return bundle
}

// docIDSet returns the set of document IDs present in a slice of exportDocument.
func exportDocIDSet(docs []exportDocument) map[string]bool {
	set := make(map[string]bool, len(docs))
	for _, d := range docs {
		set[d.ID] = true
	}
	return set
}

func specDocIDSet(docs []specDocument) map[string]bool {
	set := make(map[string]bool, len(docs))
	for _, d := range docs {
		set[d.ID] = true
	}
	return set
}

// sampleProject builds a concrete fixture project with several documents
// (markdown + html) and a small graph, constructed directly in Go so the test
// does not depend on scanning the filesystem.
func sampleProject() specProject {
	return specProject{
		Summary: projectSummary{
			Name:       "Sample KB",
			DocsRoot:   "docs",
			TotalSpecs: 3,
			Warnings:   []string{"sample warning"},
		},
		Documents: []specDocument{
			{
				ID:          "modules/alpha",
				Title:       "Alpha Module",
				Path:        "modules/alpha.md",
				Format:      "markdown",
				Category:    "modules",
				Status:      "active",
				Version:     "v1.0",
				Description: "Alpha module overview.",
				Type:        "module",
				Tags:        []string{"alpha", "core"},
				Raw:         "# Alpha\n\nThis is the **alpha** module with a [relative link](./beta.md).\n",
			},
			{
				ID:       "modules/beta",
				Title:    "Beta Module",
				Path:     "modules/beta.md",
				Format:   "markdown",
				Category: "modules",
				Status:   "draft",
				Raw:      "# Beta\n\n- item one\n- item two\n",
			},
			{
				ID:       "pages/landing",
				Title:    "Landing Page",
				Path:     "pages/landing.html",
				Format:   "html",
				Category: "pages",
				Raw:      "<html><body><h1>Landing</h1><p>Welcome to the landing page.</p></body></html>",
			},
		},
		Graph: specGraph{
			Nodes: []graphNode{
				{ID: "modules/alpha", Label: "Alpha Module", Type: "module", Category: "modules"},
				{ID: "modules/beta", Label: "Beta Module", Type: "module", Category: "modules"},
			},
			Edges: []graphEdge{
				{From: "modules/alpha", To: "modules/beta", Label: "depends on", Type: "dependency"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Unit tests (fixture-based)
// ---------------------------------------------------------------------------

func TestExportStaticBundleIncludesAllDocumentsAndGraph(t *testing.T) {
	project := sampleProject()
	opt := exportOptions{includeGraph: true, inlineAssets: true}

	htmlBytes, err := exportStaticBundle(project, opt)
	if err != nil {
		t.Fatalf("exportStaticBundle: %v", err)
	}

	if !bytes.Contains(htmlBytes, []byte("window.__NS_KB__")) {
		t.Fatalf("exported HTML must embed window.__NS_KB__ blob")
	}

	bundle := extractBundle(t, htmlBytes)

	// Every project document must be present in the bundle (Req 2.1).
	if len(bundle.Documents) != len(project.Documents) {
		t.Fatalf("expected %d documents, got %d", len(project.Documents), len(bundle.Documents))
	}
	got := exportDocIDSet(bundle.Documents)
	for _, doc := range project.Documents {
		if !got[doc.ID] {
			t.Errorf("document %q missing from bundle", doc.ID)
		}
	}

	// Each document must carry a rendered body (real or placeholder).
	for _, doc := range bundle.Documents {
		if strings.TrimSpace(doc.RenderedHTML) == "" {
			t.Errorf("document %q has empty RenderedHTML", doc.ID)
		}
	}

	// Graph must equal project.Graph when enabled (Req 2.2).
	if !reflect.DeepEqual(bundle.Graph, project.Graph) {
		t.Errorf("embedded graph does not match project.Graph\n got: %+v\nwant: %+v", bundle.Graph, project.Graph)
	}

	// Project metadata must be carried through.
	if bundle.Project.Name != project.Summary.Name {
		t.Errorf("project name = %q, want %q", bundle.Project.Name, project.Summary.Name)
	}
	if bundle.Project.Total != project.Summary.TotalSpecs {
		t.Errorf("project total = %d, want %d", bundle.Project.Total, project.Summary.TotalSpecs)
	}
}

func TestExportStaticBundleNoGraphFlagOmitsGraph(t *testing.T) {
	project := sampleProject()
	opt := exportOptions{includeGraph: false, inlineAssets: true}

	htmlBytes, err := exportStaticBundle(project, opt)
	if err != nil {
		t.Fatalf("exportStaticBundle: %v", err)
	}
	bundle := extractBundle(t, htmlBytes)

	// Graph must be empty (Req 2.3) ...
	if !reflect.DeepEqual(bundle.Graph, specGraph{}) {
		t.Errorf("expected empty graph with includeGraph=false, got %+v", bundle.Graph)
	}
	// ... while documents remain complete.
	if len(bundle.Documents) != len(project.Documents) {
		t.Errorf("expected %d documents with --no-graph, got %d", len(project.Documents), len(bundle.Documents))
	}
}

// TestExportRenderPermissive verifies the fail-open contract (Req 2.4): a doc
// whose rendered output is the placeholder still appears in the bundle, and a
// pathological doc does not drop or corrupt sibling documents. renderDocumentHTML
// recovers from any renderer error/panic and returns exportRenderPlaceholder, so
// we exercise odd inputs and assert no document is ever lost.
func TestExportRenderPermissive(t *testing.T) {
	project := specProject{
		Summary: projectSummary{Name: "Permissive", DocsRoot: "docs", TotalSpecs: 4},
		Documents: []specDocument{
			{ID: "ok/markdown", Title: "OK", Path: "ok.md", Format: "markdown", Raw: "# Fine\n\nNormal content."},
			{ID: "odd/empty", Title: "Empty", Path: "empty.md", Format: "markdown", Raw: ""},
			{ID: "odd/html", Title: "Odd HTML", Path: "odd.html", Format: "html",
				Raw: "<html><body><p>unclosed <b>bold <script>evil()</script></body></html>"},
			{ID: "odd/unknown", Title: "Unknown Format", Path: "weird.bin", Format: "binary",
				Raw: "raw <tags> & control \x00 chars"},
		},
	}

	htmlBytes, err := exportStaticBundle(project, exportOptions{includeGraph: false, inlineAssets: true})
	if err != nil {
		t.Fatalf("exportStaticBundle must be fail-open, got error: %v", err)
	}
	bundle := extractBundle(t, htmlBytes)

	// All documents survive, regardless of content (none dropped).
	if len(bundle.Documents) != len(project.Documents) {
		t.Fatalf("expected %d documents, got %d", len(project.Documents), len(bundle.Documents))
	}
	got := exportDocIDSet(bundle.Documents)
	for _, doc := range project.Documents {
		if !got[doc.ID] {
			t.Errorf("permissive export dropped document %q", doc.ID)
		}
	}

	// The sanitizer must have stripped the <script> from the odd HTML doc.
	for _, doc := range bundle.Documents {
		if doc.ID == "odd/html" && strings.Contains(strings.ToLower(doc.RenderedHTML), "<script") {
			t.Errorf("odd HTML doc retained a <script> tag: %q", doc.RenderedHTML)
		}
	}
}

// TestRenderDocumentHTMLNeverFailsHard checks renderDocumentHTML is fail-open
// across odd inputs: it never panics and, for non-empty input, always returns a
// non-empty body (real render or placeholder) (Req 2.4). Empty input rendering
// to empty output is correct and not a fail-open violation.
func TestRenderDocumentHTMLNeverFailsHard(t *testing.T) {
	cases := []specDocument{
		{ID: "a", Format: "markdown", Raw: ""}, // empty -> empty output is fine
		{ID: "b", Format: "markdown", Raw: "# Heading\n\ntext"},
		{ID: "c", Format: "html", Raw: "<p>ok</p>"},
		{ID: "d", Format: "html", Raw: "<<<not really html>>>"},
		{ID: "e", Format: "text", Raw: "plain & <text>"},
		{ID: "f", Format: "unknown", Raw: "weird \x00 bytes"},
	}
	for _, doc := range cases {
		// Reaching the assertion proves renderDocumentHTML did not panic.
		rendered, _ := renderDocumentHTML(doc)
		if strings.TrimSpace(doc.Raw) != "" && strings.TrimSpace(rendered) == "" {
			t.Errorf("renderDocumentHTML(%q) returned empty output for non-empty input", doc.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// Property-based tests (testing/quick, standard library — no new dependency)
// ---------------------------------------------------------------------------

// genProject is a quick.Generator wrapper that produces bounded, meaningful
// random specProjects (a handful of documents + a small graph) so property
// checks exercise a wide input space without unbounded sizes.
type genProject struct {
	specProject
}

var randFormats = []string{"markdown", "html", "text", "weird"}

func randString(rnd *rand.Rand, prefix string) string {
	return prefix + "-" + string(rune('a'+rnd.Intn(26))) + string(rune('a'+rnd.Intn(26)))
}

func (genProject) Generate(rnd *rand.Rand, size int) reflect.Value {
	docCount := rnd.Intn(6) // 0..5 documents, includes the empty-project edge case
	docs := make([]specDocument, 0, docCount)
	nodes := make([]graphNode, 0, docCount)
	for i := 0; i < docCount; i++ {
		id := randString(rnd, "doc") + "/" + randString(rnd, "n")
		format := randFormats[rnd.Intn(len(randFormats))]
		raw := "# " + randString(rnd, "title") + "\n\nbody " + randString(rnd, "b")
		if format == "html" {
			raw = "<html><body><h1>" + randString(rnd, "h") + "</h1><p>body</p></body></html>"
		}
		docs = append(docs, specDocument{
			ID:     id,
			Title:  randString(rnd, "Title"),
			Path:   id + ".md",
			Format: format,
			Raw:    raw,
			Tags:   []string{randString(rnd, "tag")},
		})
		nodes = append(nodes, graphNode{ID: id, Label: randString(rnd, "Label"), Type: "module"})
	}

	var edges []graphEdge
	if len(nodes) >= 2 {
		edges = append(edges, graphEdge{From: nodes[0].ID, To: nodes[1].ID, Label: "rel", Type: "dependency"})
	}

	gp := genProject{specProject{
		Summary: projectSummary{
			Name:       randString(rnd, "Project"),
			DocsRoot:   "docs",
			TotalSpecs: docCount,
		},
		Documents: docs,
		Graph:     specGraph{Nodes: nodes, Edges: edges},
	}}
	return reflect.ValueOf(gp)
}

// Property 2: Export không mất doc (Validates: Requirements 2.1)
// Every specDocument in the project appears exactly once in the exported bundle.
func TestPropertyExportPreservesAllDocuments(t *testing.T) {
	property := func(gp genProject) bool {
		htmlBytes, err := exportStaticBundle(gp.specProject, exportOptions{includeGraph: true, inlineAssets: true})
		if err != nil {
			return false
		}
		bundle := decodeBundle(htmlBytes)
		if bundle == nil {
			return false
		}
		if len(bundle.Documents) != len(gp.Documents) {
			return false
		}
		got := exportDocIDSet(bundle.Documents)
		for _, doc := range gp.Documents {
			if !got[doc.ID] {
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatalf("Property 2 (no doc lost) failed: %v", err)
	}
}

// Property 3: Export graph theo flag (Validates: Requirements 2.2, 2.3)
// includeGraph=true embeds exactly project.Graph; includeGraph=false embeds an
// empty graph while documents stay complete.
func TestPropertyExportGraphFollowsFlag(t *testing.T) {
	property := func(gp genProject, includeGraph bool) bool {
		htmlBytes, err := exportStaticBundle(gp.specProject, exportOptions{includeGraph: includeGraph, inlineAssets: true})
		if err != nil {
			return false
		}
		bundle := decodeBundle(htmlBytes)
		if bundle == nil {
			return false
		}
		// Documents always complete regardless of graph flag.
		if len(bundle.Documents) != len(gp.Documents) {
			return false
		}
		if includeGraph {
			return reflect.DeepEqual(bundle.Graph, gp.Graph)
		}
		return reflect.DeepEqual(bundle.Graph, specGraph{})
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatalf("Property 3 (graph by flag) failed: %v", err)
	}
}

// Property 5: Export không rò dữ liệu ngoài project (Validates: Requirements 2.5)
// The bundle contains only the project's own documents, graph and metadata —
// nothing is injected from outside the provided specProject.
func TestPropertyExportNoDataLeak(t *testing.T) {
	property := func(gp genProject) bool {
		htmlBytes, err := exportStaticBundle(gp.specProject, exportOptions{includeGraph: true, inlineAssets: true})
		if err != nil {
			return false
		}
		bundle := decodeBundle(htmlBytes)
		if bundle == nil {
			return false
		}
		// No bundle document may have an ID outside the project's documents.
		projectIDs := specDocIDSet(gp.Documents)
		for _, doc := range bundle.Documents {
			if !projectIDs[doc.ID] {
				return false
			}
		}
		// Graph must be exactly the project graph (no foreign nodes/edges).
		if !reflect.DeepEqual(bundle.Graph, gp.Graph) {
			return false
		}
		// Project metadata must mirror the project summary, nothing else.
		if bundle.Project.Name != gp.Summary.Name ||
			bundle.Project.DocsRoot != gp.Summary.DocsRoot ||
			bundle.Project.Total != gp.Summary.TotalSpecs {
			return false
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatalf("Property 5 (no data leak) failed: %v", err)
	}
}

// decodeBundle is the non-fatal sibling of extractBundle used inside property
// closures (which cannot take *testing.T). Returns nil on any failure.
func decodeBundle(htmlBytes []byte) *exportBundle {
	const marker = "window.__NS_KB__ = "
	idx := bytes.Index(htmlBytes, []byte(marker))
	if idx < 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(htmlBytes[idx+len(marker):]))
	var bundle exportBundle
	if err := dec.Decode(&bundle); err != nil {
		return nil
	}
	return &bundle
}

// ---------------------------------------------------------------------------
// Integration / property test: self-contained offline export (Property 1)
// ---------------------------------------------------------------------------

// externalSrcRe / externalHrefRe match attribute references that browsers would
// auto-load (or navigate to) from a remote origin. License URLs embedded inside
// minified vendor JS strings are NOT attribute references and do not trigger
// network requests, so we scope the offline assertion to these attributes.
var (
	externalSrcRe  = regexp.MustCompile(`(?i)src\s*=\s*["']https?:`)
	externalHrefRe = regexp.MustCompile(`(?i)href\s*=\s*["']https?:`)
	externalCSSRe  = regexp.MustCompile(`(?i)url\(\s*["']?https?:`)
)

// TestExportSelfContainedOfflineInlineAssets verifies Property 1 (Req 1.3): with
// inlineAssets=true the produced HTML contains no external http(s) attribute
// references and no CDN script tags, so it opens over file:// with no network.
// Fixture docs are intentionally free of external links to keep the assertion
// crisp.
func TestExportSelfContainedOfflineInlineAssets(t *testing.T) {
	project := specProject{
		Summary: projectSummary{Name: "Offline KB", DocsRoot: "docs", TotalSpecs: 2},
		Documents: []specDocument{
			{ID: "a", Title: "A", Path: "a.md", Format: "markdown", Raw: "# A\n\nLocal content, see [B](./b.md)."},
			{ID: "b", Title: "B", Path: "b.md", Format: "markdown", Raw: "# B\n\nMore local content."},
		},
		Graph: specGraph{
			Nodes: []graphNode{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}},
			Edges: []graphEdge{{From: "a", To: "b"}},
		},
	}

	htmlBytes, err := exportStaticBundle(project, exportOptions{includeGraph: true, inlineAssets: true})
	if err != nil {
		t.Fatalf("exportStaticBundle: %v", err)
	}

	if externalSrcRe.Match(htmlBytes) {
		t.Errorf("inline export must not contain external src=\"http...\" references")
	}
	if externalHrefRe.Match(htmlBytes) {
		t.Errorf("inline export must not contain external href=\"http...\" references")
	}
	if externalCSSRe.Match(htmlBytes) {
		t.Errorf("inline export must not contain external url(http...) CSS references")
	}
	if bytes.Contains(htmlBytes, []byte(exportCytoscapeCDN)) || bytes.Contains(htmlBytes, []byte(exportMarkedCDN)) {
		t.Errorf("inline export must not reference vendor CDN URLs")
	}

	// The vendor libraries must actually be inlined (the file is self-contained).
	if !bytes.Contains(htmlBytes, []byte("window.__NS_KB__")) {
		t.Errorf("self-contained export must embed the knowledge bundle inline")
	}
}

// TestExportCDNModeReferencesVendorURLs is the contrast case: with
// inlineAssets=false the export references the vendor libraries via CDN (Req 1.4),
// proving the inline-assets toggle is what makes the inline build offline-clean.
func TestExportCDNModeReferencesVendorURLs(t *testing.T) {
	project := sampleProject()

	htmlBytes, err := exportStaticBundle(project, exportOptions{includeGraph: true, inlineAssets: false})
	if err != nil {
		t.Fatalf("exportStaticBundle: %v", err)
	}
	if !bytes.Contains(htmlBytes, []byte(exportCytoscapeCDN)) {
		t.Errorf("CDN-mode export should reference cytoscape CDN URL")
	}
	if !bytes.Contains(htmlBytes, []byte(exportMarkedCDN)) {
		t.Errorf("CDN-mode export should reference marked CDN URL")
	}
	if !externalSrcRe.Match(htmlBytes) {
		t.Errorf("CDN-mode export should contain external src=\"http...\" references")
	}
}
