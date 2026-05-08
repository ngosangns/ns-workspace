---
name: read-search-docs
description: Đọc, tìm kiếm và trả lời dựa trên docs/specs của dự án mà không sửa file. Dùng khi user yêu cầu tìm docs, search specs, giải thích tài liệu hiện tại, định vị reference architecture/module/spec, kiểm tra sync state hoặc trả lời câu hỏi cần dựa trên knowledge base của repo.
---

# Đọc Và Tìm Kiếm Docs

Dùng skill này cho các công việc chỉ đọc trên knowledge base của dự án. Không sửa docs bằng skill này; dùng `update-docs` cho mọi việc tạo/cập nhật/đồng bộ.

## Nguyên Tắc Bắt Buộc

- **Research sâu và làm rõ ý định:** Phải luôn research sâu vào codebase và docs/specs để hiểu rõ bối cảnh. Nếu ý muốn của user chưa rõ ràng, **BẮT BUỘC phải đọc và tham khảo các file trong `docs/specs/` trước tiên**. Chỉ khi đã tự tìm hiểu qua specs mà vẫn không thể tự giải đáp, mới được liệt kê các câu hỏi cụ thể để hỏi lại user.

## Phạm Vi

- Đọc và tìm kiếm trong `docs/`, `docs/specs/`, `docs/features/`, `docs/architecture/`, `docs/modules/`, `docs/shared/`, `docs/research/`, `docs/learnings/`, và `docs/compliance/`.
- Ưu tiên guidance của chính repo trước: đọc `AGENTS.md` hoặc `presets/agents/AGENTS.md` khi có.
- Ưu tiên docs trước code đối với architecture, hành vi feature, quan hệ module, phạm vi dự án và câu hỏi về spec.
- Chỉ fallback sang code khi docs bị thiếu, stale, mơ hồ hoặc mâu thuẫn với implementation.

## Quy Trình

1. Kiểm tra docs hiện có:
   - `rg --files docs`
   - Đọc `docs/README.md`, `docs/_index.md`, `docs/overview.md`, và `docs/_sync.md` khi có.
2. Kiểm tra sync state trước khi tin docs:
   - Trích xuất synced commit/HEAD từ `docs/_sync.md`.
   - So sánh với `git rev-parse HEAD`.
   - Nếu docs đang behind hoặc thiếu sync state, nói rõ điều đó và xem docs là bối cảnh thay vì chân lý tuyệt đối.
3. Tìm kiếm hẹp trước:
   - Dùng `rg -n "<keyword>" docs`.
   - Dùng filter folder theo ý định: `docs/specs` cho hành vi dự kiến, `docs/features` cho hành vi đã shipped, `docs/modules` cho thiết kế module, `docs/architecture` cho boundary và pattern hệ thống.
4. Theo các Markdown link thật đến file `.md` liên quan. Khi đã tìm được doc liên quan, ưu tiên docs được link hơn là search rộng.
5. Nếu docs reference code paths, chỉ inspect các code path đó vừa đủ để verify hoặc làm rõ.
6. Trả lời kèm file references và nói rõ câu trả lời dựa trên docs, code, hay suy luận từ cả hai.

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
- Không xem commit history là nội dung docs.
- Nếu docs stale so với HEAD, nói rõ trước khi dựa vào chúng.
- Nếu user yêu cầu cập nhật docs sau khi đọc/search, chuyển sang `update-docs`.
