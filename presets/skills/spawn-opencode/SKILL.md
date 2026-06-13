---
name: spawn-opencode
description: Spawn OpenCode process như sub-agent full-permission cho research, review, triển khai hoặc làm việc song song có phạm vi rõ.
---

# Spawn OpenCode Sub-Agent

Dùng khi cần giao việc cho OpenCode process riêng. Backend luôn là OpenCode CLI, phù hợp khi cần full-permission, model linh hoạt, hoặc chạy song song.

## Nguyên Tắc

- Chỉ spawn khi task có phạm vi rõ, đầu ra rõ, lợi từ context tách biệt hoặc song song.
- Mặc định `--dangerously-skip-permissions`. Chỉ bỏ khi user yêu cầu.
- Giao quyền sở hữu file/module cụ thể. Nói rõ không revert thay đổi không liên quan.
- Luôn yêu cầu sub-agent trả về: tóm tắt, file đã đọc/sửa, validation đã chạy, rủi ro.
- Sau khi hoàn tất, main agent review kết quả, kiểm tra diff/conflict, chạy validation.

## Detect

```bash
command -v opencode || true
```

Nếu không có, báo và dùng fallback khi user đồng ý.

## Lệnh Gọi

Một lượt:

```bash
opencode run --dir ABSOLUTE_PROJECT_PATH --dangerously-skip-permissions "FULL_PROMPT"
```

Flags hữu ích: `--model provider/model`, `--agent <name>`, `--file <path>`, `--format json`, `--title "..."`.

Continue/fork session:

```bash
opencode run --dir ... --dangerously-skip-permissions --continue "PROMPT"
opencode run --dir ... --dangerously-skip-permissions --session SESSION_ID "PROMPT"
opencode run --dir ... --dangerously-skip-permissions --session SESSION_ID --fork "PROMPT"
```

Server mode:

```bash
opencode serve --hostname 127.0.0.1 --port 4096
opencode run --attach http://127.0.0.1:4096 --dir ... --dangerously-skip-permissions "PROMPT"
```

## Mẫu Prompt

```markdown
Bạn là OpenCode sub-agent trong project: ABSOLUTE_PROJECT_PATH

Nhiệm vụ: <one bounded task>

Phạm vi:

- Sở hữu file/module: <paths>
- Không sửa ngoài phạm vi; nếu bị chặn, báo lại trước.
- Không revert thay đổi không liên quan.
- Full permission qua `--dangerously-skip-permissions`.

Bối cảnh:

- Mục tiêu: <goal>
- Docs/specs/tests liên quan: <paths>
- Triệu chứng/hành vi mong đợi: <details>

Kết quả trả về:

- Tìm thấy gì, file đã đọc/sửa, command đã chạy, rủi ro/follow-up
```

## Chạy Song Song

Chọn số lượng process theo task. Chỉ song song khi: khác file/module hoặc read-only, không phụ thuộc nhau, verify riêng được. Review từng diff, xử lý conflict, chạy validation chung khi kết quả về.
