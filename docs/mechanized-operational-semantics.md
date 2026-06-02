# Mechanized operational semantics for the migration DSL

Patchline pins a mechanized **operational semantics** for its migration DSL, with a checked-in proof object per reduction rule.

## How it works

The worker walks every reduction rule and confirms it has a non-empty statement and a proof marked machine-checked.

## What the gate proves

- Every reduction rule is mechanized with a checked proof.
- A rule shipped without a checked proof is rejected.

## Why it matters

A mechanized semantics turns the analyzer's correctness claims into statements about a fixed, proof-checked language.

## Reproduce

```
make mechanized-operational-semantics-gate
```
