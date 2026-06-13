# Module Harness

## Meta

- **Status**: active
- **Description**: Module harness cung cấp engine, task registry, evaluator, loop controller, dispatcher và memory store cho `ns-workspace` CLI.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Feature agentic loop](../features/agentic-loop.md), [Spec harness looping agentic](../specs/planning/harness-looping-agentic.md)

## Tổng Quan

Module `internal/harness` quản lý vòng đở của một harness task: đọc task, chạy eval, điều phối loop, dispatch subagent và lưu state.

## Thành Phần

| File              | Vai trò                                |
| ----------------- | -------------------------------------- |
| `task.go`         | Định nghĩa task struct, load YAML/JSON |
| `memory.go`       | State struct và dual memory store      |
| `evaluator.go`    | Chạy eval commands từ nhiều nguồn      |
| `dispatcher.go`   | Subagent driver abstraction            |
| `loop.go`         | Loop controller và guardrails          |
| `engine.go`       | Harness engine và CLI-facing API       |
| `harness_test.go` | Tests                                  |

## Engine API

```go
engine := harness.NewEngine(projectRoot, reporter)
engine.ListTasks()
engine.LoadTask(id)
engine.Run(ctx, id, dryRun)
engine.Eval(id)
engine.Status(id)
engine.Resume(ctx, id)
engine.Stop(id)
```

## Task File

Task định nghĩa trong `.harness/tasks/<id>.yaml` hoặc `.json`:

```yaml
id: sample
description: Sample task
requirements:
  - id: REQ-1
    text: Do something
scope:
  include:
    - internal/**
acceptance:
  - command: go test ./...
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
stopping:
  max_consecutive_failures: 3
  require_human_on_ambiguity: true
```

## Evaluator

Evaluator kết hợp:

1. Task-defined acceptance commands/scripts
2. `package.json` scripts: `test`, `lint`, `typecheck`, `build`
3. `go test ./...` mặc định

## Dispatcher

`SubagentDriver` là interface để dispatch subagent. Hiện tại có:

- `OpenCodeDriver`: gọi `opencode run --dangerously-skip-permissions`
- `MockDriver`: dùng trong tests

## Memory

Dual store:

- Project path: `.harness/state/<id>.json`
- Shared path: `~/.agents/harness/<project>/<id>.json`

Load ưu tiên project path, fallback shared path. Save ghi cả hai.

## Loop Controller

Luồng phase:

```mermaid
flowchart LR
    Plan[Plan] --> Execute[Execute]
    Execute --> Verify[Verify]
    Verify -->|PASS| Final[Finalize]
    Verify -->|FAIL| Diagnose[Diagnose/Research/Fix]
    Diagnose --> Execute
```

Guardrails:

- Verify pass
- State lặp lại
- Hết hypothesis
- Consecutive failures vượt ngưỡng
- Ambiguity
- Acceptance criteria thỏa mãn
- Subtasks hoàn thành

## Quan Hệ

- `internal/cli/harness.go` gọi `internal/harness`.
- `main.go` route command `harness`.
- Preset skills/subagents cung cấp instruction cho AI agents.
