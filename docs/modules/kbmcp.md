---
type: module
title: "Module kbmcp"
description: "Module `internal/kbmcp` cung cấp command-line truy cập docs knowledge base: list-docs, lookup-doc, search-docs. Không còn là MCP server persistent, mỗi lần chạy là một command một lần."
tags: ["module", "kbmcp"]
timestamp: 2026-06-23T00:00:00Z
status: active
compliance: current-state
---

# Module kbmcp

## Meta

- **Status**: active
- **Description**: Module `internal/kbmcp` cung cấp command-line truy cập docs knowledge base: `list-docs`, `lookup-doc`, `search-docs`. Không còn là MCP server persistent, mỗi lần chạy là một command một lần.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Module preview](./preview.md), [Kiến trúc tổng quan](../architecture/overview.md)

## Tổng Quan

`internal/kbmcp` expose `docs/` knowledge base dưới dạng các CLI command. Mỗi command chạy một lần, in kết quả JSON ra stdout, rồi thoát ngay. Không có vòng lặp server, không bind network port, không có process chạy nền. `Lệnh `mcp`trong`main.go`route tới`kbmcp.Run`.

Module đi qua `Knowledge` façade của `internal/preview` (`OpenKnowledge`) để đọc/search docs, dùng đúng knowledge core chung (`scanSpecProject`, `buildPreviewSearchResponse`) thay vì nhân đôi logic.

## Thành Phần

| File                     | Vai trò                                                                                                 |
| ------------------------ | ------------------------------------------------------------------------------------------------------- |
| `server.go`              | Flag parsing (`Run`), `Server` context, dispatch subcommand sang handler, in JSON ra stdout             |
| `tools.go`               | Tool handlers `handleListDocs`/`handleLookupDoc`/`handleSearchDocs`/`handleModifyDoc`, `resolveDocPath` |
| `tools_test.go`          | Tests cho tool handlers                                                                                 |
| `resolvedocpath_test.go` | Tests cho guard path traversal                                                                          |

## Commands

| Command       | Flags                   | Mục đích                                                                                                         |
| ------------- | ----------------------- | ---------------------------------------------------------------------------------------------------------------- |
| `list-docs`   | `[--type T] [--tag G]`  | Liệt kê docs trong docs root (id, title, type, tags, path), filter tùy chọn theo `type`/`tag` (case-insensitive) |
| `lookup-doc`  | `--id ID`               | Trả full content + metadata theo `id`; id không tồn tại trả error rõ ràng, không panic                           |
| `search-docs` | `--query Q [--limit N]` | Search bằng cùng pipeline với preview/search (mode `hybrid`, operator `sum`, limit mặc định 8)                   |

Global flags cho mọi command:

- `--project PATH` — project root, mặc định current working directory.
- `--docs PATH` / `--docs-dir PATH` — docs directory, mặc định `docs`.

## Ví dụ Usage

```bash
# Liệt kê tất cả docs trong project hiện tại
go run . mcp list-docs

# Lọc theo type, chỉ định project
go run . mcp --project /path/to/project list-docs --type module

# Đọc một doc
go run . mcp --project /path/to/project lookup-doc --id modules/preview.md

# Search docs
go run . mcp --project /path/to/project search-docs --query "preview search" --limit 4
```

## Quy Tắc Nghiệp Vụ

- `search_docs` trả đúng response của `buildPreviewSearchResponse` cho cùng query (single contract); code graph để nil vì façade nhắm docs knowledge base.
- `modify_doc` không được expose qua CLI command để giảm blast radius (không có server giới hạn). Nếu cần ghi doc, dùng `kb modify` hoặc viết file trực tiếp.
- Tool args không hợp lệ trả error ra stderr với exit code != 0.
- Không bind network và không thêm dependency cloud.

## Quan Hệ

- `main.go` route command `mcp` tới `kbmcp.Run`.
- Đọc/search docs qua `Knowledge` façade của [Module preview](./preview.md).
