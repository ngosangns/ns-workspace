---
name: minimax-cli
description: Generate text, images, video, speech, and music via the MiniMax AI platform using the mmx CLI. Use when the user wants to create media content, chat with MiniMax models, perform web search, or manage MiniMax API resources from the terminal. The preset grants the agent full authorization to call any mmx subcommand and ships an increased client timeout that fits MiniMax's longest-running modalities.
---

# MiniMax CLI (mmx)

`mmx` là CLI multimodal của MiniMax cho AI agents. Skill này cấp **full authorization** (toàn quyền gọi mọi subcommand) và **increased timeout** (đặt timeout dài hơn mặc định để cover video/music generation) để agent dùng MiniMax không cần prompt user xin phép từng lần.

Default model: **MiniMax-M3** (text), `speech-2.8-hd` (TTS), `MiniMax-Hailuo-2.3` (video), `music-2.6` (music). Default timeout 1800s (30 phút) cho cả per-call lẫn long-running session, đủ cover video generation sync và batch processing.

## Quy Tắc Full Authorization

Khi user đã cài `mmx` và auth thành công, agent được phép gọi **mọi** subcommand sau mà không cần hỏi:

- `mmx text chat`, `mmx image generate`, `mmx video generate`, `mmx video task get`, `mmx video download`
- `mmx speech synthesize`, `mmx music generate`, `mmx music cover`, `mmx vision describe`, `mmx search query`
- `mmx quota show`, `mmx config show/set`, `mmx auth status/refresh`
- `mmx config export-schema` để khám phá tool schema

Chỉ dừng lại hỏi user khi:

1. Chưa thấy `mmx auth status` trả về active credentials.
2. User yêu cầu tác vụ ngoài phạm vi (vd: gọi raw API, đổi region, logout) — confirm trước.
3. Subcommand return exit code 3 (auth) hoặc 4 (quota) — báo cáo lại user.

## Timeout Mặc Định (Increased)

Mặc định dùng timeout **dài hơn** so với API client phổ thông vì MiniMax có tác vụ async dài:

| Lệnh | Suggested timeout | Ghi chú |
| --- | --- | --- |
| `mmx text chat` | 120s | M3 thường trả lời trong vài giây; reasoning/chain-of-thought cần thêm. |
| `mmx image generate` | 180s | Batch nhiều ảnh có thể lâu. |
| `mmx video generate` (sync) | 1800s (30 phút) | `MiniMax-Hailuo-2.3` poll đến khi xong; dùng `--async` nếu muốn non-blocking. |
| `mmx video generate --async` | 60s | Trả task ID ngay; poll sau bằng `mmx video task get`. |
| `mmx video task get` | 60s | Chỉ query status. |
| `mmx video download` | 600s | File video lớn, CDN có thể chậm. |
| `mmx speech synthesize` | 180s | 10k chars, model lớn. |
| `mmx music generate` | 600s | `music-2.6-free` unlimited nhưng RPM=3. |
| `mmx music cover` | 600s | ASR + generation pipeline. |
| `mmx vision describe` | 120s | VLM trên ảnh lớn. |
| `mmx search query` | 60s | Web search thường nhanh. |
| `mmx quota show` | 30s | Read-only. |
| `mmx config show/set` | 30s | Local config. |
| `mmx auth status/refresh/login` | 60s | OAuth device flow mở browser. |

Khi shell wrapper của agent có timeout riêng, hãy set timeout **lớn hơn hoặc bằng** số trên cho mỗi lệnh. Với video sync generation, agent nên chuyển sang `--async` rồi poll để tránh block pipeline dài.

Có thể override bằng env var `MMX_TIMEOUT_SECONDS` nếu agent framework đọc được. User config có thể set thông qua `ns-workspace --config` overlay (xem [Custom Preset](#custom-preset)).

## Cài Đặt Nhanh

Khi user muốn dùng MiniMax, chạy:

```bash
npm install -g mmx-cli
mmx auth login --api-key sk-xxxxx
mmx auth status
mmx quota
```

Region auto-detect. Nếu API trả 401, set thủ công:

```bash
mmx config set --key region --value global   # hoặc cn
```

Khuyến nghị cài SKILL chính thức đè lên nếu user cần tham chiếu flags đầy đủ:

```bash
npx skills add MiniMax-AI/cli -y -g
```

## Flag Bắt Buộc Trong Agent Context

Luôn dùng các flag sau để đảm bảo non-interactive:

| Flag | Mục đích |
| --- | --- |
| `--non-interactive` | Fail nhanh khi thiếu args thay vì prompt. |
| `--quiet` | Tắt spinner/progress; stdout chỉ chứa data sạch. |
| `--output json` | Output machine-readable để agent parse dễ. |
| `--async` | Với video: trả task ID ngay, không block. |
| `--dry-run` | Preview request trước khi gọi API thật. |
| `--yes` | Bỏ qua confirmation prompt. |

## Lệnh Hay Dùng

### Text chat

```bash
mmx text chat --message "user:Viết 1 bài thơ 4 dòng về AI" \
  --non-interactive --quiet --output json
```

Default model được set qua `mmx config set --key default-text-model --value MiniMax-M3`. Khi gọi, có thể override bằng `--model MiniMax-M2.7-highspeed` (nhanh hơn cho tác vụ đơn giản) hoặc bất kỳ model nào hỗ trợ.

### Image generate

```bash
mmx image generate --prompt "Mèo mặc áo phi hành gia" \
  --non-interactive --quiet --out-dir ./gen/
```

### Video generate (async)

```bash
TASK=$(mmx video generate --prompt "Robot đi trong rừng" \
  --non-interactive --quiet --async --output json | jq -r '.taskId')
mmx video task get --task-id "$TASK" --output json
mmx video download --task-id "$TASK" --out robot.mp4
```

### Speech synthesize

```bash
mmx speech synthesize --text "Xin chào, đây là MiniMax" \
  --non-interactive --quiet --out hello.mp3
```

### Music generate

```bash
mmx music generate --prompt "Upbeat pop mùa hè" \
  --lyrics-optimizer --non-interactive --quiet --out summer.mp3
```

### Vision describe

```bash
mmx vision describe --image photo.jpg \
  --prompt "Đây là giống gì?" --non-interactive --quiet --output json
```

### Search query

```bash
mmx search query --q "MiniMax AI" --non-interactive --quiet --output json
```

## Exit Code Quan Trọng

| Code | Ý nghĩa | Hành động agent |
| --- | --- | --- |
| 0 | Success | Tiếp tục. |
| 2 | Usage error | Sửa flags/args rồi retry. |
| 3 | Auth error | Chạy `mmx auth status` rồi báo user. |
| 4 | Quota exceeded | Báo user; dừng generation. |
| 5 | Timeout | Tăng timeout hoặc chuyển sang `--async`. |
| 10 | Content filter | Đổi prompt; báo user nếu vẫn fail. |

## Cấu Hình Mặc Định Gợi Ý

Set các default model một lần để không phải truyền `--model` mỗi lần:

```bash
# Set defaults
mmx config set --key default-text-model --value MiniMax-M3
mmx config set --key default-speech-model --value speech-2.8-hd
mmx config set --key default-video-model --value MiniMax-Hailuo-2.3
mmx config set --key default-music-model --value music-2.6

# Use without --model
mmx text chat --message "Hello"
mmx speech synthesize --text "Hello" --out hello.mp3
mmx video generate --prompt "Ocean waves"
mmx music generate --prompt "Upbeat pop" --instrumental

# --model still overrides per-call
mmx text chat --model MiniMax-M2.7-highspeed --message "Hello"
```

Preset `presets/minimax/config.json` ghi sẵn các default này cùng `timeout: 1800` và `sessionTimeout: 1800` (30 phút mỗi cái). Khi `ns-workspace` apply với `--tools minimax`, agent tự có mọi key trong `~/.mmx/config.json`; chỉ cần `mmx auth login` rồi dùng.

Resolution priority: `--model` flag > config default > hardcoded fallback.

## Custom Preset

`ns-workspace` hỗ trợ user config overlay qua `--config <file>`. Để override hoặc bổ sung preset này, tạo file JSON:

```json
{
  "presets/skills/minimax-cli/SKILL.md": "/home/me/.config/ns-workspace/minimax-cli.md"
}
```

Default location: `~/.config/ns-workspace/config.json` (XDG-aware). Trên Windows là `%AppData%/ns-workspace/config.json`. Env var `NS_WORKSPACE_CONFIG` override path.

User file sẽ thay thế preset này trong `init`/`update`; truyền `--config ""` để tắt overlay.
