# Generated intervention diff minimization

A generated intervention should be the smallest diff a reviewer must read. This gate proves
Patchline can minimize an intervention bundle across four component categories — **tests, guards,
instrumentation, and repair candidates** — to the smallest subset that still does the job.

For each real finding with required policy evidence, the workflow:

1. Builds a deliberately **redundant** bundle of components, each covering some required evidence
   item (guard, rollback, approval, dry-run, test) across the four categories.
2. Minimizes the bundle by **set-cover** over the required-evidence universe.
3. Proves the result is **1-minimal**: removing any remaining component drops coverage.

Guarantees enforced by the gate:

- The minimized diff still covers every required evidence item.
- Every minimized diff is strictly smaller than the full bundle (a real reduction).
- Every minimized diff is 1-minimal — no generated line is redundant.

```
make diff-minimization-gate
```

Outputs land in `results/generated/diff-minimization/`.
