# Security Hardening

This project is hardened as a local-only CLI.

## Runtime Controls

- No shell execution for download commands.
- `https` URL scheme only.
- YouTube host allowlist by default.
- Control-character rejection for URLs.
- URL file size limit.
- Output directory safety checks.
- Audio format allowlist.
- No telemetry, background service, or network calls except `yt-dlp` itself.

## Repository Controls

- `CODEOWNERS` assigns all files to the maintainer.
- CI uses least-privilege workflow permissions.
- CodeQL scans Go code.
- Gitleaks scans for committed secrets.
- OSSF Scorecard publishes supply-chain security signals.
- Dependabot monitors Go modules and GitHub Actions.
- `.gitignore` blocks common downloaded media, binaries, cookies, and local secrets.

## Local Checks

Run:

```bash
make security
make check
```

`make security` checks for common tracked secret-like files and secret-like content. If `govulncheck` is installed, it also checks Go vulnerability data.

Install `govulncheck`:

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
```

## GitHub Settings To Enable

Repository settings still matter. Enable:

- Secret scanning
- Push protection
- Dependabot alerts
- Dependabot security updates
- Private vulnerability reporting
- Branch protection for `main`
- Required pull request review
- Required status checks before merge

## Threat Model

This CLI protects against accidental unsafe local inputs and repository supply-chain issues. It does not sandbox `yt-dlp`, `ffmpeg`, or media processing. Keep those tools updated.
