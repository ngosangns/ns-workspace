# Dùng TypeScript Cho Preview Web

## Meta

- **Status**: implemented
- **Description**: Spec đã triển khai cho việc chuyển frontend preview sang TypeScript, gồm typecheck, lint, format, build và embed runtime assets.
- **Compliance**: current-state
- **Links**: [Module preview](../../modules/preview.md), [Preview web](../../features/preview-web.md), [Quy ước frontend preview](../../development/conventions/preview-frontend.md), [Chỉ mục](../../_index.md)

## Tổng Quan

Frontend preview dùng TypeScript làm source of truth trong `internal/preview/preview_ui_src/`. Build TypeScript emit JavaScript vào `internal/preview/preview_ui/` để Go embed và phục vụ runtime mà không yêu cầu Node trên máy người dùng.

## Yêu Cầu

### REQ-1: Source TypeScript

**Tiêu Chí Chấp Nhận:**

- [x] `app.ts` chứa logic router, state, Markdown render, search UI và preview modal.
- [x] `js/graph.ts` chứa graph rendering.
- [x] `types.d.ts` khai báo các browser globals cần thiết cho CDN libraries.

### REQ-2: Toolchain kiểm tra

**Tiêu Chí Chấp Nhận:**

- [x] `npm run check:preview` typecheck source.
- [x] `npm run lint:preview` lint source.
- [x] `npm run format:preview:check` kiểm tra format.
- [x] `npm run build:preview` generate runtime assets.

### REQ-3: Runtime embed ổn định

**Tiêu Chí Chấp Nhận:**

- [x] `index.html` vẫn load `/app.js`.
- [x] Go embed vẫn phục vụ `/app.js`, `/js/graph.js`, `/style.css`, `/favicon.svg` và SPA fallback.
- [x] Tests preview đọc source TypeScript cho assertion logic và kiểm tra runtime assets được serve.

## Ghi Chú Triển Khai

Không sửa tay generated JavaScript khi thay đổi logic frontend. Mọi thay đổi trong `preview_ui_src` phải được build lại trước khi verify Go tests.

## Quan Hệ

Quy ước vận hành nằm trong [Quy ước frontend preview](../../development/conventions/preview-frontend.md). Module sử dụng source này được mô tả ở [Module preview](../../modules/preview.md).
