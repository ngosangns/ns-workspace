---
name: init
description: "Khởi tạo hiểu biết về repo mới: quét codebase, lập aspect inventory markdown cho người mới. Không ghi vào `docs/`."
---

# Init Aspect Inventory

Dùng khi user muốn khởi tạo hoặc làm mới hiểu biết về repo từ gần như không biết gì.

## Kết Quả

- Aspect inventory markdown (trình bày trong hội thoại, hoặc file path user chỉ định ngoài `docs/`).
- Không sửa source code trừ khi user yêu cầu rõ.
- Không tạo/cập nhật thư mục `docs/`.

## Workflow

1. **Scan codebase:**
   - Đọc `AGENTS.md`, README root nếu có.
   - `rg --files` quét entrypoints, commands/API, packages, config, data model, tests, scripts, integration boundaries.
   - `lsp-code-graph` khi cần symbol/caller/callee context.

2. **Aspect inventory:** Mỗi aspect cần: tên, lý do quan trọng, source paths, khoảng trống hiểu biết, priority. Bao phủ tối thiểu:
   - Domain purpose, user workflows, business rules, acceptance criteria, vocabulary.
   - Module boundaries, public API, data models, integrations, invariants, failure modes, security, dev workflow, generated artifacts, architecture decisions, conventions.

3. **Review:** Đối chiếu inventory với code đã quét; đảm bảo aspect P0/P1 có source path rõ. Trình bày inventory cho user.

## Nguyên Tắc

- Code-first: source code là nguồn chân lý; chỉ đọc để hiểu, không sửa.
- Không tạo placeholder rỗng. Mỗi aspect phải giúp người mới hiểu phần cụ thể của hệ thống.
- Không đọc/ghi thư mục `docs/`.
