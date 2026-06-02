# Governance

Patchline ships **role-specific governance** documentation so that maintainers,
security reviewers, research reviewers, and ecosystem owners each have an explicit,
reproducible charter rather than tribal knowledge.

## One charter per role

The governance worker renders a charter for every role from structured data, each
with:

- a **scope**,
- named **responsibilities**,
- an **escalation** path, and
- the **accountable gates** that role is responsible for keeping green.

A governance index (`governance.json`, `governance.md`) collects all four charters.

## Why it stays honest

The gate asserts that all four mandated roles — `maintainer`, `security-reviewer`,
`research-reviewer`, and `ecosystem-owner` — are present, that every charter
declares a scope, at least three responsibilities, an escalation path, and at least
one accountable gate, and that each rendered charter contains the four mandated
sections. Governance therefore cannot silently lose a role or a responsibility.

## Reproduce

```
make governance-gate
```
