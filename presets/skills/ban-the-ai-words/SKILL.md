---
name: ban-the-ai-words
description: Pass từ vựng — thay hoặc xóa mọi từ/cụm từ bị cấm trong danh sách AI tells, không đụng cấu trúc. Trigger: bỏ từ AI, ban AI words, thay từ sáo rỗng.
---

# Ban The AI Words

Chỉ làm tầng từ vựng. Cấu trúc câu do `ban-the-ai-patterns` xử lý; cả ba tầng do `humanizer` xử lý.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`. Danh sách cấm: `_shared/AI-TELLS.md` Nhóm 1 và Nhóm 2.

## Kết Quả

Bảng: từ bị cấm · vị trí · thay bằng gì (hoặc `XÓA`) · câu sau khi sửa. Kèm bản đã áp dụng.

## Workflow

1. Quét toàn văn theo Nhóm 1 (từ) và Nhóm 2 (câu mở/đóng). Quét cả biến thể: dạng danh từ hóa, dạng tiếng Anh xen tiếng Việt, dạng viết hoa.
2. Với mỗi hit, chọn theo thứ tự ưu tiên:
   1. **Xóa** — bỏ đi mà câu không mất thông tin. Đây là lựa chọn mặc định.
   2. **Thay bằng từ cụ thể** — từ trừu tượng đổi thành sự việc đo được. `mạnh mẽ` → `chịu được 10k request/s`.
   3. **Viết lại câu** — chỉ khi từ bị cấm là trục của câu.
3. Sau khi sửa hết, quét lại một lượt: bản sửa có sinh ra từ cấm mới không.
4. Báo cáo bảng đối chiếu.

## Ràng Buộc

- Không đổi cấu trúc câu ngoài mức tối thiểu để câu đứng được.
- Không thay bằng từ đồng nghĩa cũng trừu tượng (`liền mạch` → `mượt mà` là sai).
- Không đổi nghĩa. Không biết thay bằng gì → xóa cả mệnh đề và báo lại, không đoán số liệu.
- Từ bị cấm nằm trong trích dẫn nguyên văn hoặc tên riêng → giữ nguyên, ghi chú trong bảng.
