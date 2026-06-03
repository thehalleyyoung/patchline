# Incident-response drill

Patchline can rehearse a hypothetical false negative as a public incident with evidence-hashed detection, disclosure, mitigation, remediation, regression-gate, and postmortem milestones.

`patchline incident-response-drill` reads a versioned drill spec, resolves every cited evidence artifact under the repo root, hashes the public report, timeline notes, ownership records, disclosure text, remediation proof, and gate report, then emits a deterministic report. The verifier rejects drills whose disclosure or remediation deadlines slip, whose timeline endpoints run backward, whose public summaries contain obvious private markers, whose role ownership is concentrated, or whose regression-gate report hash no longer matches the reviewed artifact.

## Reproduce

```bash
go run ./cmd/patchline incident-response-drill \
  --spec examples/incident-response-drill.json \
  --root . \
  --out results/generated/incident-response-drill \
  --json

make incident-response-drill-gate
```
