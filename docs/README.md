---
type: reference
title: "Tài Liệu Dự Án"
description: "Cổng vào knowledge base của `ns-workspace`, hướng dẫn điều hướng các tài liệu hiện hành và quy ước duy trì docs."
tags: ["reference"]
timestamp: 2026-06-23T00:00:00Z
status: active
compliance: current-state
---

# Tài Liệu Dự Án

## Meta

- **Status**: active
- **Description**: Cổng vào knowledge base của `ns-workspace`, hướng dẫn điều hướng các tài liệu hiện hành và quy ước duy trì docs.
- **Compliance**: current-state
- **Links**: [Chỉ mục](./_index.md), [Kiến trúc tổng quan](./architecture/overview.md), [Module agentsync](./modules/agentsync.md), [Preview web](./features/preview-web.md), [Module preview](./modules/preview.md), [Module kbmcp](./modules/kbmcp.md), [Module graph query](./modules/graphquery.md), [Aspect inventory](./research/aspect-inventory.md), [Thuật ngữ](./shared/glossary.md)

## Tổng Quan

`ns-workspace` là Go CLI dùng để bootstrap và đồng bộ cấu hình agent cá nhân. Dự án dùng `~/.agents` làm nguồn cấu hình chính, sau đó materialize instructions, skills, subagents, settings, hooks và MCP server presets sang các agent được hỗ trợ.

Knowledge base trong thư mục `docs/` mô tả trạng thái hiện tại của repo. Tài liệu không giữ lịch sử thay đổi; mỗi file chỉ mô tả thiết kế, hành vi và ràng buộc đang đúng ở thời điểm sync.

## Cách Điều Hướng

- Bắt đầu với [Chỉ mục](./_index.md) để xem toàn bộ tài liệu đã được nối graph.
- Đọc [Kiến trúc tổng quan](./architecture/overview.md) để hiểu CLI, adapter sync và preview web.
- Đọc [Aspect inventory](./research/aspect-inventory.md) khi cần onboarding nhanh qua các boundary, source paths và docs gaps chính.
- Đọc [Module agentsync](./modules/agentsync.md) khi sửa `init`, `update`, `status`, `doctor`, `registry`, `agents`, preset materialization hoặc adapter native targets.
- Đọc [Preview web](./features/preview-web.md) và [Module preview](./modules/preview.md) khi làm việc với `go run . preview`, `go run . search` hoặc `go run . export`.
- Đọc [Module kbmcp](./modules/kbmcp.md) khi làm việc với MCP server `go run . mcp`.
- Đọc [Module graph query](./modules/graphquery.md) khi làm việc với `go run . graph`, `go run . lsp` hoặc LSP installer/cache.
- Khi cần triển khai thay đổi lớn, tạo plan mới trong `docs/specs/planning/` trước rồi link plan đó vào chỉ mục sau khi file tồn tại.

## Quy Ước

Tài liệu shipped hoặc mô tả hành vi hiện tại nằm trong `docs/features/`, `docs/modules/`, `docs/architecture/` và `docs/shared/`. Plan chưa hoặc đang triển khai theo từng yêu cầu nằm trong `docs/specs/planning/` khi có file cụ thể. Knowledge base theo **Open Knowledge Format (OKF)**: mọi doc bắt đầu bằng YAML frontmatter với `type` (bắt buộc) cùng `title`/`description`/`tags`/`timestamp` (xem `presets/skills/_shared/templates/frontmatter-schema.md`). Block `## Meta` cũ vẫn được giữ để tương thích ngược và để cung cấp `**Links**` cho preview graph; frontmatter thắng ở key trùng. Dùng link tương đối thật (ưu tiên dạng bundle-relative `/path/doc.md`) để preview graph đi được hai chiều.
