# Support HTML Body Links Trong Docs Graph

## Bối Cảnh

Docs Graph xây dựng đồ thị quan hệ giữa các document bằng cách extract edges từ metadata và nội dung. Hiện tại, hệ thống đã hỗ trợ extract `<a href>` từ `<doc-meta>` và `<doc-relation>` tags, nhưng **không extract `<a href>` từ nội dung body** của HTML docs.

project sử dụng HTML docs với links navigation trong body:

```html
<ul>
  <li><a href="../architecture/overview.html">Architecture</a></li>
  <li><a href="../modules/editor-system.html">Editor System</a></li>
</ul>
```

Những link này hiện bị bỏ qua hoàn toàn khi xây dựng đồ thị.

## Nguyên Nhân

Tại `spec_project.go:802-811`, `parseDocumentContentEdges` xử lý HTML body bằng cách:

1. Gọi `htmlContentWithoutMetadata(doc.Raw)` → trả về `data.BodyText` (plain text, đã strip HTML tags)
2. Truyền plain text vào `edgesFromMarkdownLinks()` → chỉ match `[text](url)` Markdown syntax

→ Mọi `<a href>` trong HTML body bị invisible đối với edge extraction.

Ngược lại, `parseHTMLDocumentData` (line 909-928) chỉ extract `<a>` tags khi chúng nằm **bên trong** `<doc-meta>`. Các `<a>` tags ngoài `<doc-meta>` bị ignore.

## Góc Nhìn Tổng Quan

```
parseDocumentConnectionEdges
├── parseDocumentMetadataEdges   ← đã extract <a> từ <doc-meta> ✅
└── parseDocumentContentEdges    ← chỉ extract plain text, bỏ qua <a> ❌
```

Phần cần sửa tập trung ở `parseDocumentContentEdges` và `parseHTMLDocumentData`.

## Mục Tiêu

HTML `<a href>` links trong body content của HTML docs được extract thành graph edges, tương đương với cách Markdown `[text](url)` links đã được xử lý.

## Ngoài Phạm Vi

- Frontend rendering/decoration của links (đã có `decorateInternalDocLinks` xử lý)
- Thay đổi behavior của `<doc-relation>` hoặc `<doc-meta>` links
- Thay đổi graph search algorithm

## Cấu Trúc Giải Pháp

Thêm function `edgesFromHTMLLinks` để parse `<a href>` từ HTML content, gọi trong `parseDocumentContentEdges` khi document là HTML format.

```mermaid
flowchart LR
  A["parseDocumentContentEdges"] --> B{doc.Format == "html"?}
  B -- Yes --> C["htmlContentWithoutMetadata (plain text)"]
  B -- Yes --> D["edgesFromHTMLLinks (NEW)"]
  B -- No --> E["contentWithoutMetadata"]
  C --> F["edgesFromSemanticReferences"]
  C --> G["edgesFromMarkdownLinks"]
  C --> H["edgesFromPlainDocPaths"]
  D --> I["dedupeEdges"]
  F --> I
  G --> I
  H --> I
```

## Chi Tiết Triển Khai

### 1. Thêm `edgesFromHTMLLinks` function

Location: `spec_project.go`, sau `edgesFromPlainDocPaths` (line ~1189)

```go
func edgesFromHTMLLinks(sourcePath, from, raw string, relationType, origin string, docByPath map[string]specDocument, diagramLabelSet map[string]bool) []graphEdge {
	out := []graphEdge{}
	root, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return out
	}
	walkHTML(root, func(node *html.Node) {
		if node.Type != html.ElementNode || !strings.EqualFold(node.Data, "a") {
			return
		}
		if insideHTMLTag(node, "doc-meta") {
			return
		}
		href := htmlAttr(node, "href")
		if href == "" {
			return
		}
		if target, ok := resolveSpecReference(sourcePath, href, docByPath, diagramLabelSet); ok && from != target {
			out = append(out, graphEdge{From: from, To: target, Label: relationType, Type: relationType, Origin: origin, Raw: href})
		}
	})
	return out
}
```

Key decisions:

- Parse HTML thay vì dùng regex để xử lý đúng attribute quoting, nested tags
- Skip `<a>` bên trong `<doc-meta>` vì đã được `parseDocumentMetadataEdges` xử lý
- Dùng `walkHTML` + `htmlAttr` (infrastructure đã có sẵn)
- Gọi `resolveSpecReference` để resolve relative paths

### 2. Sửa `parseDocumentContentEdges`

Location: `spec_project.go:802-811`

Thêm gọi `edgesFromHTMLLinks` khi `doc.Format == "html"`:

```go
func parseDocumentContentEdges(doc specDocument, from string, docByPath map[string]specDocument, diagramLabelSet map[string]bool) []graphEdge {
	content := contentWithoutMetadata(doc.Raw)
	if doc.Format == "html" {
		content = htmlContentWithoutMetadata(doc.Raw)
	}
	edges := edgesFromSemanticReferences(doc.Path, from, content, "inline", docByPath, diagramLabelSet)
	edges = append(edges, edgesFromMarkdownLinks(doc.Path, from, content, defaultSpecRelation, "inline", docByPath, diagramLabelSet)...)
	edges = append(edges, edgesFromPlainDocPaths(doc.Path, from, content, defaultSpecRelation, "inline", docByPath, diagramLabelSet)...)
	if doc.Format == "html" {
		edges = append(edges, edgesFromHTMLLinks(doc.Path, from, doc.Raw, defaultSpecRelation, "inline", docByPath, diagramLabelSet)...)
	}
	return dedupeEdges(edges)
}
```

Lưu ý: truyền `doc.Raw` (full HTML) thay vì `content` (plain text) vì cần parse HTML DOM.

### 3. Thêm test

Location: `spec_project_test.go`

Test case mới verify rằng `<a href>` trong HTML body được extract thành graph edges:

```go
func TestScanSpecProjectExtractsHTMLBodyLinks(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "docs/modules/billing.html", `<doc-meta status="Active"><doc-title>Billing</doc-title></doc-meta>
<main>
  <p>See <a href="../shared/data-models.html">Data Models</a> for details.</p>
</main>`)
	writeTestFile(t, root, "docs/shared/data-models.html", `<doc-meta status="Active"><doc-title>Data Models</doc-title></doc-meta><p>Models.</p>`)

	project, err := scanSpecProject(root, "docs")
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(project.Graph.Edges, "billing", "data-models") {
		t.Fatalf("expected edge from billing to data-models for body link, got: %+v", project.Graph.Edges)
	}
}
```

## Công Việc Cần Làm

1. Thêm `edgesFromHTMLLinks` function vào `spec_project.go`
2. Sửa `parseDocumentContentEdges` để gọi `edgesFromHTMLLinks` cho HTML docs
3. Thêm test `TestScanSpecProjectExtractsHTMLBodyLinks` vào `spec_project_test.go`
4. Chạy `go test ./internal/preview/...` để verify

## Rủi Ro Và Ràng Buộc

- **Double parsing HTML**: `edgesFromHTMLLinks` parse HTML lần thứ hai (lần đầu ở `parseHTMLDocumentData`). Có thể tối ưu bằng cách truyền parsed DOM, nhưng hiện tại không cần thiết vì HTML parsing nhanh và code rõ ràng hơn.
- **Deduplication**: `dedupeEdges` đã có sẵn, đảm bảo không tạo edge trùng lặp từ metadata và content.
- **`<a>` trong `<doc-meta>`**: `edgesFromHTMLLinks` skip `<a>` bên trong `<doc-meta>` để tránh duplicate với `parseDocumentMetadataEdges`.

## Kiểm Chứng

```sh
go test ./internal/preview/... -run TestScanSpecProject -v
```
