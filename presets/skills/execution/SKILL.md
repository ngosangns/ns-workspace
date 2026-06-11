---
name: execution
description: Triển khai thay đổi code đã được duyệt (sau `plan`) hoặc task nhỏ rõ ràng (sau `research`). Trigger: viết code, triển khai tính năng, refactor đã duyệt.
---

# Thực Thi Code

Dùng khi đã đến bước sửa code. Với task lớn, chỉ dùng sau khi user duyệt plan. Với task nhỏ đã rõ, dùng ngay sau `research`.

**Khác `fix`:** `execution` triển khai từ thiết kế rõ, có thể chạm nhiều file, được phép phá tương thích ngược nội bộ để giữ kiến trúc sạch. `fix` bắt đầu từ triệu chứng, ưu tiên diff nhỏ nhất.

Quy tắc chung: đọc `_shared/CONVENTIONS.md` (hỏi khi vướng, requirements.md, diff review loop, comment, worktree).

## Nguyên Tắc Riêng

- **Không giữ tương thích ngược vô điều kiện:** Khi code mới cần đổi contract nội bộ để đúng kiến trúc hơn, được quyền thay đổi. Chỉ giữ khi user yêu cầu rõ hoặc public contract bắt buộc.
- **Báo cáo có diễn giải:** Giải thích ý nghĩa từng nhóm thay đổi: vấn đề xử lý, requirements tuân thủ, behavior/contract đổi/giữ, rủi ro, user cần lưu ý gì.
- **Liệt kê việc còn lại:** Nếu chưa xử lý trọn vẹn, nêu rõ phần còn lại và bước tiếp theo.

## Quy Trình

1. Đọc lại plan/research. Xác định nguyên nhân gốc rễ, module boundary, contract, call site.
2. Đọc `requirements.md` của feature/module liên quan (xem CONVENTIONS.md).
3. Thu hẹp phạm vi: file cần sửa, pattern lân cận, test phù hợp, ngoài scope.
4. Triển khai theo style, helper, kiến trúc hiện có.
5. Diff review loop (xem CONVENTIONS.md).
6. Chạy validation mục tiêu khi phù hợp.
7. Nếu thay đổi ảnh hưởng flow, business rule, architecture → dùng `update-docs`.
8. Tổng kết và liệt kê việc còn lại nếu có.

## Ràng Buộc

- Không để docs/spec cũ nằm cạnh behavior mới.
- Không để lại comment tiếng Việt trong code mới hoặc code vừa chạm.
