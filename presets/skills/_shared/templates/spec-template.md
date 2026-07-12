# Mẫu Spec

Dùng cho specs trong cả hai cây audience:

- `docs/business/specs/*.md` — business specs: user stories, acceptance criteria, business rules.
- `docs/developer/specs/*.md` — technical specs: design, API, implementation details.

```markdown
---
type: spec
audience: business | developer
title: "[Tính năng hoặc thay đổi]"
description: "[Mô tả ngắn]"
tags: [spec]
timestamp: <ISO 8601>
status: draft | approved | implemented
---

# [Tính năng hoặc thay đổi]

## Tổng Quan

## Yêu Cầu

### REQ-1: [Yêu cầu]

**Tiêu Chí Chấp Nhận:**

- [ ] AC-1.1: [Tiêu chí]

**Kịch Bản:**
GIVEN [bối cảnh]
WHEN [hành động]
THEN [kết quả mong đợi]

## Ghi Chú Triển Khai

## Tham Chiếu

## Ghi Chú
```
