# Trang Search Cho Preview

## Meta

- **Status**: implemented
- **Description**: Spec đã triển khai cho Search tab của preview, mô tả docs/code semantic search, graph search và UI bốn panel.
- **Compliance**: current-state
- **Links**: [Preview web](../../features/preview-web.md), [Module preview](../../modules/preview.md), [Chỉ mục](../../_index.md)

## Tổng Quan

Preview web có Search tab riêng để tìm trên docs, source code và graph context. UI hiển thị bốn panel cố định: Docs Semantic, Docs Graph, Code Semantic và Code Graph. Backend phục vụ API `/api/search` với response ổn định để từng panel render độc lập.

## Yêu Cầu

### REQ-1: Search docs và code

**Tiêu Chí Chấp Nhận:**

- [x] `/api/search?q=...` trả kết quả docs semantic và code semantic.
- [x] Query rỗng không làm UI crash.
- [x] Kết quả docs có `specId` để mở bằng preview router.
- [x] Kết quả code có path, line và excerpt khi có thể.
- [x] Docs semantic scan cả file text trong docs root; code semantic bỏ qua docs root để kết quả không bị trùng.

### REQ-2: Search graph

**Tiêu Chí Chấp Nhận:**

- [x] Docs Graph dùng typed docs graph hiện có.
- [x] Code Graph đọc `graphify-out/graph.json` khi file tồn tại.
- [x] Graph panels ưu tiên mở rộng từ Docs Semantic và Code Semantic results trước khi fallback sang query graph search.
- [x] Graphify thiếu hoặc invalid chỉ tạo warning, không làm API lỗi.
- [x] Neighbor relationships được trả về để UI hiển thị context.

### REQ-3: UI search page

**Tiêu Chí Chấp Nhận:**

- [x] Header có Search tab icon-only.
- [x] `/search` route giữ query trong URL.
- [x] Bốn panel có count, loading state và empty state riêng.
- [x] Kết quả spec/file mở được preview modal hoặc Doc tab đúng target.

## Ghi Chú Triển Khai

Search dùng local scoring và fallback an toàn thay vì bắt buộc embedding runtime. Hybrid mode merge nhiều tín hiệu, gồm keyword, semantic fallback và graph context. Docs Graph và Code Graph dùng semantic results làm anchor để mở rộng qua docs graph hoặc graphify graph theo nhiều tầng, rồi fallback sang query graph search nếu không map được anchor. Code scanner bỏ qua binary, cache, docs root và generated folder lớn để preview vẫn nhẹ. File preview cho docs root cho phép mở file UTF-8 không có extension source-code quen thuộc, còn file ngoài docs vẫn dùng allowlist previewable.

## Quan Hệ

Spec này đã được phản ánh trong [Preview web](../../features/preview-web.md) và được implement bởi [Module preview](../../modules/preview.md). Quy ước build frontend nằm trong [Quy ước frontend preview](../../development/conventions/preview-frontend.md).
