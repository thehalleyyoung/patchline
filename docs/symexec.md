# Bounded symbolic execution

Patchline ships **symbolic execution** hooks that explore a migration guard over its full
space of symbolic inputs to decide whether any feasible path reaches an unsafe state,
returning a concrete **witness** assignment when one exists — so an unsafe migration
cannot hide behind an input combination a human reviewer failed to imagine.

## Explore every path

The worker enumerates the bounded symbolic domain of the guard variables, evaluates the
guarded program on every assignment, reports which leaves are reachable, and produces a
satisfying witness for any unsafe leaf. A hardened guard that is unsafe-free yields no
such path.

## Why it matters

Reviewers reason about the inputs they think of. Exhaustively exploring the bounded input
space turns "looks safe" into "no unsafe path exists, and here is the witness if it does".

## Reproduce

```
make symexec-gate
```
