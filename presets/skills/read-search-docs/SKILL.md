---
name: read-search-docs
description: Đọc và tìm kiếm trong knowledge base của repo (`docs/`, `docs/specs/`, `docs/features/`, `docs/modules/`, `docs/architecture/`, `docs/shared/`, `docs/research/`, `docs/learnings/`, `docs/compliance/`) để trả lời câu hỏi mà không sửa file. Trigger: tìm tài liệu, search specs, giải thích docs, kiểm tra trạng thái sync, định vị tham chiếu kiến trúc/module.
---

# Đọc Và Tìm Kiếm Tài Liệu

Dùng skill này cho các công việc chỉ đọc trên knowledge base của dự án. Không sửa docs bằng skill này; dùng `update-docs` cho mọi việc tạo, cập nhật hoặc đồng bộ. Giọng làm việc của skill này là kỹ, rõ nguồn, không phỏng đoán quá tay.

## Nguyên Tắc Bắt Buộc

- **Đọc sâu và làm rõ ý định:** Phải nghiên cứu đủ vào codebase và docs/specs để hiểu bối cảnh. Nếu ý muốn của user chưa rõ, **bắt buộc đọc và tham khảo các file trong `docs/specs/` trước tiên**. Chỉ khi đã tự tìm hiểu qua specs mà vẫn không thể tự giải đáp, mới liệt kê câu hỏi cụ thể để hỏi lại user.

## Phạm Vi

- Đọc và tìm kiếm trong `docs/`, `docs/specs/`, `docs/features/`, `docs/architecture/`, `docs/modules/`, `docs/shared/`, `docs/research/`, `docs/learnings/`, và `docs/compliance/`.
- Ưu tiên hướng dẫn của chính repo trước: đọc `AGENTS.md` hoặc `presets/agents/AGENTS.md` khi có.
- Ưu tiên docs trước code đối với architecture, hành vi feature, quan hệ module, phạm vi dự án và câu hỏi về spec.
- Chỉ fallback sang code khi docs bị thiếu, stale, mơ hồ hoặc mâu thuẫn với implementation.
- Khi cần code graph context, dùng skill `lsp-code-graph` để chạy Graph Query CLI; cú pháp đúng là từ checkout local của `ns-workspace`, không dùng `@latest`:

  ```sh
  cd /Users/ngosangns/Github/ns-workspace
  go run . graph --project /path/to/project --query "<symbol-or-concept>" --json
  ```

  `graph --query` tự ensure/cài language server còn thiếu theo mặc định vào cache user của `ns-workspace`; chỉ dùng `--no-ensure-lsp` khi workflow bắt buộc read-only hoặc cần cấm network/install side effect. Đọc `warnings` trước. Nếu install/prerequisite/relation expansion fail hoặc Code Graph không đủ kết quả, nói rõ fallback sang `rg`/code inspection.

## Quy Trình

1. Kiểm tra docs hiện có:
   - `rg --files docs`
   - Đọc `docs/README.md`, `docs/_index.md`, `docs/overview.md`, và `docs/_sync.md` khi có.
2. Kiểm tra trạng thái sync trước khi tin docs:
   - Trích xuất commit/HEAD đã sync từ `docs/_sync.md`.
   - So sánh với `git rev-parse HEAD`.
   - Nếu docs đang behind hoặc thiếu sync state, nói rõ điều đó và xem docs là bối cảnh thay vì chân lý tuyệt đối.
3. Tìm kiếm hẹp trước:
   - Dùng `rg -n "<keyword>" docs`.
   - Dùng filter folder theo ý định: `docs/specs` cho hành vi dự kiến, `docs/features` cho hành vi đã shipped, `docs/modules` cho thiết kế module, `docs/architecture` cho boundary và pattern hệ thống.
4. Theo các Markdown link thật đến file `.md` liên quan. Khi đã tìm được doc liên quan, ưu tiên docs được link hơn là search rộng.
5. Nếu docs reference code paths, chỉ inspect các code path đó vừa đủ để verify hoặc làm rõ.
6. Với code path có quan hệ symbol/call phức tạp, dùng skill `lsp-code-graph` (chạy `cd /Users/ngosangns/Github/ns-workspace && go run . graph --project <repo> --query "<symbol-or-concept>" --json`) và ưu tiên `panels.codeGraph` để kiểm tra symbol, caller/callee hoặc references trước khi kết luận; dùng `panels.docsGraph` khi cần quan hệ tài liệu/spec/module.
7. Trả lời kèm file references và nói rõ câu trả lời dựa trên docs, LSP Code Graph, code, hay suy luận từ các nguồn đó.

## Mẫu Tìm Kiếm

- Với feature plan: search `docs/specs` trước, rồi `docs/features`.
- Với hành vi đã implement: search `docs/features`, rồi `docs/modules`, rồi code.
- Với quyết định kiến trúc: search `docs/architecture/decisions` và `docs/architecture/patterns`.
- Với thuật ngữ hoặc model dùng chung: search `docs/shared`.
- Với investigation hoặc câu hỏi chưa giải quyết: search `docs/research`.
- Với lesson learned từ công việc trước: search `docs/learnings`.

## Ràng Buộc

- Không tạo, sửa, move hoặc xóa file.
- Không cập nhật `docs/_sync.md`.
- Không xem lịch sử commit là nội dung docs.
- Nếu docs stale so với HEAD, nói rõ trước khi dựa vào chúng.
- Nếu user yêu cầu cập nhật docs sau khi đọc/search, chuyển sang `update-docs`.
