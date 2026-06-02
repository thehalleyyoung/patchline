# Hardware-cost throughput-per-dollar model

Patchline reports analysis throughput **per dollar** across machine classes via a hardware-cost model.

## How it works

The worker checks each machine class has positive throughput, positive cost, and a positive per-dollar figure.

## What the gate proves

- Every machine class has a valid throughput-per-dollar number.
- A zero-throughput class is rejected.

## Why it matters

Throughput per dollar turns 'it scales' into a concrete budgeting decision for adopters.

## Reproduce

```
make hardware-cost-model-gate
```
