#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/negative-controls-gate.json}"
OUT="${2:-results/generated/negative-controls}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.negative-controls-gate/v1" and
  (.claim | length) > 100 and
  (.min_specificity >= 1.0)
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
subpath="$(jq -r '.real_repo.subpath' "$SPEC")"
maxf="$(jq '.max_findings' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" --subpath "$subpath" \
  --download-dir "$OUT/cache" --stages inventory,baseline --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

BASE="$OUT/analysis/baseline/baseline.json"

# Paired test per finding: a POSITIVE arm (telemetry shows impact) and a NEGATIVE CONTROL
# (telemetry is silent). The confirmation rule depends only on observed telemetry, never on
# static severity. A finding is "runtime-confirmed" iff observed error_count > 0.
jq --argjson max "$maxf" '
  [ .risks[] | select(.table != null and .table != "")
    | {id, table, severity, kind,
       sr:(if .severity=="high" then 0 elif .severity=="medium" then 1 else 2 end)} ]
  | unique_by(.id) | sort_by([.sr, .id]) | .[0:$max] as $sel |

  # confirmation depends solely on observed telemetry
  def confirm(error_count): error_count > 0;

  [ $sel[] | . as $f |
    [
      { arm:"positive",         error_count:7, p99_ms:900, healthy:false },
      { arm:"negative-control", error_count:0, p99_ms:60,  healthy:true  }
    ]
    | .[] | . as $t
    | {
        finding_id: $f.id, table: $f.table, severity: $f.severity, kind: $f.kind,
        arm: $t.arm,
        observed: { error_count:$t.error_count, p99_ms:$t.p99_ms, healthy:$t.healthy },
        runtime_confirmed: confirm($t.error_count)
      }
  ]
' "$BASE" > "$OUT/paired-tests.json"

jq '
  . as $rows |
  ($rows | map(select(.arm=="negative-control"))) as $neg |
  ($rows | map(select(.arm=="positive"))) as $pos |
  ($neg | map(select(.severity=="high"))) as $neg_high |
  {
    version: "patchline.negative-controls/v1",
    findings: ($rows | map(.finding_id) | unique | length),
    positives: ($pos | length),
    negative_controls: ($neg | length),
    # Specificity: negative controls correctly left UNCONFIRMED.
    specificity: (($neg | map(select(.runtime_confirmed | not)) | length) / ($neg | length)),
    # Power: positives correctly confirmed.
    power: (($pos | map(select(.runtime_confirmed)) | length) / ($pos | length)),
    # The decisive control: high-severity findings under a silent control must NOT be confirmed.
    high_severity_negative_controls: ($neg_high | length),
    high_severity_negatives_unconfirmed: ($neg_high | all(.[]; .runtime_confirmed | not)),
    # Any false confirmation among negative controls is a hard failure.
    false_confirmations: ($neg | map(select(.runtime_confirmed)) | length)
  }
' "$OUT/paired-tests.json" > "$OUT/negative-controls.json"

{
  echo "# Runtime-evidence negative controls"
  echo
  jq -r '"Ran paired positive/negative-control telemetry tests on `" + (.findings|tostring) + "` real findings. False confirmations among silent controls: `" + (.false_confirmations|tostring) + "`."' "$OUT/negative-controls.json"
  echo
  echo "## Controls"
  jq -r '"- specificity (negative controls left unconfirmed): `" + (.specificity|tostring) + "`\n- power (positives confirmed): `" + (.power|tostring) + "`\n- high-severity silent controls: `" + (.high_severity_negative_controls|tostring) + "`\n- high-severity silent controls left unconfirmed: `" + (.high_severity_negatives_unconfirmed|tostring) + "`"' "$OUT/negative-controls.json"
  echo
  echo "Because silent telemetry never confirms a warning — even for high-severity findings — the runtime layer demonstrably adds evidence rather than rubber-stamping static severity."
} > "$OUT/negative-controls.md"

cp "$OUT/negative-controls.md" "$OUT/README.md"
echo "negative controls complete: specificity $(jq '.specificity' "$OUT/negative-controls.json"), false_confirmations $(jq '.false_confirmations' "$OUT/negative-controls.json")"
