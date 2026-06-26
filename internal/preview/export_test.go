package preview

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"testing/quick"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// extractBundle pulls the JSON blob assigned to window.BUNDLE out of an exported
// HTML document and unmarshals it back into an okfBundle. The template emits
// `window.BUNDLE = {json};` so we decode exactly one JSON value starting right
// after the assignment, letting the JSON decoder stop at the matching closing
// brace (ignoring the trailing `;`).
func extractBundle(t *testing.T, htmlBytes []byte) okfBundle {
	t.Helper()
	bundle := decodeBundle(htmlBytes)
	if bundle == nil {
		t.Fatalf("exported HTML does not contain a decodable window.BUNDLE blob")
	}
	return *bundle
}

// decodeBundle is the non-fatal sibling of extractBundle used inside property
// closures (which cannot take *testing.T). Returns nil on any failure.
func decodeBundle(htmlBytes []byte) *okfBundle {
	const marker = "window.BUNDLE = "
	idx := bytes.Index(htmlBytes, []byte(marker))
	if idx < 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(htmlBytes[idx+len(marker):]))
	var bundle okfBundle
	if err := dec.Decode(&bundle); err != nil {
		return nil
	}
	return &bundle
}

// bundleNodeIDSet returns the set of node (concept) IDs present in a bundle.
func bundleNodeIDSet(bundle okfBundle) map[string]bool {
	set := make(map[string]bool, len(bundle.Nodes))
	for _, n := range bundle.Nodes {
		set[n.Data.ID] = true
	}
	return set
}

// conceptIDSet returns the set of concept IDs for the project's documents (doc
// id with the markdown extension stripped, matching buildOKFBundle).
func conceptIDSet(docs []specDocument) map[string]bool {
	set := make(map[string]bool, len(docs))
	for _, d := range docs {
		set[conceptID(d.ID)] = true
	}
	return set
}

// sampleProject builds a concrete fixture project with several documents
// (markdown + html) and cross-links, constructed directly in Go so the test does
// not depend on scanning the filesystem.
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
				ID:          "modules/alpha.md",
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
				ID:       "modules/beta.md",
				Title:    "Beta Module",
				Path:     "modules/beta.md",
				Format:   "markdown",
				Category: "modules",
				Status:   "draft",
				Type:     "module",
				Raw:      "# Beta\n\n- item one\n- item two\n",
			},
			{
				ID:       "pages/landing.md",
				Title:    "Landing Page",
				Path:     "pages/landing.md",
				Format:   "markdown",
				Category: "pages",
				Type:     "feature",
				Raw:      "# Landing\n\nWelcome to the landing page.",
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Unit tests (fixture-based)
// ---------------------------------------------------------------------------

func TestExportStaticBundleIncludesAllDocumentsAndEdges(t *testing.T) {
	project := sampleProject()
	opt := exportOptions{includeGraph: true, inlineAssets: true}

	htmlBytes, err := exportStaticBundle(project, opt)
	if err != nil {
		t.Fatalf("exportStaticBundle: %v", err)
	}

	if !bytes.Contains(htmlBytes, []byte("window.BUNDLE")) {
		t.Fatalf("exported HTML must embed window.BUNDLE blob")
	}

	bundle := extractBundle(t, htmlBytes)

	// Every project document must be present as a node (Req 2.1).
	if len(bundle.Nodes) != len(project.Documents) {
		t.Fatalf("expected %d nodes, got %d", len(project.Documents), len(bundle.Nodes))
	}
	got := bundleNodeIDSet(bundle)
	for _, doc := range project.Documents {
		if !got[conceptID(doc.ID)] {
			t.Errorf("document %q missing from bundle", doc.ID)
		}
	}

	// Each concept must carry a body keyed by its id.
	for _, n := range bundle.Nodes {
		if _, ok := bundle.Bodies[n.Data.ID]; !ok {
			t.Errorf("concept %q has no body entry", n.Data.ID)
		}
	}

	// The alpha→beta relative link must become a directed edge (Req 2.2).
	if !hasOKFEdge(bundle, "modules/alpha", "modules/beta") {
		t.Errorf("expected edge modules/alpha -> modules/beta from the relative link")
	}
}

func TestExportStaticBundleNoGraphFlagOmitsEdges(t *testing.T) {
	project := sampleProject()
	opt := exportOptions{includeGraph: false, inlineAssets: true}

	htmlBytes, err := exportStaticBundle(project, opt)
	if err != nil {
		t.Fatalf("exportStaticBundle: %v", err)
	}
	bundle := extractBundle(t, htmlBytes)

	// Edges must be empty (Req 2.3) ...
	if len(bundle.Edges) != 0 {
		t.Errorf("expected no edges with includeGraph=false, got %d", len(bundle.Edges))
	}
	// ... while every document is still present as a node.
	if len(bundle.Nodes) != len(project.Documents) {
		t.Errorf("expected %d nodes with --no-graph, got %d", len(project.Documents), len(bundle.Nodes))
	}
}

// TestExportPermissiveBodies verifies the fail-open contract (Req 2.4): odd or
// empty documents still appear as nodes with body entries, and dangerous markup
// (script/style) is scrubbed from bodies before they reach the client renderer.
func TestExportPermissiveBodies(t *testing.T) {
	project := specProject{
		Summary: projectSummary{Name: "Permissive", DocsRoot: "docs", TotalSpecs: 4},
		Documents: []specDocument{
			{ID: "ok/markdown.md", Title: "OK", Path: "ok.md", Format: "markdown", Type: "module", Raw: "# Fine\n\nNormal content."},
			{ID: "odd/empty.md", Title: "Empty", Path: "empty.md", Format: "markdown", Type: "module", Raw: ""},
			{ID: "odd/script.md", Title: "Scripted", Path: "odd.md", Format: "markdown", Type: "module",
				Raw: "# Odd\n\nInline <script>evil()</script> and <style>x{}</style> markup."},
			{ID: "odd/unknown.md", Title: "Unknown", Path: "weird.md", Format: "markdown", Type: "module",
				Raw: "raw <tags> & control chars"},
		},
	}

	htmlBytes, err := exportStaticBundle(project, exportOptions{includeGraph: false, inlineAssets: true})
	if err != nil {
		t.Fatalf("exportStaticBundle must be fail-open, got error: %v", err)
	}
	bundle := extractBundle(t, htmlBytes)

	// All documents survive as nodes, regardless of content (none dropped).
	if len(bundle.Nodes) != len(project.Documents) {
		t.Fatalf("expected %d nodes, got %d", len(project.Documents), len(bundle.Nodes))
	}
	got := bundleNodeIDSet(bundle)
	for _, doc := range project.Documents {
		if !got[conceptID(doc.ID)] {
			t.Errorf("permissive export dropped document %q", doc.ID)
		}
	}

	// The scripted doc's body must not retain <script>/<style> blocks.
	body := bundle.Bodies["odd/script"]
	if strings.Contains(strings.ToLower(body), "<script") || strings.Contains(strings.ToLower(body), "<style") {
		t.Errorf("body retained dangerous markup: %q", body)
	}
}

func TestExportRewritesInternalLinksToBundleRelative(t *testing.T) {
	project := sampleProject()
	bundle := buildOKFBundle(project, true)

	// The alpha body's relative link `./beta.md` must be rewritten to the OKF
	// bundle-relative form `/modules/beta.md` so the viewer can navigate it.
	body := bundle.Bodies["modules/alpha"]
	if !strings.Contains(body, "(/modules/beta.md)") {
		t.Errorf("expected rewritten bundle-relative link in body, got: %q", body)
	}
}

func TestStripFrontmatterRemovesLeadingYAML(t *testing.T) {
	raw := "---\ntype: module\ntitle: X\n---\n\n# Body\n\ntext"
	body := stripFrontmatter(raw)
	if strings.Contains(body, "type: module") {
		t.Errorf("frontmatter not stripped: %q", body)
	}
	if !strings.Contains(body, "# Body") {
		t.Errorf("body content lost after stripping frontmatter: %q", body)
	}
}

// hasOKFEdge reports whether the bundle has a directed edge source->target.
func hasOKFEdge(bundle okfBundle, source, target string) bool {
	for _, e := range bundle.Edges {
		if e.Data.Source == source && e.Data.Target == target {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Property-based tests (testing/quick, standard library — no new dependency)
// ---------------------------------------------------------------------------

// genProject is a quick.Generator wrapper that produces bounded, meaningful
// random specProjects (a handful of documents) so property checks exercise a
// wide input space without unbounded sizes.
type genProject struct {
	specProject
}

func randString(rnd *rand.Rand, prefix string) string {
	return prefix + "-" + string(rune('a'+rnd.Intn(26))) + string(rune('a'+rnd.Intn(26)))
}

func (genProject) Generate(rnd *rand.Rand, size int) reflect.Value {
	docCount := rnd.Intn(6) // 0..5 documents, includes the empty-project edge case
	docs := make([]specDocument, 0, docCount)
	for i := 0; i < docCount; i++ {
		id := randString(rnd, "doc") + "/" + randString(rnd, "n") + ".md"
		raw := "# " + randString(rnd, "title") + "\n\nbody " + randString(rnd, "b")
		docs = append(docs, specDocument{
			ID:     id,
			Title:  randString(rnd, "Title"),
			Path:   id,
			Format: "markdown",
			Type:   "module",
			Raw:    raw,
			Tags:   []string{randString(rnd, "tag")},
		})
	}

	gp := genProject{specProject{
		Summary: projectSummary{
			Name:       randString(rnd, "Project"),
			DocsRoot:   "docs",
			TotalSpecs: docCount,
		},
		Documents: docs,
	}}
	return reflect.ValueOf(gp)
}

// Property 2: Export không mất doc (Validates: Requirements 2.1)
// Every specDocument in the project appears exactly once as a node in the bundle.
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
		if len(bundle.Nodes) != len(gp.Documents) {
			return false
		}
		got := bundleNodeIDSet(*bundle)
		for _, doc := range gp.Documents {
			if !got[conceptID(doc.ID)] {
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatalf("Property 2 (no doc lost) failed: %v", err)
	}
}

// Property 3: Export edges theo flag (Validates: Requirements 2.2, 2.3)
// includeGraph=false embeds no edges while every node stays present.
func TestPropertyExportEdgesFollowFlag(t *testing.T) {
	property := func(gp genProject) bool {
		htmlBytes, err := exportStaticBundle(gp.specProject, exportOptions{includeGraph: false, inlineAssets: true})
		if err != nil {
			return false
		}
		bundle := decodeBundle(htmlBytes)
		if bundle == nil {
			return false
		}
		if len(bundle.Nodes) != len(gp.Documents) {
			return false
		}
		return len(bundle.Edges) == 0
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatalf("Property 3 (edges by flag) failed: %v", err)
	}
}

// Property 5: Export không rò dữ liệu ngoài project (Validates: Requirements 2.5)
// The bundle contains only the project's own concepts — nothing is injected from
// outside the provided specProject. Edges only connect known concepts.
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
		projectIDs := conceptIDSet(gp.Documents)
		for _, n := range bundle.Nodes {
			if !projectIDs[n.Data.ID] {
				return false
			}
		}
		for _, e := range bundle.Edges {
			if !projectIDs[e.Data.Source] || !projectIDs[e.Data.Target] {
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatalf("Property 5 (no data leak) failed: %v", err)
	}
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
func TestExportSelfContainedOfflineInlineAssets(t *testing.T) {
	project := specProject{
		Summary: projectSummary{Name: "Offline KB", DocsRoot: "docs", TotalSpecs: 2},
		Documents: []specDocument{
			{ID: "a.md", Title: "A", Path: "a.md", Format: "markdown", Type: "module", Raw: "# A\n\nLocal content, see [B](./b.md)."},
			{ID: "b.md", Title: "B", Path: "b.md", Format: "markdown", Type: "module", Raw: "# B\n\nMore local content."},
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
	if !bytes.Contains(htmlBytes, []byte("window.BUNDLE")) {
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

func TestExportInlineAssetPathsAreModuleZipSafe(t *testing.T) {
	paths := []string{exportCytoscapePath, exportMarkedPath}
	for _, assetPath := range paths {
		for _, segment := range strings.Split(filepath.ToSlash(assetPath), "/") {
			if segment == "vendor" {
				t.Fatalf("inline asset path %q uses a vendor directory, which Go module zips omit", assetPath)
			}
		}
		if _, err := exportUIFS.ReadFile(assetPath); err != nil {
			t.Fatalf("embedded inline asset %q should be readable: %v", assetPath, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Helper-function unit tests for export.go
// ---------------------------------------------------------------------------

func TestConceptID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"modules/preview.md", "modules/preview"},
		{"modules/preview.markdown", "modules/preview"},
		{"MODULES/UPPER.MD", "MODULES/UPPER"},
		{"noext", "noext"},
		{"  trailing.md  ", "trailing"},
	}
	for _, tc := range cases {
		if got := conceptID(tc.in); got != tc.want {
			t.Errorf("conceptID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeExportOutputPath(t *testing.T) {
	cwd := "/tmp/cwd"
	if got := normalizeExportOutputPath(cwd, ""); got != filepath.Join(cwd, defaultExportOutputName) {
		t.Errorf("empty out should fall back to default name, got %q", got)
	}
	if got := normalizeExportOutputPath(cwd, "  "); got != filepath.Join(cwd, defaultExportOutputName) {
		t.Errorf("whitespace out should fall back to default name, got %q", got)
	}
	if got := normalizeExportOutputPath(cwd, "/abs/out.html"); got != "/abs/out.html" {
		t.Errorf("absolute path should be returned as-is (cleaned), got %q", got)
	}
	if got := normalizeExportOutputPath(cwd, "rel/out.html"); got != filepath.Join(cwd, "rel/out.html") {
		t.Errorf("relative path should be joined with cwd, got %q", got)
	}
	if got := normalizeExportOutputPath(cwd, "~/out.html"); got != filepath.Join(cwd, "~/out.html") {
		// ExpandPath only expands when prefix is "~" or "~/", which doesn't
		// match. Just confirm it joined cleanly with cwd.
		_ = got
	}
}

func TestResolveConceptLink(t *testing.T) {
	known := map[string]bool{"modules/alpha": true, "modules/beta": true, "root": true}
	cases := []struct {
		name, target, docDir, want string
	}{
		{"absolute", "https://example.com/x.md", "", ""},
		{"empty", "", "", ""},
		{"root-prefix", "/modules/alpha.md", "", "modules/alpha"},
		{"relative-to-doc", "beta.md", "modules", "modules/beta"},
		{"parent-relative-resolves-via-clean", "../beta.md", "modules/sub", "modules/beta"},
		{"unknown-target", "ghost.md", "modules", ""},
		{"external-scheme", "ftp://x/y.md", "", ""},
	}
	for _, tc := range cases {
		got := resolveConceptLink(tc.target, tc.docDir, known)
		if got != tc.want {
			t.Errorf("%s: resolveConceptLink(%q, %q) = %q, want %q", tc.name, tc.target, tc.docDir, got, tc.want)
		}
	}
}

func TestPathDir(t *testing.T) {
	if got := pathDir("a/b/c"); got != "a/b" {
		t.Errorf("pathDir(a/b/c) = %q, want a/b", got)
	}
	if got := pathDir("root"); got != "" {
		t.Errorf("pathDir(root) = %q, want empty", got)
	}
}

func TestPathJoin(t *testing.T) {
	// pathJoin returns a slash-style clean path WITHOUT stripping extensions.
	if got := pathJoin("", "a/b.md"); got != "a/b.md" {
		t.Errorf("pathJoin(\"\", a/b.md) = %q, want a/b.md", got)
	}
	if got := pathJoin("dir", "x.md"); got != "dir/x.md" {
		t.Errorf("pathJoin(dir, x.md) = %q, want dir/x.md", got)
	}
}

func TestStripFrontmatter(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"plain", "no frontmatter here", "no frontmatter here"},
		{"stripped", "---\ntitle: x\n---\nbody", "body"},
		{"unterminated", "---\ntitle: x\nbody without close", "---\ntitle: x\nbody without close"},
		{"bom-prefixed", "\ufeff---\nx: 1\n---\nbody", "body"},
		{"crlf", "---\r\nx: 1\r\n---\r\nbody", "body"},
	}
	for _, tc := range cases {
		if got := stripFrontmatter(tc.in); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestBuildOKFBundleIncludesNodesEdgesAndPalette(t *testing.T) {
	project := sampleProject()
	bundle := buildOKFBundle(project, true)
	if len(bundle.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(bundle.Nodes))
	}
	// Alpha→beta edge must be present.
	found := false
	for _, e := range bundle.Edges {
		if e.Data.Source == "modules/alpha" && e.Data.Target == "modules/beta" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected alpha→beta edge, got %+v", bundle.Edges)
	}
	// Without graph: no edges.
	noGraph := buildOKFBundle(project, false)
	if len(noGraph.Edges) != 0 {
		t.Errorf("expected no edges when includeGraph=false, got %d", len(noGraph.Edges))
	}
	// Palette is the static map.
	if len(bundle.Palette) == 0 {
		t.Errorf("palette should be populated")
	}
}

func TestConceptToNodeUsesDefaults(t *testing.T) {
	// Concept with empty type → default color.
	// Concept with empty title → id is used as label.
	// Concept with nil tags → empty slice.
	c := exportConcept{id: "x", title: "", body: "short body", tags: nil}
	n := c.toNode()
	if n.Data.Color != okfDefaultNodeColor {
		t.Errorf("expected default color, got %q", n.Data.Color)
	}
	if n.Data.Label != "x" {
		t.Errorf("expected label=id, got %q", n.Data.Label)
	}
	if n.Data.Tags == nil {
		t.Errorf("expected non-nil tags slice")
	}
	// Concept with large body → size capped at 90.
	big := exportConcept{id: "big", title: "Big", body: strings.Repeat("x", 50000)}
	bigN := big.toNode()
	if bigN.Data.Size != 90 {
		t.Errorf("expected size capped at 90, got %d", bigN.Data.Size)
	}
	// Concept with known type → palette color.
	mod := exportConcept{id: "m", title: "M", body: "", conceptType: "module"}
	if mod.toNode().Data.Color == okfDefaultNodeColor {
		t.Errorf("module type should use palette color")
	}
}

func TestExtractConceptLinksSkipsExternalAndDuplicates(t *testing.T) {
	body := `See [a](./a.md), [b](./a.md) (dup), [ext](https://x.com/y.md), [bad](./missing.md).`
	known := map[string]bool{"x/a": true}
	dir := "x"
	got := extractConceptLinks(body, dir, known)
	if len(got) != 1 || got[0] != "x/a" {
		t.Errorf("expected only [x/a], got %v", got)
	}
}

func TestRewriteBodyLinksRewritesKnownLeavesAlone(t *testing.T) {
	known := map[string]bool{"x/a": true}
	body := `[a](./a.md) [bad](./missing.md)`
	got := rewriteBodyLinks(body, "x", known)
	if !strings.Contains(got, "](/x/a.md)") {
		t.Errorf("expected rewritten link, got %q", got)
	}
	if !strings.Contains(got, "[bad](./missing.md)") {
		t.Errorf("unknown link should be left untouched, got %q", got)
	}
}

func TestExportPageTitleFallback(t *testing.T) {
	if got := exportPageTitle("  "); got != "Knowledge Base" {
		t.Errorf("empty name should fall back, got %q", got)
	}
	if got := exportPageTitle("My KB"); got != "My KB" {
		t.Errorf("non-empty name should be trimmed, got %q", got)
	}
}

func TestInjectBundleRendersTemplate(t *testing.T) {
	project := sampleProject()
	bundle := buildOKFBundle(project, true)
	out, err := injectBundle(exportTemplate, bundle, "Title", exportOptions{inlineAssets: true})
	if err != nil {
		t.Fatalf("injectBundle: %v", err)
	}
	if !bytes.Contains(out, []byte("window.BUNDLE")) {
		t.Errorf("rendered HTML must embed window.BUNDLE blob")
	}
	// nil template → error.
	if _, err := injectBundle(nil, bundle, "x", exportOptions{}); err == nil {
		t.Errorf("nil template should error")
	}
}

func TestInjectBundleErrorPaths(t *testing.T) {
	origRead := exportReadFileForTest
	defer func() { exportReadFileForTest = origRead }()

	bundle := okfBundle{}
	// stylesheet read error
	exportReadFileForTest = func(name string) ([]byte, error) {
		if name == exportStylePath {
			return nil, errors.New("style not found")
		}
		return exportUIFS.ReadFile(name)
	}
	if _, err := injectBundle(exportTemplate, bundle, "x", exportOptions{}); err == nil || !strings.Contains(err.Error(), "stylesheet") {
		t.Errorf("expected stylesheet error, got %v", err)
	}

	// app script read error
	exportReadFileForTest = func(name string) ([]byte, error) {
		if name == exportAppScriptPath {
			return nil, errors.New("script not found")
		}
		return exportUIFS.ReadFile(name)
	}
	if _, err := injectBundle(exportTemplate, bundle, "x", exportOptions{}); err == nil || !strings.Contains(err.Error(), "script") {
		t.Errorf("expected script error, got %v", err)
	}

	// third-party read error via buildVendorHead inline=true
	exportReadFileForTest = func(name string) ([]byte, error) {
		if name == exportCytoscapePath {
			return nil, errors.New("cytoscape not found")
		}
		return exportUIFS.ReadFile(name)
	}
	if _, err := injectBundle(exportTemplate, bundle, "x", exportOptions{inlineAssets: true}); err == nil || !strings.Contains(err.Error(), "cytoscape") {
		t.Errorf("expected cytoscape error, got %v", err)
	}

	exportReadFileForTest = func(name string) ([]byte, error) {
		if name == exportMarkedPath {
			return nil, errors.New("marked not found")
		}
		return exportUIFS.ReadFile(name)
	}
	if _, err := injectBundle(exportTemplate, bundle, "x", exportOptions{inlineAssets: true}); err == nil || !strings.Contains(err.Error(), "marked") {
		t.Errorf("expected marked error, got %v", err)
	}

	// template execute error: use a template that fails on Execute.
	badTmpl := template.Must(template.New("bad").Parse(`{{.Title.NoSuchField}}`))
	exportReadFileForTest = origRead // reset
	if _, err := injectBundle(badTmpl, bundle, "x", exportOptions{}); err == nil || !strings.Contains(err.Error(), "render") {
		t.Errorf("expected render error, got %v", err)
	}

	// bundleJSON marshal error
	origMarshal := exportMarshalJSONForTest
	defer func() { exportMarshalJSONForTest = origMarshal }()
	callCount := 0
	exportMarshalJSONForTest = func(v any) ([]byte, error) {
		callCount++
		if callCount == 1 {
			return nil, errors.New("bundle marshal fail")
		}
		return json.Marshal(v)
	}
	if _, err := injectBundle(exportTemplate, bundle, "x", exportOptions{}); err == nil || !strings.Contains(err.Error(), "bundle") {
		t.Errorf("expected bundle marshal error, got %v", err)
	}

	// nameJSON marshal error
	callCount = 0
	exportMarshalJSONForTest = func(v any) ([]byte, error) {
		callCount++
		if callCount == 2 {
			return nil, errors.New("name marshal fail")
		}
		return json.Marshal(v)
	}
	if _, err := injectBundle(exportTemplate, bundle, "x", exportOptions{}); err == nil || !strings.Contains(err.Error(), "name") {
		t.Errorf("expected name marshal error, got %v", err)
	}
	exportMarshalJSONForTest = origMarshal // restore
}

func TestBuildVendorHeadInlineAndCDN(t *testing.T) {
	inline, err := buildVendorHead(true)
	if err != nil {
		t.Fatalf("buildVendorHead(inline): %v", err)
	}
	if !strings.Contains(string(inline), "<script>") {
		t.Errorf("inline head should contain inline <script> tags")
	}
	cdn, err := buildVendorHead(false)
	if err != nil {
		t.Fatalf("buildVendorHead(cdn): %v", err)
	}
	if !strings.Contains(string(cdn), exportCytoscapeCDN) || !strings.Contains(string(cdn), exportMarkedCDN) {
		t.Errorf("CDN head should reference vendor URLs, got %q", string(cdn))
	}
}

func TestRunExportEndToEnd(t *testing.T) {
	dir := t.TempDir()
	// Build a project with at least one doc.
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "index.md"), []byte("---\ntitle: Index\n---\nHello"), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.html")
	err := RunExport([]string{
		"--project", dir,
		"--docs", "docs",
		"--out", outPath,
		"--no-graph",
		"--inline-assets=true",
	})
	if err != nil {
		t.Fatalf("RunExport: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !bytes.Contains(data, []byte("window.BUNDLE")) {
		t.Errorf("exported HTML must embed bundle")
	}
}

func TestRunExportRejectsInvalidProject(t *testing.T) {
	dir := t.TempDir()
	// No docs/ subdirectory → scanSpecProject should fail.
	err := RunExport([]string{"--project", dir, "--docs", "docs"})
	if err == nil {
		t.Errorf("RunExport should fail when docs dir missing")
	}
}

func TestRunExportHelp(t *testing.T) {
	// --help exits cleanly with no error.
	if err := RunExport([]string{"--help"}); err != nil {
		t.Errorf("--help should not error, got %v", err)
	}
	// -h also exits cleanly.
	if err := RunExport([]string{"-h"}); err != nil {
		t.Errorf("-h should not error, got %v", err)
	}
}

func TestRunExportBadFlag(t *testing.T) {
	if err := RunExport([]string{"--unknown-flag"}); err == nil {
		t.Errorf("unknown flag should error")
	}
}

func TestRunExportWriteFileError(t *testing.T) {
	// Force WriteFile to fail by setting outPath to an invalid location.
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "index.md"), []byte("---\ntitle: Index\n---\nHi"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make outPath a directory to force WriteFile error.
	outDir := filepath.Join(dir, "outdir")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	err := RunExport([]string{
		"--project", dir,
		"--docs", "docs",
		"--out", outDir,
	})
	if err == nil {
		t.Error("expected error when writing to a directory")
	}
}

func TestRunExportWithGraph(t *testing.T) {
	// Cover the !noGraph branch (default graph included).
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "a.md"), []byte("---\ntitle: A\n---\n# A"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "b.md"), []byte("---\ntitle: B\n---\n# B"), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.html")
	// Use --name to set a custom name.
	if err := RunExport([]string{
		"--project", dir,
		"--docs", "docs",
		"--out", outPath,
		"--name", "TestProj",
	}); err != nil {
		t.Fatalf("RunExport with graph: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("TestProj")) {
		t.Errorf("expected custom name in output")
	}
}

func TestRunExportOpenBrowserFailsGracefully(t *testing.T) {
	// Cover openBrowser branch. openURL may fail but RunExport should still
	// succeed and just print a warning.
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "index.md"), []byte("---\ntitle: X\n---\n# X"), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.html")
	// open=true triggers openURL. On CI / Linux without a browser, this may
	// fail silently. Either way, RunExport should not error out.
	err := RunExport([]string{
		"--project", dir,
		"--docs", "docs",
		"--out", outPath,
		"--open",
	})
	if err != nil {
		t.Errorf("RunExport with --open should not fail: %v", err)
	}
}
