---
type: planning
title: "Skill /goal: implementation sử dụng harness"
description: "Kế hoạch triển khai skill /goal: nhận mục tiêu ngôn ngữ tự nhiên, materialize thành harness task, chạy harness loop và báo cáo kết quả."
status: draft
timestamp: 2026-07-27T00:00:00Z
links: [docs/modules/harness.md, docs/features/agentic-loop.md, docs/specs/planning/harness-improvement-plan.md]
---

# Skill /goal — Implementation Dùng Harness

## Bối cảnh & nguyên nhân gốc rễ

Hiện tại để chạy harness, user phải tự viết task file `.harness/tasks/<id>.yaml`
đúng schema (requirements, scope, acceptance, routing, stopping) rồi gọi
`harness run`. Đây là rào cản lớn cho luồng "tôi có một mục tiêu, hãy tự làm đến
khi xong". Skill `/goal` giải quyết bằng cách **materialize mục tiêu ngôn ngữ
tự nhiên thành harness task** rồi để harness loop (plan → execute → verify →
diagnose) tự chạy, tận dụng toàn bộ guardrails, memory persistence và HITL đã
có sẵn. Điểm then chốt: `/goal` **không re-implement loop**, chỉ là lớp
orchestration mỏng phía trên harness.

## Mục tiêu

- User gõ `/goal <mục tiêu>` → agent sinh task file hợp lệ → chạy
  `harness run` → báo cáo finalized/paused + eval results + file đã sửa.
- Task file được sinh **dự đoán được, kiểm chứng được** (template cố định +
  auto-detect acceptance từ repo), không phụ thuộc hoàn toàn vào LLM sáng tác.

## Phạm vi / Ngoài phạm vi

- **Trong phạm vi**: skill preset `goal`, CLI sugar `harness run --goal`,
  trigger registration, docs, tests.
- **Ngoài phạm vi**: thay đổi loop semantics của harness; goal dạng multi-task
  (chia 1 goal thành nhiều task); UI riêng.

## Thiết kế đề xuất

### Quyết định kiến trúc (đề xuất: phương án B, hybrid 2 lớp)

| Phương án | Mô tả | Đánh giá |
| --------- | ----- | -------- |
| A. Skill-only | SKILL.md hướng dẫn agent tự viết YAML rồi chạy CLI. Không đụng code. | Nhanh nhưng task file phụ thuộc LLM tự do, khó test, dễ sai schema. |
| B. Hybrid (khuyến nghị) | Lớp 1: CLI `harness run --goal "<text>"` materialize task file bằng code (slug, template, auto-detect acceptance) + tùy chọn refine qua plan subagent. Lớp 2: skill `/goal` chỉ gọi CLI đó và xử lý kết quả/pause. | Task file deterministic, có unit test; skill mỏng; tái dùng được từ CI. |
| C. CLI-only | Chỉ thêm `--goal`, không có skill. | Thiếu điểm chạm tự nhiên trong agent workflow. |

### Lớp 1: CLI `harness run --goal`

**File mới**: `internal/harness/goal.go` + `internal/harness/goal_test.go`

- `MaterializeGoalTask(projectRoot, goalText, opts) (*Task, path, error)`:
  - **Slug**: kebab-case từ goal (tối đa 6 từ có nghĩa, bỏ stopword tiếng
    Việt/Anh cơ bản), vd `"refactor auth module"` → `refactor-auth-module`.
    Trùng ID với task đã có → suffix `-2`, `-3`.
  - **Template Task**:
    ```yaml
    id: <slug>
    description: <goal text nguyên văn>
    requirements:
      - id: REQ-1
        text: <goal text>
    scope:
      include: ["**"]        # mặc định rộng; có thể siết qua --scope flag
    acceptance: []           # để trống → evaluator auto-discover
    routing:
      default: opencode
      plan: {agent: opencode-planner}
      execute: {agent: opencode-executor}
      verify: {agent: eval-judge}
      diagnose: {agent: opencode-fixer}
    stopping:
      max_consecutive_failures: 3
      require_human_on_ambiguity: true
    ```
  - Acceptance để trống có chủ đích: evaluator đã auto-discover
    `go test ./...` + package.json scripts (hành vi hiện có), nên goal task
    "chạy được ngay" trên repo Go/Node mà không cần đoán command.
  - Ghi file vào `.harness/tasks/<slug>.yaml`; nếu đã tồn tại và giống hệt →
    reuse (idempotent); khác → tạo slug mới.
  - Flag `--goal-refine`: dispatch plan subagent để đề xuất
    `requirements`/`scope` chi tiết hơn, merge vào task trước khi ghi (opt-in,
    vì tốn 1 LLM call).

- CLI: `internal/cli/harness.go` thêm flag `--goal` vào subcommand `run` (và
  `eval`?). `harness run --goal "..."` = Materialize → `engine.Run` như thường.
  In rõ task file path đã tạo để user inspect/chỉnh tay.

### Lớp 2: Skill `/goal`

**File mới**: `presets/skills/goal/SKILL.md` (sync về
`~/.config/opencode/skill/goal/SKILL.md` qua agentsync như các skill khác).

Trigger: `/goal` (dạng slash-name) và short tag `//g`.

Nội dung skill hướng dẫn agent:

1. Nhận goal text từ user. Nếu goal mơ hồ về scope (vd: "làm web") → hỏi lại 1
   câu trước khi materialize (tránh scope `**` quá rộng).
2. Chạy `go run . harness run --goal "<goal>" --project .`.
3. Kết quả:
   - **Finalized**: tóm tắt iterations, eval results, git diff --stat, báo cáo
     ngắn cho user.
   - **Paused (CI)**: đọc `.harness/decision-request.md`, hỏi user, sau khi có
     câu trả lờicập nhật và chạy `harness resume`.
   - **Paused (interactive)**: CLI tự xử lý prompt; agent chờ process kết thúc.
4. Khi user muốn bỏ: `harness reset --task <slug>`.

Khuyến nghị trong skill: goal lớn nên đi qua `//p` (plan skill) trước, sau đó
`/goal` chỉ cho phần execution — tránh lạm dụng harness cho việc thiết kế.

### Trigger registration

- `presets/agents/AGENTS.md` (nguồn sync): thêm dòng `//g` / `/goal` → skill
  `goal` vào bảng "Short Tags Cho Skill Local"; mô tả: "Nhận mục tiêu tự nhiên,
  materialize thành harness task rồi chạy loop tới khi xong/pause."
- Trigger ghép tiềm năng (ghi nhận, không bắt buộc phase này): `//rg` (search
  docs trước khi materialize), `//pg` (plan rồi goal).

### Docs

- `docs/modules/harness.md`: thêm section "Goal mode" mô tả `--goal`,
  `MaterializeGoalTask`, slug rules, acceptance auto-discover.
- `docs/features/agentic-loop.md`: 1 đoạn usage `/goal` trong Sử Dụng.
- Skill `harness` (preset + local): bổ sung `--goal` vào CLI list.

## Các bước triển khai

| Bước | Việc | File chính |
| ---- | ---- | ---------- |
| 1 | `MaterializeGoalTask`: slug, template, idempotent write | `internal/harness/goal.go` |
| 2 | Unit tests: slug edge cases, trùng ID, YAML hợp lệ parse lại được | `internal/harness/goal_test.go` |
| 3 | CLI flag `--goal` cho `harness run` (+ in task path) | `internal/cli/harness.go`, test |
| 4 | Flag `--goal-refine` (opt-in LLM refine requirements/scope) | `internal/harness/goal.go` |
| 5 | Skill preset `goal` + sync local | `presets/skills/goal/SKILL.md` |
| 6 | Trigger registration trong `presets/agents/AGENTS.md` | AGENTS.md (cả bản local nếu cần) |
| 7 | Docs: harness.md, agentic-loop.md, skill harness | docs/... |
| 8 | Integration test: `--goal` trên repo mẫu (MockDriver) chạy đủ loop | `internal/cli/harness_test.go` |

Mỗi bước 1 commit (Conventional Commits), kèm tests.

## Acceptance criteria

- `go run . harness run --goal "add health endpoint" --project <tmp-repo>` tạo
  `.harness/tasks/add-health-endpoint.yaml`, YAML parse lại bằng `LoadTask`
  được, loop chạy với MockDriver đến finalized/paused như task viết tay.
- Chạy lại cùng goal 2 lần: lần 2 reuse task file (không đổi), state resume
  đúng.
- Goal trùng ID task khác nội dung → slug mới không ghi đè.
- `--goal-refine` gọi đúng plan agent qua MockDriver, merge requirements.
- `go test ./...` pass; docs và trigger table khớp implementation.

## Rủi ro & giảm thiểu

- **Scope `**` quá rộng** → subagent sửa lan. Giảm: skill bắt hỏi lại khi goal
  mơ hồ; flag `--scope` cho user siết; `harness-runner` subagent preset đã ràng
  "chỉ sửa trong scope.include".
- **Goal không đo được bằng test** → verify fail hoài đến max failures rồi
  pause. Đây là hành vi đúng của guardrail; HITL xử lý.
- **Slug va chạm/đọc vô nghĩa** → chỉ là identifier; description giữ nguyên
  goal gốc nên vẫn rõ nghĩa.
- **Lạm dụng cho design work** → skill khuyến nghị `//p` trước cho goal lớn.
