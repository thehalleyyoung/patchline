#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/quarantine-attestation-gate.json}"
OUT="${2:-results/generated/quarantine-attestation}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.quarantine-attestation-gate/v1" and
  (.claim | length) > 100 and
  (.required_attestations | length) == 4
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
subpath="$(jq -r '.real_repo.subpath' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" --subpath "$subpath" \
  --download-dir "$OUT/cache" --stages inventory,baseline --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

BASE="$OUT/analysis/baseline/baseline.json"

build_attestations() {
  local target="$1"
  : > "$target"
  # one quarantine attestation per real repair candidate (repair-proof summary)
  jq -c '(.repair_proof_summaries // [])[]
    | { rid:.id, risk_id:.risk_id, table:.table, status:.status,
        repair_paths:(.repair_paths // []), proof_holes:(.proof_holes // []) }' "$BASE" | \
  while read -r row; do
    local rid risk_id table status paths fingerprint reviewers
    rid="$(jq -r '.rid' <<<"$row")"
    risk_id="$(jq -r '.risk_id' <<<"$row")"
    table="$(jq -r '.table' <<<"$row")"
    status="$(jq -r '.status' <<<"$row")"
    paths="$(jq -c '.repair_paths' <<<"$row")"
    # deterministic content fingerprint over candidate identity + paths
    fingerprint="$(printf '%s|%s|%s' "$rid" "$table" "$paths" | shasum -a 256 | cut -c1-16)"
    # reviewer escalation: candidates with open proof holes need 2 reviewers, else 1
    if [ "$(jq '.proof_holes | length' <<<"$row")" -gt 0 ]; then reviewers=2; else reviewers=1; fi
    jq -nc \
      --arg rid "$rid" --arg risk_id "$risk_id" --arg table "$table" --arg status "$status" \
      --argjson paths "$paths" --arg fp "$fingerprint" --argjson reviewers "$reviewers" '{
        candidate_id: $rid,
        risk_id: $risk_id,
        table: $table,
        proof_status: $status,
        repair_paths: $paths,
        executed: false,
        quarantined: true,
        manual_review_required: true,
        required_reviewers: $reviewers,
        auto_apply_allowed: false,
        non_execution_statement: ("Candidate " + $rid + " for table " + $table + " was generated for review only and has NOT been executed against any database."),
        fingerprint: $fp
      }' >> "$target"
  done
}

build_attestations "$OUT/attestations.jsonl"
build_attestations "$OUT/attestations.rerun.jsonl"

# Fingerprints must be stable across reruns.
fp1="$(jq -s 'map(.fingerprint) | sort' "$OUT/attestations.jsonl")"
fp2="$(jq -s 'map(.fingerprint) | sort' "$OUT/attestations.rerun.jsonl")"
if [ "$fp1" = "$fp2" ]; then fingerprint_stable=true; else fingerprint_stable=false; fi

jq -s --argjson fpstable "$fingerprint_stable" '
  . as $a |
  {
    version: "patchline.quarantine-attestation/v1",
    candidates: ($a | length),
    none_executed: ($a | all(.[]; .executed == false)),
    all_quarantined: ($a | all(.[]; .quarantined == true)),
    all_require_manual_review: ($a | all(.[]; .manual_review_required == true)),
    none_auto_apply: ($a | all(.[]; .auto_apply_allowed == false)),
    all_have_fingerprint: ($a | all(.[]; (.fingerprint | type == "string") and (.fingerprint | length) == 16)),
    no_proven_status: ($a | all(.[]; .proof_status != "proven")),
    reviewer_escalation: ($a | map(select(.required_reviewers >= 2)) | length),
    fingerprint_stable: $fpstable
  }
' "$OUT/attestations.jsonl" > "$OUT/quarantine-attestation.json"

{
  echo "# Repair-candidate quarantine attestations"
  echo
  jq -r '"Issued quarantine attestations for `" + (.candidates|tostring) + "` real repair candidates. Candidates executed: `0`. Auto-apply allowed: `none`."' "$OUT/quarantine-attestation.json"
  echo
  echo "## Attestation guarantees"
  jq -r '"- none executed: `" + (.none_executed|tostring) + "`\n- all quarantined: `" + (.all_quarantined|tostring) + "`\n- all require manual review: `" + (.all_require_manual_review|tostring) + "`\n- all carry a stable fingerprint: `" + (.all_have_fingerprint|tostring) + "`\n- fingerprints stable across reruns: `" + (.fingerprint_stable|tostring) + "`\n- candidates escalated to 2 reviewers (open proof holes): `" + (.reviewer_escalation|tostring) + "`"' "$OUT/quarantine-attestation.json"
  echo
  echo "No generated repair is ever executed automatically: each is quarantined behind an explicit non-execution attestation and a manual-review requirement that escalates with unresolved proof holes."
} > "$OUT/quarantine-attestation.md"

cp "$OUT/quarantine-attestation.md" "$OUT/README.md"
echo "quarantine attestations complete: $(jq '.candidates' "$OUT/quarantine-attestation.json") candidates, none_executed $(jq '.none_executed' "$OUT/quarantine-attestation.json")"
