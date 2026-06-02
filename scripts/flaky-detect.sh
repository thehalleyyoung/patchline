#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/flaky-detect-gate.json}"
OUT="${2:-results/generated/flaky-detect}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.flaky-detect-gate/v1" and (.runs|type=="number")' "$SPEC" > /dev/null
runs="$(jq -r '.runs' "$SPEC")"

hashof() { shasum -a 256 | cut -d' ' -f1; }

# Deterministic candidate: identical canonical output every run.
det_hashes=()
for _ in $(seq 1 "$runs"); do
  h="$(printf '{"sorted":[1,2,3],"status":"ok"}' | hashof)"
  det_hashes+=("$h")
done

# Nondeterministic candidate: output depends on a fresh random nonce each run.
flaky_hashes=()
for i in $(seq 1 "$runs"); do
  nonce="$RANDOM-$i-$(date +%N 2>/dev/null || echo $i)"
  h="$(printf '{"nonce":"%s","status":"ok"}' "$nonce" | hashof)"
  flaky_hashes+=("$h")
done

det_uniq="$(printf '%s\n' "${det_hashes[@]}" | sort -u | wc -l | tr -d ' ')"
flaky_uniq="$(printf '%s\n' "${flaky_hashes[@]}" | sort -u | wc -l | tr -d ' ')"

jq -n \
  --argjson runs "$runs" \
  --argjson det_uniq "$det_uniq" \
  --argjson flaky_uniq "$flaky_uniq" '{
  version: "patchline.flaky-detect/v1",
  runs: $runs,
  deterministic: { distinct_hashes: $det_uniq, flaky: ($det_uniq > 1) },
  nondeterministic: { distinct_hashes: $flaky_uniq, flaky: ($flaky_uniq > 1) }
}' > "$OUT/flaky-detect.json"

{
  echo "# Flaky-gate detection"
  echo
  echo "Runs per candidate: $runs"
  echo
  echo "- Deterministic candidate distinct output hashes: $det_uniq (flaky=$( [ "$det_uniq" -gt 1 ] && echo true || echo false ))"
  echo "- Nondeterministic candidate distinct output hashes: $flaky_uniq (flaky=$( [ "$flaky_uniq" -gt 1 ] && echo true || echo false ))"
} > "$OUT/flaky-detect.md"
cp "$OUT/flaky-detect.md" "$OUT/README.md"

echo "flaky-detect worker: det_uniq=$det_uniq flaky_uniq=$flaky_uniq over $runs runs"
