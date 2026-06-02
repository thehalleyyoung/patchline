# Contributor check

`patchline contributor check` gives contributors one deterministic command for local hygiene before opening a pull request. It runs the same categories maintainers otherwise ask for manually:

```bash
go run ./cmd/patchline contributor check \
  --packages ./cmd/patchline \
  --gates gate,impact-gate \
  --out results/generated/contributor-check \
  --json
```

The command writes `contributor-check.json`, `contributor-check.md`, and per-step logs under `logs/`. The default run includes:

| Step | Purpose |
| --- | --- |
| `roadmap-ignore` | Proves the private roadmap file remains ignored with the repository's configured ignore rules. |
| `forbidden-doc-refs` | Scans tracked Markdown for private roadmap references while excluding ignored private planning files. |
| `gofmt` | Checks all non-generated Go files for formatting drift without rewriting them. |
| `diff-check` | Runs `git diff --check` for whitespace errors. |
| `focused-go-tests` | Runs `go test` only for selected or inferred packages instead of the entire repository. |
| `fast-gate-*` | Runs explicitly selected fast `make` gates such as `gate` and `impact-gate`. |

Use `--packages pkg[,pkg...]` to make the focused test set explicit, `--gates target[,target...]` to bound the fast gates, and `--plan-only` when documenting what would run without executing commands.

Run `make contributor-check-gate` to prove the command on the real Patchline repository. The gate requires the report to include formatting, focused test, fast gate, ignored-roadmap, and forbidden-reference scan steps, with all executed steps passing.
