---
name: lsp-code-graph
description: Dùng khi cần tìm symbol, entry point, caller/callee, references hoặc code graph context bằng Search/Code Graph dựa trên LSP của ns-workspace. Trigger khi user nhắc code graph, LSP graph, search graph, caller/callee, symbol references, hoặc khi cần hiểu quan hệ code trước khi grep sâu.
---

# LSP Code Graph Search

Dùng skill này để lấy bối cảnh code graph có cấu trúc trước khi inspect raw files. Command dùng cùng search backend với Preview web, nên kết quả gồm Docs Semantic, Docs Graph, Code Semantic và Code Graph; phần Code Graph lấy symbol và quan hệ từ language server khi có.

## Workflow

1. Chạy query bằng command `graph --query`:

   ```sh
   go run github.com/ngosangns/ns-workspace@latest graph --project . --query "<symbol-or-concept>" --json
   ```

   Khi đang làm ngay trong checkout `ns-workspace`, dùng:

   ```sh
   go run . graph --project . --query "<symbol-or-concept>" --json
   ```

2. Đọc `warnings` trước. Nếu command báo thiếu language server hoặc relation expansion không khả dụng, nói rõ fallback và tiếp tục bằng `rg`/code inspection.
3. Ưu tiên `panels.codeGraph` cho symbol, owner/container, caller/callee, references và path:line cần inspect.
4. Dùng `panels.docsGraph` khi câu hỏi cần quan hệ tài liệu/spec/module.
5. Sau khi có path/line, inspect file bằng `sed`, `rg`, hoặc test liên quan. Không kết luận chỉ từ title graph nếu cần hiểu logic chi tiết.

## Flags Hữu Ích

```sh
--project PATH
--docs-dir docs
--query "term"
--limit 8
--keyword-op sum
--keyword-op difference
--json
```

Không dùng browser tools cho workflow này. Nếu cần giao diện trực quan, chạy `graph` không có `--query` để mở Search standalone.
