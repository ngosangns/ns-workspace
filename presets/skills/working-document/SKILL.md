---
name: working-document
description: >-
  Đọc một commit hoặc toàn bộ commits của một branch và viết ra tài liệu
  (working document) giải thích chi tiết từng bước thay đổi: nguyên nhân, lý do,
  cách thay đổi, và vị trí cụ thể của từng method/property/logic trong codebase.
  Kèm theo diagrams/flows khi cần thiết. Tạo phiên bản chi tiết cho developer và
  phiên bản tóm tắt cho business khi thay đổi có tác động nghiệp vụ. Dùng khi
  ngườidùng muốn tài liệu hoá, review, hoặc giải thích các thay đổi từ git history
  một cách dễ hiểu.
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
tài liệu giải thích dễ hiểu, đi qua từng bước thay đổi, nêu rõ nguyên nhân,
lý do, cách thực hiện, và chỉ rõ vị trí của từng method/property/logic trong code.

Mặc định ghi **một** working document kỹ thuật tại `docs/working-documents/`. Nếu thay đổi có business impact rõ và user muốn bản tóm tắt riêng, thêm section business trong cùng file hoặc file sibling cùng thư mục — không bắt buộc cây `docs/business` / `docs/developer`.

## Khi nào dùng skill này

- Ngườidùng yêu cầu "viết tài liệu cho commit/branch này"
- Cần giải thích cho ngườikhác (hoặc cho chính mình sau này) một loạt thay đổi
- Onboarding, review, hoặc tổng kết một feature/bugfix vừa hoàn thành
- Cần một bản walkthrough có diagram/flow cho thay đổi phức tạp

## Đầu vào cần làm rõ

Trước khi bắt đầu, xác định (hỏi ngườidùng nếu chưa rõ):

1. **Phạm vi**: một commit cụ thể (`<sha>`) hay toàn bộ commits của một branch?
2. **Mốc so sánh** (nếu là branch): so với branch nền nào? (mặc định thử
   `main`/`master`, hoặc merge-base giữa branch hiện tại và branch nền)
3. **Nơi lưu tài liệu**:
   - Mặc định: `docs/working-documents/<tên-mô-tả>.md`
   - Chỉ định khác nếu user yêu cầu.
4. **Ngôn ngữ tài liệu**: viết theo ngôn ngữ ngườidùng đang dùng (mặc định tiếng Việt).

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

### Bước 3 — Xác định business impact

Đánh giá xem thay đổi có ảnh hưởng nghiệp vụ không. Có business impact nếu thay đổi:

- User workflow, UI/UX, public behavior
- Business rule, validation, acceptance criteria
- Public contract/API ảnh hưởng consumer
- Quyền, phân quyền, compliance

Nếu có → tạo thêm business version. Nếu chỉ là refactor nội bộ, công cụ, log → chỉ cần developer version.

### Bước 4 — Viết tài liệu

Tạo file markdown theo cấu trúc bên dưới. Nguyên tắc viết:

- **Dễ hiểu trước, chi tiết sau**: mở đầu bằng tóm tắt một câu cho mỗi thay đổi,
  rồi mới đi sâu.
- **Giải thích "tại sao" trước "như thế nào"**: nêu nguyên nhân/vấn đề, rồi mới
  đến cách giải quyết.
- **Triển khai theo từng bước hợp lý**: sắp xếp các thay đổi theo trình tự triển khai logic và dễ theo dõi (ví dụ: định nghĩa model/interface -> xử lý logic/service -> tích hợp API/UI/handler).
- **Gắn code liên quan trực tiếp (Code Snippets)**: với mỗi bước triển khai, trích dẫn đoạn code quan trọng được thêm/sửa/xóa (dạng code block với syntax highlighting), không dán cả file hay diff thô quá dài.
- **Giải thích code chi tiết**: đi kèm mỗi đoạn code là lời giải thích rõ ràng về mục đích, cơ chế hoạt động, và lý do xử lý logic như vậy.
- **Luôn neo vào code thật**: khi nhắc đến `methodX`, nói rõ nó thuộc `ClassY`
  trong `path/to/file`.
- **Dùng diagram/flow khi cần**: nếu thay đổi liên quan đến luồng dữ liệu, thứ tự
  gọi, vòng đời hoặc kiến trúc, hãy vẽ bằng Mermaid (`flowchart`, `sequenceDiagram`,
  `classDiagram`). Không vẽ diagram thừa cho thay đổi đơn giản.

## Cấu trúc tài liệu đầu ra

### Working document (`docs/working-documents/<name>.md`)

````markdown
---
type: working-document
title: "Working Document: <Tiêu đề / tên branch hoặc commit>"
description: "Chi tiết kỹ thuật các thay đổi từ <commit/branch>."
tags: [working-document]
timestamp: <ISO 8601>
---

# Working Document: <Tiêu đề / tên branch hoặc commit>

## Tổng quan

- **Phạm vi**: <commit sha | branch base..head>
- **Mục tiêu chung**: <1-2 câu mô tả thay đổi giải quyết vấn đề gì>
- **Các file chính bị ảnh hưởng**: <danh sách ngắn>

## Bối cảnh & Vấn đề

<Mô tả trạng thái trước khi thay đổi và lý do cần thay đổi>

## Chi tiết thay đổi & Triển khai theo từng bước

### Bước 1 — <Tiêu đề bước triển khai> (`<commit sha ngắn>` nếu có)

- **Vấn đề / Nguyên nhân**: <Tại sao cần thực hiện bước này>
- **Cách triển khai & Mục tiêu**: <Mô tả hướng giải quyết và các công việc cần triển khai>
- **Vị trí trong code**:
  - `tênMethod()` thuộc `Class/Module` trong `path/to/file.ext`
  - ...
- **Code liên quan**:
  ```<language>
  // Trích dẫn đoạn code quan trọng được thêm/sửa/xóa trực tiếp ở bước này
  ```
- **Giải thích chi tiết đoạn code**: <Giải thích mục đích, logic hoạt động của các dòng code trên và lý do thay đổi>
- (Tùy chọn) Diagram/flow nếu thay đổi phức tạp:
  ```mermaid
  flowchart TD
    A[...] --> B[...]
  ```

### Bước 2 — <Tiêu đề bước triển khai tiếp theo>

...

## Sơ đồ tổng thể (nếu cần)

<Mermaid diagram thể hiện luồng/kiến trúc sau thay đổi>

## Tác động & Lưu ý

- Ảnh hưởng đến phần nào của hệ thống
- Rủi ro, breaking changes, điểm cần kiểm thử
- Các bước tiếp theo (nếu có)
- (Tùy chọn) Section **Tóm tắt nghiệp vụ** trong cùng file khi có user impact
````

## Quy tắc bắt buộc

- LUÔN dùng `git --no-pager` (hoặc tool đọc file) để tránh treo terminal.
- LUÔN đọc code thật để xác nhận vị trí method/property/logic — không suy đoán.
- LUÔN trình bày cách triển khai theo từng bước hợp lý, giải thích rõ nguyên nhân, cách hoạt động và gắn đoạn code (code snippet) liên quan trực tiếp vào từng bước.
- LUÔN chỉ rõ "thành phần → thuộc gì → ở file nào" khi nhắc đến code.
- Diagram chỉ thêm khi nó làm rõ luồng/kiến trúc; bỏ qua nếu thay đổi đơn giản.
- Viết để ngườidọc không quen codebase vẫn hiểu được.
- KHÔNG sửa code trong skill này; chỉ đọc git + code và viết tài liệu.

