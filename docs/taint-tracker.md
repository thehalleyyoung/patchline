# Inter-procedural taint tracking

Patchline tracks **taint** across function-call boundaries, linking untrusted request
inputs to migration-affected database columns. A column touched by a migration is flagged
when any tainted value can reach a write to it, even when that value passes through several
intermediate functions.

## Forward propagation with sanitizers

The worker propagates taint forward over the inter-procedural flow graph from the declared
sources. Propagation is blocked at any edge marked sanitized and at an inserted sanitizer
node. It then reports which migration-affected sinks remain tainted.

## What the gate proves

- A multi-hop path (`http_request.body -> parse_payload -> build_user -> users.email`)
  carries taint all the way to the migration-affected column.
- Inserting a sanitizer node on that path cuts the flow: the column is no longer tainted.
- A sink whose only inbound path is sanitized (`logs.line`) stays clean.

## Why it matters

Single-function taint analysis misses flows that cross call boundaries — exactly where
request data reaches the columns a migration is about to constrain. Inter-procedural
propagation closes that gap.

## Reproduce

```
make taint-tracker-gate
```
