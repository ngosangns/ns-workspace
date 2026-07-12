# Frontmatter Schema (OKF)

Docs trong repo tuân theo **Open Knowledge Format (OKF)**: mỗi doc là một markdown file với **YAML frontmatter** ở đầu file. Frontmatter là cách khai báo metadata **bắt buộc** cho doc mới và cho doc đang được sửa metadata. `## Meta` prose chỉ còn để tương thích ngược; không tạo mới `## Meta`.

## Frontmatter Chuẩn

```yaml
---
type: module # BẮT BUỘC — loại concept
audience: business | developer # RECOMMENDED — audience chính của doc, dùng khi doc nằm trong docs/business/ hoặc docs/developer/
title: "[Tên hiển thị, nên trùng/gần trùng H1]"
description: "[Một câu mô tả phạm vi hiện tại]"
tags: ["domain", "area"] # optional, list string ngắn
timestamp: 2026-06-23T00:00:00Z # optional, ISO 8601 lần sửa cuối có nghĩa
resource: "https://..." # optional, URI canonical của asset doc mô tả
# Các key tương thích vẫn được parse khi cần:
status: active # draft | proposed | approved | implemented | active | deprecated | archived
version: current
compliance: current-state
priority: P1
---
```

## Quy Tắc

- **`type` là field bắt buộc** và phải có giá trị không rỗng. Giá trị mô tả, tự giải thích.
- **`audience` là field khuyến nghị** cho mọi doc audience-specific. Giá trị hợp lệ: `business` hoặc `developer`. Dùng để lọc, export, và kiểm tra doc nằm đúng cây thư mục.
- Giá trị `type` dùng trong repo này: `module`, `feature`, `spec`, `architecture`, `decision`, `pattern`, `reference`, `research`, `shared`, `development`, `index`, `working-document`. Type lạ vẫn hợp lệ (consumer permissive), nhưng ưu tiên tập trên cho nhất quán.
- `title`/`description` nên có để index, search snippet và preview hiển thị tốt.
- `tags` là YAML list; một string đơn cũng được normalize về list.
- `timestamp` theo ISO 8601; cập nhật khi có thay đổi có nghĩa.
- `resource` chỉ khi doc mô tả một asset có URI canonical (table, API, service…). Bỏ qua với concept trừu tượng.
- **Permissive consumer:** không bao giờ thêm key rỗng; key lạ không gây lỗi nhưng đừng tạo key vô nghĩa.

## Tương Thích Ngược `## Meta`

- Doc cũ chỉ có `## Meta` vẫn parse đúng, không cần migrate gấp.
- Khi sửa metadata của doc cũ: thêm frontmatter OKF; có thể giữ `## Meta` nhưng frontmatter thắng ở key trùng, `## Meta` chỉ điền field còn trống.
- Khi có cả hai, `status`/`type`/`audience` phải nhất quán giữa hai nơi.

## Reference Doc (OKF references)

Doc tổng hợp từ nguồn ngoài đặt trong `docs/references/` với:

```yaml
---
type: reference
title: "[Nguồn]"
timestamp: <ISO 8601>
tags: [reference]
---
```
