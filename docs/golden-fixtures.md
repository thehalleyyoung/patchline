# Golden fixture generation

Patchline can turn a pinned real-repo slice into a tiny deterministic Go test without vendoring the repository. The generator analyzes the real slice, selects a small number of high-signal files that preserve ranked repair risks, recomputes expectations from the miniature tree, and writes a self-contained test package.

```bash
go run ./cmd/patchline golden-fixture generate \
  --github lobsters/lobsters \
  --ref 3b80b47aa5aaba37ec44413e7d1dc96fcf1585b6 \
  --subpath db/migrate \
  --out results/generated/golden-fixtures/lobsters \
  --max-files 3 \
  --json

(cd results/generated/golden-fixtures/lobsters && go test .)
```

The output directory contains:

| Path | Purpose |
| --- | --- |
| `generated_golden_test.go` | Recreates the selected files in `t.TempDir()` and asserts deterministic inventory, intake, and baseline counts. |
| `go.mod` | Tiny module with a local `replace` back to the Patchline checkout so the test imports internal analyzer packages without copying them. |
| `golden-fixture.json` | Machine-readable manifest with source metadata, selected file hashes, reduction ratio, expected counts, and output paths. |
| `golden-fixture.md` | Maintainer-readable summary of selected files and why they were chosen. |

Selection starts with top-ranked baseline risk paths and falls back to small analyzer inputs such as SQL, Ruby, Python, Go, JavaScript, TypeScript, YAML, and JSON files. File count and byte budgets prevent accidental vendoring; generated gates assert that selected fixtures are much smaller than the original real-repo slice.

Use `make golden-fixture-gate` to prove fixture generation on the pinned public slice matrix.
