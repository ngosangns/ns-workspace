---
type: module
title: "Module Agentsync"
description: "Tài liệu module `internal/agentsync`, mô tả sync plan, adapter sync, preset materialization, managed operations, registry skills, native targets và safety rules."
tags: ["module", "agentsync"]
timestamp: 2026-07-17T00:00:00Z
status: active
compliance: current-state
---

# Module Agentsync

## Meta

- **Status**: active
- **Description**: Tài liệu module `internal/agentsync`, mô tả sync plan, adapter sync, preset materialization, managed operations, registry skills, native targets và safety rules.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Kiến trúc tổng quan](../architecture/overview.md), [Aspect inventory](../research/aspect-inventory.md), [Thuật ngữ](../shared/glossary.md), [Portal](../features/portal.md)

## Tổng Quan

`internal/agentsync` là lõi đồng bộ cấu hình agent của `ns-workspace`. Package này đọc presets được embed từ `presets/`, tạo `SyncPlan` theo phase, tạo shared home mặc định `~/.agents`, rồi materialize instructions, skills, subagents, settings, registry helpers và MCP presets sang các native user-level paths của từng agent.

Các command `init`, `update`, `status`, `doctor`, `registry`, `agents` và alias `catalog` đi qua `internal/cli.RunAgentSync`, sau đó gọi `agentsync.Manager`. Các command preview/search/graph/lsp không dùng module này.

## CLI Boundary

- `Manager.BuildPlan(opt, update)` tạo `SyncPlan` inspectable gồm core phase, registry helper phase, registry install phase, MCP phase và adapter phase.
- `Manager.Apply(opt, update=false)` phục vụ `init`: build plan, tạo shared layout và adapter native output nhưng mặc định bỏ qua file đã tồn tại, trừ khi dùng `--force`.
- `Manager.Apply(opt, update=true)` phục vụ `update`: build plan, rewrite phần tool quản lý từ preset hiện tại (replace-in-place, không backup-before-write), và không giữ key/entry managed đã bị xóa khỏi preset.
- `Manager.Status()` in trạng thái path shared và native theo adapter đang chọn.
- `Manager.Doctor()` validate JSON shared/native, in OS/arch, path tồn tại và các agent CLI executable có trong `PATH`.
- `Manager.Catalog()` liệt kê adapter support tier và artifact support.
- `Manager.InstallRegistrySkills()` ghi registry helpers rồi chạy cài registry skill qua `npx` với `AGENTS_HOME` trỏ tới shared home đã chọn.

`internal/cli` chịu trách nhiệm parse flags chung như `--agents-home`, `--tools`, `--dry-run`, `--force`, `--copy`, `--no-mcp` và `--no-registry`, sau đó truyền `agentsync.Options` vào manager. `main.go` chỉ route nhóm command agentsync qua `internal/cli` và giữ dispatch riêng cho preview/search/graph/lsp.

## Data Model

- `Options` giữ command options và filter adapter.
- `Context` gom options, user home, XDG config home, preset filesystem, user config overlay, reporter và `Update` mode.
- `UserConfig` ánh xạ preset path (ví dụ `presets/opencode/opencode.json`) tới file tuyệt đối trên disk do user cung cấp, dùng để override hoặc bổ sung embedded presets.
- `SyncPlan` là contract build-before-apply cho `init` và `update`.
- `PlanPhase` gom operation theo thứ tự core, registry helpers, registry install, MCP và adapters.
- `PlannedOperation` gắn `Operation` với owner adapter/core/registry và artifact kind để tests/status logic có thể inspect.
- `AdapterSpec` mô tả phần data-driven của adapter: id, alias, tier, docs, executable và native targets.
- `AdapterPlugin` gắn behavior riêng cho adapter đặc biệt mà không làm phình common adapter spec.
- `AgentAdapter` định nghĩa adapter contract: `Name`, `Capabilities`, `Plan`, `StatusPaths`, `DoctorExecutables`.
- `Operation` là thao tác materialization có `Apply`, `Describe` và `Path`.
- `MCPManifest`, `SettingsManifest`, `RegistryManifest` và `OpenCodeConfigManifest` parse JSON preset từ shared home hoặc embedded presets trong dry-run.
- `AdapterSettingsProfile` mô tả cách materialize settings cho một provider: target path (relative to home), preset files, và merge strategy per field (deep-merge, shallow-merge, replace).
- `AdapterSettingsManifest` chứa catalog trung tâm cho adapters cần settings profile, đọc từ `presets/manifest.json`.
- `ApplyAdapterSettings` là operation materialization settings cho provider có profile; provider chưa migrate vẫn dùng `LinkOrCopy` cũ.
- `AgentCapabilities` mô tả support tier, docs URL, artifact kinds và notes cho `agents/catalog`.

## Managed Operations

- `EnsureDir` tạo directory trong core phase.
- `InstallPresetFile` ghi một file preset vào shared/native path.
- `InstallPresetTree` ghi cả cây preset, dùng cho skills và subagents.
- `LinkOrCopy` link hoặc copy file từ shared home sang native path tùy `--copy` hoặc Windows.
- `LinkSkillDirs` link/copy từng skill/subagent dir; trong dry-run có thể đọc embedded preset names khi source shared chưa tồn tại. Mỗi entry preset luôn ghi đè entry cùng tên trong provider target (per-entry replace), kể cả khi `init` không `--force`, nên toàn bộ skill trong preset luôn override skill trong provider target; entry cũ bị thay tại chỗ, không tạo bản backup.
- `MergeJSON` merge JSON vào key path cụ thể và có thể rewrite object managed khi update.
- `AppendManagedBlock` thay managed block có label trong file text như Codex config.
- `ManualStep` ghi guidance vào `~/.agents/generated/<agent>/README.md` cho adapter chưa có native write path chắc chắn.
- `WriteRegistryHelpers`, `RegistryInstall` và `WriteMCPReadme` model hóa các phase đặc biệt thay vì để chúng ẩn trong core apply.

Mọi write đi qua `writeFileManaged()`. Nếu nội dung đã đúng thì in `ok`; nếu path tồn tại và không replace thì `init` bỏ qua; nếu replace thì ghi đè trực tiếp (không backup).

## Adapter Catalog

Stable adapters hiện gồm Claude Code, OpenCode, Grok Build, Kimi Code CLI, Kiro/Kiro CLI, Qwen Code, Gemini CLI, Codex CLI, Cline và ZCode. Stable adapters ghi hoặc link/copy trực tiếp tới native user-level locations. Adapter phổ thông đi qua `AdapterSpec`; adapter có logic riêng dùng plugin nhỏ.

Plugin adapter hiện có:

- OpenCode merge MCP presets dưới key `mcp` (remote: `type/url/enabled`; local: `type/command[]/enabled`) và config values từ `presets/opencode/opencode.json` vào native config qua `MergeJSON` / `OpenCodeAdapter.Plan`.
- Claude tạo script helper `~/.agents/generated/claude/mcp.commands.sh` để add MCP bằng CLI user scope.
- Codex append managed TOML block vào `~/.codex/config.toml` cho MCP servers.
- Grok link `~/.agents/AGENTS.md` → `~/.grok/AGENTS.md` và dùng `GrokPlugin.ExtraOperations` để append managed TOML block MCP vào `~/.grok/config.toml` (`[mcp_servers.<name>]`). Trước khi ghi block, các `[mcp_servers.<name>]` trùng tên với preset bên ngoài block được dọn để tránh lỗi TOML duplicate key. Skills **không** mirror — Grok đọc `~/.agents/skills` native.
- OpenCode / ZCode / Codex / Gemini / Kimi: skills chỉ qua `~/.agents/skills` (không mirror native skills dir). OpenCode vẫn link AGENTS.md + subagents + merge MCP; ZCode link AGENTS.md; stale symlink cũ dưới `~/.config/opencode/skill`, `~/.grok/skills`, `~/.zcode/skills` được cleanup nếu trỏ vào shared home.
- Cline skills/agents: `~/.cline/skills` và `~/.cline/agents` (theo docs); cleanup path cũ `~/.cline/data/skills` và `data/agents`.
- Kiro dùng `KIRO_HOME` nếu env var có giá trị; nếu không dùng `~/.kiro`. Ghi custom agent `~/.kiro/agents/ns-full.json` với `tools: ["*"]`, `allowedTools: ["@builtin", "@*"]`, `includeMcpJson: true` và `resources` trỏ đến synced skills/steering. MCP sync ghi `~/.kiro/settings/mcp.json` và **force `disabled: false`** trên mọi server còn trong catalog (portal enable) — tránh Kiro panel toggle (`disabled: true`) khiến MCP không load sau sync.

## Preset Và Registry Rules

Preset embedded trong `main.go` bao gồm `presets/agents`, `presets/skills/*`, `presets/subagents`, `presets/settings`, `presets/adapters`, `presets/mcp`, `presets/registry` và `presets/opencode`. `BuildPlan()` luôn đặt shared directories và preset core trước registry, MCP và adapter operations.

Phase order của `SyncPlan`:

1. Core shared directories và shared preset content.
2. Registry helper files.
3. Registry install nếu enabled.
4. Shared MCP preset và README nếu enabled.
5. Adapter native operations theo selected adapters.

Khi build `update`, MCP/settings manifests được đọc từ embedded presets (và portal overlay nếu có) để stale shared output không đi tiếp sang native configs. Khi build `init`, existing shared manifest / portal MCP enabled overlay được ưu tiên khi file đã tồn tại; nếu chưa có, embedded preset là fallback. Portal disable overlays (`portal/disabled.json`, `servers.disabled.json`, `skills.disabled.json`) và MCP enabled overlay được agentsync áp dụng khi materialize — xem [Portal](../features/portal.md).

Registry skills trong `presets/registry/skills.json` được ghi thành `~/.agents/registry/skills.json` và `install.sh`. Khi không dùng `--no-registry`, manager cài từng entry vào shared skills home theo `installer`:

- mặc định / `npx-skills`: `npx --yes skills add <source> --skill <skill> --global --agent universal --yes`
- `but-skill` (GitButler): `but skill install --path <agents-home>/skills/<skill> --format none` theo [but skill docs](https://docs.gitbutler.com/commands/but-skill) (non-interactive cần `--path` hoặc `--detect`)

Adapter fan-out skills (khi còn mirror) lấy từ `~/.agents/skills`. Lỗi từng registry skill là warning để các bước khác vẫn chạy. `but` CLI không có trên PATH → skip entry `but-skill` kèm warning.

Skill GitButler (`but`) có hai lớp:

1. **Preset** `presets/skills/but/` — `SKILL.md` + `references/{concepts,examples,reference}.md` (snapshot từ `but skill install`, Agent Skills layout). Core phase materialize vào `~/.agents/skills/but` trên mọi `init`/`update`.
2. **Registry** entry `gitbutler` với `installer: "but-skill"` — khi có CLI `but` trên PATH, update chạy `but skill install --path <agents-home>/skills/but --format none` để refresh cho khớp version CLI (theo [but skill docs](https://docs.gitbutler.com/commands/but-skill)). Không dùng `npx skills add`.

`but skill install` chỉ ship **một** skill package (name `but`); interactive mode chọn *format path* (`.agents`, `.claude`, OpenCode, …), không phải nhiều skill khác nhau.

`--no-mcp` bỏ qua MCP materialization cho adapter và shared MCP preset. `--no-registry` vẫn ghi registry helper files nhưng không chạy cài skills.

## Per-Adapter Settings Profiles

Settings cho mỗi provider được materialize qua adapter settings profile thay vì share một file chung. Mỗi provider có profile riêng trong `presets/adapters/` mô tả:

- `target`: đường dẫn relative to user home, ví dụ `.claude/settings.json`.
- `defaultPreset`: preset cross-cutting (hooks, ...) trong `presets/settings/default.json`.
- `preset`: preset riêng provider trong `presets/settings/<id>.json`.
- `merge`: bảng field → strategy → source (`default`, `preset`, hoặc `shared` cho MCP).

Catalog trung tâm ở `presets/manifest.json` liệt kê adapter id, support tier, executable, docs URL và profile path. Khi `specAdapter.Plan` thấy adapter id trong manifest, nó emit operation `ApplyAdapterSettings` thay cho `LinkOrCopy` settings cũ + `MergeJSON` hooks. Provider chưa có profile vẫn dùng path cũ để giữ backward-compat.

Cấu trúc preset settings:

- `presets/settings/default.json`: hooks cross-cutting rỗng `{}`, làm source `default` cho mọi provider.
- `presets/settings/claude.json`: `{ "permissions": { "defaultMode": "bypassPermissions" } }` chỉ dùng cho Claude Code, merge strategy `replace`.
- `presets/settings/qwen.json`: `{ "permissions": { "defaultMode": "yolo", "confirmShellCommands": false, "confirmFileEdits": false } }` — full bypass mọi confirm cho Qwen Code.
- `presets/settings/gemini.json`: `{ "general": { "defaultApprovalMode": "auto_edit" } }` — auto-approve edit tools (Gemini CLI YOLO mode chỉ enable qua CLI flag `--yolo`/`--approval-mode=yolo`, không ghi được từ settings.json).
- `presets/settings/cline.json`: `{}` (Cline lưu YOLO mode trong `~/.cline/data/settings/global-settings.json` qua UI; preset chỉ set `trust: true` cho từng MCP server ở transform step để auto-approve MCP tool calls).

Cấu trúc adapter profiles:

- `presets/adapters/claude.json`: target `.claude/settings.json`, merge `hooks` (deep, từ default), `permissions` (replace, từ claude.json), `mcpServers` (shallow, từ shared).
- `presets/adapters/qwen.json`: target `.qwen/settings.json`, merge `hooks` (deep, từ default), `permissions` (replace, từ qwen.json), `mcpServers` (shallow, từ shared).
- `presets/adapters/gemini.json`: target `.gemini/settings.json`, merge `general` (replace, từ gemini.json), `mcpServers` (shallow, từ shared).
- `presets/adapters/cline.json`: target `.cline/data/settings/cline_mcp_settings.json`, chỉ merge `mcpServers` (transform set `trust: true` cho mỗi server).
- `presets/adapters/opencode.json`: profile raw, copy thẳng `presets/opencode/opencode.json` sang `~/.config/opencode/opencode.json` (opencodePlugin xử lý MCP merge ở apply phase).

### Full Bypass / Auto-Approve Per Provider

Mỗi provider CLI có field riêng để enable auto-approve, preset đã được cấu hình sẵn cho full bypass:

| Provider    | Field trong settings            | Value preset          | Ghi chú                                                                                |
| ----------- | ------------------------------- | --------------------- | -------------------------------------------------------------------------------------- |
| Claude Code | `permissions.defaultMode`       | `"bypassPermissions"` | Bypass mọi permission prompt.                                                          |
| OpenCode    | `permission`                    | `"allow"`             | Allow mọi tool call mà không prompt.                                                   |
| Qwen Code   | `permissions.defaultMode`       | `"yolo"`              | Full bypass mode. Cộng thêm `confirmShellCommands: false` + `confirmFileEdits: false`. |
| Gemini CLI  | `general.defaultApprovalMode`   | `"auto_edit"`         | Auto-approve edit tools (YOLO mode chỉ enable qua CLI flag).                           |
| Cline       | per-MCP-server `trust`          | `true`                | Auto-approve MCP tool calls. YOLO mode riêng quản lý qua UI (global-settings.json).    |

Test `TestProviderFullBypassConfig` assert cả 4 provider đều sinh ra config full bypass đúng schema docs.

### MCP Server Shape Per Provider

Shared manifest `presets/mcp/servers.json` dùng shape chuẩn MCP (`type`/`url` cho HTTP, `command`/`args` cho stdio). Mỗi provider CLI lại đọc field khác nhau, nên trước khi merge settings hoặc MCP, hàm `transformMCPServersForAdapter` (trong `internal/agentsync/agentsync.go`) rewrite từng server entry theo schema của provider đích:

- **claude**: giữ nguyên `type: "http"` + `url` (Claude Code chấp nhận shape này).
- **opencode**: remote HTTP → `type: "remote"` + `url` + `enabled: true`; stdio → `type: "local"` + `command` argv array (gộp `command` string + `args`) + `enabled: true`; `env` → `environment`. Theo [OpenCode MCP docs](https://opencode.ai/docs/mcp-servers/).
- **qwen**: drop `type`, đổi `url` → `httpUrl` cho HTTP servers. SSE giữ `url`, stdio giữ `command`+`args`. Theo [Qwen MCP docs](https://qwenlm.github.io/qwen-code-docs/en/users/features/mcp/).
- **gemini**: cùng rule với qwen (`httpUrl`, không `type`). Theo [Gemini CLI MCP docs](https://github.com/google-gemini/gemini-cli/blob/main/docs/tools/mcp-server.md).
- **cline**: drop `type` (Cline docs không document field này). Giữ `url` (HTTP/SSE) hoặc `command`+`args` (stdio).
- **kimi** và các provider khác: giữ nguyên shape chuẩn MCP.
- **grok**: không đi qua transform JSON; MCP được render thành managed TOML block trong `~/.grok/config.toml` qua `grokMCPBlock` (`[mcp_servers.<name>]`, bỏ `type`).

Transform JSON áp dụng cho profile-based (`ApplyAdapterSettings` → `buildAdapterSettings`) và inline merge (`MergeJSON`). Grok/Codex dùng `AppendManagedBlock` TOML riêng. Test `TestProviderMCPServerShapeMatchesVendorDocs`, `TestTransformMCPServersForAdapter`, và `TestGrokMCPBlock*` lock-in contract.

Ưu điểm:

- Thêm field mới cho provider X không leak sang provider Y.
- Thêm provider mới không cần đụng Go code, chỉ cần thêm profile + preset + manifest row.
- Tách rõ field cross-cutting (`default.json`) khỏi field provider-specific.
- MCP server shape luôn khớp docs chính thức của từng provider.

Hạn chế:

- Một số provider cần logic đặc biệt (opencode merge top-level object) vẫn dùng plugin riêng, không qua `ApplyAdapterSettings`.
- File name `default.json` thay vì `_default.json` vì `embed.FS` skip file bắt đầu `_`.
- MCP schema của mỗi provider có thể đổi theo version; `transformMCPServersForAdapter` cần được cập nhật và test khi vendor phát hành breaking change.

## User Config Overlay

`Options.ConfigPath` trỏ tới file JSON user-level dùng để override hoặc bổ sung embedded presets. File này cho phép cá nhân hoá preset mà không cần fork repo hay sửa binary. `--config ""` tắt overlay hoàn toàn.

### Vị Trí Mặc Định

| Nền tảng    | Đường dẫn                                                                                   |
| ----------- | ------------------------------------------------------------------------------------------- |
| Linux/macOS | `$XDG_CONFIG_HOME/ns-workspace/config.json` (mặc định `~/.config/ns-workspace/config.json`) |
| Windows     | `%AppData%\ns-workspace\config.json`                                                        |

Env var `NS_WORKSPACE_CONFIG` override vị trí. Flag `--config <file>` override cả env var và default.

### Định Dạng

JSON object phẳng, key là preset path bắt đầu bằng `presets/`, value là đường dẫn tuyệt đối tới file user:

```json
{
  "presets/agents/AGENTS.md": "/home/me/.config/ns-workspace/AGENTS.md",
  "presets/opencode/opencode.json": "/home/me/.config/ns-workspace/opencode.json",
  "presets/skills/custom-skill/SKILL.md": "/home/me/.config/ns-workspace/custom-skill.md",
  "presets/mcp/servers.json": "/home/me/.config/ns-workspace/servers.json"
}
```

Key được chuẩn hoá: `\\` chuyển thành `/`, khoảng trắng đầu/cuối bị cắt, leading `/` bị bỏ. Value phải là đường dẫn tuyệt đối tới file tồn tại. Path không khớp preset nào vẫn hợp lệ và hoạt động như phép cộng (vd: skill hoàn toàn mới).

### Thứ Tự Ưu Tiên

Khi materialization đọc preset path, thứ tự là:

1. **User config** nếu có entry cho path đó.
2. **Embedded preset** trong binary nếu user không cung cấp.

Trên `init`, nếu shared home (`~/.agents/...`) đã có file, nó vẫn được giữ nguyên (không overwrite trừ khi `--force`). Trên `update`, user config đè embedded, và embedded đè shared home cũ để cleanup stale entries.

### Phạm Vi Áp Dụng

- `InstallPresetFile`: đọc qua `readPresetFile()` — áp dụng cho `presets/agents/AGENTS.md`, `presets/settings/settings.json`, `presets/mcp/servers.json`.
- `InstallPresetTree`: walk embedded + materialize user additions cho `presets/skills/*` và `presets/subagents/*`.
- `opencodePlugin`: đọc full `presets/opencode/opencode.json` dưới dạng map rồi merge, cho phép user thêm `timeout`, `provider`, v.v. ngoài `permission` mặc định.
- `readMCPManifest`/`readSettingsManifest`/`readRegistryManifest`: dùng overlay làm fallback khi shared home không có (update mode) hoặc không tồn tại (init mode).

### Ví Dụ: Default Preset Có Full Authorization + Increased Timeout

Để có preset opencode mặc định với full authorization và timeout 300 giây cho mọi tool:

```json
{
  "presets/opencode/opencode.json": "/home/me/.config/ns-workspace/opencode.json"
}
```

`/home/me/.config/ns-workspace/opencode.json`:

```json
{
  "permission": "allow",
  "timeout": 300000
}
```

Sau `ns-workspace init` hoặc `ns-workspace update`, `~/.config/opencode/opencode.json` chứa cả `permission` và `timeout` do user cung cấp.

## Safety Rules

- Luôn dùng `--dry-run` trước khi áp dụng lên môi trường quan trọng vì module này có thể ghi vào user-level config thật.
- `init` không overwrite path có sẵn nếu không có `--force`; ngoại lệ: skill/subagent entry trong provider target luôn bị preset ghi đè tại chỗ để đảm bảo preset là source of truth cho skills. `update` dùng replace mode cho output managed.
- Replace mode ghi đè hoặc xóa file/tree managed tại chỗ (replace-in-place).
- JSON native bị invalid làm command fail sớm để tránh ghi chồng lên config không parse được.
- `--copy` (với `init`) tránh symlink khi người dùng muốn snapshot file hoặc khi platform không dùng symlink. Lệnh `update` luôn bật copy mode: materialize file/directory thật thay vì symlink, vì một số consumer (Kiro IDE) không follow skill-directory symlink.
- Tool filter `--tools stable`, `--tools manual`, `--tools experimental` hoặc danh sách agent cụ thể giới hạn adapter plans được chạy.

## Failure Modes

- Registry install cần `npx`; nếu thiếu, `registry` báo lỗi với hướng dẫn chạy `--no-registry` hoặc dùng script sau.
- Native agent CLI không bắt buộc cho `init/update`; `doctor` mới report executable thiếu.
- MCP server presets là config local/user-level, không validate remote availability tại thời điểm sync.
- Manual/experimental adapters không ghi native config thật; người dùng phải đọc helper generated.
- `DryRun` vẫn có thể cần đọc preset JSON hợp lệ; nếu preset embedded sai format thì command fail.

## Validation

Tests chính nằm trong `main_test.go` và `internal/agentsync/agentsync_test.go`. Khi sửa adapter, preset materialization hoặc registry behavior, chạy:

```bash
go test ./internal/agentsync
go test ./internal/cli
go test ./...
```

Nếu sửa preset Markdown hoặc docs liên quan, chạy thêm:

```bash
npm run lint:doc    # lint + format check
npm run lint:doc:fix # lint fix + format
```

## Quan Hệ

Module này triển khai phần CLI sync được mô tả trong [Kiến trúc tổng quan](../architecture/overview.md), dùng preset model được map trong [Aspect inventory](../research/aspect-inventory.md), và cung cấp thuật ngữ adapter/preset cho [Thuật ngữ](../shared/glossary.md).
