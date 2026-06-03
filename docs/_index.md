# Chỉ Mục Tài Liệu

## Meta

- **Status**: active
- **Description**: Chỉ mục điều hướng của knowledge base, liệt kê tài liệu chính, trạng thái hiện tại và quan hệ graph giữa các docs.
- **Compliance**: current-state
- **Links**: [Tài liệu dự án](./README.md), [Trạng thái sync](./_sync.md), [Kiến trúc tổng quan](./architecture/overview.md), [Aspect inventory](./research/aspect-inventory.md), [Module agentsync](./modules/agentsync.md), [Preview web](./features/preview-web.md), [Module preview](./modules/preview.md), [Module graph query](./modules/graphquery.md), [Thuật ngữ](./shared/glossary.md), [Agentsync preset architecture](./specs/planning/refactor-agentsync-preset-architecture.md), [Search standalone](./specs/planning/standalone-search-graph-command.md), [Cải thiện tốc độ Preview Search](./specs/planning/improve-preview-search-performance.md), [Tối ưu Preview Web](./specs/planning/optimize-preview-web-surface.md), [LSP Code Graph Search](./specs/planning/lsp-code-graph-search.md), [LSP Search Graph Command Và Skill](./specs/planning/package-lsp-search-graph-command-skill.md), [Tự động cài LSP cho Graph Query](./specs/planning/auto-install-lsp-for-graph.md), [Mở rộng LSP coverage](./specs/planning/expand-lsp-language-coverage.md), [Tự động ensure LSP khi query graph](./specs/planning/auto-ensure-lsp-on-graph-query.md), [Sửa timeout LSP Code Graph](./specs/planning/fix-lsp-graph-timeout-partial-index.md)

## Modules

| Module                | Spec File                                                                                                              | Status      | Version | Compliance    | Priority |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------- | ----------- | ------- | ------------- | -------- |
| Tài liệu dự án        | [README.md](./README.md)                                                                                               | active      | current | current-state | P0       |
| Trạng thái sync       | [\_sync.md](./_sync.md)                                                                                                | active      | current | current-state | P0       |
| Kiến trúc tổng quan   | [architecture/overview.md](./architecture/overview.md)                                                                 | active      | current | current-state | P0       |
| Aspect inventory      | [research/aspect-inventory.md](./research/aspect-inventory.md)                                                         | active      | current | current-state | P0       |
| Module agentsync      | [modules/agentsync.md](./modules/agentsync.md)                                                                         | active      | current | current-state | P0       |
| Preview web           | [features/preview-web.md](./features/preview-web.md)                                                                   | active      | current | current-state | P0       |
| Module preview        | [modules/preview.md](./modules/preview.md)                                                                             | active      | current | current-state | P0       |
| Module graph query    | [modules/graphquery.md](./modules/graphquery.md)                                                                       | active      | current | current-state | P0       |
| Thuật ngữ             | [shared/glossary.md](./shared/glossary.md)                                                                             | active      | current | current-state | P1       |
| Quy ước frontend      | [development/conventions/preview-frontend.md](./development/conventions/preview-frontend.md)                           | active      | current | current-state | P1       |
| Agentsync refactor    | [specs/planning/refactor-agentsync-preset-architecture.md](./specs/planning/refactor-agentsync-preset-architecture.md) | in-progress | current | planning      | P1       |
| Search standalone     | [specs/planning/standalone-search-graph-command.md](./specs/planning/standalone-search-graph-command.md)               | planned     | current | planning      | P1       |
| Preview search perf   | [specs/planning/improve-preview-search-performance.md](./specs/planning/improve-preview-search-performance.md)         | planned     | current | planning      | P1       |
| Preview web cleanup   | [specs/planning/optimize-preview-web-surface.md](./specs/planning/optimize-preview-web-surface.md)                     | implemented | current | current-state | P1       |
| LSP Code Graph        | [specs/planning/lsp-code-graph-search.md](./specs/planning/lsp-code-graph-search.md)                                   | active      | current | planning      | P1       |
| LSP graph command     | [specs/planning/package-lsp-search-graph-command-skill.md](./specs/planning/package-lsp-search-graph-command-skill.md) | active      | current | planning      | P1       |
| LSP auto install      | [specs/planning/auto-install-lsp-for-graph.md](./specs/planning/auto-install-lsp-for-graph.md)                         | active      | current | planning      | P1       |
| LSP coverage          | [specs/planning/expand-lsp-language-coverage.md](./specs/planning/expand-lsp-language-coverage.md)                     | active      | current | planning      | P1       |
| LSP graph auto ensure | [specs/planning/auto-ensure-lsp-on-graph-query.md](./specs/planning/auto-ensure-lsp-on-graph-query.md)                 | active      | current | planning      | P1       |
| LSP graph timeout     | [specs/planning/fix-lsp-graph-timeout-partial-index.md](./specs/planning/fix-lsp-graph-timeout-partial-index.md)       | active      | current | planning      | P1       |

## Specs Và Planning

Planning/spec hiện có: [Tối Ưu Agentsync, Preset Và Cấu Trúc CLI](./specs/planning/refactor-agentsync-preset-architecture.md), [Tách Search Page Thành Frontend Standalone Và Thêm Lệnh Graph](./specs/planning/standalone-search-graph-command.md), [Cải Thiện Tốc Độ Preview Search](./specs/planning/improve-preview-search-performance.md), [Tối Ưu Và Rút Gọn Preview Web](./specs/planning/optimize-preview-web-surface.md), [Thay Code Graph Graphify Bằng LSP](./specs/planning/lsp-code-graph-search.md), [Đóng Gói LSP Search Graph Thành Command Và Skill](./specs/planning/package-lsp-search-graph-command-skill.md), [Tự Động Cài LSP Cho Graph Query](./specs/planning/auto-install-lsp-for-graph.md), [Mở Rộng LSP Coverage](./specs/planning/expand-lsp-language-coverage.md), [Tự Động Ensure LSP Khi Query Graph](./specs/planning/auto-ensure-lsp-on-graph-query.md) và [Sửa Timeout Làm LSP Code Graph Trả Thiếu Kết Quả](./specs/planning/fix-lsp-graph-timeout-partial-index.md). Hành vi đã shipped được mô tả trực tiếp trong [Preview web](./features/preview-web.md), [Module agentsync](./modules/agentsync.md), [Module preview](./modules/preview.md), [Module graph query](./modules/graphquery.md) và [Kiến trúc tổng quan](./architecture/overview.md).

## Dependency Graph

```mermaid
flowchart LR
  "README.md" --> "_index.md"
  "_index.md" --> "_sync.md"
  "_index.md" --> "architecture/overview.md"
  "_index.md" --> "research/aspect-inventory.md"
  "architecture/overview.md" --> "modules/agentsync.md"
  "modules/agentsync.md" --> "specs/planning/refactor-agentsync-preset-architecture.md"
  "research/aspect-inventory.md" --> "modules/agentsync.md"
  "research/aspect-inventory.md" --> "modules/preview.md"
  "research/aspect-inventory.md" --> "modules/graphquery.md"
  "architecture/overview.md" --> "modules/preview.md"
  "architecture/overview.md" --> "modules/graphquery.md"
  "modules/agentsync.md" --> "shared/glossary.md"
  "modules/preview.md" --> "features/preview-web.md"
  "modules/preview.md" --> "modules/graphquery.md"
  "modules/preview.md" --> "development/conventions/preview-frontend.md"
  "modules/preview.md" --> "specs/planning/standalone-search-graph-command.md"
  "modules/preview.md" --> "specs/planning/improve-preview-search-performance.md"
  "modules/preview.md" --> "specs/planning/optimize-preview-web-surface.md"
  "modules/preview.md" --> "specs/planning/lsp-code-graph-search.md"
  "modules/preview.md" --> "specs/planning/package-lsp-search-graph-command-skill.md"
  "modules/graphquery.md" --> "specs/planning/auto-install-lsp-for-graph.md"
  "modules/graphquery.md" --> "specs/planning/expand-lsp-language-coverage.md"
  "modules/graphquery.md" --> "specs/planning/auto-ensure-lsp-on-graph-query.md"
  "modules/preview.md" --> "specs/planning/fix-lsp-graph-timeout-partial-index.md"
  "shared/glossary.md" --> "architecture/overview.md"
```
