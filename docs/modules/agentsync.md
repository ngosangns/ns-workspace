# Module Agentsync

## Meta

- **Status**: active
- **Description**: Tài liệu module `internal/agentsync`, mô tả sync plan, adapter sync, preset materialization, managed operations, registry skills, native targets và safety rules.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Kiến trúc tổng quan](../architecture/overview.md), [Aspect inventory](../research/aspect-inventory.md), [Thuật ngữ](../shared/glossary.md), [Plan tối ưu Agentsync](../specs/planning/refactor-agentsync-preset-architecture.md)

## Tổng Quan

`internal/agentsync` là lõi đồng bộ cấu hình agent của `ns-workspace`. Package này đọc presets được embed từ `presets/`, tạo `SyncPlan` theo phase, tạo shared home mặc định `~/.agents`, rồi materialize instructions, skills, subagents, settings, registry helpers và MCP presets sang các native user-level paths của từng agent.

Các command `init`, `update`, `status`, `doctor`, `registry`, `agents` và alias `catalog` đi qua `internal/cli.RunAgentSync`, sau đó gọi `agentsync.Manager`. Các command preview/search/graph/lsp không dùng module này.

## CLI Boundary

- `Manager.BuildPlan(opt, update)` tạo `SyncPlan` inspectable gồm core phase, registry helper phase, registry install phase, MCP phase và adapter phase.
- `Manager.Apply(opt, update=false)` phục vụ `init`: build plan, tạo shared layout và adapter native output nhưng mặc định bỏ qua file đã tồn tại, trừ khi dùng `--force`.
- `Manager.Apply(opt, update=true)` phục vụ `update`: build plan, rewrite phần tool quản lý từ preset hiện tại, backup path cũ trước khi ghi, và không giữ key/entry managed đã bị xóa khỏi preset.
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
- `AgentCapabilities` mô tả support tier, docs URL, artifact kinds và notes cho `agents/catalog`.

## Managed Operations

- `EnsureDir` tạo directory trong core phase.
- `InstallPresetFile` ghi một file preset vào shared/native path.
- `InstallPresetTree` ghi cả cây preset, dùng cho skills và subagents.
- `LinkOrCopy` link hoặc copy file từ shared home sang native path tùy `--copy` hoặc Windows.
- `LinkSkillDirs` link/copy từng skill/subagent dir; trong dry-run có thể đọc embedded preset names khi source shared chưa tồn tại.
- `MergeJSON` merge JSON vào key path cụ thể và có thể rewrite object managed khi update.
- `AppendManagedBlock` thay managed block có label trong file text như Codex config hoặc Aider config.
- `ManualStep` ghi guidance vào `~/.agents/generated/<agent>/README.md` cho adapter chưa có native write path chắc chắn.
- `WriteRegistryHelpers`, `RegistryInstall` và `WriteMCPReadme` model hóa các phase đặc biệt thay vì để chúng ẩn trong core apply.

Mọi write đi qua `writeFileManaged()`. Nếu nội dung đã đúng thì in `ok`; nếu path tồn tại và không replace thì `init` bỏ qua; nếu replace thì backup bằng suffix timestamp trước khi ghi.

## Adapter Catalog

Stable adapters hiện gồm Claude Code, OpenCode, Grok Build, Kimi Code CLI, Kiro/Kiro CLI, Qwen Code, Gemini CLI, Codex CLI, Cline, Windsurf và Aider. Stable adapters ghi hoặc link/copy trực tiếp tới native user-level locations. Adapter phổ thông đi qua `AdapterSpec`; adapter có logic riêng dùng plugin nhỏ.

Manual adapters hiện gồm Cursor, GitHub Copilot và JetBrains AI. Experimental/manual guarded adapters hiện gồm Antigravity, Trae và Roo. Nhóm này chỉ tạo helper trong `~/.agents/generated/<agent>/` vì native path hoặc support contract chưa đủ ổn định.

Plugin adapter hiện có:

- OpenCode nhận MCP presets dưới key `mcp` và đổi server type `http` thành `remote`; permission lấy từ `presets/opencode/opencode.json`.
- Claude tạo script helper `~/.agents/generated/claude/mcp.commands.sh` để add MCP bằng CLI user scope.
- Codex append managed TOML block vào `~/.codex/config.toml` cho MCP servers.
- Aider append managed conventions block vào `~/.aider.conf.yml`.
- Kiro dùng `KIRO_HOME` nếu env var có giá trị; nếu không dùng `~/.kiro`.

## Preset Và Registry Rules

Preset embedded trong `main.go` bao gồm `presets/agents`, `presets/skills/*`, `presets/subagents`, `presets/settings`, `presets/mcp`, `presets/registry` và `presets/opencode`. `BuildPlan()` luôn đặt shared directories và preset core trước registry, MCP và adapter operations.

Phase order của `SyncPlan`:

1. Core shared directories và shared preset content.
2. Registry helper files.
3. Registry install nếu enabled.
4. Shared MCP preset và README nếu enabled.
5. Adapter native operations theo selected adapters.

Khi build `update`, MCP/settings manifests được đọc từ embedded presets để stale shared output không đi tiếp sang native configs. Khi build `init`, existing shared manifest được dùng nếu file đã tồn tại; nếu chưa có, embedded preset là fallback.

Registry skills trong `presets/registry/skills.json` được ghi thành `~/.agents/registry/skills.json` và `install.sh`. Khi không dùng `--no-registry`, manager chạy `npx --yes skills add ... --global --agent universal --yes` cho từng skill để cài vào shared skills home trước; adapter fan-out vẫn do `ns-workspace update` link/copy từ `~/.agents/skills`. Lỗi từng registry skill được report thành warning để các bước khác vẫn có thể hoàn tất.

Registry hiện ship 5 skill: `find-skills`, `dispatching-parallel-agents`, `gitbutler`, `taste-skill` và `minimax-cli` (từ `MiniMax-AI/cli`, skill name `mmx-cli`). `minimax-cli` cài official SKILL.md của MiniMax CLI để các coding agent có thể invoke `mmx` command mà không cần user tự `npx skills add MiniMax-AI/cli -y -g`.

`--no-mcp` bỏ qua MCP materialization cho adapter và shared MCP preset. `--no-registry` vẫn ghi registry helper files nhưng không chạy cài skills.

## User Config Overlay

`Options.ConfigPath` trỏ tới file JSON user-level dùng để override hoặc bổ sung embedded presets. File này cho phép cá nhân hoá preset mà không cần fork repo hay sửa binary. `--config ""` tắt overlay hoàn toàn.

### Vị Trí Mặc Định

| Nền tảng | Đường dẫn |
| --- | --- |
| Linux/macOS | `$XDG_CONFIG_HOME/ns-workspace/config.json` (mặc định `~/.config/ns-workspace/config.json`) |
| Windows | `%AppData%\ns-workspace\config.json` |

Env var `NS_WORKSPACE_CONFIG` override vị trí. Flag `--config <file>` override cả env var và default.

### Định Dạng

JSON object phẳng, key là preset path bắt đầu bằng `presets/`, value là đường dẫn tuyệt đối tới file user:

```json
{
  "presets/agents/AGENTS.md": "/home/me/.config/ns-workspace/AGENTS.md",
  "presets/opencode/opencode.json": "/home/me/.config/ns-workspace/opencode.json",
  "presets/skills/minimax-cli/SKILL.md": "/home/me/.config/ns-workspace/minimax-cli.md",
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
- `init` không overwrite path có sẵn nếu không có `--force`; `update` dùng replace mode cho output managed.
- Replace mode backup file/tree cũ trước khi ghi hoặc remove.
- JSON native bị invalid làm command fail sớm để tránh ghi chồng lên config không parse được.
- `--copy` tránh symlink khi người dùng muốn snapshot file hoặc khi platform không dùng symlink.
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
npm run lint:docs
npm run format:docs:check
```

## Quan Hệ

Module này triển khai phần CLI sync được mô tả trong [Kiến trúc tổng quan](../architecture/overview.md), dùng preset model được map trong [Aspect inventory](../research/aspect-inventory.md), và cung cấp thuật ngữ adapter/preset cho [Thuật ngữ](../shared/glossary.md).
