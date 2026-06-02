# Paper build pipeline

Patchline builds **paper build** LaTeX artifacts directly from generated gate data,
turning the live catalog of capabilities into a compiled table, a figure, an appendix,
and a list of artifact links rather than hand-maintained prose that can drift from the
implementation.

## Pipeline

The worker reads a manifest of gate names, verifies each has a backing gate script and
documentation file, emits a LaTeX table of capabilities with a bar-style figure and an
appendix of artifact links, and then compiles the result with **pdflatex** into a PDF.

## Why it stays honest

A paper that is generated from the implementation cannot silently drift from it. The gate
proves every referenced artifact exists, the generated LaTeX compiles to a non-empty PDF
under **pdflatex**, and the table contains a row for every manifest entry — so the
capability table is always backed by real, runnable gates.

## Reproduce

```
make paper-build-gate
```
