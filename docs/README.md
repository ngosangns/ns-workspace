# Tài Liệu Dự Án

## Meta

- **Status**: active
- **Description**: Cổng vào knowledge base của `ns-workspace`, hướng dẫn điều hướng các tài liệu hiện hành và quy ước duy trì docs.
- **Compliance**: current-state
- **Links**: [Chỉ mục](./_index.md), [Kiến trúc tổng quan](./architecture/overview.md), [Preview web](./features/preview-web.md), [Module preview](./modules/preview.md), [Thuật ngữ](./shared/glossary.md)

## Tổng Quan

`ns-workspace` là Go CLI dùng để bootstrap và đồng bộ cấu hình agent cá nhân. Dự án dùng `~/.agents` làm nguồn cấu hình chính, sau đó materialize instructions, skills, subagents, settings, hooks và MCP server presets sang các agent được hỗ trợ.

Knowledge base trong thư mục `docs/` mô tả trạng thái hiện tại của repo. Tài liệu không giữ lịch sử thay đổi; mỗi file chỉ mô tả thiết kế, hành vi và ràng buộc đang đúng ở thời điểm sync.

## Cách Điều Hướng

- Bắt đầu với [Chỉ mục](./_index.md) để xem toàn bộ tài liệu đã được nối graph.
- Đọc [Kiến trúc tổng quan](./architecture/overview.md) để hiểu CLI, adapter sync và preview web.
- Đọc [Preview web](./features/preview-web.md) và [Module preview](./modules/preview.md) khi làm việc với `go run . preview`.
- Đọc các plan trong [specs/planning](./specs/planning/) trước khi triển khai những thay đổi lớn.

## Quy Ước

Tài liệu shipped hoặc mô tả hành vi hiện tại nằm trong `docs/features/`, `docs/modules/`, `docs/architecture/` và `docs/shared/`. Plan chưa hoặc vừa triển khai theo từng yêu cầu nằm trong `docs/specs/planning/`. Mọi file quan trọng phải có `## Meta` với các field `**Status**`, `**Compliance**` và `**Links**`, dùng link tương đối thật tới tài liệu liên quan để preview graph đi được hai chiều.
