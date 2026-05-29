# Quy Ước Frontend Preview

## Meta

- **Status**: active
- **Description**: Quy ước phát triển frontend preview, gồm source TypeScript, generated assets, lệnh kiểm tra và metadata docs được preview scan.
- **Compliance**: current-state
- **Links**: [Module preview](../../modules/preview.md), [Preview web](../../features/preview-web.md)

## Quy Ước

Source chính của frontend preview nằm trong `internal/preview/preview_ui_src/`. Entry `index.html` mount preview shell đầy đủ, còn `search.html` mount Search standalone cho lệnh `search`. File generated trong `internal/preview/preview_ui/` được Go embed và phải được cập nhật bằng build, không sửa tay khi thay đổi logic.

Các lệnh kiểm tra chính:

```bash
npm run check:preview
npm run lint:preview
npm run format:preview:check
npm run build:preview
```

Sau khi chỉnh UI, chạy build để đồng bộ HTML entry và hashed assets trong `internal/preview/preview_ui/`. Nếu thay đổi route, HTML static hoặc entry standalone, cập nhật test preview tương ứng trong `internal/preview/preview_test.go`.

Tài liệu preview phải dùng metadata đúng parser hiện tại: `_sync.md` có `## Current Sync`, `_index.md` có `## Modules`, và mỗi doc chính có frontmatter `---` hoặc `## Meta` với key tiếng Anh dạng bullet/table. Docs refs trong metadata dùng key như `related`, `Links`, `link` hoặc `relation.*` để preview render thành badge links. Không sửa tay JavaScript generated trong `internal/preview/preview_ui/` nếu logic source TypeScript chưa được cập nhật tương ứng.

Styling cho rendered docs nằm trong `internal/preview/preview_ui/style.css`. Link state phải nhất quán giữa unvisited và visited để preview không đổi màu sau khi đọc, còn inline code phải khác biệt với text thường bằng background, radius, text color và font weight riêng nhưng không ảnh hưởng code block.
