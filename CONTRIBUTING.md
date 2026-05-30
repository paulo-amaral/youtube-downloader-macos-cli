# Contributing

Thanks for improving YouTube Downloader for macOS.

## Development

Requirements:

- Go 1.22+
- `yt-dlp`
- `ffmpeg`

Run checks:

```bash
make check
```

Run security checks only:

```bash
make security
```

Run the CLI:

```bash
make run
```

Preview a download command without downloading:

```bash
go run ./cmd/youtube-downloader download --dry-run --quality 720 "https://www.youtube.com/watch?v=VIDEO_ID"
```

## Pull Requests

- Keep changes focused.
- Update `README.md` for user-facing behavior.
- Add or update tests when logic becomes complex enough to warrant it.
- Do not commit downloaded videos, credentials, cookies, or local binaries.
