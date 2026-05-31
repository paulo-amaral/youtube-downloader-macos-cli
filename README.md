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
- Portuguese subtitle download and embedding for video files
- Controlled parallel downloads for batches of video URLs
- Video dubbing through the ElevenLabs Dubbing API
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

Create local configuration from the example file:

```bash
cp .env.example .env
```

The CLI loads `.env` automatically when it runs. Keep real API keys in `.env` only.

If `yt-dlp` warns that no JavaScript runtime is available for YouTube extraction,
install `node` or `deno`. The CLI auto-detects either one and passes it to `yt-dlp`.
You can also force one in `.env`:

```bash
YTDLP_JS_RUNTIME="node:/path/to/node"
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

Download up to 3 videos at the same time:

```bash
./bin/youtube-downloader download --concurrent 3 --file urls.txt
```

Download and then choose a dubbing engine:

```bash
./bin/youtube-downloader download --dub-after "https://www.youtube.com/watch?v=5ZsPtbD4P9s"
```

Prefer a specific dubbing engine after download:

```bash
./bin/youtube-downloader download --dub-after --dub-engine gemini "https://www.youtube.com/watch?v=5ZsPtbD4P9s"
```

Download video with Portuguese subtitles embedded:

```bash
./bin/youtube-downloader download --subtitles "https://www.youtube.com/watch?v=5ZsPtbD4P9s"
```

Include auto-generated Portuguese subtitles when manual subtitles are unavailable:

```bash
./bin/youtube-downloader download --subtitles --auto-subtitles "https://www.youtube.com/watch?v=5ZsPtbD4P9s"
```

Dub a video to Portuguese with ElevenLabs:

```bash
# Add this to .env:
# ELEVENLABS_API_KEY="your_api_key"

./bin/youtube-downloader dub --to pt "https://www.youtube.com/watch?v=5ZsPtbD4P9s"
```

Dub a video to Brazilian Portuguese with Gemini:

```bash
# Add this to .env:
# GEMINI_API_KEY="your_api_key"

./bin/youtube-downloader dub-gemini --to pt-BR "https://www.youtube.com/watch?v=5ZsPtbD4P9s"
```

Gemini dubbing creates a translated narration track and muxes it into a new MP4. It is useful for practical translated audio, but it is not lip-synced studio dubbing.

Dub a video using local AI tools:

```bash
# Required local tools:
# - whisper or whisper-cli
# - ollama
# - piper
# - ffmpeg
#
# Add this to .env and point PIPER_VOICE to a local .onnx voice:
# OLLAMA_MODEL="llama3.1:8b"
# PIPER_VOICE="/path/to/piper/voice.onnx"

./bin/youtube-downloader dub-local --to pt-BR "https://www.youtube.com/watch?v=5ZsPtbD4P9s"
```

The local pipeline uses Whisper for transcription, Ollama for translation, Piper for text-to-speech, and ffmpeg to create the dubbed MP4.

Dub videos that are already in `~/Downloads/YouTube` with Gemini:

```bash
./bin/youtube-downloader dub-downloaded --to pt-BR
```

Dub videos that are already in `~/Downloads/YouTube` with local AI:

```bash
./bin/youtube-downloader dub-downloaded-local --to pt-BR
```

Use a different downloaded-video folder:

```bash
./bin/youtube-downloader dub-downloaded --input ./downloads --output ./downloads --to pt-BR
./bin/youtube-downloader dub-downloaded-local --input ./downloads --output ./downloads --to pt-BR
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
    --subtitles           download and embed Portuguese subtitles
    --auto-subtitles      include auto-generated subtitles
    --subtitle-langs VAL  yt-dlp subtitle languages. Default: pt.*,pt-BR,pt
-i, --input DIR           folder with downloaded videos. Default: ~/Downloads/YouTube
-j, --concurrent NUM      videos to download at the same time. Default: 1
    --dub-after           offer dubbing after download
    --dub-engine VALUE    auto, gemini, or local. Default: auto
-t, --to VALUE            target language for dub. Default: pt
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
