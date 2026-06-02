# Shell portability

Patchline guards its gate scripts against non-portable shell constructs so that gates
run identically on macOS bash 3.2 and Linux without relying on GNU-only or
interactive-only features. This **portability** lint is itself a gate.

## Hazard catalog

The linter scans for:

- `mapfile` / `readarray` — GNU bash 4+ only, absent on macOS bash 3.2;
- writes to `/tmp` — forbidden by the environment content-exclusion policy;
- `sed -i` without the empty backup suffix (`sed -i ''`) that BSD/macOS sed requires.

## Why it stays honest

A lint that never fires proves nothing. The gate pairs the shipped, clean scripts with
a deliberately non-portable **negative-control** fixture that contains every hazard.
The gate proves the shipped scripts are clean while the negative control is flagged for
all three hazards — so the linter is demonstrably capable of catching real problems.

## Reproduce

```
make shell-portability-gate
```
