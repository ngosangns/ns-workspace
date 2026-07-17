---
type: feature
title: "Portal Web UI"
description: "Trang web local quản lý skills, MCPs, registry skills và chạy sync cho ns-workspace."
tags: ["feature", "portal", "ui"]
timestamp: 2026-07-17T00:00:00Z
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
| GET    | `/api/skills`                     | Danh sách skills (`enabled`, `source`, optional `registrySource`)      |
| GET    | `/api/skills/registries`          | Unique GitHub registry sources từ Registry overlay (enabled+disabled)  |
| GET    | `/api/skills/catalog?registry=&q=`| List full SKILL.md trong registry (GitHub tree API; filter name)       |
| GET    | `/api/skills/search?q=`           | Search skills.sh (min 2 chars; optional `registry=`)                   |
| POST   | `/api/skills/install`             | Cài skill (`source`+`skill`) vào agents home + upsert registry overlay |
| POST   | `/api/skills/install-batch`       | Cài nhiều skills (tối đa 50)                                           |
| POST   | `/api/skills/uninstall`           | Gỡ skill khỏi `~/.agents/skills/<id>` + xóa entry registry overlay     |
| GET    | `/api/skills/{id}`                | Chi tiết skill                                                         |
| PUT    | `/api/skills/{id}`                | Cập nhật skill                                                         |
| DELETE | `/api/skills/{id}`                | Reset skill về default                                                 |
| POST   | `/api/skills/{id}/enabled`        | Enable/disable skill (`{"enabled":bool}`) → `portal/disabled.json`     |
| GET    | `/api/mcps`                       | MCP catalog: `items[]`, `content` (unified), `disabledServers`         |
| PUT    | `/api/mcps`                       | Ghi **toàn bộ catalog** (`content` hoặc `mcpServers`+`disabled`)       |
| DELETE | `/api/mcps`                       | Reset MCP overlay về embedded default                                  |
| POST   | `/api/mcps/{name}/enabled`        | Enable/disable một MCP trong catalog                                   |
| DELETE | `/api/mcps/{name}`                | Xóa hẳn server khỏi enabled + disabled overlay                         |
| GET    | `/api/mcps/preset`                | Embedded MCP preset (read-only)                                        |
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

UI dùng **page kit** SolidJS chung (`PageHeader`, `UiSegmented`, `EnableSwitch`, `ResourceRow`, `EmptyState`, `ListSkeleton`, `StatusPill`, `PageFeedback`, `SearchInput`) — xem [conventions frontend](../development/conventions/preview-frontend.md).

- **Dashboard**: metrics skills / MCP / registry / adapters (total + enabled/disabled), deep-link tới từng trang, path status `~/.agents`, Sync panel.
- **Skills**: tab **Installed** (registry filter gồm **All**, name filter, enable switch, dialog Markdown) và tab **Discover** (cùng registry filter **All** + từng source, catalog GitHub, multi-select / batch install tối đa 50; skill đã cài có **Reinstall** + **Uninstall**).
- **MCPs**: **card grid** (transport badge, summary command/url, enable switch); search + filter All/Enabled/Disabled; dialog form (stdio / HTTP / SSE) + Raw JSON; Add / edit / Remove; **Advanced** (catalog JSON + embedded preset); Reset overlay.
- **Registry**: list thống nhất (filter, enable switch) + Edit JSON (enabled list); Reset khi custom overlay.
- **Adapters**: list provider (tier, artifacts, docs) + enable switch.
- **Sync Panel**: `status`, `doctor`, `init`, `update`, `registry` + SSE log; filter provider.

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
