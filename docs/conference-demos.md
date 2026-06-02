# Conference demos

Patchline ships audience-tailored **conference demo** scripts for Datadog, Microsoft
RISE, database, and programming-languages audiences, each built entirely from
reproducible gate commands so a presenter can run the demo live with zero risk of a
broken slide.

## Run sheets

Every demo declares an audience, a focus, and an ordered list of steps where each
runnable step is a real `make` gate target. The worker renders a presenter run sheet
with timing and a talking point per step. Because every step is a gate, *if it is
green on main, it is green on stage*.

## Why it stays **gate-backed**

The gate asserts that all four mandated audiences — Datadog, Microsoft RISE,
database, and programming-languages — are present and that **every** runnable demo
step maps to a real gate target. A demo can therefore never reference a command that
does not exist.

## Reproduce

```
make conference-demos-gate
```
