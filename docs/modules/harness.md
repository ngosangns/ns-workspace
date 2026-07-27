---
type: module
title: "Module Harness"
description: "Module harness cung cấp engine, task registry, evaluator, loop controller, dispatcher và memory store cho `ns-workspace` CLI."
tags: ["module", "harness"]
timestamp: 2026-06-23T00:00:00Z
status: active
compliance: current-state
---

# Module Harness

## Meta

- **Status**: active
- **Description**: Module harness cung cấp engine, task registry, evaluator, loop controller, dispatcher và memory store cho `ns-workspace` CLI.
- **Compliance**: current-state
- **Links**: [Chỉ mục](../_index.md), [Feature agentic loop](../features/agentic-loop.md)

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
| `enrich.go`       | Task type `enrich-docs` với hard caps  |
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
engine.Stop(id)  // pause và lưu state để resume
engine.Reset(id) // xóa state
```

## CLI

```bash
go run . harness run --task refactor-auth --project . [--dry-run]
go run . harness eval --task refactor-auth --project .
go run . harness status --task refactor-auth --project .
go run . harness resume --task refactor-auth --project .
go run . harness stop --task refactor-auth --project .   # pause
go run . harness reset --task refactor-auth --project .  # xóa state
```

`--dry-run` in phases sẽ chạy, agents đã resolve và danh sách eval commands; không spawn subagent và không ghi state.

## Goal Mode

`harness run --goal "<mục tiêu>"` materialize mục tiêu ngôn ngữ tự nhiên thành
task file qua `MaterializeGoalTask` (`goal.go`) rồi chạy loop như task viết tay:

- **Slug**: kebab-case từ goal, tối đa 6 từ có nghĩa, bỏ stopword; trùng id
  nhưng khác nội dung → suffix `-2`, `-3`. Cùng nội dung → reuse (idempotent).
- **Template**: routing mặc định `opencode-planner` / `opencode-executor` /
  `eval-judge` / `opencode-fixer`; `max_consecutive_failures: 3`;
  `require_human_on_ambiguity: true`.
- **Acceptance để trống** có chủ đích: evaluator auto-discover
  `go test ./...` và package.json scripts, nên goal task chạy được ngay trên
  repo Go/Node.
- Flags: `--scope "a/**,b/**"` siết scope (mặc định `**`); `--goal-refine` nhờ
  plan subagent đề xuất requirements/scope chi tiết trước khi ghi file.

## Task File

Task định nghĩa trong `.harness/tasks/<id>.yaml` hoặc `.json`:

```yaml
id: sample
description: Sample task
type: refactor
requirements:
  - id: REQ-1
    text: Do something
scope:
  include:
    - internal/**
acceptance:
  - command: go test ./...
    must_pass: true
  - text: "code follows project conventions" # được đánh giá bởi verify agent
    must_pass: false
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
  diagnose:
    agent: opencode-fixer
memory:
  project_path: .harness/state/sample.json
  shared_path: ~/.agents/harness/<project>/sample.json
stopping:
  max_consecutive_failures: 3
  require_human_on_ambiguity: true
```

## Evaluator

Evaluator kết hợp:

1. Task-defined acceptance commands/scripts
2. Task-defined acceptance `text` (đánh giá bởi verify agent qua LLM judge)
3. `package.json` scripts: `test`, `lint`, `typecheck`, `build`
4. `go test ./...` mặc định

Các acceptance entry có `must_pass: true` phải pass thì `engine.Eval` mới trả về
không lỗi. Các lệnh eval chạy theo thứ tự tên được sort để output ổn định.

## Dispatcher

`SubagentDriver` là interface để dispatch subagent. Hiện tại có:

- `OpenCodeDriver`: gọi `opencode run --dangerously-skip-permissions`
- `MockDriver`: dùng trong tests

Routing hỗ trợ `plan`, `execute`, `verify` và `diagnose`. Nếu phase `verify` có agent
riêng, các acceptance `text` sẽ được gửi cho agent đó dưới dạng prompt judge.

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

Phase `diagnose` được dispatch thật qua agent được chỉ định trong
`routing.diagnose`. Output của diagnose được parse cùng schema với plan để bổ
sung hypotheses/subtasks mới.

Guardrails:

- Verify pass
- State lặp lại
- Hết hypothesis
- Consecutive failures vượt ngưỡng
- Ambiguity (phát hiện qua marker `AMBIGUITY: <câu hỏi>` hoặc fallback từ khóa)
- Acceptance criteria thỏa mãn
- Subtasks hoàn thành

Interactive pause cho phép user nhập free-form guidance qua option `[a] answer`;
guidance được lưu vào state và đưa vào prompt phase tiếp theo. CI pause ghi
`.harness/decision-request.md` như cũ.

## Task `enrich-docs`

Khi `task.Type == "enrich-docs"`, loop controller chạy nhánh enrichment riêng (`runEnrich` trong `enrich.go`) ngay từ phase plan; phase execute là no-op còn phase verify vẫn chạy acceptance command như thường. Luồng: plan (LLM đề xuất URL ứng viên từ seeds) → fetch (guarded, fail-open) → execute (LLM tổng hợp corpus thành doc change JSON) → write (ghi file giới hạn trong docs root).

Cấu hình qua `EnrichConfig` trong task:

```yaml
type: enrich-docs
enrich:
  seeds:
    - url: https://example.com/guide
    - file: docs/seed-notes.md
  caps:
    max_pages: 10
    max_depth: 1
    allowed_hosts:
      - example.com
    fetch_timeout_seconds: 15
  target:
    mode: references # references | enrich
    references_dir: docs/references
```

Hard caps là code-enforced, không dựa vào LLM tự giới hạn:

- `max_pages`: chặn số trang fetch (mặc định 10 khi <= 0).
- `allowed_hosts` ∪ host của seeds: chỉ fetch URL trong allowlist; redirect đổi host bị chặn.
- `max_depth`: giới hạn link-follow; depth 0 chỉ fetch seeds + URL do plan đề xuất.
- `fetch_timeout_seconds`: timeout mỗi fetch (mặc định 15s), body cap 5 MiB.

Fetch lỗi/timeout hay URL ngoài allowlist được ghi vào `state.Warnings` qua `State.AddWarning` và loop tiếp tục (fail-open). Mode `references` tạo doc mới trong `references_dir` với frontmatter `type: reference`; mode `enrich` chỉ sửa doc đã tồn tại. Mọi đường ghi đều đi qua `confineToDocsRoot` để chặn path traversal ra ngoài docs root.

## Quan Hệ

- `internal/cli/harness.go` gọi `internal/harness`.
- `main.go` route command `harness`.
- Preset skills/subagents cung cấp instruction cho AI agents.
