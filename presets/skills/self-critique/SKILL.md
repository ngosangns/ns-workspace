---
name: self-critique
description: Tự phản biện và sửa lặp bản thảo đến khi đạt tiêu chí, có điều kiện dừng rõ ràng. Trigger: tự phản biện, self critique, sửa đến khi ổn, review lại chính mình.
---

# Self Critique

Vòng lặp phản biện — sửa — phản biện lại, chạy trên chính output vừa tạo. Dùng ngay trước khi bàn giao.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`. Ngưỡng đạt về văn phong: `_shared/AI-TELLS.md` mục "Ngưỡng Đạt".

## Kết Quả

- Bản cuối.
- Nhật ký từng vòng: phát hiện gì, sửa gì, còn lại gì.
- Kết luận: `ĐẠT` hoặc `DỪNG VÌ HẾT VÒNG` kèm danh sách vấn đề chưa xử lý.

## Tiêu Chí Đạt

Chốt trước khi bắt đầu vòng 1. Mặc định nếu user không nêu:

1. Không có claim sai hoặc không có bằng chứng.
2. Không có mâu thuẫn nội bộ.
3. Đạt ngưỡng của `_shared/AI-TELLS.md`.
4. Cắt được ≤ 5% số từ mà không mất thông tin.
5. Đúng brief: người đọc, định dạng, độ dài.

## Workflow

1. Chốt tiêu chí đạt và số vòng tối đa (mặc định 3).
2. Mỗi vòng:
   - **Phản biện** — đọc bản hiện tại như người phản đối nó. Với mỗi tiêu chí, tìm bằng chứng nó *không* đạt. Không tìm ra thì ghi rõ đã tìm gì.
   - **Sửa** — chỉ sửa những chỗ vừa nêu. Không sửa kèm thứ khác.
   - **Đối chiếu** — chấm lại toàn bộ tiêu chí trên bản mới. Ghi vòng này sửa được gì, làm hỏng thêm gì.
3. Dừng khi: đạt hết tiêu chí, **hoặc** hết số vòng, **hoặc** vòng mới không cải thiện gì so với vòng trước.
4. Báo kết quả kèm nhật ký. Chưa đạt → nói thẳng còn thiếu gì, không bàn giao như đã xong.

## Ràng Buộc

- Vòng phản biện phải nêu vấn đề cụ thể kèm trích dẫn. "Có thể mượt hơn" không tính là phát hiện.
- Không lặp lại phát hiện đã xử lý ở vòng trước.
- Không tự nới tiêu chí giữa chừng để kết luận `ĐẠT`.
- Sửa làm bản thảo tệ đi → quay lại bản trước và ghi rõ vào nhật ký.
