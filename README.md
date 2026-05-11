# Personal Agent Bootstrap

`ns-workspace` là Go CLI nhỏ dùng để bootstrap và đồng bộ cấu hình cho các AI coding agent cá nhân. Dự án gom các preset về instructions, skills, subagents, settings, hooks và MCP servers, rồi đẩy chúng sang vị trí native của từng tool như Claude Code, OpenCode, Kimi, Kiro, Qwen, Gemini, Codex, Cline, Windsurf, Aider và một số adapter khác.

Ý tưởng chính là dùng `~/.agents` làm nguồn cấu hình chung, sau đó sync sang các agent khác để cùng một bộ workflow và convention hoạt động trên nhiều tool.

## Lưu Ý

Đây là dự án cá nhân được phát triển nhanh để phục vụ workflow riêng. Một số function, adapter path, MCP config hoặc generated artifact có thể không chính xác tuyệt đối trên mọi máy, mọi phiên bản tool hay mọi môi trường.

Hãy dùng `doctor`, `status`, `--dry-run` và đọc diff/backups trước khi apply lên môi trường quan trọng.

## Cài Đặt Và Chạy Nhanh

Không cần clone repo nếu chỉ muốn chạy bản mới nhất:

```bash
go run github.com/ngosangns/ns-workspace@latest init
```

Một số lệnh hay dùng:

```bash
go run github.com/ngosangns/ns-workspace@latest status
go run github.com/ngosangns/ns-workspace@latest doctor
go run github.com/ngosangns/ns-workspace@latest update
go run github.com/ngosangns/ns-workspace@latest agents
```

Nếu đang làm trong checkout của repo này:

```bash
go run . status
go run . doctor
go run . preview --project . --open
```

## Cách Dùng Đơn Giản

1. Kiểm tra trạng thái hiện tại:

```bash
go run github.com/ngosangns/ns-workspace@latest status
```

2. Xem trước những gì sẽ được ghi:

```bash
go run github.com/ngosangns/ns-workspace@latest init --dry-run
```

3. Tạo cấu hình ban đầu:

```bash
go run github.com/ngosangns/ns-workspace@latest init
```

4. Cập nhật lại preset đã quản lý:

```bash
go run github.com/ngosangns/ns-workspace@latest update
```

5. Kiểm tra JSON config và các CLI agent đã cài:

```bash
go run github.com/ngosangns/ns-workspace@latest doctor
```

## Các Lệnh Chính

- `init`: tạo cấu hình shared và link/copy sang các adapter native. Mặc định bỏ qua file đã tồn tại, trừ khi dùng `--force`.
- `update`: thay thế các phần config do tool quản lý bằng preset embedded và tạo backup timestamp trước khi ghi.
- `status`: hiển thị path đã cài, path thiếu và link hiện có.
- `doctor`: validate JSON config và report các local agent CLI.
- `registry`: cài các skill lấy từ registry.
- `agents`: liệt kê adapter được hỗ trợ, support tier và artifact support.
- `preview`: chạy web dashboard local để đọc và search thư mục `docs/` của một project.

## Flag Hay Dùng

```bash
--agents-home ~/.agents
--tools all
--tools stable
--tools claude,opencode,kimi,kiro,qwen,gemini,codex,cline,windsurf,aider,cursor,trae
--tools kiro-cli
--dry-run
--force
--copy
--no-mcp
--no-registry
```

Dùng `--copy` nếu không muốn tạo symlink.

## Dữ Liệu Được Quản Lý

- Shared instructions: `~/.agents/AGENTS.md`
- Shared subagents: `~/.agents/agents/*.md`
- Custom/private skills: `~/.agents/skills/<name>/SKILL.md`
- Registry-managed skills: `~/.agents/registry/skills.json`
- Shared settings/hooks: `~/.agents/settings.json`
- Shared MCP presets: `~/.agents/mcp/servers.json`
- User-level adapters cho Claude Code, OpenCode, Kimi Code CLI, Kiro/Kiro CLI, Qwen Code, Gemini CLI, Codex CLI, Cline, Windsurf, Aider, Cursor, GitHub Copilot, JetBrains AI, Antigravity, Trae và Roo.

## Adapter Support

Stable adapters ghi vào các user-level path đã biết:

| Agent         | User-level targets                                                                                                         |
| ------------- | -------------------------------------------------------------------------------------------------------------------------- |
| Claude Code   | `~/.claude/CLAUDE.md`, `~/.claude/settings.json` với hooks, `~/.claude/skills`, `~/.claude/agents`, generated MCP commands |
| OpenCode      | `$XDG_CONFIG_HOME/opencode/AGENTS.md`, `skill/`, `agent/`, `opencode.json` với hooks và MCP                                |
| Kimi Code CLI | `~/.kimi/AGENTS.md`, `~/.kimi/skills`, `~/.kimi/mcp.json`                                                                  |
| Kiro / CLI    | `~/.kiro/steering/AGENTS.md`, `~/.kiro/settings/mcp.json`; `--tools kiro-cli` là alias của `kiro`                          |
| Qwen Code     | `~/.qwen/QWEN.md`, `~/.qwen/skills`, `~/.qwen/settings.json` với hooks và MCP                                              |
| Gemini CLI    | `~/.gemini/GEMINI.md`, `~/.gemini/skills`, `~/.gemini/settings.json` với hooks và MCP                                      |
| Codex CLI     | `~/.codex/AGENTS.md`, `~/.codex/skills`, managed MCP block trong `~/.codex/config.toml`                                    |
| Cline         | `~/.cline/data/skills`, `~/.cline/data/agents`, `~/.cline/data/settings/cline_mcp_settings.json`                           |
| Windsurf      | `~/.codeium/windsurf/memories/global_rules.md`                                                                             |
| Aider         | managed conventions block trong `~/.aider.conf.yml`                                                                        |

Manual hoặc experimental adapters tạo guidance trong `~/.agents/generated/<agent>/` thay vì ghi trực tiếp vào native path chưa chắc chắn. Nhóm này hiện gồm Cursor, GitHub Copilot, JetBrains AI, Antigravity, Trae và Roo.

## Docs Preview

Lệnh `preview` chạy một localhost web server để đọc thư mục `docs/` của project. Dashboard có danh sách docs, Markdown preview, typed docs graph và trang search gồm Docs Semantic, Docs Graph, Code Semantic, Code Graph. Trang tổng quan riêng đã được bỏ; preview mặc định mở tài liệu đầu tiên trong Doc tab. Các graph panel dùng `graphify-out/graph.json` nếu file này tồn tại. Markdown code fence `mermaid` sẽ được render thành diagram trong browser.

Ví dụ:

```bash
go run github.com/ngosangns/ns-workspace@latest preview --project /Users/ngosangns/Github/viclass
go run github.com/ngosangns/ns-workspace@latest preview --project . --open
go run . preview --project . --open
```

Preview flags:

```bash
--project PATH
--docs-dir docs
--addr 127.0.0.1:0
--open
```

Preview frontend source nằm trong `internal/preview/preview_ui_src/` và được build thành static assets trong `internal/preview/preview_ui/` để Go có thể embed sẵn. Khi sửa preview UI:

```bash
npm install
npm run check:preview
npm run lint:preview
npm run format:preview
npm run build:preview
```
