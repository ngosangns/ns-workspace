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
- **Frontend**: Vue 3 + TypeScript + Tailwind CSS v4 trong `internal/portal/portal_ui_src/`, build ra `internal/portal/portal_ui/` qua Vite.
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

## Enable / Disable

Portal hỗ trợ bật/tắt **skills**, **providers (adapters)**, **MCP servers** và **registry skills**.

**Nguyên tắc:** disable = **chuyển sang file disabled JSON riêng**, **không xóa** entry và **không comment** trong file enabled. Portal **luôn liệt kê** mọi item và gắn trạng thái **Enabled / Disabled**.

| Loại      | File enabled (overlay)             | File disabled (overlay)                      | Khi disable                                      | Sync (materialize)                                      |
| --------- | ---------------------------------- | -------------------------------------------- | ------------------------------------------------ | ------------------------------------------------------- |
| MCP       | `presets/mcp/servers.json`         | `presets/mcp/servers.disabled.json` (internal) | Entry disabled trong catalog; UI **một** document | `~/.agents/mcp/servers.json` chỉ ghi server **enabled** |
| Skills    | (SKILL.md không đổi)               | `presets/portal/disabled.json` → `skills[]`  | Thêm skill id vào list; **không** xóa `SKILL.md` | Không cài skill disabled vào `~/.agents/skills`         |
| Providers | (adapter registry không đổi)       | `presets/portal/disabled.json` → `providers[]` | Thêm provider id vào list                      | Không fan-out adapter disabled                          |
| Registry  | `presets/registry/skills.json`     | `presets/registry/skills.disabled.json`      | Entry chuyển sang disabled file (full object)    | Chỉ install skill **enabled**                           |

Shape tham chiếu:

```json
// presets/portal/disabled.json
{ "skills": ["spawn-kimi"], "providers": ["gemini"] }

// presets/mcp/servers.disabled.json
{ "mcpServers": { "kimi": { "command": "npx", "args": ["-y", "kimi-for-claude"] } } }

// presets/registry/skills.disabled.json
{ "skills": [{ "name": "find-skills", "source": "vercel-labs/skills", "skill": "find-skills" }] }
```

Mặc định (không có disabled file): mọi skill, provider, MCP và registry skill đều **enabled**.

**Migration:** overlay cũ dùng `//` comments trong JSONC (`toggles.jsonc`, `servers.json`) vẫn được đọc; lần disable/enable tiếp theo ghi sang file disabled JSON mới.

## Giao Diện

- **Dashboard**: tổng quan số lượng skills, MCP servers, registry skills, adapters và trạng thái `~/.agents`.
- **Skills**: list skill kèm description (frontmatter), toggle On/Off, dialog xem Markdown, reset về default.
- **MCPs**: **một catalog** — List (toggle) + Edit JSON (`mcpServers` + `disabled[]`); không tách preset/file enabled. Save thay thế toàn bộ catalog. Reset về embedded default.
- **Registry**: tab List (toggle per skill) + File (enabled JSON); disable → `skills.disabled.json`.
- **Adapters**: list (không card) adapter với tier, artifacts, docs và toggle enable/disable.
- **Sync Panel**: nút chạy `status`, `doctor`, `init`, `update`, `registry` với log stream (job giữ lại sau khi xong để SSE bắt kịp lệnh nhanh như status/doctor).

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
