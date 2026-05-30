#!/usr/bin/env bash

set -euo pipefail

tracked_files="$(git ls-files)"

if printf '%s\n' "$tracked_files" | grep -E '(^|/)(cookies\.txt|.*\.cookies|\.env(\..*)?|.*\.netrc)$' >/dev/null; then
  printf 'Refusing to pass security check: tracked secret-like file found.\n' >&2
  exit 1
fi

if git grep -n -I -E '(gho_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|AKIA[0-9A-Z]{16}|BEGIN (RSA|OPENSSH|EC|DSA) PRIVATE KEY)' -- . ':!.github/workflows/*' >/dev/null; then
  printf 'Refusing to pass security check: secret-like content found.\n' >&2
  exit 1
fi

if command -v govulncheck >/dev/null 2>&1; then
  govulncheck ./...
else
  printf 'govulncheck not installed; skipping Go vulnerability database check.\n'
fi

printf 'Security check passed.\n'
