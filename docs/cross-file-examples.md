# Cross-file repair clue examples

`patchline repo cross-file-examples` reads one or more `repo analyze` outputs and writes side-by-side examples where Patchline finds a cross-file clue that grep-only and SQL-only baselines miss. The report focuses on repair, incident, and source/test paths linked to a ranked data-change risk through repository identifiers and provenance slices.

```bash
go run ./cmd/patchline repo cross-file-examples \
  --analyses results/generated/cross-file-examples-gate/analyses/lobsters-rails-migrations,results/generated/cross-file-examples-gate/analyses/bytebase-go-migrator \
  --out results/generated/cross-file-examples-gate/examples \
  --json
```

The JSON report writes `cross-file-examples.json` with:

- `examples[]`: side-by-side examples containing `patchline_clue`, `grep_only_result`, `sql_only_result`, `why_grep_only_missed`, `why_sql_only_missed`, `maintainer_action`, `clue_kind`, `clue_paths`, `risk_path`, `risk_id`, `identifiers`, and supporting `evidence`.
- `baseline_comparison`: the Patchline evidence-link count beside `grep_only_matches` and `sql_only_ranked_risks`.
- `summary`: counts for analyses, public repos, examples, `repair_clues`, `incident_clues`, `source_clues`, `grep_only_misses`, and `sql_only_misses`.
- `corpus`: every public repository slice used to build the examples.

The Markdown report renders each example as a table with Patchline, grep-only, and SQL-only rows. The baseline rows are intentionally not framed as broken tools: grep-only and SQL-only are useful narrow checks, but they do not preserve repository-native provenance, repair wording, temporal hints, native commands, or non-SQL evidence paths. The maintainer action names the concrete review step that follows from the cross-file clue.

`make cross-file-examples-gate` validates the command against pinned public repository slices and requires real repair clues with both grep-only and SQL-only misses.
