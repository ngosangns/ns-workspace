---
name: lsp-code-graph
description: Dùng khi cần tìm symbol, entry point, caller/callee, references hoặc code graph context bằng Search/Code Graph dựa trên LSP của ns-workspace. Trigger khi user nhắc code graph, LSP graph, search graph, caller/callee, symbol references, hoặc khi cần hiểu quan hệ code trước khi grep sâu.
---

# LSP Code Graph Search

Dùng skill này để lấy bối cảnh code graph có cấu trúc trước khi inspect raw files. Command dùng cùng search backend với Preview web, nên kết quả gồm Docs Semantic, Docs Graph, Code Semantic và Code Graph; phần Code Graph lấy symbol và quan hệ từ language server khi có.

## Workflow

1. Chạy query non-interactive bằng lệnh `graph` từ checkout local của `ns-workspace`, rồi trỏ `--project` sang repo cần inspect. `graph --query` tự ensure language server còn thiếu cho các language phát hiện trong project:

   ```sh
   cd ~/path/to/ns-workspace
   go run . graph --project /path/to/project --query "<symbol-or-concept>" --json
   ```

   Khi đang inspect chính checkout `ns-workspace`, `--project .` là đủ. Không dùng `go run github.com/ngosangns/ns-workspace@latest` cho workflow này cho tới khi bản module đã publish query flags.

2. Nếu `graph` báo không có `--query`/`--json` hoặc không tự ensure LSP, command đang chạy không phải bản local mới. Không fallback qua Search UI/API; chuyển về checkout `ns-workspace` rồi chạy lại `go run . graph --query`.
3. Đọc `warnings` trước. Nếu command báo install fail, thiếu prerequisite hoặc relation expansion không khả dụng, nói rõ fallback và tiếp tục bằng `rg`/code inspection.
4. Ưu tiên `panels.codeGraph` cho symbol, owner/container, caller/callee, references và path:line cần inspect.
5. Dùng `panels.docsGraph` khi câu hỏi cần quan hệ tài liệu/spec/module.
6. Sau khi có path/line, inspect file bằng `sed`, `rg`, hoặc test liên quan. Không kết luận chỉ từ title graph nếu cần hiểu logic chi tiết.

## Language Servers

Code Graph cần language server local. Resolver tìm trong `PATH`, Go bin dirs, project `node_modules/.bin`, checkout `node_modules/.bin` và cache của `ns-workspace`. Cache mặc định nằm dưới `os.UserCacheDir()/ns-workspace/lsp`; có thể override bằng `NS_WORKSPACE_LSP_CACHE`.

Language coverage hiện có:

- HTML: `vscode-html-language-server --stdio`
- CSS/SCSS/Sass: `vscode-css-language-server --stdio`
- JavaScript/TypeScript: `typescript-language-server --stdio`
- Go/Golang: `gopls serve`
- Kotlin: `kotlin-lsp --stdio`

Kiểm tra trạng thái:

```sh
go run . lsp list --project /path/to/project
```

Cài thủ công nếu muốn chuẩn bị trước hoặc nếu auto ensure báo prerequisite/install failure:

```sh
go run . lsp install auto --project /path/to/project
go run . lsp install html
go run . lsp install css
go run . lsp install typescript
go run . lsp install go
go run . lsp install kotlin
```

Aliases được chấp nhận: `scss`/`sass` map về CSS server, `javascript`/`js` map về TypeScript server, `golang` map về Go server và `kt` map về Kotlin. Kotlin hiện có resolver và warning/install guide, nhưng `lsp install kotlin` không tự tải binary; cài `kotlin-lsp` thủ công rồi chạy lại `lsp list` hoặc `graph --query`.

## Flags Hữu Ích

```sh
--project PATH
--docs-dir docs
--limit 8
--keyword-op sum
--keyword-op difference
--ensure-lsp
--no-ensure-lsp
--query "term"
--json
```

`graph` chỉ dành cho query terminal và yêu cầu `--query`. Dùng `--no-ensure-lsp` khi cần cấm network/install side effect. Không dùng browser tools hoặc Search standalone API cho workflow này.
