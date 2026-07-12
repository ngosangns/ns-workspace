---
name: read-search-docs
description: Đọc và tìm kiếm trong knowledge base (`docs/`) mà không sửa file. Trigger: tìm tài liệu, search specs, giải thích docs.
---

# Đọc Và Tìm Kiếm Tài Liệu

Dùng cho công việc chỉ đọc trên knowledge base. Không sửa docs; dùng `update-docs` cho tạo/cập nhật.

## Nguyên Tắc

- **Đọc sâu trước:** Nghiên cứu codebase và docs/specs để hiểu bối cảnh. Chỉ hỏi user khi đã tự tìm hiểu mà vẫn không giải đáp.
- **Ưu tiên docs trước code** cho architecture, feature behavior, module relations, spec questions. Fallback sang code khi docs thiếu/stale/mơ hồ.
- **Phân biệt audience:** Khi cần góc nhìn nghiệp vụ, tìm trong `docs/business/`. Khi cần góc nhìn kỹ thuật, tìm trong `docs/developer/`.

## Phạm Vi

`docs/`, bao gồm:

- `docs/business/features/` — user-facing behavior, requirements, acceptance criteria
- `docs/business/modules/` — business view của module (contract, business rules)
- `docs/business/specs/planning/` — business specs, user stories
- `docs/business/shared/` — glossary, domain vocabulary
- `docs/business/research/`
- `docs/developer/architecture/` — architecture overview, decisions, patterns
- `docs/developer/modules/` — technical module docs (boundary, API, dependencies)
- `docs/developer/features/` — technical implementation notes cho features
- `docs/developer/specs/planning/` — technical design specs
- `docs/developer/development/conventions/`
- `docs/developer/research/`, `docs/developer/learnings/`, `docs/developer/compliance/`

Đọc `AGENTS.md` hoặc `presets/agents/AGENTS.md` trước.

## Quy Trình

1. `rg --files docs` → đọc `docs/README.md`, `docs/_index.md`, `docs/_sync.md`, `docs/business/README.md`, `docs/developer/README.md`, `docs/business/_index.md`, `docs/developer/_index.md`.
2. Kiểm tra sync state: trích commit từ `_sync.md`, so `git rev-parse HEAD`. Nếu behind → docs là bối cảnh.
3. Search hẹp: `rg -n "<keyword>" docs/business/<folder>` hoặc `docs/developer/<folder>` tùy audience.
4. Theo Markdown link thật đến file liên quan.
5. Inspect code path vừa đủ để verify nếu docs reference code.
6. Dùng `lsp-code-graph` khi cần symbol/caller/callee context.
7. Trả lờikèm file references, nói rõ dựa trên docs/code/suy luận và audience của doc.

## Mẫu Tìm Kiếm

| Mục tiêu                  | Audience  | Bắt đầu từ                                                    |
| ------------------------- | --------- | ------------------------------------------------------------- |
| Feature plan              | business  | `docs/business/specs/planning/` → `docs/business/features/`   |
| Feature plan              | developer | `docs/developer/specs/planning/` → `docs/developer/features/` |
| Behavior đã implement     | business  | `docs/business/features/`                                     |
| Behavior đã implement     | developer | `docs/developer/features/` → `docs/developer/modules/` → code |
| Architecture decision     | developer | `docs/developer/architecture/decisions/` + `patterns/`        |
| Module boundary kỹ thuật  | developer | `docs/developer/modules/`                                     |
| Module boundary nghiệp vụ | business  | `docs/business/modules/`                                      |
| Thuật ngữ chung           | business  | `docs/business/shared/`                                       |
| Investigation             | both      | `docs/business/research/` + `docs/developer/research/`        |
| Lesson learned            | developer | `docs/developer/learnings/`                                   |

## Ràng Buộc

- Không tạo, sửa, move, xóa file.
- Không cập nhật `_sync.md`.
- Docs stale → nói rõ trước khi dựa vào.
- User yêu cầu cập nhật docs → chuyển `update-docs`.
