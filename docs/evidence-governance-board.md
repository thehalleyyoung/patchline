# Shared evidence governance board

Patchline's **shared evidence governance board** is a deterministic process for
accepting, deprecating, or quarantining evidence that has already passed the
public evidence marketplace. It does not delete disputed records. Instead, it
binds each decision to the published evidence hash, certificate subject hash,
reviewer votes, conflict checks, and the marketplace archive mirror's existing
withdrawal/tombstone metadata.

## Lifecycle decisions

```bash
go run ./cmd/patchline evidence-marketplace govern \
  --spec examples/evidence-marketplace/governance-board.json \
  --out results/generated/evidence-governance-board \
  --json
```

The command writes `governance-board.json`, `governance-board.md`, and
`index.html`. A passing report must include at least one accepted, one
deprecated, and one quarantined evidence record so the full lifecycle is tested.

| Decision | Gate behavior |
| --- | --- |
| Accept | Requires a published marketplace example, matching evidence and certificate hashes, public-release eligibility, quorum, and enough independent non-conflicted approvals. |
| Deprecate | Requires the same hash-bound approval path plus an effective date, replacement or continuing-validity note, and preserved archive mirror entries. |
| Quarantine | Requires the same hash-bound approval path plus a trigger, review link, quarantine reason, `preserve_tombstone=true`, and preserved archive mirror entries. |

## Archive-preserving tombstones

Deprecation and quarantine reuse the evidence marketplace archive mirror rather
than inventing a second preservation scheme. Every transitioned artifact must
retain its `archive/sha256/<hash>` mirror path, checksum, withdrawal ID, review
requirement, tombstone requirement, and `preserve_checksum_after_withdrawal`
flag. The board report removes quarantined evidence from active release
eligibility while keeping the redacted proof auditable.

## Reproduce

```bash
make evidence-governance-board-gate
```

The gate runs focused Go tests, executes the governance command on a three-entry
marketplace fixture, verifies accepted/deprecated/quarantined counts and
preserved tombstones, then mutates the board spec to prove weak quorum,
conflicted approvals, and missing tombstone preservation are rejected.
