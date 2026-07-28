---
name: auto-block-banned-words
description: "Cổng chặn trước khi bàn giao — quét từ cấm và chặn output nếu còn hit. Trigger: chặn từ cấm, auto block, quét trước khi publish, kiểm tra lần cuối."
---

# Auto Block Banned Words

Cổng kiểm cuối cùng. Chạy sau `writer`, `humanizer`, `editor` và trước mọi lần bàn giao hoặc publish. Đây là bước kiểm, không phải bước sửa.

Quy tắc chung: đọc `_shared/CONVENTIONS.md`. Danh sách cấm: `_shared/AI-TELLS.md` Nhóm 1 và Nhóm 2 (cấm cứng), Nhóm 3 (cảnh báo).

## Kết Quả

Verdict rõ ràng, luôn ở một trong hai trạng thái:

- `PASS` — 0 hit Nhóm 1/2, ≤ 1 hit Nhóm 3.
- `BLOCK` — kèm bảng hit: từ/mẫu · vị trí · trích nguyên văn câu chứa nó.

## Workflow

1. Xác định phạm vi quét: file, đoạn được dán, hoặc nội dung vừa sinh ra. Không rõ → hỏi, không tự đoán.
2. Quét Nhóm 1 và Nhóm 2. Bắt cả biến thể: dạng danh từ hóa, dạng tiếng Anh, viết hoa, có dấu và không dấu.
3. Quét Nhóm 3 ở mức cảnh báo. Đếm em dash trên tổng số từ.
4. Loại trừ vùng không áp dụng: code block, output lệnh, trích dẫn nguyên văn có nguồn, tên riêng. Ghi rõ đã loại trừ vùng nào.
5. Ra verdict.
   - `BLOCK` → **không bàn giao nội dung**. Trả bảng hit và đề nghị chạy `ban-the-ai-words` / `ban-the-ai-patterns`.
   - `PASS` → bàn giao kèm một dòng xác nhận đã quét.

## Ràng Buộc

- Không tự sửa. Skill này chỉ chặn.
- Không hạ verdict xuống `PASS` vì "chỉ còn vài chỗ nhỏ". Nhóm 1/2 là cấm cứng.
- Không bỏ qua bước quét khi nội dung ngắn.
- User yêu cầu bỏ qua cổng → làm theo, nhưng nói rõ còn bao nhiêu hit và ở đâu.
