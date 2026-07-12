---
name: research
description: Phân tích, nghiên cứu và làm rõ yêu cầu trước khi triển khai. Dùng cho bước đầu khi ý định mơ hồ hoặc phụ thuộc docs/specs.
---

# Nghiên Cứu Trước

Dùng trước khi lập plan hoặc triển khai thay đổi. Đọc trước, hỏi sau, kết luận dựa trên file thật.

## Mục Tiêu

- Hiểu ý định user, behavior hiện tại, docs/specs liên quan.
- Tìm nguyên nhân gốc rễ, không dừng ở triệu chứng.
- Đủ rộng để thấy boundary/rủi ro; đủ hẹp để xác định đúng file/module.
- Ưu tiên tự nghiên cứu thay vì hỏi ngay.

## Quy Trình

1. Search bằng `rg` và `rg --files`. Dùng `lsp-code-graph` khi cần symbol/caller/callee context.
2. Dùng `read-search-docs` khi đọc/tìm kiếm trong docs/specs.
3. Nếu ý định mơ hồ, đọc specs trước:
   - `docs/business/specs/` để hiểu yêu cầu nghiệp vụ, user stories, acceptance criteria.
   - `docs/developer/specs/` để hiểu thiết kế kỹ thuật, planning notes.
     Suy luận từ requirements/planning notes.
4. Xác định nguyên nhân gốc rễ bằng cách đối chiếu docs, code path, call site, data I/O.
5. Tóm tắt bức tranh tổng quan + phạm vi tập trung + boundary + rủi ro + ngoài scope.
6. Docs stale → nêu rõ rủi ro, coi là bối cảnh.
7. Chỉ hỏi user sau khi đã đọc mà vẫn còn câu hỏi cụ thể.
8. Task lớn → chuyển `plan`. Task nhỏ rõ → chuyển `execution`.

## Đầu Ra

- Tóm tắt bối cảnh liên quan.
- Nguyên nhân gốc rễ hoặc giả thuyết đã kiểm chứng.
- Phạm vi tập trung + boundary quan trọng.
- Câu hỏi thật sự đang chặn (nếu có).

## Ràng Buộc

- Không sửa file.
- Không chạy build chỉ để hoàn tất nghiên cứu.
