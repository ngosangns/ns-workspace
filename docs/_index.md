---
type: index
title: "Chỉ Mục Tài Liệu"
description: "Chỉ mục điều hướng của knowledge base, liệt kê tài liệu chính, trạng thái hiện tại và quan hệ graph giữa các docs."
okf_version: "0.1"
tags: ["index"]
timestamp: 2026-07-17T00:00:00Z
status: active
compliance: current-state
---

# Chỉ Mục Tài Liệu

## Meta

- **Status**: active
- **Description**: Chỉ mục điều hướng của knowledge base, liệt kê tài liệu chính, trạng thái hiện tại và quan hệ graph giữa các docs.
- **Compliance**: current-state
- **Links**: [Tài liệu dự án](./README.md), [Trạng thái sync](./_sync.md), [Kiến trúc tổng quan](./architecture/overview.md), [Aspect inventory](./research/aspect-inventory.md), [Module agentsync](./modules/agentsync.md), [Module harness](./modules/harness.md), [Preview web](./features/preview-web.md), [Portal](./features/portal.md), [Agentic loop](./features/agentic-loop.md), [Module preview](./modules/preview.md), [Module kbmcp](./modules/kbmcp.md), [Module graph query](./modules/graphquery.md), [Thuật ngữ](./shared/glossary.md), [Quy ước frontend](./development/conventions/preview-frontend.md), [Cleanup repo](./specs/planning/cleanup-repo.md)

## Modules

| Module              | Spec File                                                                                    | Status   | Version | Compliance    | Priority |
| ------------------- | -------------------------------------------------------------------------------------------- | -------- | ------- | ------------- | -------- |
| Tài liệu dự án      | [README.md](./README.md)                                                                     | active   | current | current-state | P0       |
| Trạng thái sync     | [\_sync.md](./_sync.md)                                                                      | active   | current | current-state | P0       |
| Kiến trúc tổng quan | [architecture/overview.md](./architecture/overview.md)                                       | active   | current | current-state | P0       |
| Aspect inventory    | [research/aspect-inventory.md](./research/aspect-inventory.md)                               | active   | current | current-state | P0       |
| Module agentsync    | [modules/agentsync.md](./modules/agentsync.md)                                               | active   | current | current-state | P0       |
| Module harness      | [modules/harness.md](./modules/harness.md)                                                   | active   | current | current-state | P0       |
| Preview web         | [features/preview-web.md](./features/preview-web.md)                                         | active   | current | current-state | P0       |
| Portal              | [features/portal.md](./features/portal.md)                                                   | active   | current | current-state | P0       |
| Agentic loop        | [features/agentic-loop.md](./features/agentic-loop.md)                                       | active   | current | current-state | P1       |
| Module preview      | [modules/preview.md](./modules/preview.md)                                                   | active   | current | current-state | P0       |
| Module kbmcp        | [modules/kbmcp.md](./modules/kbmcp.md)                                                       | active   | current | current-state | P1       |
| Module graph query  | [modules/graphquery.md](./modules/graphquery.md)                                             | active   | current | current-state | P0       |
| Thuật ngữ           | [shared/glossary.md](./shared/glossary.md)                                                   | active   | current | current-state | P1       |
| Quy ước frontend    | [development/conventions/preview-frontend.md](./development/conventions/preview-frontend.md) | active   | current | current-state | P1       |
| Cleanup repo        | [specs/planning/cleanup-repo.md](./specs/planning/cleanup-repo.md)                           | implemented | current | current-state | P1       |

## Specs Và Planning

Planning/spec hiện có: [Cleanup Toàn Bộ Repo](./specs/planning/cleanup-repo.md). Hành vi đã shipped được mô tả trực tiếp trong [Preview web](./features/preview-web.md), [Portal](./features/portal.md), [Agentic loop](./features/agentic-loop.md), [Module agentsync](./modules/agentsync.md), [Module harness](./modules/harness.md), [Module preview](./modules/preview.md), [Module graph query](./modules/graphquery.md) và [Kiến trúc tổng quan](./architecture/overview.md).

## Dependency Graph

```mermaid
flowchart LR
  "README.md" --> "_index.md"
  "_index.md" --> "_sync.md"
  "_index.md" --> "architecture/overview.md"
  "_index.md" --> "research/aspect-inventory.md"
  "_index.md" --> "features/portal.md"
  "_index.md" --> "specs/planning/cleanup-repo.md"
  "architecture/overview.md" --> "modules/agentsync.md"
  "architecture/overview.md" --> "modules/preview.md"
  "architecture/overview.md" --> "modules/graphquery.md"
  "architecture/overview.md" --> "features/portal.md"
  "research/aspect-inventory.md" --> "modules/agentsync.md"
  "research/aspect-inventory.md" --> "modules/preview.md"
  "research/aspect-inventory.md" --> "modules/graphquery.md"
  "modules/preview.md" --> "modules/kbmcp.md"
  "modules/preview.md" --> "features/preview-web.md"
  "modules/preview.md" --> "modules/graphquery.md"
  "modules/preview.md" --> "development/conventions/preview-frontend.md"
  "modules/agentsync.md" --> "shared/glossary.md"
  "modules/agentsync.md" --> "modules/harness.md"
  "modules/agentsync.md" --> "features/portal.md"
  "modules/harness.md" --> "features/agentic-loop.md"
  "shared/glossary.md" --> "architecture/overview.md"
```
