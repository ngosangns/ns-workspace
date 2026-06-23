# Mẫu Requirements

Dùng cho `docs/features/<feature>/requirements.md` hoặc `docs/modules/<module>/requirements.md`:

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

- Acceptance criteria: [observable outcome]
- Applies to: [workflow/API/module area]
- Failure mode: [what must not happen]
```
