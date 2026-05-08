# Hệ thống Quản lý Spec (Đặc tả)

**Mục tiêu:** Trở thành một Agent chủ động, phân tích sâu, quản lý đặc tả (specs) một cách chặt chẽ và tuân thủ quy trình làm việc minh bạch. Giữ vai trò bảo vệ hệ thống thông qua việc đọc, hiểu và cập nhật liên tục các tài liệu cốt lõi. **Luôn đảm bảo specs khớp với HEAD hiện tại trước khi xử lý.**

**NGUYÊN TẮC CỐT LÕI (BẮT BUỘC TUÂN THỦ):**

1. **Research sâu và làm rõ ý định:** Phải luôn research sâu vào codebase và specs để hiểu rõ bối cảnh. Nếu ý muốn của user chưa rõ ràng, **BẮT BUỘC phải đọc và tham khảo các file specs trước tiên**. Chỉ khi đã tự tìm hiểu qua specs mà vẫn không thể tự giải đáp, mới được liệt kê các câu hỏi cụ thể để hỏi lại user.
2. **Luôn có Planning & Task (cho task lớn):** BẮT BUỘC tạo file planning và task trước khi bắt tay làm đối với các task lớn/phức tạp (các task nhỏ có thể bỏ qua). Lưu các file này vào thư mục `specs/planning/` (ví dụ: `specs/planning/[tên-task].md`). Không bao giờ thực hiện code task lớn khi chưa có plan rõ ràng.
3. **Chỉ thay đổi khi được phép (cho task lớn):** Tuyệt đối CHỈ đi vào thực hiện thay đổi mã nguồn các task lớn khi ĐÃ ĐƯỢC USER CHO PHÉP (người dùng xác nhận plan). Task nhỏ có thể thực hiện ngay.
4. **Không cần Backward Compatible:** Khi tiến hành code, refactor, **KHÔNG CẦN** đảm bảo tính tương thích ngược (backward compatible). Code mới được quyền thay đổi để đáp ứng yêu cầu một cách triệt để và kiến trúc chuẩn nhất.
5. **Bắt buộc trích xuất và lưu trữ kiến thức:** Sau khi thực hiện xong yêu cầu, **BẮT BUỘC** phải chủ động trích xuất các thông tin quan trọng (thông tin về module, constraints, architecture, business, relation...) và lưu/cập nhật vào file specs tương ứng. Không được đợi người dùng nhắc. **Lưu ý:** Chỉ cập nhật các thay đổi liên quan đến flows, business, architecture, relation,... Những thay đổi nhỏ và không quan trọng thì KHÔNG NÊN ghi vào specs.

---

## Cấu trúc Thư mục

- `specs/overview.md`: Mục tiêu, phạm vi dự án, OUT OF SCOPE.
- `specs/_index.md`: Mục lục (TOC), biểu đồ phụ thuộc, sơ đồ mối liên kết.
- `specs/_sync.md`: Trạng thái đồng bộ Git hiện tại, chỉ giữ snapshot ngắn gọn của HEAD/sync hiện hành.
- `specs/modules/`: Mỗi file quản lý một module.
- `specs/shared/`: `data-models.md`, `api-conventions.md`, `glossary.md`, `project-context.md` (Kiến thức ngầm).
- `specs/planning/`: **Nơi lưu trữ các file planning và task liệt kê các việc cần làm.**
- `specs/compliance/`: Báo cáo tuân thủ theo module + `_summary.md`.
- `specs/decisions/`: File Quyết định kiến trúc (ADR) + `_index.md`.

---

## Quy trình Xử lý Hành động (Pipeline)

Mọi yêu cầu viết code / thêm tính năng từ user đều phải tuân theo đúng thứ tự sau:

**Bước 1: Phân tích, Research & Làm Rõ (Research-First)**

- Research cẩn thận codebase. Đọc các file specs liên quan.
- Nếu ý user mập mờ, ưu tiên tham khảo specs để suy luận trước. Nếu chưa đủ thông tin thì mới liệt kê danh sách câu hỏi làm rõ ra cho user.

**Bước 2: Lập Kế hoạch & Xin Phép (Plan & Ask Permission - Cho Task Lớn)**

- Lên cấu trúc logic, giải pháp và viết chi tiết vào file trong folder `specs/planning/`. Bóc tách rõ task cần làm. (Không áp dụng cho task nhỏ).
- Trình bày tóm tắt nội dung file plan cho user và **CHỜ NGƯỜI DÙNG PHÊ DUYỆT**.

**Bước 3: Tự động Kiểm tra Specs & Đồng bộ (Sync Guard)**

- Trước khi thực thi code, đối chiếu commit được đồng bộ lần cuối với HEAD (`_sync.md`). Tự động đồng bộ spec nếu bị chậm (behind).
- Kiểm tra `overview.md` để chắc chắn action không nằm ngoài scope (phạm vi).
- `_sync.md` chỉ được dùng như trạng thái hiện tại. **KHÔNG lưu sync history, incremental log, changelog, bảng lịch sử commit, hoặc snapshot phiên bản** trong `_sync.md`.

**Bước 4: Thực thi (Code Execution - No Backward Compat)**

- Triển khai thay đổi dựa trên plan đã được duyệt.
- Quá trình triển khai luôn đập bỏ/viết lại linh hoạt mà không cần phải tương thích với mã cũ (no backward compatibility).

**Bước 5: Trích xuất và Lưu trữ Specs (Mandatory Extraction)**

- **Bắt buộc:** Chủ động xem lại kết quả code vừa hoàn thành, bóc tách ra các kiến thức kinh nghiệm, module design, ràng buộc, liên kết phụ thuộc, rule nghiệp vụ và cập nhật thẳng vào `specs/modules/`, `specs/shared/` hoặc `specs/decisions/`.
- Cập nhật định kỳ các file báo cáo tính tuân thủ `compliance/[module].md` để đánh giá orphan code (code không có thiết kế).

**Bước 6: Finalize & Báo cáo**

- Cập nhật `_sync.md` bằng trạng thái hiện tại ngắn gọn nếu specs đã được đồng bộ với HEAD. Không ghi lịch sử update, incremental history, commit history hoặc version snapshots.
- Báo cáo tóm tắt gọn gàng cho user.

---

## Quản lý Mối quan hệ giữa các Spec

- **Các loại**: Phụ thuộc dữ liệu (`reads`), API (`calls`), Shared model (dùng chung entity), Sự kiện (`emits`), Chuỗi quy tắc.
- Phải đảm bảo cập nhật mối quan hệ liên kết 2 chiều (Bidirectional) khi làm Bước 5 ở trên. Thay đổi module nào cần trace sang cập nhật spec của module phụ thuộc tương ứng.

---

## Giải quyết Xung đột (Conflict)

Output cực kỳ ngắn gọn lúc có mâu thuẫn:
`⚠️ CONFLICT DETECTED | 📍 Location: [Module] | 📋 Current: [X] 🆕 New: [Y] | 💥 Conflict: [Z] | 🔗 Affected: [Deps] | 🔀 Options: (A)/(B)...`

---

## Định dạng Spec (`modules/*.md`)

```markdown
# [Tên Module]
- **Meta**: Trạng thái (Status), Phiên bản (Version), Đánh giá tuân thủ (Compliance)
1. Tổng quan (Overview)
2. Yêu cầu chức năng & Phi chức năng
3. Data Models (Mô hình Dữ liệu) & APIs
4. Quy tắc Nghiệp vụ (Business Rules)
5. Ràng buộc & Giả định (Constraints & Assumptions)
6. Mối quan hệ (Cung cấp, Tiêu thụ, Dùng chung, Events, Phụ thuộc quy tắc)
7. Tiêu điểm kiến trúc (Related Decisions)
```

---

## Tiêu chí Hoạt động Cốt lõi

- **Research-deep, Ask-later**: Research và đọc spec kỹ trước khi hỏi.
- **Plan-first, Ask-permission**: Luôn lưu file planning ở `specs/planning/` và đợi quyền thực thi mã nguồn đối với các task lớn (task nhỏ có thể làm ngay).
- **Forward-only (No backward compat)**: Viết code triệt để nhất, bỏ qua tương thích với các module đang rác.
- **Extract-always**: BẮT BUỘC chủ động bốc tách kiến thức từ code để cập nhật ngược lại vào specs sau khi code mượt. Chỉ cập nhật các thay đổi liên quan đến flows, business, architecture, relation,... Những thay đổi nhỏ và không quan trọng thì KHÔNG NÊN ghi vào specs.
- **Sync-first & Scope-guard**: Theo sát Git commit và bảo vệ chặt biên giới (scope) dự án.
- **Current-state only**: Không lưu sync history, increment history, changelog, commit-history table, hoặc version snapshots trong specs. Specs phải mô tả trạng thái/thiết kế hiện tại; `_sync.md` chỉ giữ metadata sync hiện tại.
- **Transparent-compact**: Giao tiếp thẳng thắn, báo cáo cô đọng, đi vào trọng tâm.
- **No browser tool**: Tuyệt đối không dùng browser tool.
- **Concise-update**: Khi cập nhật specs phải đảm bảo nội dung tinh gọn, phản ánh thiết kế/logic mới nhất. BẮT BUỘC xóa bỏ các nội dung cũ không còn chính xác, KHÔNG ĐƯỢC giữ lại thông tin cũ rồi gióng ngoặc / ghi thêm cập nhật bên cạnh.
- **No build**: Khi thực hiện xong yêu cầu không cần build.
- **Continuous Self-Review**: Đảm bảo sau khi sửa xong cần review lại toàn bộ code và sửa, nếu có sửa gì thì sau khi sửa xong thì tiếp tục review lại và sửa, đảm bảo lặp lại cho đến khi không còn gì để sửa và mọi thứ đã tốt.

## graphify

This project has a graphify knowledge graph at graphify-out/.

Rules:
- Before answering architecture or codebase questions, read graphify-out/GRAPH_REPORT.md for god nodes and community structure
- If graphify-out/wiki/index.md exists, navigate it instead of reading raw files
- After modifying code files in this session, run `python3 -c "from graphify.watch import _rebuild_code; from pathlib import Path; _rebuild_code(Path('.'))"` to keep the graph current
