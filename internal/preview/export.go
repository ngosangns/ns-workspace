package preview

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ngosangns/ns-workspace/internal/internalutil"
)

// defaultExportOutputName là tên file mặc định khi không truyền --out (Req 1.2).
const defaultExportOutputName = "ns-workspace-kb.html"

// RunExport parse flags và ghi MỘT file HTML tĩnh self-contained chứa toàn bộ
// docs + graph của project, render bằng OKF Bundle Viewer (ported từ
// GoogleCloudPlatform/knowledge-catalog). Tái dùng knowledge core
// (normalizePreviewProjectRoot, scanSpecProject, docsRoot) thay vì nhân đôi logic.
//
// Validate trước khi ghi: nếu project root / docs dir không hợp lệ thì trả lỗi
// rõ ràng và KHÔNG ghi bất kỳ file nào (Req 1.5). Chỉ khi scanSpecProject thành
// công mới build HTML và os.WriteFile (Req 1.1, 1.2).
func RunExport(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	opt := exportOptions{
		projectRoot:  cwd,
		docsDir:      "docs",
		outPath:      defaultExportOutputName,
		includeGraph: true,
		inlineAssets: true,
		openBrowser:  false,
	}

	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.StringVar(&opt.projectRoot, "project", opt.projectRoot, "project root to export")
	fs.StringVar(&opt.docsDir, "docs", opt.docsDir, "docs directory relative to project root, or absolute path")
	fs.StringVar(&opt.outPath, "out", opt.outPath, "output HTML file path")
	fs.StringVar(&opt.name, "name", opt.name, "display name shown in the viewer header (default project name)")
	noGraph := fs.Bool("no-graph", false, "export documents only, without the relationship edges")
	fs.BoolVar(&opt.inlineAssets, "inline-assets", opt.inlineAssets, "inline render libraries for fully offline output (false references CDN)")
	fs.BoolVar(&opt.openBrowser, "open", opt.openBrowser, "open the generated file after writing")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *noGraph {
		opt.includeGraph = false
	}

	opt.projectRoot = normalizePreviewProjectRoot(opt.projectRoot)
	opt.outPath = normalizeExportOutputPath(cwd, opt.outPath)

	// Validate project root / docs dir TRƯỚC khi ghi: lỗi rõ ràng và KHÔNG ghi
	// file khi không hợp lệ (Req 1.5).
	project, err := scanSpecProject(opt.projectRoot, opt.docsDir)
	if err != nil {
		return fmt.Errorf("export: cannot read docs at %s: %w", docsRoot(opt.projectRoot, opt.docsDir), err)
	}

	htmlBytes, err := exportStaticBundle(project, opt)
	if err != nil {
		return err
	}
	if err := os.WriteFile(opt.outPath, htmlBytes, 0o644); err != nil {
		return fmt.Errorf("export: write %s: %w", opt.outPath, err)
	}

	fmt.Printf("export: %s\n", opt.outPath)
	fmt.Printf("project: %s\n", opt.projectRoot)
	fmt.Printf("docs: %s\n", docsRoot(opt.projectRoot, opt.docsDir))

	if opt.openBrowser {
		if err := openURL(opt.outPath); err != nil {
			fmt.Printf("open browser failed: %v\n", err)
		}
	}
	return nil
}

// normalizeExportOutputPath resolve --out thành đường dẫn tuyệt đối, gắn với cwd
// khi người dùng truyền đường dẫn tương đối (Req 1.2: file mặc định nằm trong
// current working directory).
func normalizeExportOutputPath(cwd, out string) string {
	out = strings.TrimSpace(out)
	if out == "" {
		out = defaultExportOutputName
	}
	out = internalutil.ExpandPath(out)
	if filepath.IsAbs(out) {
		return filepath.Clean(out)
	}
	return filepath.Join(cwd, out)
}

// exportUIFS embeds the static OKF viewer assets (template, viz.js/viz.css, and
// vendored render libraries) so that `export --inline-assets=true` produces a
// fully self-contained HTML file that opens over file:// with no network
// requests. The assets live under export_ui/ and are hand-maintained (no Vite
// build), keeping the export independent of the preview_ui_src/ pipeline.
//
//go:embed export_ui
var exportUIFS embed.FS

// exportOptions gom các tham số điều khiển một lần export tĩnh.
type exportOptions struct {
	projectRoot  string
	docsDir      string
	outPath      string
	name         string
	includeGraph bool
	inlineAssets bool
	openBrowser  bool
}

// Đường dẫn asset trong embed FS (export_ui/...). Tách thành hằng để inject và
// loader template dùng chung, tránh lệch path.
const (
	exportTemplatePath  = "export_ui/viz.html.tmpl"
	exportStylePath     = "export_ui/viz.css"
	exportAppScriptPath = "export_ui/viz.js"
	exportCytoscapePath = "export_ui/vendor/cytoscape.min.js"
	exportMarkedPath    = "export_ui/vendor/marked.min.js"
)

// CDN fallback khi --inline-assets=false. Trùng version với vendor embed
// (xem export_ui/vendor/README.md) để render đồng nhất giữa hai chế độ.
const (
	exportCytoscapeCDN = "https://cdn.jsdelivr.net/npm/cytoscape@3.30.2/dist/cytoscape.min.js"
	exportMarkedCDN    = "https://cdn.jsdelivr.net/npm/marked@12.0.2/marked.min.js"
)

// exportTemplate là template HTML đã parse từ embed FS. Parse một lần lúc init;
// vì file được //go:embed nên parse luôn thành công ở build hợp lệ, template.Must
// chỉ panic khi asset bị hỏng/thiếu (lỗi lập trình, không phải input runtime).
var exportTemplate = template.Must(template.ParseFS(exportUIFS, exportTemplatePath))

// exportTemplateData khớp các field mà viz.html.tmpl tham chiếu. Các kiểu
// template.CSS/HTML/JS đánh dấu nội dung đã tin cậy (asset tĩnh + JSON đã escape)
// nên html/template chèn verbatim thay vì escape lần nữa.
type exportTemplateData struct {
	Title      string
	StyleCSS   template.CSS
	VendorHead template.HTML
	BundleName template.JS
	BundleJSON template.JS
	AppJS      template.JS
}

// ---------------------------------------------------------------------------
// OKF bundle model.
//
// The shapes below mirror the JSON that the OKF Bundle Viewer (viz.js) reads
// from window.BUNDLE: a Cytoscape-style elements graph (nodes/edges) plus a map
// of raw markdown bodies, the sorted set of concept types, and a type→color
// palette. Building this shape in Go is a faithful port of the reference
// generator (okf/src/reference_agent/viewer/generator.py).
// ---------------------------------------------------------------------------

// okfBundle is the full knowledge base serialized for the static viewer.
type okfBundle struct {
	Nodes   []okfNode         `json:"nodes"`
	Edges   []okfEdge         `json:"edges"`
	Bodies  map[string]string `json:"bodies"`
	Types   []string          `json:"types"`
	Palette map[string]string `json:"palette"`
}

type okfNode struct {
	Data okfNodeData `json:"data"`
}

type okfNodeData struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Resource    string   `json:"resource"`
	Tags        []string `json:"tags"`
	Color       string   `json:"color"`
	Size        int      `json:"size"`
}

type okfEdge struct {
	Data okfEdgeData `json:"data"`
}

type okfEdgeData struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}

// exportConcept is the intermediate per-document model used to build the bundle,
// mirroring generator.py's Concept dataclass.
type exportConcept struct {
	id          string
	conceptType string
	title       string
	description string
	resource    string
	tags        []string
	body        string
	linksTo     []string
}

// okfTypePalette maps the doc types this repo uses to stable node colors. Types
// outside the palette fall back to okfDefaultNodeColor; the viewer tolerates any
// type (permissive consumer), so the palette is presentation-only.
var okfTypePalette = map[string]string{
	"module":       "#3b82f6",
	"feature":      "#10b981",
	"spec":         "#f59e0b",
	"architecture": "#8b5cf6",
	"decision":     "#a855f7",
	"pattern":      "#ec4899",
	"reference":    "#14b8a6",
	"research":     "#0ea5e9",
	"shared":       "#64748b",
	"index":        "#0f172a",
}

const okfDefaultNodeColor = "#94a3b8"

// exportMarkdownLinkRe extracts markdown links whose target ends in ".md"
// (optionally with a "#anchor"), e.g. `](../modules/preview.md#meta)`. It mirrors
// the link regex in generator.py and drives both edge construction and the
// rewrite to OKF bundle-relative form.
var exportMarkdownLinkRe = regexp.MustCompile(`\]\(([^)\s]+\.md)(#[A-Za-z0-9_\-]*)?\)`)

// conceptID converts a docs-root-relative document id/path into an OKF concept
// id by stripping a trailing markdown extension. e.g. "modules/preview.md" →
// "modules/preview". The viewer keys bodies and nodes on this id.
func conceptID(docID string) string {
	id := strings.TrimSpace(docID)
	for _, ext := range []string{".md", ".markdown"} {
		if strings.HasSuffix(strings.ToLower(id), ext) {
			return id[:len(id)-len(ext)]
		}
	}
	return id
}

// exportStaticBundle builds the OKF bundle from a scanned project and renders the
// complete self-contained HTML. Only this project's documents/graph/metadata are
// embedded (Req 2.5); rendering is permissive (a single bad doc never aborts the
// export — Req 2.4). Edges are included only when opt.includeGraph (Req 2.2/2.3).
func exportStaticBundle(project specProject, opt exportOptions) ([]byte, error) {
	bundle := buildOKFBundle(project, opt.includeGraph)
	name := strings.TrimSpace(opt.name)
	if name == "" {
		name = exportPageTitle(project.Summary.Name)
	}
	return injectBundle(exportTemplate, bundle, name, opt)
}

// buildOKFBundle ports generator.py: walk the project's documents into concepts,
// derive links between them, and assemble the Cytoscape-shaped graph plus the
// body/type/palette side tables the viewer consumes. Every document becomes a
// node (Req 2.1). When includeGraph is false, edges are omitted (Req 2.3).
func buildOKFBundle(project specProject, includeGraph bool) okfBundle {
	concepts := walkConcepts(project)

	ids := make(map[string]bool, len(concepts))
	for _, c := range concepts {
		ids[c.id] = true
	}

	nodes := make([]okfNode, 0, len(concepts))
	bodies := make(map[string]string, len(concepts))
	typeSet := map[string]bool{}
	for _, c := range concepts {
		nodes = append(nodes, c.toNode())
		bodies[c.id] = c.body
		typeSet[c.conceptType] = true
	}

	edges := []okfEdge{}
	if includeGraph {
		seen := map[string]bool{}
		for _, c := range concepts {
			for _, target := range c.linksTo {
				if target == c.id || !ids[target] {
					continue
				}
				key := c.id + "\x00" + target
				if seen[key] {
					continue
				}
				seen[key] = true
				edges = append(edges, okfEdge{Data: okfEdgeData{
					ID:     c.id + "__" + target,
					Source: c.id,
					Target: target,
				}})
			}
		}
	}

	types := make([]string, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}
	sort.Strings(types)

	return okfBundle{
		Nodes:   nodes,
		Edges:   edges,
		Bodies:  bodies,
		Types:   types,
		Palette: okfTypePalette,
	}
}

// walkConcepts converts each scanned document into an exportConcept with its
// frontmatter-derived metadata, a markdown body (frontmatter stripped so the
// detail panel does not show it twice) whose internal links are rewritten to OKF
// bundle-relative form, and the list of concepts it links to.
func walkConcepts(project specProject) []exportConcept {
	known := make(map[string]bool, len(project.Documents))
	dirByID := make(map[string]string, len(project.Documents))
	for _, doc := range project.Documents {
		id := conceptID(doc.ID)
		known[id] = true
		dirByID[id] = pathDir(id)
	}

	concepts := make([]exportConcept, 0, len(project.Documents))
	for _, doc := range project.Documents {
		id := conceptID(doc.ID)
		rawBody := stripFrontmatter(doc.Raw)
		body := scrubDangerousMarkup(rawBody)
		linksTo := extractConceptLinks(body, dirByID[id], known)
		body = rewriteBodyLinks(body, dirByID[id], known)

		conceptType := internalutil.FirstNonEmpty(doc.Type, doc.Category, "Concept")
		concepts = append(concepts, exportConcept{
			id:          id,
			conceptType: conceptType,
			title:       internalutil.FirstNonEmpty(doc.Title, id),
			description: doc.Description,
			resource:    doc.Resource,
			tags:        doc.Tags,
			body:        body,
			linksTo:     linksTo,
		})
	}
	return concepts
}

// toNode renders a concept as a Cytoscape node, mirroring Concept.to_node():
// color comes from the type palette and size scales gently with body length.
func (c exportConcept) toNode() okfNode {
	color := okfTypePalette[strings.ToLower(c.conceptType)]
	if color == "" {
		color = okfDefaultNodeColor
	}
	size := 30 + len(c.body)/200
	if size > 90 {
		size = 90
	}
	label := c.title
	if strings.TrimSpace(label) == "" {
		label = c.id
	}
	tags := c.tags
	if tags == nil {
		tags = []string{}
	}
	return okfNode{Data: okfNodeData{
		ID:          c.id,
		Label:       label,
		Type:        c.conceptType,
		Description: c.description,
		Resource:    c.resource,
		Tags:        tags,
		Color:       color,
		Size:        size,
	}}
}

// extractConceptLinks resolves every ".md" markdown link in a body to a concept
// id, returning the unique set of in-bundle targets. Mirrors generator.py's
// _extract_links: external/anchor-only links are ignored, relative and
// bundle-relative paths are resolved against docDir, and only links that land on
// a known concept are kept.
func extractConceptLinks(body, docDir string, known map[string]bool) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, m := range exportMarkdownLinkRe.FindAllStringSubmatch(body, -1) {
		target := resolveConceptLink(m[1], docDir, known)
		if target == "" || seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	return out
}

// rewriteBodyLinks rewrites in-bundle ".md" links to OKF bundle-relative form
// (`/<concept-id>.md`) so the unmodified viewer's rewriteInternalLinks can turn
// them into in-page navigation. Links that do not resolve to a known concept are
// left untouched (the viewer treats them as external).
func rewriteBodyLinks(body, docDir string, known map[string]bool) string {
	return exportMarkdownLinkRe.ReplaceAllStringFunc(body, func(match string) string {
		sub := exportMarkdownLinkRe.FindStringSubmatch(match)
		if sub == nil {
			return match
		}
		target := resolveConceptLink(sub[1], docDir, known)
		if target == "" {
			return match
		}
		return "](/" + target + ".md)"
	})
}

// resolveConceptLink turns a single markdown link target into a concept id, or
// returns "" when it is external, absolute-URL, or does not resolve to a known
// concept. A leading "/" is treated as bundle-relative (from the docs root);
// other paths are relative to docDir.
func resolveConceptLink(target, docDir string, known map[string]bool) string {
	target = strings.TrimSpace(target)
	if target == "" || strings.Contains(target, "://") {
		return ""
	}

	var joined string
	if strings.HasPrefix(target, "/") {
		joined = strings.TrimPrefix(target, "/")
	} else {
		joined = pathJoin(docDir, target)
	}
	id := conceptID(filepath.ToSlash(filepath.Clean(joined)))
	if id == "" || strings.HasPrefix(id, "..") {
		return ""
	}
	if !known[id] {
		return ""
	}
	return id
}

// pathDir returns the slash-style directory of a concept id ("" for root docs).
func pathDir(id string) string {
	dir := filepath.ToSlash(filepath.Dir(id))
	if dir == "." {
		return ""
	}
	return dir
}

// pathJoin joins a slash-style dir and a relative target, returning a clean
// slash path.
func pathJoin(dir, target string) string {
	if dir == "" {
		return filepath.ToSlash(filepath.Clean(target))
	}
	return filepath.ToSlash(filepath.Clean(dir + "/" + target))
}

// scrubDangerousMarkupRe removes <script>/<style> blocks from a markdown body so
// embedded raw HTML cannot execute when the viewer renders the body via marked
// (marked does not sanitize). This is a defensive measure on top of the OKF
// viewer; it does not affect ordinary markdown.
var scrubDangerousMarkupRe = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</\s*(script|style)\s*>`)

func scrubDangerousMarkup(body string) string {
	return scrubDangerousMarkupRe.ReplaceAllString(body, "")
}

// stripFrontmatter loại bỏ block YAML frontmatter `---...---` ở đầu body (nếu có)
// để detail panel của viewer không hiển thị metadata hai lần. Permissive: không
// có frontmatter thì trả nguyên body.
func stripFrontmatter(content string) string {
	trimmed := strings.TrimLeft(content, "\ufeff")
	if !strings.HasPrefix(trimmed, "---\n") && !strings.HasPrefix(trimmed, "---\r\n") {
		return content
	}
	rest := trimmed[strings.IndexByte(trimmed, '\n')+1:]
	lines := strings.Split(rest, "\n")
	for i, line := range lines {
		if strings.TrimRight(line, "\r") == "---" {
			return strings.TrimLeft(strings.Join(lines[i+1:], "\n"), "\n")
		}
	}
	// Không tìm thấy dấu đóng → trả nguyên content (permissive).
	return content
}

// injectBundle nhúng bundle (JSON blob → window.BUNDLE) + tên + assets vào
// template. inlineAssets=true: inline viz.css, viz.js và vendor libs từ embed FS
// để file mở offline qua file://. inlineAssets=false: vendor libs tham chiếu CDN,
// vẫn inline CSS/JS của viewer.
func injectBundle(tmpl *template.Template, bundle okfBundle, name string, opt exportOptions) ([]byte, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("export template is nil")
	}

	styleCSS, err := exportUIFS.ReadFile(exportStylePath)
	if err != nil {
		return nil, fmt.Errorf("read export stylesheet: %w", err)
	}
	appJS, err := exportUIFS.ReadFile(exportAppScriptPath)
	if err != nil {
		return nil, fmt.Errorf("read export script: %w", err)
	}

	vendorHead, err := buildVendorHead(opt.inlineAssets)
	if err != nil {
		return nil, err
	}

	// json.Marshal mặc định escape <, >, & thành \u003c/\u003e/\u0026, nên blob
	// an toàn khi nhúng trong <script> (không thể đóng tag sớm bằng </script>).
	bundleJSON, err := json.Marshal(bundle)
	if err != nil {
		return nil, fmt.Errorf("marshal export bundle: %w", err)
	}
	nameJSON, err := json.Marshal(name)
	if err != nil {
		return nil, fmt.Errorf("marshal bundle name: %w", err)
	}

	data := exportTemplateData{
		Title:      exportPageTitle(name),
		StyleCSS:   template.CSS(styleCSS),
		VendorHead: vendorHead,
		BundleName: template.JS(nameJSON),
		BundleJSON: template.JS(bundleJSON),
		AppJS:      template.JS(appJS),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render export template: %w", err)
	}
	return buf.Bytes(), nil
}

// buildVendorHead dựng các <script> tag cho thư viện render (Cytoscape.js, marked).
// inline=true: nhúng toàn bộ source từ embed FS (offline tuyệt đối). inline=false:
// chỉ phát tag tham chiếu CDN.
func buildVendorHead(inline bool) (template.HTML, error) {
	var head strings.Builder
	if inline {
		cytoscape, err := exportUIFS.ReadFile(exportCytoscapePath)
		if err != nil {
			return "", fmt.Errorf("read vendor cytoscape: %w", err)
		}
		marked, err := exportUIFS.ReadFile(exportMarkedPath)
		if err != nil {
			return "", fmt.Errorf("read vendor marked: %w", err)
		}
		head.WriteString("<script>")
		head.Write(cytoscape)
		head.WriteString("</script>\n")
		head.WriteString("<script>")
		head.Write(marked)
		head.WriteString("</script>")
		return template.HTML(head.String()), nil
	}

	head.WriteString(`<script src="` + exportCytoscapeCDN + `"></script>` + "\n")
	head.WriteString(`<script src="` + exportMarkedCDN + `"></script>`)
	return template.HTML(head.String()), nil
}

// exportPageTitle chọn title hiển thị cho file export, fallback khi project chưa
// có tên.
func exportPageTitle(name string) string {
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		return trimmed
	}
	return "Knowledge Base"
}
