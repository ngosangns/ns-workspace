# ns-workspace

`ns-workspace` là Go CLI để bootstrap và đồng bộ cấu hình AI coding agent cá nhân. Repo gom preset dùng chung cho instructions, skills, subagents, settings, hooks, registry và MCP servers, rồi materialize chúng sang các vị trí native của Claude Code, OpenCode, Grok Build, Kimi, Kiro, Qwen, Gemini, Codex, Cline, Windsurf, Aider và các adapter khác.

Ý tưởng chính là dùng `~/.agents` làm nguồn cấu hình chung. Từ đó, mỗi agent nhận cùng workflow, trigger skill và convention mà không phải bảo trì thủ công từng thư mục cấu hình riêng.

Repo cũng có các lệnh đọc knowledge base: `preview` chạy web dashboard local cho `docs/`, `search` mở Search/Code Graph standalone, `graph` chạy query terminal dạng text/JSON, `export` dump docs + graph thành một file HTML tĩnh self-contained, `mcp` expose `docs/` cho AI agent qua MCP server stdio local, còn `lsp` quản lý language server dùng cho Code Graph qua graph-query LSP registry.

## Trạng Thái

Đây là dự án cá nhân, phát triển nhanh để phục vụ workflow riêng. Một số adapter path, hook command, MCP config hoặc generated artifact có thể phụ thuộc vào phiên bản tool và môi trường local.

Trước khi apply lên môi trường quan trọng, hãy dùng `doctor`, `status`, `--dry-run` và đọc diff/backups.

## Sử Dụng Nhanh

Không cần clone repo nếu chỉ muốn chạy bản mới nhất:

```bash
go run github.com/ngosangns/ns-workspace@latest status
go run github.com/ngosangns/ns-workspace@latest doctor
go run github.com/ngosangns/ns-workspace@latest init --dry-run
go run github.com/ngosangns/ns-workspace@latest init
go run github.com/ngosangns/ns-workspace@latest update
go run github.com/ngosangns/ns-workspace@latest harness list
go run github.com/ngosangns/ns-workspace@latest harness run --task <id> --project .
```

Trong checkout local:

```bash
go run . status
go run . doctor
go run . preview --project . --open
go run . search --project .
go run . graph --project . --query buildPreviewSearchResponse --json
go run . export --project . --out ./kb.html --open
go run . mcp --project .
go run . setup
go run . lsp list --project .
```

Khi dùng checkout này để preview một project khác, chạy `go run .` từ thư mục `ns-workspace` và trỏ `--project` sang project cần đọc:

```bash
cd /Users/ngosangns/Github/ns-workspace
go run . preview --project /Users/ngosangns/Github/viclass --open
```

Không dùng dạng `go run /Users/ngosangns/Github/ns-workspace ...` từ một repo không có `go.mod`, vì Go sẽ cố tìm module từ current working directory trước khi chương trình này kịp chạy.

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
| `harness`  | Chạy harness task: list, run, eval, status, resume, stop. Hỗ trợ self-correct loop, multi-agent routing và memory persistence.                 |
| `preview`  | Chạy web dashboard local để đọc và search thư mục `docs/` của một project.                                                                     |
| `search`   | Mở Search/Code Graph standalone bằng HTML launcher và local API server.                                                                        |
| `graph`    | Chạy query terminal bằng cùng backend Search/LSP Code Graph.                                                                                   |
| `export`   | Xuất toàn bộ docs + graph thành một file HTML tĩnh self-contained, mở offline qua `file://`.                                                   |
| `mcp`      | Khởi động MCP server stdio local expose `docs/` cho agent (list/lookup/search/modify).                                                         |
| `kb`       | Thao tác OKF trên docs: `kb validate` kiểm conformance, `kb index` sinh lại `index.md` từng thư mục.                                           |
| `setup`    | Sinh hoặc merge `Taskfile.yml` ở cwd liệt kê toàn bộ scripts/commands của ns-workspace để dùng với [go-task](https://taskfile.dev/). Hỗ trợ `--dry-run` xem trước và `--force` ghi đè. |
| `lsp`      | Liệt kê hoặc cài language server mà LSP Code Graph dùng.                                                                                       |

## Flag Hay Dùng

```bash
--agents-home ~/.agents
--config ~/.config/ns-workspace/config.json
--tools all
--tools stable
--tools claude,opencode,grok,kimi,kiro,qwen,gemini,codex,cline,windsurf,aider,minimax,cursor,trae
--tools kiro-cli
--tools minimax-cli
--tools mmx
--dry-run
--force
--copy
--no-mcp
--no-registry
```

Harness flags:

```bash
--project PATH   project root to inspect, default current directory
--task ID        task id for run/eval/status/resume/stop
--dry-run        show planned actions without running
```

Dùng `--copy` nếu không muốn tạo symlink. Dùng `--config <file>` trỏ tới file JSON user-level để override hoặc bổ sung embedded preset (xem [User Config Overlay](#user-config-overlay)).

## User Config Overlay

`ns-workspace` cho phép cá nhân hoá preset mà không cần fork repo. Tạo file JSON ở vị trí mặc định `~/.config/ns-workspace/config.json` (override bằng `NS_WORKSPACE_CONFIG` hoặc `--config`) với format:

```json
{
  "presets/agents/AGENTS.md": "/home/me/.config/ns-workspace/AGENTS.md",
  "presets/opencode/opencode.json": "/home/me/.config/ns-workspace/opencode.json",
  "presets/skills/custom-skill/SKILL.md": "/home/me/.config/ns-workspace/skill.md"
}
```

Key là preset path (bắt đầu bằng `presets/`, dùng `/`), value là đường dẫn tuyệt đối tới file user. User file đè embedded preset; nếu key chỉ vào path mà embedded không có (vd: `presets/skills/custom-skill/SKILL.md`), file đó được cài như skill mới.

Ví dụ preset mặc định opencode với full authorization + tăng timeout:

```json
// ~/.config/ns-workspace/opencode.json
{
  "permission": "allow",
  "timeout": 300000
}
```

Sau `ns-workspace init`/`update`, `~/.config/opencode/opencode.json` sẽ có cả `permission` lẫn `timeout`. Tắt overlay bằng `--config ""`.

Preset MiniMax CLI (`presets/skills/minimax-cli/SKILL.md`) mặc định đã có sẵn, cấp full authorization cho mọi subcommand và đề xuất timeout dài hơn cho video/music generation. Có thể override qua user config hoặc qua `npx skills add MiniMax-AI/cli -y -g` để lấy bản chính thức mới nhất.

## MiniMax CLI Adapter

MiniMax CLI (`mmx`) được hỗ trợ như một stable adapter, chọn bằng `--tools minimax` (alias: `minimax-cli`, `mmx`). mmx-cli là multimodal CLI (text/image/video/speech/music) nên adapter chỉ quản lý config — không có skills/agents/MCP user-level directory để fan-out:

- Preset: `presets/minimax/config.json` (default model + region).
- Native target: `~/.mmx/config.json`, ghi qua `MergeJSON` với `Replace: true` trên `update` để cleanup stale managed keys, mirror cùng pattern với `opencode`.
- Default models: `MiniMax-M3` (text), `speech-2.8-hd` (speech), `MiniMax-Hailuo-2.3` (video), `music-2.6` (music).
- Default timeouts: `timeout: 1800` (per-call, 30 phút) + `sessionTimeout: 1800` (long-running session như video generation sync).
- Bật: `go run . init --tools minimax` (sau khi `npm install -g mmx-cli && mmx auth login`).
- Override defaults qua user config: thêm `"presets/minimax/config.json": "/path/to/your.json"` vào `~/.config/ns-workspace/config.json`.
- Official SKILL tự động cài qua registry: `npx skills add MiniMax-AI/cli -y -g` chạy trong phase registry install khi không dùng `--no-registry`. User có thể chạy lại bất cứ lúc nào qua `sh ~/.agents/registry/install.sh`.

## Dữ Liệu Được Quản Lý

- Shared instructions: `~/.agents/AGENTS.md`
- Shared subagents: `~/.agents/agents/*.md`
- Custom/private skills: `~/.agents/skills/<name>/SKILL.md`
- Registry-managed skills: `~/.agents/registry/skills.json`
- Shared settings/hooks: `~/.agents/settings.json`
- Shared MCP presets: `~/.agents/mcp/servers.json`

Stable adapters ghi vào các user-level path đã biết:

| Agent         | User-level targets                                                                                                                  |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| Claude Code   | `~/.claude/CLAUDE.md`, `~/.claude/settings.json` với hooks, `~/.claude/skills`, `~/.claude/agents`, generated MCP commands          |
| OpenCode      | `$XDG_CONFIG_HOME/opencode/AGENTS.md`, `skill/`, `agent/`, `opencode.json` với hooks và MCP                                         |
| Grok Build    | `~/.grok/skills`; Grok cũng đọc `AGENTS.md` trong project và `~/.agents/skills` theo compatibility của Grok Build                   |
| Kimi Code CLI | `~/.kimi/AGENTS.md`, `~/.kimi/skills`, `~/.kimi/mcp.json`                                                                           |
| Kiro / CLI    | `~/.kiro/steering/AGENTS.md`, `~/.kiro/skills`, `~/.kiro/settings/mcp.json`; `--tools kiro-cli` là alias của `kiro`                 |
| Qwen Code     | `~/.qwen/QWEN.md`, `~/.qwen/skills`, `~/.qwen/settings.json` với hooks và MCP                                                       |
| Gemini CLI    | `~/.gemini/GEMINI.md`, `~/.gemini/skills`, `~/.gemini/settings.json` với MCP (HTTP servers dùng `httpUrl`, không có `hooks` ở root) |
| Codex CLI     | `~/.codex/AGENTS.md`, `~/.codex/skills`, managed MCP block trong `~/.codex/config.toml`                                             |
| Cline         | `~/.cline/data/skills`, `~/.cline/data/agents`, `~/.cline/data/settings/cline_mcp_settings.json`                                    |
| Windsurf      | `~/.codeium/windsurf/memories/global_rules.md`                                                                                      |
| Aider         | Managed conventions block trong `~/.aider.conf.yml`                                                                                 |
| MiniMax CLI   | `~/.mmx/config.json` (default model presets); alias `minimax-cli` / `mmx` qua `--tools`                                             |

Manual hoặc experimental adapters tạo guidance trong `~/.agents/generated/<agent>/` thay vì ghi trực tiếp vào native path chưa chắc chắn. Nhóm này hiện gồm Cursor, GitHub Copilot, JetBrains AI, Antigravity, Trae và Roo.

## Preview, Search, Graph Và Harness

`preview` chạy localhost web server để đọc thư mục `docs/` của project. Dashboard có sidebar tài liệu, Markdown/HTML preview, Graph tab và Search tab. Search có các panel Docs Semantic, Docs Graph, Code Semantic và Code Graph; Code Graph index symbol từ LSP trên file code tracked bởi Git, bỏ qua generated preview UI artifacts của repo, rồi mở rộng caller/callee hoặc references khi language server hỗ trợ.

`harness` chạy task tự động hóa workflow dev với self-correct loop. Mỗi task là file YAML/JSON trong `.harness/tasks/`, định nghĩa requirements, scope, acceptance criteria, routing và stopping rules. Loop đi qua các phase plan → execute → verify → diagnose, lưu checkpoint sau mỗi phase và dừng khi verify pass, state lặp, hết hypothesis, hoặc phát hiện ambiguity. Ngoài task dev generic, harness còn hỗ trợ task type `enrich-docs`: enrich docs từ seed URL với hard caps code-enforced (`max_pages`, `allowed_hosts`, `max_depth`, timeout) và chỉ ghi file bên trong docs root. Xem thêm [docs/features/agentic-loop.md](docs/features/agentic-loop.md) và [docs/modules/harness.md](docs/modules/harness.md).

```bash
go run github.com/ngosangns/ns-workspace@latest preview --project /Users/ngosangns/Github/viclass --open
go run . preview --project . --open
```

Preview flags:

```bash
--project PATH
--docs-dir docs
--addr 127.0.0.1:0
--open
--no-reload
```

`search` dùng cùng backend search với `preview`, nhưng mở entry Search standalone từ HTML launcher. Command cần tiếp tục sống trong terminal để frontend gọi local API server.

`graph` chỉ chạy query terminal: không sinh launcher, không mở browser và không giữ server sống. Mặc định command tự ensure language server còn thiếu cho project trước khi query, cài vào cache user của `ns-workspace` và vẫn fail-open nếu cài đặt lỗi hoặc server không hỗ trợ relation expansion.

```bash
go run . search --project . --out ./search.html
go run . graph --project . --query buildPreviewSearchResponse --json
go run . graph --project . --no-ensure-lsp --query buildPreviewSearchResponse --json
go run . graph --project . --query auth,session --keyword-op difference --limit 5
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

## Export Tĩnh Và MCP

`export` build một file HTML duy nhất, self-contained, nhúng toàn bộ knowledge base rồi render client-side bằng **OKF Bundle Viewer** (port từ [GoogleCloudPlatform/knowledge-catalog](https://github.com/GoogleCloudPlatform/knowledge-catalog), Apache 2.0). Viewer có force-directed graph (Cytoscape.js), detail panel hiện frontmatter + body render bằng marked, danh sách "Cited by" backlinks, search theo title/id/tag, filter theo type và đổi layout (cose/concentric/breadth-first/circle/grid). Mặc định `--inline-assets=true` nhúng luôn thư viện render nên file mở được qua `file://` mà không gọi mạng; `--inline-assets=false` tham chiếu CDN. Command tái dùng cùng knowledge core với `preview`/`search`, validate docs dir trước khi ghi và fail-open khi một doc lỗi. Internal link `.md` giữa các doc được rewrite sang dạng OKF bundle-relative để điều hướng ngay trong viewer.

```bash
go run . export --project . --out ./ns-workspace-kb.html --open
go run . export --project . --name "ns-workspace KB"
go run . export --project . --no-graph
go run . export --project . --inline-assets=false
```

Export flags:

```bash
--project PATH        project root to export, default current directory
--docs PATH           docs directory, default docs
--out PATH            output HTML file path, default ./ns-workspace-kb.html
--name NAME           display name in the viewer header, default project name
--no-graph            export documents only, without the relationship edges
--inline-assets       inline render libraries for fully offline output, default true
--open                open the generated file after writing
```

`mcp` khởi động một MCP server local-only giao tiếp JSON-RPC 2.0 qua stdin/stdout (không bind network port), để AI agent đọc/sửa knowledge base trực tiếp. Server expose bốn tool: `list_docs` (liệt kê docs, filter theo `type`/`tag`), `lookup_doc` (lấy full content + metadata theo id), `search_docs` (search bằng cùng pipeline với preview/search), và `modify_doc` (tạo/sửa doc, chặn path traversal ra ngoài docs root). Server được spawn như stdio subprocess bởi agent MCP-capable.

```bash
go run . mcp --project .
```

MCP flags:

```bash
--project PATH        project root to expose, default current directory
--docs PATH           docs directory, default docs
```

## Metadata Docs Theo OKF

Docs có thể khai báo metadata bằng YAML frontmatter chuẩn (`---`) theo tinh thần OKF, với các key `type`, `description`, `tags`, `timestamp` cùng các key tương thích (`status`, `version`, `compliance`, `priority`, `links`). `tags` nhận cả string đơn lẫn array và được normalize về `[]string`. Parser là permissive consumer: key lạ hoặc `type` không biết vẫn được chấp nhận, không báo lỗi.

Frontmatter tương thích ngược với section `## Meta` dạng prose đang dùng: doc chỉ có `## Meta` hoạt động y như trước; doc có cả hai thì frontmatter thắng ở key trùng và `## Meta` điền các field còn trống. Nếu frontmatter lỗi cú pháp, hệ thống fallback sang `## Meta`, ghi warning và không panic.

## Lệnh kb (OKF)

`kb` gom các thao tác trên knowledge base theo Open Knowledge Format:

- `kb validate` kiểm OKF conformance: mọi doc (trừ file reserved `index.md`/`log.md`) phải có YAML frontmatter parse được với `type` không rỗng. Thiếu key khuyến nghị (`title`/`description`/`timestamp`) là warning; `--strict` nâng warning thành lỗi. Exit non-zero khi có doc không conformant nên dùng được trong CI. `--json` xuất report cho tooling.
- `kb index` sinh lại file `index.md` cho từng thư mục (progressive disclosure, theo OKF SPEC §6): group entry theo `type`, kèm description, và liệt kê subdirectory. `--dry-run` in danh sách file sẽ ghi mà không ghi thật.

```bash
go run . kb validate --project .
go run . kb validate --project . --strict --json
go run . kb index --project . --dry-run
go run . kb index --project .
```

KB flags:

```bash
--project PATH        project root, default current directory
--docs PATH           docs directory, default docs
--json                (validate) in report dạng JSON
--strict              (validate) coi warning key khuyến nghị là lỗi
--dry-run             (index) in file sẽ ghi mà không ghi
```

## LSP Cho Code Graph

`lsp` hỗ trợ HTML, CSS, SCSS/Sass, JavaScript, TypeScript, Go/Golang và Kotlin. `lsp install` cài vào cache user của `ns-workspace` thay vì sửa project được inspect. Mặc định dùng `os.UserCacheDir()/ns-workspace/lsp`; có thể override bằng `NS_WORKSPACE_LSP_CACHE`.

Resolver ưu tiên binary có sẵn trong `PATH`, Go bin dirs và `node_modules/.bin` của project/checkout trước cache. Kotlin dùng `kotlin-lsp`; `lsp install kotlin` tải JetBrains Kotlin LSP standalone archive theo OS/arch, verify SHA-256 đã pin, extract vào cache versioned và tạo wrapper `<cache>/kotlin/bin/kotlin-lsp`.

```bash
go run . lsp list --project . --json
go run . lsp install auto --project .
go run . lsp install kotlin --project .
```

## Lệnh `setup`

`setup` sinh (hoặc merge) file `Taskfile.yml` ở thư mục hiện tại để dùng với [go-task](https://taskfile.dev/). Taskfile liệt kê toàn bộ commands của `ns-workspace` (nhóm `ns:*`), npm scripts trong `package.json` (nhóm `lint:*`, `format:*`, `build:*`) và Go toolchain tasks (`go:build`, `go:test`). Sau khi setup, `task --list` sẽ hiển thị tất cả task có thể chạy trong repo.

```bash
go run . setup                # tạo/merge Taskfile.yml ở cwd
go run . setup --dry-run      # xem nội dung sẽ ghi, không tạo file
go run . setup --force        # ghi đè Taskfile.yml thay vì merge
go run . setup --target ~/p   # ghi Taskfile.yml vào thư mục khác
```

Sau khi setup, dùng `task --list` để xem tất cả task và gọi trực tiếp:

```bash
task --list
task ns:status
task ns:preview
task lint:docs:fix
task go:test
```

Setup flags:
```bash
--target PATH     directory to write Taskfile.yml, default current directory
--dry-run         print planned Taskfile.yml on stdout instead of writing
--force           replace existing Taskfile.yml instead of merging
```

Khi merge, **task trùng tên** trong `Taskfile.yml` hiện có sẽ bị rewrite từ preset (kể cả `desc` và `cmds`). Đặt tên task riêng — không dùng các prefix `ns:`, `lint:`, `format:`, `build:`, `go:` — để giữ task do bạn tự định nghĩa qua nhiều lần setup. Các key khác ở top-level (vd `vars`, `includes`) được giữ nguyên.

## Phát Triển

Xem [DEVELOPER.md](./DEVELOPER.md) để biết cấu trúc repo, workflow test/lint/format và các quy ước khi sửa preset, adapter hoặc preview web.

## Copyright

Xem [COPYRIGHT.md](./COPYRIGHT.md). Repo hiện chưa khai báo open-source license riêng; không mặc định có quyền sử dụng lại ngoài quyền được nêu trong file đó hoặc thỏa thuận bằng văn bản.
