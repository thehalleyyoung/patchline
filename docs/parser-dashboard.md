# Ecosystem parser quality dashboard

Breadth claims are cheap; Patchline makes its ecosystem coverage **measurable**. The parser quality
dashboard unifies four signals for every supported ecosystem in one place:

- **coverage** — the detector fact kinds each ecosystem emits.
- **real-repo proof** — the gate that proves the detector on real public code.
- **known gaps** — documented limitations, so reviewers see what is *not* yet handled.
- **fuzz robustness** — a deterministic corpus of malformed and high-byte inputs that the analyzer
  must process without a single crash.

The dashboard is generated from [`docs/parser-coverage.json`](parser-coverage.json) plus a live fuzz
run against a freshly built analyzer binary, producing a coverage table and a robustness summary.

Guarantees enforced by the gate:

1. Every ecosystem has a real-repository proof gate.
2. Known gaps are documented for the ecosystems.
3. The full fuzz-seed corpus is processed with zero crashes (each seed survives).

```
make parser-dashboard-gate
```

Outputs land in `results/generated/parser-dashboard/`.
