# Starter issues

Patchline generates ready-to-claim **starter issues** directly from structured
**roadmap cards**, so every *good first issue* is concrete, reproducible, and tied
to a gate from the moment it is filed.

## From card to issue

Each roadmap card declares four fields:

- a **title**,
- the **failure mode** the work prevents,
- the **expected gate** the contributor must make pass, and
- the **artifact path** where the generated evidence lands.

The worker renders a complete GitHub-style issue per card with context, the failure
mode, the expected gate, the artifact path, acceptance criteria, and a one-line
reproduce command. A sorted `starter-issues.json` index and a Markdown summary are
also produced.

## Why it stays honest

The gate asserts that every card produces exactly one issue carrying all four
structured fields, that the expected gate name is a valid `*-gate` target, that the
artifact path is under `results/generated/`, and that each rendered issue file
contains the four mandated sections. Contributors therefore never receive a vague
issue: each one already names its failure mode, its gate, and its evidence path.

## Reproduce

```
make starter-issues-gate
```
