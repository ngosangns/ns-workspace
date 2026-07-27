---
type: planning
title: "Allowed providers cho skills và MCP servers"
description: "Kế hoạch triển khai per-item allowed providers: skills và MCP servers khai báo danh sách providers được phép (hoặc all), sync engine enforce khi init/update, portal UI chỉnh được."
status: implemented
timestamp: 2026-07-27T00:00:00Z
links: [docs/modules/agentsync.md, docs/features/portal.md]
---

# Allowed Providers Cho Skills Và MCP Servers

## Bối cảnh & nguyên nhân gốc rễ

Hiện tại mọi skill và MCP server được fan-out đồng đều tới **tất cả** 11
providers (adapters) đã chọn. Cơ chế lọc duy nhất là global: disable skill qua
`presets/portal/disabled.json`, disable MCP server qua
`presets/mcp/servers.disabled.json`, disable cả provider. Không có cách nào nói
"skill X chỉ dành cho opencode" hay "MCP server Y không đưa vào claude-desktop".
Nhu cầu thực tế: một số skill/MCP chỉ hợp lệ hoặc chỉ muốn expose trên một số
providers nhất định.

Điểm enforce tự nhiên đã xác định qua research:

- Skills mirror: construction site duy nhất `adapter_base.go:169,172`
  (`LinkSkillDirs`), thực thi tại `agentsync.go:288-341` (đã có cơ chế prune
  stale khi `Replace=true` — update luôn bật).
- MCP per-adapter: single dispatch `transformMCPServersForAdapter`
  (`mcp.go:210-304`) + TOML managed blocks (`grokMCPBlock`, `codexMCPBlock`) +
  claude helper script (`mcpCommandScript`, `mcp.go:309`).
- Update = `Manager.Apply(opt, true)` force `CopyMode` + `replace=true` mọi
  phase (`plan.go:45-90`) → chỉ cần filter đúng tại các điểm trên, update tự
  prune mirror/entry không còn được allow.

## Mục tiêu

- Skill (preset + registry) và MCP server khai báo được `providers`: danh sách
  adapter ids hoặc `all`. Mặc định (không khai báo) = `all` → backward
  compatible.
- `init`/`update` enforce: chỉ mirror/merge artifact vào providers được allow;
  `update` prune những mirror/entry trước đó đã sync nhưng giờ bị chặn.
- Portal UI chỉnh được allowed providers per skill / per MCP server, lưu ở
  user overlay (cùng cơ chế `disabled.json` hiện có).
- `doctor` cảnh báo provider id không hợp lệ trong file khai báo.

## Quyết định thiết kế (đã chốt với user)

| Quyết định | Lựa chọn |
| ---------- | -------- |
| Metadata skills | File trung tâm `presets/skills/providers.json` (không đụng frontmatter SKILL.md) |
| Metadata MCP | File riêng `presets/mcp/providers.json` (không nhúng vào `servers.json`) |
| Phạm vi | Cả sync engine lẫn portal UI |
| Default | Absent/empty = `all` (backward compatible) |

Shape 2 file preset (thuần JSON, theo convention `disabled.json`):

```json
{
  "spawn-kimi": ["opencode", "claude"],
  "cleanup": "all"
}
```

- Key = skill top-level dir name (preset hoặc registry) / MCP server name.
- Value = `"all"` hoặc array adapter ids (lowercase, canonical; alias như
  `kiro-cli`/`zcode-cli` normalize về canonical trước khi match).
- Key không có trong file → allow all.

Overlay semantics theo convention hiện có: `readPresetFile` ưu tiên user
overlay; portal đọc effective → mutate → ghi nguyên file vào
`<overlayDir>/presets/{skills,mcp}/providers.json` + đăng ký vào `config.json`
(qua `writeOverlay`, `store.go:89-99`).

Phạm vi lọc:

- **Skills**: lọc tại bước mirror per-provider (`LinkSkillDirs`). Shared tree
  `~/.agents/skills` vẫn chứa đủ mọi skill enabled (không lọc ở PhaseCore) vì
  đây là nguồn chung; mỗi provider chỉ nhận subset được allow. Registry skills
  dùng chung file `presets/skills/providers.json` (key = skill name sau khi
  install vào shared tree).
- **Subagents**: không đổi (ngoài scope).
- **MCP**: lọc manifest per-adapter trước mọi transform/write.
- Tương tác với toggle cũ: `disabled` (global) vẫn thắng trước;
  `providers` chỉ siết thêm per-provider.

## Các bước triển khai

### 1. Sync engine — loader (file mới `internal/agentsync/providers.go`)

- Consts: `SkillsProvidersPath = "presets/skills/providers.json"`,
  `MCPProvidersPath = "presets/mcp/providers.json"`.
- `type ProviderRules map[string]any` (value `string` hoặc `[]string`).
- `LoadProviderRules(ctx, presetKey) ProviderRules` — đọc qua
  `readPresetFile` (overlay-aware), file thiếu → rules rỗng.
- `func (r ProviderRules) Allows(id, adapterID string) bool` — normalize
  lowercase + alias (dùng mapping alias sẵn có trong `adapter_registry.go`;
  expose helper `CanonicalAdapterID` nếu chưa có).
- `ParseProviderRules(data []byte) (ProviderRules, error)` +
  `FormatProviderRules(rules) ([]byte, error)` cho portal ghi file
  (mirror `ParsePortalDisabled`/`FormatPortalDisabled`, `toggles.go:116-163`).

### 2. Skills enforcement

- Thêm field `Allow func(name string) bool` vào `LinkSkillDirs`
  (`agentsync.go:282`): nil = allow all. Khi skip 1 entry: không link, và
  **không** đưa vào `srcNames` → nhánh prune (`Replace=true`) xóa mirror cũ.
  Chỉ prune được nếu entry đó là managed (link/copy do ta tạo) — giữ nguyên
  hành vi prune hiện tại, không mở rộng sang xóa foreign tops.
- `adapter_base.go:169`: truyền `Allow` cho skills op (build từ
  `LoadProviderRules(ctx, SkillsProvidersPath)` + adapter id); subagents op
  (`:172`) giữ nil.
- Kiểm tra các adapter concrete khác có mirror skills ngoài `BaseAdapter`
  không (opencode/claude trong `adapter_concrete.go`) — nếu có, truyền tương tự.

### 3. MCP enforcement

- Helper `filterMCPServersForAdapter(ctx, adapterID, servers map[string]any)
  map[string]any` trong `providers.go`/`mcp.go`: drop server không Allows.
- Áp tại mọi điểm tiêu thụ manifest per-adapter:
  - `transformMCPServersForAdapter` (`mcp.go:210`) — lọc đầu hàm (cần ctx hoặc
    rules truyền vào; đổi signature có kiểm soát, cập nhật call sites).
  - TOML blocks: `grokMCPBlock` (`mcp.go:418`), `codexMCPBlock` (`mcp.go:344`).
  - Claude script: `mcpCommandScript` (`mcp.go:309`).
- Prune khi update: `MergeJSON` (`agentsync.go:407-430`) là merge, không xóa
  key cũ → cần bước pre-remove: trước khi merge, xóa khỏi dst những server
  name **có trong preset catalog nhưng bị chặn** với adapter này (chỉ đụng
  tên trong catalog, không đụng server user tự thêm). TOML managed block tự
  prune vì rewrite cả block (`AppendMCPManagedBlock`, `agentsync.go:475-519`).

### 4. Doctor / status

- `doctor` validate: provider id trong 2 file `providers.json` phải thuộc
  registry (`AdapterRegistry.Ids()` + alias); id lạ → warning (không fail).
- `status` (tùy chọn, ưu tiên thấp): đếm số skill/MCP bị lọc per provider.

### 5. Portal backend

- `internal/portal/api.go`: thêm `AllowedProviders []string` vào `Skill`
  (`api.go:4-16`; nil/`["all"]` = all) và `MCPServerItem` (`api.go:34`).
- `store.go`:
  - `ListSkills` (`:152-214`) đọc rules effective 1 lần, gắn vào từng item.
  - `buildMCPItems` (`:884-904`) gắn tương tự.
  - Writers mới: `SetSkillProviders(id, providers)` /
    `SetMCPServerProviders(name, providers)` → đọc effective file → mutate →
    `writeOverlay("presets/{skills,mcp}/providers.json", FormatProviderRules(...))`.
    `"all"`/rỗng → xóa key khỏi map (về mặc định).
- Handlers: `PUT /api/skills/{id}/providers`,
  `PUT /api/mcps/{name}/providers` (route trong `server.go:46-49`), body
  `{ "providers": ["all"] | ["opencode", ...] }`; validate ids qua adapter
  registry, trả 400 nếu id lạ.

### 6. Portal frontend (SolidJS, `internal/portal/portal_ui_src/`)

- `api.ts`: extend `Skill` (`:3-14`), `MCPServerItem` (`:95-99`) +
  2 method mới.
- Component multi-select providers (tái dùng `components/UiSelect.tsx` hoặc
  popover checkbox mới `components/ProviderPicker.tsx`): hiển thị "All" mặc
  định, tick chọn subset; lấy danh sách providers từ `api.getAdapters()`.
- Gắn vào rows: `views/Skills.tsx:518-554` (cạnh `EnableSwitch:545`),
  `views/MCPs.tsx:383-410` (footer card, cạnh `EnableSwitch:400`).
- Disable picker khi item đã bị disable global (tránh config mâu thuẫn).

### 7. Preset seed + docs

- Seed `presets/skills/providers.json` / `presets/mcp/providers.json` với `{}`
  (hoặc vài rule thật nếu có skill/MCP hiện chỉ hợp lệ trên vài provider —
  rà soát trong lúc implement; mặc định `{}` là đủ cho tính năng).
- Cập nhật `docs/modules/agentsync.md` (mục skills/MCP sync: thêm cơ chế
  per-provider allowlist, file format, semantics prune on update) và
  `docs/features/portal.md` (endpoint + UI mới).

### 8. Tests

- Go: `providers_test.go` — parse/format roundtrip, `Allows` với
  all/absent/list/alias/case; `LinkSkillDirs` skip + prune khi disallowed;
  `filterMCPServersForAdapter` trên 3 đường (JSON merge, TOML block, claude
  script); pre-remove prune không đụng foreign server.
- Portal: parse/format qua store writers; handler 400 khi provider id lạ.
- Frontend: `npm run check:portal` (tsc) pass.

## Verification

```bash
go build ./... && go test ./...
npm run check:portal && npm run build:portal
npm run lint:portal
```

Manual smoke: thêm rule `"cleanup": ["opencode"]` → `go run . update` →
mirror `cleanup` chỉ còn ở opencode dir, bị prune ở các provider khác; rule
MCP tương tự với 1 server → biến mất khỏi config native của provider bị chặn,
vẫn còn ở provider được allow.

## Rủi ro / lưu ý

- `LinkSkillDirs` prune hiện đã xóa dst entries không có trong src khi
  `Replace=true`; thêm filter mở rộng tập bị prune — cần test kỹ không xóa
  foreign/user tops ở provider dir (hành vi hiện tại đã giữ foreign tops ở
  shared tree, nhưng provider dst prune theo srcNames).
- Đổi signature `transformMCPServersForAdapter` chạm nhiều call site (inline
  merge, settings profile, TOML, script) — ưu tiên truyền rules đã load 1 lần
  trong ctx thay vì đọc file lặp lại.
- Kiro force `disabled:false` (`mcp.go:289-297`): server bị chặn phải biến
  mất hẳn, không chỉ bị flag.
- Portal picker hiển thị providers theo canonical id; alias không hiển thị
  riêng.
