# Module Preview

## Meta

- **Status**: active
- **Description**: Tài liệu module `internal/preview`, mô tả data models, API read-only, parser metadata Markdown/HTML, search, graph và ràng buộc build frontend.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Preview web](../features/preview-web.md), [Kiến trúc tổng quan](../architecture/overview.md), [Remove preview editor](../specs/planning/remove-preview-web-editor.md), [HTML docs](../specs/planning/generate-docs-html-tailwind-custom-tags.md), [Mermaid/C4 rendering](../specs/planning/support-mermaid-and-c4-model-rendering.md), [TypeScript cho preview web](../specs/planning/use-full-typescript-for-preview-web.md), [Renderer graph preview](../specs/planning/use-specialized-graph-renderer.md), [Quy ước frontend preview](../development/conventions/preview-frontend.md)

## Tổng Quan

`internal/preview` cung cấp HTTP preview read-only cho knowledge base. Backend Go scan docs, parse metadata Markdown/HTML, dựng graph và trả API JSON. Frontend TypeScript render UI, router, Markdown preview bằng TOAST UI Viewer, HTML fragment preview bằng custom tag normalizer, Mermaid/Mermaid C4/LikeC4 rendering, selection copy menu, graph Sigma/Graphology, search panels, modal preview với raw/rendered toggle và hot reload.

## Data Models Và APIs

- `specProject` gom `projectSummary`, danh sách `specDocument` và `specGraph`.
- `projectSummary` mô tả project root, docs root, trạng thái `_index.md`, `_sync.md`, số lượng tài liệu, category, status, compliance và warning.
- `specDocument` đại diện cho từng file text trong docs với `id`, `title`, `path`, `language`, `format`, metadata, raw content và search text đã normalize.
- `specGraph` chứa nodes, edges, relationships, constraints và dependency diagram.
- `GET /api/docs/{id}` trả raw document content và metadata đã scan; `PUT /api/docs/{id}` bị từ chối vì preview không còn chỉnh sửa tài liệu.
- `/api/search` trả response hybrid với bốn panel để UI render độc lập và hỗ trợ `keywordOp=sum|difference` cho query nhiều keyword phân tách bằng dấu phẩy.
- Docs semantic search dùng `docsSearchDoc` để gom cả spec documents và file text còn lại trong docs root.
- Code semantic search scan project root nhưng bỏ qua docs root, cache, dependency folders, generated folders lớn và binary file.
- `specDocument.description` được parse từ metadata `Description`; docs semantic results truyền field này sang `previewSearchResult.description` để UI search hiển thị mô tả metadata trước excerpt nội dung.
- Keyword operator mặc định là `sum`, giữ hành vi cộng dồn match từ mọi keyword. Khi operator là `difference`, backend dùng keyword/nhóm đầu làm truy vấn chính và loại docs, code files hoặc graph nodes match các keyword/nhóm còn lại trước khi score.
- Docs Graph và Code Graph nhận semantic results làm anchors, match anchors vào docs graph hoặc graphify graph, rồi breadth-first expand qua neighbor relationships với depth và result cap.
- `/api/files` đọc file UTF-8 trong project root; file trong docs root được preview dù extension không nằm trong allowlist code.

## Quy Tắc Nghiệp Vụ

Preview không yêu cầu Node ở runtime vì Go embed static assets đã build. Khi sửa frontend, source of truth là `internal/preview/preview_ui_src/`; sau đó chạy build để cập nhật `internal/preview/preview_ui/`.

Doc tab không có edit mode và không có raw Markdown/source toggle. Markdown docs luôn render bằng TOAST UI Viewer; frontmatter đầu file được chuyển thành metadata table read-only, còn body Markdown được sanitize sau khi viewer render. HTML docs render bằng sanitize + custom tag normalizer và dùng MVP.css đã scope vào `.html-doc` làm baseline cho semantic HTML. `doc-meta` thành metadata table, `doc-title`/`doc-description` thành heading/copy, `<a href="...">label</a>` thành link metadata/router, `doc-relation` thành typed badge, `doc-callout` thành tone-aware callout, `doc-code` thành labelled code block và `doc-diagram`/`doc-graph` thành diagram source. Advanced layout tags include `doc-section`, `doc-grid`, `doc-card`, `doc-steps`, `doc-step`, `doc-flow`, `doc-flow-step`, `doc-metrics` and `doc-metric`. HTML preview root dùng class `html-doc` để padding, table, list, heading, code, report panel classes and custom tag styles không phụ thuộc vào TOAST UI Markdown DOM.

Diagram rendering chạy ở client cho cả Markdown và HTML. Code fence `mermaid`, `c4`, `c4-context`, `c4-container`, `c4-component`, `c4-dynamic`, `c4-deployment` và HTML `doc-diagram` được normalize vào cùng pipeline. Code fence `likec4` hoặc `c4-model` chỉ auto-render khi nội dung là `model { ... }`; frontend parse subset kiến trúc gồm `softwareSystem`, `container`, `component`, `description` và quan hệ `a.b -> c.d`, rồi chuyển sang Mermaid C4 để dùng cùng pipeline render, sanitize và pan/zoom. Diagram wheel zoom chỉ chạy khi người dùng giữ Command hoặc Ctrl trong lúc scroll, để scroll thường vẫn cuộn trang. Diagram dark theme cấu hình Mermaid theme variables, append `UpdateElementStyle` và `UpdateRelStyle` cho C4 source, rồi post-process SVG C4 styles để container border, container label, edge, marker và edge label giữ màu sáng có tương phản.

Metadata parser ưu tiên bảng `## Modules` trong `docs/_index.md` khi có. Fallback trong từng doc đọc metadata từ frontmatter `---`, bullet trong `## Meta`, bảng Markdown metadata hoặc HTML `doc-meta`. Frontend render metadata thành bảng read-only để preview dễ đọc; scalar string được bỏ quote ngoài, còn array như `["docs", "compliance"]` hiển thị thành danh sách badge. Graph metadata đọc các key liên kết như `Links`, `Depends`, `Provides`, `Consumes`, HTML `<a href="...">label</a>` trong `doc-meta` và `doc-relation`.

Internal link router resolve cả Markdown và HTML docs. Anchors thường, `<a href="...">label</a>`, `doc-relation target="..."`, `.md`/`.html` path và mention `@doc/...`/`@spec/...` đều được map qua alias của `specDocument` trước khi điều hướng trong SPA.

Trang tổng quan không còn là route hoặc tab riêng. Nếu URL cũ `/overview` được mở, router rơi về Doc tab mặc định. Sidebar chỉ tô active tài liệu khi route hiện tại là `/spec/...`, nên Graph và Search không giữ active doc cũ bằng state nội bộ.

## Ràng Buộc Và Giả Định

Server preview chạy local, mặc định bind `127.0.0.1:0` để hệ điều hành tự chọn port rảnh, và frontend runtime dùng CDN giống các thư viện UI hiện có. Khi chạy hot reload trong checkout của module, supervisor chọn một port rảnh một lần rồi truyền xuống child process để URL preview ổn định qua các lần restart. Semantic search hiện có fallback local khi embedding runtime không khả dụng. Graphify data là optional và không được coi là nguồn bắt buộc; khi không có graphify hoặc không map được semantic anchor, graph panels degrade bằng warning hoặc fallback query graph search. Graph UI dùng Sigma/Graphology WebGL renderer với layout ForceAtlas2 để xem graph nhiều nodes/edges nhẹ hơn D3 SVG force-layout cũ. Click node chỉ chọn node và cập nhật details panel; click nền graph bỏ chọn node; incoming/outgoing edge rows trong details panel chọn node liên quan trong graph hiện tại. Người dùng mở preview doc/file bằng nút trong details panel. Wheel zoom trên graph chỉ chạy khi người dùng giữ `Ctrl` hoặc `Meta`, để scroll trang không bị graph bắt ngoài ý muốn.

## Quan Hệ

Module này implements [Preview web](../features/preview-web.md), consumes docs structure trong [Chỉ mục](../_index.md), và được phát triển theo [Quy ước frontend preview](../development/conventions/preview-frontend.md).
