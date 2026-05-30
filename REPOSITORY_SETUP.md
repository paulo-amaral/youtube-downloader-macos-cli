# Repository Setup

Suggested GitHub repository:

- Name: `youtube-downloader-macos-cli`
- Visibility: public
- Description: `Polished macOS CLI for downloading YouTube videos, playlists, and audio with yt-dlp.`
- Homepage: empty for now

Suggested topics:

```text
youtube-downloader
yt-dlp
macos
macos-cli
golang
go-cli
video-downloader
audio-downloader
playlist-downloader
ffmpeg
terminal-app
youtube-dl
```

## Publish Commands

Run this after authenticating GitHub CLI:

```bash
gh auth login -h github.com
git init -b main
git add README.md LICENSE SECURITY.md CONTRIBUTING.md CODE_OF_CONDUCT.md CHANGELOG.md VERSION go.mod main.go download_videos.sh .gitignore .github REPOSITORY_SETUP.md
git commit -m "release v0.1.0"
gh repo create youtube-downloader-macos-cli \
  --public \
  --source . \
  --remote origin \
  --description "Polished macOS CLI for downloading YouTube videos, playlists, and audio with yt-dlp." \
  --push
gh repo edit paulo-amaral/youtube-downloader-macos-cli \
  --add-topic youtube-downloader \
  --add-topic yt-dlp \
  --add-topic macos \
  --add-topic macos-cli \
  --add-topic golang \
  --add-topic go-cli \
  --add-topic video-downloader \
  --add-topic audio-downloader \
  --add-topic playlist-downloader \
  --add-topic ffmpeg \
  --add-topic terminal-app \
  --add-topic youtube-dl
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

## Release Notes

Title:

```text
v0.1.0 - Initial macOS CLI release
```

Body:

```text
Initial release of YouTube Downloader macOS CLI.

Highlights:
- Interactive macOS terminal interface
- yt-dlp powered video, playlist, and audio downloads
- Quality presets and audio extraction
- Clipboard URL detection
- Dry-run command previews
- Dependency checks for yt-dlp and ffmpeg
- Bash wrapper for quick usage
```
