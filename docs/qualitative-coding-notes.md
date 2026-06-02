# Qualitative coding notes

`patchline repo qualitative-notes` turns one or more `repo analyze` outputs into reviewer-facing qualitative coding notes. The notes are designed for public-corpus sampling and paper/artifact review: they preserve candidate adjudications for `false_positive_candidate`, `false_negative_candidate`, `proof_hole`, and `maintainer_decision` without pretending that deterministic static analysis alone can confirm human ground truth.

```bash
go run ./cmd/patchline repo qualitative-notes \
  --analyses results/generated/qualitative-notes-gate/analyses/lobsters-rails-migrations,results/generated/qualitative-notes-gate/analyses/bytebase-go-migrator \
  --out results/generated/qualitative-notes-gate/notes \
  --json
```

The JSON report writes `qualitative-notes.json` with:

- `rubric`: the coding purpose, labels, statuses, and limitations.
- `notes[]`: individual qualitative notes with `label`, `status`, `confidence`, `repo`, `ref`, `subpath`, optional `risk_id`, `source`, `observation`, `evidence`, `coder_instruction`, `maintainer_question`, and `recommended_decision`.
- `summary`: aggregate counts for analyses, public repos, total notes, `false_positive_notes`, `false_negative_notes`, `proof_hole_notes`, and `maintainer_decision_notes`.
- `corpus`: every public slice that contributed evidence.

The command intentionally uses candidate labels for false positives and false negatives. A note becomes confirmed only when a maintainer or study coder reviews the public source context and writes down why the analyzer should have suppressed, escalated, or relabeled the finding. Proof holes remain explicit evidence gaps, and maintainer decisions are concrete review actions such as requesting rollback evidence, owner review, repeated-run tests, or generated-code rejection.

`make qualitative-notes-gate` validates the command against eight pinned public repository slices and requires all four qualitative categories to appear with evidence and maintainer-facing questions.
