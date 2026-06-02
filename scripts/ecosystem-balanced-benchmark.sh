#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/ecosystem-balanced-benchmark-gate.json}"
OUT="${2:-results/generated/ecosystem-balanced-benchmark}"
rm -rf "$OUT"
mkdir -p "$OUT/analyses" "$OUT/cache"

CATALOG="$(jq -r '.source_catalog' "$SPEC")"

jq -e '
  .version == "patchline.ecosystem-balanced-benchmark-gate/v1" and
  (.claim | length) > 100 and
  (.required_frameworks | length) == 9 and
  (.proof_slices | length) >= 5 and
  all(.proof_slices[]; (.repo | contains("/")) and (.ref | test("^[0-9a-f]{40}$")) and (.subpath | length) > 0)
' "$SPEC" > /dev/null

jq -e '.version == "patchline.real-repo-catalog/v1" and (.slices | length) >= 25' "$CATALOG" > /dev/null

# Build a balanced manifest: equal number of slices per required framework.
jq -n \
  --slurpfile spec "$SPEC" \
  --slurpfile catalog "$CATALOG" \
  '
  ($spec[0].required_frameworks) as $req |
  ($catalog[0].slices) as $slices |
  ($req | map(. as $fw | {
    framework: $fw,
    available: ([$slices[] | select(.migration_framework == $fw)] | length),
    slices: [$slices[] | select(.migration_framework == $fw) | {id, repo, ref, subpath, ecosystem, framework: .migration_framework}]
  })) as $groups |
  ($groups | map(.available) | min) as $balanced_count |
  {
    version: "patchline.ecosystem-balanced-benchmark/v1",
    required_frameworks: $req,
    balanced_count: $balanced_count,
    groups: ($groups | map(. + {selected: (.slices[0:$balanced_count])})),
    balance: {
      frameworks: ($groups | length),
      min_available: ($groups | map(.available) | min),
      max_available: ($groups | map(.available) | max),
      balanced_count: $balanced_count,
      total_balanced_slices: ($balanced_count * ($groups | length)),
      perfectly_balanced: ($balanced_count > 0)
    }
  }
  ' > "$OUT/balanced-manifest.json"

# Flatten the balanced selection into a benchmark manifest (one line per slice).
manifest_jsonl="$OUT/benchmark-manifest.jsonl"
jq -c '.groups[].selected[] |
  . + {command: ("go run ./cmd/patchline repo analyze --github " + .repo + " --ref " + .ref + " --subpath " + .subpath + " --stages inventory,baseline --no-llm --out results/generated/ecosystem-balanced/" + .id)}
' "$OUT/balanced-manifest.json" > "$manifest_jsonl"

# Prove real-code execution across a diverse proof sample.
proof_jsonl="$OUT/proof.jsonl"
: > "$proof_jsonl"
proof_count="$(jq '.proof_slices | length' "$SPEC")"
for ((p=0; p<proof_count; p++)); do
  repo="$(jq -r ".proof_slices[$p].repo" "$SPEC")"
  ref="$(jq -r ".proof_slices[$p].ref" "$SPEC")"
  subpath="$(jq -r ".proof_slices[$p].subpath" "$SPEC")"
  ecosystem="$(jq -r ".proof_slices[$p].ecosystem" "$SPEC")"
  framework="$(jq -r ".proof_slices[$p].framework" "$SPEC")"
  id="$(printf '%s' "${repo//\//-}" | tr -c 'A-Za-z0-9_.-' '-')"
  analysis="$OUT/analyses/$id"
  mkdir -p "$analysis"
  go run ./cmd/patchline repo analyze \
    --github "$repo" \
    --ref "$ref" \
    --subpath "$subpath" \
    --download-dir "$OUT/cache" \
    --stages inventory,baseline \
    --no-llm \
    --out "$analysis" \
    --json > "$OUT/analyze-$id.json"
  jq -n \
    --arg repo "$repo" --arg ref "$ref" --arg subpath "$subpath" \
    --arg ecosystem "$ecosystem" --arg framework "$framework" \
    --slurpfile analyze "$analysis/analyze.json" \
    '{
      repo:$repo, ref:$ref, subpath:$subpath, ecosystem:$ecosystem, framework:$framework,
      files_scanned:$analyze[0].summary.files_scanned,
      facts:$analyze[0].summary.facts,
      ranked_risks:$analyze[0].summary.ranked_risks,
      hash:$analyze[0].hash,
      verified:(($analyze[0].summary.files_scanned > 0) and ($analyze[0].summary.facts > 0) and (($analyze[0].hash | length) > 0))
    }' >> "$proof_jsonl"
done

jq -n \
  --slurpfile manifest "$OUT/balanced-manifest.json" \
  --slurpfile proof <(jq -s '.' "$proof_jsonl") \
  '{
    version: "patchline.ecosystem-balanced-benchmark/v1",
    balance: $manifest[0].balance,
    required_frameworks: $manifest[0].required_frameworks,
    groups: ($manifest[0].groups | map({framework, available, selected: (.selected | length)})),
    proof: {
      slices: $proof[0],
      frameworks_proven: ($proof[0] | map(.framework) | unique | length),
      ecosystems_proven: ($proof[0] | map(.ecosystem) | unique | length),
      total_ranked_risks: ([$proof[0][].ranked_risks] | add // 0),
      total_files: ([$proof[0][].files_scanned] | add // 0),
      verified: (all($proof[0][]; .verified == true))
    },
    summary: {
      frameworks: ($manifest[0].required_frameworks | length),
      balanced_count: $manifest[0].balanced_count,
      total_balanced_slices: $manifest[0].balance.total_balanced_slices,
      proof_slices: ($proof[0] | length),
      proof_ranked_risks: ([$proof[0][].ranked_risks] | add // 0),
      verified: (($manifest[0].balance.perfectly_balanced) and (all($proof[0][]; .verified == true)))
    }
  }' > "$OUT/ecosystem-balanced-benchmark.json"

{
  echo "# Ecosystem-balanced benchmark slices"
  echo
  echo "Patchline builds an ecosystem-balanced benchmark manifest that gives equal representation to Rails, Django, Alembic, Prisma, TypeORM, Liquibase, Flyway, EF Core, and Go migrators, audits the balance, and proves the slices analyze real downloaded code across a diverse proof sample."
  echo
  echo "## Balance audit"
  jq -r '.balance | "- frameworks: `" + (.frameworks|tostring) + "`\n- slices per framework (balanced): `" + (.balanced_count|tostring) + "`\n- total balanced slices: `" + (.total_balanced_slices|tostring) + "`\n- catalog availability per framework: `" + (.min_available|tostring) + "`..`" + (.max_available|tostring) + "`\n- perfectly balanced: `" + (.perfectly_balanced|tostring) + "`"' "$OUT/ecosystem-balanced-benchmark.json"
  echo
  echo "## Frameworks"
  echo
  echo "| Framework | Catalog available | Balanced selected |"
  echo "| --- | ---: | ---: |"
  jq -r '.groups[] | "| " + .framework + " | " + (.available|tostring) + " | " + (.selected|tostring) + " |"' "$OUT/ecosystem-balanced-benchmark.json"
  echo
  echo "## Real-code proof sample"
  echo
  echo "| Repo | Framework | Ecosystem | Files | Facts | Ranked risks | Verified |"
  echo "| --- | --- | --- | ---: | ---: | ---: | :---: |"
  jq -r '.proof.slices[] | "| `" + .repo + "` | " + .framework + " | " + .ecosystem + " | " + (.files_scanned|tostring) + " | " + (.facts|tostring) + " | " + (.ranked_risks|tostring) + " | " + (if .verified then "yes" else "no" end) + " |"' "$OUT/ecosystem-balanced-benchmark.json"
} > "$OUT/ecosystem-balanced-benchmark.md"

cp "$OUT/ecosystem-balanced-benchmark.md" "$OUT/README.md"
echo "ecosystem-balanced benchmark complete: frameworks $(jq '.summary.frameworks' "$OUT/ecosystem-balanced-benchmark.json"), balanced/framework $(jq '.summary.balanced_count' "$OUT/ecosystem-balanced-benchmark.json"), proof risks $(jq '.summary.proof_ranked_risks' "$OUT/ecosystem-balanced-benchmark.json")"
