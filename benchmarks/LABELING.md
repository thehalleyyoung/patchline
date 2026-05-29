# Patchline benchmark labeling protocol

Ground-truth labels must be source-grounded and phase-aware.

## Phases

| Phase | Meaning |
| --- | --- |
| `pre_deploy` | The claim may use migration text, schema, policy, prior archive entries, and known invariants. |
| `during_migration` | The claim may use migration-runner output and observed transition events. |
| `during_repair` | The claim may use bad state, repair plan, rollback facts, bounded store, and available trace/replay data. |
| `postmortem` | The claim may use public incident narrative and final root-cause/repair facts. |
| `archive_only` | The claim may use prior incident artifacts and archive order. |

## Expected results

| Result | Meaning |
| --- | --- |
| `flag` | Patchline should flag the case as risky or review-required. |
| `pass` | Patchline should accept or not flag the case under the declared phase. |
| `verified` | Patchline should produce replay/proof evidence for the declared claim. |
| `cannot_prove` | Patchline has enough input to explain why the proof/replay cannot be established. |
| `insufficient_evidence` | The public or local record is too incomplete for a stronger claim. |
| `unsupported_fragment` | The fragment is outside the current semantic fragment. |

## Label rules

1. Every case must include `case_id`, `case_type`, `phase`, `labels.expected_result`, `labels.risk`, `evidence`, `allowed_inputs`, and `excluded_inputs`.
2. Pre-deploy claims must not cite postmortem-only evidence.
3. Public sources should include a URL, pinned commit or content hash, retrieval date, and license/provenance note where possible.
4. File-based labels must point to committed fixtures or to explicit `requires_fetch` manifests whose fetch target verifies pinned hashes before execution.
5. Ambiguous cases should be labeled `insufficient_evidence` rather than forced into a positive or negative result.

## Evidence kinds

Allowed evidence kinds in the smoke validator:

- `file`
- `url`
- `rule`
- `postmortem`
- `archive`
- `fixture`
- `public_source`
- `sha256`

Future validators may add commit-hash and source-bundle evidence kinds.
