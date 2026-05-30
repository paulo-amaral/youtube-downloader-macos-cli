# Architecture

This repository is intentionally small and dependency-light.

## Layout

```text
cmd/youtube-downloader/  Go CLI entrypoint
scripts/                 Optional shell utilities
docs/                    Project and publishing notes
.github/                 CI, issue templates, and PR template
```

## Design Principles

- Keep the CLI local-first: no telemetry, no service dependency, no background daemon.
- Delegate download behavior to `yt-dlp` instead of reimplementing video extraction.
- Keep the interactive flow safe: preview commands with `--dry-run`, avoid writing files during previews, and ignore local downloads in Git.
- Validate inputs before invoking `yt-dlp`: HTTPS-only, YouTube host allowlist, no URL control characters, and constrained audio formats.
- Prefer the standard library until a dependency removes clear complexity.

## Release Flow

1. Update `VERSION` and the `version` constant in `cmd/youtube-downloader/main.go`.
2. Update `CHANGELOG.md`.
3. Run `make check`.
4. Commit, tag with `vX.Y.Z`, push, and create a GitHub release.

GoReleaser config is included for future binary releases, but release assets are optional for now.
