---
name: sound-like-your-posts
description: Dựng voice profile từ bài viết cũ của user rồi viết/sửa nội dung mới khớp giọng đó. Trigger: viết giống giọng tôi, match voice, dựa theo bài cũ của tôi.
---

# Sound Like Your Posts

Khớp giọng người viết dựa trên mẫu thật, không dựa trên mô tả kiểu "giọng thân thiện, chuyên nghiệp". Chạy trước `writer` khi có mẫu; chạy sau `humanizer` khi cần chỉnh lại giọng bản đã làm sạch.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`. Dấu hiệu cần tránh: `_shared/AI-TELLS.md`.

## Kết Quả

- **Voice profile** — file `docs/working-documents/voice-profile-<tên>.md`, tái sử dụng cho lần sau.
- Nội dung mới hoặc bản đã chỉnh theo profile.
- Bảng đối chiếu: chỗ nào lệch profile và đã chỉnh thế nào.

## Voice Profile Gồm

Mỗi mục phải trích được ví dụ thật từ mẫu. Không có ví dụ → không đưa vào profile.

1. **Độ dài** — số từ trung bình mỗi câu, câu ngắn nhất, đoạn dài nhất.
2. **Mở bài** — 5 câu mở đầu nguyên văn từ 5 bài. Đây là mẫu quan trọng nhất.
3. **Kết bài** — 5 câu kết nguyên văn. Người viết chốt bằng gì: câu hỏi, ví dụ, bỏ ngỏ?
4. **Từ hay dùng** — từ và cụm lặp lại ở nhiều bài, kể cả tật ngôn ngữ.
5. **Từ không bao giờ dùng** — từ phổ biến mà mẫu không có lần nào.
6. **Dấu câu** — có dùng em dash không, ngoặc đơn, ba chấm, viết hoa nhấn mạnh, emoji.
7. **Xưng hô** — tôi/mình/bọn mình, bạn/anh chị/các bạn. Trích nguyên văn.
8. **Cách vào luận điểm** — kể chuyện trước, hay chốt trước rồi giải thích?
9. **Cách dùng bằng chứng** — số liệu, ví dụ cá nhân, hay khẳng định trần?
10. **Tật riêng** — thứ chỉ người này làm: câu cụt, chêm tiếng Anh, xuống dòng giữa ý.

## Workflow

1. Xin mẫu. Cần **tối thiểu 5 bài**, cùng loại nội dung với bài sắp viết. Ít hơn → nói rõ profile sẽ yếu và hỏi user có muốn tiếp không.
2. Đọc hết mẫu, trích ví dụ nguyên văn cho từng mục ở trên. Viết profile ra file.
3. Viết nội dung mới (hoặc chỉnh bản có sẵn) bám profile. Mục 2, 3, 5, 7, 10 là ưu tiên cao nhất — mở bài, kết bài và tật riêng là thứ lộ giọng rõ nhất.
4. Đối chiếu: đặt bản mới cạnh một bài mẫu. Chỗ nào đọc ra khác người → chỉnh.
5. Chạy `auto-block-banned-words`. Từ cấm mà user thật sự hay dùng (có trong mục 4) → giữ và ghi chú vào bảng.

## Ràng Buộc

- Không suy voice từ mô tả của user. Chỉ suy từ văn bản mẫu.
- Không bịa tật ngôn ngữ hay giai thoại cá nhân không có trong mẫu.
- Không nhại tới mức chép lại nguyên câu từ bài cũ.
- Profile mâu thuẫn với `_shared/AI-TELLS.md` → profile thắng, và ghi chú lý do.
