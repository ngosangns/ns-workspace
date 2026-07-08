---
type: feature
title: "Portal Web UI"
description: "Trang web local quản lý skills, MCPs, registry skills và chạy sync cho ns-workspace."
tags: ["feature", "portal", "ui"]
timestamp: 2026-07-04T00:00:00Z
status: active
compliance: current-state
---

# Portal Web UI

## Tổng Quan

Portal là giao diện web tích hợp trong `ns-workspace`, chạy local single-user, cho phép xem, chỉnh sửa và chạy sync các preset skills, MCP servers và registry skills. Portal không còn bao gồm docs preview; để xem docs dùng lệnh `preview` (Quartz) riêng biệt.

## Chạy

```bash
go run . portal
```

Flags:

- `--addr HOST:PORT`: địa chỉ bind, mặc định `127.0.0.1:0` (tự chọn port).
- `--open`: tự động mở trình duyệt.
- `--no-reload`: tắt hot reload supervisor khi chạy trong checkout.
- `--agents-home PATH`: thư mục shared agents, mặc định `~/.agents`.

## Kiến Trúc

- **Backend**: module `internal/portal` viết bằng Go, expose REST API và embed static UI.
- **Frontend**: Vue 3 + TypeScript trong `internal/portal/portal_ui_src/`, build ra `internal/portal/portal_ui/` qua Vite.
- **Preset editing**: vì embedded presets là read-only, mọi chỉnh sửa được lưu qua **user-config overlay** (`~/.config/ns-workspace/config.json`) vào thư mục `~/.config/ns-workspace/portal/`. `agentsync.Manager` tự động ưu tiên overlay khi chạy sync.

## API

| Method | Endpoint                     | Mô tả                                             |
| ------ | ---------------------------- | ------------------------------------------------- |
| GET    | `/api/skills`                | Danh sách skills                                  |
| GET    | `/api/skills/{id}`           | Chi tiết skill                                    |
| PUT    | `/api/skills/{id}`           | Cập nhật skill                                    |
| DELETE | `/api/skills/{id}`           | Reset skill về default                            |
| GET    | `/api/mcps`                  | MCP servers                                       |
| PUT    | `/api/mcps`                  | Cập nhật MCP servers                              |
| GET    | `/api/registry`              | Registry skills                                   |
| PUT    | `/api/registry`              | Cập nhật registry skills                          |
| GET    | `/api/adapters`              | Danh sách adapters                                |
| GET    | `/api/status`                | Trạng thái `~/.agents`                            |
| GET    | `/api/config`                | User overlay entries                              |
| POST   | `/api/sync/{command}`        | Bắt đầu sync (init/update/registry/doctor/status) |
| GET    | `/api/sync/stream?jobId=...` | SSE log stream                                    |

## Giao Diện

- **Dashboard**: tổng quan số lượng skills, MCP servers, registry skills, adapters và trạng thái `~/.agents`.
- **Skills**: danh sách skill, editor Markdown dựa trên CodeMirror 6, reset về default.
- **MCPs**: editor JSON dựa trên CodeMirror 6 cho `presets/mcp/servers.json`, có lint JSON inline.
- **Registry**: editor JSON dựa trên CodeMirror 6 cho `presets/registry/skills.json`, có lint JSON inline.
- **Claude**: editor JSON dựa trên CodeMirror 6 chỉ đọc cho preset Claude Code settings.
- **Adapters**: danh sách adapter với tier và artifacts.
- **Sync Panel**: nút chạy `status`, `doctor`, `init`, dry-run `update`, `update`, `registry` với log stream.

Docs preview đã được tách sang lệnh `preview` và không còn trong portal.

## Serve Và Lint

```bash
task ns:portal         # serve portal server
task ns:portal -- --addr 127.0.0.1:8080
npm run check:portal   # type check
npm run lint:portal    # lint (bao gồm format check)
npm run lint:portal:fix
```

Portal không cần build task trong Taskfile; UI static output được build bằng `npm run build:portal` khi cần cập nhật artifact embed.

## Ràng Buộc

- Chỉ bind localhost; không có auth vì local single-user.
- Mọi thay đổi preset đều lưu qua user overlay, không sửa embedded presets.
- Sync chạy bất đồng bộ, log được stream về frontend qua SSE.
