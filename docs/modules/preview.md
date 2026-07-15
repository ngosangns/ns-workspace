---
type: module
title: "Module Preview"
description: "Tài liệu module `internal/preview`: scan docs, search/graph, export, kb, và lệnh `preview` serve SolidJS SPA + PreviewHandler."
tags: ["module", "preview"]
timestamp: 2026-07-15T00:00:00Z
status: active
compliance: current-state
---

# Module Preview

## Meta

- **Status**: active
- **Description**: Tài liệu module `internal/preview`: scan docs, search/graph, export, kb, và lệnh `preview` serve SolidJS SPA + PreviewHandler.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Module graph query](./graphquery.md), [Kiến trúc tổng quan](../architecture/overview.md)

## Tổng Quan

`internal/preview` là knowledge-base backend của `ns-workspace`. Nó scan docs, parse metadata Markdown/HTML, dựng graph, chạy search hybrid, query LSP Code Graph, validate/index OKF và export static bundle. Lệnh `preview` serve **SolidJS SPA** embed kèm REST/SSE API (`PreviewHandler`).

Preview và portal tách riêng:

- `preview` — docs SPA + API.
- `portal` — quản lý preset/skills/MCP/registry (SolidJS).

## Thành Phần

| File / path              | Vai trò                                                                            |
| ------------------------ | ---------------------------------------------------------------------------------- |
| `preview.go`             | Lệnh `preview`: HTTP server SPA embed + API                                        |
| `preview_ui_src/`        | Source SolidJS preview SPA                                                         |
| `preview_ui/`            | Build artifact embed                                                               |
| `preview_api.go`         | `PreviewHandler` HTTP API                                                          |
| `preview_search.go`      | Search pipeline hybrid                                                             |
| `export.go` + `export_ui/` | Export OKF HTML; viewer Solid build (`export_ui_src` → `viz.js`)                 |
| `graph.go`               | Lệnh `graph` / `search` CLI                                                        |
| `kb.go` / `knowledge.go` | OKF validate/index và façade kbmcp                                                 |
| `quartz.go`              | Legacy helpers (không còn dùng bởi `preview` default)                              |

## Lệnh `preview`

`go run . preview --project .`:

1. `NewPreviewHandler` + register `/api/*`.
2. Serve embed `preview_ui` SPA tại `/`.
3. In URL và mở browser nếu `--open`.

## Export

`go run . export` ghi một file HTML self-contained. Viewer client là SolidJS IIFE (`viz.js`) + CSS; Cytoscape/marked vendor khi `--inline-assets`.

## Quy Tắc Nghiệp Vụ

- `search`/`graph` dùng backend Go; không phụ thuộc runtime ngoài trừ embeddings optional.
- Portal không còn tab Docs; docs preview là command riêng.
- `Knowledge` façade tiếp tục cung cấp API cho `internal/kbmcp`.
