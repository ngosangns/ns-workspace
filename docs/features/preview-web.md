# Preview Web

## Meta

- **Status**: active
- **Description**: Mô tả hành vi shipped của preview web, bao gồm Doc view read-only, Markdown/HTML rendering, Graph view, Search tab, routing nội bộ và các API frontend.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Module preview](../modules/preview.md), [Kiến trúc tổng quan](../architecture/overview.md), [HTML docs](../specs/planning/generate-docs-html-tailwind-custom-tags.md), [Mermaid/C4 rendering](../specs/planning/support-mermaid-and-c4-model-rendering.md), [Trang search preview](../specs/planning/add-preview-search-page.md), [Internal links và mentions](../specs/planning/resolve-preview-internal-links-and-mentions.md), [Renderer graph preview](../specs/planning/use-specialized-graph-renderer.md)

## Tổng Quan

Lệnh `preview` chạy một web server local để đọc thư mục `docs/` của project. Giao diện có sidebar tài liệu, khu vực đọc Doc, tab Graph và tab Search. Trang tổng quan riêng đã được bỏ; màn hình mặc định mở tài liệu đầu tiên trong danh sách scan được.

## Hành Vi Chính

- Doc tab là read-only và luôn hiển thị rendered document; không còn nút raw Markdown/source trên màn hình tài liệu chính.
- Doc tab render Markdown client-side bằng TOAST UI Viewer, hiển thị frontmatter và section `## Meta` thành bảng read-only, render docs refs metadata thành badge links, giữ `data-source-language` cho code fence và chạy code highlight sau khi render.
- Doc tab render HTML docs bằng fragment đã sanitize và nạp MVP.css đã scope vào `.html-doc` để semantic HTML như `main`, `section`, `article`, `table`, `pre`, `ol` và `ul` có baseline đọc tốt mà không ảnh hưởng chrome của app. HTML preview hỗ trợ custom tags `doc-meta`, `doc-title`, `doc-description`, `doc-relation`, `doc-callout`, `doc-code`, `doc-diagram`, `doc-section`, `doc-grid`, `doc-card`, `doc-steps`, `doc-step`, `doc-flow`, `doc-flow-step`, `doc-graph`, `doc-metrics` và `doc-metric`. Metadata trong `doc-meta` được chuyển thành bảng read-only, link metadata dùng `<a href="...">label</a>` với label là content trong thẻ và target trong `href`, hiển thị thành badge links trong metadata table, còn custom tags trong body được normalize thành HTML preview an toàn với padding nội dung, styling riêng cho title, description, callout tone, relation badges, steps, flows, metric cards, code blocks, report panels, tables và diagram/graph sources.
- Markdown và HTML preview không đổi màu riêng cho visited links; link visited dùng cùng màu link hiện tại theo theme. Inline code dùng nền, bo góc, text color và weight riêng để khác biệt với văn bản thường, còn code block vẫn giữ nền block tối và màu syntax/highlight riêng.
- Markdown và HTML cùng dùng pipeline diagram read-only: Mermaid diagram, Mermaid C4 như `C4Component`, alias C4 fence như `c4-container`, LikeC4 `model { ... }` dạng C4 model và pan/zoom cho diagram; diagram dùng Command/Ctrl + scroll để zoom bằng wheel nên scroll thường vẫn cuộn trang. Dark theme giữ edge, marker và edge label sáng để đọc được trên nền tối.
- Preview modal cho doc/file có nút chuyển giữa rendered preview và raw source hiện tại.
- Khi chọn text trong Doc hoặc preview modal, context menu có nút Copy để copy reference dạng `path:start-end`.
- Topbar chỉ điều hướng các view phụ như Graph và Search; tài liệu hoặc folder docs đang đọc được chọn trong sidebar và được xác nhận bằng route `/spec/...`. Route folder như `/spec/docs`, `/spec/specs/planning` hoặc `/spec/docs/specs/planning` hiển thị danh sách folder con và docs trực tiếp trong folder đó.
- Graph tab hiển thị graph tài liệu từ `_index.md`, metadata, relationship và dependency diagram bằng Sigma/Graphology WebGL renderer; click node chỉ chọn node và cập nhật details panel, click nền graph bỏ chọn node, còn preview doc/file được mở bằng nút trong details panel. Danh sách incoming/outgoing trong details panel có thể chọn node liên quan để điều hướng trong graph hiện tại.
- Graph route lưu filter hiện tại bằng query param `q`, ví dụ `/graph?q=preview`, và khôi phục lại filter khi reload hoặc điều hướng browser history.
- Search tab có bốn panel: Docs Semantic, Docs Graph, Code Semantic và Code Graph. Docs search lấy toàn bộ file text trong `docs/`, còn code search bỏ qua docs root để tránh trùng kết quả. Search route lưu query hiện tại bằng `q` và chế độ kết hợp keyword bằng `keywordOp=sum|difference`, ví dụ `/search?q=auth,session&keywordOp=difference`; reload hoặc browser history khôi phục lại cả query và chế độ kết hợp. Query nhiều keyword phân tách bằng dấu phẩy có thể chạy theo chế độ tổng keyword hoặc hiệu keyword; hiệu keyword dùng nhóm đầu làm tập gốc rồi loại kết quả match các nhóm sau. Result cards của tài liệu ưu tiên hiển thị metadata `Description` khi doc có khai báo. Graph panels search trực tiếp trên docs graph và graphify graph bằng query hiện tại, vẫn trả neighbors để UI hiển thị context và preview actions; click graph node trong panels cũng chỉ chọn node để hiển thị preview actions trong details panel, click nền graph bỏ chọn node và các edge rows trong details panel chọn node liên quan.
- Link nội bộ trong Markdown và HTML, bao gồm `<a href="...">label</a>`, `doc-relation target="..."`, path `.md`/`.html`, và mention dạng `@doc/...` hoặc `@spec/...`, được resolve bằng router preview khi target khớp tài liệu.
- External link và anchor nội trang vẫn giữ hành vi browser bình thường.
- API file preview cho phép đọc file UTF-8 trong docs root ngay cả khi extension không thuộc nhóm source code previewable; file ngoài docs vẫn phải qua allowlist extension.

## API Liên Quan

Preview frontend gọi `/api/project`, `/api/docs`, `/api/docs/{id}`, `/api/graph`, `/api/search`, `/api/files` và `/api/events`. `GET /api/docs/{id}` đọc tài liệu; `PUT /api/docs/{id}` không còn được dùng và endpoint tài liệu là read-only. Search graph dùng typed docs graph và dùng thêm `graphify-out/graph.json` nếu file này có trong project root. `/api/search` nhận `q`, `limit` và `keywordOp=sum|difference`; response trả graph result kèm `nodeId`, path/line và neighbors để search graph panels render quan hệ trực tiếp quanh node match.

## Quan Hệ

Feature này được implement bởi [Module preview](../modules/preview.md). Các kế hoạch đã triển khai được mô tả trong [HTML docs](../specs/planning/generate-docs-html-tailwind-custom-tags.md), [Mermaid/C4 rendering](../specs/planning/support-mermaid-and-c4-model-rendering.md), [Trang search preview](../specs/planning/add-preview-search-page.md), [Internal links và mentions](../specs/planning/resolve-preview-internal-links-and-mentions.md) và [Renderer graph preview](../specs/planning/use-specialized-graph-renderer.md).
