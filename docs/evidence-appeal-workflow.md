# Evidence appeal workflow

Patchline's **evidence appeal workflow** turns disputed findings into auditable records instead of private side channels. An appeal is valid only when it is bound to the published marketplace evidence hash, certificate subject hash, governance-board decision, preserved archive mirror entries, independent reviewer rationales, and a final resolution trail.

## What the gate proves

```bash
go run ./cmd/patchline evidence-marketplace appeal \
  --spec examples/evidence-marketplace/appeal-workflow.json \
  --out results/generated/evidence-appeal-workflow \
  --json
```

The command writes `appeal-workflow.json`, `appeal-workflow.md`, and `index.html`. The fixture processes three realistic appeal outcomes: one upheld finding, one modified finding, and one overturned interpretation. All three keep the original evidence intact.

| Requirement | Enforcement |
| --- | --- |
| Preserved evidence | Every appeal must list every marketplace archive mirror entry for the disputed evidence, including checksum, mirror path, withdrawal ID, and tombstone flags. |
| Reviewer rationale | Appeal reviewers must satisfy quorum, cite preserved evidence, and be independent of both the submitter and original board approvers. |
| Resolution audit trail | The final outcome must be `upheld`, `modified`, or `overturned`, include resolver, rationale, follow-up actions, and occur after submission. |
| Private-data safety | All appeal text, reviewer rationale, and resolution text are scanned for high-signal private markers before publication. |

Appeals do not mutate marketplace history. They publish a follow-on audit artifact that can uphold, modify, or overturn the interpretation while preserving the evidence chain needed for later review.

## Reproduce

```bash
make evidence-appeal-workflow-gate
```
