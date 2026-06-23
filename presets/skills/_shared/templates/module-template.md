# Mẫu Module Doc

Dùng cho `docs/modules/*.md`. Bắt đầu bằng frontmatter OKF (xem `frontmatter-schema.md`):

```markdown
---
type: module
title: "[Tên Module]"
description: "[Một câu mô tả boundary của module]"
tags: ["module"]
timestamp: <ISO 8601>
status: active
compliance: current-state
---

# [Tên Module]

## Tổng Quan

## Yêu Cầu Chức Năng Và Phi Chức Năng

## Data Models Và APIs

## Quy Tắc Nghiệp Vụ

## Ràng Buộc Và Giả Định

## Quan Hệ

Dùng link OKF bundle-relative tới docs liên quan, ví dụ [Chỉ mục](/_index.md).

## Quyết Định Liên Quan
```
