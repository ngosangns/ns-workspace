---
name: ban-the-ai-patterns
description: Pass cấu trúc — gỡ so sánh phủ định, bộ ba, câu chốt tuyên ngôn, em dash thừa và các mẫu câu lộ AI. Trigger: bỏ cấu trúc AI, ban AI patterns, sửa mẫu câu máy.
---

# Ban The AI Patterns

Chỉ làm tầng cấu trúc câu và đoạn. Từ vựng do `ban-the-ai-words` xử lý; cả ba tầng do `humanizer` xử lý.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`. Danh sách mẫu cấm: `_shared/AI-TELLS.md` Nhóm 3.

## Kết Quả

Bảng: mẫu · vị trí · trích trước · trích sau. Kèm bản đã áp dụng và số em dash trước/sau.

## Workflow

Quét từng mẫu một, hết bản thảo rồi mới sang mẫu tiếp theo. Quét gộp sẽ sót.

1. **So sánh phủ định** — tìm `không phải … mà là`, `it's not … it's`. Giữ vế khẳng định, bỏ vế phủ định.
2. **Tuyên bố lớn ở cuối** — đọc đoạn cuối và câu cuối mỗi mục. Câu nào chỉ để gây ấn tượng, không thêm thông tin → xóa.
3. **Bộ ba** — tìm chuỗi 3 tính từ/danh từ nối bằng dấu phẩy + `và`. Giữ mục đúng nhất.
4. **Trạng từ** — quét đuôi `-ly` (EN) và `một cách …`, `đáng kể`, `hiệu quả`, `nhanh chóng` (VI). Bỏ, hoặc thay bằng số đo.
5. **Đơn giản hóa quá mức** — `hầu hết mọi người`, `ai cũng biết`, `nói chung thì`. Nêu nguồn hoặc bỏ mệnh đề.
6. **Hỏi-rồi-tự-trả-lời** — xóa câu hỏi, giữ câu trả lời.
7. **Cấu trúc gương** — hai câu liền nhau lặp cú pháp. Viết lại một trong hai.
8. **Em dash** — đếm. Vượt 1 / 300 từ → chuyển phần thừa thành dấu phẩy, ngoặc đơn hoặc câu riêng.
9. **Vế "-ing" bám đuôi** — `…, cho phép/giúp/mang lại …`. Tách câu hoặc bỏ.
10. **Quy kết mơ hồ** — `các chuyên gia`, `nghiên cứu chỉ ra`. Nêu tên nguồn hoặc bỏ.
11. **Đoạn đều nhau** — 5 đoạn liên tiếp cùng 3–4 câu → gộp hoặc tách để phá đều.
12. **Bullet đồng dạng** — mọi gạch đầu dòng đều `**Cụm in đậm:** giải thích` → đổi thể loại một phần.

## Ràng Buộc

- Không đổi từ vựng ngoài mức tối thiểu để câu đứng được sau khi đổi cấu trúc.
- Không đổi nghĩa, không bỏ dữ kiện.
- Mẫu nằm trong trích dẫn nguyên văn → giữ nguyên, ghi chú.
- Bỏ một mẫu làm câu tối nghĩa → giữ nguyên và ghi lý do vào bảng.
