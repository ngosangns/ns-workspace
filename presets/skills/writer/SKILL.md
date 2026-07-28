---
name: writer
description: "Viết bản thảo mới (bài đăng, email, docs, landing copy) từ brief, tránh sẵn dấu hiệu văn phong AI. Trigger: viết bài, soạn nội dung, draft post, viết email."
---

# Writer

Dùng khi cần tạo nội dung mới từ đầu. Không dùng để sửa bản thảo đã có — đó là việc của `editor`.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`. Dấu hiệu cần tránh: đọc `_shared/AI-TELLS.md` **trước khi viết câu đầu tiên**.

## Kết Quả

- Bản thảo hoàn chỉnh, đúng định dạng và độ dài yêu cầu.
- Danh sách giả định đã dùng khi brief thiếu thông tin.
- Ghi chú chỗ cần user bổ sung dữ kiện thật (số liệu, tên, ví dụ cá nhân).

## Workflow

1. Chốt brief trước khi viết. Thiếu bất kỳ mục nào thì hỏi, không tự đoán:
   - Người đọc là ai, họ đã biết gì.
   - Một việc duy nhất muốn họ làm/hiểu sau khi đọc.
   - Định dạng và độ dài.
   - Giọng: có tài liệu mẫu của user không (nếu có → chuyển sang `sound-like-your-posts` để dựng voice profile trước).
2. Viết dàn ý dạng câu khẳng định, mỗi mục là một luận điểm — không phải nhãn chủ đề.
3. Viết bản thảo. Trong lúc viết, áp dụng `_shared/AI-TELLS.md`:
   - Không dùng đoạn dẫn nhập. Câu đầu tiên đã là nội dung.
   - Mỗi luận điểm phải kèm bằng chứng cụ thể: số, ví dụ, hệ quả nếu sai.
   - Nêu rõ quan điểm ít nhất một lần.
   - Kết thúc ở thông tin cuối cùng, không có câu chốt tuyên ngôn.
4. Chạy `auto-block-banned-words` trên bản thảo. Còn hit → sửa rồi chạy lại.
5. Bàn giao kèm danh sách giả định và chỗ cần user điền dữ kiện thật.

## Ràng Buộc

- Không bịa số liệu, trích dẫn, tên người hoặc case study. Thiếu thì để placeholder `[CẦN SỐ LIỆU: …]`.
- Không viết dài hơn độ dài yêu cầu để "cho đủ ý".
- Không thêm phần FAQ, disclaimer, call-to-action nếu brief không yêu cầu.
