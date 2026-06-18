# Tái Thiết Kế Preset Settings/Config Theo Provider

## Meta

- **Status**: implemented
- **Description**: Tách preset `settings.json` dùng chung hiện tại thành preset per-adapter, bổ sung manifest catalog và transform layer, đảm bảo mỗi provider (Claude Code, OpenCode, Qwen, Gemini, Cline...) có config format riêng nhưng vẫn đi qua một pipeline thống nhất.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../../_index.md), [Module agentsync](../../modules/agentsync.md), [Aspect inventory](../../research/aspect-inventory.md), [Plan tối ưu agentsync/preset/CLI](./refactor-agentsync-preset-architecture.md), [Thuật ngữ](../../shared/glossary.md)

## Bối Cảnh

Hiện tại `presets/settings/settings.json` là file duy nhất phục vụ cho `settings` artifact của mọi adapter:

```json
{
  "hooks": {}
}
```

File này được materialize về `~/.agents/settings.json`, rồi `LinkOrCopy` tới:

- `~/.claude/settings.json` (Claude Code)
- `~/.qwen/settings.json` (Qwen, dùng `hooks` key trong `MergeJSON`)
- `~/.gemini/settings.json` (Gemini, dùng `hooks` key trong `MergeJSON`)

Khi user thêm field `permissions.defaultMode = "bypassPermissions"` cho Claude Code, field đó xuất hiện trong cả `~/.qwen/settings.json` và `~/.gemini/settings.json` mặc dù hai provider đó không hiểu field `permissions`. Đây là triệu chứng của shared preset không phân biệt provider.

Một preset khác cũng có vấn đề tương tự về cấu trúc: `presets/opencode/opencode.json` chứa `"permission": "allow"` được merge thẳng vào config JSON của OpenCode. Field này có ý nghĩa hoàn toàn khác với `permissions.defaultMode` của Claude Code, nhưng cả hai cùng nói về "full permissions" cho provider tương ứng. Hiện không có chỗ nào trong manifest Go hay preset model nói rõ rằng `settings.json` của provider X phải merge key nào, với format nào.

Plan refactor tổng ở `refactor-agentsync-preset-architecture.md` (in-progress) đã chỉ ra rằng:

- `OpenCodeConfigManifest` cần được chuẩn hoá và chuyển sang plugin riêng.
- `SettingsManifest` hiện chỉ parse `hooks` cũng nên mở rộng để mô tả config fields theo provider.
- Cần có `presets/manifest.json` và `presets/adapters/*.json` typed thay vì để catalog nằm trong Go code.

Plan này tập trung vào nhánh settings/config, làm tiền đề cho các plan khác về preset model.

## Nguyên Nhân Và Lý Do Thiết Kế

**Triệu chứng 1**: Thêm một field chỉ có ý nghĩa với Claude Code làm field đó lan sang Qwen/Gemini. Cùng một file `presets/settings/settings.json`, cùng một `SettingsManifest` parse, cùng một `LinkOrCopy` operation — không có boundary nào để dừng việc viết field không áp dụng.

**Triệu chứng 2**: Format config khác nhau giữa các provider nhưng bị gộp vào cùng abstraction:

- Claude Code: `permissions.defaultMode`, `hooks`, `mcpServers` ở root. MCP server dùng `type: "http"` + `url` hoặc `command`+`args`.
- OpenCode: `permission` (string "allow" hoặc object map tool → action), `mcp`, `provider`, `model`. MCP server dùng `type: "remote"` + `url` hoặc `type: "local"` + `command`.
- Qwen: `hooks`, `mcpServers` ở root. MCP HTTP server dùng `httpUrl` (không `type`), SSE dùng `url`, stdio dùng `command`+`args`.
- Gemini: `mcpServers` ở root. KHÔNG có `hooks` ở root và không có `permissions.defaultMode` (Gemini dùng `general.defaultApprovalMode`). MCP server dùng `httpUrl`/`url`/`command` như Qwen.
- Cline: `mcpServers` ở root, không có `hooks` ở root. MCP server dùng `url` (HTTP/SSE) hoặc `command`+`args` (stdio). Cline docs không document field `type`.

`SettingsManifest` hiện chỉ biết `hooks`; mọi field khác bị ignore hoặc nằm ngoài shared preset.

**Nguyên nhân trực tiếp**: Một `SettingsManifest` duy nhất, một path preset duy nhất, một operation `LinkOrCopy` chung — không có cách nào để adapter nói "tôi muốn áp dụng field A, không muốn field B".

**Nguyên nhân gỗ rễ**: Preset model chưa có manifest typed cho artifact `settings`. Adapter target paths và transform rule (key path nào được merge, replace hay append) nằm rải rác trong Go code thay vì trong manifest. `presets/` là source of truth cho content nhưng chưa là source of truth cho config model per-provider.

**Hướng thiết kế**: Mỗi provider có một preset settings riêng + một adapter settings profile mô tả cách merge preset đó vào target file. Pipeline đọc preset settings + áp dụng profile = config cuối cùng cho provider. Tính đa hình đến từ việc thêm một profile mới không cần đụng Go code, không cần đụng preset của provider khác.

## Góc Nhìn Tổng Quan Và Phạm Vi Tập Trung

Phạm vi gồm năm ranh giới:

- **Layout preset**: chuyển `presets/settings/settings.json` thành `presets/settings/<adapter-id>.json` cho từng provider. File shared chỉ còn defaults cross-provider (ví dụ `presets/settings/default.json` cho hooks cross-cutting).
- **Adapter settings profile**: thêm file `presets/adapters/<adapter-id>.json` mô tả `targetPath`, `keyPaths` cho hooks, MCP, permissions, custom fields, và `mergePolicy` (replace/append/merge-deep) per field.
- **Manifest trung tâm**: thêm `presets/manifest.json` liệt kê adapter id, support tier, docs URL, executable, default preset path, settings profile path. Đây là source of truth cho catalog, thay thế phần Go code đang hard-code catalog.
- **Reader/transform layer**: thay `SettingsManifest` đơn lẻ bằng một `AdapterSettings` parser/transform đọc preset settings của provider, áp dụng profile, sinh ra final config object. Tương tự cho `OpenCodeConfigManifest` nhưng giữ plugin riêng.
- **Tests + docs**: test cho mỗi provider (settings hợp lệ, field sai bị loại bỏ, merge policy đúng), cập nhật `Module agentsync` và `Aspect inventory`.

## Kế Hoạch Chi Tiết

### 1. Phân Tích Hiện Trạng Preset Settings

- Liệt kê field nào trong `presets/settings/settings.json` hiện tại áp dụng cho provider nào.
- Liệt kê field nào Claude Code, OpenCode, Qwen, Gemini, Cline đọc từ file settings tương ứng (tham chiếu docs chính thức).
- Quyết định field nào là cross-cutting default (ví dụ hooks), field nào là per-provider.
- Viết bảng map `field -> provider(s) -> source-of-truth` làm input cho bước 2.

### 2. Tái Cấu Trúc Layout Preset

- Tạo `presets/settings/default.json` chứa các key cross-cutting (ví dụ `hooks` rỗng).
- Tạo `presets/settings/claude.json` chứa field chỉ Claude Code (ví dụ `permissions.defaultMode`).
- Tạo `presets/settings/opencode.json` (chuyển từ `presets/opencode/opencode.json` nếu phù hợp) — hoặc giữ `presets/opencode/opencode.json` riêng vì OpenCode có format JSON riêng biệt, không nên gộp.
- Tạo `presets/settings/qwen.json`, `presets/settings/gemini.json`, `presets/settings/cline.json` nếu cần (mặc định có thể rỗng vì các provider này dùng `hooks`/`mcpServers` shared).
- Cập nhật `SettingsManifest` thành một kiểu parse mới (xem bước 4).

Quyết định: OpenCode giữ preset ở `presets/opencode/opencode.json` vì config format đặc thù. Không ép vào `presets/settings/*` để tránh mapping giả.

### 3. Thêm Adapter Settings Profile

Thêm file JSON cho mỗi provider mô tả cách merge settings vào target file:

```json
{
  "id": "claude",
  "target": ".claude/settings.json",
  "merge": {
    "hooks": { "strategy": "merge-deep", "source": "shared:hooks" },
    "permissions": { "strategy": "merge-shallow", "source": "claude.json" },
    "mcpServers": { "strategy": "merge-shallow", "source": "shared:mcpServers" }
  }
}
```

- `source` trỏ tới preset path (relative từ `presets/`) hoặc shared manifest (ví dụ `shared:hooks`, `shared:mcpServers`).
- `strategy` là `merge-deep`/`merge-shallow`/`replace`/`append` tuỳ field.
- Validate profile khi `BuildPlan` chạy: profile path tồn tại, source path tồn tại, target path template hợp lệ.

### 4. Manifest Trung Tâm Và Reader/Transform Layer

- Thêm `presets/manifest.json` với schema tối thiểu:

```json
{
  "version": 1,
  "adapters": [
    {
      "id": "claude",
      "tier": "stable",
      "executable": "claude",
      "docs": ["https://docs.claude.com/en/docs/claude-code/settings"],
      "settings": "presets/adapters/claude.json"
    }
  ]
}
```

- Trong `internal/agentsync`, tách file `adapter_settings.go` chứa:
  - `AdapterSettingsProfile` struct parse từ `presets/adapters/<id>.json`.
  - `ReadAdapterSettings(ctx, profile)` đọc preset settings theo `source` từng field, merge theo `strategy`, trả `map[string]any` cuối cùng.
  - Thay `MergeJSON` hard-code cho settings bằng `ApplyAdapterSettings` dùng profile.
- Trong `plan.go`, thay operation `LinkOrCopy` cho `targets.Settings` bằng `ApplyAdapterSettings` cho các provider có profile settings.
- Backward-compat: nếu provider chưa có profile, dùng `LinkOrCopy` cũ (giữ behavior cũ cho Qwen/Gemini nếu chưa cần đổi).

### 5. Tương Thích OpenCode

- Tách phần `permission` của `presets/opencode/opencode.json` thành một `OpenCodePermissionProfile` riêng (hoặc giữ inline nếu muốn tối thiểu thay đổi).
- Không bắt buộc đưa OpenCode vào `presets/settings/`, nhưng nếu đưa vào thì phải có profile riêng trong `presets/adapters/opencode.json` mô tả `permission` là custom field không qua `ApplyAdapterSettings`.
- Tài liệu hoá quyết định trong plan này.

### 6. Tests

- Thêm `TestAdapterSettingsMergesProviderFieldsOnly` chứng minh `claude.json` không leak field sang Qwen.
- Thêm `TestAdapterSettingsHonorsMergeStrategy` cho từng strategy.
- Thêm `TestAdapterSettingsFallsBackToLegacy` cho provider chưa có profile.
- Cập nhật `TestApplyCreatesStableAndManualAgentLayout` để assert content provider-specific (ví dụ `~/.claude/settings.json` có `permissions.defaultMode`, `~/.qwen/settings.json` không có).
- Cập nhật `TestUpdateRewritesManagedPresetContent` để cover profile reload.

### 7. Cập Nhật Docs

- Cập nhật `Module agentsync` (`docs/modules/agentsync.md`):
  - Mô tả preset layout mới (`presets/settings/`, `presets/adapters/`, `presets/manifest.json`).
  - Mô tả `AdapterSettingsProfile` và merge strategy.
  - Mô tả `ApplyAdapterSettings` operation.
- Cập nhật `Aspect inventory` (`docs/research/aspect-inventory.md`) nếu thay đổi source paths.
- Cập nhật `Kiến trúc tổng quan` (`docs/architecture/overview.md`) nếu pipeline materialize đổi.
- Cập nhật `docs/_index.md` và `docs/_sync.md` sau implementation.
- Link ngược về plan này và plan tổng `refactor-agentsync-preset-architecture.md`.

## Công Việc Cần Làm

1. Khảo sát docs chính thức từng provider để có bảng map field → provider → source.
2. Tạo `presets/settings/default.json` và các file per-provider cần thiết.
3. Tạo `presets/adapters/<id>.json` cho mỗi provider stable cần settings profile.
4. Thêm `presets/manifest.json` (hoặc merge vào manifest tổng nếu plan refactor tạo sẵn).
5. Tách `internal/agentsync/adapter_settings.go` với parser, reader và operation `ApplyAdapterSettings`.
6. Cập nhật `plan.go` để dùng operation mới.
7. Bổ sung test cho từng provider và fallback path.
8. Cập nhật docs (`modules/agentsync.md`, `aspect-inventory.md`, `overview.md`, `_index.md`, `_sync.md`).

## Rủi Ro Và Ràng Buộc

- Đây là refactor chạm user-level config writer, phải giữ behavior cũ cho provider chưa có profile.
- Plan tổng `refactor-agentsync-preset-architecture.md` đang in-progress, có thể chọn thứ tự triển khai khác (spec/plugin trước, manifest sau). Kế hoạch này nên bám theo thứ tự đó, tránh manifest quá sớm.
- Provider docs có thể đổi; bảng map field cần cập nhật khi provider nâng version.
- User config overlay hiện dùng path preset làm key (`presets/settings/settings.json`); khi đổi sang per-provider path, cần quyết định user config key tương ứng (vd `presets/settings/claude.json`).
- Một số provider (Qwen, Gemini, Cline) dùng `MergeJSON` với key path cố định (`["hooks"]`, `["mcpServers"]`); refactor không được phá contract này nếu provider chưa migrate sang profile.
- OpenCode preset có format JSON riêng biệt; ép vào `presets/settings/` có thể làm gộp giả, giữ tách là hợp lý hơn.
- Go `embed.FS` skip file bắt đầu bằng `_` hoặc `.` khi match directory, nên cross-cutting preset phải tên `default.json` chứ không phải `_default.json`.
- OpenCode preset có format JSON riêng biệt; ép vào `presets/settings/` có thể làm gộp giả, giữ tách là hợp lý hơn.

## Kiểm Chứng

```bash
go test ./internal/agentsync
go test ./internal/cli
go test .
go run . status --agents-home /tmp/ns-workspace-status-check
go run . doctor --agents-home /tmp/ns-workspace-doctor-check
go run . update --dry-run --no-registry --no-mcp --tools stable
```

Sau khi đổi docs:

```bash
npm run lint:docs
npm run format:docs:check
```

Khi đã chắc chắn dry-run đúng phạm vi:

```bash
go run . update --no-registry --no-mcp --tools stable
```

## Tiêu Chí Chấp Nhận

- Mỗi provider có preset settings riêng (nếu cần), không còn shared file ghi field lẫn nhau.
- Có `presets/adapters/<id>.json` mô tả target, key paths, merge strategy.
- Có `presets/manifest.json` typed hoặc ít nhất có data model sẵn để chuyển manifest.
- `internal/agentsync` có `adapter_settings.go` riêng với `ApplyAdapterSettings` operation.
- `ApplyAdapterSettings` tôn trọng merge strategy và không leak field sai provider.
- Backward-compat: provider chưa có profile vẫn dùng path cũ không lỗi.
- Tests pass: `go test ./...`.
- Docs mô tả đúng preset layout mới, profile, manifest, operation mới.
- Link hai chiều với plan tổng `refactor-agentsync-preset-architecture.md` và module docs hiện hành.
