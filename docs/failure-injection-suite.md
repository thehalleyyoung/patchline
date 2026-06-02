# Failure-injection suite

Patchline's failure-injection suite proves artifact gates fail loudly when refs, caches, or generated evidence drift from expected public-code artifacts.

```bash
make failure-injection-suite-gate
```

The suite runs a public-code replication baseline, then injects:

- `ref-drift`: an invalid public repository ref rejected by the capstone gate.
- `cache-drift`: a corrupted public archive checksum rejected by archive validation.
- `generated-evidence-drift`: a corrupted generated-evidence summary rejected by release thresholds.

The generated outputs include `failure-injection-results.json`, reviewer-readable Markdown, per-probe logs, and the public-code evidence baseline used to anchor the probes.
