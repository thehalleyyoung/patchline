# Plugin interfaces

Patchline exposes deterministic plugin interfaces for each analysis seam where ecosystem-specific logic should attach. These interfaces live in `internal/plugins` and wrap the current built-in implementation so plugin authors can build against concrete request/result types before Patchline supports out-of-process loading.

## Interface contract

Every plugin returns an `Info` record:

| Field | Purpose |
| --- | --- |
| `name` | Stable plugin identifier used in catalogs and gates. |
| `kind` | One of `parser`, `fact-extractor`, `linker`, `ranker`, `proposal-generator`, `compare-check`, or `report-renderer`. |
| `version` | Plugin contract version. |
| `deterministic` | Whether identical inputs should produce identical outputs. |
| `inputs` / `outputs` | Human-readable type and artifact expectations. |
| `description` | Maintainer-facing summary. |

The default catalog is available with:

```bash
go run ./cmd/patchline plugins list --json
```

## Extension points

| Kind | Go interface | Input | Output | Built-in adapter |
| --- | --- | --- | --- | --- |
| Parser | `Parser` | local source root | `project.Inventory`, `[]project.Fact` | `project-inventory-parser` |
| Fact extractor | `FactExtractor` | source root plus inventory | `intake.Report`, candidate facts | `project-intake-fact-extractor` |
| Linker | `Linker` | inventory facts plus intake report | `project.BaselineReport`, evidence links | `baseline-evidence-linker` |
| Ranker | `Ranker` | baseline report | ranked baseline risks | `baseline-risk-ranker` |
| Proposal generator | `ProposalGenerator` | baseline, kind, budget | untrusted `project.ProposalReport` | `template-proposal-generator` |
| Compare check | `CompareCheck` | baseline plus proposal | `project.CompareReport`, generated checks | `intervention-compare-check` |
| Report renderer | `ReportRenderer` | report struct plus format | JSON or Markdown bytes plus hash | `json-report-renderer`, `markdown-report-renderer` |

## Probe command

`plugins probe` runs the built-in registry through a full repair-analysis loop on real code:

```bash
go run ./cmd/patchline plugins probe \
  --github lobsters/lobsters \
  --ref 3b80b47aa5aaba37ec44413e7d1dc96fcf1585b6 \
  --subpath db/migrate \
  --out results/generated/plugin-probe/lobsters \
  --json
```

The probe writes `plugin-probe.json`, `plugin-probe.md`, fetched source metadata, baseline/proposal/compare artifacts, and rendered report samples. It verifies that parsers, fact extractors, linkers, rankers, proposal generators, compare checks, and renderers can be composed without network access after fetch and without trusting generated artifacts before compare.

## Design rules for future plugins

1. Keep deterministic plugins deterministic: sort outputs, hash content, and report all proof holes instead of inventing stronger claims.
2. Attach at the earliest honest layer. A framework parser should emit facts; a risk scorer should be a ranker; a CI formatter should be a renderer.
3. Treat proposal outputs as untrusted. New proposal generators must feed compare checks before any review badge can become positive.
4. Prefer narrow types over raw maps. If a plugin needs a new fact shape, add a typed field or property and document its semantics.
5. Preserve enterprise defaults. Plugins should work on local source roots and cached artifacts without requiring production credentials.
