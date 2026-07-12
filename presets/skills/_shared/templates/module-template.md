# Mẫu Module Doc

Dùng cho module docs trong cả hai cây audience:

- `docs/business/modules/<module>/*.md` — business view: purpose, contract, business rules.
- `docs/developer/modules/<module>/*.md` — technical view: boundary, API, dependencies, invariants.

Bắt đầu bằng frontmatter OKF (xem `frontmatter-schema.md`):

```markdown
---
type: module
audience: business | developer
title: "[Tên Module]"
description: "[Một câu mô tả boundary của module từ góc nhìn audience]"
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

Dùng link OKF bundle-relative tới docs liên quan, ví dụ [Chỉ mục developer](/developer/_index.md) hoặc [Chỉ mục business](/business/_index.md).

## Quyết Định Liên Quan
```
