# Mẫu Requirements

Dùng cho requirements trong cả hai cây audience:

- `docs/business/features/<feature>/requirements.md` — acceptance criteria, user impact, business rules.
- `docs/developer/features/<feature>/requirements.md` — technical constraints, implementation requirements.
- `docs/business/modules/<module>/requirements.md` — business view của module.
- `docs/developer/modules/<module>/requirements.md` — technical view của module.

```markdown
---
type: feature | module
audience: business | developer
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
