# Agent Instructions

## Skills

Agent chọn skill theo ý định của user (không dùng short-tag / trigger prefix).

### Bắt buộc: load skill mới nhất khi trigger

Mỗi lần dùng skill (theo ý định user hoặc pipeline gợi ý), **phải đọc lại file `SKILL.md` của skill đó trên disk trước khi làm theo** — không dựa vào bộ nhớ session, tóm tắt system prompt, hay lần đọc trước.

- Đường dẫn thường: `~/.agents/skills/<skill-name>/SKILL.md` (hoặc absolute path trong danh sách skill available).
- Trigger ghép / pipeline nhiều skill: đọc lại `SKILL.md` cho **từng** skill ngay trước khi chạy skill đó.
- Skill có file phụ (`references/`, `_shared/`, …): đọc các file skill chỉ định; vẫn ưu tiên nội dung disk hơn cache.
- Mục tiêu: luôn áp dụng bản skill mới nhất (sau sync, edit preset, hoặc cài registry).

### Workflow / dev (inline — không còn skill riêng)

Các mode dưới đây nằm trong `AGENTS.md`. Không tìm/load `SKILL.md` cho `research` / `plan` / `execution` / `fix` / `cleanup` / `init`.

Quy tắc chung khi sửa code: đọc `~/.agents/skills/_shared/CONVENTIONS.md` (hỏi khi vướng, diff review loop, comment, worktree, không đọc/ghi `docs/`).

#### Pipeline gợi ý

- Task lớn: **research** → **plan** → (user duyệt) → **execution**. Dừng sau plan cho đến khi user duyệt rõ ràng trước khi sửa source code.
- Task nhỏ đã rõ: research ngắn → execution.
- Bug có triệu chứng: **fix** (hoặc research nếu còn mơ hồ).
- Cleanup: **cleanup** → **plan** → (user duyệt) → **execution**.
- Goal tự chạy: skill `goal` / `harness` + `loop` (+ `eval` khi cần).

#### Research

Dùng trước khi lập plan hoặc triển khai. Đọc trước, hỏi sau, kết luận dựa trên file thật.

Mục tiêu:

- Hiểu ý định user và behavior hiện tại trong code.
- Tìm nguyên nhân gốc rễ, không dừng ở triệu chứng.
- Đủ rộng để thấy boundary/rủi ro; đủ hẹp để xác định đúng file/module.
- Ưu tiên tự nghiên cứu thay vì hỏi ngay.

Quy trình:

1. Search bằng `rg` và `rg --files`. Dùng skill `lsp-code-graph` khi cần symbol/caller/callee context.
2. Đọc code path, call site, data I/O, test liên quan để xác định behavior thật.
3. Xác định nguyên nhân gốc rễ bằng cách đối chiếu code path, call site, data I/O.
4. Tóm tắt bức tranh tổng quan + phạm vi tập trung + boundary + rủi ro + ngoài scope.
5. Chỉ hỏi user sau khi đã đọc mà vẫn còn câu hỏi cụ thể.
6. Task lớn → plan. Task nhỏ rõ → execution.

Đầu ra: tóm tắt bối cảnh; nguyên nhân gốc rễ hoặc giả thuyết đã kiểm chứng; phạm vi + boundary; câu hỏi đang chặn (nếu có).

Ràng buộc: không sửa file; không chạy build chỉ để hoàn tất nghiên cứu; không đọc/ghi `docs/`.

#### Plan

Dùng sau research khi yêu cầu lớn, phức tạp, liên quan kiến trúc hoặc rủi ro. Công việc nhỏ và rõ có thể bỏ qua.

Ngôn ngữ:

- Viết kế hoạch bằng tiếng Việt có dấu. Pha tiếng Anh chỉ cho tên riêng, thuật ngữ kỹ thuật, tên API/module/field.
- Viết như tài liệu thiết kế, không như changelog hay nhật ký Git.
- Thêm Mermaid khi cấu trúc, luồng dữ liệu, quan hệ module khó hiểu bằng chữ.

Nguyên tắc:

- Tìm nguyên nhân gốc rễ trước; phân biệt triệu chứng vs nguyên nhân.
- Nhìn tổng quát, giữ trọng tâm: context, module boundary, contract, rủi ro; chỉ đề xuất việc trong phạm vi mục tiêu.
- Trình bày plan trong hội thoại (hoặc file path user chỉ định ngoài `docs/`). Không ghi `docs/`.

Từ branch hoặc commit (khi user yêu cầu):

- Chỉ đọc: `git merge-base`, `git log`, `git diff --stat`, `git diff`, `git show`. Không switch branch.
- Không đưa vào plan: tên branch, hash commit, danh sách commit, tác giả, "files changed" table.
- Chuyển hóa thành: mục tiêu thiết kế, cấu trúc giải pháp, module boundary, logic nghiệp vụ, rủi ro, kiểm chứng.

Quy trình:

1. Đảm bảo research đã xác định code path, ràng buộc, boundary. Dùng `lsp-code-graph` khi cần.
2. Làm rõ nguyên nhân gốc rễ và động lực thiết kế.
3. Xác định bức tranh tổng quan rồi thu hẹp: module boundary, data flow, API/contract, vùng ảnh hưởng, ngoài phạm vi.
4. Nếu từ branch/commit, đọc thay đổi bằng Git chỉ đọc.
5. Soạn kế hoạch theo mẫu `~/.agents/skills/_shared/templates/plan-template.md`.
6. Nếu có impact nghiệp vụ rõ, ghi acceptance criteria / user impact (section riêng).
7. Trình bày tóm tắt kế hoạch cô đọng bằng tiếng Việt.
8. **Dừng và chờ user phê duyệt** trước khi sửa code.

Ràng buộc: không triển khai code lớn trước khi user duyệt; không switch/checkout chỉ để đọc; không đưa siêu dữ liệu Git vào kế hoạch; không đọc/ghi `docs/`.

#### Execution

Dùng khi đã đến bước sửa code. Task lớn: chỉ sau khi user duyệt plan. Task nhỏ đã rõ: ngay sau research.

**Khác fix:** execution triển khai từ thiết kế rõ, có thể chạm nhiều file, được phép phá tương thích ngược nội bộ để giữ kiến trúc sạch. Fix bắt đầu từ triệu chứng, ưu tiên diff nhỏ nhất.

Nguyên tắc riêng:

- Không giữ tương thích ngược vô điều kiện: khi code mới cần đổi contract nội bộ để đúng kiến trúc hơn, được quyền thay. Chỉ giữ khi user yêu cầu rõ hoặc public contract bắt buộc.
- Báo cáo có diễn giải: ý nghĩa từng nhóm thay đổi, behavior/contract đổi/giữ, rủi ro, user cần lưu ý gì.
- Liệt kê việc còn lại nếu chưa xử lý trọn vẹn.

Quy trình:

1. Đọc lại plan/research. Xác định nguyên nhân gốc rễ, module boundary, contract, call site.
2. Thu hẹp phạm vi: file cần sửa, pattern lân cận, test phù hợp, ngoài scope.
3. Triển khai theo style, helper, kiến trúc hiện có.
4. Diff review loop (CONVENTIONS.md).
5. Chạy validation mục tiêu khi phù hợp.
6. Tổng kết và liệt kê việc còn lại nếu có.

Ràng buộc: không để lại comment tiếng Việt trong code mới hoặc code vừa chạm; không đọc/ghi `docs/`.

#### Fix

Dùng khi nhiệm vụ là sửa lỗi đã có triệu chứng rõ: test fail, lỗi runtime, regression, bug report.

**Khác execution:** fix bắt đầu từ triệu chứng, dùng tái hiện làm bằng chứng, ưu tiên diff nhỏ nhất đúng nguyên nhân.

Nếu lỗi mơ hồ, dùng research trước.

Nguyên tắc riêng:

- Tái hiện trước bằng test/command/log. Nếu không, ghi rõ lý do và bằng chứng thay thế.
- Sửa nguyên nhân gốc: triệu chứng → nguyên nhân trực tiếp → nguyên nhân gốc rễ. Fix nhỏ nhất đúng nguyên nhân, không vá triệu chứng.
- Chặn regression: thêm/cập nhật test khi bug có bề mặt test hợp lý.
- Giải thích sau fix: lỗi gốc bị loại ra sao, regression nào chặn, behavior đổi/giữ, rủi ro còn.

Quy trình:

1. Đọc bug report/triệu chứng. Kiểm tra `git status` tránh đè việc user.
2. Xác định code path bằng `rg`, test, log. Dùng `lsp-code-graph` khi cần.
3. Dựng giả thuyết nguyên nhân gốc rễ.
4. Tái hiện lỗi bằng command nhỏ nhất.
5. Sửa nguyên nhân gốc theo pattern hiện có.
6. Thêm regression guard nếu phù hợp.
7. Diff review loop (CONVENTIONS.md).
8. Chạy lại command tái hiện + validation mục tiêu.
9. Báo nguyên nhân, cách sửa, xác minh, việc còn lại.

Ràng buộc: không mở rộng sang refactor lớn nếu không cần để fix; không đổi behavior public ngoài phần cần sửa; không đọc/ghi `docs/`.

#### Cleanup

Dùng khi cần đánh giá cleanup trước khi xóa hoặc refactor. Gom bằng chứng từ diff và code, rồi lập plan cleanup. Không tự xóa nếu user chưa duyệt.

Kết quả:

- Inventory có bằng chứng cho dead code/flows, legacy compatibility, duplicate logic.
- Kế hoạch cleanup (theo section Plan ở trên) để user duyệt.
- Danh sách phần không đủ bằng chứng để xóa + cách kiểm chứng tiếp.

Quy trình:

1. Xác định nguồn audit:
   - Worktree: `git status --short`, `git diff --stat`, `git diff`, `git diff --staged`.
   - Branch: `git merge-base`, `git log`, `git diff --stat`, `git diff` (chỉ đọc, không switch).
   - Commit/ref: `git show --stat`, `git show`, `git diff <ref>^..<ref>`.
2. Đọc `AGENTS.md` / README liên quan nếu cần; quét code bằng `rg --files` + `rg -n`. Dùng `lsp-code-graph` khi cần. Phân biệt "không tìm thấy reference" vs "đủ bằng chứng để xóa".
3. Phân loại candidate:
   - **Dead code:** symbol/helper/type/import không còn được gọi, duplicate branch, fallback không reachable.
   - **Dead flows:** CLI/UI/API path không còn entrypoint, feature flag không tác dụng.
   - **Legacy:** compatibility layer, migration note, adapter branch chỉ phục vụ kiến trúc cũ.
4. Đánh giá rủi ro: public contract, config user-level, data migration, generated artifact, test surface. Tách phase nếu cần. Bằng chứng yếu → "cần xác minh thêm".
5. Lập plan cleanup bằng tiếng Việt: bối cảnh, phạm vi, candidate, thứ tự, rủi ro, validation.
6. **Dừng và chờ user duyệt.**

Ràng buộc: không dùng `git switch` / `git checkout` / `git reset` / `git clean` để đọc; không xóa chỉ vì tên có vẻ cũ; không tạo plan kiểu changelog hoặc diff thô; không đọc/ghi `docs/`.

### Skill còn lại

| Skill | Khi Dùng |
| ----- | -------- |
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

## Hướng Dẫn Sử Dụng Harness, Loop Và Eval

### Khi nào dùng cái gì

| Pipeline | Khi nào dùng |
| -------- | ------------ |
| harness | Cần chạy kiểm chứng một task cụ thể theo acceptance criteria. |
| loop | Kích hoạt self-correct loop (plan → execute → verify → diagnose). |
| harness + loop | Task đã có task file `.harness/tasks/<id>.yaml`, cần loop tự chạy. |
| harness + eval | Đánh giá kết quả cuối theo tiêu chí khách quan. |
| harness + loop + eval | Vừa chạy loop vừa eval tổng thể sau mỗi iteration. |
| harness + loop + execution | Nếu loop dừng ở trạng thái pause, tiếp tục execution trực tiếp theo workflow inline ở trên. |

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
