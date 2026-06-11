# Frontmatter Schema

Dùng YAML frontmatter cho docs mới hoặc docs đã có frontmatter:

```yaml
---
title: "[Tên ngắn, rõ nghĩa — nên trùng hoặc gần trùng H1]"
description: "[Một câu mô tả phạm vi hiện tại]"
type: spec | feature | module | architecture | decision | pattern | shared | development | research | learning | compliance | index | sync
status: draft | proposed | approved | implemented | active | deprecated | archived
tags: ["tag-ngan", "domain"]
owners: ["team-or-area"]
source_paths:
  - "internal/example/example.go"
related:
  - "../modules/example.md"
---
```

## Khối `## Meta`

Thêm ngay sau H1 khi doc cần quan hệ rõ cho agent:

```markdown
## Meta

- Trạng thái: active
- Phạm vi: [module/capability/workflow]
- Nguồn code: `path/to/file.go`
- Tuân thủ: [policy/spec/pattern hoặc "Không áp dụng"]
- Links: [Tên Doc](../path/doc.md)
```

Nếu có cả frontmatter và `## Meta`, `status` phải nhất quán. Không thêm field nếu không có giá trị thật.
