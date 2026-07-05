---
type: module
title: "Module Preview"
description: "Tài liệu module `internal/preview`: scan docs, search/graph, export, kb, và lệnh `preview` dùng Quartz để serve docs dưới dạng digital garden."
tags: ["module", "preview"]
timestamp: 2026-06-23T00:00:00Z
status: active
compliance: current-state
---

# Module Preview

## Meta

- **Status**: active
- **Description**: Tài liệu module `internal/preview`: scan docs, search/graph, export, kb, và lệnh `preview` dùng Quartz để serve docs dưới dạng digital garden.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Module graph query](./graphquery.md), [Kiến trúc tổng quan](../architecture/overview.md)

## Tổng Quan

`internal/preview` là knowledge-base backend của `ns-workspace`. Nó scan docs, parse metadata Markdown/HTML, dựng graph, chạy search hybrid, query LSP Code Graph, validate/index OKF và export static bundle. Lệnh `preview` giờ dùng [Quartz](https://quartz.jzhao.xyz/) để build và serve docs như một digital garden static site, thay vì custom Vue SPA cũ.

Preview và portal đã được tách riêng:

- `preview` — docs preview bằng Quartz.
- `portal` — quản lý preset/skills/MCP/registry, không còn docs preview.

## Thành Phần

| File                | Vai trò                                                                            |
| ------------------- | ---------------------------------------------------------------------------------- |
| `preview.go`        | Lệnh `preview`: chuẩn bị Quartz workspace và chạy `npx quartz build --serve`       |
| `quartz.go`         | Quản lý Quartz repo cache, workspace per-project, symlink/copy docs vào `content/` |
| `preview_api.go`    | `PreviewHandler` HTTP API cho `search`/`graph` commands                            |
| `preview_search.go` | Search pipeline hybrid (docs semantic, docs graph, code semantic, code graph)      |
| `spec_project.go`   | Scanner docs, parser metadata OKF, graph builder                                   |
| `graph.go`          | Lệnh `graph` và `search` CLI                                                       |
| `kb.go`             | Lệnh `kb validate` / `kb index`                                                    |
| `export.go`         | Lệnh `export`: self-contained OKF HTML bundle                                      |
| `knowledge.go`      | `Knowledge` façade cho `internal/kbmcp`                                            |
| `preview_lsp*.go`   | LSP Code Graph provider và setup adapter                                           |

## Lệnh `preview` Với Quartz

`go run . preview --project .` thực hiện:

1. Clone Quartz vào `~/.cache/ns-workspace/quartz/repo` (nếu chưa có) và `npm install`.
2. Tạo workspace per-project trong `~/.cache/ns-workspace/quartz/workspaces/<hash>/`.
3. Symlink/copy thư mục `docs/` vào `content/` của workspace.
4. Chạy `npx quartz build --serve --directory <workspace>/content --port <port>` từ repo cache.
5. In URL và mở browser nếu `--open`.

Khi Quartz chưa được clone, lần chạy đầu tiên cần mạng để tải repo và npm dependencies. Các lần sau dùng cache.

Dùng `--quartz-dir <path>` để chỉ đến một Quartz checkout local (có `package.json`) thay vì clone tự động. Điều này hữu ích khi muốn custom theme/plugin hoặc chạy offline sau khi đã chuẩn bị sẵn Quartz.

## Các Lệnh Khác

- `search` — serve `PreviewHandler` API tại `/api/search` và ghi HTML launcher mở tab Search.
- `graph` — query terminal, dùng `buildPreviewSearchResponse` với LSP Code Graph provider, in text/JSON.
- `export` — build self-contained HTML bundle bằng OKF Viewer.
- `kb` — validate/index OKF docs.

## Quy Tắc Nghiệp Vụ

- `preview` yêu cầu Node.js và npm để chạy Quartz.
- `search`/`graph` vẫn dùng backend Go; không phụ thuộc Quartz.
- Portal không còn tab Docs; docs preview là command riêng biệt.
- `Knowledge` façade tiếp tục cung cấp `OpenKnowledge`, `Documents`, `Document`, `Search` cho `internal/kbmcp`.

## Quan Hệ

- Module này consumes docs structure trong [Chỉ mục](../_index.md).
- Tái sử dụng LSP setup/cache từ [Module graph query](./graphquery.md).
- Cung cấp `Knowledge` façade cho [Module kbmcp](./kbmcp.md).
