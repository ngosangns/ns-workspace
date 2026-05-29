# ns-workspace

`ns-workspace` là Go CLI để bootstrap và đồng bộ cấu hình AI coding agent cá nhân. Repo này gom một bộ preset dùng chung cho instructions, skills, subagents, settings, hooks, registry và MCP servers, rồi materialize chúng sang các vị trí native của từng tool như Claude Code, OpenCode, Grok Build, Kimi, Kiro, Qwen, Gemini, Codex, Cline, Windsurf và Aider.

Ý tưởng chính là dùng `~/.agents` làm nguồn cấu hình chung. Từ đó, mỗi agent có thể nhận cùng một bộ workflow, trigger skill và convention mà không phải bảo trì thủ công từng thư mục cấu hình riêng.

Repo cũng có lệnh `preview` để chạy web dashboard local cho thư mục `docs/` của một project, bao gồm Markdown/HTML preview, docs graph, search và code graph dựa trên LSP khi language server của project có sẵn. Lệnh `search` mở riêng trải nghiệm Search/Code Graph standalone bằng một file HTML launcher sinh tại thư mục hiện tại, lệnh `graph` chạy query non-interactive để agent lấy kết quả Search/Code Graph dạng text/JSON, còn nhóm `lsp` quản lý language server dùng cho Code Graph.

## Trạng Thái

Đây là dự án cá nhân, phát triển nhanh để phục vụ workflow riêng. Một số adapter path, hook command, MCP config hoặc generated artifact có thể phụ thuộc vào phiên bản tool và môi trường local.

Trước khi apply lên môi trường quan trọng, hãy dùng `doctor`, `status`, `--dry-run` và đọc diff/backups.

## Cài Đặt Nhanh

Không cần clone repo nếu chỉ muốn chạy bản mới nhất:

```bash
go run github.com/ngosangns/ns-workspace@latest init
```

Các lệnh kiểm tra thường dùng:

```bash
go run github.com/ngosangns/ns-workspace@latest status
go run github.com/ngosangns/ns-workspace@latest doctor
go run github.com/ngosangns/ns-workspace@latest update
go run github.com/ngosangns/ns-workspace@latest agents
```

Nếu đang làm trong checkout local:

```bash
go run . status
go run . doctor
go run . preview --project . --open
go run . search --project .
go run . graph --project . --query buildPreviewSearchResponse --json
go run . lsp list --project .
```

Khi muốn dùng checkout này để preview một project khác, chạy `go run .` từ thư mục `ns-workspace` và trỏ `--project` sang project cần đọc:

```bash
cd /Users/ngosangns/Github/ns-workspace
go run . preview --project /Users/ngosangns/Github/viclass --open
```

Không dùng dạng `go run /Users/ngosangns/Github/ns-workspace ...` từ một repo không có `go.mod`, vì Go sẽ cố tìm module từ current working directory trước khi chương trình này kịp chạy.

## Quy Trình Cơ Bản

1. Kiểm tra trạng thái cấu hình hiện tại:

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

5. Kiểm tra JSON config và các local agent CLI:

```bash
go run github.com/ngosangns/ns-workspace@latest doctor
```

## Lệnh Chính

| Lệnh       | Mục đích                                                                                                                                       |
| ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `init`     | Tạo cấu hình shared và link/copy sang adapter native. Mặc định bỏ qua file đã tồn tại, trừ khi dùng `--force`.                                 |
| `update`   | Rewrite các phần config do tool quản lý từ preset embedded, tạo backup timestamp trước khi ghi và xóa nội dung managed không còn trong preset. |
| `status`   | Hiển thị path đã cài, path thiếu và link hiện có.                                                                                              |
| `doctor`   | Validate JSON config và report các local agent CLI.                                                                                            |
| `registry` | Cài các skill lấy từ registry.                                                                                                                 |
| `agents`   | Liệt kê adapter được hỗ trợ, support tier và artifact support.                                                                                 |
| `catalog`  | Alias của `agents`.                                                                                                                            |
| `preview`  | Chạy web dashboard local để đọc và search thư mục `docs/` của một project.                                                                     |
| `search`   | Mở Search/Code Graph standalone bằng HTML launcher và local API server.                                                                        |
| `graph`    | Chạy query terminal bằng cùng backend Search/LSP Code Graph.                                                                                   |
| `lsp`      | Liệt kê hoặc cài language server mà LSP Code Graph dùng.                                                                                       |

## Flag Hay Dùng

```bash
--agents-home ~/.agents
--tools all
--tools stable
--tools claude,opencode,grok,kimi,kiro,qwen,gemini,codex,cline,windsurf,aider,cursor,trae
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
- User-level adapters cho Claude Code, OpenCode, Grok Build, Kimi Code CLI, Kiro/Kiro CLI, Qwen Code, Gemini CLI, Codex CLI, Cline, Windsurf, Aider, Cursor, GitHub Copilot, JetBrains AI, Antigravity, Trae và Roo.

## Adapter Support

Stable adapters ghi vào các user-level path đã biết:

| Agent         | User-level targets                                                                                                         |
| ------------- | -------------------------------------------------------------------------------------------------------------------------- |
| Claude Code   | `~/.claude/CLAUDE.md`, `~/.claude/settings.json` với hooks, `~/.claude/skills`, `~/.claude/agents`, generated MCP commands |
| OpenCode      | `$XDG_CONFIG_HOME/opencode/AGENTS.md`, `skill/`, `agent/`, `opencode.json` với hooks và MCP                                |
| Grok Build    | `~/.grok/skills`; Grok cũng đọc `AGENTS.md` trong project và `~/.agents/skills` theo compatibility của Grok Build          |
| Kimi Code CLI | `~/.kimi/AGENTS.md`, `~/.kimi/skills`, `~/.kimi/mcp.json`                                                                  |
| Kiro / CLI    | `~/.kiro/steering/AGENTS.md`, `~/.kiro/skills`, `~/.kiro/settings/mcp.json`; `--tools kiro-cli` là alias của `kiro`        |
| Qwen Code     | `~/.qwen/QWEN.md`, `~/.qwen/skills`, `~/.qwen/settings.json` với hooks và MCP                                              |
| Gemini CLI    | `~/.gemini/GEMINI.md`, `~/.gemini/skills`, `~/.gemini/settings.json` với hooks và MCP                                      |
| Codex CLI     | `~/.codex/AGENTS.md`, `~/.codex/skills`, managed MCP block trong `~/.codex/config.toml`                                    |
| Cline         | `~/.cline/data/skills`, `~/.cline/data/agents`, `~/.cline/data/settings/cline_mcp_settings.json`                           |
| Windsurf      | `~/.codeium/windsurf/memories/global_rules.md`                                                                             |
| Aider         | Managed conventions block trong `~/.aider.conf.yml`                                                                        |

Manual hoặc experimental adapters tạo guidance trong `~/.agents/generated/<agent>/` thay vì ghi trực tiếp vào native path chưa chắc chắn. Nhóm này hiện gồm Cursor, GitHub Copilot, JetBrains AI, Antigravity, Trae và Roo.

## Docs Preview

Lệnh `preview` chạy một localhost web server để đọc thư mục `docs/` của project. Dashboard có sidebar tài liệu, Markdown/HTML preview, Graph tab và Search tab. Search có các panel Docs Semantic, Docs Graph, Code Semantic và Code Graph; Code Graph index symbol từ LSP trên các file code tracked bởi Git, rồi mở rộng caller/callee hoặc references khi language server hỗ trợ. Bản `go run github.com/ngosangns/ns-workspace@latest preview` dùng static preview UI đã được Go embed, nên không yêu cầu Node.js ở runtime.

Ví dụ:

```bash
go run github.com/ngosangns/ns-workspace@latest preview --project /Users/ngosangns/Github/viclass
go run github.com/ngosangns/ns-workspace@latest preview --project . --open
cd /Users/ngosangns/Github/ns-workspace
go run . preview --project /Users/ngosangns/Github/viclass --open
```

Preview flags:

```bash
--project PATH
--docs-dir docs
--addr 127.0.0.1:0
--open
--no-reload
```

Preview frontend source nằm trong `internal/preview/preview_ui_src/` và được build thành static assets trong `internal/preview/preview_ui/` để Go có thể embed. `internal/preview/preview_ui/index.html`, `style.css`, `favicon.svg` và bundle JS hashed trong `internal/preview/preview_ui/assets/` phải được cập nhật cùng nhau sau khi build để bản module `@latest` serve được đầy đủ asset. Khi sửa preview UI:

```bash
npm install
npm run check:preview
npm run lint:preview
npm run format:preview
npm run build:preview
```

## Search Standalone Và Graph Query

Lệnh `search` dùng cùng backend search với `preview`, nhưng mở trực tiếp entry Search standalone. Command tạo file launcher mặc định `ns-workspace-search.html` trong thư mục đang chạy, start local API server, rồi mở browser mặc định tới launcher đó. File launcher trỏ tới server đang chạy, nên command cần tiếp tục sống trong terminal để search động hoạt động.

Lệnh `graph` chỉ chạy query terminal: nó không sinh launcher, không mở browser và không giữ server sống. Output dùng cùng response với `/api/search`, gồm `docsSemantic`, `docsGraph`, `codeSemantic`, `codeGraph`, `stats` và `warnings`. Mặc định command tự ensure language server còn thiếu cho các language phát hiện trong project trước khi query, cài vào cache user của `ns-workspace` và vẫn fail-open nếu cài đặt lỗi hoặc server không hỗ trợ relation expansion. Dùng `--no-ensure-lsp` khi cần query read-only không có network/install side effect.

```bash
go run github.com/ngosangns/ns-workspace@latest search --project /Users/ngosangns/Github/viclass
go run . search --project . --out ./search.html
go run . graph --project . --query buildPreviewSearchResponse --json
go run . graph --project . --no-ensure-lsp --query buildPreviewSearchResponse --json
go run . graph --project . --query auth,session --keyword-op difference --limit 5
go run . lsp list --project .
go run . lsp install auto --project .
```

Search flags:

```bash
--project PATH
--docs-dir docs
--addr 127.0.0.1:0
--out ./ns-workspace-search.html
--no-open
```

Graph flags:

```bash
--project PATH
--docs-dir docs
--query "symbol-or-concept"
--limit 8
--keyword-op sum|difference
--ensure-lsp
--no-ensure-lsp
--json
```

LSP commands:

```bash
lsp list [--project PATH] [--docs-dir docs] [--json]
lsp install <language|auto> [--project PATH] [--docs-dir docs] [--force] [--dry-run] [--json]
```

`lsp` hỗ trợ HTML, CSS, SCSS/Sass, JavaScript, TypeScript, Go/Golang và Kotlin. `lsp install` cài vào cache user của `ns-workspace` thay vì sửa project được inspect. Mặc định dùng `os.UserCacheDir()/ns-workspace/lsp`; có thể override bằng `NS_WORKSPACE_LSP_CACHE`. Resolver vẫn ưu tiên binary có sẵn trong `PATH`, Go bin dirs và `node_modules/.bin` của project/checkout trước khi dùng cache. Kotlin dùng `kotlin-lsp`; do upstream chưa có artifact release ổn định kèm checksum để tải tự động an toàn, `lsp install kotlin` trả hướng dẫn cài thủ công thay vì tự tải archive.

## Phát Triển

Xem [DEVELOPER.md](./DEVELOPER.md) để biết cấu trúc repo, workflow test/lint/format và các quy ước khi sửa preset, adapter hoặc preview web.

## Copyright

Xem [COPYRIGHT.md](./COPYRIGHT.md). Repo hiện chưa khai báo open-source license riêng; không mặc định có quyền sử dụng lại ngoài quyền được nêu trong file đó hoặc thỏa thuận bằng văn bản.
