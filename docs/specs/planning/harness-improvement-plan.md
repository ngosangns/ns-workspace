---
type: planning
title: "Harness: kế hoạch cải tiến chi tiết"
description: "Plan triển khai chi tiết theo file/function cho các deviation đã xác định trong harness-compliance-review.md."
status: draft
timestamp: 2026-07-27T00:00:00Z
links: [docs/specs/planning/harness-compliance-review.md]
---

# Harness Improvement Plan (chi tiết)

Phụ thuộc: đọc `harness-compliance-review.md` trước. Mỗi item gồm: file sửa,
thay đổi cụ thể, test bắt buộc, risk.

---

## Phase 1 — Loop self-correct thật (P0)

### 1a. Parse plan output vào Hypotheses/Subtasks

**File**: `internal/harness/loop.go`, mới `internal/harness/parse.go`

- Thêm struct trung gian:

```go
type planOutput struct {
    Subtasks    []subtaskJSON    `json:"subtasks"`
    Hypotheses  []hypothesisJSON `json:"hypotheses"`
}
```

- `buildPlanPrompt` (loop.go:245): đổi instruction cuối thành yêu cầu trả về 1
  fenced block ```json với schema trên (markdown giải thích được phép kèm
  trước block).
- Mới `parse.go`:
  - `ExtractJSONBlock(s string) (string, bool)`: tìm fenced ```json block cuối
    cùng; fallback: tìm substring từ `{` đầu tới `}` cuối.
  - `ParsePlanOutput(s string, state *State)`: unmarshal, merge vào
    `state.Subtasks`/`state.Hypotheses` (skip ID đã tồn tại). Fail-open:
    unmarshal lỗi → tạo 1 hypothesis mặc định `{ID: "auto-N", Description:
    state.LastError hoặc "continue from plan"}` để `UntriedHypotheses()` không
    rỗng sau verify fail đầu tiên.
- `runPlan` (loop.go:154): sau khi lưu `ContextNotes["last_plan"]`, gọi
  `ParsePlanOutput`.
- `runExecute` (loop.go:175): sau dispatch thành công, đánh dấu hypothesis đầu
  tiên trong `UntriedHypotheses()` là `Tried` (qua `MarkHypothesisTried`) và
  parse output tương tự để cập nhật `Subtasks.Done` nếu subagent báo xong.

**Tests** (`parse_test.go`, mở rộng `harness_test.go`):
- Parse fenced block đúng, parse fallback `{...}`, parse lỗi → hypothesis
  mặc định được tạo.
- Merge không trùng ID khi plan chạy lại ở iteration 2.
- Integration: verify fail lần 1 → loop KHÔNG pause, quay lại execute với
  hypothesis untried (dùng MockDriver).

**Risk**: subagent không theo schema → đã có fallback fail-open; không block
loop.

### 1b. Sửa `State.Hash()` để repeat detection hoạt động

**File**: `internal/harness/memory.go:67-73`

- Trong `Hash()`: ngoài `History = nil`, thêm `Iteration = 0` (và
  `Paused/PausedReason = zero` nếu muốn hash thuần "work state"). Giữ nguyên
  `Phase`, `ConsecutiveFailures`, `Hypotheses`, `Subtasks`, `ContextNotes`.

**Tests**: hai snapshot liên tiếp với cùng work-state nhưng khác Iteration →
`HasRepeatedState() == true`; state thay đổi (hypothesis tried) → false.

**Risk**: hash nhạy với `ContextNotes["last_plan"]` chứa timestamp/text LLM
khác nhau mỗi lần → repeat state hiếm khi trigger trong thực tế. Chấp nhận
được (guardrail này là safety net); có thể loại `ContextNotes` khỏi hash nếu
test thực tế cho thấy quá nhạy — quyết định khi implement, ghi chú trong code.

### 1c. Dispatch phase diagnose

**File**: `internal/harness/task.go:68-73`, `internal/harness/loop.go`

- `task.go`: thêm `Diagnose RoutingRule` vào `Routing`; `SelectAgent` thêm
  case `"diagnose"`.
- `loop.go`:
  - Trong `runVerify` trả về thêm thông tin fail; sau block `case "verify":`
    khi `state.ConsecutiveFailures >= 1` và còn budget
    (`< maxFail`), gọi thật `lc.runDiagnose(ctx, task, state)` thay vì chỉ
    set label. Diagnose lỗi → ghi `LastError`, KHÔNG tăng
    `ConsecutiveFailures` (tránh double-count với verify fail).
  - `runDiagnose` (loop.go:221): đổi `SelectAgent("execute")` →
    `SelectAgent("diagnose")`; sau dispatch, parse output bằng
    `ParsePlanOutput` (tái dùng 1a) để bổ sung hypotheses mới và đánh dấu
    hypothesis đã exhausted (`Tried=true`).
  - `buildDiagnosePrompt`: bổ sung danh sách hypotheses hiện có + yêu cầu
    output JSON block cùng schema.

**Tests**: MockDriver trả diagnose output có hypotheses mới → iteration sau
`UntriedHypotheses()` chứa hypotheses mới; routing `diagnose.agent` được tôn
trọng.

**Risk**: tăng 1 LLM call mỗi chu kỳ fail → chi phí; mitigated bởi
`max_consecutive_failures`.

---

## Phase 2 — Đúng behavior docs (P1)

### 2a. Verify agent (LLM judge)

**File**: `internal/harness/task.go`, `internal/harness/evaluator.go`,
`internal/harness/loop.go`

- `task.go`: `Acceptance` thêm field `Text string` (criteria tự do, không có
  command/script).
- `loop.go` `runVerify`: tách acceptance có command → Evaluator như hiện tại;
  acceptance chỉ có `Text` → nếu `routing.verify.agent` được set thì dispatch
  judge prompt (criteria + `LastError` + `last_execute` output), parse verdict
  `PASS`/`FAIL` dòng đầu. Không có agent → coi criteria text là advisory,
  skip.
- Kết quả judge merge vào `state.AcceptanceStatus["judge-N"]`.

**Tests**: task có acceptance text-only + MockDriver verdict PASS/FAIL.

### 2b. Interactive HITL capture guidance

**File**: `internal/harness/loop.go:28-62`

- Menu pause: thêm option `[a] answer` — đọc free-form input (bufio, 1 dòng
  hoặc kết thúc bằng dòng trống), lưu vào
  `ContextNotes["user_guidance"]`, unpause và tiếp tục.
- `buildPlanPrompt`/`buildExecutePrompt`: nếu có `user_guidance`, chèn vào
  prompt và xóa sau khi dùng (tránh lặp).

**Tests**: khó test stdin → tách hàm `readGuidance(r io.Reader) string` để
test; menu giữ nguyên behavior cũ khi chọn r/s/q.

### 2c. `harness stop` = pause, thêm reset

**File**: `internal/harness/engine.go:181-188`, `internal/cli/harness.go`

- `Engine.Stop`: load state → set `Paused=true, PausedReason="stopped by
  user"` → Save (không Remove).
- Thêm `engine.Reset(id)` gọi `store.Remove()`; CLI thêm subcommand `reset`
  (hoặc flag `stop --reset`). Cập nhật `validSubcommands`.
- `Resume` vốn đã bỏ pause → flow stop→resume hoạt động.

**Tests**: stop rồi status thấy `paused=true`; stop rồi resume chạy tiếp;
reset xóa cả 2 file state.

**Risk**: user quen với stop=xóa cũ → ghi rõ trong docs + output CLI gợi ý
`harness reset`.

---

## Phase 3 — Polish + docs (P2)

### 3a. Dry-run in kế hoạch

**File**: `internal/harness/engine.go:127-130`

- In: phases đã resolve, agent từng phase (`SelectAgent`), danh sách eval
  commands (`evaluator.buildCommands` — cần export hoặc method public
  `PlannedCommands(task)`), memory paths.

**Tests**: dry-run output chứa phase list + command list; không ghi state.

### 3b. Eval deterministic + propagate error

**File**: `internal/harness/evaluator.go:42`, `internal/harness/engine.go:141-149`

- `EvaluateAll`: sort keys của `commands` trước khi chạy.
- `Engine.Eval`: trả error khi bất kỳ must_pass nào fail (hoặc trả
  `ErrMustPassFailed` kèm results — CLI in results rồi exit non-zero).

**Tests**: thứ tự output ổn định qua 2 lần chạy; eval fail → error non-nil.

### 3c. Ambiguity marker có cấu trúc

**File**: `internal/harness/loop.go:235-243`, prompt builders

- Plan/diagnose prompt yêu cầu subagent in dòng `AMBIGUITY: <câu hỏi>` khi
  cần quyết định. `detectAmbiguity` ưu tiên parse dòng này (lưu câu hỏi vào
  `PausedReason`), fallback substring `"ambiguous"` như cũ.

**Tests**: output có `AMBIGUITY:` → pause với reason chứa câu hỏi.

### 3d. Docs sync

- Unify bảng trigger `//h*` giữa `~/.config/opencode/AGENTS.md` và skill
  `harness`/`loop` (quyết định: `//hle` = harness+loop+execution theo
  AGENTS.md, sửa skill harness).
- Cập nhật `docs/modules/harness.md`: thêm diagnose routing, stop/reset
  semantics, dry-run output, verify agent.
- Cập nhật `docs/features/agentic-loop.md`: sơ đồ thêm cạnh diagnose dispatch
  thật, bổ sung stop/reset.

---

## Thứ tự triển khai đề xuất

| Bước | Item | Lệ thuộc |
| ---- | ---- | -------- |
| 1 | 1b (Hash) | độc lập, nhỏ nhất |
| 2 | 1a (parse plan output) | nền cho 1c, 2b |
| 3 | 1c (diagnose dispatch) | cần 1a |
| 4 | 2c (stop/reset) | độc lập |
| 5 | 2a (verify agent) | độc lập |
| 6 | 2b (HITL guidance) | cần 1a (prompt builder) |
| 7 | 3a-3c (polish) | sau 1-2 |
| 8 | 3d (docs sync) | cuối cùng, theo behavior thật |

Mỗi bước là 1 commit riêng (Conventional Commits), kèm tests.

## Acceptance tổng

- `go test ./...` pass; coverage không giảm ở `internal/harness`.
- Scenario end-to-end (MockDriver): task có acceptance fail lần 1 → loop đi
  `plan → execute → verify(fail) → diagnose → execute → verify(pass) →
  finalized`, state có ≥1 hypothesis tried, không pause sớm.
- `harness stop` → `status` thấy paused → `resume` chạy tiếp → `reset` xóa
  state.
- Dry-run in đủ phases/agents/commands mà không tạo file state.
