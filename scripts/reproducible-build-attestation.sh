#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/reproducible-build-attestation-gate.json}"; OUT="${2:-results/generated/reproducible-build-attestation}"
rm -rf "$OUT"; mkdir -p "$OUT"
jq -e '.version == "patchline.reproducible-build-attestation-gate/v1"' "$SPEC" > /dev/null
jq '

  .build_a as $a | .build_b as $b | .nondeterministic_b as $n
  | {version:"patchline.reproducible-build-attestation/v1",
     pinned:(($a.source_pin==$b.source_pin) and ($a.toolchain_pin==$b.toolchain_pin)),
     reproducible:($a.digest==$b.digest),
     digest:$a.digest,
     nondeterministic_reproducible:($a.digest==$n.digest)}

' "$SPEC" > "$OUT/out.json"
{ echo "# Reproducible build attestation"; echo; echo "Reproducible $(jq -r .reproducible "$OUT/out.json"); digest $(jq -r .digest "$OUT/out.json")"; } > "$OUT/out.md"
cp "$OUT/out.md" "$OUT/README.md"
echo "reproducible-build-attestation worker: reproducible=$(jq -r .reproducible "$OUT/out.json")"
