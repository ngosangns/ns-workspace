---
name: eval
description: Chạy evaluator để đánh giá skill, subagent hoặc task theo acceptance criteria. Trigger //v.
---

# Eval

Dùng để chạy đánh giá (eval) độc lập mà không cần chạy full loop.

## Trigger

- `//v`: gọi eval
- `//hv`: harness + eval
- `//hlv`: harness + loop + eval

## CLI

```bash
go run . harness eval --task <id> --project <path>
```

## Evaluator

Evaluator kết hợp các nguồn command:

1. Task-defined acceptance commands/scripts
2. `package.json` scripts: `test`, `lint`, `typecheck`, `build`
3. `go test ./...` mặc định

Kết quả trả về:

```text
name: passed exit_code stdout/stderr
```

## Nguyên Tắc

- `must_pass: true` sẽ làm toàn bộ eval fail nếu command đó fail.
- Chỉ chạy đánh giá, không spawn subagent, không thay đổi state.
- Dùng để kiểm tra nhanh trước khi chạy loop dài.

## Ví Dụ Task

```yaml
id: sample
acceptance:
  - command: go test ./...
    must_pass: true
  - script: scripts/check-coverage.sh
    must_pass: true
```

```bash
go run . harness eval --task sample --project .
```
