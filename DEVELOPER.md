# Developer Guide

Tài liệu này dành cho việc phát triển `ns-workspace` trong checkout local. Nếu chỉ muốn dùng CLI, đọc [README.md](./README.md).

## Yêu Cầu

- Go 1.22 trở lên, theo `go.mod`.
- Node.js và npm khi sửa preview frontend, chạy markdown/html tooling hoặc build static assets.
- Các agent CLI liên quan chỉ cần có khi muốn kiểm tra adapter bằng `doctor`.

## Cấu Trúc Repo

| Path                               | Vai trò                                                                                                                                               |
| ---------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `main.go`                          | CLI entrypoint, parse lệnh `init`, `update`, `status`, `doctor`, `registry`, `agents`, `preview`, `graph`.                                            |
| `internal/agentsync/`              | Logic adapter sync, path native của từng agent, backup và operation apply/status/doctor.                                                              |
| `internal/preview/`                | Backend preview docs, API, search, graph và hot reload supervisor.                                                                                    |
| `internal/preview/preview_ui_src/` | Source Vue 3/TypeScript của preview UI.                                                                                                               |
| `internal/preview/preview_ui/`     | Static build output được Go embed; `index.html`, `search.html`, `style.css`, `favicon.svg` và bundle JS hashed là artifact release của preview/graph. |
| `presets/`                         | Preset embedded cho agents, skills, settings, subagents, registry, OpenCode và MCP servers.                                                           |
| `docs/`                            | Knowledge base hiện trạng của repo, gồm index, sync snapshot, architecture, modules, features, specs.                                                 |

## Chạy CLI Local

```bash
go run . status
go run . doctor
go run . agents
go run . init --dry-run
go run . preview --project . --open
go run . graph --project .
```

Khi chạy preview từ chính checkout này, supervisor sẽ build frontend bằng `npm run build:preview`, giữ một port ổn định rồi restart child process khi source thay đổi. Dùng `--no-reload` khi cần chạy server trực tiếp bằng static assets hiện có. Khi chạy `go run github.com/ngosangns/ns-workspace@latest preview` từ project khác, preview dùng static UI đã embed trong module và không chạy Node/npm ở runtime.

Lệnh `graph` sinh HTML launcher vào current working directory, start local API server và mở Search standalone từ static entry `search.html`. Dùng `--no-open` khi kiểm tra command trong script hoặc test thủ công mà không muốn mở browser.

## Test Và Validation

Go:

```bash
go test ./...
go test ./internal/preview
go test ./internal/agentsync
```

Preview frontend:

```bash
npm install
npm run check:preview
npm run lint:preview
npm run build:preview
```

Docs và markdown:

```bash
npm run lint:docs
npm run format:docs:check
```

Format khi cần:

```bash
gofmt -w main.go internal
npm run format:docs
npm run format:preview
```

Không cần chạy full build chỉ để sửa docs thuần. Với thay đổi nhỏ, chọn validation sát phạm vi thay đổi.

## Workflow Sửa Preset Và Adapter

1. Dùng `go run . status` để xem output hiện tại.
2. Chạy `go run . init --dry-run` hoặc `go run . update --dry-run` trước khi ghi file user-level.
3. Khi sửa adapter stable trong `internal/agentsync/`, kiểm tra cả path create/update, backup, symlink/copy mode và filter `--tools`.
4. Khi thêm agent mới, cập nhật catalog/support tier, preset materialization và test liên quan.
5. Sau khi chỉnh preset trong `presets/`, kiểm tra command tương ứng bằng `--dry-run` trước khi apply thật.

## Workflow Sửa Preview Web

1. Sửa source trong `internal/preview/preview_ui_src/`.
2. Nếu đổi behavior API/search/graph, cập nhật backend trong `internal/preview/` và test tương ứng.
3. Chạy `npm run check:preview`, `npm run lint:preview` và `npm run build:preview`.
4. Review static output trong `internal/preview/preview_ui/`; nếu hashed JS filename đổi, đảm bảo file mới trong `assets/` được track cùng `index.html` hoặc `search.html`.
5. Chạy test Go liên quan, thường là `go test ./internal/preview`.
6. Nếu behavior user-facing đổi, cập nhật docs trong `docs/features/`, `docs/modules/` hoặc `docs/specs/planning/`.

## Quy Ước Docs

Docs trong `docs/` mô tả trạng thái hiện tại, không giữ changelog dài. File quan trọng nên có `## Meta` với:

- `**Status**`
- `**Description**`
- `**Compliance**`
- `**Links**`

`docs/_index.md` là entrypoint graph của knowledge base. `docs/_sync.md` ghi snapshot sync và những thay đổi worktree đã biết. Khi docs đang behind HEAD, xem docs như bối cảnh cần verify lại bằng code.

## Release Và Commit

- Commit message nên theo Conventional Commits, ví dụ `docs: add developer and copyright guides`.
- Với thay đổi preset hoặc adapter, mô tả rõ adapter/tool bị ảnh hưởng.
- Với thay đổi preview UI, ghi rõ đã chạy validation nào, static assets có được rebuild hay không, và bundle JS embed có được cập nhật cùng HTML hay không.

## Rủi Ro Cần Nhớ

- Repo có thể ghi vào user-level config thật; luôn dùng `--dry-run` trước với thay đổi adapter hoặc preset.
- `--force` thay thế file đã tồn tại trong `init`, nên chỉ dùng khi đã đọc diff/backups.
- Preview search có thể dùng embedding runtime local nếu được cấu hình; fallback lexical vẫn phải cho kết quả hợp lý khi embedding không khả dụng.
- Code Graph dựa vào language server cài trong môi trường local; resolver kiểm tra `PATH`, Go bin dirs như `GOBIN`/`GOPATH/bin`/`~/go/bin` và local `node_modules/.bin`. Khi LSP binary thiếu hoặc không hỗ trợ call hierarchy/references, search phải fail-open bằng warning thay vì làm hỏng preview.
- `node_modules/` là dữ liệu local, không phải source of truth.
