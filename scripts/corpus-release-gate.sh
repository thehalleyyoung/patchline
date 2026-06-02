#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GATES="${1:-examples/corpus-release-gates.json}"
OUT="${2:-results/generated/corpus-release}"
rm -rf "$OUT"
mkdir -p "$OUT/reports"

jq -e '
  .version == "patchline.corpus-release-gates/v1" and
  (.release.claims | length) >= 3 and
  all(.release.claims[]; contains("release artifacts"))
' "$GATES" > /dev/null

bash scripts/dataset-card-gate.sh examples/real-repo-catalog.json "$OUT/reports/dataset-cards" > "$OUT/dataset-card.log"
bash scripts/corpus-fairness-gate.sh examples/real-repo-catalog.json "$OUT/reports/fairness" > "$OUT/fairness.log"
bash scripts/stratified-benchmark-gate.sh examples/benchmarks/stratified-public-catalog.json "$OUT/reports/stratified" > "$OUT/stratified.log"

cp examples/real-repo-catalog.json "$OUT/real-repo-catalog.json"
cp examples/benchmarks/stratified-public-catalog.json "$OUT/stratified-public-catalog.json"

{
  printf '#!/usr/bin/env bash\n'
  printf 'set -euo pipefail\n'
  printf 'make dataset-card-gate corpus-fairness-gate stratified-benchmark-gate stale-ref-gate\n'
} > "$OUT/reproduce.sh"
chmod +x "$OUT/reproduce.sh"

(
  cd "$OUT"
  find . -type f \
    ! -name checksums.sha256 \
    ! -name checksums.attestation.json \
    -print | LC_ALL=C sort | while read -r file; do
      shasum -a 256 "$file"
    done
) > "$OUT/checksums.sha256"

seed="$(go run ./cmd/patchline attestation-keygen --json | jq -r '.seed_hex')"
go run ./cmd/patchline sign-artifact "$OUT/checksums.sha256" --subject "patchline-public-corpus-release" --seed-hex "$seed" --out "$OUT/checksums.attestation.json" > "$OUT/sign.log"
go run ./cmd/patchline verify-artifact "$OUT/checksums.attestation.json" --artifact "$OUT/checksums.sha256" --json > "$OUT/verify.json"

jq -n \
  --slurpfile gates "$GATES" \
  --slurpfile dataset "$OUT/reports/dataset-cards/index.json" \
  --slurpfile fairness "$OUT/reports/fairness/fairness.json" \
  --slurpfile stratified "$OUT/reports/stratified/summary.json" \
  --slurpfile verification "$OUT/verify.json" \
  --arg checksums "checksums.sha256" \
  --arg attestation "checksums.attestation.json" \
  --arg reproduce "reproduce.sh" \
  '{
    version:"patchline.corpus-release/v1",
    release_id:$gates[0].release.id,
    dataset_cards:$dataset[0].card_count,
    fairness_flags:($fairness[0].flags | length),
    ecosystem_manifests:$stratified[0].ecosystem_manifests,
    framework_manifests:$stratified[0].framework_manifests,
    checksums:$checksums,
    attestation:$attestation,
    reproduction_command:$reproduce,
    signature_verified:$verification[0].ok,
    artifact_hash:("sha256:" + $verification[0].artifact_hash)
  }' > "$OUT/release.json"

{
  printf '# Patchline public corpus release\n\n'
  jq -r '"- release: `" + .release_id + "`", "- dataset cards: `" + (.dataset_cards|tostring) + "`", "- fairness flags: `" + (.fairness_flags|tostring) + "`", "- ecosystem manifests: `" + (.ecosystem_manifests|tostring) + "`", "- framework manifests: `" + (.framework_manifests|tostring) + "`", "- checksums: `" + .checksums + "`", "- attestation: `" + .attestation + "`", "- reproduce: `" + .reproduction_command + "`"' "$OUT/release.json"
} > "$OUT/release.md"

jq -e '
  .version == "patchline.corpus-release/v1" and
  .dataset_cards >= 25 and
  .ecosystem_manifests >= 7 and
  .framework_manifests >= 12 and
  .signature_verified == true and
  (.artifact_hash | startswith("sha256:"))
' "$OUT/release.json" > /dev/null
test -s "$OUT/checksums.sha256"
test -s "$OUT/checksums.attestation.json"
test -x "$OUT/reproduce.sh"
echo "corpus release gate passed: signed release artifacts with $(jq '.dataset_cards' "$OUT/release.json") dataset cards"
