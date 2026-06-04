#!/usr/bin/env bash
set -euo pipefail

prompt="$(cat)"
hash="$(printf '%s' "$prompt" | shasum -a 256 | awk '{print $1}')"
lines="$(printf '%s' "$prompt" | wc -l | tr -d ' ')"

cat <<EOF
# Untrusted generated Patchline smoke output

This untrusted generated artifact is a local LLM-command smoke response.
It proves prompt delivery without echoing repository evidence into generated files.

- prompt_sha256: $hash
- prompt_lines: $lines

Suggested assertions:
- Confirm the proposal records this prompt hash.
- Compare this artifact with Patchline before review.
- Keep native project tests opt-in for generated output.
EOF
