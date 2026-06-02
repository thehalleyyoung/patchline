# Generated test mutation checks

A generated test is only useful as review evidence if it actually fails when its stated assumption
is violated. A test that always passes — a *tautological* test — looks reassuring in a diff but
proves nothing. This gate applies **mutation testing to the generated tests themselves**.

Each generated reviewability test carries an explicit oracle derived from a real repair-proof
summary:

> the candidate must target the declared **table** and touch only its declared **repair paths**.

The gate then builds assumption-violating **mutants** of the candidate state and checks that the
test's oracle **kills** each one (evaluates false):

- `m-wrong-table` — point the candidate at a different real table.
- `m-out-of-scope` — add a path outside the declared scope.
- `m-drop-precond` — remove the table-existence precondition.

A test is **effective** when it kills every mutant. The **mutation score** is killed / total.

Guarantees enforced by the gate:

1. Every test passes on its canonical (real) candidate state.
2. Every test kills all of its mutants — **mutation score 1.0**.
3. Determinism across reruns.
4. **Negative control** — a tautological test kills zero mutants, proving the metric distinguishes
   strong tests from vacuous ones.

```
make generated-test-mutation-gate
```

Outputs land in `results/generated/generated-test-mutation/`.
