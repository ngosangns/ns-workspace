# Developer Guide

Tài liệu này dành cho việc phát triển `ns-workspace` trong checkout local. Nếu chỉ muốn dùng CLI, đọc [README.md](./README.md).

## Yêu Cầu

- Go 1.22 trở lên, theo `go.mod`.
- Node.js và npm khi sửa preview frontend, chạy markdown/html tooling hoặc build static assets.
- Các agent CLI liên quan chỉ cần có khi muốn kiểm tra adapter bằng `doctor`.

## Cấu Trúc Repo

| Path                               | Vai trò                                                                                                                                                |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `main.go`                          | CLI entrypoint, route nhóm lệnh agentsync, harness, preview/search, graph, export, mcp và lsp.                                                          |
| `internal/cli/`                    | Parse flags và dispatch nhóm lệnh `init`, `update`, `status`, `doctor`, `registry`, `agents`/`catalog`, `harness`.                                       |
| `internal/agentsync/`              | Logic adapter sync, `SyncPlan`, path native của từng agent, backup và operation apply/status/doctor.                                                   |
| `internal/harness/`                | Engine, task registry, evaluator, loop controller, subagent dispatcher, memory store và enrichment task `enrich-docs` (`enrich.go`) cho lệnh `harness`. |
| `internal/preview/`                | Backend scan docs, search/graph API, static export (`export.go` + `export_ui/`), knowledge façade (`knowledge.go`), lệnh `kb` validate/index OKF (`kb.go`), và lệnh `preview` dùng Quartz. |
| `internal/kbmcp/`                  | Command-line truy cập `docs/`: dispatcher (`server.go`) và tool handlers list/lookup/search/modify (`tools.go`).                                       |
| `internal/graphquery/`             | Registry/setup/cache LSP cho Search/LSP Code Graph, CLI `lsp`, installer npm/go/archive và warning dùng chung.                                         |
| `internal/portal/portal_ui_src/`   | Source Vue 3/TypeScript của portal UI.                                                                                                                 |
| `internal/portal/portal_ui/`       | Static build output được Go embed cho portal.                                                                                                          |
| `presets/`                         | Preset embedded cho agents, skills, settings, subagents, registry, OpenCode và MCP servers.                                                            |
| `docs/`                            | Knowledge base hiện trạng của repo, gồm index, sync snapshot, architecture, modules, features, specs.                                                  |

## Chạy CLI Local

```bash
go run . status
go run . doctor
go run . agents
go run . init --dry-run
go run . preview --project . --open
go run . graph --project . --query buildPreviewSearchResponse --json
go run . export --project . --out ./kb.html
go run . mcp --project . list-docs
go run . lsp list --project .
go run . harness list
go run . harness run --task <id> --project . --dry-run
```

Lệnh `preview` dùng Quartz để build và serve docs dưới dạng digital garden. Lần chạy đầu tiên sẽ clone Quartz vào `~/.cache/ns-workspace/quartz/repo` và chạy `npm install`; cần Node.js ≥ 22 và npm. Các lần sau dùng cache. Quartz dev server có hot-reload riêng; `--no-reload` không còn tác dụng với preview nhưng vẫn được chấp nhận để tương thích.

Lệnh `search` sinh HTML launcher vào current working directory, start local API server và mở Search standalone từ static entry `search.html`. Dùng `--no-open` khi kiểm tra command trong script hoặc test thủ công mà không muốn mở browser. Lệnh `graph --query <text> --json` chạy cùng Search/LSP Code Graph pipeline ở chế độ terminal non-interactive; chế độ này không sinh launcher, tự ensure LSP theo mặc định và phải giữ JSON sạch trên stdout. Lệnh `export` ghi một file HTML tĩnh self-contained (docs + graph nhúng client-side) qua knowledge core dùng chung, validate docs dir trước khi ghi và fail-open khi một doc render lỗi. Lệnh `mcp` chạy các command một lần để truy vấn `docs/` và trả JSON ra stdout, không còn là server persistent. Các command hiện có: `list-docs`, `lookup-doc`, `search-docs`. Lệnh `lsp list/install` đi qua `internal/graphquery` để quản lý language server cache dùng cho Code Graph.

## Test Và Validation

Go:

```bash
go test ./...
go test ./internal/preview
go test ./internal/graphquery
go test ./internal/cli
go test ./internal/agentsync
go test ./internal/harness
go test ./internal/kbmcp
```

Serve và lint:

```bash
# Serve
npm install
task ns:portal
task ns:preview

# Lint (đã bao gồm cả format check; :fix sẽ format và auto-fix)
task lint:portal
task lint:portal:fix
task lint:preview
task lint:preview:fix
task lint:doc
task lint:doc:fix
```

`npm install` chạy `prepare` để cài Git pre-commit hook bằng `simple-git-hooks`. Hook gọi `lint-staged`, chạy ESLint/Biome/Prettier fix trên file portal đã stage và để lint-staged cập nhật lại staged changes trước khi commit.

Không cần chạy full build chỉ để sửa docs thuần. Với thay đổi nhỏ, chọn validation sát phạm vi thay đổi.

## Workflow Sửa Preset, Adapter Và Harness

1. Dùng `go run . status` để xem output hiện tại.
2. Chạy `go run . init --dry-run` hoặc `go run . update --dry-run` trước khi ghi file user-level.
3. Khi sửa adapter stable trong `internal/agentsync/`, kiểm tra cả path create/update, backup, symlink/copy mode và filter `--tools`.
4. Khi thêm agent mới, cập nhật catalog/support tier, preset materialization và test liên quan.
5. Khi sửa harness trong `internal/harness/`, chạy `go test ./internal/harness/...` và thử `go run . harness run --task <id> --project . --dry-run`.
6. Sau khi chỉnh preset trong `presets/`, kiểm tra command tương ứng bằng `--dry-run` trước khi apply thật.

## Workflow Sửa Preview Web

1. Lệnh `preview` dùng [Quartz](https://quartz.jzhao.xyz/) để build và serve docs. Cấu hình Quartz nằm trong `internal/preview/quartz/`. Nếu đổi giao diện/docs layout, sửa `quartz.config.ts`, `quartz.layout.ts` hoặc theme trong template.
2. Nếu đổi behavior API/search/graph, cập nhật backend trong `internal/preview/` và test tương ứng.
3. Nếu đổi setup/cache/installer LSP, cập nhật `internal/graphquery/`, adapter trong `internal/preview/preview_lsp_setup.go` và test CLI `lsp`/`graph`.
4. Chạy `go run . preview --project .` để kiểm tra Quartz dev server.
5. Chạy test Go liên quan, thường là `go test ./internal/preview`; với LSP setup chạy thêm `go test ./internal/graphquery`.
6. Nếu behavior user-facing đổi, cập nhật docs trong `docs/features/`, `docs/modules/` hoặc `docs/specs/planning/`.

## Quy Ước Docs

Docs trong `docs/` mô tả trạng thái hiện tại, không giữ changelog dài. File quan trọng nên có metadata, khai báo bằng một trong hai cách:

- YAML frontmatter chuẩn OKF ở đầu file (`---`) với `type`, `description`, `tags`, `timestamp` cùng các key tương thích (`status`, `version`, `compliance`, `priority`, `links`).
- Section `## Meta` dạng prose (tương thích ngược) với `**Status**`, `**Description**`, `**Compliance**`, `**Links**`.

Khi một doc có cả hai, frontmatter thắng ở key trùng và `## Meta` điền các field còn trống; frontmatter lỗi cú pháp sẽ fallback sang `## Meta` kèm warning. Parser là permissive consumer: key lạ hoặc link gãy bị bỏ qua, không crash.

`docs/_index.md` là entrypoint graph của knowledge base. `docs/_sync.md` ghi snapshot sync và những thay đổi worktree đã biết. Khi docs đang behind HEAD, xem docs như bối cảnh cần verify lại bằng code.

## Release Và Commit

- Commit message nên theo Conventional Commits, ví dụ `docs: add developer and copyright guides`.
- Với thay đổi preset hoặc adapter, mô tả rõ adapter/tool bị ảnh hưởng.
- Với thay đổi preview UI, ghi rõ đã chạy validation nào, static assets có được rebuild hay không, và bundle JS embed có được cập nhật cùng HTML hay không.

## Rủi Ro Cần Nhớ

- Repo có thể ghi vào user-level config thật; luôn dùng `--dry-run` trước với thay đổi adapter hoặc preset.
- `--force` thay thế file đã tồn tại trong `init`, nên chỉ dùng khi đã đọc diff/backups.
- Preview search có thể dùng embedding runtime local nếu được cấu hình; fallback lexical vẫn phải cho kết quả hợp lý khi embedding không khả dụng.
- Code Graph dựa vào language server cài trong môi trường local hoặc cache `ns-workspace`; resolver kiểm tra `PATH`, Go bin dirs như `GOBIN`/`GOPATH/bin`/`~/go/bin`, local `node_modules/.bin` và cache dirs từ `internal/graphquery`. LSP source scan bỏ generated artifacts và node_modules khỏi index; khi LSP binary thiếu, một file symbol timeout hoặc relation expansion thiếu capability, search phải fail-open bằng warning thay vì làm hỏng preview.
- `graph --query` tự ensure LSP theo mặc định và có thể tải package/archive vào user cache; dùng `--no-ensure-lsp` cho kiểm tra read-only. Preview/Search HTTP không được tự cài LSP trong request.
- `node_modules/` là dữ liệu local, không phải source of truth.
