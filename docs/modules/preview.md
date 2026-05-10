# Module Preview

## Meta

- **Status**: active
- **Description**: Tài liệu module `internal/preview`, mô tả data models, API, parser metadata, search, graph và ràng buộc build frontend.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Preview web](../features/preview-web.md), [Kiến trúc tổng quan](../architecture/overview.md), [TypeScript cho preview web](../specs/planning/use-full-typescript-for-preview-web.md), [Renderer graph preview](../specs/planning/use-specialized-graph-renderer.md), [Quy ước frontend preview](../development/conventions/preview-frontend.md)

## Tổng Quan

`internal/preview` cung cấp HTTP preview cho knowledge base. Backend Go scan docs, parse metadata, dựng graph và trả API JSON. Frontend TypeScript render UI, router, Markdown preview, raw Markdown toggle, selection copy menu, graph Sigma/Graphology, search panels, modal preview với raw/rendered toggle và hot reload.

## Data Models Và APIs

- `specProject` gom `projectSummary`, danh sách `specDocument` và `specGraph`.
- `projectSummary` mô tả project root, docs root, trạng thái `_index.md`, `_sync.md`, số lượng tài liệu, category, status, compliance và warning.
- `specDocument` đại diện cho từng file text trong docs với `id`, `title`, `path`, `language`, metadata và raw content.
- `specGraph` chứa nodes, edges, relationships, constraints và dependency diagram.
- `/api/search` trả response hybrid với bốn panel để UI render độc lập và hỗ trợ `keywordOp=sum|difference` cho query nhiều keyword phân tách bằng dấu phẩy.
- Docs semantic search dùng `docsSearchDoc` để gom cả spec documents và file text còn lại trong docs root.
- Code semantic search scan project root nhưng bỏ qua docs root, cache, dependency folders, generated folders lớn và binary file.
- `specDocument.description` được parse từ metadata `Description`; docs semantic results truyền field này sang `previewSearchResult.description` để UI search hiển thị mô tả metadata trước excerpt nội dung.
- Keyword operator mặc định là `sum`, giữ hành vi cộng dồn match từ mọi keyword. Khi operator là `difference`, backend dùng keyword/nhóm đầu làm truy vấn chính và loại docs, code files hoặc graph nodes match các keyword/nhóm còn lại trước khi score.
- Docs Graph và Code Graph nhận semantic results làm anchors, match anchors vào docs graph hoặc graphify graph, rồi breadth-first expand qua neighbor relationships với depth và result cap.
- `/api/files` đọc file UTF-8 trong project root; file trong docs root được preview dù extension không nằm trong allowlist code.

## Quy Tắc Nghiệp Vụ

Preview không yêu cầu Node ở runtime vì Go embed static assets đã build. Khi sửa frontend, source of truth là `internal/preview/preview_ui_src/`; sau đó chạy build để cập nhật `internal/preview/preview_ui/`.

Metadata parser ưu tiên bảng `## Modules` trong `docs/_index.md` khi có. Fallback trong từng doc đọc metadata từ frontmatter `---`, bullet trong `## Meta`, hoặc bảng Markdown metadata. Frontend render frontmatter đầu file thành bảng metadata để preview dễ đọc; scalar string được bỏ quote ngoài, còn array như `["docs", "compliance"]` hiển thị thành danh sách badge. Graph metadata đọc các key liên kết như `Links`, `Depends`, `Provides` và `Consumes`.

Trang tổng quan không còn là route hoặc tab riêng. Nếu URL cũ `/overview` được mở, router rơi về Doc tab mặc định. Sidebar chỉ tô active tài liệu khi route hiện tại là `/spec/...`, nên Graph và Search không giữ active doc cũ bằng state nội bộ.

## Ràng Buộc Và Giả Định

Server preview chạy local và frontend runtime dùng CDN giống các thư viện UI hiện có. Semantic search hiện có fallback local khi embedding runtime không khả dụng. Graphify data là optional và không được coi là nguồn bắt buộc; khi không có graphify hoặc không map được semantic anchor, graph panels degrade bằng warning hoặc fallback query graph search. Graph UI dùng Sigma/Graphology WebGL renderer với layout ForceAtlas2 để xem graph nhiều nodes/edges nhẹ hơn D3 SVG force-layout cũ. Click node chỉ chọn node và cập nhật details panel; người dùng mở preview doc/file bằng nút trong details panel.

## Quan Hệ

Module này implements [Preview web](../features/preview-web.md), consumes docs structure trong [Chỉ mục](../_index.md), và được phát triển theo [Quy ước frontend preview](../development/conventions/preview-frontend.md).
