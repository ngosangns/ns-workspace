---
name: read-search-docs
description: Đọc và tìm kiếm trong knowledge base (`docs/`) mà không sửa file. Trigger: tìm tài liệu, search specs, giải thích docs.
---

# Đọc Và Tìm Kiếm Tài Liệu

Dùng cho công việc chỉ đọc trên knowledge base. Không sửa docs; dùng `update-docs` cho tạo/cập nhật.

## Nguyên Tắc

- **Đọc sâu trước:** Nghiên cứu codebase và docs/specs để hiểu bối cảnh. Chỉ hỏi user khi đã tự tìm hiểu mà vẫn không giải đáp.
- **Ưu tiên docs trước code** cho architecture, feature behavior, module relations, spec questions. Fallback sang code khi docs thiếu/stale/mơ hồ.
- **Layout flat:** Knowledge base dùng cây `docs/` phẳng (không bắt buộc `docs/business` + `docs/developer`). Audience thể hiện trong nội dung/frontmatter khi cần, không qua thư mục bắt buộc.

## Phạm Vi

`docs/`, bao gồm:

- `docs/README.md`, `docs/_index.md`, `docs/_sync.md`
- `docs/architecture/` — overview, decisions
- `docs/modules/` — module boundary, API, invariants
- `docs/features/` — behavior shipped, user-facing workflows
- `docs/specs/planning/` — design plans đang active
- `docs/shared/` — glossary, domain vocabulary
- `docs/development/conventions/`
- `docs/research/`, `docs/working-documents/`

Đọc `AGENTS.md` hoặc `presets/agents/AGENTS.md` trước.

## Quy Trình

1. `rg --files docs` → đọc `docs/README.md`, `docs/_index.md`, `docs/_sync.md`.
2. Kiểm tra sync state: trích commit từ `_sync.md`, so `git rev-parse HEAD`. Nếu behind → docs là bối cảnh.
3. Search hẹp: `rg -n "<keyword>" docs/<folder>` theo mục tiêu (modules, features, specs, …).
4. Theo Markdown link thật đến file liên quan.
5. Inspect code path vừa đủ để verify nếu docs reference code.
6. Dùng `lsp-code-graph` khi cần symbol/caller/callee context.
7. Trả lời kèm file references, nói rõ dựa trên docs/code/suy luận.

## Mẫu Tìm Kiếm

| Mục tiêu              | Bắt đầu từ                                              |
| --------------------- | ------------------------------------------------------- |
| Feature plan          | `docs/specs/planning/` → `docs/features/`               |
| Behavior đã implement | `docs/features/` → `docs/modules/` → code               |
| Architecture          | `docs/architecture/`                                    |
| Module boundary       | `docs/modules/`                                         |
| Thuật ngữ             | `docs/shared/`                                          |
| Investigation         | `docs/research/`                                        |
| Working document      | `docs/working-documents/`                               |

## Ràng Buộc

- Không tạo, sửa, move, xóa file.
- Không cập nhật `_sync.md`.
- Docs stale → nói rõ trước khi dựa vào.
- User yêu cầu cập nhật docs → chuyển `update-docs`.
