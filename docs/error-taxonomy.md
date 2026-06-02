# Error taxonomy

Patchline classifies every failure into a structured **error taxonomy** spanning the
six pipeline stages — fetch, parse, analyze, generate, compare, and report — so that
any failure carries a stable code, the stage it belongs to, whether it is
**retryable**, and a concrete remediation.

## Structure

Each entry declares:

- a stable, well-formed **code** (`E_<UPPER_SNAKE>`),
- the **stage** it belongs to,
- a **retryable** boolean, and
- a non-empty **remediation**.

The worker renders the taxonomy into a stable table and a machine-readable index
keyed by code.

## Why it stays honest

The gate asserts that all six stages are represented, that error codes are globally
**unique** and well-formed, and that every entry declares a retryable flag and a
non-empty remediation. A failure can therefore always be triaged by code, stage, and
retry semantics rather than by an opaque string.

## Reproduce

```
make error-taxonomy-gate
```
