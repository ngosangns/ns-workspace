---
name: init
description: Khởi tạo knowledge base cho repo mới: quét codebase, lập aspect inventory, rồi cập nhật docs/specs.
---

# Init Knowledge Base

Dùng khi user muốn khởi tạo hoặc làm mới knowledge base từ gần như không biết gì.

## Kết Quả

- Aspect inventory markdown trong `docs/specs/planning/` hoặc `docs/research/`.
- Docs được tạo/cập nhật từ inventory: `docs/README.md`, `docs/_index.md`, `docs/architecture/`, `docs/modules/`, `docs/features/`, `docs/shared/`, `docs/_sync.md`.
- Không sửa source code trừ khi user yêu cầu rõ.

## Workflow

1. **Search (`//rp`):**
   - `read-search-docs`: đọc `AGENTS.md`, README, docs index/sync, specs hiện có.
   - `rg --files` quét entrypoints, commands/API, packages, config, data model, tests, scripts, integration boundaries.
   - `lsp-code-graph` khi cần symbol/caller/callee context.
   - `plan` tạo aspect inventory markdown.

2. **Aspect inventory:** Mỗi aspect cần: tên, lý do quan trọng, source paths, docs hiện có, khoảng trống, doc target, priority. Bao phủ tối thiểu: domain purpose, user workflows, public API, module boundaries, data models, integrations, vocabulary, invariants, failure modes, security, dev workflow, generated artifacts, docs gaps.

3. **Update docs (`//ru`):**
   - `read-search-docs` đọc lại inventory.
   - `update-docs` cập nhật docs nhỏ nhất đủ mô tả hiện tại.
   - Tạo link tương đối thật, cập nhật `_index.md` và `_sync.md`.

4. **Review:** Đối chiếu docs với inventory, đảm bảo aspect P0/P1 có doc target. Chạy validation docs hoặc `git diff --check`.

## Nguyên Tắc

- Docs-first. Source code chỉ đọc để hiểu, không sửa.
- OKF-first: mọi doc tạo mới theo Open Knowledge Format — markdown + YAML frontmatter với `type` bắt buộc (xem `_shared/templates/frontmatter-schema.md`). Cross-link dùng dạng bundle-relative `/path/doc.md`.
- Docs stale → coi là bối cảnh, không phải chân lý.
- Không tạo placeholder rỗng. Mỗi doc phải giúp người mới hiểu aspect cụ thể.
