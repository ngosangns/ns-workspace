---
name: red-pen
description: "Đánh dấu điểm yếu trong bản thảo mà không viết lại — chỉ ra chỗ hỏng, mức độ, lý do. Trigger: đánh dấu điểm yếu, red pen, soi bản thảo, chỉ lỗi đừng sửa."
---

# Red Pen

Dùng khi user muốn biết bản thảo hỏng ở đâu và tự quyết định cách sửa. Skill này **không viết lại**.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`. Dấu hiệu văn phong: `_shared/AI-TELLS.md`.

## Kết Quả

Danh sách đánh dấu, sắp theo mức độ giảm dần. Mỗi mục:

```
[MỨC] vị trí — trích nguyên văn
  Vấn đề: <một câu>
  Vì sao hỏng: <hệ quả với người đọc>
```

Mức độ:

- `CHẶN` — sai sự thật, mâu thuẫn nội bộ, luận điểm không đứng được. Không được publish.
- `NẶNG` — người đọc sẽ hiểu sai hoặc bỏ ngang.
- `NHẸ` — câu chữ, nhịp, dấu hiệu văn phong AI.

## Workflow

1. Đọc hết một lượt, không ghi chú. Ghi lại một câu: bài này đang cố làm gì.
2. Soi theo thứ tự, dừng ở mỗi tầng trước khi xuống tầng dưới:
   - **Luận điểm** — có luận điểm không? Có bằng chứng không? Có mâu thuẫn với đoạn khác không?
   - **Cấu trúc** — thứ tự có phục vụ người đọc không? Đoạn nào tồn tại mà không đóng góp gì?
   - **Bằng chứng** — khẳng định mạnh nào không có số/nguồn/ví dụ?
   - **Câu chữ** — đối chiếu `_shared/AI-TELLS.md`.
3. Với mỗi mục, viết "vì sao hỏng" theo góc người đọc, không theo luật viết. `NHẸ` không giải thích được hệ quả thì bỏ khỏi danh sách.
4. Kết bằng 3 dòng: điểm mạnh nhất của bản thảo · lỗi lặp lại nhiều nhất · một việc nên sửa trước.

## Ràng Buộc

- Không viết câu thay thế, không đưa bản sửa. User hỏi cách sửa → chuyển sang `editor` hoặc `humanizer`.
- Không liệt kê lỗi trùng nhau. Cùng một lỗi lặp 8 lần → một mục, ghi số lần.
- Không đánh dấu chỉ vì "có thể hay hơn". Phải nêu được cái hỏng.
