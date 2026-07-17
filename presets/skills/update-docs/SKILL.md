---
name: update-docs
description: Cập nhật knowledge base trong `./docs` cho khớp codebase theo layout flat (features, modules, architecture, specs). Trigger: cập nhật tài liệu, sync docs, làm mới spec, ghi lại implementation, cập nhật requirements.
---

# Cập Nhật Tài Liệu

Dùng skill này để cập nhật knowledge base sau nghiên cứu hoặc implementation. Đọc `AGENTS.md` hoặc `presets/agents/AGENTS.md` trước, rồi làm theo kiến trúc docs bên dưới trừ khi repo quy định khác.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`.

## Nguyên Tắc

- **Chỉ mô tả trạng thái hiện tại.** Không lưu history, changelog, version snapshot trong docs.
- **Root docs cố định:** Mọi tài liệu nằm trong `./docs` tính từ project root. Không ghi ra ngoài.
- **OKF-first:** Docs theo Open Knowledge Format — markdown + YAML frontmatter. Mọi doc mới hoặc doc đang sửa metadata **bắt buộc** có frontmatter với `type` (xem `_shared/templates/frontmatter-schema.md`). Không tạo mới block `## Meta`; chỉ giữ `## Meta` cũ để tương thích ngược.
- **Layout flat:** Dùng `docs/features/`, `docs/modules/`, `docs/architecture/`, `docs/specs/planning/`, `docs/shared/`. Không bắt buộc tách `docs/business` + `docs/developer` trừ khi user yêu cầu migrate.
- **Cập nhật tinh gọn:** Sửa statement cũ tại chỗ khi behavior thay đổi. Không thêm correction bên cạnh nội dung stale.
- **Requirements theo feature/module (optional):** Có thể thêm `requirements.md` cạnh feature/module khi có critical requirements.

## Cấu Trúc Thư Mục

```text
docs/
├── README.md
├── _index.md
├── _sync.md
├── architecture/
├── modules/
├── features/
├── specs/planning/
├── shared/
├── development/conventions/
├── research/
└── working-documents/
```

## Phân Loại Nội Dung

| Loại nội dung                                      | Ví dụ vị trí                                      |
| -------------------------------------------------- | ------------------------------------------------- |
| User workflow, acceptance criteria, business rules | `docs/features/<feature>.md`                      |
| Technical implementation, API, module boundary     | `docs/modules/<module>.md`                        |
| Architecture, patterns, dev conventions            | `docs/architecture/`, `docs/development/`         |
| Domain glossary                                    | `docs/shared/`                                    |
| Active design plans                                | `docs/specs/planning/`                            |

Góc nhìn viết:

- Feature docs: _người dùng làm gì_, _kết quả mong đợi_, _ràng buộc_, _acceptance criteria_, kèm note kỹ thuật khi cần.
- Module docs: _kiến trúc_, _contract_, _API_, _dependency_, _failure mode_, _invariants_.

## Quy Tắc Viết Docs

### Nội Dung

- Viết như tài liệu vận hành hiện tại. Tránh "trước đây", "vừa thêm", "sẽ đổi".
- Ưu tiên câu ngắn, heading rõ. Xóa mô tả lặp, wrapper văn xuôi, câu chung chung.
- Ghi rõ constraint, assumption, failure mode, security/compliance rule nếu ảnh hưởng vận hành.

### Liên Kết

- Dùng link tương đối thật: `[Tên](../path/doc.md)`. Không tạo link placeholder.
- Ưu tiên dạng OKF bundle-relative `[Tên](/modules/preview.md)` (bắt đầu bằng `/`, tính từ docs root) khi phù hợp.
- Source/related references nằm trong metadata frontmatter (`resource`, `tags`) hoặc link trong body.
- Giữ quan hệ hai chiều khi cần: kiểm tra doc đích có cần link ngược.

### Markdown

- **Frontmatter YAML là bắt buộc** cho doc mới, theo `_shared/templates/frontmatter-schema.md`: `type` (required) + `title`/`description`/`tags`/`timestamp`. Không tạo mới `## Meta`.
- Mermaid/diagram chỉ khi giúp giải thích nhanh hơn văn bản.

### HTML

- Output là fragment, không full document shell.
- Dùng custom semantic tags (`doc-meta`, `doc-title`, `doc-description`) khi repo hỗ trợ.
- Không inline script/style, event handler, framework attributes, id tự sinh, class rỗng.

### Chất Lượng Diff

- Giữ diff nhỏ, có chủ đích. Không rewrite hàng loạt chỉ đổi style.
- Sau khi edit, đọc lại diff bắt link sai, stale statement, duplicate section.
- Chạy `npm run lint:doc:fix` khi repo có script (lint fix đã bao gồm format).
- Khi repo có CLI `ns-workspace`, chạy `kb validate` để xác nhận docs còn OKF-conformant (mọi doc có frontmatter + `type`).

## Quy Trình

1. `git status --short` + `rg --files ./docs` để định vị docs hiện có.
2. Đọc `docs/_sync.md` để lấy synced commit. Nếu không có, xem docs là chưa sync.
3. So sánh sync-state commit với target commit bằng `git log --oneline`, `git diff --name-status`, `git diff`.
4. Nếu nhiều commit, duyệt theo thứ tự thời gian qua `git rev-list --reverse` để hiểu final behavior.
5. Đọc docs/specs bị chạm + code path đã đổi vừa đủ hiểu final behavior. Dùng `lsp-code-graph` khi cần code graph context.
6. Quyết định tập docs nhỏ nhất cần cập nhật. Tránh rewrite rộng.
7. Cập nhật docs mô tả thiết kế hiện tại. Xóa statement stale.
8. Cập nhật `_index.md` khi thêm/move/xóa docs. Duy trì link hai chiều.
9. Cập nhật `docs/_sync.md` như snapshot sync cuối cùng. Xem `_shared/templates/sync-state-template.md`.
10. Chạy formatter + lint docs khi repo có script.

## Templates

Khi tạo docs mới, dùng templates trong `_shared/templates/`:

- `spec-template.md` cho specs/plans
- `requirements-template.md` cho requirements.md (optional)
- `module-template.md` cho module docs
- `frontmatter-schema.md` cho metadata
- `sync-state-template.md` cho \_sync.md

## Phản Hồi Cuối

Báo cáo docs đã thay đổi, kết quả sync state và validation đã chạy. Giữ cô đọng.
