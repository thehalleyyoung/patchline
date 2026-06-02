#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/incident-export-gate.json}"
OUT="${2:-results/generated/incident-export}"
rm -rf "$OUT"
mkdir -p "$OUT/cache"

jq -e '
  .version == "patchline.incident-export-gate/v1" and
  (.claim | length) > 80 and
  (.adapters | length) == 4
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

# Select real high/medium findings and build one incident per finding for each adapter, in each
# provider's native schema with a severity/priority mapping and a link back to the finding.
jq --argjson max "$maxf" '
  [ .risks[] | select(.table != null and .table != "")
    | {id, table, severity, kind, score,
       sr:(if .severity=="high" then 0 elif .severity=="medium" then 1 else 2 end)} ]
  | unique_by(.id) | sort_by([.sr, .id]) | .[0:$max] as $sel |

  def pd_sev:   if . == "high" then "critical" elif . == "medium" then "warning" else "info" end;
  def og_prio:  if . == "high" then "P1" elif . == "medium" then "P3" else "P5" end;
  def sp_impact:if . == "high" then "critical" elif . == "medium" then "major" else "minor" end;

  {
    pagerduty: [ $sel[] | {
      routing_key: "R0UTING_KEY_PLACEHOLDER",
      event_action: "trigger",
      dedup_key: ("patchline-" + .id),
      payload: {
        summary: ("Data-change risk on " + .table + " (" + .kind + ")"),
        source: ("patchline:" + .table),
        severity: (.severity | pd_sev),
        component: .table,
        group: "schema-migration",
        custom_details: { finding_id: .id, table: .table, patchline_severity: .severity, score: .score }
      }
    } ],
    opsgenie: [ $sel[] | {
      message: ("Data-change risk on " + .table),
      alias: ("patchline-" + .id),
      description: ("Patchline flagged " + .kind + " on " + .table),
      priority: (.severity | og_prio),
      tags: ["patchline", .table, .kind],
      details: { finding_id: .id, table: .table, patchline_severity: .severity }
    } ],
    slack: [ $sel[] | {
      channel: "#incidents",
      blocks: [
        { type:"header", text:{ type:"plain_text", text:("Patchline: " + .table) } },
        { type:"section", text:{ type:"mrkdwn",
            text:("*" + .severity + "* risk (" + .kind + ") on `" + .table + "` — finding `" + .id + "`") },
          fields:[
            { type:"mrkdwn", text:("*Table:*\n" + .table) },
            { type:"mrkdwn", text:("*Finding:*\n" + .id) }
          ] }
      ]
    } ],
    statuspage: [ $sel[] | {
      incident: {
        name: ("Schema risk: " + .table),
        status: "investigating",
        impact: (.severity | sp_impact),
        body: ("Patchline flagged " + .kind + " on " + .table + " (finding " + .id + ")"),
        metadata: { patchline: { finding_id: .id, table: .table } }
      }
    } ]
  }
' "$BASE" > "$OUT/incident-exports.json"

for adapter in pagerduty opsgenie slack statuspage; do
  jq ".$adapter" "$OUT/incident-exports.json" > "$OUT/$adapter.json"
done

# Validate each adapter payload against its provider's required-field contract and linkage.
validate="$(jq '
  .pagerduty as $pd | .opsgenie as $og | .slack as $sl | .statuspage as $sp |
  {
    pagerduty_valid: ($pd | length>0 and all(.[];
      has("routing_key") and (.event_action=="trigger") and (.dedup_key|startswith("patchline-")) and
      (.payload|has("summary") and (.severity|IN("critical","warning","info","error")) and (.custom_details.finding_id|length>0)))),
    opsgenie_valid: ($og | length>0 and all(.[];
      (.message|length>0) and (.alias|startswith("patchline-")) and
      (.priority|IN("P1","P2","P3","P4","P5")) and (.details.finding_id|length>0))),
    slack_valid: ($sl | length>0 and all(.[];
      (.blocks|length>=2) and (.blocks[0].type=="header") and
      (any(.blocks[]; .type=="section")))),
    statuspage_valid: ($sp | length>0 and all(.[];
      (.incident.name|length>0) and (.incident.status|IN("investigating","identified","monitoring","resolved")) and
      (.incident.impact|IN("critical","major","minor","none")) and (.incident.metadata.patchline.finding_id|length>0))),
    severity_mapped: (
      ($pd | all(.[]; .payload.severity|IN("critical","warning","info"))) and
      ($og | all(.[]; .priority|IN("P1","P3","P5"))) and
      ($sp | all(.[]; .incident.impact|IN("critical","major","minor")))
    )
  }
' "$OUT/incident-exports.json")"

# Linkage: every adapter references the same set of real finding ids.
linkage="$(jq '
  ([.pagerduty[].payload.custom_details.finding_id] | sort | unique) as $a |
  ([.opsgenie[].details.finding_id] | sort | unique) as $b |
  ([.slack[].blocks[1].text.text | capture("finding `(?<id>[^`]+)`").id] | sort | unique) as $c |
  ([.statuspage[].incident.metadata.patchline.finding_id] | sort | unique) as $d |
  ($a == $b and $b == $d and $c == $a)
' "$OUT/incident-exports.json")"

count="$(jq '.pagerduty | length' "$OUT/incident-exports.json")"

jq -n --argjson v "$validate" --argjson linkage "$linkage" --argjson count "$count" '
  {
    version: "patchline.incident-export/v1",
    incidents_per_adapter: $count,
    pagerduty_valid: $v.pagerduty_valid,
    opsgenie_valid: $v.opsgenie_valid,
    slack_valid: $v.slack_valid,
    statuspage_valid: $v.statuspage_valid,
    severity_mapped: $v.severity_mapped,
    cross_adapter_linkage: $linkage,
    all_valid: ($v.pagerduty_valid and $v.opsgenie_valid and $v.slack_valid and $v.statuspage_valid)
  }
' > "$OUT/incident-export.json"

{
  echo "# Incident export adapters"
  echo
  jq -r '"Exported `" + (.incidents_per_adapter|tostring) + "` real findings to PagerDuty, Opsgenie, Slack, and Statuspage."' "$OUT/incident-export.json"
  echo
  echo "## Adapter validation"
  jq -r '"- PagerDuty Events API v2 valid: `" + (.pagerduty_valid|tostring) + "`\n- Opsgenie alerts valid: `" + (.opsgenie_valid|tostring) + "`\n- Slack Block Kit valid: `" + (.slack_valid|tostring) + "`\n- Statuspage incidents valid: `" + (.statuspage_valid|tostring) + "`\n- severity/priority mapped per provider: `" + (.severity_mapped|tostring) + "`\n- cross-adapter finding linkage: `" + (.cross_adapter_linkage|tostring) + "`"' "$OUT/incident-export.json"
  echo
  echo "Each provider receives a payload in its own native schema, with Patchline severity mapped to the provider's vocabulary and a stable link back to the originating finding."
} > "$OUT/incident-export.md"

cp "$OUT/incident-export.md" "$OUT/README.md"
echo "incident export complete: per-adapter $count, all_valid $(jq '.all_valid' "$OUT/incident-export.json")"
