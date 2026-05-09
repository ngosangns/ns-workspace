---
name: research
description: Phân tích, nghiên cứu và làm rõ yêu cầu của user trước khi triển khai. Dùng cho bước đầu của công việc code hoặc tính năng, nhất là khi ý định còn mơ hồ hoặc câu trả lời phụ thuộc vào docs/specs và bối cảnh codebase.
---

# Nghiên Cứu Trước

Dùng skill này trước khi lập plan hoặc triển khai các thay đổi code, tính năng, refactor, sửa lỗi hoặc yêu cầu nhạy cảm về kiến trúc. Giọng làm việc của skill này là tò mò có kỷ luật: đọc trước, hỏi sau, kết luận dựa trên file thật.

## Mục Tiêu

- Hiểu ý định của user, hành vi hiện tại của codebase và docs/specs liên quan trước khi quyết định làm gì.
- Ưu tiên tự nghiên cứu thay vì hỏi user ngay lập tức.
- Đảm bảo specs phù hợp kỳ vọng với `HEAD` trước khi dựa vào chúng cho các task phụ thuộc specs.

## Quy Trình

1. Nghiên cứu codebase cẩn thận bằng công cụ tìm kiếm local nhanh như `rg` và `rg --files`.
2. Khi đọc, tìm kiếm, giải thích hoặc trả lời từ docs/specs, dùng `read-search-docs` tại `presets/skills/read-search-docs/SKILL.md`.
3. Nếu ý định của user mơ hồ, đọc các file liên quan trong `docs/specs/` trước và suy luận ý định khả dĩ từ requirements, module docs và planning notes hiện có.
4. Nếu docs/specs stale so với `HEAD`, nêu rõ rủi ro và xem chúng là bối cảnh thay vì chân lý tuyệt đối.
5. Chỉ hỏi user sau khi đã đọc docs/specs và code mà vẫn còn câu hỏi cụ thể chưa giải đáp.
6. Với task lớn hoặc phức tạp, chuyển sang `plan` trước khi sửa code.
7. Với task nhỏ và rõ ràng, đi thẳng sang skill `execution` sau khi đã gom đủ bối cảnh.

## Đầu Ra

- Tóm tắt ngắn gọn bối cảnh liên quan.
- Chỉ liệt kê các câu hỏi thật sự đang chặn tiến độ an toàn.
- Tránh suy đoán rộng; kết luận phải dựa trên file, specs hoặc code.

## Ràng Buộc

- Không sửa file trong bước này.
- Không dùng browser tools.
- Không chạy build chỉ để hoàn tất bước nghiên cứu.
