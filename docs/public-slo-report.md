# Public uptime and reproducibility SLO report

Patchline now treats its public-good infrastructure as an auditable artifact: hosted docs, artifacts, marketplace evidence, and corpus APIs must each have a public URL, public status URL, fresh uptime probes, rerunnable reproducibility probes, hash-backed evidence, latency bounds, and incident-review obligations.

`patchline public-slo-report --spec examples/public-slo-report.json --root . --out results/generated/public-slo-report --json` writes a canonical JSON report plus Markdown summary. The report hash is stable across reruns, and the gate mutates the spec to prove missing status pages, broken artifact hashes, stale probes, breached uptime/reproducibility, missing evidence, and unreviewed incidents are rejected.

Run `make public-slo-report-gate` to reproduce the positive proof and negative controls.
