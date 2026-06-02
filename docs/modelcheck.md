# Model checking of migration rollout

Patchline **model check**s a migration rollout as a finite state machine, exhaustively
exploring every reachable state from the initial state and verifying a safety property
holds on all of them. When the property can be violated it returns a concrete
**counterexample** trace of transitions leading to the bad state rather than merely
reporting a boolean.

## Reachability + counterexample

The worker performs a bounded reachability search over the transition relation, evaluates
the *never reach data_loss* invariant on every reachable state, and emits the shortest
offending trace for a model that can reach the bad state.

## Why it matters

A boolean "unsafe" is hard to act on. A shortest counterexample trace shows exactly the
sequence of transitions that leads to data loss, turning verification into a debuggable
artifact.

## Reproduce

```
make modelcheck-gate
```
