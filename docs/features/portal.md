---
type: feature
title: "Portal Web UI"
description: "Trang web local quản lý skills, MCPs, registry skills và chạy sync cho ns-workspace."
tags: ["feature", "portal", "ui"]
timestamp: 2026-07-15T00:00:00Z
status: active
compliance: current-state
---

# Portal Web UI

## Tổng Quan

Portal là giao diện web tích hợp trong `ns-workspace`, chạy local single-user, cho phép xem, chỉnh sửa và chạy sync các preset skills, MCP servers và registry skills. Portal không bao gồm docs preview; để xem docs dùng lệnh `preview` (SolidJS SPA + PreviewHandler).

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
- **Frontend**: SolidJS + TypeScript 7 + Tailwind CSS v4 trong `internal/portal/portal_ui_src/`, build ra `internal/portal/portal_ui/` qua Vite (`vite-plugin-solid`).
- **Preset editing**: vì embedded presets là read-only, mọi chỉnh sửa được lưu qua **user-config overlay** (`~/.config/ns-workspace/config.json`) vào thư mục `~/.config/ns-workspace/portal/`. `agentsync.Manager` tự động ưu tiên overlay khi chạy sync.

## API

| Method | Endpoint                          | Mô tả                                                                  |
| ------ | --------------------------------- | ---------------------------------------------------------------------- |
| GET    | `/api/skills`                     | Danh sách skills (`enabled` flag)                                      |
| GET    | `/api/skills/{id}`                | Chi tiết skill                                                         |
| PUT    | `/api/skills/{id}`                | Cập nhật skill                                                         |
| DELETE | `/api/skills/{id}`                | Reset skill về default                                                 |
| POST   | `/api/skills/{id}/enabled`        | Enable/disable skill (`{"enabled":bool}`) → `portal/disabled.json`     |
| GET    | `/api/mcps`                       | MCP catalog: `items[]`, `content` (unified), `disabledServers`         |
| PUT    | `/api/mcps`                       | Ghi **toàn bộ catalog** (`content` hoặc `mcpServers`+`disabled`)       |
| DELETE | `/api/mcps`                       | Reset MCP overlay về embedded default                                  |
| POST   | `/api/mcps/{name}/enabled`        | Enable/disable một MCP trong catalog                                   |
| GET    | `/api/registry`                   | Registry skills + disabled + items                                     |
| PUT    | `/api/registry`                   | Cập nhật registry skills (removed → `skills.disabled.json`)            |
| DELETE | `/api/registry`                   | Reset registry overlay (enabled + disabled)                            |
| POST   | `/api/registry/{name}/enabled`    | Enable/disable registry skill by name                                  |
| GET    | `/api/adapters`                   | Danh sách adapters (`enabled` flag)                                    |
| POST   | `/api/adapters/{id}/enabled`      | Enable/disable provider → `portal/disabled.json`                       |
| GET    | `/api/status`                     | Trạng thái `~/.agents`                                                 |
| GET    | `/api/config`                     | User overlay entries                                                   |
| POST   | `/api/sync/{command}`             | Bắt đầu sync (init/update/registry/doctor/status)                      |
| GET    | `/api/sync/stream?jobId=...`      | SSE log stream                                                         |

## Giao Diện

- **Dashboard**: tổng quan số lượng skills, MCP servers, registry skills, adapters và trạng thái `~/.agents`.
- **Skills**: list skill kèm description, toggle On/Off, dialog xem Markdown, reset về default.
- **MCPs**: một catalog — List (toggle) + Edit JSON; Save thay thế toàn bộ catalog. Reset về embedded default.
- **Registry**: tab List (toggle per skill) + File (enabled JSON).
- **Adapters**: list adapter với tier, artifacts, docs và toggle enable/disable.
- **Sync Panel**: nút chạy `status`, `doctor`, `init`, `update`, `registry` với log stream SSE.

## Serve Và Lint

```bash
task ns:portal         # serve portal server
npm run check:portal   # type check (tsc TypeScript 7)
npm run lint:portal
npm run build:portal   # rebuild embed artifact
```

## Ràng Buộc

- Chỉ bind localhost; không có auth vì local single-user.
- Mọi thay đổi preset đều lưu qua user overlay, không sửa embedded presets.
- Sync chạy bất đồng bộ, log được stream về frontend qua SSE.
