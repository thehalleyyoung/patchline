# Office hours

Patchline generates public **office-hours** agendas driven by real signals rather
than guesswork. Each agenda is assembled from recent **reproducibility** failures
and active roadmap cards, time-boxed into review, triage, and planning blocks.

## What drives the agenda

- **Review** — recent gate failures, each rendered with its `make <gate>` command so
  the room can reproduce it live;
- **Triage** — assign owners and decide which failures block the next release;
- **Planning** — active roadmap cards to discuss next.

## Why it stays honest

Every failure item references a gate that actually exists, so the agenda always
points contributors at reproducible work. The gate asserts each failure item maps
to a real gate script, that roadmap discussion items are present, and that the
rendered agenda contains the mandated review, triage, and planning sections.

## Reproduce

```
make office-hours-gate
```
