---
name: spawn-kimi
description: Spawn Kimi Code CLI process như sub-agent (official headless `kimi -p`) cho research, review, triển khai hoặc làm việc song song có phạm vi rõ.
---

# Spawn Kimi Sub-Agent

Dùng khi cần giao việc cho **Kimi Code CLI** process riêng. Backend luôn là official binary `kimi` (MoonshotAI / Kimi Code), phù hợp khi cần model Kimi coding, context lớn, hoặc chạy song song ngoài main agent.

Nguồn surface chính chủ:

- Headless one-shot: `kimi -p` / `--prompt`
- Session: `-c` / `--continue`, `-S` / `--session`
- ACP (IDE/client): `kimi acp`
- Docs: [Kimi CLI docs](https://moonshotai.github.io/kimi-cli/) · [MoonshotAI/kimi-cli](https://github.com/MoonshotAI/kimi-cli)

## Nguyên Tắc

- Chỉ spawn khi task có phạm vi rõ, đầu ra rõ, lợi từ context tách biệt hoặc song song.
- Ưu tiên official CLI (`kimi -p`). Không thay bằng wrapper Cline/OpenCode trừ khi user yêu cầu.
- Giao quyền sở hữu file/module cụ thể. Nói rõ không revert thay đổi không liên quan.
- Print/prompt mode tự chạy tool calls; chỉ thêm `-y` / `--yolo` khi cần và CLI chấp nhận.
- Luôn yêu cầu sub-agent trả về: tóm tắt, file đã đọc/sửa, validation đã chạy, rủi ro.
- Sau khi hoàn tất, main agent review kết quả, kiểm tra diff/conflict, chạy validation.

## Detect

```bash
command -v kimi || true
kimi -V || true
```

Nếu không có:

```bash
# Official install (macOS/Linux)
curl -fsSL https://code.kimi.com/kimi-code/install.sh | bash
# rồi login
kimi login
```

## MCP (tuỳ chọn)

Preset MCP `kimi` trong `~/.agents/mcp/servers.json` dùng community bridge [kimi-for-claude](https://github.com/7D-codes/kimi-for-claude) — shell ra official `kimi --prompt`. Tools: `kimi_delegate`, `kimi_continue`, `kimi_status`, `kimi_cancel`.

Khi MCP available, có thể dùng MCP tools thay vì shell trực tiếp. Khi không, luôn fallback shell official CLI.

## Lệnh Gọi

Một lượt (cwd = project root):

```bash
cd ABSOLUTE_PROJECT_PATH
kimi -p "FULL_PROMPT" --output-format text
```

Với model override:

```bash
kimi -p "FULL_PROMPT" -m kimi-code/kimi-for-coding --output-format text
```

Plan / read-only:

```bash
kimi -p "FULL_PROMPT" --plan --output-format text
```

Stream JSON (programmatic):

```bash
kimi -p "FULL_PROMPT" --output-format stream-json
```

Continue / resume session:

```bash
kimi -c -p "PROMPT"
kimi -S SESSION_ID -p "PROMPT"
```

ACP server (IDE / agent client):

```bash
kimi acp
```

Flags hữu ích: `-m/--model`, `--add-dir <path>`, `--skills-dir <dir>`, `--plan`, `--output-format text|stream-json`.

## Mẫu Prompt

```markdown
Bạn là Kimi Code CLI sub-agent trong project: ABSOLUTE_PROJECT_PATH

Nhiệm vụ: <one bounded task>

Phạm vi:

- Sở hữu file/module: <paths>
- Không sửa ngoài phạm vi; nếu bị chặn, báo lại trước.
- Không revert thay đổi không liên quan.

Bối cảnh:

- Mục tiêu: <goal>
- Docs/specs/tests liên quan: <paths>
- Triệu chứng/hành vi mong đợi: <details>

Kết quả trả về:

- Tìm thấy gì, file đã đọc/sửa, command đã chạy, rủi ro/follow-up
```

## Chạy Song Song

Chọn số lượng process theo task. Chỉ song song khi: khác file/module hoặc read-only, không phụ thuộc nhau, verify riêng được. Review từng diff, xử lý conflict, chạy validation chung khi kết quả về.

## Không Dùng

- `moonshotai/kimi-cli@codex-worker` — skill **bên trong** Kimi để spawn Codex, không phải spawn Kimi.
- `robsonrung/rar-skills@kimi-runner` — hiện forward model Kimi qua **Cline**, không gọi `kimi` CLI.
- MCP chỉ gọi Moonshot API mà không spawn agent/tool loop khi cần edit codebase đầy đủ.
