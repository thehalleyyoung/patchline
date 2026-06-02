# Runtime redaction stability

Patchline's deterministic `[redacted:<kind>:<hash>]` token policy also covers **runtime
evidence**: trace attributes, log lines, **metric labels**, and **incident text**. This gate
proves the policy is stable when applied to runtime telemetry derived from real findings.

For each real finding the workflow synthesises runtime artifacts carrying sensitive values
(emails, IPv4 addresses, bearer tokens, customer ids, file paths), redacts them, and then
regenerates the whole set independently. The stability contract:

1. **Rerun byte-identical** — re-running the redactor produces the exact same output.
2. **Deterministic tokens** — the same sensitive value maps to the same token across the trace,
   log, metric, and incident artifacts.
3. **No raw-value leaks** — no email, bearer token, customer id, path, or raw IP survives.
4. **Structure preserved** — `finding_id` and `table` stay intact so evidence remains joinable.

```
make runtime-redaction-gate
```

The gate fails on any raw-value leak, any non-deterministic token, any structural loss, or if the
rerun is not byte-identical. Outputs land in `results/generated/runtime-redaction/`.
