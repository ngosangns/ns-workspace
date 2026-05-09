# Resolve Internal Links Và Mentions Trong Preview

## Meta

- **Status**: implemented
- **Description**: Spec đã triển khai cho việc resolve Markdown internal links, mentions dạng path và heading fragments bằng router preview.
- **Compliance**: current-state
- **Links**: [Preview web](../../features/preview-web.md), [Module preview](../../modules/preview.md), [Chỉ mục](../../_index.md)

## Tổng Quan

Markdown preview xử lý link nội bộ bằng router của preview thay vì để browser đi tới raw path. Sau khi Markdown được render và sanitize, frontend decorate anchors và text mentions để gắn navigation handler khi target khớp một tài liệu trong `state.specs`.

## Yêu Cầu

### REQ-1: Link Markdown nội bộ dùng router

**Tiêu Chí Chấp Nhận:**

- [x] Link tương đối như `./auth.md` hoặc `../module/doc.md` resolve theo path tài liệu hiện tại.
- [x] Link dạng `docs/...`, `specs/...` và path docs-relative được normalize trước khi match.
- [x] Click link nội bộ gọi `selectSpec` và cập nhật route `/spec/...`.
- [x] External links như `https://...` và `mailto:` không bị intercept.

### REQ-2: Mentions được link hóa an toàn

**Tiêu Chí Chấp Nhận:**

- [x] `@doc/...` và `@spec/...` trở thành link nội bộ khi resolve được.
- [x] Plain path có đuôi `.md` được link hóa khi target tồn tại.
- [x] Text trong `code`, `pre`, `script` và `style` không bị link hóa.

### REQ-3: Fragment heading

**Tiêu Chí Chấp Nhận:**

- [x] Fragment trong `target.md#heading` được giữ trong route.
- [x] Sau khi mở target, preview scroll tới heading hoặc anchor tương ứng nếu tìm thấy.

## Ghi Chú Triển Khai

Resolver chạy hoàn toàn ở client dựa trên danh sách docs đã load. Helper điều hướng đóng preview modal khi click link nội bộ để tránh route preview file/doc chồng lên route tài liệu chính.

## Quan Hệ

Spec này bổ sung hành vi cho [Preview web](../../features/preview-web.md) và phụ thuộc frontend TypeScript trong [Module preview](../../modules/preview.md).
