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

## Input availability

Ground truth declares the concrete input kinds a prediction may consume. The validator rejects any `allowed_inputs` entry that first becomes available after the case phase, so pre-deploy cases cannot accidentally rely on postmortem or repair-only evidence.

| Input kind | Earliest phase |
| --- | --- |
| `migration_text` | `pre_deploy` |
| `schema` | `pre_deploy` |
| `policy` | `pre_deploy` |
| `prior_archive` | `pre_deploy` |
| `invariants` | `pre_deploy` |
| `repair_plan` | `during_repair` |
| `bounded_store` | `during_repair` |
| `rollback_plan` | `during_repair` |
| `available_backups` | `during_repair` |
| `snapshot_rollback` | `during_repair` |
| `evidence_jsonl` | `postmortem` |
| `postmortem_text` | `postmortem` |
| `source_observations` | `postmortem` |
| `repair_outcome` | `postmortem` |
| `public_postmortem_text` | `postmortem` |
| `public_issue_text` | `postmortem` |
| `root_cause_report` | `postmortem` |
| `current_archive_entry` | `archive_only` |

Forbidden inputs such as `private_production_data`, `preincident_oracle`, and verbatim private database rows should appear in `excluded_inputs`, not in the availability table. They are claim boundaries, not later-available artifact inputs.

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
5. `allowed_inputs` may only list input kinds whose earliest phase is at or before the case phase.
6. Manifest cases may declare `input_kind` when one case type has multiple admissible fixture formats, such as `evidence_jsonl` and `source_observations` for public incidents. The declared input kind must appear in `allowed_inputs`.
7. Ambiguous cases should be labeled `insufficient_evidence` rather than forced into a positive or negative result.

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
