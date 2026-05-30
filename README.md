# YouTube Downloader macOS CLI

[![CI](https://github.com/paulo-amaral/youtube-downloader-macos-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/paulo-amaral/youtube-downloader-macos-cli/actions/workflows/ci.yml)
[![Version](https://img.shields.io/badge/version-0.1.0-blue)](./CHANGELOG.md)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](./LICENSE)

A polished macOS command-line interface for downloading YouTube videos, playlists, and audio with [`yt-dlp`](https://github.com/yt-dlp/yt-dlp).

Designed for people who want the power of `yt-dlp` with a cleaner terminal experience: guided prompts, quality presets, clipboard detection, dependency checks, and safe command previews.

This repo includes:

- `cmd/youtube-downloader`: a Go CLI with interactive UX, subcommands, presets, clipboard support on macOS, and dry-run previews
- `scripts/security_check.sh`: a local maintenance script for security checks

## Keywords

`youtube downloader`, `yt-dlp`, `macos cli`, `youtube audio downloader`, `youtube playlist downloader`, `video downloader`, `golang cli`, `ffmpeg`, `terminal app`

## Features

- Guided interactive mode for macOS Terminal
- Download YouTube videos, playlists, or batches of URLs
- Quality presets: best, 4K, 1440p, 1080p, 720p, and audio only
- Audio extraction to MP3, M4A, OPUS, WAV, and other `yt-dlp` supported formats
- Clipboard URL detection on macOS
- `--dry-run` preview before running `yt-dlp`
- Dependency checks for `yt-dlp` and `ffmpeg`
- Finder integration with `--open`
- CI-ready Go project with security and contribution docs
- Hardened inputs: HTTPS-only, YouTube-only by default, URL control-character rejection, audio format allowlist

## Requirements

- `yt-dlp`
- `ffmpeg` for best video/audio merging and thumbnails
- Go, only if you want to rebuild the CLI

Check that `yt-dlp` is installed:

```bash
yt-dlp --version
```

## Go CLI

Build the CLI:

```bash
make build
```

The binary is written to `bin/youtube-downloader`.

Run without arguments for the guided interface:

```bash
./bin/youtube-downloader
```

Check your setup:

```bash
./bin/youtube-downloader check
```

Update `yt-dlp`:

```bash
./bin/youtube-downloader update
```

Download directly:

```bash
./bin/youtube-downloader download "https://www.youtube.com/watch?v=5ZsPtbD4P9s"
```

Download at a maximum quality:

```bash
./bin/youtube-downloader download --quality 1080 "https://www.youtube.com/watch?v=5ZsPtbD4P9s"
```

Open the folder in Finder when done:

```bash
./bin/youtube-downloader download --open "https://www.youtube.com/watch?v=5ZsPtbD4P9s"
```

Download a playlist:

```bash
./bin/youtube-downloader download --playlist "https://www.youtube.com/playlist?list=PLAYLIST_ID"
```

Download audio only:

```bash
./bin/youtube-downloader download --quality audio --audio-format mp3 "https://www.youtube.com/watch?v=5ZsPtbD4P9s"
```

Preview the `yt-dlp` command without downloading:

```bash
./bin/youtube-downloader download --dry-run --quality 720 "https://www.youtube.com/watch?v=5ZsPtbD4P9s"
```

List available formats:

```bash
./bin/youtube-downloader formats "https://www.youtube.com/watch?v=5ZsPtbD4P9s"
```

The Go CLI saves to `~/Downloads/YouTube` by default.

Print the current version:

```bash
./bin/youtube-downloader version
```

### Go CLI Options

```text
-q, --quality VALUE       best, 1080, 720, audio, or yt-dlp format
-o, --output DIR          download folder
-f, --file FILE           read URLs from file
-p, --playlist            download full playlists
    --audio-format VALUE  mp3, m4a, opus, wav, etc. Default: mp3
    --open                open folder in Finder
    --dry-run             preview yt-dlp command
```

## URL Quoting

In `zsh`, YouTube URLs must be quoted because characters like `?` and `&` are interpreted by the shell:

```bash
./bin/youtube-downloader download "https://www.youtube.com/watch?v=5ZsPtbD4P9s"
```

Without quotes, you may see:

```text
zsh: no matches found: https://www.youtube.com/watch?v=5ZsPtbD4P9s
```

## URL File Format

Use `--file` to download a list of URLs:

```bash
./bin/youtube-downloader download --file urls.txt
```

Example `urls.txt`:

```text
https://www.youtube.com/watch?v=VIDEO_ID_1
https://www.youtube.com/watch?v=VIDEO_ID_2
# Lines starting with # are ignored
```

## Project Status

Current version: `0.1.0`

This project is a local CLI wrapper around `yt-dlp`. Use it responsibly and follow the terms of the websites you access.

## Repository Layout

```text
cmd/youtube-downloader/  Go CLI entrypoint
scripts/                 Local maintenance scripts
docs/                    Architecture and security notes
.github/                 CI, Dependabot, issue templates, PR template
```

For development details, see [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md).

## Security

Do not commit cookies, browser session files, private URLs, or tokens. See [SECURITY.md](./SECURITY.md).

Security posture:

- No shell execution for `yt-dlp` commands
- HTTPS-only YouTube URL allowlist
- Secret scanning, CodeQL, OSSF Scorecard, Dependabot
- Local security checks with `make security`

For details, see [docs/SECURITY_HARDENING.md](./docs/SECURITY_HARDENING.md).

## License

MIT. See [LICENSE](./LICENSE).
