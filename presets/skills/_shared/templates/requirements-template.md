# Mẫu Requirements

Dùng cho requirements tùy chọn cạnh feature/module trong layout flat:

- `docs/features/<feature>.md` hoặc section requirements trong feature/module doc.
- `docs/modules/<module>.md` — technical + business rules trong cùng file khi phù hợp.

```markdown
---
type: feature | module
title: "[Feature/Module] Requirements"
description: "Critical requirements that must always be followed for [feature/module]."
tags: ["requirements", "[domain]"]
timestamp: <ISO 8601>
status: active
---

# [Feature/Module] Requirements

Phạm vi: [feature/module boundary]. Liên quan: [Overview](./overview.md).

## Critical Requirements

### REQ-1: [Requirement]

- Acceptance criteria / Technical constraint: [observable outcome or engineering invariant]
- Applies to: [workflow/API/module area]
- Failure mode: [what must not happen]
```
