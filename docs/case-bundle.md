# Archival case-study bundle

Patchline ships an archival **case-study bundle** that pairs a small number of deep
narrative case studies with a long tail of **lightweight** worked examples, so a reviewer
can read a handful of end-to-end stories in depth while still seeing the breadth of
situations the tool handles across dozens of compact examples.

## Two tiers, one bundle

The worker assembles the bundle, requiring every deep case study to carry a narrative of
at least a minimum length plus linked evidence, while lightweight examples need only a
one-line summary. Any deep study whose narrative is too shallow to count is rejected from
the deep tier.

## Why it matters

Depth and breadth are both evidence. The bundle proves the tool is documented at both
scales, and the shallow-narrative rejection keeps the deep tier from being padded.

## Reproduce

```
make case-bundle-gate
```
