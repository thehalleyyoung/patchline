# Incident export adapters

Findings with observed runtime impact should flow into the on-call stack teams already use. This
gate exports real findings to four incident-management adapters in each provider's native schema:

- **PagerDuty** — Events API v2 (`routing_key`, `event_action: trigger`, `payload.summary`,
  `severity`, `custom_details`).
- **Opsgenie** — alerts (`message`, `alias`, `priority` P1–P5, `details`).
- **Slack** — Block Kit (`header` + `section` blocks).
- **Statuspage** — incidents (`incident.name`, `status`, `impact`, `metadata`).

Contract enforced by the gate:

1. **Schema-valid** — each payload carries its provider's required fields.
2. **Severity mapped** — Patchline severity maps to each provider's vocabulary (high →
   critical / P1 / critical-impact, etc.).
3. **Cross-adapter linkage** — every adapter references the same set of real finding ids.

```
make incident-export-gate
```

Outputs land in `results/generated/incident-export/`, with one file per adapter plus a combined
`incident-exports.json`.
