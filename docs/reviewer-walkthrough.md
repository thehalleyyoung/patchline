# Reviewer walkthrough

The reviewer walkthrough is a fresh-machine reproduction path for a future artifact evaluator. It starts from a clean checkout with `go`, `git`, `bash`, and `jq`, downloads pinned public repository slices, and ends with regenerated tables, figures, reports, and a case-study bundle.

```bash
git clone https://github.com/thehalleyyoung/patchline.git
cd patchline
make reviewer-walkthrough-gate
```

The underlying command is:

```bash
bash scripts/reviewer-walkthrough.sh \
  examples/reviewer-walkthrough-gate.json \
  results/generated/reviewer-walkthrough-gate
```

Outputs:

- `environment.json`: tool versions, repository commit, and reproducibility assumptions.
- `analyses/`: fresh `repo analyze` outputs for each pinned public repo slice.
- `tables/evaluation-tables.json` and `tables/evaluation-tables.md`: regenerated corpus, risk, generated-artifact, figure, report, and case-study tables.
- `figures/`: SVG/JSON figures for the repair-analysis loop, architecture, corpus composition, ablations, and before/after intervention outcomes.
- `reports/`: claims-to-evidence, limitations ledger, failure taxonomy, qualitative notes, and aggregate metrics reports.
- `case-study-bundle/`: case-study JSON/Markdown, bundle manifest, checksums, and reproduction commands.
- `walkthrough.md` and `summary.json`: reviewer-facing index and machine-checkable success summary.

The walkthrough intentionally uses pinned public refs and deterministic `--no-llm` generation. It does not claim production repair success; it proves that an evaluator can regenerate the paper-facing artifacts from real public code without hidden state.
