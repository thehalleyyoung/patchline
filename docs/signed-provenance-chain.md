# Signed provenance chain

Patchline binds every finding to an end-to-end signed **provenance** chain that runs from the input commit through each analysis stage to the printed verdict.

## How it works

The worker walks the ordered chain, verifies each link references its predecessor's digest and carries a signature, and confirms the terminal verdict link is reachable from the input commit.

## What the gate proves

- The full chain is intact and every link is signed.
- A chain with a broken digest link is rejected.

## Why it matters

Cryptographic attribution from input commit to verdict is what lets an auditor trust a finding without re-running it.

## Reproduce

```
make signed-provenance-chain-gate
```
