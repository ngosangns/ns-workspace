---
name: read-search-docs
description: Đọc và tìm kiếm trong knowledge base (`docs/`) mà không sửa file. Trigger: tìm tài liệu, search specs, giải thích docs.
---

# Đọc Và Tìm Kiếm Tài Liệu

Dùng cho công việc chỉ đọc trên knowledge base. Không sửa docs; dùng `update-docs` cho tạo/cập nhật.

## Nguyên Tắc

- **Đọc sâu trước:** Nghiên cứu codebase và docs/specs để hiểu bối cảnh. Đọc `docs/specs/` trước tiên. Chỉ hỏi user khi đã tự tìm hiểu mà vẫn không giải đáp.
- **Ưu tiên docs trước code** cho architecture, feature behavior, module relations, spec questions. Fallback sang code khi docs thiếu/stale/mơ hồ.

## Phạm Vi

`docs/`, `docs/specs/`, `docs/features/`, `docs/architecture/`, `docs/modules/`, `docs/shared/`, `docs/research/`, `docs/learnings/`, `docs/compliance/`.

Đọc `AGENTS.md` hoặc `presets/agents/AGENTS.md` trước.

## Quy Trình

1. `rg --files docs` → đọc `docs/README.md`, `docs/_index.md`, `docs/_sync.md`.
2. Kiểm tra sync state: trích commit từ `_sync.md`, so `git rev-parse HEAD`. Nếu behind → docs là bối cảnh.
3. Search hẹp: `rg -n "<keyword>" docs/<folder>`.
4. Theo Markdown link thật đến file liên quan.
5. Inspect code path vừa đủ để verify nếu docs reference code.
6. Dùng `lsp-code-graph` khi cần symbol/caller/callee context.
7. Trả lời kèm file references, nói rõ dựa trên docs/code/suy luận.

## Mẫu Tìm Kiếm

- Feature plan → `docs/specs` → `docs/features`
- Behavior đã implement → `docs/features` → `docs/modules` → code
- Architecture decision → `docs/architecture/decisions` + `patterns`
- Thuật ngữ chung → `docs/shared`
- Investigation → `docs/research`
- Lesson learned → `docs/learnings`

## Ràng Buộc

- Không tạo, sửa, move, xóa file.
- Không cập nhật `_sync.md`.
- Docs stale → nói rõ trước khi dựa vào.
- User yêu cầu cập nhật docs → chuyển `update-docs`.
