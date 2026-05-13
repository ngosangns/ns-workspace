---
name: spawn-opencode
description: Spawn OpenCode process như sub-agent cho research, review, triển khai hoặc làm việc song song có phạm vi rõ.
---

# Spawn OpenCode Sub-Agent

Dùng skill này khi user muốn gọi riêng OpenCode process làm sub-agent. Skill này khác `spawn-sub-agent` ở chỗ backend luôn là OpenCode, phù hợp khi cần model/provider linh hoạt, agent config của OpenCode, phiên `opencode serve` có thể attach, hoặc prompt chạy một lượt bằng `opencode run`.

## Nguyên Tắc

- Chỉ spawn OpenCode khi task có phạm vi rõ, đầu ra rõ, và có lợi từ context tách biệt hoặc chạy song song.
- Nếu sub-agent sửa code, giao quyền sở hữu file/module cụ thể. Nói rõ main agent hoặc agent khác có thể đang sửa code gần đó và không được revert thay đổi không liên quan.
- Không bật `--dangerously-skip-permissions` trừ khi user yêu cầu rõ và môi trường đã được chấp nhận rủi ro.
- Ưu tiên `opencode run --dir ABSOLUTE_PROJECT_PATH "FULL_PROMPT"` cho tác vụ một lượt, vì đây là CLI surface được OpenCode docs mô tả cho scripting và automation.
- Dùng `--model provider/model` khi cần chọn model cụ thể, `--agent <name>` khi muốn chạy agent config của OpenCode, `--file <path>` để attach file liên quan, và `--format json` khi cần parse event output.
- Dùng `opencode serve` và `opencode run --attach http://HOST:PORT ...` khi muốn tái sử dụng server đang chạy hoặc cần điều khiển qua OpenCode HTTP/OpenAPI server.
- Luôn yêu cầu sub-agent trả về: tóm tắt, file đã đọc/sửa, lệnh validation đã chạy, rủi ro còn lại.
- Sau khi OpenCode process hoàn tất, main agent phải review kết quả, kiểm tra diff/conflict và chạy validation mục tiêu khi phù hợp.

## Detect OpenCode

Trước khi gọi, kiểm tra CLI có sẵn:

```bash
command -v opencode || true
opencode --help
opencode run --help
```

Nếu không có `opencode`, báo ngắn gọn rằng máy chưa có OpenCode CLI và dùng fallback phù hợp chỉ khi user đồng ý hoặc task vẫn làm được không cần sub-agent.

## Lệnh Gọi Một Lượt

Mặc định chạy non-interactive trong đúng project:

```bash
opencode run --dir ABSOLUTE_PROJECT_PATH "FULL_PROMPT"
```

Khi cần chọn model, agent, title, attach file, hoặc output JSON:

```bash
opencode run --dir ABSOLUTE_PROJECT_PATH --model anthropic/claude-sonnet-4-5 "FULL_PROMPT"
opencode run --dir ABSOLUTE_PROJECT_PATH --agent build --title "bounded task title" "FULL_PROMPT"
opencode run --dir ABSOLUTE_PROJECT_PATH --file path/to/relevant-file.md "FULL_PROMPT"
opencode run --dir ABSOLUTE_PROJECT_PATH --format json "FULL_PROMPT"
```

Khi cần tiếp tục hoặc fork một session:

```bash
opencode run --dir ABSOLUTE_PROJECT_PATH --continue "FOLLOW_UP_PROMPT"
opencode run --dir ABSOLUTE_PROJECT_PATH --session SESSION_ID "FOLLOW_UP_PROMPT"
opencode run --dir ABSOLUTE_PROJECT_PATH --session SESSION_ID --fork "FOLLOW_UP_PROMPT"
```

Khi đã có OpenCode server:

```bash
opencode serve --hostname 127.0.0.1 --port 4096
opencode run --attach http://127.0.0.1:4096 --dir ABSOLUTE_PROJECT_PATH "FULL_PROMPT"
```

Nếu server có basic auth, dùng env thay vì hardcode secret trong command:

```bash
OPENCODE_SERVER_PASSWORD=... opencode run --attach http://127.0.0.1:4096 "FULL_PROMPT"
```

Nếu prompt dài, ghi prompt vào temp file ngoài repo hoặc dùng stdin nếu phiên bản CLI hỗ trợ. Không commit temp prompt file.

## Mẫu Prompt

```markdown
Bạn là OpenCode sub-agent được giao việc trong project: ABSOLUTE_PROJECT_PATH

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
- Nếu cần web/API context hiện tại, tự kiểm tra docs chính thức hoặc source liên quan trước khi kết luận.

Kết quả cần trả về:
- Bạn đã tìm thấy gì
- File đã đọc/sửa, nếu có
- Command đã chạy và kết quả
- Rủi ro, giả định và follow-up cần thiết
```

Giữ prompt đủ cụ thể để OpenCode làm được việc, nhưng chỉ đưa lát cắt cần thiết thay vì toàn bộ lịch sử hội thoại.

## Chạy Song Song

Chỉ chạy nhiều OpenCode process song song khi các task độc lập:

- Khác file/module hoặc chỉ read-only.
- Không phụ thuộc kết quả của nhau.
- Có thể verify riêng trước khi merge kết quả.

Mỗi process nên chạy trong shell session riêng. Trong lúc chờ, main agent tiếp tục xử lý phần không trùng scope. Khi kết quả về, review từng diff, xử lý conflict nếu có, rồi chạy validation chung.
