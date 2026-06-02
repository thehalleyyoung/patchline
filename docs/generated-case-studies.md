# Generated public-repo case studies

`patchline repo case-studies` converts completed `repo analyze` directories into narrative case studies. Each case is generated from real Patchline artifacts and includes:

- the top detected data-change problem;
- deterministic evidence summaries from baseline and compare reports;
- the generated untrusted intervention summary;
- deterministic acceptance or rejection status from `repo compare`;
- the maintainer-facing next action; and
- the reproduction command for the pinned repo slice.

Example:

```bash
patchline repo case-studies \
  --analyses results/generated/lobsters,results/generated/bytebase \
  --out results/generated/case-studies \
  --json
```

The command writes `case-studies.json` and `case-studies.md`. The report is designed for paper-quality narratives: it keeps the raw artifacts reproducible while making the problem, evidence, generated intervention, deterministic outcome, and maintainer action easy to inspect side by side.

In JSON, each case includes `problem`, `evidence`, `generated_intervention`, `deterministic_outcome`, and `maintainer_action`.

Run `make generated-case-studies-gate` to generate and validate at least eight case studies from pinned public repositories.
