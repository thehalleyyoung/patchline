#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/disaster-recovery-exercise-gate.json}"
OUT="${2:-results/generated/disaster-recovery-exercise}"
mkdir -p "$(dirname "$OUT")"

jq -e '
  .version == "patchline.disaster-recovery-exercise-gate/v1"
  and (.claim | length) > 350
  and (.mirrors | length) == 2
  and (.components | length) == 4
  and ([.components[].id] | sort) == ["certificate-logs","docs-site","public-corpus","release-manifest"]
  and any(.components[]; .id == "public-corpus" and .primary_failure.action == "none")
  and ([.components[] | select(.primary_failure.action != "none")] | length) == 3
' "$SPEC" > /dev/null

for phrase in "disaster recovery exercise" "restored from mirrors" "make disaster-recovery-exercise-gate"; do
  grep -F "$phrase" docs/disaster-recovery-exercise.md README.md > /dev/null
done

bash scripts/disaster-recovery-exercise.sh "$SPEC" "$OUT" > "$OUT.run.log"

jq -e '
  .version == "patchline.disaster-recovery-exercise/v1"
  and .summary.components == 4
  and .summary.all_restored == true
  and .summary.primary_restores >= 1
  and .summary.secondary_restores >= 3
  and .summary.rejected_primary_mirrors >= 3
  and .certificate_replay.matches_source == true
  and .negative_control.rejected == true
  and all(.components[]; .restored == true and .checksum_verified == true)
  and any(.components[]; .id == "public-corpus" and .restored_from == "primary")
  and any(.components[]; .id == "docs-site" and .restored_from == "secondary" and (.failed_mirrors | index("primary")))
  and any(.components[]; .id == "release-manifest" and .restored_from == "secondary" and (.failed_mirrors | index("primary")))
  and any(.components[]; .id == "certificate-logs" and .restored_from == "secondary" and (.failed_mirrors | index("primary")))
' "$OUT/disaster-recovery-exercise.json" > /dev/null

while IFS=$'\t' read -r component rel; do
  test -s "$OUT/restored/$component/$rel"
done < <(jq -r '.components[] | .id as $id | .required_files[] | [$id, .] | @tsv' "$SPEC")

jq -e '
  .version == "patchline.corpus-release/v1"
  and .dataset_cards >= 25
  and .signature_verified == true
  and (.artifact_hash | startswith("sha256:"))
' "$OUT/restored/public-corpus/release.json" > /dev/null

jq -e '
  .version == "patchline.docs-site/v1"
  and (.pages | index("index.html")) != null
  and .public_demo.summary.ranked_risks > 0
' "$OUT/restored/docs-site/site/site-manifest.json" > /dev/null

jq -e '
  .version == "patchline.artifact-release-manifest/v1"
  and .summary.verified == true
  and .summary.public_repos >= 4
  and (.release.content_hash | test("^sha256:[0-9a-f]{64}$"))
' "$OUT/restored/release-manifest/artifact-release-manifest.json" > /dev/null

jq -e '
  .version == "patchline.certificate-revocation-replay/v1"
  and .all_ok == true
  and .records == 1
  and .revoked == 1
  and (.checkpoint.tip_hash | test("^[0-9a-f]{64}$"))
' "$OUT/restored/certificate-logs/replay-restored.json" > /dev/null

cmp -s "$OUT/source/docs-site/site/site-manifest.json" "$OUT/restored/docs-site/site/site-manifest.json"
cmp -s "$OUT/source/release-manifest/artifact-release-manifest.json" "$OUT/restored/release-manifest/artifact-release-manifest.json"
cmp -s "$OUT/source/certificate-logs/replay.json" "$OUT/restored/certificate-logs/replay.json"

jq -n --slurpfile report "$OUT/disaster-recovery-exercise.json" '{
  version: "patchline.disaster-recovery-exercise-gate-results/v1",
  components: $report[0].summary.components,
  primary_restores: $report[0].summary.primary_restores,
  secondary_restores: $report[0].summary.secondary_restores,
  rejected_primary_mirrors: $report[0].summary.rejected_primary_mirrors,
  certificate_replay_matches_source: $report[0].certificate_replay.matches_source,
  negative_control_rejected: $report[0].negative_control.rejected,
  verified: true
}' > "$OUT/gate-summary.json"

echo "disaster-recovery-exercise gate passed: public corpus, docs, releases, and certificate logs restored from mirrors with negative control rejected"
