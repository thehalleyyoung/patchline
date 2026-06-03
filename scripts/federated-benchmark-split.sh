#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
SPEC="${1:-examples/federated-benchmark-split-gate.json}"
OUT="${2:-results/generated/federated-benchmark-split}"
rm -rf "$OUT"; mkdir -p "$OUT/bench/fixtures" "$OUT/bench/ground_truth" "$OUT/bench/manifests"

ADOPTER_ID="$(jq -r '.adopter_id' "$SPEC")"
MIN_PRIVATE_CASES="$(jq -r '.min_private_cases' "$SPEC")"
PARTITION_SALT="$(jq -r '.partition_salt' "$SPEC")"
SEED_HEX="$(jq -r '.seed_hex' "$SPEC")"

cat > "$OUT/bench/fixtures/private-broad-update-1.sql" <<'SQL'
UPDATE accounts SET repaired = true;
SQL
cat > "$OUT/bench/fixtures/private-broad-update-2.sql" <<'SQL'
UPDATE users SET repaired = true;
SQL
cat > "$OUT/bench/fixtures/private-broad-update-3.sql" <<'SQL'
UPDATE invoices SET repaired = true;
SQL
cat > "$OUT/bench/fixtures/public-safe.sql" <<'SQL'
UPDATE invoices SET repaired = true WHERE id = 1;
SQL

for case_id in private-broad-update-1 private-broad-update-2 private-broad-update-3; do
  cat > "$OUT/bench/ground_truth/$case_id.json" <<JSON
{
  "case_id": "$case_id",
  "case_type": "migration",
  "phase": "pre_deploy",
  "labels": {"expected_result": "flag", "risk": "high"},
  "evidence": [{"kind": "fixture", "locator": "fixtures/$case_id.sql", "rationale": "unscoped update should be flagged"}],
  "allowed_inputs": ["migration_text"],
  "excluded_inputs": ["postmortem_text"]
}
JSON
done
cat > "$OUT/bench/ground_truth/public-safe.json" <<'JSON'
{
  "case_id": "public-safe",
  "case_type": "migration",
  "phase": "pre_deploy",
  "labels": {"expected_result": "pass", "risk": "low"},
  "evidence": [{"kind": "fixture", "locator": "fixtures/public-safe.sql", "rationale": "scoped update should pass"}],
  "allowed_inputs": ["migration_text"],
  "excluded_inputs": ["postmortem_text"]
}
JSON
cat > "$OUT/bench/manifests/federated.json" <<'JSON'
{
  "version": "patchline.artifact-benchmark/v1",
  "dataset_id": "federated-benchmark-split-gate",
  "description": "Gate fixture proving private local evaluation can publish signed aggregate metrics only.",
  "cases": [
    {"case_id": "private-broad-update-1", "case_type": "migration", "available_at": "pre_deploy", "fixture": "../fixtures/private-broad-update-1.sql", "ground_truth": "../ground_truth/private-broad-update-1.json"},
    {"case_id": "private-broad-update-2", "case_type": "migration", "available_at": "pre_deploy", "fixture": "../fixtures/private-broad-update-2.sql", "ground_truth": "../ground_truth/private-broad-update-2.json"},
    {"case_id": "private-broad-update-3", "case_type": "migration", "available_at": "pre_deploy", "fixture": "../fixtures/private-broad-update-3.sql", "ground_truth": "../ground_truth/private-broad-update-3.json"},
    {"case_id": "public-safe", "case_type": "migration", "available_at": "pre_deploy", "fixture": "../fixtures/public-safe.sql", "ground_truth": "../ground_truth/public-safe.json"}
  ]
}
JSON

go run ./cmd/patchline artifact-benchmark federated-split \
  --manifest "$OUT/bench/manifests/federated.json" \
  --out "$OUT/split.json" \
  --adopter-id "$ADOPTER_ID" \
  --min-private-cases "$MIN_PRIVATE_CASES" \
  --partition-salt "$PARTITION_SALT" \
  --private-case private-broad-update-1 \
  --private-case private-broad-update-2 \
  --private-case private-broad-update-3 \
  --json > "$OUT/split.stdout.json"

go run ./cmd/patchline artifact-benchmark federated-run \
  --split "$OUT/split.json" \
  --seed-hex "$SEED_HEX" \
  --out "$OUT/aggregate.json" \
  --json > "$OUT/aggregate.stdout.json"

go run ./cmd/patchline artifact-benchmark federated-verify \
  --report "$OUT/aggregate.json" \
  --json > "$OUT/verify.json"

jq '.metrics.buckets.matched = 99' "$OUT/aggregate.json" > "$OUT/tampered-aggregate.json"
if go run ./cmd/patchline artifact-benchmark federated-verify --report "$OUT/tampered-aggregate.json" > "$OUT/tampered.log" 2>&1; then
  tampered_ok=true
else
  tampered_ok=false
fi

jq '.case_id = "private-broad-update-1"' "$OUT/aggregate.json" > "$OUT/leaky-aggregate.json"
if go run ./cmd/patchline artifact-benchmark federated-verify --report "$OUT/leaky-aggregate.json" > "$OUT/leaky.log" 2>&1; then
  leaky_ok=true
else
  leaky_ok=false
fi

if grep -E 'private-broad-update-[123]|"fixture"|"ground_truth"|"signals"|"hashes"' "$OUT/aggregate.json" > /dev/null; then
  aggregate_leaks_private=true
else
  aggregate_leaks_private=false
fi

jq -n \
  --slurpfile aggregate "$OUT/aggregate.json" \
  --slurpfile verify "$OUT/verify.json" \
  --arg tampered_ok "$tampered_ok" \
  --arg leaky_ok "$leaky_ok" \
  --arg aggregate_leaks_private "$aggregate_leaks_private" \
  '{
    version:"patchline.federated-benchmark-split/v1",
    ok: (
      $aggregate[0].ok == true and
      $verify[0].ok == true and
      $aggregate[0].signature.version == "patchline.signed-attestation/v1" and
      $aggregate[0].private_case_count == 3 and
      $aggregate[0].metrics.buckets.matched == 3 and
      $aggregate[0].metrics.buckets["actual:flag"] == 3 and
      $tampered_ok == "false" and
      $leaky_ok == "false" and
      $aggregate_leaks_private == "false"
    ),
    all_ok: ($aggregate[0].ok == true and $verify[0].ok == true),
    bad_ok: ($tampered_ok == "true" or $leaky_ok == "true" or $aggregate_leaks_private == "true"),
    signed: ($aggregate[0].signature.version == "patchline.signed-attestation/v1"),
    aggregate_only: ($aggregate_leaks_private == "false"),
    private_case_count: $aggregate[0].private_case_count,
    suppressed_buckets: $aggregate[0].metrics.suppressed_buckets,
    aggregate_hash: $aggregate[0].hash
  }' > "$OUT/out.json"

{
  echo "# Federated benchmark split"
  echo
  echo "Signed aggregate: $(jq -r .signed "$OUT/out.json")"
  echo "Aggregate-only public output: $(jq -r .aggregate_only "$OUT/out.json")"
  echo "Tampered/leaky aggregates rejected: $(jq -r '(.bad_ok|not)' "$OUT/out.json")"
} > "$OUT/out.md"

echo "federated-benchmark-split worker: ok=$(jq -r .ok "$OUT/out.json") aggregate_hash=$(jq -r .aggregate_hash "$OUT/out.json")"

