#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/intervention-contracts-gate.json}"
OUT="${2:-results/generated/intervention-contracts}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.intervention-contracts-gate/v1" and
  (.claim | length) > 100 and
  (.required_sections | length) == 4
' "$SPEC" > /dev/null

repo="$(jq -r '.real_repo.repo' "$SPEC")"
ref="$(jq -r '.real_repo.ref' "$SPEC")"
subpath="$(jq -r '.real_repo.subpath' "$SPEC")"

go run ./cmd/patchline repo analyze \
  --github "$repo" --ref "$ref" --subpath "$subpath" \
  --download-dir "$OUT/cache" --stages inventory,baseline --no-llm \
  --out "$OUT/analysis" --json > "$OUT/analyze.log"

BASE="$OUT/analysis/baseline/baseline.json"

# An intervention contract joins each real repair-proof summary with the policy check on its risk.
# preconditions  = the policy evidence still missing before the intervention can be trusted.
# postconditions = the scope/frame obligations the repair must preserve.
# rollback       = whether a rollback signal is required and present.
# proof_holes    = the unresolved holes the proof summary records (never hidden).
jq '
  (.policy_checks // []) as $pol |
  [ (.repair_proof_summaries // [])[] | . as $p |
    ($pol | map(select(.risk_id == $p.risk_id)) | .[0]) as $pc |
    {
      contract_id: ("contract-" + ($p.id | sub("repair-proof:"; ""))),
      risk_id: $p.risk_id,
      table: $p.table,
      repair_source: $p.repair_source,
      status: $p.status,
      preconditions: (($pc.missing // []) | map({ evidence: ., satisfied: false })),
      postconditions: ($p.obligations // []),
      rollback_assumptions: {
        rollback_required: (($pc.required // []) | index("rollback") != null),
        rollback_present: (($pc.satisfied // []) | index("rollback") != null),
        assumption: (if (($pc.satisfied // []) | index("rollback")) != null
                     then "rollback evidence present"
                     else "rollback NOT proven — intervention must supply a reversible path or be quarantined" end)
      },
      proof_holes: ($p.proof_holes // []),
      claims_proven: false
    }
  ]
' "$BASE" > "$OUT/intervention-contracts.json"

jq '
  . as $c | ($c | length) as $n |
  {
    version: "patchline.intervention-contracts/v1",
    contracts: $n,
    distinct_risks: ($c | map(.risk_id) | unique | length),
    statuses: ($c | group_by(.status) | map({key:.[0].status, value:length}) | from_entries),
    all_four_sections: ($c | all(.[];
      has("preconditions") and has("postconditions") and has("rollback_assumptions") and has("proof_holes"))),
    none_claim_proven: ($c | all(.[]; .claims_proven == false and .status != "proven")),
    contracts_with_open_holes: ($c | map(select((.proof_holes | length) > 0)) | length),
    contracts_with_rollback_gap: ($c | map(select(.rollback_assumptions.rollback_required and (.rollback_assumptions.rollback_present | not))) | length),
    postconditions_present: ($c | all(.[]; (.postconditions | length) > 0))
  }
' "$OUT/intervention-contracts.json" > "$OUT/intervention-contracts-summary.json"

{
  echo "# Intervention contracts"
  echo
  jq -r '"Built `" + (.contracts|tostring) + "` intervention contracts across `" + (.distinct_risks|tostring) + "` real risks. Contracts claiming full proof: `0`."' "$OUT/intervention-contracts-summary.json"
  echo
  echo "## Contract integrity"
  jq -r '"- all four sections present (pre/post/rollback/holes): `" + (.all_four_sections|tostring) + "`\n- none claim a proven status: `" + (.none_claim_proven|tostring) + "`\n- contracts surfacing open proof holes: `" + (.contracts_with_open_holes|tostring) + "`\n- contracts with an unmet rollback assumption: `" + (.contracts_with_rollback_gap|tostring) + "`"' "$OUT/intervention-contracts-summary.json"
  echo
  echo "## Status distribution"
  jq -r '.statuses | to_entries[] | "- " + .key + ": `" + (.value|tostring) + "`"' "$OUT/intervention-contracts-summary.json"
  echo
  echo "Each contract states what must hold before the intervention, what it must preserve, what it assumes about rollback, and which proof holes remain — never claiming proof it does not have."
} > "$OUT/intervention-contracts.md"

cp "$OUT/intervention-contracts.md" "$OUT/README.md"
echo "intervention contracts complete: $(jq '.contracts' "$OUT/intervention-contracts-summary.json") contracts, none_proven $(jq '.none_claim_proven' "$OUT/intervention-contracts-summary.json")"
