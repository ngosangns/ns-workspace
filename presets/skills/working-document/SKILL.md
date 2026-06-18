---
name: working-document
description: >-
  Đọc một commit hoặc toàn bộ commits của một branch và viết ra một tài liệu
  (working document) giải thích chi tiết từng bước thay đổi: nguyên nhân, lý do,
  cách thay đổi, và vị trí cụ thể của từng method/property/logic trong codebase.
  Kèm theo diagrams/flows khi cần thiết. Dùng khi người dùng muốn tài liệu hoá,
  review, hoặc giải thích các thay đổi từ git history một cách dễ hiểu.
keywords:
  - working document
  - tài liệu thay đổi
  - giải thích commit
  - document commit
  - document branch
  - changelog chi tiết
  - code walkthrough
  - giải thích thay đổi code
---

# Working Document Skill

Skill này biến lịch sử git (một commit hoặc toàn bộ commits của một branch) thành
một **tài liệu giải thích dễ hiểu**, đi qua từng bước thay đổi, nêu rõ nguyên nhân,
lý do, cách thực hiện, và chỉ rõ vị trí của từng method/property/logic trong code.

## Khi nào dùng skill này

- Người dùng yêu cầu "viết tài liệu cho commit/branch này"
- Cần giải thích cho người khác (hoặc cho chính mình sau này) một loạt thay đổi
- Onboarding, review, hoặc tổng kết một feature/bugfix vừa hoàn thành
- Cần một bản walkthrough có diagram/flow cho thay đổi phức tạp

## Đầu vào cần làm rõ

Trước khi bắt đầu, xác định (hỏi người dùng nếu chưa rõ):

1. **Phạm vi**: một commit cụ thể (`<sha>`) hay toàn bộ commits của một branch?
2. **Mốc so sánh** (nếu là branch): so với branch nền nào? (mặc định thử
   `main`/`master`, hoặc merge-base giữa branch hiện tại và branch nền)
3. **Nơi lưu tài liệu**: mặc định ghi vào `docs/working-documents/<tên-mô-tả>.md`
   trừ khi người dùng chỉ định khác.
4. **Ngôn ngữ tài liệu**: viết theo ngôn ngữ người dùng đang dùng (mặc định tiếng Việt).

## Quy trình thực hiện

### Bước 1 — Thu thập dữ liệu git

Chạy các lệnh git ở chế độ non-interactive (KHÔNG dùng pager). Gợi ý:

- Lấy danh sách commits của branch so với nền:
  ```bash
  git log --no-pager --oneline <base>..<branch>
  ```
- Lấy chi tiết một commit (message + diff):
  ```bash
  git show --no-pager --stat <sha>
  git show --no-pager <sha>
  ```
- Lấy toàn bộ diff của branch:
  ```bash
  git diff --no-pager <base>...<branch>
  ```
- Xem các file bị ảnh hưởng:
  ```bash
  git diff --no-pager --stat <base>...<branch>
  ```

Với branch nhiều commit, xử lý **theo từng commit** (commit-by-commit) để giữ
được thứ tự logic và lý do của từng bước, thay vì gộp một diff khổng lồ.

### Bước 2 — Hiểu ngữ cảnh code

Với mỗi thay đổi quan trọng, **đọc file thực tế** (không chỉ đọc diff) để hiểu:

- Method/function/property đó **nằm ở file nào, dòng nào**
- Nó **thuộc class/module/package nào**
- Nó được gọi từ đâu và gọi đến đâu (caller/callee) khi cần

Luôn ghi rõ vị trí theo định dạng: `tên thành phần` → thuộc `Class/Module` →
trong `đường/dẫn/file.ext`.

### Bước 3 — Viết tài liệu

Tạo file markdown theo cấu trúc bên dưới. Nguyên tắc viết:

- **Dễ hiểu trước, chi tiết sau**: mở đầu bằng tóm tắt một câu cho mỗi thay đổi,
  rồi mới đi sâu.
- **Giải thích "tại sao" trước "như thế nào"**: nêu nguyên nhân/vấn đề, rồi mới
  đến cách giải quyết.
- **Luôn neo vào code thật**: khi nhắc đến `methodX`, nói rõ nó thuộc `ClassY`
  trong `path/to/file`.
- **Dùng diagram/flow khi cần**: nếu thay đổi liên quan đến luồng dữ liệu, thứ tự
  gọi, vòng đời, hoặc kiến trúc, hãy vẽ bằng Mermaid (`flowchart`, `sequenceDiagram`,
  `classDiagram`). Không vẽ diagram thừa cho thay đổi đơn giản.
- Tránh dán nguyên diff dài; trích đoạn ngắn (vài dòng) kèm giải thích là đủ.

## Cấu trúc tài liệu đầu ra

```markdown
# Working Document: <Tiêu đề / tên branch hoặc commit>

## Tổng quan
- **Phạm vi**: <commit sha | branch base..head>
- **Mục tiêu chung**: <1-2 câu mô tả thay đổi giải quyết vấn đề gì>
- **Các file chính bị ảnh hưởng**: <danh sách ngắn>

## Bối cảnh & Vấn đề
<Mô tả trạng thái trước khi thay đổi và lý do cần thay đổi>

## Chi tiết thay đổi theo từng bước

### Bước 1 — <Tiêu đề thay đổi> (`<commit sha ngắn>` nếu có)
- **Vấn đề / Nguyên nhân**: <tại sao cần làm>
- **Cách thay đổi**: <làm gì, theo hướng nào>
- **Vị trí trong code**:
  - `tênMethod()` thuộc `Class/Module` trong `path/to/file.ext`
  - ...
- **Giải thích logic**: <giải thích dễ hiểu cách hoạt động sau thay đổi>
- (Tùy chọn) Diagram/flow nếu thay đổi phức tạp:
  ```mermaid
  flowchart TD
    A[...] --> B[...]
  ```

### Bước 2 — ...
...

## Sơ đồ tổng thể (nếu cần)
<Mermaid diagram thể hiện luồng/kiến trúc sau thay đổi>

## Tác động & Lưu ý
- Ảnh hưởng đến phần nào của hệ thống
- Rủi ro, breaking changes, điểm cần kiểm thử
- Các bước tiếp theo (nếu có)
```

## Quy tắc bắt buộc

- LUÔN dùng `git --no-pager` (hoặc tool đọc file) để tránh treo terminal.
- LUÔN đọc code thật để xác nhận vị trí method/property/logic — không suy đoán.
- LUÔN chỉ rõ "thành phần → thuộc gì → ở file nào" khi nhắc đến code.
- Diagram chỉ thêm khi nó làm rõ luồng/kiến trúc; bỏ qua nếu thay đổi đơn giản.
- Viết để người đọc không quen codebase vẫn hiểu được.
- KHÔNG sửa code trong skill này; chỉ đọc git + code và viết tài liệu.
