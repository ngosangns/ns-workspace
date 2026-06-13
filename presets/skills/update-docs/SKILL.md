---
name: update-docs
description: Cập nhật knowledge base trong `./docs` cho khớp codebase. Trigger: cập nhật tài liệu, sync docs, làm mới spec, ghi lại implementation, cập nhật requirements.
---

# Cập Nhật Tài Liệu

Dùng skill này để cập nhật knowledge base sau nghiên cứu hoặc implementation. Đọc `AGENTS.md` hoặc `presets/agents/AGENTS.md` trước, rồi làm theo kiến trúc docs bên dưới trừ khi repo quy định khác.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`.

## Nguyên Tắc

- **Chỉ mô tả trạng thái hiện tại.** Không lưu history, changelog, version snapshot trong docs.
- **Root docs cố định:** Mọi tài liệu nằm trong `./docs` tính từ project root. Không ghi ra ngoài.
- **Cập nhật tinh gọn:** Sửa statement cũ tại chỗ khi behavior thay đổi. Không thêm correction bên cạnh nội dung stale.
- **Requirements theo feature/module:** Mỗi feature/module folder nên có `requirements.md` chứa critical requirements. Tạo/cập nhật khi user yêu cầu hoặc khi implementation thay đổi business rule, contract, invariant.

## Cấu Trúc Thư Mục

```text
docs/
├── README.md, overview.md, _index.md, _sync.md
├── specs/planning/          # spec/plan trước và trong triển khai
├── features/<feature>/      # hành vi đã shipped + requirements.md
├── modules/<module>/        # boundary, API, business rules + requirements.md
├── architecture/            # overview, decisions/, patterns/
├── shared/                  # models, glossary, conventions
├── development/conventions/
├── research/, learnings/, compliance/
```

## Quy Tắc Viết Docs

### Nội Dung

- Viết như tài liệu vận hành hiện tại. Tránh "trước đây", "vừa thêm", "sẽ đổi".
- Mỗi doc có phạm vi rõ: feature = hành vi đã shipped; module = boundary/API/rules; spec = yêu cầu chưa/đang triển khai.
- Ưu tiên câu ngắn, heading rõ. Xóa mô tả lặp, wrapper văn xuôi, câu chung chung.
- Ghi rõ constraint, assumption, failure mode, security/compliance rule nếu ảnh hưởng vận hành.

### Liên Kết

- Dùng link tương đối thật: `[Tên](../path/doc.md)`. Không tạo link placeholder.
- Source/related references nằm trong metadata (frontmatter hoặc `## Meta`), không tạo section tham khảo riêng trong body.
- Giữ quan hệ hai chiều khi cần: kiểm tra doc đích có cần link ngược.

### Markdown

- Frontmatter YAML + `## Meta` khi cần. Xem `_shared/templates/frontmatter-schema.md`.
- Mermaid/diagram chỉ khi giúp giải thích nhanh hơn văn bản.

### HTML

- Output là fragment, không full document shell.
- Dùng custom semantic tags (`doc-meta`, `doc-title`, `doc-description`) khi repo hỗ trợ.
- Không inline script/style, event handler, framework attributes, id tự sinh, class rỗng.

### Chất Lượng Diff

- Giữ diff nhỏ, có chủ đích. Không rewrite hàng loạt chỉ đổi style.
- Sau khi edit, đọc lại diff bắt link sai, stale statement, duplicate section.
- Chạy `npm run format:docs` rồi `npm run lint:docs` khi repo có script.

## Quy Trình

1. `git status --short` + `rg --files ./docs` để định vị docs hiện có.
2. Đọc `docs/_sync.md` để lấy synced commit. Nếu không có, xem docs là chưa sync.
3. So sánh sync-state commit với target commit bằng `git log --oneline`, `git diff --name-status`, `git diff`.
4. Nếu nhiều commit, duyệt theo thứ tự thời gian qua `git rev-list --reverse` để hiểu final behavior.
5. Đọc docs/specs bị chạm + code path đã đổi vừa đủ hiểu final behavior. Dùng `lsp-code-graph` khi cần code graph context.
6. Quyết định tập docs nhỏ nhất cần cập nhật. Tránh rewrite rộng.
7. Cập nhật docs mô tả thiết kế hiện tại. Xóa statement stale.
8. Tạo/cập nhật `requirements.md` khi scope là feature/module và có requirements critical.
9. Cập nhật `_index.md` khi thêm/move/xóa docs. Duy trì link hai chiều.
10. Cập nhật `docs/_sync.md` như snapshot sync cuối cùng. Xem `_shared/templates/sync-state-template.md`.
11. Chạy formatter + lint docs khi repo có script.

## Templates

Khi tạo docs mới, dùng templates trong `_shared/templates/`:

- `spec-template.md` cho specs
- `requirements-template.md` cho requirements.md
- `module-template.md` cho module docs
- `frontmatter-schema.md` cho metadata
- `sync-state-template.md` cho \_sync.md

## Phản Hồi Cuối

Báo cáo docs đã thay đổi, kết quả sync state và validation đã chạy. Giữ cô đọng.
