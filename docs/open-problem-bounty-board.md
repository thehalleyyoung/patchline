# Open-problem bounty board

Patchline provides an open-problem bounty board with gate-checkable success criteria, reproducible submissions, and approved payouts for minimized reproductions of false negatives and high-impact ecosystem gaps. This capability is **bounty board** and is proven reproducibly on real Patchline self-data.

## How it works

The worker loads a frozen, content-addressed spec and confirms that every claimed bounty:

- is categorized as either a `false_negative` or `ecosystem_gap`;
- names a minimized reproduction artifact that exists in the repository;
- references concrete backing evidence drawn from existing Patchline gates;
- has a positive `payout_usd`; and
- has `payment_status` set to `approved`.

## What the gate proves

- False-negative and ecosystem-gap bounty records both clear the workflow.
- Every accepted bounty has an approved payout and a minimized reproduction.
- An unsupported item with empty evidence, no minimized reproduction, and no payout is rejected, so the bounty board claim cannot pass vacuously.

## Why it matters

It keeps the bounty board claim honest: payment credit only counts when a contributor supplies a minimized reproduction that closes a measured false negative or high-impact ecosystem gap and remains reproducible against the repository's own gate-backed evidence.

## Reproduce

```
make open-problem-bounty-board-gate
```
