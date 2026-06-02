# Polyglot ORM front-end with shared core

Patchline provides a polyglot ORM front-end where every **framework** lowers into one shared core semantics.

## How it works

The worker checks each supported framework has an extractor emitting the shared core representation.

## What the gate proves

- Every framework shares the single core via an extractor.
- A framework with no extractor is rejected.

## Why it matters

A shared core means correctness work done once benefits every framework, instead of N times.

## Reproduce

```
make polyglot-orm-frontend-gate
```
