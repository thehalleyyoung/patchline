# Canonical JSON + checksum

Patchline **canonical**izes JSON before hashing so that two documents that are
semantically equal but differ only in key order or insignificant whitespace produce
identical canonical bytes and identical **checksum**s, while any document that differs
in content produces a different checksum.

## Method

The worker canonicalizes each input by recursively sorting object keys and emitting
compact separators (`jq -cS`), then computes a SHA-256 checksum over the canonical
bytes.

## Why it stays honest

Proof checksums must be order-independent: a tool that re-emits the same facts in a
different key order should not appear to have changed anything. The gate proves that a
reordered-but-equal document collides with its twin under both canonical form and
checksum, and that a content-changed document does **not** collide — so the checksum
detects real changes and ignores cosmetic ones.

## Reproduce

```
make canonical-json-gate
```
