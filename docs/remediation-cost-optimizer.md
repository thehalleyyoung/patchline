# Remediation-cost optimizer

`patchline remediation-cost` chooses between guard, backfill, expand/contract, and manual review by ranking each option's uncertainty-adjusted expected loss.

```bash
go run ./cmd/patchline remediation-cost \
  --spec examples/remediation-cost-optimizer.json \
  --out results/generated/remediation-cost-optimizer \
  --json
```

The optimizer uses explicit units: `expected_loss = probability * affected_rows * impact_per_row`, then adds `expected_loss * uncertainty` as a risk premium. Options are viable only when their declared evidence is present, except `manual_review`, which is always available as the human fallback. If uncertainty exceeds policy, Patchline escalates to manual review before ranking automated choices. Otherwise it selects the viable option with the lowest total expected loss among options that clear the residual-loss bound.

Reproduce the positive and negative controls with:

```bash
make remediation-cost-optimizer-gate
```
