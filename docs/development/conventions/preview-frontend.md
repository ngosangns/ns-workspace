# Quy Ước Frontend Preview

## Meta

- **Status**: active
- **Description**: Quy ước phát triển frontend preview, gồm source TypeScript, generated assets, lệnh kiểm tra và metadata docs được preview scan.
- **Compliance**: current-state
- **Links**: [Module preview](../../modules/preview.md), [Preview web](../../features/preview-web.md)

## Quy Ước

Source chính của frontend preview nằm trong `internal/preview/preview_ui_src/`. Entry `index.html` mount preview shell đầy đủ; route `/search` do SPA fallback xử lý và hiển thị tab Search trong cùng app. Vue components và các shared module trong `preview_ui_src/js/` là nguồn sự thật cho UI logic; không giữ hoặc phục hồi shell TypeScript/vanilla JS cũ khi thay đổi preview.

Các renderer dùng chung nằm trong `internal/preview/preview_ui_src/js/`: `markdown.ts` cho Markdown/docs, `html-doc.ts` cho HTML document shell, `metadata.ts` cho metadata cards/links, `code-preview.ts` cho code/text preview, và `diagrams.ts` cho Mermaid. Khi Doc view và preview modal cần cùng hành vi render, cập nhật các module này trước thay vì nhân đôi logic trong component.

File generated trong `internal/preview/preview_ui/` được Go embed và phải được cập nhật bằng build, không sửa tay khi thay đổi logic. Backend search bỏ qua generated assets trong `internal/preview/preview_ui/**`, nên nội dung searchable phải đến từ docs, source Go hoặc source frontend dưới `preview_ui_src/`.

Các lệnh kiểm tra chính:

```bash
npm run check:preview
npm run lint:preview
npm run lint:preview:fix
npm run format:preview:check
npm run build:preview
```

Preview frontend dùng ESLint cho TypeScript/Vue, Biome làm guard lint bổ sung và Prettier làm formatter chính. `npm run lint:preview` chạy cả ESLint và Biome; `npm run lint:preview:fix` chạy các fix tự động rồi format preview. Pre-commit hook được cài bằng `simple-git-hooks` qua `prepare` và chạy `lint-staged`, nên file preview đã stage sẽ nhận lint/format fix trước commit mà không thêm file ngoài staged set.

Sau khi chỉnh UI, chạy build để đồng bộ HTML entry và hashed assets trong `internal/preview/preview_ui/`. Nếu thay đổi route, HTML static hoặc entry standalone, cập nhật test preview tương ứng trong `internal/preview/preview_test.go`.

Tài liệu preview phải dùng metadata đúng parser hiện tại: `_sync.md` có `## Current Sync`, `_index.md` có `## Modules`, và mỗi doc chính có frontmatter `---` hoặc `## Meta` với key tiếng Anh dạng bullet/table. Docs refs trong metadata dùng key như `related`, `Links`, `link` hoặc `relation.*` để preview render thành badge links. Không sửa tay JavaScript generated trong `internal/preview/preview_ui/` nếu logic source TypeScript/Vue chưa được cập nhật tương ứng.

Styling cho rendered docs nằm trong `internal/preview/preview_ui/style.css`. Link state phải nhất quán giữa unvisited và visited để preview không đổi màu sau khi đọc, còn inline code phải khác biệt với text thường bằng background, radius, text color và font weight riêng nhưng không ảnh hưởng code block.
