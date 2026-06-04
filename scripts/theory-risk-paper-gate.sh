#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

DOC="docs/theory-risk-paper.md"
OUT="${1:-results/generated/theory-risk-paper-gate}"
BIN="$OUT/patchline"

test -s "$DOC"

for phrase in \
  "finite effect lattice" \
  "high   iff score >= 90" \
  "medium iff 50 <= score < 90" \
  "high-risk-sql" \
  "persistent-write-code-path" \
  "a navigation aid, not proof of causality" \
  "A risk is a request for review backed"; do
  grep -F "$phrase" "$DOC" > /dev/null
done

for code_phrase in \
  "func rankRisks" \
  "func severityForScore" \
  "score >= 90" \
  "score >= 50" \
  "\"high-risk-sql\", 100" \
  "\"medium-risk-sql\", 30" \
  "\"persistent-write-code-path\", 45" \
  "func buildSymbolicChecks" \
  "func buildPolicyChecks" \
  "EffectUnknown          Effect = \"unknown\""; do
  rg -F "$code_phrase" internal > /dev/null
done

rm -rf "$OUT"
mkdir -p "$OUT"

go build -o "$BIN" ./cmd/patchline

"$BIN" repo inventory demos/billing --out "$OUT/inventory" > "$OUT/inventory.log"
"$BIN" intake demos/billing --out "$OUT/intake" > "$OUT/intake.log"
"$BIN" repo baseline \
  --inventory "$OUT/inventory" \
  --intake "$OUT/intake" \
  --out "$OUT/baseline" > "$OUT/baseline.log"

jq -e '
  .version == "patchline.repo-baseline/v1" and
  (.summary.ranked_risks > 0) and
  (.summary.ranking_explanations > 0) and
  (.summary.abstract_effects > 0) and
  (.summary.symbolic_checks > 0) and
  (.summary.policy_checks > 0) and
  (.summary.proof_hole_minimizations > 0) and
  ([.risks[].factors[].name] | index("high-risk-sql") != null) and
  ([.ranking_explanations[].contributions[].feature] | index("high-risk-sql") != null)
' "$OUT/baseline/baseline.json" > /dev/null

jq -n --slurpfile b "$OUT/baseline/baseline.json" '{
  version: "patchline.theory-risk-paper-gate/v1",
  ranked_risks: $b[0].summary.ranked_risks,
  ranking_explanations: $b[0].summary.ranking_explanations,
  abstract_effects: $b[0].summary.abstract_effects,
  symbolic_checks: $b[0].summary.symbolic_checks,
  policy_checks: $b[0].summary.policy_checks,
  proof_hole_minimizations: $b[0].summary.proof_hole_minimizations,
  verified: true
}' > "$OUT/gate-summary.json"

echo "theory-risk-paper gate passed: paper constants match implementation and demo baseline produced ranked, explained, proof-obligated risks"
