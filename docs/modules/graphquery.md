# Module Graph Query

## Meta

- **Status**: active
- **Description**: Tài liệu module `internal/graphquery`, mô tả registry/setup/cache LSP dùng cho Search/LSP Code Graph và CLI `lsp`.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Kiến trúc tổng quan](../architecture/overview.md), [Module preview](./preview.md), [Preview web](../features/preview-web.md), [Tự động cài LSP cho Graph Query](../specs/planning/auto-install-lsp-for-graph.md), [Mở rộng LSP coverage](../specs/planning/expand-lsp-language-coverage.md), [Tự động ensure LSP khi query graph](../specs/planning/auto-ensure-lsp-on-graph-query.md)

## Tổng Quan

`internal/graphquery` sở hữu phần quản lý language server cho Search/LSP Code Graph. Package này chứa registry language/server, CLI `lsp list/install`, resolver cache, installer npm/go/archive và warning text dùng chung. `internal/preview` không sở hữu auto-install LSP; preview chỉ cung cấp detector source file qua interface `SourceDetector` và tái sử dụng API của graph query cho `graph --query`, `lsp` CLI và warning fail-open trong Preview/Search.

## API Và Boundary

- `RunLSP(args, detector)` là entrypoint CLI cho nhóm lệnh `lsp`.
- `RunLSPList()` và `RunLSPInstall()` trả output text hoặc JSON ổn định cho automation/test.
- `SourceDetector` là interface để luồng chính truyền detection từ project source files sang graph query mà không kéo dependency ngược từ `internal/graphquery` sang `internal/preview`.
- `LanguageSpecs()`, `InstallSpecs()`, `InstallSpecByID()` và `InstallSpecByServerID()` là registry lookup cho runtime preview và installer.
- `InstallProjectLSPs()` và `EnsureProjectLSP()` cài các server còn thiếu theo detected install IDs, trả `InstallResult` hoặc warnings thay vì làm query fail cứng.
- `CacheCommandDirs()`, `ResolveCommandWithSource()` và `CommandSource()` expose resolver/cache layout để preview LSP runtime tìm binary đã cài mà không hardcode đường dẫn cache.
- `UnavailableWarning()` tạo warning thống nhất cho HTTP Preview/Search và CLI graph khi language server thiếu hoặc không hỗ trợ relation expansion.

## LSP Implementations

Mỗi server LSP có implementation riêng, cùng implement interface `lspImplementation` nội bộ:

- HTML: `vscode-html-language-server --stdio`, cài qua `vscode-langservers-extracted`.
- CSS/SCSS/Sass: `vscode-css-language-server --stdio`, cài qua `vscode-langservers-extracted`.
- JavaScript/TypeScript: `typescript-language-server --stdio`, cài qua npm package `typescript-language-server` và `typescript`.
- Go/Golang: `gopls serve`, cài bằng `GOBIN=<cache>/go/bin go install golang.org/x/tools/gopls@latest`.
- Kotlin: `kotlin-lsp --stdio`, cài bằng JetBrains Kotlin LSP standalone archive đã pin version/checksum theo OS/arch, extract vào cache versioned và tạo wrapper `<cache>/kotlin/bin/kotlin-lsp`.

Alias install hiện có: `scss`/`sass` map về CSS server, `javascript`/`js` và `ts`/`tsx` map về TypeScript server, `golang` map về Go server, `kt` map về Kotlin.

## Cache Và Side Effects

LSP install luôn ghi vào cache user của `ns-workspace`, mặc định là `os.UserCacheDir()/ns-workspace/lsp`, hoặc override bằng `NS_WORKSPACE_LSP_CACHE`. Installer không mutate project được inspect: không sửa `package.json`, không thêm devDependency và không ghi artifact vào repo target.

Resolver ưu tiên binary đã có trong môi trường trước cache: `PATH`, project/check-out `node_modules/.bin`, Go bin dirs như `GOBIN`, `GOPATH/bin` và `~/go/bin`. Cache của `ns-workspace` là fallback ổn định cho agent workflow và GUI-launched process thiếu `PATH`.

## Failure Modes

`graph --query` tự ensure LSP theo mặc định, nhưng mọi lỗi prerequisite, download, checksum, extract, binary check hoặc thiếu capability LSP đều được đưa vào warnings và query vẫn fail-open. Dùng `--no-ensure-lsp` khi cần cấm network/install side effect.

Preview/Search HTTP không tự cài package trong request. Khi server thiếu, các panel Docs Semantic, Docs Graph và Code Semantic vẫn hoạt động; Code Graph có thể rỗng và response có warning kèm command `go run . lsp install <language>`.

JSON output của `lsp` và `graph --json` phải giữ stdout sạch. Progress hoặc install status không thuộc JSON phải đi qua stderr.

## Quan Hệ

`internal/graphquery` được gọi từ `main.go` cho nhóm lệnh `lsp`, từ `internal/preview/graph.go` qua adapter để ensure trước graph query, và từ `internal/preview/preview_lsp.go` để runtime resolver biết cache dirs. Thiết kế preview/search UI vẫn nằm trong [Module preview](./preview.md); hành vi user-facing nằm trong [Preview web](../features/preview-web.md).
