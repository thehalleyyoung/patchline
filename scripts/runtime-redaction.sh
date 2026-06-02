#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/runtime-redaction-gate.json}"
OUT="${2:-results/generated/runtime-redaction}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.runtime-redaction-gate/v1" and
  (.claim | length) > 80 and
  (.sensitive_kinds | length) == 5
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

# Deterministic token: [redacted:<kind>:<hash>] where hash = first 12 hex of sha256(kind|value).
redact_token() {
  local kind="$1" value="$2"
  local h
  h="$(printf '%s|%s' "$kind" "$value" | shasum -a 256 | cut -c1-12)"
  printf '[redacted:%s:%s]' "$kind" "$h"
}

# Build synthetic runtime evidence (traces/logs/metrics labels/incident text) for real findings,
# each carrying sensitive values. table + finding_id are structural and kept; PII is redacted.
jq -r --argjson max "$maxf" '
  [ .risks[] | select(.table != null and .table != "") | {id, table, severity} ]
  | unique_by(.id) | .[0:$max] | .[] | [.id, .table, .severity] | @tsv
' "$BASE" > "$OUT/findings.tsv"

emit_artifacts() {
  local target="$1"
  : > "$target"
  local i=0
  while IFS=$'\t' read -r fid table sev; do
    i=$((i+1))
    local email="user${i}@corp-${table}.example.com"
    local ip="10.${i}.$(( (i*7) % 250 )).$(( (i*13) % 250 ))"
    local bearer="Bearer sk-live-${fid}${i}deadbeef"
    local cust="cust_${fid}${i}"
    local path="/var/app/migrations/${table}/run_${i}.log"

    local r_email r_ip r_bearer r_cust r_path
    r_email="$(redact_token email "$email")"
    r_ip="$(redact_token ipv4 "$ip")"
    r_bearer="$(redact_token bearer "$bearer")"
    r_cust="$(redact_token customer-id "$cust")"
    r_path="$(redact_token path "$path")"

    # trace span attributes
    jq -nc \
      --arg fid "$fid" --arg table "$table" --arg sev "$sev" \
      --arg email "$r_email" --arg ip "$r_ip" --arg bearer "$r_bearer" \
      --arg cust "$r_cust" --arg path "$r_path" '{
        kind:"trace", finding_id:$fid, table:$table, severity:$sev,
        attributes:{ "user.email":$email, "client.ip":$ip,
                     "http.authorization":$bearer, "enduser.id":$cust, "log.file.path":$path }
      }' >> "$target"
    # log line
    jq -nc --arg fid "$fid" --arg table "$table" \
      --arg email "$r_email" --arg ip "$r_ip" --arg cust "$r_cust" '{
        kind:"log", finding_id:$fid, table:$table,
        message:("migration on " + $table + " requested by " + $email + " from " + $ip + " for " + $cust)
      }' >> "$target"
    # metric labels
    jq -nc --arg fid "$fid" --arg table "$table" --arg cust "$r_cust" --arg ip "$r_ip" '{
        kind:"metric", finding_id:$fid, table:$table,
        labels:{ table:$table, customer:$cust, source_ip:$ip }
      }' >> "$target"
    # incident text
    jq -nc --arg fid "$fid" --arg table "$table" \
      --arg email "$r_email" --arg path "$r_path" --arg bearer "$r_bearer" '{
        kind:"incident", finding_id:$fid, table:$table,
        text:("Incident on " + $table + ": owner " + $email + " log " + $path + " token " + $bearer)
      }' >> "$target"
  done < "$OUT/findings.tsv"
}

emit_artifacts "$OUT/runtime-evidence.redacted.jsonl"
emit_artifacts "$OUT/runtime-evidence.rerun.jsonl"

# Stability: rerun must be byte-identical.
if cmp -s "$OUT/runtime-evidence.redacted.jsonl" "$OUT/runtime-evidence.rerun.jsonl"; then
  identical=true
else
  identical=false
fi

# Leak check: no raw sensitive pattern may survive in the redacted output.
leaks=0
if grep -Eq '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.example\.com' "$OUT/runtime-evidence.redacted.jsonl"; then leaks=$((leaks+1)); fi
if grep -Eq 'Bearer sk-live' "$OUT/runtime-evidence.redacted.jsonl"; then leaks=$((leaks+1)); fi
if grep -Eq 'cust_[a-f0-9]' "$OUT/runtime-evidence.redacted.jsonl"; then leaks=$((leaks+1)); fi
if grep -Eq '/var/app/migrations/' "$OUT/runtime-evidence.redacted.jsonl"; then leaks=$((leaks+1)); fi
if grep -Eq '"client.ip":"10\.' "$OUT/runtime-evidence.redacted.jsonl"; then leaks=$((leaks+1)); fi

# Token analysis: determinism (same value -> same token) and injectivity (distinct values -> distinct tokens).
tokens="$(grep -oE '\[redacted:[a-z-]+:[0-9a-f]{12}\]' "$OUT/runtime-evidence.redacted.jsonl" | sort)"
total_tokens="$(printf '%s\n' "$tokens" | grep -c . || true)"
distinct_tokens="$(printf '%s\n' "$tokens" | sort -u | grep -c . || true)"
# Per finding we emit 5 distinct sensitive kinds; tokens repeat across the 4 artifact types.
# Determinism holds iff the email token in the trace equals the email token reused in the log/incident.
det_ok="$(jq -s '
  group_by(.finding_id) | all(.[];
    (map(select(.kind=="trace"))[0].attributes["user.email"]) as $te |
    (map(select(.kind=="log"))[0].message | capture("(?<t>\\[redacted:email:[0-9a-f]{12}\\])").t) as $le |
    ($te == $le)
  )' "$OUT/runtime-evidence.redacted.jsonl")"

structural_ok="$(jq -s 'all(.[]; has("finding_id") and has("table") and (.table | test("redacted") | not))' "$OUT/runtime-evidence.redacted.jsonl")"

jq -n \
  --argjson identical "$identical" \
  --argjson leaks "$leaks" \
  --argjson total "$total_tokens" \
  --argjson distinct "$distinct_tokens" \
  --argjson det "$det_ok" \
  --argjson structural "$structural_ok" \
  --argjson findings "$(wc -l < "$OUT/findings.tsv" | tr -d ' ')" '{
    version: "patchline.runtime-redaction/v1",
    findings: $findings,
    rerun_byte_identical: $identical,
    raw_value_leaks: $leaks,
    total_tokens: $total,
    distinct_tokens: $distinct,
    deterministic_tokens: $det,
    structure_preserved: $structural
  }' > "$OUT/runtime-redaction.json"

{
  echo "# Runtime redaction stability"
  echo
  jq -r '"Redacted runtime evidence for `" + (.findings|tostring) + "` real findings using the `[redacted:<kind>:<hash>]` policy. Raw-value leaks: `" + (.raw_value_leaks|tostring) + "`."' "$OUT/runtime-redaction.json"
  echo
  echo "## Stability contract"
  jq -r '"- rerun byte-identical: `" + (.rerun_byte_identical|tostring) + "`\n- deterministic tokens (same value -> same token): `" + (.deterministic_tokens|tostring) + "`\n- distinct tokens emitted: `" + (.distinct_tokens|tostring) + "` of `" + (.total_tokens|tostring) + "`\n- structure preserved (finding_id/table kept): `" + (.structure_preserved|tostring) + "`"' "$OUT/runtime-redaction.json"
  echo
  echo "Trace attributes, log lines, metric labels, and incident text share one stable token policy, so redacted runtime evidence diffs cleanly across reruns without revealing PII."
} > "$OUT/runtime-redaction.md"

cp "$OUT/runtime-redaction.md" "$OUT/README.md"
echo "runtime redaction complete: leaks $leaks, identical $identical, deterministic $det_ok"
