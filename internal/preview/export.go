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
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/ngosangns/ns-workspace/internal/internalutil"
)

// defaultExportOutputName là tên file mặc định khi không truyền --out (Req 1.2).
const defaultExportOutputName = "ns-workspace-kb.html"

// RunExport parse flags và ghi MỘT file HTML tĩnh self-contained chứa toàn bộ
// docs + graph của project. Tái dùng knowledge core (normalizePreviewProjectRoot,
// scanSpecProject, docsRoot) thay vì nhân đôi logic.
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
	noGraph := fs.Bool("no-graph", false, "export documents only, without the graph")
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

// exportUIFS embeds the static export UI assets (template, vanilla JS/CSS, and
// vendored render libraries) so that `export --inline-assets=true` produces a
// fully self-contained HTML file that opens over file:// with no network
// requests. The assets live under export_ui/ and are hand-written (no Vite
// build), keeping the export independent of the preview_ui_src/ pipeline.
//
// NOTE: exportStaticBundle, injectBundle and RunExport are added by subsequent
// tasks (1.3–1.4); this file currently provides the embed directive, the export
// data models, and the per-document renderer + metadata collector.
//
//go:embed export_ui
var exportUIFS embed.FS

// exportBundle là toàn bộ knowledge base được serialize vào file tĩnh.
// Đây là JSON blob nhúng vào HTML (window.__NS_KB__); Graph tái dùng nguyên
// specGraph nên export không nhân đôi logic dựng graph của knowledge core.
type exportBundle struct {
	Schema    string            `json:"schema"`    // "ns-workspace/export@1"
	Generated string            `json:"generated"` // RFC3339
	Project   exportProjectMeta `json:"project"`
	Documents []exportDocument  `json:"documents"`
	Graph     specGraph         `json:"graph"` // tái dùng struct sẵn có
}

// exportProjectMeta là metadata cấp project nhúng vào bundle.
type exportProjectMeta struct {
	Name     string   `json:"name"`
	DocsRoot string   `json:"docsRoot"`
	Total    int      `json:"total"`
	Warnings []string `json:"warnings,omitempty"`
}

// exportDocument là specDocument đã render sang HTML an toàn để hiển thị offline.
type exportDocument struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Path         string            `json:"path"`
	Format       string            `json:"format"` // markdown | html | text
	Category     string            `json:"category"`
	Meta         map[string]string `json:"meta,omitempty"` // status, version, tags...
	RenderedHTML string            `json:"renderedHtml"`   // markdown→HTML đã sanitize
}

// exportRenderPlaceholder là nội dung thay thế khi render một doc thất bại.
// Permissive consumer: một doc lỗi không được làm hỏng toàn bộ export.
const exportRenderPlaceholder = "<p>(render failed)</p>"

// renderDocumentHTML chuyển raw markdown/html của doc sang HTML đã sanitize cho
// viewer tĩnh. Markdown render bằng goldmark (renderMarkdown, đã là dependency);
// HTML doc đi qua sanitizer dựa trên golang.org/x/net/html (đã có). Hàm fail-open:
// mọi lỗi (kể cả panic) được nuốt và trả về placeholder thay vì lan ra ngoài, kèm
// error để caller có thể ghi warning.
func renderDocumentHTML(doc specDocument) (rendered string, err error) {
	defer func() {
		if r := recover(); r != nil {
			rendered = exportRenderPlaceholder
			err = fmt.Errorf("render panic for %s: %v", doc.ID, r)
		}
	}()

	switch strings.ToLower(strings.TrimSpace(doc.Format)) {
	case "html":
		out, sanitizeErr := sanitizeExportHTML(doc.Raw)
		if sanitizeErr != nil {
			return exportRenderPlaceholder, fmt.Errorf("sanitize html for %s: %w", doc.ID, sanitizeErr)
		}
		return out, nil
	case "markdown", "":
		out, convErr := renderMarkdown([]byte(doc.Raw))
		if convErr != nil {
			return exportRenderPlaceholder, fmt.Errorf("render markdown for %s: %w", doc.ID, convErr)
		}
		return out, nil
	default:
		// Plain text (hoặc format không biết): escape và bọc trong <pre>.
		return "<pre>" + exportEscapeText(doc.Raw) + "</pre>", nil
	}
}

// collectDocMeta map metadata của doc (status/version/tags...) thành map[string]string
// cho viewer tĩnh. Chỉ giữ các field không rỗng để bundle gọn.
func collectDocMeta(doc specDocument) map[string]string {
	meta := map[string]string{}
	set := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			meta[key] = value
		}
	}
	set("status", doc.Status)
	set("version", doc.Version)
	set("compliance", doc.Compliance)
	set("priority", doc.Priority)
	set("type", doc.Type)
	set("timestamp", doc.Timestamp)
	set("description", doc.Description)
	set("category", doc.Category)
	set("language", doc.Language)
	if len(doc.Tags) > 0 {
		set("tags", strings.Join(doc.Tags, ", "))
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

// exportBlockedHTMLTags là các phần tử bị loại bỏ hoàn toàn khỏi HTML doc khi
// export tĩnh (script/style/embed... có thể chạy code hoặc gọi mạng).
var exportBlockedHTMLTags = map[string]bool{
	"script": true,
	"style":  true,
	"iframe": true,
	"object": true,
	"embed":  true,
	"link":   true,
	"meta":   true,
	"base":   true,
	"form":   true,
}

// sanitizeExportHTML parse HTML doc bằng golang.org/x/net/html, loại bỏ các tag
// nguy hiểm + attribute event handler (on*) + URL scheme nguy hiểm, rồi render
// lại body. Permissive: nội dung hợp lệ được giữ nguyên.
func sanitizeExportHTML(raw string) (string, error) {
	root, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return "", err
	}
	sanitizeHTMLChildren(root)

	body := findHTMLBody(root)
	var buf bytes.Buffer
	if body != nil {
		for child := body.FirstChild; child != nil; child = child.NextSibling {
			if err := html.Render(&buf, child); err != nil {
				return "", err
			}
		}
		return buf.String(), nil
	}
	if err := html.Render(&buf, root); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// sanitizeHTMLChildren duyệt và làm sạch các node con của node hiện tại tại chỗ.
func sanitizeHTMLChildren(node *html.Node) {
	var toRemove []*html.Node
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && exportBlockedHTMLTags[strings.ToLower(child.Data)] {
			toRemove = append(toRemove, child)
			continue
		}
		if child.Type == html.ElementNode {
			child.Attr = sanitizeHTMLAttrs(child.Attr)
		}
		sanitizeHTMLChildren(child)
	}
	for _, child := range toRemove {
		node.RemoveChild(child)
	}
}

// sanitizeHTMLAttrs loại bỏ event handler (on*) và URL scheme nguy hiểm.
func sanitizeHTMLAttrs(attrs []html.Attribute) []html.Attribute {
	cleaned := make([]html.Attribute, 0, len(attrs))
	for _, attr := range attrs {
		key := strings.ToLower(attr.Key)
		if strings.HasPrefix(key, "on") {
			continue
		}
		if (key == "href" || key == "src" || key == "xlink:href" || key == "action" || key == "formaction") && hasDangerousURLScheme(attr.Val) {
			continue
		}
		cleaned = append(cleaned, attr)
	}
	return cleaned
}

// hasDangerousURLScheme kiểm tra URL có scheme thực thi script / nhúng HTML.
func hasDangerousURLScheme(value string) bool {
	lower := strings.ToLower(strings.TrimLeft(value, "\t\n\r\f\v "))
	return strings.HasPrefix(lower, "javascript:") ||
		strings.HasPrefix(lower, "vbscript:") ||
		strings.HasPrefix(lower, "data:text/html")
}

// findHTMLBody tìm phần tử <body> đầu tiên trong cây đã parse.
func findHTMLBody(root *html.Node) *html.Node {
	var body *html.Node
	walkHTML(root, func(node *html.Node) {
		if body == nil && node.Type == html.ElementNode && strings.EqualFold(node.Data, "body") {
			body = node
		}
	})
	return body
}

// exportEscapeText escape các ký tự HTML nhạy cảm cho nội dung plain text.
var exportTextEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

func exportEscapeText(value string) string {
	return exportTextEscaper.Replace(value)
}

// exportOptions gom các tham số điều khiển một lần export tĩnh. RunExport (task
// 1.4) parse flags vào struct này, exportStaticBundle/injectBundle đọc nó để
// quyết định nhúng graph và inline/CDN assets.
type exportOptions struct {
	projectRoot  string
	docsDir      string
	outPath      string
	includeGraph bool
	inlineAssets bool
	openBrowser  bool
}

// Đường dẫn asset trong embed FS (export_ui/...). Tách thành hằng để inject và
// loader template dùng chung, tránh lệch path.
const (
	exportTemplatePath  = "export_ui/export.html.tmpl"
	exportStylePath     = "export_ui/export.css"
	exportAppScriptPath = "export_ui/export.js"
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

// exportTemplateData khớp các field mà export.html.tmpl tham chiếu. Các kiểu
// template.CSS/HTML/JS đánh dấu nội dung đã tin cậy (asset tĩnh + JSON đã escape)
// nên html/template chèn verbatim thay vì escape lần nữa.
type exportTemplateData struct {
	Title      string
	StyleCSS   template.CSS
	VendorHead template.HTML
	BundleJSON template.JS
	AppJS      template.JS
}

// exportStaticBundle build exportBundle từ specProject rồi render HTML hoàn chỉnh.
// Chỉ nhúng docs + graph + meta của CHÍNH project được truyền vào (Req 2.5): không
// đọc thêm file hay nguồn nào ngoài project.Documents/project.Graph/project.Summary.
// Graph chỉ được gắn khi opt.includeGraph (Req 2.2/2.3), ngược lại để rỗng.
func exportStaticBundle(project specProject, opt exportOptions) ([]byte, error) {
	bundle := exportBundle{
		Schema:    "ns-workspace/export@1",
		Generated: time.Now().UTC().Format(time.RFC3339),
		Project: exportProjectMeta{
			Name:     project.Summary.Name,
			DocsRoot: project.Summary.DocsRoot,
			Total:    project.Summary.TotalSpecs,
			Warnings: project.Summary.Warnings,
		},
	}

	// Mọi doc của project đều có mặt trong bundle (Req 2.1). Render permissive:
	// lỗi một doc trả placeholder thay vì làm hỏng cả export (Req 2.4).
	for _, doc := range project.Documents {
		rendered, err := renderDocumentHTML(doc)
		if err != nil {
			rendered = exportRenderPlaceholder
		}
		bundle.Documents = append(bundle.Documents, exportDocument{
			ID:           doc.ID,
			Title:        doc.Title,
			Path:         doc.Path,
			Format:       doc.Format,
			Category:     doc.Category,
			Meta:         collectDocMeta(doc),
			RenderedHTML: rendered,
		})
	}

	if opt.includeGraph {
		bundle.Graph = project.Graph // tái dùng nguyên specGraph
	}

	return injectBundle(exportTemplate, bundle, opt)
}

// injectBundle nhúng bundle (JSON blob → window.__NS_KB__) + assets vào template.
// inlineAssets=true: inline export.css, export.js và vendor libs từ embed FS để
// file mở offline qua file:// (Req 2 self-contained). inlineAssets=false: vendor
// libs tham chiếu CDN, vẫn inline CSS/JS của UI export.
func injectBundle(tmpl *template.Template, bundle exportBundle, opt exportOptions) ([]byte, error) {
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

	data := exportTemplateData{
		Title:      exportPageTitle(bundle.Project.Name),
		StyleCSS:   template.CSS(styleCSS),
		VendorHead: vendorHead,
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
