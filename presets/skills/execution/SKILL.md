---
name: execution
description: Triển khai thay đổi code đã được duyệt hoặc các task nhỏ đã rõ ràng theo hướng forward-only. Dùng cho bước execution sau research, và sau plan nếu task lớn.
---

# Thực Thi Code Không Cần Backward Compat

Dùng skill này khi đã đến bước sửa code.

Với task lớn, chỉ dùng skill này sau khi user đã duyệt plan được tạo bởi `plan`. Với task nhỏ đã rõ, có thể dùng ngay sau `research`.

## Nguyên Tắc Bắt Buộc

- **Không cần Backward Compatible:** Khi tiến hành code, refactor, **KHÔNG CẦN** đảm bảo tính tương thích ngược (backward compatible). Code mới được quyền thay đổi để đáp ứng yêu cầu một cách triệt để và kiến trúc chuẩn nhất.
- **Scope-guard:** Theo sát Git commit và bảo vệ chặt biên giới (scope) dự án.
- **Transparent-compact:** Giao tiếp thẳng thắn, báo cáo cô đọng, đi vào trọng tâm.
- **No build:** Khi thực hiện xong yêu cầu không cần build.
- **Continuous Self-Review:** Đảm bảo sau khi sửa xong cần review lại toàn bộ code và sửa; nếu có sửa gì thì sau khi sửa xong tiếp tục review lại và sửa, lặp lại cho đến khi không còn gì để sửa và mọi thứ đã tốt.

## Nguyên Tắc Thực Thi

- Triển khai hành vi được yêu cầu theo plan đã duyệt hoặc theo bối cảnh đã research đủ rõ.
- Ưu tiên kiến trúc hiện tại sạch và đúng hơn các lớp vá để giữ backward compatibility.
- Xóa hoặc thay thế hành vi lỗi thời khi thiết kế mới khiến nó không còn cần thiết.
- Giữ thay đổi đúng phạm vi yêu cầu và các module bị ảnh hưởng.
- Giữ nguyên các thay đổi không liên quan của user trong worktree.

## Quy Trình

1. Đọc lại plan đã duyệt hoặc ghi chú research liên quan trước khi sửa.
2. Kiểm tra chính xác các file cần sửa và pattern code lân cận.
3. Triển khai thay đổi theo style, helper và kiến trúc hiện có của repo.
4. Không giữ backward compatibility trừ khi user yêu cầu rõ hoặc public contract hiện tại bắt buộc phải giữ.
5. Sau mỗi lượt sửa đáng kể, review lại code đã thay đổi và sửa các vấn đề phát hiện được.
6. Lặp lại review và cleanup cho đến khi không còn vấn đề rõ ràng cần sửa.
7. Chạy validation có mục tiêu khi có sẵn và phù hợp, nhưng không chạy full build chỉ để kết thúc nếu guidance của repo nói không cần build.
8. Nếu thay đổi code ảnh hưởng đến flow, business rule, architecture, quan hệ module hoặc constraint, dùng `update-docs` để cập nhật docs/specs liên quan.

## Ràng Buộc

- Không dùng browser tools.
- Không chạy build chỉ để kết thúc task.
- Không để nội dung docs/spec cũ nằm cạnh hành vi mới; thay thế các mô tả đã lỗi thời.
- Không revert các thay đổi worktree không liên quan.
