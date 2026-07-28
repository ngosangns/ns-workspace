---
name: anti-ai-style
description: "Bộ luật văn phong áp cho mọi nội dung sinh ra — áp lúc viết, không phải sửa sau. Trigger: anti-AI style, viết theo style không giống AI, áp style guide."
---

# Anti-AI Style

Đây là **hợp đồng văn phong**, không phải pass sửa bài. Load skill này khi bắt đầu một phiên viết để mọi output tuân luật ngay từ đầu. Bản thảo đã có → dùng `humanizer`.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`. Danh sách dấu hiệu: `_shared/AI-TELLS.md`.

## Luật

Áp cho mọi câu sinh ra trong phiên, kể cả câu trả lời trong chat.

1. **Câu đầu là nội dung.** Không dẫn nhập, không nhắc lại câu hỏi, không nêu sắp làm gì.
2. **Câu cuối là thông tin cuối.** Không tóm tắt lại, không chúc, không tuyên ngôn.
3. **Cụ thể trước trừu tượng.** Có số thì dùng số. Không có số thì dùng ví dụ. Không có ví dụ thì hạ giọng khẳng định.
4. **Một tính từ cho một danh từ.** Không xâu chuỗi.
5. **Không so sánh phủ định.** Nói thẳng cái đúng.
6. **Trạng từ phải kiếm được chỗ đứng.** Bỏ được mà nghĩa không đổi → bỏ.
7. **Nhịp lệch.** Không để 5 câu liên tiếp cùng độ dài. Cho phép câu 3 từ, cho phép đoạn 1 câu.
8. **Em dash ≤ 1 / 300 từ.**
9. **Có quan điểm.** Khi có cơ sở đánh giá, nói rõ tốt hay dở. Không liệt kê hai phía rồi để trống.
10. **Không biết thì nói không biết.** Không lấp bằng câu chung chung.
11. **Không đối xứng giả.** Không ép nội dung thành 3 mục, 5 bước nếu nó không có bấy nhiêu.
12. **Danh sách cấm ở `_shared/AI-TELLS.md` Nhóm 1 và Nhóm 2 là cấm cứng.**

## Tự Kiểm Trước Khi Xuất

Trước khi trả bất kỳ nội dung nào, soát 4 điểm:

- Câu đầu và câu cuối có vi phạm luật 1–2 không?
- Có từ nào trong Nhóm 1 không?
- Có so sánh phủ định hoặc bộ ba không?
- Có ít nhất một câu ngắn không?

Có vi phạm → sửa trước khi xuất, không xuất rồi xin lỗi.

## Ràng Buộc

- Luật này không được làm mất chính xác kỹ thuật. Đụng nhau thì chính xác thắng, và ghi chú lý do.
- Trích dẫn nguyên văn của người khác giữ nguyên.
- Không áp luật này lên code, log, output lệnh hoặc tài liệu có định dạng bắt buộc.
