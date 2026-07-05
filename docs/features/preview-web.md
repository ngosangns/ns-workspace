---
type: feature
title: "Preview Web"
description: "Lệnh `preview` dùng Quartz để build và serve docs dưới dạng digital garden local."
tags: ["feature", "preview-web"]
timestamp: 2026-06-23T00:00:00Z
status: active
compliance: current-state
---

# Preview Web

## Meta

- **Status**: active
- **Description**: Lệnh `preview` dùng Quartz để build và serve docs dưới dạng digital garden local.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Module preview](../modules/preview.md), [Kiến trúc tổng quan](../architecture/overview.md)

## Tổng Quan

Lệnh `preview` chạy một Quartz dev server local để đọc thư mục `docs/` của project. Quartz biến Markdown content thành một website đầy đủ với search, graph/backlinks và routing nội bộ.

Lệnh `search` và `graph` vẫn dùng backend Go riêng (xem [Module preview](../modules/preview.md)); chúng không phụ thuộc Quartz.

## Chạy

```bash
go run . preview --project .
go run . preview --project . --open
```

Lần chạy đầu tiên sẽ clone Quartz vào `~/.cache/ns-workspace/quartz/repo` và chạy `npm install`; cần có Node.js ≥ 22 và npm.

## Hành Vi Chính

- Quartz workspace được tạo trong `~/.cache/ns-workspace/quartz/workspaces/<hash>/`.
- Thư mục `docs/` được symlink/copy vào `content/` của workspace.
- Quartz dev server tự động hot-reload khi file Markdown thay đổi.
- Preview UI, search, graph/backlinks do Quartz cung cấp.

## Quan Hệ

Feature này được implement bởi [Module preview](../modules/preview.md).
