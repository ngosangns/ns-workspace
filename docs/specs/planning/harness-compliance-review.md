---
type: planning
title: "Harness: đánh giá compliance với docs và kế hoạch cải tiến"
description: "Đối chiếu implementation internal/harness với docs/modules/harness.md, docs/features/agentic-loop.md, AGENTS.md và skill harness/loop; liệt kê deviation và plan sửa theo priority."
status: draft
timestamp: 2026-07-27T00:00:00Z
---

# Harness Compliance Review

Nguồn tham chiếu: `docs/modules/harness.md`, `docs/features/agentic-loop.md`,
`~/.config/opencode/AGENTS.md`, skill `harness`, `loop`, `eval`.

## Phần đã đúng với docs

- Engine API + CLI subcommands `list|run|eval|status|resume|stop` (internal/cli/harness.go).
- Task file YAML/JSON schema, default phases `plan/execute/verify`, default `max_consecutive_failures=3`.
- Evaluator kết hợp 3 nguồn: acceptance commands, `package.json` scripts (`test/lint/typecheck/build`), `go test ./...`; discovered commands là advisory.
- Dual memory store: load ưu tiên project path, save ghi cả hai; shared path có project slug.
- Dispatcher abstraction + `OpenCodeDriver` (`opencode run --dangerously-skip-permissions`) + `MockDriver`.
- CI/non-interactive: ghi `.harness/decision-request.md` rồi dừng.
- Task `enrich-docs`: hard caps code-enforced, allowlist host, fail-open warnings, confine writes trong docs root.

## Deviation (xếp theo mức độ nghiêm trọng)

### P0 — Loop không self-correct như docs tuyên bố

1. **Hypotheses/subtasks không bao giờ được populate.** Plan prompt yêu cầu
   subagent trả về "subtasks and hypotheses" dạng markdown (loop.go:263), nhưng
   output chỉ lưu raw vào `ContextNotes["last_plan"]` (loop.go:171) — không
   parse vào `state.Hypotheses`/`state.Subtasks`. Hệ quả:
   `UntriedHypotheses()` luôn rỗng → sau verify fail đầu tiên, loop pause ngay
   với "verify failed and no untried hypotheses" (loop.go:213-216) thay vì tiếp
   tục tự sửa. Docs (`agentic-loop.md`) mô tả loop tự correct qua nhiều
   hypothesis — thực tế không bao giờ xảy ra.

2. **Guardrail "state lặp lại" là dead code.** `State.Hash()` marshal toàn bộ
   struct kể cả `Iteration` (memory.go:67-73); iteration tăng mỗi vòng nên hash
   không bao giờ trùng → `HasRepeatedState()` (loop.go:63) không bao giờ true.
   Docs liệt kê "State lặp: Hash state trùng với history" là điều kiện dừng.

3. **Phase diagnose không bao giờ dispatch.** `runDiagnose` (loop.go:221) không
   có caller; loop chỉ set label `state.Phase = "diagnose"` (loop.go:100-102).
   Routing `diagnose: agent: opencode-fixer` trong ví dụ của agentic-loop.md:70
   cũng không có field tương ứng trong `Routing` struct (task.go:68-73).

### P1 — Tính năng docs mô tả nhưng thiếu/sai behavior

4. **Verify không dùng routing agent.** `verify: agent: eval-judge` bị ignore —
   verify chỉ chạy shell commands (loop.go:195-219). Không có LLM judge cho
   criteria không biểu diễn được bằng command.

5. **Interactive HITL thiếu "câu hỏi → câu trả lờirồi resume".** Hiện chỉ có
   menu resume/stop/quit (loop.go:32-52); guidance của user không được ghi vào
   state như docs mô tả ("in câu hỏi, chờ user trả lờirồi resume").

6. **`harness stop` xóa state** (engine.go:181-188, Store.Remove) thay vì đánh
   dấu paused → không resume được sau stop, mâu thuẫn với "có thể pause và
   resume". Cần chốt semantics: stop = pause (giữ state) hay stop = reset (xóa)?
   Đề xuất: `stop` = pause + flag `--reset` để xóa.

### P2 — Chất lượng / polish

7. Ambiguity detection chỉ là substring `"ambiguous"` trong plan/diagnose
   output (loop.go:235-243) — heuristic yếu, dễ false positive/negative.

8. `--dry-run` chỉ in 1 dòng "would run" (engine.go:127-130), không "in kế
   hoạch" như skill harness mô tả.

9. Eval order nondeterministic (range map trong evaluator.go:42); nên sort tên
   command để output ổn định. `Engine.Eval` nuốt error của `EvaluateAll`
   (engine.go:147).

10. Docs tự mâu thuẫn: skill `harness` nói `//hle` = harness+loop+eval còn
    AGENTS.md nói `//hle` = harness+loop+execution; skill `harness` thiếu
    `//hv`, AGENTS.md thiếu `//he`. Cần unify bảng trigger.

## Kế hoạch cải tiến

### Phase 1 (P0): làm loop self-correct thật

- [ ] 1a. Parse plan output: yêu cầu plan subagent trả về JSON block
  (`{"subtasks": [...], "hypotheses": [...]}`) kèm markdown; parse vào
  `state.Subtasks`/`state.Hypotheses` với fallback fail-open (không parse được
  thì tạo 1 hypothesis mặc định từ LastError để loop không pause sớm).
- [ ] 1b. Sửa `State.Hash()`: loại `Iteration` (và field volatile khác nếu có)
  khỏi hash, giống cách đã loại `History`; thêm test hash trùng khi state
  không đổi giữa 2 iteration.
- [ ] 1c. Dispatch thật phase diagnose: thêm `Routing.Diagnose`, gọi
  `runDiagnose` sau verify fail trước khi quay lại execute; parse hypotheses
  mới từ diagnose output (cùng cơ chế 1a).

### Phase 2 (P1): đúng behavior docs

- [ ] 2a. Verify agent: nếu `routing.verify.agent` được set và có acceptance
  entry không có command/script (vd: chỉ có `text`), dispatch LLM judge để đánh
  giá; giữ command-based eval cho phần còn lại.
- [ ] 2b. Interactive HITL: khi pause ở chế độ interactive, in `PausedReason` +
  câu hỏi, đọc free-form answer vào `ContextNotes["user_guidance"]` để phase
  tiếp theo dùng trong prompt.
- [ ] 2c. Đổi `harness stop` → đánh dấu `Paused=true, reason="stopped by user"`;
  thêm `harness reset` (hoặc `stop --reset`) cho hành vi xóa state hiện tại.

### Phase 3 (P2): polish + docs

- [ ] 3a. Dry-run in kế hoạch: phases, agents đã resolve, eval commands sẽ chạy.
- [ ] 3b. Sort eval commands theo tên; `Engine.Eval` trả error khi có must_pass
  fail (hoặc ít nhất propagate lỗi chạy).
- [ ] 3c. Thay heuristic "ambiguous" bằng marker có cấu trúc (vd: subagent trả
  `AMBIGUITY:` line hoặc JSON field) + giữ substring làm fallback.
- [ ] 3d. Unify bảng trigger `//h*` giữa AGENTS.md và skill `harness`/`loop`;
  cập nhật `docs/modules/harness.md` + `docs/features/agentic-loop.md` nếu
  behavior thay đổi sau Phase 1-2.

## Acceptance đề xuất

- `go test ./internal/harness/ ./internal/cli/` pass, kèm test mới cho:
  hash-repeat detection, plan-output parsing, diagnose dispatch, stop/resume.
- Chạy thử 1 task mẫu có acceptance fail ban đầu → loop tự qua ít nhất 1 chu
  kỳ diagnose→execute trước khi finalize/pause (không pause ngay sau verify
  fail đầu tiên).
