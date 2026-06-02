# End-to-end provenance for every paper number

Patchline proves end-to-end **provenance** from raw corpus to every number in the camera-ready PDF.

## How it works

The worker checks each number in the paper traces through a recorded pipeline back to raw data.

## What the gate proves

- Every paper number is provenance-traced.
- An untraceable number is rejected.

## Why it matters

End-to-end provenance means a reviewer can follow any number in the PDF back to the raw bytes.

## Reproduce

```
make end-to-end-provenance-gate
```
