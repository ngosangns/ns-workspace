---
name: humanizer
description: Viết lại nội dung để loại bỏ dấu hiệu AI qua 3 pass — từ ngữ, cấu trúc, nhịp. Trigger: humanize, làm cho tự nhiên, nghe bớt AI, viết lại cho giống người.
---

# Humanizer

Pass đầy đủ để gỡ dấu hiệu AI khỏi một bản thảo. Bao gồm cả `ban-the-ai-words` và `ban-the-ai-patterns`; chạy skill này khi cần cả ba tầng, chạy skill lẻ khi chỉ cần một tầng.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`. Danh sách dấu hiệu: `_shared/AI-TELLS.md` — đọc trước khi sửa.

## Kết Quả

- Bản đã viết lại.
- Bảng đối chiếu trước/sau cho mọi thay đổi từ vựng và cấu trúc.
- Số liệu nhịp: số từ, độ dài câu ngắn nhất/dài nhất, số em dash.

## Workflow

Chạy đúng thứ tự. Không trộn ba pass.

1. **Pass 1 — Từ ngữ.** Áp `_shared/AI-TELLS.md` Nhóm 1 và Nhóm 2. Mỗi hit thay bằng từ cụ thể hơn, hoặc xóa. Không thay bằng từ đồng nghĩa cũng trừu tượng.
2. **Pass 2 — Cấu trúc.** Áp Nhóm 3. Bỏ so sánh phủ định, bộ ba, câu chốt tuyên ngôn, cấu trúc gương, vế "-ing" bám đuôi, hỏi-rồi-tự-trả-lời. Siết em dash về ≤ 1 / 300 từ.
3. **Pass 3 — Nhịp và chất người.** Áp Nhóm 4:
   - Phá đều nhịp câu. Chèn câu ngắn dưới 6 từ.
   - Dùng dạng rút gọn, khẩu ngữ nếu ngữ cảnh cho phép.
   - Để một ý bỏ ngỏ nếu bản gốc chốt mọi thứ.
   - Làm lộ quan điểm người viết ít nhất một lần.
   - Cho phép đoạn 1 câu.
4. Đọc to bản cuối (đọc tuần tự từng câu). Câu nào vấp → viết lại.
5. Chạy `auto-block-banned-words`. Còn hit → quay lại pass tương ứng.

## Ràng Buộc

- Không đổi nghĩa, không đổi kết luận, không bỏ dữ kiện. Đây là viết lại văn phong, không phải biên tập nội dung.
- Không thêm lỗi chính tả, lỗi ngữ pháp hoặc chi tiết cá nhân bịa ra để "giống người hơn".
- Không thay bằng cụm sáo rỗng khác — kết quả phải cụ thể hơn bản gốc.
- Bản gốc vốn đã sạch → nói thẳng là không cần sửa, không sửa cho có.
