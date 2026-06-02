# Feedback forms

Patchline ships **analytics-free** documentation feedback forms that turn reader
input into structured **local issue** templates — without any tracking, network
calls, or third-party scripts.

## How they work

Each form is defined by a name, a set of labelled fields with required flags, and
the issue label it produces. The worker renders:

- a plain-Markdown **form** the reader fills in, and
- a structured **local issue template** (front-matter + fields) the reader can copy
  and paste into a local issue.

There are no external assets, no scripts, and no URLs of any kind.

## Why it stays honest

The gate asserts that every form renders all its fields, marks required fields in
both the form and the template, emits a structured template, and contains **zero**
external URLs or analytics references (`https?://`, `gtag`, `plausible`, `segment`,
`mixpanel`, `<script>`, …). The "analytics-free" claim is therefore enforced, not
just stated.

## Reproduce

```
make feedback-forms-gate
```
