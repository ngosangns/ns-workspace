---
name: goal
description: Nhận mục tiêu ngôn ngữ tự nhiên, materialize thành harness task rồi chạy harness loop tới khi xong hoặc pause. Trigger /goal hoặc //g.
---

# Goal

Dùng khi user đưa một mục tiêu tổng quát ("làm X", "thêm Y") và muốn hệ thống
tự chạy đến khi xong thay vì agent tự làm tay từng bước. Implementation dựa
hoàn toàn trên harness: goal → task file → `harness run` → loop
plan/execute/verify/diagnose.

## Trigger

- `/goal <mục tiêu>` hoặc `//g <mục tiêu>`

## Quy Trình

1. **Làm rõ scope nếu goal mơ hồ.** Nếu goal không gắn với module/thư mục cụ
   thể (vd: "làm web", "sửa app"), hỏi lại user 1 câu về phạm vi trước khi
   chạy — mặc định scope là `**`, dễ sửa lan.
2. **Materialize + chạy:**

   ```bash
   go run . harness run --goal "<mục tiêu>" --project .
   ```

   Tuỳ chọn:
   - `--scope "internal/auth/**,web/**"`: siết scope thay vì `**`.
   - `--goal-refine`: nhờ plan subagent đề xuất requirements/scope chi tiết
     hơn trước khi ghi task file (tốn 1 LLM call).
   - `--dry-run`: xem phases/agents/eval commands trước khi chạy thật.

   CLI in ra task file đã tạo (`.harness/tasks/<slug>.yaml`). Task file là
   deterministic: cùng goal → cùng file, chạy lại sẽ reuse và resume state.

3. **Xử lý kết quả:**
   - **Finalized**: tóm tắt số iterations, kết quả eval (`harness eval
     --task <slug>` nếu cần xem lại), `git diff --stat` các file đã sửa, báo
     cáo ngắn cho user.
   - **Paused (CI/non-interactive)**: đọc `.harness/decision-request.md`, hỏi
     user để lấy quyết định, cập nhật theo hướng dẫn trong file rồi chạy
     `go run . harness resume --task <slug> --project .`.
   - **Paused (interactive)**: CLI tự in menu; để user trả lờitrực tiếp trên
     terminal, agent chờ process kết thúc rồi đọc status.
4. **Khi user muốn huỷ**: `go run . harness reset --task <slug> --project .`
   (xóa state). `harness stop` chỉ pause, có thể resume lại.

## Nguyên Tắc

- Goal lớn/mơ hồ về thiết kế: làm **plan** theo workflow trong `AGENTS.md` trước;
  `/goal` chỉ cho phần execution đã rõ acceptance.
- Không tự viết/sửa task file bằng tay khi đang dùng `--goal`; nếu cần task
  tùy biến, viết YAML thủ công và chạy `harness run --task` (skill `harness`).
- Không re-implement loop; mọi guardrail (max failures, ambiguity, HITL) do
  harness xử lý.
- Acceptance mặc định là auto-discover (`go test ./...`, package.json
  scripts); nếu goal cần tiêu chí không đo được bằng command, thêm acceptance
  `text` vào task file để verify agent (eval-judge) đánh giá.
