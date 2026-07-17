# Mẫu Spec

Dùng cho specs/plans trong layout flat:

- `docs/specs/planning/*.md` — design plans, technical specs, acceptance notes.

```markdown
---
type: planning
title: "[Tính năng hoặc thay đổi]"
description: "[Mô tả ngắn]"
tags: [planning]
timestamp: <ISO 8601>
status: draft | proposed | approved | implemented
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
