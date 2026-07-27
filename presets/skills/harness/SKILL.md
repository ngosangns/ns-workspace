---
name: harness
description: Chạy harness để đánh giá và kiểm chứng task. Trigger khi cần chạy test/eval harness, kiểm tra acceptance criteria hoặc spawn subagent đánh giá. Trigger //h.
---

# Harness

Dùng để chạy và đánh giá task qua bộ harness của `ns-workspace`.

## Trigger

- `//h`: gọi harness
- `//hv`: harness + eval
- `//hl`: harness + loop
- `//hlv`: harness + loop + eval
- `//hle`: harness + loop + execution

## CLI

```bash
go run . harness list
go run . harness run --task <id> --project <path> [--dry-run]
go run . harness run --goal "<mục tiêu>" --project <path> [--scope "a/**"] [--goal-refine]
go run . harness eval --task <id> --project <path>
go run . harness status --task <id> --project <path>
go run . harness resume --task <id> --project <path>
go run . harness stop --task <id> --project <path>   # pause
go run . harness reset --task <id> --project <path>  # xóa state
```

## Task File

Task định nghĩa trong `.harness/tasks/<id>.yaml` hoặc `.json`:

```yaml
id: refactor-auth
description: Refactor auth module
domain: backend
type: refactor
requirements:
  - id: REQ-1
    text: Separate auth logic from handler
scope:
  include:
    - internal/auth/**
  exclude:
    - internal/auth/_legacy/**
acceptance:
  - command: go test ./internal/auth/...
    must_pass: true
  - command: go run . lint
    must_pass: true
phases:
  - plan
  - execute
  - verify
routing:
  default: opencode
  plan:
    agent: opencode-planner
  execute:
    agent: opencode-executor
  verify:
    agent: eval-judge
memory:
  project_path: .harness/state/refactor-auth.json
  shared_path: ~/.agents/harness/<project>/refactor-auth.json
stopping:
  max_consecutive_failures: 3
  require_human_on_ambiguity: true
```

## Nguyên Tắc

- Evaluator kết hợp command từ task, `package.json` scripts và `go test ./...`.
- State lưu ở cả project path và shared path để resume và share.
- `--dry-run` chỉ in kế hoạch, không chạy subagent.
- Dừng khi verify pass, state lặp, hết hypothesis, hoặc phát hiện ambiguity.
