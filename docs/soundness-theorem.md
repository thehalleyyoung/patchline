# Soundness theorem for safe verdicts

Patchline carries a **soundness** theorem: a 'safe' verdict implies the absence of the modeled hazard class.

## How it works

The worker checks every modeled hazard class has a soundness lemma whose proof is machine-checked.

## What the gate proves

- Every modeled hazard class has a sound, proof-backed lemma.
- A class asserted safe without a proof is rejected.

## Why it matters

Soundness is the difference between 'we did not find a hazard' and 'no hazard of this class exists in the model'.

## Reproduce

```
make soundness-theorem-gate
```
