#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/issue-to-artifact-gate.json}"
OUT="${2:-results/generated/issue-to-artifact}"
rm -rf "$OUT"
mkdir -p "$OUT/proofs"

jq -e '.version == "patchline.issue-to-artifact-gate/v1" and (.submissions|length) >= 2' "$SPEC" > /dev/null

results="$OUT/results.jsonl"
: > "$results"
accepted=0
rejected=0

n="$(jq '.submissions | length' "$SPEC")"
for i in $(seq 0 $((n-1))); do
  title="$(jq -r ".submissions[$i].title" "$SPEC")"
  repo="$(jq -r ".submissions[$i].repo" "$SPEC")"
  ref="$(jq -r ".submissions[$i].ref" "$SPEC")"
  cap="$(jq -r ".submissions[$i].capability" "$SPEC")"
  claim="$(jq -r ".submissions[$i].claim // empty" "$SPEC")"

  reason=""
  # A submission is admissible only if it is reproducibly pinned and maps to a gate.
  if ! printf '%s' "$ref" | grep -Eq '^[0-9a-f]{40}$|^refs/tags/.+'; then
    reason="unpinned-ref"
  elif [ ! -f "scripts/${cap}-gate.sh" ]; then
    reason="unknown-capability"
  elif [ -z "$claim" ]; then
    reason="missing-claim"
  fi

  if [ -z "$reason" ]; then
    accepted=$((accepted+1))
    # Deterministic proof-entry id from repo+ref+capability.
    pid="$(printf '%s|%s|%s' "$repo" "$ref" "$cap" | shasum | cut -c1-12)"
    jq -n --arg id "$pid" --arg title "$title" --arg repo "$repo" --arg ref "$ref" \
      --arg cap "$cap" --arg claim "$claim" -c \
      '{id:$id, title:$title, repo:$repo, ref:$ref, capability:$cap, claim:$claim, gate:("scripts/"+$cap+"-gate.sh")}' \
      > "$OUT/proofs/$pid.json"
    jq -n --arg title "$title" --arg status "accepted" --arg id "$pid" -c \
      '{title:$title, status:$status, proof_id:$id}' >> "$results"
  else
    rejected=$((rejected+1))
    jq -n --arg title "$title" --arg status "rejected" --arg reason "$reason" -c \
      '{title:$title, status:$status, reason:$reason}' >> "$results"
  fi
done

sort -o "$results" "$results"

# Build a deterministic proofs index from the accepted entries.
jq -s 'sort_by(.id)' "$OUT"/proofs/*.json > "$OUT/proofs-index.json" 2>/dev/null || echo "[]" > "$OUT/proofs-index.json"

{
  echo "# Pinned public proof entries"
  echo
  echo "Accepted user submissions converted into reproducible, pinned proof entries."
  echo
  echo "| Title | Repo | Ref | Capability |"
  echo "|---|---|---|---|"
  jq -r '.[] | "| \(.title) | `\(.repo)` | `\(.ref)` | `\(.capability)` |"' "$OUT/proofs-index.json"
} > "$OUT/proofs.md"
cp "$OUT/proofs.md" "$OUT/README.md"

jq -n \
  --argjson accepted "$accepted" \
  --argjson rejected "$rejected" \
  --slurpfile results <(jq -s '.' "$results") '
  {
    version: "patchline.issue-to-artifact/v1",
    accepted: $accepted,
    rejected: $rejected,
    results: $results[0]
  }' > "$OUT/issue-to-artifact.json"

echo "issue-to-artifact worker: $accepted accepted, $rejected rejected"
