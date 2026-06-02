#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/claim-freeze-gate.json}"
OUT="${2:-results/generated/claim-freeze}"
rm -rf "$OUT"
mkdir -p "$OUT"

jq -e '.version == "patchline.claim-freeze-gate/v1" and (.claims|length) >= 1' "$SPEC" > /dev/null

checksum() { shasum -a 256 "$1" | cut -d' ' -f1; }

# Freeze: checksum each claim's backing artifact.
freeze="[]"
while IFS= read -r c; do
  id="$(jq -r '.id' <<<"$c")"
  art="$(jq -r '.artifact' <<<"$c")"
  test -f "$art"
  h="$(checksum "$art")"
  freeze="$(jq --arg id "$id" --arg art "$art" --arg h "$h" '. + [{id:$id, artifact:$art, checksum:$h}]' <<<"$freeze")"
done < <(jq -c '.claims[]' "$SPEC")
echo "$freeze" > "$OUT/freeze.json"

# Re-verify the live artifacts against the freeze (expect no drift).
verify="[]"
while IFS= read -r e; do
  art="$(jq -r '.artifact' <<<"$e")"
  frozen="$(jq -r '.checksum' <<<"$e")"
  now="$(checksum "$art")"
  match=$([ "$frozen" = "$now" ] && echo true || echo false)
  verify="$(jq --arg art "$art" --argjson match "$match" '. + [{artifact:$art, match:$match}]' <<<"$verify")"
done < <(jq -c '.[]' "$OUT/freeze.json")

# Negative control: tamper with a copy of the first artifact and re-checksum.
first_art="$(jq -r '.[0].artifact' "$OUT/freeze.json")"
first_frozen="$(jq -r '.[0].checksum' "$OUT/freeze.json")"
cp "$first_art" "$OUT/tampered.md"
echo "<!-- post-submission edit -->" >> "$OUT/tampered.md"
tampered_now="$(checksum "$OUT/tampered.md")"
drift_detected=$([ "$first_frozen" != "$tampered_now" ] && echo true || echo false)

jq -n \
  --argjson verify "$verify" \
  --argjson drift "$drift_detected" '{
  version: "patchline.claim-freeze/v1",
  verifications: $verify,
  no_drift: ($verify | all(.[]; .match)),
  tamper_drift_detected: $drift
}' > "$OUT/claim-freeze.json"

{
  echo "# Paper claim freeze"
  echo
  echo "Live artifacts match freeze (no drift): $(jq -r .no_drift "$OUT/claim-freeze.json")"
  echo
  echo "Tampered copy flagged as drift: $(jq -r .tamper_drift_detected "$OUT/claim-freeze.json")"
} > "$OUT/claim-freeze.md"
cp "$OUT/claim-freeze.md" "$OUT/README.md"

echo "claim-freeze worker: no_drift=$(jq -r .no_drift "$OUT/claim-freeze.json") tamper_drift_detected=$(jq -r .tamper_drift_detected "$OUT/claim-freeze.json")"
