---
type: feature
title: "Preview Web"
description: "Lệnh `preview` serve SolidJS SPA local để đọc docs, search hybrid và graph qua PreviewHandler."
tags: ["feature", "preview-web"]
timestamp: 2026-07-15T00:00:00Z
status: active
compliance: current-state
---

# Preview Web

## Meta

- **Status**: active
- **Description**: Lệnh `preview` serve SolidJS SPA local để đọc docs, search hybrid và graph qua PreviewHandler.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Module preview](../modules/preview.md), [Kiến trúc tổng quan](../architecture/overview.md)

## Tổng Quan

Lệnh `preview` chạy HTTP server local (Go) với:

1. **SolidJS SPA** embed (`internal/preview/preview_ui/`) — browse docs, search, graph.
2. **PreviewHandler API** dưới `/api/` — project, docs, files, graph, search, SSE events.

Lệnh `search` mở launcher trỏ tới `#/search` trên cùng stack server. Lệnh `graph` vẫn là terminal query.

## Chạy

```bash
go run . preview --project .
go run . preview --project . --open
```

## Hành Vi Chính

- SPA routes (hash): `/` docs list/detail, `/search`, `/graph`.
- Hot reload docs qua SSE `/api/events` (client có thể soft-reload project).
- Không clone Quartz; flag `--quartz-dir` bị deprecate (warn + ignore).
- Frontend source: `internal/preview/preview_ui_src/` (SolidJS + TypeScript 7 + Tailwind); build: `npm run build:preview`.

## Quan Hệ

Feature này được implement bởi [Module preview](../modules/preview.md).
