# Support Kiro And Kiro CLI Adapters

## Meta

- **Status**: implemented
- **Description**: Plan và trạng thái triển khai adapter Kiro/Kiro CLI cho `ns-workspace`, bao gồm alias selection, steering instructions và MCP settings.
- **Compliance**: current-state
- **Links**: [Kiến trúc tổng quan](../../architecture/overview.md), [Adapter agent user-level](./user-level-agent-adapter-framework.md), [Chỉ mục](../../_index.md)

## Bối Cảnh

`ns-workspace` đồng bộ cấu hình agent cá nhân từ `~/.agents` sang các adapter user-level trong `internal/agentsync`. Adapter Kiro được đăng ký với tên chính `kiro` và alias `kiro-cli`, nên `--tools kiro` và `--tools kiro-cli` cùng chọn một adapter.

Kiro CLI docs hiện mô tả global configuration dưới `<user-home>/.kiro/`: MCP ở `~/.kiro/settings/mcp.json`, custom agents ở `~/.kiro/agents`, steering ở `~/.kiro/steering`, settings ở `~/.kiro/settings/cli.json`. Kiro IDE docs cũng dùng user-level MCP path `~/.kiro/settings/mcp.json`, nên MCP có thể dùng chung cho Kiro IDE và Kiro CLI. CLI executable chính là `kiro-cli`; wrapper `kiro` cũng cần được doctor kiểm tra nếu có trên máy.

## Hành Vi Hiện Tại

- Adapter `kiro` thuộc tier stable.
- `--tools kiro` và `--tools kiro-cli` đều chọn adapter Kiro.
- Shared instructions được link/copy vào `~/.kiro/steering/AGENTS.md`.
- MCP presets được merge vào `~/.kiro/settings/mcp.json` dưới key `mcpServers`.
- Doctor report kiểm tra cả `kiro` và `kiro-cli` executable.
- Catalog hiển thị note rằng `kiro-cli` là alias của `kiro`.

## Ngoài Phạm Vi

- Không thêm integration UI riêng cho Kiro IDE ngoài các file user-level mà Kiro docs xác nhận.
- Không tạo custom Kiro agent JSON phức tạp nếu chưa cần để load shared skills/subagents.
- Không migrate existing user config ngoài merge managed MCP/hook fields theo engine hiện có.

## Thiết Kế

1. Model adapter hỗ trợ alias selection.
   - Thêm `aliases []string` vào `fileAdapter` hoặc interface method tương đương.
   - `selected()` match `adapter.Name()` và aliases, đồng thời giữ tier selection hiện tại.
   - Catalog hiển thị notes nhắc alias `kiro-cli`.

2. Adapter Kiro.
   - Name: `kiro`.
   - Aliases: `kiro-cli`.
   - Executables: `kiro`, `kiro-cli`.
   - Instruction path: `~/.kiro/steering/AGENTS.md`.
   - MCP path: `~/.kiro/settings/mcp.json`, key path `mcpServers`.
   - Docs URL: Kiro CLI configuration, Kiro CLI MCP, Kiro IDE MCP configuration.

3. Cập nhật CLI help và README.
   - Thêm `kiro` và `kiro-cli` vào `--tools` help.
   - Thêm Kiro vào Adapter Support table, nêu rõ shared MCP path và alias CLI.

4. Cập nhật tests.
   - `main_test.go` và `internal/agentsync/agentsync_test.go` assert có `~/.kiro/settings/mcp.json`.
   - Test selection riêng cho `ParseTools("kiro-cli")` để đảm bảo alias chọn adapter.
   - Test stable/all vẫn tạo Kiro artifacts đúng phạm vi.

## Công Việc Cần Làm

- [x] Sửa `internal/agentsync/agentsync.go`:
  - thêm alias support;
  - thêm adapter `kiro`;
  - cập nhật doctor/catalog nếu cần.
- [x] Sửa `main.go` help string cho `--tools`.
- [x] Sửa `README.md` adapter list và ví dụ `--tools`.
- [x] Sửa test coverage trong `main_test.go` và `internal/agentsync/agentsync_test.go`.
- [x] Chạy validation mục tiêu:
  - `go test ./...`
  - `go run . agents --tools kiro`
  - `go run . agents --tools kiro-cli`
  - `go run . init --tools kiro-cli --no-registry --dry-run`

## Rủi Ro Và Ràng Buộc

- Kiro custom agents có format riêng; không nên ghi bừa shared `AGENTS.md` vào `~/.kiro/agents` nếu file đó không phải agent configuration hợp lệ.
- MCP path `~/.kiro/settings/mcp.json` được cả Kiro CLI và IDE docs mô tả, nên đây là phần ít rủi ro nhất để auto-sync.
- Alias support thay đổi selection chung, cần test để không làm hỏng `all`, `stable`, `manual`, `experimental`.
- Nếu `kiro` wrapper không tồn tại trên mọi install, doctor nên report từng executable độc lập thay vì coi thiếu một executable là fatal.

## Kiểm Chứng

- `go test ./...` pass.
- `go run . agents --tools kiro` hiển thị Kiro adapter.
- `go run . agents --tools kiro-cli` cũng hiển thị Kiro adapter.
- `go run . init --tools kiro --no-registry --dry-run` mô tả write/merge vào đúng `~/.kiro/...` paths.
