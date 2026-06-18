# Agent Instructions

## Trigger Skills

Agent phải nhận diện trigger skill được viết ở đầu message của user theo cú pháp:

```text
//<short-tag-of-skill>
```

Riêng trigger `//s` hoặc `/s` ở đầu message là trigger tắt cho skill
`spawn-opencode`, dùng để spawn OpenCode process như sub-agent.

Trigger có thể chứa một tag hoặc nhiều tag ghép liền nhau. Khi có nhiều tag, áp
dụng các skill tương ứng theo đúng thứ tự chữ cái trong trigger.

Ví dụ:

```text
//rpe add account notifications
```

Nghĩa là: chạy `read-search-docs` như bước search, sau đó chạy `plan`, rồi chạy
`execution`.

## Short Tags Cho Skill Local

| Trigger | Skill              | Khi Dùng                                                                                                     |
| ------- | ------------------ | ------------------------------------------------------------------------------------------------------------ |
| `//c`   | `commit`           | Chuẩn bị và tạo git commit an toàn cho thay đổi hiện tại, với phạm vi staged rõ ràng và message súc tích.    |
| `//d`   | `cleanup`          | Quét diff/work đã triển khai/branch/commit, đọc docs và lập plan cleanup dead code, dead flows, dead docs.   |
| `//e`   | `execution`        | Triển khai thay đổi đã được duyệt hoặc task nhỏ đã rõ theo kiến trúc hiện tại của repo.                      |
| `//f`   | `fix`              | Chẩn đoán và sửa bug, failing test, regression hoặc lỗi runtime đã có triệu chứng cụ thể.                    |
| `//h`   | `harness`          | Chạy harness để đánh giá và kiểm chứng task qua subagent.                                                    |
| `//i`   | `init`             | Khởi tạo knowledge base: quét codebase, lập aspect inventory markdown cho ngườimới, rồi cập nhật docs.       |
| `//l`   | `loop`             | Kích hoạt looping agentic self-correct với multi-agent routing và memory persistence.                        |
| `//r`   | `read-search-docs` | Search/đọc docs và specs, không sửa file.                                                                    |
| `//s`   | `spawn-opencode`   | Spawn OpenCode process như sub-agent cho research, review, triển khai hoặc làm việc song song có phạm vi rõ. |
| `//p`   | `plan`             | Tạo hoặc cập nhật file planning cho task lớn và chờ user duyệt trước khi sửa source.                         |
| `//u`   | `update-docs`      | Cập nhật docs/specs, gồm cả `requirements.md` của feature/module folder khi user yêu cầu.                    |
| `//v`   | `eval`             | Chạy evaluator để đánh giá task/skill/subagent theo acceptance criteria.                                     |
| `/s`    | `spawn-opencode`   | Spawn OpenCode process như sub-agent cho research, review, triển khai hoặc làm việc song song có phạm vi rõ. |

## Trigger Ghép

Các trigger ghép thường dùng:

| Trigger | Pipeline                                                                                                          |
| ------- | ----------------------------------------------------------------------------------------------------------------- |
| `//ec`  | Execution thay đổi code, rồi commit nếu diff đúng phạm vi và validation phù hợp đã chạy hoặc được nêu rõ.         |
| `//uc`  | Update docs/specs, rồi commit nếu diff đúng phạm vi và validation phù hợp đã chạy hoặc được nêu rõ.               |
| `//rf`  | Search docs/specs liên quan, rồi fix theo nguồn tham chiếu hiện có.                                               |
| `//sf`  | Spawn OpenCode sub-agent, rồi fix khi đã đủ bối cảnh.                                                             |
| `//fu`  | Fix lỗi, rồi update docs nếu behavior, architecture, business rules hoặc quan hệ module thay đổi.                 |
| `//rp`  | Search docs, rồi tạo plan.                                                                                        |
| `//ru`  | Search docs, rồi cập nhật docs/specs theo phạm vi đã xác định.                                                    |
| `//rpe` | Search docs, tạo plan, rồi execution sau khi được duyệt nếu task cần.                                             |
| `//re`  | Search docs, rồi execution cho thay đổi nhỏ đã rõ.                                                                |
| `//spe` | Spawn OpenCode sub-agent, tạo plan, rồi execution sau khi được duyệt nếu task cần.                                |
| `//eu`  | Execution thay đổi code, rồi update docs nếu behavior, architecture, business rules hoặc quan hệ module thay đổi. |
| `//hl`  | Chạy harness looping agentic: plan → execute → verify → self-correct.                                             |
| `//hv`  | Chạy harness rồi eval kết quả.                                                                                    |
| `//hlv` | Chạy harness looping agentic, sau đó eval tổng thể.                                                               |
| `//hle` | Chạy harness looping agentic, sau đó execution trực tiếp nếu loop dừng ở trạng thái pause.                        |

Nếu trigger ghép có `plan` đứng trước `execution` cho task lớn, dừng lại sau
bước plan và chờ user duyệt rõ ràng trước khi sửa source code.

## Hướng Dẫn Sử Dụng Harness, Loop Và Eval

### Khi nào dùng cái gì

| Trigger | Pipeline                   | Khi nào dùng                                                       |
| ------- | -------------------------- | ------------------------------------------------------------------ |
| `//h`   | harness                    | Cần chạy kiểm chứng một task cụ thể theo acceptance criteria.      |
| `//l`   | loop                       | Kích hoạt self-correct loop (plan → execute → verify → diagnose).  |
| `//hl`  | harness + loop             | Task đã có task file `.harness/tasks/<id>.yaml`, cần loop tự chạy. |
| `//hv`  | harness + eval             | Đánh giá kết quả cuối theo tiêu chí khách quan.                    |
| `//hlv` | harness + loop + eval      | Vừa chạy loop vừa eval tổng thể sau mỗi iteration.                 |
| `//hle` | harness + loop + execution | Nếu loop dừng ở trạng thái pause, tiếp tục execution trực tiếp.    |

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

- **Interactive**: pause terminal, in câu hỏi, chờ user trả lờirồi resume.
- **CI/non-interactive**: ghi `.harness/decision-request.md` và dừng, user chỉnh
  rồi `harness resume`.
