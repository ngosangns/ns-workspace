---
name: spawn-sub-agent
description: Gọi sub-agent qua các CLI local như OpenCode, Claude Code, Kimi Code hoặc Qwen Code cho research, review, triển khai hoặc làm việc song song có phạm vi rõ.
---

# Gọi Sub-Agent

Dùng skill này khi user muốn giao việc cho sub-agent, chạy nhiều agent song song, hoặc tận dụng một agent CLI khác như OpenCode, Claude Code, Kimi Code hay Qwen Code. Giọng làm việc của skill này là tỉnh táo, gọn, quyết đoán: chia việc rõ, giao đúng người, rồi main agent vẫn chịu trách nhiệm review kết quả cuối.

## Nguyên Tắc

- Chỉ gọi sub-agent khi task có phạm vi rõ, đầu ra rõ, và thật sự có lợi từ context mới, model khác hoặc khả năng làm song song.
- Nếu nhiều sub-agent cùng sửa code, chia quyền sở hữu file/module tách biệt. Nói rõ rằng agent khác cũng có thể đang làm việc và không được revert thay đổi không phải của mình.
- Không bật các cờ rủi ro như `--dangerously-skip-permissions`, `--yolo`, `--yes`, `--allow-dangerously-skip-permissions` trừ khi user yêu cầu rõ và môi trường đã được chấp nhận rủi ro.
- Luôn yêu cầu sub-agent trả về: tóm tắt, file đã đọc/sửa, lệnh validation đã chạy, rủi ro còn lại.
- Sau khi sub-agent hoàn tất, main agent phải review kết quả, kiểm tra conflict và chạy validation mục tiêu khi phù hợp.

## Chọn Backend

1. Nếu user chỉ định backend, dùng đúng backend đó.
2. Nếu cần model/provider linh hoạt hoặc một session gọn nhẹ, ưu tiên OpenCode.
3. Nếu cần hành vi native của Claude Code, agent config, worktree hoặc `--print`, dùng Claude Code.
4. Nếu muốn một CLI thay thế với skills/config riêng, dùng Kimi Code hoặc Qwen Code.
5. Nếu chưa chắc CLI có sẵn, detect trước:

```bash
command -v opencode || true
command -v claude || true
command -v kimi || true
command -v qwen || true
opencode run --help
claude --help
kimi --help
qwen --help
```

## Lệnh Gọi Một Lượt

OpenCode:

```bash
opencode run --dir "$PWD" --title "ten-task-ngan" "FULL_PROMPT"
opencode run --dir "$PWD" -m "provider/model" "FULL_PROMPT"
```

Claude Code:

```bash
claude -p --output-format text --permission-mode default "FULL_PROMPT"
claude -p --output-format text --model sonnet --append-system-prompt "rang buoc bo sung" "FULL_PROMPT"
```

Kimi Code:

```bash
kimi --work-dir "$PWD" --print --final-message-only -p "FULL_PROMPT"
kimi --work-dir "$PWD" --print --final-message-only -m "model-name" -p "FULL_PROMPT"
```

Qwen Code:

```bash
qwen -o text "FULL_PROMPT"
qwen -m "model-name" -o text "FULL_PROMPT"
```

Nếu prompt dài, ghi prompt vào temp file rồi pipe vào CLI nếu CLI hỗ trợ stdin/print mode. Không commit temp prompt file.

## Mẫu Prompt

```markdown
Bạn là sub-agent được giao việc trong project: ABSOLUTE_PROJECT_PATH

Nhiệm vụ:
<one bounded task>

Phạm vi:
- Bạn sở hữu các file/module này: <paths>
- Không sửa ngoài phạm vi này trừ khi thật sự cần thiết; nếu bị chặn, báo lại trước.
- Main agent hoặc agent khác có thể đang sửa code gần đó. Không revert thay đổi không liên quan.

Bối cảnh:
- Mục tiêu của user: <goal>
- Docs/specs/tests liên quan: <paths or snippets>
- Triệu chứng hiện tại hoặc hành vi mong đợi: <details>

Kết quả cần trả về:
- Bạn đã tìm thấy gì
- File đã sửa, nếu có
- Command đã chạy và kết quả
- Rủi ro, giả định và follow-up cần thiết
```

Giữ prompt đủ cụ thể để sub-agent làm được việc, nhưng đừng nhồi toàn bộ lịch sử hội thoại. Cho nó đúng lát cắt cần thiết là đẹp nhất.

## Chạy Song Song

Chỉ chạy song song khi các task độc lập:

- Khác file/module hoặc chỉ read-only.
- Không phụ thuộc kết quả của nhau.
- Có thể verify riêng trước khi merge kết quả.

Với các task độc lập, gọi mỗi backend bằng shell session riêng và tiếp tục làm việc ở phần không trùng scope trong lúc chờ. Khi kết quả về, review từng diff, xử lý conflict nếu có, rồi chạy validation chung.
