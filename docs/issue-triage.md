# Issue triage labels and templates

Patchline uses GitHub issue forms to keep public requests reproducible, safe, and actionable. The forms require pinned public evidence instead of private logs or customer data.

| Form | Labels | Use when |
| --- | --- | --- |
| Real repository nomination | `triage`, `corpus`, `real-repo`, `needs-repro` | Adding a public repo slice or data-change failure mode to the corpus. |
| Ecosystem support request | `triage`, `ecosystem`, `needs-repro` | Requesting a source host, framework, language, migration tool, or evidence format. |
| Parser or extractor request | `triage`, `parser`, `needs-repro` | Requesting a parser, fact extractor, linker, or normalization improvement. |
| False positive report | `triage`, `false-positive`, `needs-repro` | Patchline reported a public finding that appears not actionable. |
| False negative report | `triage`, `false-negative`, `needs-repro` | Patchline missed a risky public data-change behavior or evidence link. |
| Artifact regression report | `triage`, `artifact-regression`, `needs-repro` | Generated reports, bundles, SARIF, gates, fixtures, or CI artifacts changed unexpectedly. |

The shared label catalog lives in `.github/labels.yml` and includes `security-review` for work involving untrusted inputs, generated code execution, redaction, archives, or adapter boundaries.

Run `make issue-template-gate` to validate the label catalog, every issue form, required fields, disabled blank issues, and sample triage payloads generated from a pinned public Bytebase analysis.
