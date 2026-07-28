---
name: fact-checker
description: "Bóc tách và kiểm chứng từng khẳng định trong bản thảo, đánh dấu cái nào có nguồn, cái nào không. Trigger: kiểm chứng, fact check, xác minh số liệu, verify claims."
---

# Fact Checker

Dùng trước khi publish bất cứ nội dung nào có số liệu, trích dẫn, tên riêng hoặc khẳng định về sản phẩm/đối thủ.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`.

## Kết Quả

Bảng claim, mỗi dòng gồm: trích nguyên văn · vị trí (dòng/đoạn) · phân loại · bằng chứng · đề xuất.

Phân loại chỉ có 4 giá trị:

- `ĐÚNG` — có nguồn kiểm chứng được, ghi rõ nguồn.
- `SAI` — nguồn mâu thuẫn, ghi rõ số/nội dung đúng.
- `KHÔNG KIỂM CHỨNG ĐƯỢC` — không tìm thấy nguồn đủ tin cậy.
- `CẦN NGUỒN NỘI BỘ` — chỉ user/repo mới xác nhận được (doanh thu, roadmap, chuyện nội bộ).

## Workflow

1. Bóc tách claim. Quét từng câu, trích ra mọi khẳng định thuộc các loại:
   - Số liệu, phần trăm, mốc thời gian, giá.
   - Trích dẫn và quy kết phát ngôn.
   - Tên người, tổ chức, sản phẩm, phiên bản.
   - So sánh ("nhanh hơn X", "đầu tiên", "duy nhất").
   - Khẳng định nhân quả ("A khiến B tăng").
2. Với claim về code/hệ thống trong repo: kiểm bằng `rg`, đọc file, chạy lệnh đọc. Trích `file:line` làm bằng chứng.
3. Với claim ngoài repo: tra web. Ưu tiên nguồn gốc (tài liệu chính chủ, công bố, số liệu gốc) hơn bài viết lại. Ghi URL.
4. Soi riêng nhóm quy kết mơ hồ — "các chuyên gia cho rằng", "nghiên cứu chỉ ra". Không truy ra được nguồn cụ thể → `KHÔNG KIỂM CHỨNG ĐƯỢC`.
5. Với mỗi dòng `SAI` / `KHÔNG KIỂM CHỨNG ĐƯỢC`, đề xuất một trong ba: thay bằng số đúng, hạ giọng khẳng định, hoặc xóa.
6. Báo cáo. **Không tự sửa bản thảo** trừ khi user yêu cầu rõ.

## Ràng Buộc

- Không đánh `ĐÚNG` dựa trên trí nhớ. Không có nguồn dẫn được → `KHÔNG KIỂM CHỨNG ĐƯỢC`.
- Không suy ra claim mà tác giả không viết.
- Một nguồn duy nhất viết lại từ nguồn khác không đủ để kết luận `ĐÚNG`.
