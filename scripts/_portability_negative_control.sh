#!/usr/bin/env bash
# Portability negative-control fixture. This file intentionally contains
# non-portable constructs so the shell-portability gate can prove its linter
# flags them. It is never executed.
set -euo pipefail
mapfile -t arr < <(echo hi)
echo "$arr" > /tmp/out.txt
sed -i 's/a/b/' somefile
