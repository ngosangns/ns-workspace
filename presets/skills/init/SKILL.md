---
name: init
description: Khởi tạo knowledge base cho repo mới hoặc repo agent chưa hiểu rõ: quét codebase, lập aspect inventory markdown cho người mới, rồi cập nhật docs/specs dựa trên inventory đó.
---

# Init Knowledge Base

Dùng skill này khi user muốn khởi tạo hoặc làm mới knowledge base của một repo từ con số gần như không biết gì. Mục tiêu là tạo một bản đồ aspects đủ để người mới hiểu domain, project, module boundary và workflow chính, rồi dùng bản đồ đó để cập nhật docs hiện hành.

## Kết Quả Mong Đợi

- Một file markdown aspect inventory trong `docs/specs/planning/` hoặc `docs/research/`, tùy theo repo guidance và mục đích của lần init.
- Docs/specs được tạo hoặc cập nhật từ inventory đó, tập trung vào `docs/README.md`, `docs/_index.md`, `docs/architecture/`, `docs/modules/`, `docs/features/`, `docs/shared/` và `docs/_sync.md` khi cần.
- Không sửa source code trừ khi user yêu cầu rõ ràng sau bước docs init.

## Workflow

1. Chạy pha search theo pipeline `//rp`:
   - Dùng `read-search-docs` để đọc `AGENTS.md`, README, docs index/sync và các specs hiện có trước.
   - Dùng `read-search-docs` kết hợp `rg --files` để quét entrypoints, commands/API public, packages/modules, config, data model, tests, scripts, generated assets và integration boundaries.
   - Dùng `lsp-code-graph` khi cần symbol/caller/callee/reference context; nếu không đủ kết quả thì fallback sang `rg` và code inspection.
   - Dùng `plan` để chuyển kết quả search thành file plan/aspect inventory markdown.
2. Lập aspect inventory:
   - Tạo một file plan/aspect inventory markdown trong vị trí phù hợp với repo guidance và mục đích của lần init.
   - Mỗi aspect nên có: tên aspect, lý do quan trọng với người mới, source paths, docs hiện có, khoảng trống docs, doc target để tạo/cập nhật, và priority.
   - Aspect cần bao phủ tối thiểu: domain/project purpose, user workflows, public commands/API, module boundaries, data/config models, external integrations, domain vocabulary, invariants/business rules, failure modes, security/compliance constraints, dev/test/build workflow, generated artifacts và docs gaps.
3. Dùng inventory làm đầu vào cho pipeline `//ru`:
   - Dùng `read-search-docs` để đọc lại inventory như source of truth của phạm vi docs và đối chiếu docs liên quan.
   - Dùng `update-docs` để cập nhật docs nhỏ nhất đủ mô tả trạng thái hiện tại, không viết changelog hay lịch sử thay đổi.
   - Tạo link tương đối thật giữa docs liên quan, cập nhật `_index.md` khi thêm/move docs, và cập nhật `_sync.md` nếu repo có sync state.
4. Review kết quả:
   - Đối chiếu docs mới với inventory để đảm bảo mọi aspect P0/P1 có doc target hoặc note rõ vì sao chưa cập nhật.
   - Chạy validation docs có sẵn nếu repo cung cấp. Nếu không có, chạy `git diff --check` trên các file đã sửa.

## Nguyên Tắc

- Init docs là workflow docs-first. Source code chỉ được đọc để hiểu hệ thống, không chỉnh sửa source.
- Nếu phát hiện docs stale, nói rõ và xem docs là bối cảnh thay vì chân lý tuyệt đối.
- Không tạo placeholder rỗng. Mỗi doc mới phải giúp người mới hiểu một aspect cụ thể.
- Nếu việc init để lộ thay đổi source/architecture lớn cần làm tiếp, dùng aspect inventory để lập plan riêng và chờ user duyệt trước khi sửa code.
