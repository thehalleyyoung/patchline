# Billing-and-usage metering with reproducible invoices

Patchline meters usage and produces reproducible **invoice**s from event logs.

## How it works

The worker checks each invoice is recomputed exactly from its underlying event log.

## What the gate proves

- Every invoice is reproducible from events.
- A non-reproducible invoice is rejected.

## Why it matters

Invoices recomputable from raw events make billing disputes resolvable instead of he-said-she-said.

## Reproduce

```
make usage-metering-gate
```
