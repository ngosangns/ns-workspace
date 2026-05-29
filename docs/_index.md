# Chỉ Mục Tài Liệu

## Meta

- **Status**: active
- **Description**: Chỉ mục điều hướng của knowledge base, liệt kê tài liệu chính, trạng thái hiện tại và quan hệ graph giữa các docs.
- **Compliance**: current-state
- **Links**: [Tài liệu dự án](./README.md), [Trạng thái sync](./_sync.md), [Kiến trúc tổng quan](./architecture/overview.md), [Preview web](./features/preview-web.md), [Module preview](./modules/preview.md), [Thuật ngữ](./shared/glossary.md), [LSP Code Graph Search](./specs/planning/lsp-code-graph-search.md), [LSP Search Graph Command Và Skill](./specs/planning/package-lsp-search-graph-command-skill.md), [Tự động cài LSP cho Graph Query](./specs/planning/auto-install-lsp-for-graph.md), [Mở rộng LSP coverage](./specs/planning/expand-lsp-language-coverage.md)

## Modules

| Module              | Spec File                                                                                                              | Status  | Version | Compliance    | Priority |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------- | ------- | ------- | ------------- | -------- |
| Tài liệu dự án      | [README.md](./README.md)                                                                                               | active  | current | current-state | P0       |
| Trạng thái sync     | [\_sync.md](./_sync.md)                                                                                                | active  | current | current-state | P0       |
| Kiến trúc tổng quan | [architecture/overview.md](./architecture/overview.md)                                                                 | active  | current | current-state | P0       |
| Preview web         | [features/preview-web.md](./features/preview-web.md)                                                                   | active  | current | current-state | P0       |
| Module preview      | [modules/preview.md](./modules/preview.md)                                                                             | active  | current | current-state | P0       |
| Thuật ngữ           | [shared/glossary.md](./shared/glossary.md)                                                                             | active  | current | current-state | P1       |
| Quy ước frontend    | [development/conventions/preview-frontend.md](./development/conventions/preview-frontend.md)                           | active  | current | current-state | P1       |
| Search standalone   | [specs/planning/standalone-search-graph-command.md](./specs/planning/standalone-search-graph-command.md)               | planned | current | planning      | P1       |
| LSP Code Graph      | [specs/planning/lsp-code-graph-search.md](./specs/planning/lsp-code-graph-search.md)                                   | active  | current | planning      | P1       |
| LSP graph command   | [specs/planning/package-lsp-search-graph-command-skill.md](./specs/planning/package-lsp-search-graph-command-skill.md) | active  | current | planning      | P1       |
| LSP auto install    | [specs/planning/auto-install-lsp-for-graph.md](./specs/planning/auto-install-lsp-for-graph.md)                         | active  | current | planning      | P1       |
| LSP coverage        | [specs/planning/expand-lsp-language-coverage.md](./specs/planning/expand-lsp-language-coverage.md)                     | active  | current | planning      | P1       |

## Specs Và Planning

Planning/spec hiện có: [Tách Search Page Thành Frontend Standalone Và Thêm Lệnh Graph](./specs/planning/standalone-search-graph-command.md), [Thay Code Graph Graphify Bằng LSP](./specs/planning/lsp-code-graph-search.md), [Đóng Gói LSP Search Graph Thành Command Và Skill](./specs/planning/package-lsp-search-graph-command-skill.md), [Tự Động Cài LSP Cho Graph Query](./specs/planning/auto-install-lsp-for-graph.md) và [Mở Rộng LSP Coverage](./specs/planning/expand-lsp-language-coverage.md). Hành vi đã shipped được mô tả trực tiếp trong [Preview web](./features/preview-web.md), [Module preview](./modules/preview.md) và [Kiến trúc tổng quan](./architecture/overview.md).

## Dependency Graph

```mermaid
flowchart LR
  "README.md" --> "_index.md"
  "_index.md" --> "_sync.md"
  "_index.md" --> "architecture/overview.md"
  "architecture/overview.md" --> "modules/preview.md"
  "modules/preview.md" --> "features/preview-web.md"
  "modules/preview.md" --> "development/conventions/preview-frontend.md"
  "modules/preview.md" --> "specs/planning/standalone-search-graph-command.md"
  "modules/preview.md" --> "specs/planning/lsp-code-graph-search.md"
  "modules/preview.md" --> "specs/planning/package-lsp-search-graph-command-skill.md"
  "modules/preview.md" --> "specs/planning/auto-install-lsp-for-graph.md"
  "modules/preview.md" --> "specs/planning/expand-lsp-language-coverage.md"
  "shared/glossary.md" --> "architecture/overview.md"
```
