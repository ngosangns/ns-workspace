# User-Level Agent Adapter Framework

## Bối Cảnh

CLI hiện tại dùng `~/.agents` làm source of truth rồi sync sang một số agent native locations trong `main.go`. Các phần chính đang được quản lý gồm `AGENTS.md`, `skills`, `subagents`, `settings.json`, MCP presets, registry skills và helper scripts. Adapter hiện là struct `toolAdapter` đơn giản, hardcode trong hàm `adapters()`, với logic cài đặt chung trong `installAdapter()` và MCP merge theo vài format cố định.

Yêu cầu mới là sửa lại install/update để toàn bộ artifacts này đi vào user-level folder của từng agent như Cursor, Antigravity, Kimi Code, Claude Code, Qwen Code, Trae và các agent coding phổ biến khác. Vì mỗi agent có format, path, CLI command và precedence riêng, cần thay hardcode bằng interface adapter chung.

Research docs chính thức/primary cho thấy các user-level path đáng tin cậy hiện có:

- Claude Code: `~/.claude/CLAUDE.md`, `~/.claude/settings.json`, `~/.claude/skills`, `~/.claude/agents`; MCP nên apply qua `claude mcp add ... --scope user` hoặc `claude mcp add-json ... --scope user`.
- OpenCode: config JSON/JSONC tại `~/.config/opencode/opencode.json` hoặc tương đương XDG; custom agents có thể nằm trong `~/.config/opencode/agent(s)/`; MCP nằm trong key `mcp`.
- Kimi Code CLI: dữ liệu user-level ở `~/.kimi/`, gồm `~/.kimi/config.toml`, `~/.kimi/AGENTS.md`, `~/.kimi/mcp.json`, và skills không bị đổi bởi `KIMI_SHARE_DIR`.
- Qwen Code: user settings tại `~/.qwen/settings.json`; project context dùng `QWEN.md`; MCP nằm trong `mcpServers` và có thể ghi qua `qwen mcp add --scope user`.
- Gemini CLI: user settings tại `~/.gemini/settings.json`; context file mặc định là `GEMINI.md`; MCP nằm trong top-level `mcpServers`; CLI hỗ trợ `gemini mcp add ... --scope user`.
- Codex CLI: user config tại `~/.codex/config.toml`; MCP nằm dưới `[mcp_servers.*]`; `AGENTS.md` là instruction file chuẩn cho workspace, còn user/global config cần mapping cẩn thận qua `config.toml`/instructions tuỳ version.
- Cursor: docs chính thức xác nhận User Rules là global trong Cursor Settings, Project Rules ở `.cursor/rules`, `AGENTS.md` hiện root-level project instruction; Cursor CLI MCP dùng cùng config editor và `cursor-agent mcp`.
- Windsurf: global rules ở `~/.codeium/windsurf/memories/global_rules.md`, workspace rules ở `.windsurf/rules/*.md`.
- Cline CLI: config ở `~/.cline/data/`, MCP ở `~/.cline/data/settings/cline_mcp_settings.json`, rules/skills/workflows/hooks quản lý qua `cline config` và filesystem.
- GitHub Copilot/Coding Agent: repo instructions gồm `.github/copilot-instructions.md`, `.github/instructions/*.instructions.md`, `.github/agents/*.agent.md`; cloud coding agent cũng hỗ trợ `AGENTS.md`.
- Aider: user config default là `~/.aider.conf.yml`; conventions có thể luôn load qua config.
- JetBrains AI/Junie/Codex integration: MCP chủ yếu cấu hình qua UI hoặc ACP JSON; có global/project server level nhưng path cụ thể có thể product/version-specific.
- Antigravity và Trae hiện thiếu official, stable file-path docs đủ chắc để ghi tự động. Nên đưa vào tier `manual-or-experimental` cho đến khi xác nhận native storage path/CLI ổn định.

## Mục Tiêu

- Tạo kiến trúc adapter chung để install/update/status/doctor không hardcode từng agent trong `main.go`.
- Mỗi agent adapter tự mô tả user-level folders, artifacts được support, merge/write strategy, CLI helper commands và validation.
- Sync được `AGENTS.md`, skills, subagents/custom agents, MCP, settings/hooks/rules vào native user-level locations khi agent có docs/path ổn định.
- Với agent chưa có stable docs, generate manual helper/readme thay vì ghi bừa vào path có rủi ro.
- Mở rộng danh sách tools từ 6 agent hiện tại sang catalog coding agents theo tier, nhưng vẫn cho phép `--tools` chọn subset.
- Giữ `~/.agents` làm source-of-truth nội bộ, sau đó materialize sang user-level của từng agent.

## Ngoài Phạm Vi

- Không cố tự động support mọi extension/private beta nếu không có docs hoặc path ổn định.
- Không migrate secret/token vào repo hay presets.
- Không tự động chỉnh UI-only settings của Cursor, Trae, Antigravity, JetBrains nếu docs không công bố file/CLI ổn định.
- Không thay đổi preview/search feature đang có trong `internal/preview`.

## Agent Catalog Đề Xuất

Tier 1, implement ngay vì có path/format đủ rõ:

- `claude`: instructions, settings/hooks, skills, subagents, MCP via CLI helper.
- `opencode`: instructions/config, agents/subagents, hooks JSON merge, MCP JSON merge.
- `kimi`: global AGENTS, skills, MCP JSON, config helper.
- `qwen`: user settings JSON, hooks/MCP JSON merge hoặc CLI helper, context helper.
- `gemini`: settings JSON, hooks/MCP JSON merge hoặc CLI helper, global `GEMINI.md`.
- `codex`: config TOML MCP merge, settings, optional global instruction bridge.
- `cline`: MCP JSON, rules/skills/workflows directories or helper, config dir support.
- `windsurf`: global rules file, workspace rule helper, MCP manual until stable path is confirmed.
- `aider`: `~/.aider.conf.yml` managed block for conventions/config.

Tier 2, implement guarded/manual helper trước:

- `cursor`: create/importable user-rule text and MCP helper commands; project `AGENTS.md` remains project-level, not user-level.
- `github-copilot`: generate project/repo artifacts and optional global VS Code settings guidance; no hidden global write by default.
- `jetbrains`: generate MCP JSON snippets/ACP snippets and manual instructions; product/version-specific writes stay opt-in.
- `antigravity`: generate `GEMINI.md`/`AGENTS.md` guidance and MCP snippet only until official user-level path is verified.
- `trae`: keep manual/experimental adapter until official docs confirm user rules/MCP path.
- `roo`: keep manual/experimental because Roo Code sunset notice increases risk; only support if user explicitly enables.

Tier 3, catalog-only initially:

- Devin, Sweep/OpenHands, Continue, Sourcegraph Cody, Zed Agent Panel, Junie, Amp, Goose, Tabnine, Amazon Q Developer, Augment, Codeium/Windsurf variants, Replit Agent, Lovable/Bolt-style web agents. These should be tracked in metadata with docs links and support status, not written automatically until install surface is clear.

## Hướng Tiếp Cận Đề Xuất

1. Tách domain model khỏi `main.go`.
   - Tạo package/internal module kiểu `internal/agentsync`.
   - Model chính:
     - `ArtifactKind`: instructions, skills, subagents, settings, hooks, mcp, registry, commands, rules.
     - `AgentAdapter` interface: `Name()`, `Detect()`, `PlanInstall()`, `ApplyInstall()`, `PlanUpdate()`, `Status()`, `Doctor()`, `Validate()`.
     - `InstallPlan`: danh sách operations `WriteFile`, `LinkTree`, `CopyTree`, `MergeJSON`, `MergeTOML`, `RunCommand`, `ManualStep`.
     - `AgentCapabilities`: support matrix cho từng artifact và mức confidence.
   - `main.go` chỉ parse CLI flags rồi gọi registry.

2. Chuẩn hoá source-of-truth trong `~/.agents`.
   - Giữ `~/.agents/AGENTS.md`, `~/.agents/skills`, `~/.agents/agents`, `~/.agents/settings.json`, `~/.agents/mcp/servers.json`.
   - Thêm generated helper outputs dưới `~/.agents/generated/<agent>/`.
   - Thêm `~/.agents/agent-catalog.json` hoặc embed `presets/agents/catalog.json` để mô tả adapter metadata, docs URL, support tier và native paths.

3. Implement operation engine dùng chung.
   - Reuse logic hiện có: `writeFileManaged`, `linkOrCopy`, `backupPath`, `mergeObject`.
   - Thêm `mergeJSONAt(path, keyPath, values)`.
   - Thêm TOML writer/merger cho Codex/Aider/Kimi config nếu cần. Ưu tiên Go TOML parser thay vì string concat.
   - Thêm managed block support cho file text/YAML/TOML khi không thể fully own file.
   - Mọi write đều có dry-run, backup, update-vs-init semantics.

4. Implement adapters theo nhóm format.
   - `JSONSettingsAdapter`: OpenCode hooks/MCP, Qwen hooks/MCP, Gemini hooks/MCP, Cline MCP, Kimi MCP.
   - `ClaudeAdapter`: symlink/copy instructions, skills, agents, settings; MCP via generated shell script or direct command operation.
   - `OpenCodeAdapter`: JSON/JSONC config merge dưới key `mcp`, agents dir sync.
   - `TomlConfigAdapter`: Codex, Aider, Kimi config.
   - `RulesOnlyAdapter`: Cursor, Windsurf, Copilot, Antigravity, Trae manual/guarded mode.

5. Update CLI surface.
   - `--tools` nên support aliases: `all`, `stable`, `experimental`, `manual`, plus names.
   - Thêm `agents` hoặc `catalog` command để list supported agents, tier, docs URL, artifact support.
   - `status` hiển thị cả supported/missing/manual steps.
   - `doctor` validate JSON/TOML, check CLIs: `claude`, `opencode`, `kimi`, `qwen`, `gemini`, `codex`, `cline`, `aider`, `cursor-agent`.
   - `init/update` với tier manual chỉ tạo helper/readme, không ghi native path nếu adapter không safe.

6. Update presets/docs/tests.
   - README mô tả source-of-truth, support matrix, và path native theo agent.
   - `presets/mcp/README.md` generate theo adapter thay vì Claude-only wording.
   - Tests cho plan generation, dry-run operations, path expansion, JSON/TOML merges, selected tools aliases, manual adapter behavior.

## Công Việc Cần Làm

- Tạo `internal/agentsync` với operation engine và registry.
- Di chuyển `toolAdapter`, `adapters()`, `installAdapter()`, `mergeMCP()`, `writeMCPHelpers()` ra adapter architecture mới.
- Thêm catalog embedded cho agent list và support tiers.
- Implement Tier 1 adapters: Claude, OpenCode, Kimi, Qwen, Gemini, Codex, Cline, Windsurf, Aider.
- Implement Tier 2 guarded adapters: Cursor, GitHub Copilot, JetBrains, Antigravity, Trae, Roo.
- Update CLI help, README, MCP README/helper scripts.
- Update tests trong `main_test.go` và thêm test package cho adapter operations.
- Chạy validation focused: `go test ./...` sau khi xử lý baseline test failure hiện có nếu còn liên quan.

## Rủi Ro Và Ràng Buộc

- Nhiều tool có docs thay đổi nhanh; adapter phải lưu docs URL và confidence/tier để dễ cập nhật.
- Một số tool chỉ expose UI settings, không có stable file path; phải manual helper thay vì ghi tự động.
- MCP config chứa secrets; merge không được inline secret từ env hiện tại, chỉ preserve placeholders/env references.
- Symlink vào app config folder có thể bị editor/CLI rewrite thành file thường; status cần detect drift.
- JSONC/TOML comment preservation dễ mất nếu dùng generic marshal. Với config của user, ưu tiên managed block hoặc CLI command khi official CLI có sẵn.
- Worktree hiện đang dirty với nhiều file đã move vào `internal/preview`; không revert unrelated changes.

## Kiểm Chứng

- Unit:
  - adapter registry returns stable/manual tiers correctly.
  - `--tools stable`, `--tools all`, `--tools claude,qwen` selection đúng.
  - JSON merge preserves unrelated keys for Qwen/Gemini/Kimi/Cline.
  - TOML merge for Codex writes `[mcp_servers.<name>]` without deleting other config.
  - manual adapters produce `ManualStep` and do not write unsafe native paths.
  - dry-run emits planned operations without filesystem changes.
- Integration with temp home:
  - `init --tools stable --no-registry --dry-run`
  - `init --tools claude,qwen,gemini,codex,cline --no-registry --copy`
  - `update` creates backups for managed files.
  - `status` and `doctor` report native files and validation results.
- Manual:
  - inspect generated helper scripts under `~/.agents/generated/<agent>/`.
  - run selected official CLI MCP list commands where installed.

## Acceptance Criteria

- Có adapter interface chung, không còn hardcode toàn bộ agent behavior trong một hàm `adapters()` ở `main.go`.
- `init/update/status/doctor` chạy qua cùng operation planning/apply pipeline.
- `~/.agents` vẫn là source-of-truth, nhưng mỗi supported agent nhận đúng user-level native artifacts theo docs của nó.
- Agent không có stable user-level path chỉ nhận generated helper/manual steps, không ghi path suy đoán.
- README có support matrix và nêu rõ tier stable/manual/experimental.
- Tests cover merge behavior và selected tool behavior cho các adapter chính.

## Nguồn Docs Đã Research

- Claude Code settings/MCP/directory: https://docs.claude.com/en/docs/claude-code/settings, https://docs.claude.com/en/docs/claude-code/mcp, https://code.claude.com/docs/en/claude-directory
- OpenCode config/agents/MCP: https://opencode.ai/docs/config/, https://opencode.ai/docs/agents/, https://opencode.ai/docs/mcp-servers/
- Kimi Code data/config/MCP/customization: https://www.kimi.com/code/docs/en/kimi-code-cli/configuration/data-locations.html, https://www.kimi.com/code/docs/en/kimi-cli.html, https://www.kimi.com/help/kimi-code/cli-customization
- Qwen Code config/MCP: https://qwenlm.github.io/qwen-code-docs/en/cli/configuration/, https://qwenlm.github.io/qwen-code-docs/en/users/features/mcp/
- Gemini CLI config/MCP/settings: https://github.com/google-gemini/gemini-cli/blob/main/docs/reference/configuration.md, https://github.com/google-gemini/gemini-cli/blob/main/docs/tools/mcp-server.md, https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/settings.md
- Codex config/AGENTS.md: https://github.com/openai/codex/blob/main/docs/config.md, https://github.com/openai/codex/blob/main/docs/agents_md.md
- Cursor rules/MCP CLI: https://docs.cursor.com/en/context, https://docs.cursor.com/cli/mcp
- Windsurf memories/rules: https://docs.windsurf.com/windsurf/cascade/memories
- Cline CLI configuration/MCP: https://docs.cline.bot/cline-cli/configuration
- GitHub Copilot instructions/custom agents: https://github.blog/changelog/2025-08-28-copilot-coding-agent-now-supports-agents-md-custom-instructions/, https://code.visualstudio.com/docs/copilot/customization/custom-instructions, https://github.github.io/awesome-copilot/learning-hub/building-custom-agents/
- Aider config/conventions: https://aider.chat/docs/config/aider_conf.html, https://aider.chat/docs/usage/conventions.html
- JetBrains AI MCP/agents: https://www.jetbrains.com/help/ai-assistant/mcp.html, https://www.jetbrains.com/help/ai-assistant/activate-agents.html
