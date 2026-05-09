# Chỉ Mục Tài Liệu

## Meta

- **Status**: active
- **Description**: Chỉ mục điều hướng của knowledge base, liệt kê tài liệu chính, trạng thái hiện tại và quan hệ graph giữa các docs.
- **Compliance**: current-state
- **Links**: [Tài liệu dự án](./README.md), [Trạng thái sync](./_sync.md), [Kiến trúc tổng quan](./architecture/overview.md), [Preview web](./features/preview-web.md), [Module preview](./modules/preview.md), [Thuật ngữ](./shared/glossary.md)

## Modules

| Module                   | Spec File                                                                                                                        | Status      | Version | Compliance    | Priority |
| ------------------------ | -------------------------------------------------------------------------------------------------------------------------------- | ----------- | ------- | ------------- | -------- |
| Tài liệu dự án           | [README.md](./README.md)                                                                                                         | active      | current | current-state | P0       |
| Trạng thái sync          | [\_sync.md](./_sync.md)                                                                                                          | active      | current | current-state | P0       |
| Kiến trúc tổng quan      | [architecture/overview.md](./architecture/overview.md)                                                                           | active      | current | current-state | P0       |
| Preview web              | [features/preview-web.md](./features/preview-web.md)                                                                             | active      | current | current-state | P0       |
| Module preview           | [modules/preview.md](./modules/preview.md)                                                                                       | active      | current | current-state | P0       |
| Thuật ngữ                | [shared/glossary.md](./shared/glossary.md)                                                                                       | active      | current | current-state | P1       |
| Quy ước frontend         | [development/conventions/preview-frontend.md](./development/conventions/preview-frontend.md)                                     | active      | current | current-state | P1       |
| Trang search preview     | [specs/planning/add-preview-search-page.md](./specs/planning/add-preview-search-page.md)                                         | implemented | current | current-state | P0       |
| Internal links preview   | [specs/planning/resolve-preview-internal-links-and-mentions.md](./specs/planning/resolve-preview-internal-links-and-mentions.md) | implemented | current | current-state | P0       |
| TypeScript preview       | [specs/planning/use-full-typescript-for-preview-web.md](./specs/planning/use-full-typescript-for-preview-web.md)                 | implemented | current | current-state | P0       |
| Adapter agent user-level | [specs/planning/user-level-agent-adapter-framework.md](./specs/planning/user-level-agent-adapter-framework.md)                   | draft       | current | current-state | P2       |

## Specs Và Planning

| Spec                                                                                          | Trạng thái  | Liên kết chính                                                                                              |
| --------------------------------------------------------------------------------------------- | ----------- | ----------------------------------------------------------------------------------------------------------- |
| [Trang search preview](./specs/planning/add-preview-search-page.md)                           | implemented | [Preview web](./features/preview-web.md), [Module preview](./modules/preview.md)                            |
| [Internal links và mentions](./specs/planning/resolve-preview-internal-links-and-mentions.md) | implemented | [Preview web](./features/preview-web.md), [Module preview](./modules/preview.md)                            |
| [TypeScript cho preview web](./specs/planning/use-full-typescript-for-preview-web.md)         | implemented | [Module preview](./modules/preview.md), [Quy ước phát triển](./development/conventions/preview-frontend.md) |
| [Adapter agent user-level](./specs/planning/user-level-agent-adapter-framework.md)            | draft       | [Kiến trúc tổng quan](./architecture/overview.md), [Thuật ngữ](./shared/glossary.md)                        |

## Dependency Graph

```mermaid
flowchart LR
  "README.md" --> "_index.md"
  "_index.md" --> "_sync.md"
  "_index.md" --> "architecture/overview.md"
  "architecture/overview.md" --> "modules/preview.md"
  "modules/preview.md" --> "features/preview-web.md"
  "features/preview-web.md" --> "specs/planning/add-preview-search-page.md"
  "features/preview-web.md" --> "specs/planning/resolve-preview-internal-links-and-mentions.md"
  "modules/preview.md" --> "specs/planning/use-full-typescript-for-preview-web.md"
  "modules/preview.md" --> "development/conventions/preview-frontend.md"
  "architecture/overview.md" --> "specs/planning/user-level-agent-adapter-framework.md"
  "shared/glossary.md" --> "architecture/overview.md"
```
