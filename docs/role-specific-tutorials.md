# Role-specific tutorials

Patchline ships **role-specific tutorials** for app developers, DBAs, SREs,
security reviewers, and engineering managers. Each tutorial is compiled from a
structured spec into a Markdown playbook that names the real code or evidence
path, the hazard classes that role owns, concrete gate commands, success
criteria, and the handoff to the next reviewer.

## Why roles get separate paths

A data-change repair review crosses specialties. App developers need to inspect
ORM write paths and generated tests, DBAs need engine semantics and rollback
obligations, SREs need runtime evidence and causality limits, security reviewers
need quarantine and redaction boundaries, and engineering managers need review
burden, fairness, and cost evidence. A single generic tutorial hides those
handoffs; Patchline treats them as testable artifacts.

## What the gate proves

- All five mandated roles are present: app developer, DBA, SRE, security
  reviewer, and engineering manager.
- Every tutorial cites a real repository file or evidence spec that exists in
  the checkout.
- Every role has multiple concrete commands, at least three success checks, a
  review decision, and a handoff.
- A generic tutorial with no role-owned evidence is rejected.

## Reproduce

```bash
make role-specific-tutorials-gate
```
