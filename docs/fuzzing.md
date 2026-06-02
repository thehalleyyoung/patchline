# Fuzzing

Patchline has seed-driven Go fuzz targets for fragile input boundaries:

| Area | Target | Package |
| --- | --- | --- |
| SQL analysis | `FuzzAnalyzeBytesWithDialect` | `internal/migration` |
| Parser/source extraction | `FuzzExtractSourceSQL`, `FuzzParserPipeline` | `internal/migration`, `internal/project` |
| Fact normalization | `FuzzFactNormalization` | `internal/project` |
| Archive extraction | `FuzzArchiveExtraction` | `internal/project` |
| Report loading | `FuzzReportLoading` | `internal/project` |
| Redaction | `FuzzBundleRedactor` | `cmd/patchline` |

Normal `go test` runs the fuzz seed corpus once. To stress a target locally:

```bash
go test ./internal/migration -run '^$' -fuzz=FuzzAnalyzeBytesWithDialect -fuzztime=10s -parallel=1
go test ./internal/project -run '^$' -fuzz=FuzzArchiveExtraction -fuzztime=10s -parallel=1
go test ./cmd/patchline -run '^$' -fuzz=FuzzBundleRedactor -fuzztime=10s -parallel=1
```

Use `make fuzz-coverage-gate` for the repository gate. It runs focused fuzz seed tests, stress-runs each fuzz target briefly, then proves the same analyzer boundaries on pinned real-repo slices by generating and executing minimal golden fixtures without vendoring entire repositories.
