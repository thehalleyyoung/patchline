# Resumable gates

Patchline gates are **resumable**: when a long public-repository sweep is
**interrupt**ed, the completed per-repository analyses are preserved and a resume
run picks up exactly where it left off rather than recomputing everything.

## Behavior

The worker simulates an interruption partway through a corpus, persists a completion
marker for each finished repository, and then resumes. The resume run skips every
already-completed repository, processes only the remainder, and ends with the full
corpus complete.

## Why it stays honest

The gate asserts that:

- the interrupted run completes exactly the **prefix** (`interrupt_after` repos);
- the resume run recomputes **none** of the preserved work and processes only the
  remainder;
- across both passes, every repository is processed **exactly once** — no wasted
  recomputation and no skipped work.

## Reproduce

```
make resumable-gates-gate
```
