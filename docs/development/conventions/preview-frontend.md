# Quy Ước Frontend Preview

## Meta

- **Status**: active
- **Description**: Quy ước phát triển frontend preview, gồm source TypeScript, generated assets, lệnh kiểm tra và metadata docs được preview scan.
- **Compliance**: current-state
- **Links**: [Module preview](../../modules/preview.md), [Preview web](../../features/preview-web.md), [TypeScript cho preview web](../../specs/planning/use-full-typescript-for-preview-web.md)

## Quy Ước

Source chính của frontend preview nằm trong `internal/preview/preview_ui_src/`. File generated trong `internal/preview/preview_ui/` được Go embed và phải được cập nhật bằng build, không sửa tay khi thay đổi logic.

Các lệnh kiểm tra chính:

```bash
npm run check:preview
npm run lint:preview
npm run format:preview:check
npm run build:preview
```

Sau khi chỉnh UI, chạy build để đồng bộ `app.js` và `js/graph.js`. Nếu thay đổi route hoặc HTML static, cập nhật test preview tương ứng trong `internal/preview/preview_test.go`.

Tài liệu preview phải dùng metadata đúng parser hiện tại: `_sync.md` có `## Current Sync`, `_index.md` có `## Modules`, và mỗi doc chính có frontmatter `---` hoặc `## Meta` với key tiếng Anh dạng bullet/table. Không sửa tay JavaScript generated trong `internal/preview/preview_ui/` nếu logic source TypeScript chưa được cập nhật tương ứng.
