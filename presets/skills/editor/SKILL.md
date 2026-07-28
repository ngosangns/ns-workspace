---
name: editor
description: "Biên tập bản thảo đã có — cắt, sắp lại thứ tự, siết câu, làm rõ luận điểm. Trigger: biên tập, edit bài, rút gọn, làm gọn nội dung."
---

# Editor

Dùng khi đã có bản thảo và cần nó chặt hơn. Đây là biên tập cấu trúc và nội dung, không phải quét dấu hiệu AI — việc đó thuộc `humanizer`, `ban-the-ai-words`, `ban-the-ai-patterns`.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`.

## Kết Quả

- Bản đã biên tập.
- Bảng thay đổi: cắt gì, chuyển gì, vì sao.
- Tỷ lệ rút gọn (số từ trước → sau).

## Workflow

1. Đọc hết bản thảo trước khi sửa một chữ nào. Xác định:
   - Luận điểm chính thật sự là gì (có thể khác với câu mở đầu).
   - Người đọc là ai.
2. **Pass cắt** — bỏ trước, sửa sau:
   - Đoạn dẫn nhập và đoạn "chuẩn bị nói".
   - Câu lặp lại ý đã có ở chỗ khác.
   - Mệnh đề bổ nghĩa không đổi nghĩa khi bỏ.
   - Ví dụ thứ hai minh họa cùng một điểm.
   - Mục tiêu: cắt ≥ 20% số từ trước khi sang pass sau.
3. **Pass thứ tự** — đưa kết luận lên trước phần dẫn giải. Gom các câu cùng chủ đề đang nằm rải rác. Mỗi đoạn giữ một ý.
4. **Pass câu** — chuyển bị động sang chủ động, bỏ danh từ hóa (`việc triển khai được thực hiện` → `nhóm triển khai`), thay từ trừu tượng bằng từ cụ thể.
5. **Pass bằng chứng** — mỗi khẳng định mạnh phải có số, ví dụ hoặc nguồn. Không có → hạ giọng khẳng định hoặc đánh dấu chuyển sang `fact-checker`.
6. Đọc lại toàn bộ một lượt. Đảm bảo bản cắt vẫn đọc liền mạch, không hụt logic.

## Ràng Buộc

- Không đổi luận điểm hoặc kết luận của tác giả. Thấy luận điểm sai → báo, không tự sửa.
- Không viết lại theo giọng của mình. Giữ giọng gốc.
- Không thêm nội dung mới trừ khi cần một câu nối cho mạch đọc.
