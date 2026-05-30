# Security Policy

## Supported Versions

Security updates are provided for the latest released version.

| Version | Supported |
| ------- | --------- |
| 0.1.x   | Yes       |

## Reporting a Vulnerability

Please do not open public issues for suspected security vulnerabilities.

Report security concerns through GitHub private vulnerability reporting when available, or contact the maintainer directly through their GitHub profile.

Include:

- A clear description of the issue
- Steps to reproduce
- Expected impact
- Relevant logs or command output with secrets removed

## Scope

This project is a local CLI wrapper around `yt-dlp`. It does not run a server, expose network ports, or collect telemetry.

Never paste secrets, cookies, private tokens, or browser session files into issue reports.

## Hardening

The CLI is intentionally locked down:

- Runs `yt-dlp` through `exec.Command`, not through a shell.
- Accepts only `https` URLs.
- Accepts only YouTube hosts by default:
  - `youtube.com`
  - `*.youtube.com`
  - `youtu.be`
  - `youtube-nocookie.com`
  - `*.youtube-nocookie.com`
- Rejects URL values with control characters.
- Deduplicates URLs before invoking `yt-dlp`.
- Limits URL list files to 1 MiB.
- Refuses dangerous output directories such as filesystem root and the home directory itself.
- Restricts audio extraction formats to a known allowlist.
- Does not collect analytics or phone home.

Repository security controls:

- Minimal GitHub Actions permissions.
- CodeQL static analysis.
- Gitleaks secret scanning.
- OSSF Scorecard.
- Dependabot for Go modules and GitHub Actions.
- Local `make security` check for common accidental secret commits.

## Recommended Repository Settings

Enable these settings in GitHub:

- Private vulnerability reporting
- Secret scanning
- Push protection
- Dependabot alerts
- Dependabot security updates
- Branch protection for `main`
- Required status checks:
  - `CI`
  - `CodeQL`
  - `Secret Scan`
  - `Scorecard`

## User Responsibility

This tool delegates downloads to `yt-dlp`. Keep `yt-dlp` updated, use the tool responsibly, and follow the terms of the websites you access.
