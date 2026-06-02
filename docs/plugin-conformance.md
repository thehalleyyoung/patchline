# Plugin conformance suite

Patchline exposes a plugin API with a stable contract and a **conformance** test suite that every third-party analyzer must pass.

## How it works

The worker checks each plugin implements every required contract method and passes each conformance case, computing the conformance rate.

## What the gate proves

- A fully implemented plugin conforms.
- A plugin missing a required contract method fails conformance.

## Why it matters

A conformance suite lets an ecosystem of plugins grow without any one of them silently breaking the core guarantees.

## Reproduce

```
make plugin-conformance-gate
```
