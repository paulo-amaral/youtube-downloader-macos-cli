#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'USAGE'
Download YouTube videos with yt-dlp.

Usage:
  ./scripts/download_videos.sh [options] URL [URL...]
  ./scripts/download_videos.sh [options] -f urls.txt

Options:
  -o, --output DIR      Directory for downloads. Default: ./downloads
  -q, --quality VALUE   Video quality. Default: best
                       Examples: best, 1080, 720, audio
  -f, --file FILE       Read URLs from a text file, one URL per line.
  -p, --playlist        Download full playlists. Default downloads only one video.
  -l, --list            List available formats instead of downloading.
  -u, --update          Update yt-dlp before downloading.
      --open            Open the output folder in Finder after downloading.
  -h, --help            Show this help message.

Notes:
  - Requires yt-dlp: https://github.com/yt-dlp/yt-dlp
  - For merged video/audio downloads, yt-dlp may require ffmpeg.
  - Security: accepts only https YouTube URLs by default.
USAGE
}

die() {
  printf 'Error: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is not installed or not on PATH."
}

file_size_bytes() {
  if stat -f%z "$1" >/dev/null 2>&1; then
    stat -f%z "$1"
  else
    stat -c%s "$1"
  fi
}

validate_url() {
  local url="$1"
  [[ "$url" != *$'\n'* && "$url" != *$'\r'* && "$url" != *$'\t'* ]] || die "URL contains unsafe control characters: $url"
  [[ "$url" == https://* ]] || die "Only https URLs are allowed: $url"

  local rest="${url#https://}"
  local host="${rest%%/*}"
  host="${host%%:*}"
  host="$(printf '%s' "$host" | tr '[:upper:]' '[:lower:]')"

  case "$host" in
    youtube.com|*.youtube.com|youtu.be|youtube-nocookie.com|*.youtube-nocookie.com)
      ;;
    *)
      die "Only YouTube URLs are allowed by default: $url"
      ;;
  esac
}

output_dir="downloads"
quality="best"
url_file=""
download_playlist=false
list_formats=false
update_ytdlp=false
open_after=false
urls=()

while (($#)); do
  case "$1" in
    -o|--output)
      [[ $# -ge 2 ]] || die "$1 requires a directory."
      output_dir="$2"
      shift 2
      ;;
    -q|--quality)
      [[ $# -ge 2 ]] || die "$1 requires a quality value."
      quality="$2"
      shift 2
      ;;
    -f|--file)
      [[ $# -ge 2 ]] || die "$1 requires a file path."
      url_file="$2"
      shift 2
      ;;
    -p|--playlist)
      download_playlist=true
      shift
      ;;
    -l|--list)
      list_formats=true
      shift
      ;;
    -u|--update)
      update_ytdlp=true
      shift
      ;;
    --open)
      open_after=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      urls+=("$@")
      break
      ;;
    -*)
      die "Unknown option: $1"
      ;;
    *)
      urls+=("$1")
      shift
      ;;
  esac
done

require_command yt-dlp

if [[ -n "$url_file" ]]; then
  [[ -f "$url_file" ]] || die "URL file not found: $url_file"
  [[ "$(file_size_bytes "$url_file")" -le 1048576 ]] || die "URL file is too large: $url_file"
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
    validate_url "$line"
    urls+=("$line")
  done < "$url_file"
fi

((${#urls[@]} > 0)) || die "No YouTube URLs provided. Run with --help for examples."

for url in "${urls[@]}"; do
  validate_url "$url"
done

if $update_ytdlp; then
  yt-dlp -U
fi

mkdir -p "$output_dir"

format="bestvideo+bestaudio/best"
case "$quality" in
  best)
    format="bestvideo+bestaudio/best"
    ;;
  audio)
    format="bestaudio/best"
    ;;
  2160|1440|1080|720|480|360)
    format="bestvideo[height<=${quality}]+bestaudio/best[height<=${quality}]/best"
    ;;
  *)
    format="$quality"
    ;;
esac

playlist_flag=(--no-playlist)
if $download_playlist; then
  playlist_flag=(--yes-playlist)
fi

base_args=(
  --ignore-errors
  --continue
  --no-overwrites
  --restrict-filenames
  --windows-filenames
  --embed-metadata
  --embed-thumbnail
  --merge-output-format mp4
  -o "${output_dir}/%(title).200s [%(id)s].%(ext)s"
  "${playlist_flag[@]}"
)

if $list_formats; then
  yt-dlp --list-formats "${playlist_flag[@]}" "${urls[@]}"
else
  yt-dlp "${base_args[@]}" -f "$format" "${urls[@]}"
  if $open_after && [[ "$(uname -s)" == "Darwin" ]]; then
    open "$output_dir"
  fi
fi
