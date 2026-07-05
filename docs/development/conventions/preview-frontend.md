---
type: development
title: "Quy Ước Frontend Preview (Deprecated)"
description: "Quy ước cũ cho custom preview frontend. Lệnh preview hiện dùng Quartz, convention này không còn áp dụng."
tags: ["development", "preview-frontend", "deprecated"]
timestamp: 2026-06-23T00:00:00Z
status: deprecated
compliance: current-state
---

# Quy Ước Frontend Preview (Deprecated)

## Meta

- **Status**: deprecated
- **Description**: Quy ước cũ cho custom preview frontend. Lệnh `preview` hiện dùng Quartz, convention này không còn áp dụng.
- **Compliance**: current-state
- **Links**: [Module preview](../../modules/preview.md), [Preview web](../../features/preview-web.md)

## Ghi Chú

Custom Vue/TypeScript preview frontend trong `internal/preview/preview_ui_src/` đã bị xoá. Lệnh `preview` giờ dùng [Quartz](https://quartz.jzhao.xyz/) để build và serve docs.

Portal frontend vẫn tồn tại trong `internal/portal/portal_ui_src/` và dùng Vue 3 + Quasar. Dùng `task ns:portal` để serve, `task lint:portal` / `task lint:portal:fix` để lint (đã bao gồm format). Docs preview dùng `task ns:preview`; lint docs qua `task lint:preview` (chỉ `docs/`) hoặc `task lint:doc` (toàn bộ docs + README + presets).
