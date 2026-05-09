---
name: fix
description: Chẩn đoán và sửa bug, test fail, regression hoặc lỗi runtime theo hướng nhỏ gọn, có bằng chứng và có validation mục tiêu. Dùng khi user yêu cầu sửa lỗi cụ thể, sửa test đang fail, điều tra bug đã có triệu chứng hoặc làm ổn định hành vi hiện tại.
---

# Fix Bug Có Bằng Chứng

Dùng skill này khi nhiệm vụ chính là sửa lỗi đã có triệu chứng rõ: test fail,
lỗi runtime, regression, hành vi sai, log lỗi hoặc bug report cụ thể. Giọng làm việc của skill này là bình tĩnh và có chứng cứ: tái hiện trước, sửa đúng nguyên nhân, rồi xác minh bằng command sát lỗi nhất.

Nếu lỗi còn mơ hồ, dùng `research` trước để gom bối cảnh. Nếu fix làm đổi hành vi,
business rule, kiến trúc hoặc docs/specs liên quan, dùng `update-docs` sau khi sửa.

## Nguyên Tắc Bắt Buộc

- **Tái hiện trước:** Cố gắng tái hiện lỗi bằng test, command, log hoặc đọc code path cụ thể trước khi sửa.
- **Sửa nguyên nhân gốc:** Sửa nguyên nhân trực tiếp và phạm vi ảnh hưởng liên quan, tránh vá triệu chứng nếu còn có thể truy ra nguyên nhân.
- **Thay đổi nhỏ nhưng trọn vẹn:** Giữ diff nhỏ nhất có thể, nhưng vẫn sạch, đúng kiến trúc hiện tại và không tạo workaround khó bảo trì.
- **Chặn regression:** Khi phù hợp, thêm hoặc cập nhật test để lỗi không quay lại.
- **Validation mục tiêu:** Chạy validation sát với lỗi đã sửa; không cần full build nếu repo guidance không yêu cầu.
- **Tôn trọng worktree:** Không revert hoặc chạm vào thay đổi không liên quan của user.

## Quy Trình

1. Đọc bug report, failing output hoặc triệu chứng user đưa.
2. Kiểm tra git status để nhận diện thay đổi đang có và tránh đè việc của user.
3. Xác định code path liên quan bằng `rg`, test hiện có, docs/specs hoặc call site gần nhất.
4. Tái hiện lỗi bằng command nhỏ nhất có thể, hoặc ghi rõ nếu không tái hiện được nhưng đã có bằng chứng đủ từ code/log.
5. Sửa nguyên nhân gốc theo pattern hiện có của repo.
6. Thêm hoặc cập nhật test/regression guard nếu bug có bề mặt test hợp lý.
7. Chạy lại command tái hiện lỗi và validation mục tiêu.
8. Review diff vừa sửa, cleanup các thay đổi thừa, rồi báo ngắn gọn nguyên nhân và cách đã xác minh.

## Ràng Buộc

- Không mở rộng scope sang refactor lớn nếu không cần để fix lỗi.
- Không đổi hành vi public ngoài phần cần sửa, trừ khi bug fix bắt buộc phải đổi và cần nêu rõ.
- Không dùng browser tools trừ khi bug chỉ có thể xác minh qua UI hoặc user yêu cầu rõ.
- Không chạy build rộng chỉ để kết thúc task.
