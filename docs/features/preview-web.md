# Preview Web

## Meta

- **Status**: active
- **Description**: Mô tả hành vi shipped của preview web, bao gồm Doc view, Graph view, Search tab, routing nội bộ và các API frontend.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Module preview](../modules/preview.md), [Kiến trúc tổng quan](../architecture/overview.md), [Trang search preview](../specs/planning/add-preview-search-page.md), [Internal links và mentions](../specs/planning/resolve-preview-internal-links-and-mentions.md), [Renderer graph preview](../specs/planning/use-specialized-graph-renderer.md)

## Tổng Quan

Lệnh `preview` chạy một web server local để đọc thư mục `docs/` của project. Giao diện có sidebar tài liệu, khu vực đọc Doc, tab Graph và tab Search. Trang tổng quan riêng đã được bỏ; màn hình mặc định mở tài liệu đầu tiên trong danh sách scan được.

## Hành Vi Chính

- Doc tab render Markdown client-side, hỗ trợ code highlight, Mermaid diagram và pan/zoom cho diagram.
- Doc tab có nút xem raw Markdown để chuyển nhanh giữa rendered view và source Markdown.
- Preview modal cho doc/file có nút chuyển giữa rendered preview và raw source hiện tại.
- Khi chọn text trong Doc hoặc preview modal, context menu có nút Copy để copy reference dạng `path:start-end`.
- Topbar chỉ điều hướng các view phụ như Graph và Search; tài liệu đang đọc được chọn trong sidebar và được xác nhận bằng route `/spec/...`.
- Graph tab hiển thị graph tài liệu từ `_index.md`, metadata, relationship và dependency diagram bằng Sigma/Graphology WebGL renderer; click node chỉ chọn node và cập nhật details panel, còn preview doc/file được mở bằng nút trong details panel.
- Search tab có bốn panel: Docs Semantic, Docs Graph, Code Semantic và Code Graph. Docs search lấy toàn bộ file text trong `docs/`, còn code search bỏ qua docs root để tránh trùng kết quả. Query nhiều keyword phân tách bằng dấu phẩy có thể chạy theo chế độ tổng keyword hoặc hiệu keyword; hiệu keyword dùng nhóm đầu làm tập gốc rồi loại kết quả match các nhóm sau. Result cards của tài liệu ưu tiên hiển thị metadata `Description` khi doc có khai báo. Graph panels dùng semantic results làm anchor rồi mở rộng qua docs graph hoặc graphify graph để hiển thị context nhiều tầng; click graph node trong panels cũng chỉ chọn node để hiển thị preview actions trong details panel.
- Link Markdown nội bộ và mention dạng `@doc/...` hoặc `@spec/...` được resolve bằng router preview khi target khớp tài liệu.
- External link và anchor nội trang vẫn giữ hành vi browser bình thường.
- API file preview cho phép đọc file UTF-8 trong docs root ngay cả khi extension không thuộc nhóm source code previewable; file ngoài docs vẫn phải qua allowlist extension.

## API Liên Quan

Preview frontend gọi `/api/project`, `/api/docs`, `/api/docs/{id}`, `/api/graph`, `/api/search`, `/api/files` và `/api/events`. Search graph dùng typed docs graph và dùng thêm `graphify-out/graph.json` nếu file này có trong project root. `/api/search` nhận `q`, `limit` và `keywordOp=sum|difference`; response trả metadata graph optional như anchor, anchorId và depth để mô tả node được neo từ semantic result hay được mở rộng qua quan hệ graph.

## Quan Hệ

Feature này được implement bởi [Module preview](../modules/preview.md). Các kế hoạch đã triển khai được mô tả trong [Trang search preview](../specs/planning/add-preview-search-page.md), [Internal links và mentions](../specs/planning/resolve-preview-internal-links-and-mentions.md), [TypeScript cho preview web](../specs/planning/use-full-typescript-for-preview-web.md) và [Renderer graph preview](../specs/planning/use-specialized-graph-renderer.md).
