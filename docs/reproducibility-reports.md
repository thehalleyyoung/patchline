# Monthly reproducibility reports

Patchline monthly reproducibility reports rerun public gates and publish cache status, failures, fixes, and benchmark trends from pinned public-code evidence.

```bash
make reproducibility-report-gate
```

The report generator executes each configured public proof gate, captures logs and machine-readable summaries, counts cache files and bytes, records failure remediation notes, and compares current metrics with pinned previous values. The generated artifacts are:

- `reproducibility-report.json`: machine-readable monthly status.
- `report.md`: public monthly report with public gates, cache status, failures, fixes, and benchmark trends.
- `gates/*.run.log`: raw logs for each rerun gate.
- `gates/*/report-row.json`: per-gate status, cache, failure, fix, and trend record.
