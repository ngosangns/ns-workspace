# Sync State Template

Dùng cho `docs/_sync.md`:

```markdown
# Docs Sync State

## Meta

- Synced commit: `<commit-sha-or-HEAD>`
- Synced at: `YYYY-MM-DDTHH:MM:SSZ`
- Scope: [docs/code area đã kiểm tra]
- Status: synced | partially-synced | unsynced
- Known unsynced: Không có | [note ngắn]
```

## Quy Tắc

- `docs/_sync.md` chỉ giữ metadata sync hiện tại, không chứa history.
- Synced commit là mốc để lần sau chạy `git log`/`git diff`.
- Status: `synced` khi docs phản ánh target commit; `partially-synced` khi còn known-unsynced; `unsynced` khi mới bootstrap.
- Không đưa changelogs, commit-history, migration logs vào file này.
