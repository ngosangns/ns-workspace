---
name: lsp-code-graph
description: Dùng khi cần tìm symbol, entry point, caller/callee, references hoặc code graph context bằng Search/Code Graph dựa trên LSP của ns-workspace. Trigger khi user nhắc code graph, LSP graph, search graph, caller/callee, symbol references, hoặc khi cần hiểu quan hệ code trước khi grep sâu.
---

# LSP Code Graph Search

Dùng skill này để lấy bối cảnh code graph có cấu trúc trước khi inspect raw files. Command dùng cùng search backend với Preview web, nên kết quả gồm Docs Semantic, Docs Graph, Code Semantic và Code Graph; phần Code Graph lấy symbol và quan hệ từ language server khi có.

## Workflow

1. Chạy query non-interactive bằng lệnh `graph` từ checkout local của `ns-workspace`, rồi trỏ `--project` sang repo cần inspect:

   ```sh
   cd /Users/ngosangns/Github/ns-workspace
   go run . graph --project /path/to/project --query "<symbol-or-concept>" --json
   ```

   Khi đang inspect chính checkout `ns-workspace`, `--project .` là đủ. Không dùng `go run github.com/ngosangns/ns-workspace@latest` cho workflow này cho tới khi bản module đã publish query flags.

2. Nếu `graph` báo không có `--query`/`--json`, command đang chạy không phải bản local mới. Không fallback qua Search UI/API; chuyển về checkout `ns-workspace` rồi chạy lại `go run . graph --query`.
3. Đọc `warnings` trước. Nếu command báo thiếu language server hoặc relation expansion không khả dụng, nói rõ fallback và tiếp tục bằng `rg`/code inspection.
4. Ưu tiên `panels.codeGraph` cho symbol, owner/container, caller/callee, references và path:line cần inspect.
5. Dùng `panels.docsGraph` khi câu hỏi cần quan hệ tài liệu/spec/module.
6. Sau khi có path/line, inspect file bằng `sed`, `rg`, hoặc test liên quan. Không kết luận chỉ từ title graph nếu cần hiểu logic chi tiết.

## Language Servers

Code Graph cần language server local. Với TypeScript/JavaScript, binary phải tìm được dưới một trong các vị trí resolver biết như `PATH`, project `node_modules/.bin` hoặc checkout `node_modules/.bin`. Chạy command từ checkout `ns-workspace` giúp resolver thấy `node_modules/.bin/typescript-language-server` của chính checkout này ngay cả khi project được inspect chưa cài language server.

Nếu warning có dạng `typescript-language-server not found in PATH or known local tool locations`, chạy trong repo có `package.json` tương ứng:

```sh
npm install
```

Sau đó kiểm tra:

```sh
node_modules/.bin/typescript-language-server --version
```

## Flags Hữu Ích

```sh
--project PATH
--docs-dir docs
--limit 8
--keyword-op sum
--keyword-op difference
--query "term"
--json
```

`graph` chỉ dành cho query terminal và yêu cầu `--query`. Không dùng browser tools hoặc Search standalone API cho workflow này.
