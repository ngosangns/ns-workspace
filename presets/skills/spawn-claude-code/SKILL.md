---
name: spawn-claude-code
description: Spawn Claude Code process như sub-agent cho research, review, triển khai hoặc làm việc song song có phạm vi rõ.
---

# Spawn Claude Code Sub-Agent

Dùng skill này khi user muốn gọi riêng Claude Code process làm sub-agent. Skill này khác `spawn-sub-agent` ở chỗ backend luôn là Claude Code, phù hợp khi cần hành vi native của Claude Code, `--print`, agent config, worktree hoặc prompt chạy một lượt.

## Nguyên Tắc

- Chỉ spawn Claude Code khi task có phạm vi rõ, đầu ra rõ, và có lợi từ context tách biệt hoặc chạy song song.
- Nếu sub-agent sửa code, giao quyền sở hữu file/module cụ thể. Nói rõ main agent hoặc agent khác có thể đang sửa code gần đó và không được revert thay đổi không liên quan.
- Không bật các cờ rủi ro như `--dangerously-skip-permissions` hoặc `--allow-dangerously-skip-permissions` trừ khi user yêu cầu rõ và môi trường đã được chấp nhận rủi ro.
- Luôn yêu cầu sub-agent trả về: tóm tắt, file đã đọc/sửa, lệnh validation đã chạy, rủi ro còn lại.
- Sau khi Claude Code process hoàn tất, main agent phải review kết quả, kiểm tra diff/conflict và chạy validation mục tiêu khi phù hợp.

## Detect Claude Code

Trước khi gọi, kiểm tra CLI có sẵn:

```bash
command -v claude || true
claude --help
```

Nếu không có `claude`, báo ngắn gọn rằng máy chưa có Claude Code CLI và dùng fallback phù hợp chỉ khi user đồng ý hoặc task vẫn làm được không cần sub-agent.

## Lệnh Gọi Một Lượt

Mặc định dùng permission mode an toàn:

```bash
claude -p --output-format text --permission-mode default "FULL_PROMPT"
```

Khi cần chọn model hoặc thêm system constraint:

```bash
claude -p --output-format text --permission-mode default --model sonnet "FULL_PROMPT"
claude -p --output-format text --permission-mode default --append-system-prompt "rang buoc bo sung" "FULL_PROMPT"
```

Nếu prompt dài, ghi prompt vào temp file ngoài repo hoặc dùng stdin nếu phiên bản CLI hỗ trợ. Không commit temp prompt file.

## Mẫu Prompt

```markdown
Bạn là Claude Code sub-agent được giao việc trong project: ABSOLUTE_PROJECT_PATH

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
- File đã đọc/sửa, nếu có
- Command đã chạy và kết quả
- Rủi ro, giả định và follow-up cần thiết
```

Giữ prompt đủ cụ thể để Claude Code làm được việc, nhưng chỉ đưa lát cắt cần thiết thay vì toàn bộ lịch sử hội thoại.

## Chạy Song Song

Chỉ chạy nhiều Claude Code process song song khi các task độc lập:

- Khác file/module hoặc chỉ read-only.
- Không phụ thuộc kết quả của nhau.
- Có thể verify riêng trước khi merge kết quả.

Mỗi process nên chạy trong shell session riêng. Trong lúc chờ, main agent tiếp tục xử lý phần không trùng scope. Khi kết quả về, review từng diff, xử lý conflict nếu có, rồi chạy validation chung.
