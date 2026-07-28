# Agent Instructions

## Skills

Agent chọn skill theo ý định của user (không dùng short-tag / trigger prefix).

### Workflow / dev

| Skill | Khi Dùng |
| ----- | -------- |
| `cleanup` | Audit worktree/branch/commit; lập plan cleanup cho dead code, dead flows, legacy artifacts. |
| `execution` | Triển khai thay đổi đã được duyệt hoặc task nhỏ đã rõ theo kiến trúc hiện tại của repo. |
| `fix` | Chẩn đoán và sửa bug, failing test, regression hoặc lỗi runtime đã có triệu chứng cụ thể. |
| `plan` | Tạo kế hoạch cho công việc lớn, chờ user duyệt trước khi sửa code. |
| `research` | Phân tích, nghiên cứu và làm rõ yêu cầu trước khi triển khai. |
| `init` | Khởi tạo hiểu biết repo: quét codebase, lập aspect inventory (không ghi `docs/`). |
| `working-document` | Tài liệu hóa thay đổi từ commit/branch (walkthrough code). |
| `lsp-code-graph` | Tìm symbol, caller/callee, references qua Code Graph / LSP. |

### Harness / agentic loop

| Skill | Khi Dùng |
| ----- | -------- |
| `goal` | Nhận mục tiêu ngôn ngữ tự nhiên, materialize thành harness task rồi chạy harness loop tới khi xong hoặc pause. |
| `harness` | Chạy harness để đánh giá và kiểm chứng task qua subagent. |
| `loop` | Kích hoạt looping agentic self-correct với multi-agent routing và memory persistence. |
| `eval` | Chạy evaluator để đánh giá task/skill/subagent theo acceptance criteria. |

### Commit (registry)

Preset local `commit` đã được gỡ. Commit dùng skill registry `git-commit`
(`github/awesome-copilot`, cài qua `npx skills` khi sync registry): analyze
diff, Conventional Commits, stage có chủ đích, an toàn.

### Pipeline gợi ý

- Task lớn: `research` → `plan` → (user duyệt) → `execution`. Dừng sau `plan` cho đến khi user duyệt rõ ràng trước khi sửa source code.
- Bug có triệu chứng: `fix` (hoặc `research` nếu còn mơ hồ).
- Cleanup: `cleanup` → `plan` → (user duyệt) → `execution`.
- Goal tự chạy: `goal` / `harness` + `loop` (+ `eval` khi cần).

## Hướng Dẫn Sử Dụng Harness, Loop Và Eval

### Khi nào dùng cái gì

| Pipeline | Khi nào dùng |
| -------- | ------------ |
| harness | Cần chạy kiểm chứng một task cụ thể theo acceptance criteria. |
| loop | Kích hoạt self-correct loop (plan → execute → verify → diagnose). |
| harness + loop | Task đã có task file `.harness/tasks/<id>.yaml`, cần loop tự chạy. |
| harness + eval | Đánh giá kết quả cuối theo tiêu chí khách quan. |
| harness + loop + eval | Vừa chạy loop vừa eval tổng thể sau mỗi iteration. |
| harness + loop + execution | Nếu loop dừng ở trạng thái pause, tiếp tục execution trực tiếp. |

### Task file

Định nghĩa task trong `.harness/tasks/<id>.yaml`:

```yaml
id: refactor-auth
description: Refactor auth module
requirements:
  - id: REQ-1
    text: Extract password hashing to separate package
scope:
  include:
    - internal/auth/**
acceptance:
  - command: go test ./internal/auth/...
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

Acceptance criteria hỗ trợ command/script hoặc tham chiếu `package.json` scripts
(`test`, `lint`, `typecheck`, `build`).

### Các lệnh harness

```bash
go run . harness list --project .
go run . harness run --task refactor-auth --project .
go run . harness status --task refactor-auth --project .
go run . harness resume --task refactor-auth --project .
go run . harness stop --task refactor-auth --project .
go run . harness run --task refactor-auth --project . --dry-run
```

State lưu ở `.harness/state/<id>.json` (project) và `~/.agents/harness/<project>/<id>.json`
(shared), giúp resume giữa các môi trường.

### Luồng loop

```text
Plan → Execute → Verify → [PASS] → Finalize
                |
                [FAIL] → Diagnose/Research/Fix → Execute
```

Loop dừng khi: verify pass, state lặp lại, hết hypothesis, vượt
`max_consecutive_failures`, phát hiện ambiguity, hoặc acceptance criteria thỏa
mãn.

### Human-in-the-loop

- **Interactive**: pause terminal, in câu hỏi, chờ user trả lời rồi resume.
- **CI/non-interactive**: ghi `.harness/decision-request.md` và dừng, user chỉnh
  rồi `harness resume`.
