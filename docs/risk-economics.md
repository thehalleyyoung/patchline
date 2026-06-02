# Repair-risk economics

Patchline frames the ship-or-block decision as an explicit **expected**-cost calculation
rather than a binary risk label: it multiplies the probability of failure by the dollar
cost of that failure to get an expected loss, compares it against the known cost of
blocking and remediating up front, and recommends blocking only when the expected loss
from shipping exceeds the cost of holding the migration.

## Minimize expected cost

The worker computes the expected loss and the block cost for each scenario and emits the
cost-minimizing recommendation. This is the core of **repair economics**: trade a certain
small cost now against an uncertain large cost later.

## Why it matters

Blocking every risky-looking migration wastes effort; shipping every one courts disaster.
Expected-cost reasoning picks the economically optimal action per migration.

## Reproduce

```
make risk-economics-gate
```
