---
type: development
title: "Quy Ước Frontend Preview & Portal"
description: "SolidJS + TypeScript 7 conventions cho portal, preview SPA và export viewer."
tags: ["development", "preview-frontend", "portal", "solidjs"]
timestamp: 2026-07-17T00:00:00Z
status: active
compliance: current-state
---

# Quy Ước Frontend Preview & Portal

## Stack

| Surface | Source | Embed / output | Tooling |
| ------- | ------ | -------------- | ------- |
| Portal | `internal/portal/portal_ui_src/` | `internal/portal/portal_ui/` | Vite + `vite-plugin-solid` + Tailwind v4 |
| Preview SPA | `internal/preview/preview_ui_src/` | `internal/preview/preview_ui/` | Cùng toolchain |
| Export viewer | `internal/preview/export_ui_src/` | `export_ui/viz.js` + `viz.css` | Vite lib IIFE |

- **Framework**: SolidJS (JSX), `@solidjs/router` (hash history cho portal/preview).
- **Language**: TypeScript **7** (`tsc -p tsconfig.*.json --noEmit`).
- **Styling**: Tailwind CSS v4 (`@tailwindcss/vite`), design tokens trong `style.css`.

## Scripts

```bash
npm run build:portal
npm run build:preview
npm run build:export
npm run check:portal
npm run check:preview
npm run check:export
npm run lint:portal
```

## Quy Tắc

- Không thêm Vue/React runtime.
- Embed artifact phải rebuild trước khi commit thay đổi UI.
- Export viewer phải chạy offline (`file://`) khi `--inline-assets`; Cytoscape/marked là global vendor, không code-split remote.
- API client typed trong `api.ts` / `lib/api.ts`; không hardcode side-effect sync ngoài Go backend.

## Portal Page Kit

Portal resource pages compose shared components under `internal/portal/portal_ui_src/components/` và helpers `lib/`:

| Piece | Vai trò |
| ----- | ------- |
| `PageHeader` | Title + subtitle |
| `UiSegmented` | Tab / filter segmented control |
| `EnableSwitch` | Enable/disable (không kèm pill Enabled/Disabled trùng) |
| `ResourceRow` | Flat list row |
| `EmptyState` / `ListSkeleton` | Empty + loading |
| `StatusPill` / `PageFeedback` | Badges + error/success alerts |
| `SearchInput` | Filter search field |
| `lib/usePageFeedback` | Error + flash success |
| `lib/errors` | `errMessage(unknown)` |
| `lib/mcpConfig` | MCP transport/form helpers |

**Page shell pattern:**

```text
PageHeader
└─ surface panel (optional toolbar)
   ├─ UiSegmented | SearchInput | actions | StatusPill counts
   ├─ PageFeedback
   ├─ loading / empty / body (ResourceRow list | card grid | CodeEditor)
```

- Prefer `EnableSwitch` + opacity for disabled rows; keep pills for tier/source/Custom/transport.
- Prefer `lib/` for new hooks (`usePageFeedback`); do not reintroduce deprecated flash re-exports.
- Catch with `unknown` + `errMessage` / `usePageFeedback().fail`.
