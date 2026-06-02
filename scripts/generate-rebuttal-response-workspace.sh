#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SPEC="${1:-examples/rebuttal-response-workspace-gate.json}"
OUT="${2:-results/generated/rebuttal-response-workspace}"
rm -rf "$OUT"
mkdir -p "$OUT/evidence" "$OUT/responses"

jq -e '
  .version == "patchline.rebuttal-response-workspace-gate/v1" and
  (.claim | length) > 120 and
  (.concerns | length) >= .minimum_concerns and
  all(.concerns[]; (.id | test("^[a-z0-9-]+$")) and (.question | length) > 40 and (.reviewer_check | length) > 40)
' "$SPEC" > /dev/null

appendix_spec="$(jq -r '.appendix_spec' "$SPEC")"
release_spec="$(jq -r '.release_manifest_spec' "$SPEC")"
bash scripts/generate-paper-appendix.sh "$appendix_spec" "$OUT/evidence/paper-appendix" > "$OUT/evidence/paper-appendix.run.log"
bash scripts/generate-artifact-release-manifest.sh "$release_spec" "$OUT/evidence/release-manifest" > "$OUT/evidence/release-manifest.run.log"

APPENDIX="$OUT/evidence/paper-appendix/appendix.json"
RELEASE="$OUT/evidence/release-manifest/artifact-release-manifest.json"
test -s "$APPENDIX"
test -s "$RELEASE"

response_rows=()
count="$(jq '.concerns | length' "$SPEC")"
for ((i=0; i<count; i++)); do
  id="$(jq -r ".concerns[$i].id" "$SPEC")"
  row="$OUT/response-$id.json"
  md="$OUT/responses/$id.md"
  jq -n \
    --slurpfile spec "$SPEC" \
    --slurpfile appendix "$APPENDIX" \
    --slurpfile release "$RELEASE" \
    --argjson idx "$i" \
    '($spec[0].concerns[$idx]) as $concern |
    ($appendix[0].claims | map(select(((.claim + " " + .paper_wording) | ascii_downcase) | contains($concern.claim_select | ascii_downcase)))) as $matched_claims |
    ($appendix[0].limitations | map(select(.category == $concern.limitation_category))) as $matched_limits |
    {
      id:$concern.id,
      question:$concern.question,
      response:"The current public artifact answers this concern with generated evidence, while preserving the linked limitation instead of overstating the result.",
      evidence_links:([
        "evidence/paper-appendix/appendix.json",
        "evidence/paper-appendix/appendix.md",
        "evidence/release-manifest/artifact-release-manifest.json",
        "evidence/release-manifest/artifact-checksums.sha256"
      ] + ($matched_claims[0:2] | map(.artifacts[0])) + ($appendix[0].figures[0:1] | map(.data_path)) | map(select(. != null)) | unique),
      claims:($matched_claims[0:2] | map({id, section, status, paper_wording, reviewer_check})),
      limitations:($matched_limits[0:2] | map({id, category, severity, observation, why_it_matters, next_evidence})),
      reviewer_check:$concern.reviewer_check,
      reproduction_commands:$appendix[0].reproduction_commands,
      release_doi:$release[0].release.doi,
      archive_count:$release[0].summary.archives,
      public_repos:$appendix[0].summary.public_repos,
      ranked_risks:$appendix[0].summary.ranked_risks,
      status:(if (($matched_claims | length) > 0 and ($matched_limits | length) > 0) then "answerable-with-linked-limitation" else "needs-more-evidence" end)
    }' > "$row"
  {
    echo "# Rebuttal response: $id"
    echo
    jq -r '"**Concern.** " + .question + "\n\n**Response.** " + .response + "\n\n**Status.** `" + .status + "`\n\n**Reviewer check.** " + .reviewer_check' "$row"
    echo
    echo "## Evidence links"
    jq -r '.evidence_links[] | "- `" + . + "`"' "$row"
    echo
    echo "## Linked limitations"
    jq -r '.limitations[] | "- **" + .category + "** (`" + .severity + "`): " + .observation + " " + .why_it_matters' "$row"
    echo
    echo "## Reproduction commands"
    jq -r '.reproduction_commands[] | "- `" + .command + "`"' "$row"
  } > "$md"
  response_rows+=("$row")
done

jq -n \
  --slurpfile responses <(jq -s '.' "${response_rows[@]}") \
  --slurpfile appendix "$APPENDIX" \
  --slurpfile release "$RELEASE" \
  '{
    version:"patchline.rebuttal-response-workspace/v1",
    responses:$responses[0],
    evidence:{
      appendix:"evidence/paper-appendix/appendix.json",
      release_manifest:"evidence/release-manifest/artifact-release-manifest.json",
      release_doi:$release[0].release.doi,
      release_content_hash:$release[0].release.content_hash
    },
    summary:{
      concerns:($responses[0] | length),
      answerable:($responses[0] | map(select(.status == "answerable-with-linked-limitation")) | length),
      evidence_links:([$responses[0][].evidence_links | length] | add),
      linked_limitations:([$responses[0][].limitations | length] | add),
      public_repos:$appendix[0].summary.public_repos,
      ranked_risks:$appendix[0].summary.ranked_risks,
      claims:$appendix[0].summary.claims,
      limitations:$appendix[0].summary.limitations,
      archives:$release[0].summary.archives,
      verified:($appendix[0].summary.verified == true and $release[0].summary.verified == true)
    }
  }' > "$OUT/rebuttal-workspace.json"

{
  echo "# Public rebuttal-response workspace"
  echo
  echo "This workspace maps likely reviewer concerns to generated evidence, explicit limitations, reviewer checks, and response drafts."
  echo
  echo "## Summary"
  jq -r '.summary | "- concerns: `" + (.concerns|tostring) + "`\n- answerable with linked limitations: `" + (.answerable|tostring) + "`\n- evidence links: `" + (.evidence_links|tostring) + "`\n- linked limitations: `" + (.linked_limitations|tostring) + "`\n- public repositories: `" + (.public_repos|tostring) + "`\n- ranked risks: `" + (.ranked_risks|tostring) + "`"' "$OUT/rebuttal-workspace.json"
  echo
  echo "## Response index"
  jq -r '.responses[] | "- [`" + .id + "`](responses/" + .id + ".md): " + .question + " Status: `" + .status + "`."' "$OUT/rebuttal-workspace.json"
  echo
  echo "## Evidence roots"
  jq -r '.evidence | "- appendix: `" + .appendix + "`\n- release manifest: `" + .release_manifest + "`\n- release DOI candidate: `" + .release_doi + "`\n- release content hash: `" + .release_content_hash + "`"' "$OUT/rebuttal-workspace.json"
} > "$OUT/rebuttal-workspace.md"

cp "$OUT/rebuttal-workspace.md" "$OUT/README.md"
echo "rebuttal workspace generated: concerns $(jq '.summary.concerns' "$OUT/rebuttal-workspace.json"), limitations $(jq '.summary.linked_limitations' "$OUT/rebuttal-workspace.json"), risks $(jq '.summary.ranked_risks' "$OUT/rebuttal-workspace.json")"
