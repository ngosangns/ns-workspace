# Mẫu Requirements

Dùng cho `docs/features/<feature>/requirements.md` hoặc `docs/modules/<module>/requirements.md`:

```markdown
---
title: "[Feature/Module] Requirements"
description: "Critical requirements that must always be followed for [feature/module]."
type: feature | module
status: active
tags: ["requirements", "[domain]"]
related:
  - "./overview.md"
---

# [Feature/Module] Requirements

## Meta

- Trạng thái: active
- Phạm vi: [feature/module boundary]
- Links: [Overview](./overview.md)

## Critical Requirements

### REQ-1: [Requirement]

- Acceptance criteria: [observable outcome]
- Applies to: [workflow/API/module area]
- Failure mode: [what must not happen]
```
