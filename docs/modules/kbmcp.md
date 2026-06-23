---
type: module
title: "Module kbmcp"
description: "Module `internal/kbmcp` chạy MCP server stdio local-only expose `docs/` knowledge base cho AI agent, với tool list/lookup/search/modify và guard chống path traversal."
tags: ["module", "kbmcp"]
timestamp: 2026-06-23T00:00:00Z
status: active
compliance: current-state
---

# Module kbmcp

## Meta

- **Status**: active
- **Description**: Module `internal/kbmcp` chạy MCP server stdio local-only expose `docs/` knowledge base cho AI agent, với tool list/lookup/search/modify và guard chống path traversal.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Module preview](./preview.md), [Kiến trúc tổng quan](../architecture/overview.md)

## Tổng Quan

`internal/kbmcp` implement một Model Context Protocol (MCP) server local-only, giao tiếp JSON-RPC 2.0 qua stdin/stdout. Server không bao giờ bind network port: nó được spawn như stdio subprocess bởi agent MCP-capable, nên blast radius giới hạn trong docs root của project. Lệnh `mcp` trong `main.go` route tới `kbmcp.Run`.

Module đi qua `Knowledge` façade của `internal/preview` (`OpenKnowledge`) để đọc/search docs, dùng đúng knowledge core chung (`scanSpecProject`, `buildPreviewSearchResponse`) thay vì nhân đôi logic.

## Thành Phần

| File                     | Vai trò                                                                                                                |
| ------------------------ | ---------------------------------------------------------------------------------------------------------------------- |
| `server.go`              | Flag parsing (`Run`), `Server` type, vòng lặp read→dispatch→write JSON-RPC, panic recovery, route method               |
| `tools.go`               | Tool descriptors và handlers `handleListDocs`/`handleLookupDoc`/`handleSearchDocs`/`handleModifyDoc`, `resolveDocPath` |
| `tools_test.go`          | Tests cho tool handlers                                                                                                |
| `resolvedocpath_test.go` | Tests cho guard path traversal                                                                                         |

## Transport JSON-RPC

`Run` parse `--project` và `--docs` (alias `--docs-dir`), dựng `Server` đọc từ `os.Stdin` ghi ra `os.Stdout`. `Serve` chạy vòng lặp decode request → dispatch → encode response tới EOF hoặc context cancel. Mỗi request được dispatch độc lập: panic trong handler được recover thành JSON-RPC error, method/tool lạ trả error response, server không crash. Notification (request không có `id`) không sinh response. Method hỗ trợ: `initialize`, `tools/list`, `tools/call`.

## Tools

| Tool          | Mục đích                                                                                                         |
| ------------- | ---------------------------------------------------------------------------------------------------------------- |
| `list_docs`   | Liệt kê docs trong docs root (id, title, type, tags, path), filter tùy chọn theo `type`/`tag` (case-insensitive) |
| `lookup_doc`  | Trả full content + metadata theo `id`; id không tồn tại trả error rõ ràng, không panic                           |
| `search_docs` | Search bằng cùng pipeline với preview/search (mode `hybrid`, operator `sum`, limit mặc định 8)                   |
| `modify_doc`  | Tạo/sửa doc theo `id` + `content`, chặn path traversal ra ngoài docs root                                        |

## Quy Tắc Nghiệp Vụ

- `search_docs` trả đúng response của `buildPreviewSearchResponse` cho cùng query (single contract); code graph để nil vì façade nhắm docs knowledge base.
- `modify_doc` resolve path qua `resolveDocPath`: từ chối id rỗng, id tuyệt đối và mọi target cleaned thoát khỏi docs root (`../` hoặc khác). Thư mục cha trong docs root được tạo trước khi ghi (0o644).
- Tool args không hợp lệ trả JSON-RPC error và server vẫn tiếp tục phục vụ.
- Server local-only, không bind network và không thêm dependency cloud.

## Quan Hệ

- `main.go` route command `mcp` tới `kbmcp.Run`.
- Đọc/search docs qua `Knowledge` façade của [Module preview](./preview.md).
