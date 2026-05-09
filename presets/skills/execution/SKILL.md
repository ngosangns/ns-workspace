---
name: execution
description: Triển khai thay đổi code đã được duyệt hoặc các task nhỏ đã rõ theo hướng tiến thẳng. Dùng sau research, hoặc sau plan khi task lớn đã được user duyệt.
---

# Thực Thi Code Rõ Ràng

Dùng skill này khi đã đến bước sửa code.

Với task lớn, chỉ dùng skill này sau khi user đã duyệt plan được tạo bởi `plan`. Với task nhỏ đã rõ, có thể dùng ngay sau `research`. Giọng làm việc của skill này là chắc tay, thẳng thắn và gọn: làm đến nơi, giữ scope sạch, không lẩn vào refactor thừa.

## Nguyên Tắc Bắt Buộc

- **Không giữ tương thích ngược vô điều kiện:** Khi code mới cần thay đổi contract nội bộ để đúng kiến trúc hơn, được quyền thay đổi. Chỉ giữ tương thích khi user yêu cầu rõ hoặc public contract thật sự bắt buộc.
- **Giữ scope chặt:** Theo sát yêu cầu, Git diff và module boundary. Không kéo thêm việc phụ.
- **Báo cáo cô đọng:** Nói thẳng việc đã làm, vì sao làm, validation nào đã chạy.
- **Không build chỉ để kết thúc:** Không chạy build rộng nếu repo guidance không yêu cầu.
- **Tự review liên tục:** Sau mỗi lượt sửa đáng kể, đọc lại diff, cleanup phần thừa, rồi lặp lại đến khi không còn vấn đề rõ ràng.

## Nguyên Tắc Thực Thi

- Triển khai hành vi được yêu cầu theo plan đã duyệt hoặc theo bối cảnh đã research đủ rõ.
- Ưu tiên kiến trúc hiện tại sạch và đúng hơn các lớp vá để giữ tương thích ngược không cần thiết.
- Xóa hoặc thay thế hành vi lỗi thời khi thiết kế mới khiến nó không còn cần thiết.
- Giữ thay đổi đúng phạm vi yêu cầu và các module bị ảnh hưởng.
- Giữ nguyên các thay đổi không liên quan của user trong worktree.

## Quy Trình

1. Đọc lại plan đã duyệt hoặc ghi chú research liên quan trước khi sửa.
2. Kiểm tra chính xác các file cần sửa và pattern code lân cận.
3. Triển khai thay đổi theo style, helper và kiến trúc hiện có của repo.
4. Không giữ tương thích ngược trừ khi user yêu cầu rõ hoặc public contract hiện tại bắt buộc phải giữ.
5. Sau mỗi lượt sửa đáng kể, review lại code đã thay đổi và sửa các vấn đề phát hiện được.
6. Lặp lại review và cleanup cho đến khi không còn vấn đề rõ ràng cần sửa.
7. Chạy validation có mục tiêu khi có sẵn và phù hợp, nhưng không chạy full build chỉ để kết thúc nếu guidance của repo nói không cần build.
8. Nếu thay đổi code ảnh hưởng đến flow, business rule, architecture, quan hệ module hoặc constraint, dùng `update-docs` để cập nhật docs/specs liên quan.

## Ràng Buộc

- Không dùng browser tools.
- Không chạy build chỉ để kết thúc task.
- Không để nội dung docs/spec cũ nằm cạnh hành vi mới; thay thế các mô tả đã lỗi thời.
- Không revert các thay đổi worktree không liên quan.
